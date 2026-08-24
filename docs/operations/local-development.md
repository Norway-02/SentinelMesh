# SentinelMesh Local Development Guide

## Prerequisites
- **Go 1.23+**
- **Docker & Docker Compose**
- **kind** (Kubernetes IN Docker)
- **kubectl**
- **make**
- **protoc** (for rebuilding gRPC stubs)
- **golangci-lint** (for linting)

## Setup

1. **Clone the repository:**
   ```bash
   git clone https://github.com/sentinelmesh/sentinelmesh.git
   cd sentinelmesh
   ```

2. **Run tests and build:**
   ```bash
   make test
   make build
   make lint
   ```

3. **Bootstrap Local Infrastructure:**
   We use `docker compose` for data stores and `kind` for the Kubernetes execution plane.
   ```bash
   make dev
   make cluster-up
   ```

## Development Workflow

- The **Control Plane** can run as standard Go processes natively on your machine during development (`go run ./cmd/api-server`).
- The **Data Plane** dependencies (Kubernetes, Postgres, NATS, Redis) run in Docker/kind.
- Run `make test` before pushing any code. Ensure there are no regressions.
- For e2e tests, we deploy the SentinelMesh operator into the local `kind` cluster.

## Teardown
```bash
docker compose down -v
kind delete cluster --name sentinelmesh
```
