#!/usr/bin/env bash
# Downloads the raw Northflank swagger-json and normalizes it into a clean
# OpenAPI 3.0.3 document suitable for oapi-codegen.
#
# Usage: bash openapi/fetch.sh
# Output: openapi/northflank-openapi.json (committed to the repo)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RAW="${SCRIPT_DIR}/northflank-raw.json"
OUT="${SCRIPT_DIR}/northflank-openapi.json"

echo "Fetching Northflank swagger spec..."
curl -sSf -o "$RAW" https://api.northflank.com/v1/swagger-json
echo "Downloaded $(wc -c <"$RAW" | tr -d ' ') bytes to $RAW"

echo "Normalizing..."
python3 "${SCRIPT_DIR}/normalize.py" "$RAW" "$OUT"
echo "Done. Normalized spec written to $OUT"
