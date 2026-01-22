#!/bin/bash

BASE_URL="http://localhost:9000"
CLUSTER="integration-test"

echo "=== Simulator Integration Test ==="

# Create cluster
echo -e "\n1. Creating cluster..."
curl -s -X POST "$BASE_URL/clusters/$CLUSTER" \
  -H "Content-Type: application/json" \
  -d '{"servers": 4, "base_cpu": 50}' | jq .

# Get initial metrics
echo -e "\n2. Initial metrics..."
curl -s "$BASE_URL/metrics/$CLUSTER" | jq '.servers | length, .[0].cpu_usage'

# Inject spike
echo -e "\n3. Injecting spike (95% CPU for 1 minute)..."
curl -s -X POST "$BASE_URL/spike" \
  -H "Content-Type: application/json" \
  -d "{\"cluster_id\": \"$CLUSTER\", \"cpu_target\": 95, \"duration\": \"1m\", \"ramp_up\": \"10s\"}" | jq . 

# Monitor spike
echo -e "\n4. Monitoring spike (every 5 seconds for 30 seconds)..."
for i in {1..6}; do
  CPU=$(curl -s "$BASE_URL/metrics/$CLUSTER" | jq '.servers[0].cpu_usage')
  echo "  t+$((i*5))s: CPU = $CPU%"
  sleep 5
done

# Set pattern
echo -e "\n5. Setting daily pattern..."
curl -s -X POST "$BASE_URL/pattern" \
  -H "Content-Type: application/json" \
  -d "{\"cluster_id\": \"$CLUSTER\", \"pattern\": \"daily\"}" | jq .

# Check status
echo -e "\n6. Final cluster status..."
curl -s "$BASE_URL/clusters/$CLUSTER" | jq .

echo -e "\n=== Test Complete ==="