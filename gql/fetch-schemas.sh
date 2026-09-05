#!/bin/bash

# fetch-schemas.sh - Fetch GraphQL SDL schemas from MonsterMQ Main & Edge brokers

SCRIPT_NAME="$(basename "$0")"

show_help() {
    cat << EOF
Usage: $SCRIPT_NAME [OPTIONS] [URL...]

Fetch GraphQL SDL schemas from running MonsterMQ Main & Edge brokers.

Options:
  -m, --main     Fetch only the Main Broker schema
  -e, --edge     Fetch only the Edge Broker schema
  -a, --all      Fetch both schemas (default behavior)
  -c, --compare  Compare fetched schemas after fetching
  -h, --help     Show this help message and exit

Arguments:
  When fetching both (default):
    [MAIN_URL]   URL for Main Broker (default: http://localhost:4000/graphql)
    [EDGE_URL]   URL for Edge Broker (default: http://localhost:4001/graphql)

  When fetching only one (-m or -e):
    [URL]        URL for the specified broker

Environment Variables:
  MAIN_URL       Override default Main Broker URL
  EDGE_URL       Override default Edge Broker URL

Examples:
  ./$SCRIPT_NAME                              # Fetch both brokers (default URLs)
  ./$SCRIPT_NAME -m                           # Fetch only Main Broker
  ./$SCRIPT_NAME -e                           # Fetch only Edge Broker
  ./$SCRIPT_NAME -m http://host:4000/graphql  # Fetch Main from custom URL
  ./$SCRIPT_NAME -e http://host:4001/graphql  # Fetch Edge from custom URL
  ./$SCRIPT_NAME http://m:4000/graphql http://e:4001/graphql
EOF
}

FETCH_MAIN=false
FETCH_EDGE=false
COMPARE_AFTER=false
POSITIONAL_ARGS=()

while [[ $# -gt 0 ]]; do
    case "$1" in
        -h|--help)
            show_help
            exit 0
            ;;
        -m|--main|--main-only)
            FETCH_MAIN=true
            shift
            ;;
        -e|--edge|--edge-only)
            FETCH_EDGE=true
            shift
            ;;
        -a|--all)
            FETCH_MAIN=true
            FETCH_EDGE=true
            shift
            ;;
        -c|--compare)
            COMPARE_AFTER=true
            shift
            ;;
        -*)
            echo "Unknown option: $1"
            echo "Run './$SCRIPT_NAME --help' for usage."
            exit 1
            ;;
        *)
            POSITIONAL_ARGS+=("$1")
            shift
            ;;
    esac
done

# If neither was explicitly requested, fetch both by default
if [ "$FETCH_MAIN" = false ] && [ "$FETCH_EDGE" = false ]; then
    FETCH_MAIN=true
    FETCH_EDGE=true
fi

# Assign URLs based on selection
if [ "$FETCH_MAIN" = true ] && [ "$FETCH_EDGE" = true ]; then
    MAIN_URL="${MAIN_URL:-${POSITIONAL_ARGS[0]:-http://localhost:4000/graphql}}"
    EDGE_URL="${EDGE_URL:-${POSITIONAL_ARGS[1]:-http://localhost:4001/graphql}}"
elif [ "$FETCH_MAIN" = true ]; then
    MAIN_URL="${MAIN_URL:-${POSITIONAL_ARGS[0]:-http://localhost:4000/graphql}}"
elif [ "$FETCH_EDGE" = true ]; then
    EDGE_URL="${EDGE_URL:-${POSITIONAL_ARGS[0]:-http://localhost:4001/graphql}}"
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUTPUT_DIR="$SCRIPT_DIR"

mkdir -p "$OUTPUT_DIR"

echo "Fetching GraphQL schemas..."
if [ "$FETCH_MAIN" = true ]; then
    echo "  Main Broker: $MAIN_URL"
fi
if [ "$FETCH_EDGE" = true ]; then
    echo "  Edge Broker: $EDGE_URL"
fi
echo ""

fetch_schema() {
    local name="$1"
    local url="$2"
    local target="$3"
    local rel_target
    rel_target="$(basename "$(dirname "$target")")/$(basename "$target")"

    local tmp_file
    tmp_file=$(mktemp)
    local err_file
    err_file=$(mktemp)

    # Fetch to a temporary file first so existing valid schema files are not overwritten on failure
    if npx -y get-graphql-schema "$url" > "$tmp_file" 2> "$err_file" && [ -s "$tmp_file" ]; then
        mv "$tmp_file" "$target"
        rm -f "$err_file"
        local size
        size=$(du -h "$target" | awk '{print $1}')
        echo "✓ $name schema saved to $rel_target ($size)"
        return 0
    else
        rm -f "$tmp_file"
        echo "✗ Failed to fetch $name schema from $url"
        if [ -s "$err_file" ]; then
            sed 's/^/    /' "$err_file"
        else
            echo "    (Server offline, unreachable, or returned an empty schema)"
        fi
        rm -f "$err_file"
        return 1
    fi
}

HAS_ERROR=0

if [ "$FETCH_MAIN" = true ]; then
    fetch_schema "Main Broker" "$MAIN_URL" "$OUTPUT_DIR/main.gql" || HAS_ERROR=1
fi

if [ "$FETCH_EDGE" = true ]; then
    fetch_schema "Edge Broker" "$EDGE_URL" "$OUTPUT_DIR/edge.gql" || HAS_ERROR=1
fi

echo ""
if [ $HAS_ERROR -eq 0 ]; then
    echo "Done!"
    if [ "$COMPARE_AFTER" = true ]; then
        echo ""
        echo "Running schema comparison..."
        (cd "$SCRIPT_DIR" && go run . "$OUTPUT_DIR/main.gql" "$OUTPUT_DIR/edge.gql")
    fi
    exit 0
else
    echo "Completed with errors (check if target brokers are running)."
    exit 1
fi
