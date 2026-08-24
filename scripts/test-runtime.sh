#!/bin/bash
set -e

echo "================================================================="
echo " SentinelMesh Stage 9: Agent Runtime & Lifecycle Demo           "
echo "================================================================="

echo "[1/6] Building SentinelMesh binaries & sample agent workload..."
make build
go build -o bin/task-processor ./examples/agents/task_processor

echo "[2/6] Executing Agent Workload via Process Runtime..."
echo "--- Launching TaskProcessor (Normal Execution) ---"
./bin/runtime -run-id="run-demo-normal" -cmd="./bin/task-processor" -timeout=10

echo ""
echo "[3/6] Executing Agent Workload with Failure Simulation..."
echo "--- Launching TaskProcessor (Simulated Failure) ---"
SIMULATE_FAILURE="true" ./bin/runtime -run-id="run-demo-failure" -cmd="./bin/task-processor" -timeout=10 || true

echo ""
echo "[4/6] Executing Agent Workload with Watchdog Timeout Enforcement..."
echo "--- Launching TaskProcessor with 1s timeout (Long workload) ---"
WORKLOAD_STEPS="20" ./bin/runtime -run-id="run-demo-timeout" -cmd="./bin/task-processor" -timeout=1 || true

echo ""
echo "[5/6] Verifying Runtime Test Suite..."
go test -v ./internal/runtime/...

echo ""
echo "[6/6] Verifying Entire System Race-Condition Free..."
go test -race ./...

echo "================================================================="
echo " SUCCESS: Stage 9 Agent Runtime verified and complete!           "
echo "================================================================="
