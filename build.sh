#!/bin/bash
set -euo pipefail

# build.sh - Master build script for MonsterMQ Tools (cli, i3x, gql, mbp)
#
# Usage:
#   ./build.sh            Build native binaries for all tools (default)
#   ./build.sh --all      Cross-compile binaries for all supported platforms
#   ./build.sh --native   Build native binaries for current host
#   ./build.sh --cli      Build mmq CLI only
#   ./build.sh --i3x      Build i3x CLI only
#   ./build.sh --gql      Build compare-schemas only
#   ./build.sh --mbp      Build mbp orchestrator only
#   ./build.sh --clean    Clean all build output directories

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

BUILD_ALL=false
BUILD_NATIVE=false
BUILD_CLI=false
BUILD_I3X=false
BUILD_GQL=false
BUILD_MBP=false
CLEAN=false
EXPLICIT_SUBTOOL=false

usage() {
    echo "Usage: $0 [options]"
    echo ""
    echo "Options:"
    echo "  --all            Cross-compile all tools for linux, darwin, and windows"
    echo "  --native         Build native binaries for host platform (default)"
    echo "  --cli            Build mmq (MonsterMQ CLI) only"
    echo "  --i3x            Build i3x CLI only"
    echo "  --gql            Build compare-schemas tool only"
    echo "  --mbp            Build mbp pipeline tool only"
    echo "  --clean          Clean all bin/ output directories"
    echo "  -h, --help       Show this help message"
    echo ""
    exit 0
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --all|-a)
            BUILD_ALL=true
            shift
            ;;
        --native|-n)
            BUILD_NATIVE=true
            shift
            ;;
        --cli)
            BUILD_CLI=true
            EXPLICIT_SUBTOOL=true
            shift
            ;;
        --i3x)
            BUILD_I3X=true
            EXPLICIT_SUBTOOL=true
            shift
            ;;
        --gql)
            BUILD_GQL=true
            EXPLICIT_SUBTOOL=true
            shift
            ;;
        --mbp)
            BUILD_MBP=true
            EXPLICIT_SUBTOOL=true
            shift
            ;;
        --clean|-c)
            CLEAN=true
            shift
            ;;
        -h|--help)
            usage
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            usage
            ;;
    esac
done

if [ "$CLEAN" = true ]; then
    echo -e "${YELLOW}Cleaning all tools bin/ directories...${NC}"
    rm -rf cli/bin i3x/bin gql/bin mbp/bin
    echo -e "${GREEN}✓ All tools cleaned successfully.${NC}"
    exit 0
fi

# If no specific subtools selected, target all subtools
if [ "$EXPLICIT_SUBTOOL" = false ]; then
    BUILD_CLI=true
    BUILD_I3X=true
    BUILD_GQL=true
    BUILD_MBP=true
fi

# Default to native build if neither --all nor --native was specified
if [ "$BUILD_ALL" = false ] && [ "$BUILD_NATIVE" = false ]; then
    BUILD_NATIVE=true
fi

BUILD_MODE_FLAG="--native"
if [ "$BUILD_ALL" = true ]; then
    BUILD_MODE_FLAG="--all"
fi

echo -e "${BLUE}======================================================${NC}"
echo -e "${BLUE}  Building MonsterMQ Tools (${BUILD_MODE_FLAG})${NC}"
echo -e "${BLUE}======================================================${NC}"

if [ "$BUILD_CLI" = true ]; then
    echo -e "\n${YELLOW}>>> [MonsterMQ CLI] Building cli...${NC}"
    (cd cli && ./build.sh "$BUILD_MODE_FLAG")
fi

if [ "$BUILD_I3X" = true ]; then
    echo -e "\n${YELLOW}>>> [i3X CLI] Building i3x...${NC}"
    (cd i3x && ./build.sh "$BUILD_MODE_FLAG")
fi

if [ "$BUILD_GQL" = true ]; then
    echo -e "\n${YELLOW}>>> [GraphQL Tools] Building gql...${NC}"
    (cd gql && ./build.sh "$BUILD_MODE_FLAG")
fi

if [ "$BUILD_MBP" = true ]; then
    echo -e "\n${YELLOW}>>> [Build Pipeline] Building mbp...${NC}"
    (cd mbp && ./build.sh "$BUILD_MODE_FLAG")
fi

echo ""
echo -e "${GREEN}======================================================${NC}"
echo -e "${GREEN}  All selected MonsterMQ Tools built successfully!     ${NC}"
echo -e "${GREEN}======================================================${NC}"
