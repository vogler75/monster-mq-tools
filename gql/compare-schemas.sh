#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

if [ -x "./bin/compare-schemas" ]; then
    exec ./bin/compare-schemas "$@"
else
    exec go run . "$@"
fi
