#!/bin/bash
# test model routing strategies

set -e

OLLA_URL=${OLLA_URL:-"http://localhost:8080"}
MODEL=${MODEL:-"phi3.5:latest"}
# ALIAS_MODEL must be a `model_aliases` key in the running Olla's config whose
# listed real model names exist on none of the configured endpoints - see #191.
ALIAS_MODEL=${ALIAS_MODEL:-"nonexistent-alias-target"}

# FAILED_TESTS follows the same pass/fail convention as the other scripts in
# test/scripts/logic (test-model-routing.sh, test-provider-models.sh,
# test-provider-routing.sh): increment on a real assertion failure, exit 1 at
# the end if non-zero. Most tests in this script are informational only
# (echo ✓/✗ without affecting the exit code), but Test 4 asserts a concrete
# regression (#191) and must fail the script when it doesn't hold.
FAILED_TESTS=0

echo "Testing Model Routing Strategies"
echo "================================"
echo ""

# function to make a request and check headers
test_request() {
    local description="$1"
    local model="$2"
    local expected_strategy="$3"
    
    echo "Test: $description"
    echo "Model: $model"
    echo "Expected Strategy: $expected_strategy"
    
    response=$(curl -s -i -X POST "$OLLA_URL/olla/api/generate" \
        -H "Content-Type: application/json" \
        -d "{\"model\": \"$model\", \"prompt\": \"test\", \"stream\": false}" \
        2>&1 || true)
    
    # extract headers
    strategy_header=$(echo "$response" | grep -i "X-Olla-Routing-Strategy:" | head -n 1 || echo "")
    decision_header=$(echo "$response" | grep -i "X-Olla-Routing-Decision:" | head -n 1 || echo "")
    reason_header=$(echo "$response" | grep -i "X-Olla-Routing-Reason:" | head -n 1 || echo "")
    status_code=$(echo "$response" | head -n 1 | awk '{print $2}')
    
    echo "Response Status: $status_code"
    echo "Routing Strategy: $strategy_header"
    echo "Routing Decision: $decision_header"
    echo "Routing Reason: $reason_header"
