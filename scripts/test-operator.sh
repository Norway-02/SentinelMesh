#!/bin/bash
set -e

echo "================================================================="
echo " SentinelMesh Stage 8: Kubernetes Operator & Placement Demo      "
echo "================================================================="

# Step 1: Ensure kind cluster exists
if ! kind get clusters 2>/dev/null | grep -q "^sentinelmesh$"; then
    echo "[1/17] Creating kind cluster 'sentinelmesh'..."
    kind create cluster --name sentinelmesh
else
    echo "[1/17] Kind cluster 'sentinelmesh' already exists."
fi

export KUBECONFIG="$(kind get kubeconfig-path --name="sentinelmesh" 2>/dev/null || echo ~/.kube/config)"
export SENTINEL_KUBECONFIG="$KUBECONFIG"

# Step 2: Deploy SentinelMesh namespace & CRD & RBAC
echo "[2/17] Deploying dedicated 'sentinelmesh' namespace, CRD, and RBAC..."
kubectl apply -f deploy/manifests/namespace.yaml
kubectl apply -f deploy/manifests/agentrun_crd.yaml
kubectl apply -f deploy/manifests/rbac/

# Step 3 & 4: Start PostgreSQL & NATS
echo "[3/17] Starting PostgreSQL container..."
docker start sentinel-db 2>/dev/null || docker run -d --name sentinel-db -e POSTGRES_PASSWORD=postgres -p 5433:5432 postgres:15-alpine

echo "[4/17] Starting NATS JetStream container..."
docker start sentinel-nats 2>/dev/null || docker run -d --name sentinel-nats -p 4222:4222 -p 8222:8222 nats:alpine -js

# Wait for DB to be ready
until docker exec sentinel-db pg_isready -U postgres >/dev/null 2>&1; do
  sleep 1
done

# Wait for NATS
sleep 3

export SENTINEL_DB_URL="postgres://postgres:postgres@localhost:5433/postgres?sslmode=disable"
export SENTINEL_NATS_URL="nats://localhost:4222"
export SENTINEL_HTTP_ADDR=":8081"
export SENTINEL_GRPC_ADDR=":9091"
export SENTINEL_ENVIRONMENT="development"

# Run migrations
echo "Running database schema migrations..."
go run -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate -path db/migrations -database "$SENTINEL_DB_URL" up

# Build binaries
echo "Building binaries..."
make build

# Step 5: Start API Server
echo "[5/17] Starting SentinelMesh API server on :8081..."
./bin/api-server > /tmp/api-server.log 2>&1 &
API_PID=$!

# Step 6: Start Scheduler with real Kubernetes node provider
echo "[6/17] Starting SentinelMesh Scheduler with KubernetesResourceProvider..."
./bin/scheduler > /tmp/scheduler.log 2>&1 &
SCHEDULER_PID=$!

# Step 7: Start Operator
echo "[7/17] Starting SentinelMesh Kubernetes Operator..."
./bin/operator --health-probe-bind-address=:8082 > /tmp/operator.log 2>&1 &
OPERATOR_PID=$!

cleanup() {
    echo "Cleaning up processes..."
    kill $API_PID 2>/dev/null || true
    kill $SCHEDULER_PID 2>/dev/null || true
    kill $OPERATOR_PID 2>/dev/null || true
}
trap cleanup EXIT

echo "Waiting for services to initialize..."
sleep 4

# Discover cluster nodes
K8S_NODE=$(kubectl get nodes -o jsonpath='{.items[0].metadata.name}')
echo "Discovered Kubernetes Node: $K8S_NODE"

# Step 8: POST Agent
echo "[8/17] Registering Agent via API..."
AGENT_RES=$(curl -s -X POST http://localhost:8081/v1/agents -H "Content-Type: application/json" -d '{
  "tenant_id": "tenant-ops",
  "name": "data-processor",
  "version": "1.0",
  "priority": "critical",
  "status": "idle",
  "labels": {"env": "prod"},
  "image": "alpine:latest",
  "security_policy": {
    "profile": "standard"
  },
  "resources": {
    "cpu": "100m",
    "memory": "128Mi",
    "gpu_count": 0
  }
}')

AGENT_ID=$(echo $AGENT_RES | grep -io '"id":"[^"]*"' | head -n1 | cut -d'"' -f4)
if [ -z "$AGENT_ID" ]; then
  echo "Failed to create agent: $AGENT_RES"
  exit 1
fi
echo "Agent registered successfully: $AGENT_ID"

# Step 9: POST Run
echo "[9/17] Triggering Run via API..."
RUN_RES=$(curl -s -X POST "http://localhost:8081/v1/agents/$AGENT_ID/runs" -H "Content-Type: application/json" -d '{
  "priority": "high",
  "security_class": "standard",
  "labels": {"type": "analytics"}
}')

RUN_ID=$(echo $RUN_RES | grep -io '"id":"[^"]*"' | head -n1 | cut -d'"' -f4)
if [ -z "$RUN_ID" ]; then
  echo "Failed to create run: $RUN_RES"
  exit 1
fi
echo "Run initiated: $RUN_ID"

echo "[10/17] RunCreated published via Outbox to NATS JetStream."
echo "[11/17] Scheduler reads real Kubernetes node capacity/allocatable from K8s API..."
echo "[12/17] Scheduler evaluates candidates and assigns node: $K8S_NODE"
echo "[13/17] RunScheduled event published to NATS JetStream."

echo "[14/17] Waiting for Operator event consumer & reconciler loop..."
sleep 5

echo "[15/17] Inspecting AgentRun CR in 'sentinelmesh' namespace:"
kubectl get agentruns.sentinelmesh.io -n sentinelmesh -o wide

echo "[16/17] Inspecting Pod in 'sentinelmesh' namespace:"
kubectl get pods -n sentinelmesh -o wide

POD_NAME="agentrun-$RUN_ID"
POD_NODE=$(kubectl get pod "$POD_NAME" -n sentinelmesh -o jsonpath='{.spec.nodeName}' 2>/dev/null || echo "missing")

echo "[17/17] Verifying hard node pinning and status synchronization..."
if [ "$POD_NODE" == "$K8S_NODE" ]; then
    echo "================================================================="
    echo " SUCCESS: SentinelMesh Scheduler placed workload on '$K8S_NODE'"
    echo "          Kubernetes Pod '$POD_NAME' is pinned to '$POD_NODE'"
    echo "================================================================="
else
    echo "FAILED: Pod node is '$POD_NODE', expected '$K8S_NODE'."
    kubectl describe pod "$POD_NAME" -n sentinelmesh || true
    echo "Operator Logs:"
    cat /tmp/operator.log
    exit 1
fi

CR_PHASE=$(kubectl get agentrun "$POD_NAME" -n sentinelmesh -o jsonpath='{.status.phase}' 2>/dev/null || echo "Unknown")
echo "AgentRun CR Phase: $CR_PHASE"
echo "Stage 8 Hero Demo Complete!"
