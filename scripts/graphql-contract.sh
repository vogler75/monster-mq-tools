#!/usr/bin/env bash
set -euo pipefail
script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# Resolve caller-supplied checkout paths before changing into the standalone Go module.
args=()
while (($#)); do
  case "$1" in
    --output-dir)
      flag="$1"; shift
      if (($# == 0)); then echo "$flag requires a directory" >&2; exit 2; fi
      if [[ "$1" = /* ]]; then args+=("$flag" "$1"); else args+=("$flag" "$PWD/$1"); fi ;;
    --output-dir=*)
      value="${1#*=}"
      if [[ "$value" = /* ]]; then args+=("--output-dir" "$value"); else args+=("--output-dir" "$PWD/$value"); fi ;;
    --main-root|--edge-root|--cli-root)
      flag="$1"; shift
      if (($# == 0)); then echo "$flag requires a checkout directory" >&2; exit 2; fi
      args+=("$flag" "$(cd -- "$1" && pwd)") ;;
    --main-root=*|--edge-root=*|--cli-root=*)
      flag="${1%%=*}"; value="${1#*=}"
      args+=("$flag" "$(cd -- "$value" && pwd)") ;;
    *) args+=("$1") ;;
  esac
  shift
done
cd "$script_dir/graphql-contract"
exec go run . "${args[@]}"
