#!/bin/bash

# build.sh - Build script for mmq (MonsterMQ CLI)
#
# Usage:
#   ./build.sh            Build native Go binary for host
#   ./build.sh --all      Cross-compile binaries for all supported platforms
#   ./build.sh --clean    Clean output directory

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

VERSION="0.1.0"
if [ -f "../version.txt" ]; then
    VERSION=$(head -n 1 ../version.txt | tr -d '\n' | tr -d '\r')
elif [ -f "version.txt" ]; then
    VERSION=$(head -n 1 version.txt | tr -d '\n' | tr -d '\r')
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
    echo -e "${GREEN}Building native mmq binary (version ${VERSION})...${NC}"
    CGO_ENABLED=0 go build ${GOFLAGS} -ldflags="${LDFLAGS}" -o bin/mmq .
    echo -e "${GREEN}✓ Native binary built at: ${YELLOW}cli/bin/mmq${NC}"
fi

if [ "$BUILD_ALL" = true ]; then
    echo -e "${GREEN}Cross-compiling mmq for all targets (version ${VERSION})...${NC}"

    # Linux
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ${GOFLAGS} -ldflags="${LDFLAGS}" -o bin/mmq-linux-amd64 .
    GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build ${GOFLAGS} -ldflags="${LDFLAGS}" -o bin/mmq-linux-arm64 .
    GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 go build ${GOFLAGS} -ldflags="${LDFLAGS}" -o bin/mmq-linux-armv7 .

    # macOS (Darwin)
    GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build ${GOFLAGS} -ldflags="${LDFLAGS}" -o bin/mmq-darwin-amd64 .
    GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build ${GOFLAGS} -ldflags="${LDFLAGS}" -o bin/mmq-darwin-arm64 .

    # Windows
    GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ${GOFLAGS} -ldflags="${LDFLAGS}" -o bin/mmq-windows-amd64.exe .

    echo -e "${GREEN}✓ All binaries built in: ${YELLOW}cli/bin/${NC}"
    ls -lh bin/
fi
