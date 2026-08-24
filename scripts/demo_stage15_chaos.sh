#!/usr/bin/env bash
set -euo pipefail

# SentinelMesh Stage 15: Failure Injection & Chaos Experiment Demo
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

cd "${ROOT_DIR}"

mkdir -p "${ROOT_DIR}/bin"
echo "Building SentinelMesh Stage 15 Chaos Demo..."
go build -o "${ROOT_DIR}/bin/demo-stage15" "${ROOT_DIR}/cmd/demo-stage15/main.go"

echo "Executing Stage 15 Failure Injection & Chaos Validation Matrix..."
"${ROOT_DIR}/bin/demo-stage15"
