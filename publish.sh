#!/bin/bash
set -euo pipefail

# publish.sh - Upload MonsterMQ Tools binaries to GitHub Releases
#
# Usage:
#   ./publish.sh              # Publish available tool binaries to GitHub Release
#   ./publish.sh --all        # Publish all tool binaries
#   ./publish.sh -b, --build  # Build tools before publishing
#   ./publish.sh -y, --yes    # Auto-confirm without prompt
#   ./publish.sh -h, --help   # Show help message

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

if [ ! -f "version.txt" ]; then
    echo -e "${RED}Error: version.txt not found${NC}"
    exit 1
fi

VERSION=$(head -n 1 version.txt | tr -d '\n' | tr -d '\r')
TAG="v${VERSION}"

BUILD_BEFORE_PUBLISH=false
AUTO_CONFIRM=false

usage() {
    echo "Usage: $0 [options]"
    echo ""
    echo "Uploads MonsterMQ Tools binaries to GitHub Releases for tag ${TAG}."
    echo ""
    echo "Options:"
    echo "  --all, -a          Publish all tools release binaries"
    echo "  --build, -b        Cross-compile all tools first (via ./build.sh --all) before uploading"
    echo "  -y, --yes          Auto-confirm prompt (non-interactive)"
    echo "  -t, --tag <tag>    Override release tag (default: ${TAG})"
    echo "  -h, --help         Show this help message"
    echo ""
    exit 0
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --all|-a|all)
            shift
            ;;
        --build|-b)
            BUILD_BEFORE_PUBLISH=true
            shift
            ;;
        -y|--yes)
            AUTO_CONFIRM=true
            shift
            ;;
        -t|--tag)
            if [ -n "${2:-}" ]; then
                TAG="$2"
                shift 2
            else
                echo -e "${RED}Error: --tag requires a value${NC}"
                exit 1
            fi
            ;;
        -h|--help|help)
            usage
            ;;
        *)
            echo -e "${RED}Unknown argument: $1${NC}"
            usage
            ;;
    esac
done

echo -e "${GREEN}=== MonsterMQ Tools GitHub Publisher (${TAG}) ===${NC}"

# Check GitHub CLI
if ! command -v gh &> /dev/null; then
    echo -e "${RED}Error: GitHub CLI ('gh') is not installed.${NC}"
    exit 1
fi

if ! gh auth status &> /dev/null; then
    echo -e "${RED}Error: GitHub CLI is not authenticated.${NC}"
    echo "Run: gh auth login"
    exit 1
fi

# Build if requested
if [ "$BUILD_BEFORE_PUBLISH" = true ]; then
    echo -e "${YELLOW}Building tools before publishing...${NC}"
    ./build.sh --all
fi

# Collect binary files
RELEASE_FILES=()
shopt -s nullglob
for f in cli/bin/* i3x/bin/* gql/bin/* mbp/bin/*; do
    if [ -f "$f" ]; then
        RELEASE_FILES+=("$f")
    fi
done
shopt -u nullglob

# If no files found and build was not run, prompt or auto-build
if [ ${#RELEASE_FILES[@]} -eq 0 ]; then
    echo -e "${YELLOW}No binaries found in tool output directories.${NC}"
    if [ "$AUTO_CONFIRM" = false ]; then
        read -p "Would you like to build all tools now? (y/n) " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            ./build.sh --all
        else
            echo -e "${RED}No artifacts to publish. Exiting.${NC}"
            exit 1
        fi
    else
        ./build.sh --all
    fi

    shopt -s nullglob
    for f in cli/bin/* i3x/bin/* gql/bin/* mbp/bin/*; do
        if [ -f "$f" ]; then
            RELEASE_FILES+=("$f")
        fi
    done
    shopt -u nullglob
fi

if [ ${#RELEASE_FILES[@]} -eq 0 ]; then
    echo -e "${RED}Error: No release artifacts found to upload.${NC}"
    exit 1
fi

echo -e "${GREEN}Tools artifacts to upload for ${YELLOW}${TAG}${GREEN}:${NC}"
for file in "${RELEASE_FILES[@]}"; do
    SIZE=$(du -h "$file" | cut -f1)
    echo -e "  • ${BLUE}${file}${NC} (${SIZE})"
done
echo ""

# Confirm before upload
if [ "$AUTO_CONFIRM" = false ]; then
    read -p "Upload these artifacts to GitHub release ${TAG}? (y/n) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo -e "${YELLOW}Publish cancelled by user.${NC}"
        exit 0
    fi
fi

# Check / push git tag
if git rev-parse "$TAG" >/dev/null 2>&1; then
    if ! git ls-remote --tags origin "$TAG" 2>/dev/null | grep -q "$TAG"; then
        echo -e "${YELLOW}Tag ${TAG} exists locally but not on remote. Pushing tag...${NC}"
        git push origin "$TAG" || {
            echo -e "${YELLOW}Warning: Could not push tag to origin (may need permissions). Continuing...${NC}"
        }
    fi
fi

# Upload or create release
if gh release view "$TAG" &> /dev/null; then
    echo -e "${YELLOW}Uploading artifacts to existing GitHub release ${TAG}...${NC}"
    gh release upload "$TAG" "${RELEASE_FILES[@]}" --clobber
else
    echo -e "${YELLOW}Creating new GitHub release ${TAG}...${NC}"
    RELEASE_NOTES="releases/${TAG}.txt"
    if [ -f "$RELEASE_NOTES" ]; then
        gh release create "$TAG" "${RELEASE_FILES[@]}" --title "MonsterMQ Tools ${TAG}" --notes-file "$RELEASE_NOTES"
    else
        gh release create "$TAG" "${RELEASE_FILES[@]}" --title "MonsterMQ Tools ${TAG}" --generate-notes
    fi
fi

echo -e "${GREEN}✓ MonsterMQ Tools binaries published successfully to GitHub release ${TAG}!${NC}"
