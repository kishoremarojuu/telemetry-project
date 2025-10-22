#!/bin/bash

# GPU Telemetry API Test Script
# This script tests all API endpoints and displays results

API_BASE="http://localhost:8080"

echo "======================================"
echo "GPU Telemetry Pipeline - API Tests"
echo "======================================"
echo ""

# Color codes
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Function to make API call and display results
test_endpoint() {
    local method=$1
    local endpoint=$2
    local description=$3
    local data=$4

    echo -e "${BLUE}Testing: ${description}${NC}"
    echo -e "${YELLOW}${method} ${endpoint}${NC}"

    if [ "$method" == "GET" ]; then
        response=$(curl -s -w "\n%{http_code}" "${API_BASE}${endpoint}")
    else
        response=$(curl -s -w "\n%{http_code}" -X "${method}" "${API_BASE}${endpoint}" -H "Content-Type: application/json" -d "${data}")
    fi

    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | sed '$d')

    if [ "$http_code" -eq 200 ] || [ "$http_code" -eq 201 ]; then
        echo -e "${GREEN}✓ Success (HTTP ${http_code})${NC}"
        echo "$body" | jq '.' 2>/dev/null || echo "$body"
    else
        echo -e "${RED}✗ Failed (HTTP ${http_code})${NC}"
        echo "$body"
    fi

    echo ""
    echo "--------------------------------------"
    echo ""
}

# Check if API is running
echo "Checking if API server is running..."
if ! curl -s "${API_BASE}/health" > /dev/null; then
    echo -e "${RED}Error: API server is not running on ${API_BASE}${NC}"
    echo "Please start the API server first:"
    echo "  cd cmd/api-server && go run api_server.go"
    exit 1
fi

echo -e "${GREEN}✓ API server is running${NC}"
echo ""
echo "======================================"
echo ""

# Test 1: Health Check
test_endpoint "GET" "/health" "Health Check"

# Test 2: Get All Nodes
test_endpoint "GET" "/api/v1/nodes" "Get All GPU Nodes"

# Test 3: Get Specific Node
test_endpoint "GET" "/api/v1/nodes/node-1" "Get Node-1 Health Status"

# Test 4: Get Node Metrics
test_endpoint "GET" "/api/v1/nodes/node-1/metrics?limit=5" "Get Last 5 Metrics for Node-1"

# Test 5: Get Latest Metrics from All GPUs
test_endpoint "GET" "/api/v1/metrics/latest" "Get Latest Metrics (All GPUs)"

# Test 6: Get All Alerts
test_endpoint "GET" "/api/v1/alerts" "Get All Alerts (Last 100)"

# Test 7: Get Active Alerts Only
test_endpoint "GET" "/api/v1/alerts/active" "Get Active Alerts Only"

# Test 8: Resolve an Alert (if any exist)
echo "Checking for active alerts to resolve..."
active_alerts=$(curl -s "${API_BASE}/api/v1/alerts/active")
alert_id=$(echo "$active_alerts" | jq -r '.[0].id' 2>/dev/null)

if [ "$alert_id" != "null" ] && [ -n "$alert_id" ]; then
    test_endpoint "POST" "/api/v1/alerts/${alert_id}/resolve" "Resolve Alert ID ${alert_id}"
else
    echo -e "${YELLOW}No active alerts to resolve${NC}"
    echo ""
    echo "--------------------------------------"
    echo ""
fi

echo "======================================"
echo "Summary of Available Endpoints:"
echo "======================================"
echo ""
echo "Node Management:"
echo "  GET  /api/v1/nodes"
echo "  GET  /api/v1/nodes/{node_