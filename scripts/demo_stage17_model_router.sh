#!/usr/bin/env bash
set -euo pipefail

# SentinelMesh Stage 17: Model Router & Adaptive Intelligence Hero Demo Runner
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

cd "${ROOT_DIR}"

mkdir -p "${ROOT_DIR}/bin"

echo "================================================================================"
echo "          BUILDING SENTINELMESH STAGE 17 MODEL ROUTER DEMO                      "
echo "================================================================================"
go build -o "${ROOT_DIR}/bin/sentinel-router-demo" "${ROOT_DIR}/cmd/demo-stage17/main.go"

echo ""
"${ROOT_DIR}/bin/sentinel-router-demo"
