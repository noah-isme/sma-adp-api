#!/usr/bin/env bash
set -euo pipefail

VALUE=${1:-}
if [[ "$VALUE" != "true" && "$VALUE" != "false" ]]; then
        echo "Usage: make toggle-go value=true|false" >&2
        exit 1
fi

ENV_FILE=".env"
if [[ ! -f "$ENV_FILE" ]]; then
        if [[ -f ".env.example" ]]; then
                cp .env.example "$ENV_FILE"
        else
                echo "No .env or .env.example found" >&2
                exit 1
        fi
fi

tmp_file=$(mktemp)
trap 'rm -f "$tmp_file"' EXIT

awk -v value="$VALUE" '
BEGIN {updated=0}
/^ROUTE_TO_GO=/ {
        print "ROUTE_TO_GO=" value
        updated=1
        next
}
{ print }
END {
        if (!updated) {
                print "ROUTE_TO_GO=" value
        }
}
' "$ENV_FILE" > "$tmp_file"

mv "$tmp_file" "$ENV_FILE"
trap - EXIT
rm -f "$tmp_file"

echo "ROUTE_TO_GO set to $VALUE"
