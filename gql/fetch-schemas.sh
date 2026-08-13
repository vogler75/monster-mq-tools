#!/bin/bash

# fetch-schemas.sh - Fetch GraphQL SDL schemas from MonsterMQ Main & Edge brokers

set -e

MAIN_URL="${MAIN_URL:-${1:-http://localhost:4000/graphql}}"
EDGE_URL="${EDGE_URL:-${2:-http://localhost:4001/graphql}}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUTPUT_DIR="$SCRIPT_DIR"

mkdir -p "$OUTPUT_DIR"

echo "Fetching GraphQL schemas..."
echo "  Main Broker: $MAIN_URL"
echo "  Edge Broker: $EDGE_URL"
echo ""

# Fetch Main Broker Schema
if npx -y get-graphql-schema "$MAIN_URL" > "$OUTPUT_DIR/main.gql" 2>/dev/null; then
    echo "✓ Main Broker schema saved to gql/main.gql ($(du -h "$OUTPUT_DIR/main.gql" | cut -f1))"
else
    echo "✗ Failed to fetch Main Broker schema from $MAIN_URL"
    exit 1
fi

# Fetch Edge Broker Schema
if npx -y get-graphql-schema "$EDGE_URL" > "$OUTPUT_DIR/edge.gql" 2>/dev/null; then
    echo "✓ Edge Broker schema saved to gql/edge.gql ($(du -h "$OUTPUT_DIR/edge.gql" | cut -f1))"
else
    echo "✗ Failed to fetch Edge Broker schema from $EDGE_URL"
    exit 1
fi

echo ""
echo "Done!"
