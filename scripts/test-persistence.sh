#!/bin/bash
set -e

echo "Starting persistence verification..."

# 1. Setup Postgres
docker rm -f sentinel-db || true
docker run --name sentinel-db -e POSTGRES_PASSWORD=postgres -p 5433:5432 -d postgres:15-alpine
sleep 3 # Wait for postgres to boot

export SENTINEL_DB_URL="postgres://postgres:postgres@localhost:5433/postgres?sslmode=disable"
export SENTINEL_HTTP_ADDR=":8081"
export SENTINEL_GRPC_ADDR=":9091"

# 2. Run migrations
echo "Running migrations..."
make migrate-up

# 3. Start API server in background
echo "Starting API server..."
make build
./bin/api-server &
API_PID=$!
sleep 2 # Wait for API server to boot

# 4. Create an agent
echo "Creating agent..."
CREATE_RES=$(curl -s -X POST http://localhost:8081/v1/agents \
  -H "Content-Type: application/json" \
  -d '{"name":"persistent-agent-1", "version":"1.0", "tenant_id":"t1", "priority":"normal", "resources": {"cpu": "2", "memory": "4Gi", "gpu": 0}}')
echo $CREATE_RES
AGENT_ID=$(echo $CREATE_RES | grep -io '"id":"[^"]*' | cut -d'"' -f4)
echo "Agent ID: $AGENT_ID"

# 5. Create a run
echo "Creating run..."
RUN_RES=$(curl -s -X POST http://localhost:8081/v1/agents/$AGENT_ID/runs \
  -H "Content-Type: application/json")
echo $RUN_RES
RUN_ID=$(echo $RUN_RES | grep -io '"id":"[^"]*' | cut -d'"' -f4)
echo "Run ID: $RUN_ID"

# 6. Stop API server
echo "Stopping API server (PID: $API_PID)..."
kill $API_PID
wait $API_PID || true

# 7. Start API server again
echo "Restarting API server..."
./bin/api-server &
API_PID2=$!
sleep 2

# 8. Get the agent and verify it exists
echo "Fetching agent after restart..."
GET_AGENT=$(curl -s http://localhost:8081/v1/agents/$AGENT_ID)
echo $GET_AGENT
if [[ $GET_AGENT != *"persistent-agent-1"* ]]; then
  echo "ERROR: Agent not found or incorrect after restart!"
  kill $API_PID2
  exit 1
fi

# 9. Get the run and verify it exists
echo "Fetching run after restart..."
GET_RUN=$(curl -s http://localhost:8081/v1/runs/$RUN_ID)
echo $GET_RUN
if [[ $GET_RUN != *"$AGENT_ID"* ]]; then
  echo "ERROR: Run not found or incorrect after restart!"
  kill $API_PID2
  exit 1
fi

echo "Persistence verification SUCCESSFUL!"

kill $API_PID2
docker rm -f sentinel-db
