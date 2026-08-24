#!/usr/bin/env bash
set -euo pipefail

# ==============================================================================
# SentinelMesh Stage 20 Grand Finale Demonstration
# Dual-Path Showcase: Path A (Hero Routing) & Path B (Deep Distributed OS)
# ==============================================================================

echo "================================================================================"
echo "          BUILDING & RUNNING SENTINELMESH STAGE 20 DEMONSTRATION                "
echo "================================================================================"

echo ""
echo ">>> [PATH A]: 3-Minute Hero Adaptive Routing & Live Provider Control Plane"
go run ./cmd/demo-stage20/main.go --mode=hero

echo ""
echo ">>> [PATH B]: Deep Technical Distributed Control Plane, Fencing & Attestation"
go run ./cmd/demo-stage20/main.go --mode=deep
