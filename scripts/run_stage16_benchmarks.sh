#!/usr/bin/env bash
set -euo pipefail

# SentinelMesh Stage 16: Automated Benchmark & Performance Suite Runner
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

cd "${ROOT_DIR}"

mkdir -p "${ROOT_DIR}/benchmark/results"
mkdir -p "${ROOT_DIR}/bin"

echo "================================================================================"
echo "          BUILDING SENTINELMESH STAGE 16 BENCHMARK CLI & SUITES                 "
echo "================================================================================"
go build -o "${ROOT_DIR}/bin/sentinel-bench" "${ROOT_DIR}/cmd/benchmark"/*.go

echo ""
echo "================================================================================"
echo "                 EXECUTING SYSTEM-WIDE BENCHMARK SUITE                          "
echo "================================================================================"
"${ROOT_DIR}/bin/sentinel-bench" \
  --cpuprofile="${ROOT_DIR}/benchmark/results/cpu.pprof" \
  --memprofile="${ROOT_DIR}/benchmark/results/mem.pprof"

echo ""
echo "================================================================================"
echo "                 RUNNING GO NATIVE BENCHMARK WITH ALLOCS                        "
echo "================================================================================"
go test -run=^$ -bench=. -benchmem -count=3 ./benchmark/... | tee "${ROOT_DIR}/benchmark/results/go_benchmarks.txt"

echo ""
echo "Stage 16 Benchmarking Successfully Completed! Artifacts in benchmark/results/"
