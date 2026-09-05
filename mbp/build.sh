#!/bin/bash

# build.sh - Build script for mbp (MonsterMQ Build Pipeline)
#
# Usage:
#   ./build.sh            Build native Go binary for host (bin/mbp)
#   ./build.sh --all      Cross-compile binaries for linux, darwin, and windows
#   ./build.sh --clean    Clean output directory

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

VERSION="0.1.0"
if [ -f "version.txt" ]; then
    VERSION=$(head -n 1 version.txt | tr -d '\n' | tr -d '\r')
elif [ -f "../version.txt" ]; then
    VERSION=$(head -n 1 ../version.txt | tr -d '\n' | tr -d '\r')
fi

LDFLAGS="-s -w -X main.Version=${VERSION}"
GOFLAGS="-trimpath"

usage() {
    echo "Usage: $0 [options]"
    echo ""
    echo "Options:"
    echo "  --native         Build native binary for current platform (default)"
    echo "  --all            Cross-compile binaries for linux, darwin, and windows"
    echo "  --clean          Clean bin/ output directory"
    echo "  -h, --help       Show this help message"
    echo ""
    exit 0
}

BUILD_NATIVE=true
BUILD_ALL=false
CLEAN=false

if [ $# -gt 0 ]; then
    case "$1" in
        --all)
            BUILD_NATIVE=false
            BUILD_ALL=true
            ;;
        --native)
            BUILD_NATIVE=true
            BUILD_ALL=false
            ;;
        --clean)
            BUILD_NATIVE=false
            BUILD_ALL=false
            CLEAN=true
            ;;
        -h|--help)
            usage
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            usage
            ;;
    esac
fi

if [ "$CLEAN" = true ]; then
    echo -e "${YELLOW}Cleaning bin/ directory...${NC}"
    rm -rf bin
    echo -e "${GREEN}✓ Cleaned${NC}"
    exit 0
fi

mkdir -p bin

if [ "$BUILD_NATIVE" = true ]; then
    echo -e "${GREEN}Building native mbp binary (version ${VERSION})...${NC}"
    CGO_ENABLED=0 go build ${GOFLAGS} -ldflags="${LDFLAGS}" -o bin/mbp ./cmd/mbp
    echo -e "${GREEN}✓ Native binary built at: ${YELLOW}mbp/bin/mbp${NC}"
fi

if [ "$BUILD_ALL" = true ]; then
    echo -e "${GREEN}Cross-compiling mbp for all targets (version ${VERSION})...${NC}"

    # Linux
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ${GOFLAGS} -ldflags="${LDFLAGS}" -o bin/mbp-linux-amd64 ./cmd/mbp
    GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build ${GOFLAGS} -ldflags="${LDFLAGS}" -o bin/mbp-linux-arm64 ./cmd/mbp

    # macOS (Darwin)
    GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build ${GOFLAGS} -ldflags="${LDFLAGS}" -o bin/mbp-darwin-amd64 ./cmd/mbp
    GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build ${GOFLAGS} -ldflags="${LDFLAGS}" -o bin/mbp-darwin-arm64 ./cmd/mbp

    # Windows
    GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ${GOFLAGS} -ldflags="${LDFLAGS}" -o bin/mbp-windows-amd64.exe ./cmd/mbp

    echo -e "${GREEN}✓ All binaries built in: ${YELLOW}mbp/bin/${NC}"
    ls -lh bin/
fi
