#!/usr/bin/env bash
set -e

echo "Starting Scheduler Verification (Stage 7)..."

export SENTINEL_ENVIRONMENT="development"
export SENTINEL_HTTP_ADDR=":8081"
export SENTINEL_GRPC_ADDR=":9091"
export SENTINEL_DB_URL="postgres://postgres:postgres@localhost:5433/postgres?sslmode=disable"
export SENTINEL_NATS_URL="nats://localhost:4222"

# 1. Start Postgres and NATS in Docker
docker rm -f sentinel-db sentinel-nats 2>/dev/null || true
docker run -d --name sentinel-db -e POSTGRES_PASSWORD=postgres -p 5433:5432 postgres:15-alpine
docker run -d --name sentinel-nats -p 4222:4222 -p 8222:8222 nats:latest -js

echo "Waiting for infrastructure..."
sleep 5

# 2. Run migrations
echo "Running migrations..."
make migrate-up

# 3. Start API Server
echo "Starting API server..."
make build
./bin/api-server &
API_PID=$!
sleep 2

# 4. Start Scheduler
echo "Starting Scheduler..."
./bin/scheduler > scheduler.log 2>&1 &
SCHED_PID=$!
sleep 2

# 5. Create Agent
echo "Creating Agent..."
AGENT_ID=$(curl -s -X POST http://localhost:8081/v1/agents \
  -H "Content-Type: application/json" \
  -d '{
    "name": "scheduler-agent",
    "tenant_id": "t1",
    "version": "1.0",
    "priority": "critical",
    "resources": {"cpu": "2", "memory": "4Gi"}
  }' | grep -io '"id":"[^"]*"' | cut -d'"' -f4)

echo "Agent created: $AGENT_ID"

# 6. Create Run
echo "Creating Run..."
RUN_ID=$(curl -s -X POST "http://localhost:8081/v1/agents/$AGENT_ID/runs" \
  -H "Content-Type: application/json" \
  -d '{}' | grep -io '"id":"[^"]*"' | cut -d'"' -f4)

echo "Run created: $RUN_ID"
echo "Waiting for scheduling (3s)..."
sleep 3

# 7. Verify Logs
echo "Scheduler Output:"
cat scheduler.log

if ! grep -q "Selected node for run" scheduler.log; then
    echo "ERROR: Scheduler did not select a node!"
    kill $API_PID $SCHED_PID
    exit 1
fi

echo "Verified: Run was scheduled successfully."

# 8. Check Database for Assignment Explanation
echo "Assignment stored in Database:"
docker exec sentinel-db psql -U postgres -d postgres -tAc "SELECT decision FROM run_scheduling_assignments WHERE run_id = '$RUN_ID'" | jq .

echo "Cleaning up..."
kill $API_PID $SCHED_PID
docker rm -f sentinel-db sentinel-nats

echo "Scheduler Verification SUCCESSFUL!"
