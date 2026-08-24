#!/usr/bin/env bash
set -e

echo "Starting Outbox/NATS Verification..."

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

# 4. Start NATS Consumer
echo "Starting Test Consumer..."
go run scripts/test-outbox-consumer.go > consumer.log 2>&1 &
CONS_PID=$!
sleep 2

# 5. Test 1: Happy Path
echo "Creating Agent 1 (NATS is UP)..."
AGENT1_ID=$(curl -s -X POST http://localhost:8081/v1/agents \
  -H "Content-Type: application/json" \
  -d '{
    "name": "outbox-agent-1",
    "tenant_id": "t1",
    "version": "1.0",
    "priority": "normal",
    "resources": {"cpu": "500m", "memory": "512Mi"}
  }' | grep -io '"id":"[^"]*"' | cut -d'"' -f4)

echo "Agent 1 created: $AGENT1_ID"
sleep 2

echo "Consumer Output so far:"
cat consumer.log

if ! grep -q "sentinel.agent.v1.created" consumer.log; then
    echo "ERROR: Event 1 not received!"
    kill $API_PID $CONS_PID
    exit 1
fi

# 6. Test 2: NATS Down (Resilience)
echo "Stopping NATS..."
docker stop sentinel-nats
sleep 2

echo "Creating Agent 2 (NATS is DOWN)..."
AGENT2_ID=$(curl -s -X POST http://localhost:8081/v1/agents \
  -H "Content-Type: application/json" \
  -d '{
    "name": "outbox-agent-2",
    "tenant_id": "t1",
    "version": "1.0",
    "priority": "normal",
    "resources": {"cpu": "500m", "memory": "512Mi"}
  }' | grep -io '"id":"[^"]*"' | cut -d'"' -f4)

echo "Agent 2 created: $AGENT2_ID (API must succeed even if NATS is down)"

# Verify outbox has the pending event
echo "Checking outbox pending events..."
PENDING=$(docker exec sentinel-db psql -U postgres -d postgres -tAc "SELECT count(*) FROM outbox_events WHERE aggregate_id = '$AGENT2_ID' AND published_at IS NULL")
if [ "$PENDING" -ne 1 ]; then
    echo "ERROR: Event for Agent 2 not in outbox!"
    kill $API_PID $CONS_PID
    exit 1
fi
echo "Verified: Event is securely pending in Postgres outbox."

# 7. Test 3: NATS Recovery
echo "Restarting NATS..."
docker start sentinel-nats
sleep 5 # wait for publisher to poll and publish

echo "Consumer Output after recovery:"
cat consumer.log

if ! grep -q "MsgID.*$AGENT2_ID" consumer.log; then # Assuming aggregate ID might be part of payload, but better search for outbox-agent-2
    if ! grep -q "outbox-agent-2" consumer.log; then
        echo "ERROR: Event 2 not recovered and received!"
        kill $API_PID $CONS_PID
        exit 1
    fi
fi

echo "Verified: Event 2 delivered upon NATS recovery!"

echo "Cleaning up..."
kill $API_PID $CONS_PID
docker rm -f sentinel-db sentinel-nats

echo "Outbox Verification SUCCESSFUL!"
