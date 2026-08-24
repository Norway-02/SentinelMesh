#!/usr/bin/env bash
set -euo pipefail

# ==============================================================================
# SentinelMesh Stage 19 Hero Demonstration
# Safe Online Policy Learning & Exploration (Contextual UCB Bandit + Rollback)
# ==============================================================================

echo "================================================================================"
echo "          BUILDING & RUNNING SENTINELMESH STAGE 19 HERO DEMONSTRATION           "
echo "================================================================================"

go run ./cmd/demo-stage19/main.go
