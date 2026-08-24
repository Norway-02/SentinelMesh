# ==============================================================================
# SentinelMesh Build, Test, Benchmark & Reproduction Makefile
# ==============================================================================

.PHONY: all build test test-property test-security test-chaos bench bench-scheduler bench-routing demo demo-deep demo-stage17 demo-stage18 demo-stage19 demo-stage20 vet clean web-install web-build web-test web-dev dev live traffic

all: vet test build web-build

build:
	@echo "==> Building SentinelMesh binaries..."
	go build ./cmd/...
	go build ./internal/...

test:
	@echo "==> Running full unit and integration test suite..."
	go test -v ./internal/... ./test/...

test-property:
	@echo "==> Running 10,000-iteration property invariance tests..."
	go test -v -run=TestPolicyNeverViolatesStage17Constraints ./test/onlinepolicy/...
	go test -v -run=TestAdaptiveRouter_NeverViolatesStage17Constraints ./test/adaptive/...
	go test -v -run=TestRouter_HardConstraints ./test/router/...

test-security:
	@echo "==> Running security and attack containment suite..."
	go test -v ./test/security/...

test-chaos:
	@echo "==> Running chaos fault injection matrix..."
	go test -v ./test/chaos/...

bench:
	@echo "==> Running all microbenchmarks and trace evaluations..."
	go test -run=^$ -bench=. -benchmem ./benchmark/...

bench-scheduler:
	@echo "==> Running 1,000,000-node scheduler benchmark..."
	go test -v -run=TestScheduler_1MillionNodes_Benchmark ./benchmark/scheduler/...

bench-routing:
	@echo "==> Running Stage 17, 18, and 19 1,000-task comparison experiment..."
	go test -v -run=TestOnlinePolicy_TraceMatchedComparisonExperiment_1000Tasks ./benchmark/onlinepolicy/...

demo:
	@echo "==> Running SentinelMesh Stage 20 Hero Adaptive Routing Demo..."
	go run ./cmd/demo-stage20/main.go --mode=hero

demo-deep:
	@echo "==> Running SentinelMesh Stage 20 Deep Distributed Control Plane Demo..."
	go run ./cmd/demo-stage20/main.go --mode=deep

demo-stage17:
	@echo "==> Running Stage 17 Deterministic Router Demo..."
	./scripts/demo_stage17_model_router.sh

demo-stage18:
	@echo "==> Running Stage 18 Predictive Adaptive Scheduler Demo..."
	./scripts/demo_stage18_adaptive_scheduler.sh

demo-stage19:
	@echo "==> Running Stage 19 Safe Online Policy Learning Demo..."
	./scripts/demo_stage19_policy_learning.sh

demo-stage20:
	@echo "==> Running Stage 20 Grand Finale Production Release Demo..."
	./scripts/demo_stage20_production_release.sh

web-install:
	@echo "==> Installing frontend dependencies..."
	cd web && npm install

web-build:
	@echo "==> Building frontend web application..."
	cd web && npm run build

web-test:
	@echo "==> Running frontend tests..."
	cd web && npm test

web-dev:
	@echo "==> Starting frontend dev server..."
	cd web && npm run dev

dev:
	@echo "==> Cleaning up any existing processes on ports 8787, 8900, 9090..."
	-fuser -k 8787/tcp 8900/tcp 9090/tcp >/dev/null 2>&1 || true
	@echo "==> Starting SentinelMesh API server (127.0.0.1:8787)..."
	SENTINEL_HTTP_ADDR=127.0.0.1:8787 go run ./cmd/api-server/main.go &
	@echo "==> Starting SentinelMesh Control Plane GUI (127.0.0.1:8900)..."
	cd web && npm run dev

traffic:
	@echo "==> Starting SentinelMesh Live Traffic Generator..."
	go run ./cmd/traffic-generator/main.go --interval=1500ms

live:
	@echo "==> Cleaning up any existing processes on ports 8787, 8900, 9090..."
	-fuser -k 8787/tcp 8900/tcp 9090/tcp >/dev/null 2>&1 || true
	@echo "==> Starting SentinelMesh API server (127.0.0.1:8787)..."
	SENTINEL_HTTP_ADDR=127.0.0.1:8787 go run ./cmd/api-server/main.go &
	@echo "==> Waiting for API server initialization..."
	@sleep 2
	@echo "==> Starting Live Traffic Simulation Generator..."
	go run ./cmd/traffic-generator/main.go --interval=1500ms &
	@echo "==> Starting SentinelMesh Control Plane GUI (127.0.0.1:8900)..."
	cd web && npm run dev

vet:
	@echo "==> Running static analysis (go vet)..."
	go vet ./...

clean:
	@echo "==> Cleaning build artifacts..."
	go clean
	rm -rf web/dist
