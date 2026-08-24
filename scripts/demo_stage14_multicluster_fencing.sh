#!/usr/bin/env bash
# ==============================================================================
# SentinelMesh Stage 14 Demo: Multi-Cluster Federation & Distributed Fencing
# ==============================================================================
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

mkdir -p ./bin
echo "Building Stage 14 multi-cluster demo binary..."
go build -o ./bin/demo-stage14 ./cmd/demo-stage14

echo "Running Stage 14 Multi-Cluster Federation & Distributed Fencing Demonstration..."
./bin/demo-stage14
