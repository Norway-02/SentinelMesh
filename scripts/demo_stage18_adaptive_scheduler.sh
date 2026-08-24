#!/usr/bin/env bash
set -euo pipefail

# ==============================================================================
# SentinelMesh Stage 18 Hero Demonstration
# Predictive & Uncertainty-Aware Adaptive Routing under Dynamic Provider Drift
# ==============================================================================

echo "================================================================================"
echo "          BUILDING & RUNNING SENTINELMESH STAGE 18 HERO DEMONSTRATION           "
echo "================================================================================"

go run ./cmd/demo-stage18/main.go
