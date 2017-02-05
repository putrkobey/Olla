#!/usr/bin/env bash

# Test script for Olla request size limits
# [o] AI Enhanced: Claude[AI] 3.5, 3.7, Github Copilot
#
# Run this after starting Olla with custom size limits configured

# 30-05-2025  [TF]  - Moved to test/scripts/security, added more tests, configurable model, ui tweaks
# 21-05-2025  [ML]  - Shows when tests fail at the end (test_tracking), added more context to output
# 20-05-2025  [GW]  - Added large headers tests, improved output formatting
# 15-05-2025  [GW]  - Fix for large payloads not being created correctly
# 12-05-2025  [GW]  - Initial version created with Claude[AI] 3.5

# Ensure required environment variables are set:
# export OLLA_SERVER_MAX_BODY_SIZE="1KB"
# export OLLA_SERVER_MAX_HEADER_SIZE="500B"
##

OLLA_URL="http://localhost:40114"
PROVIDER="${PROVIDER:-ollama}"
PROXY_ENDPOINT="/olla/${PROVIDER}/api/generate"
HEALTH_ENDPOINT="/internal/health"
MODEL_NAME="phi4:latest"
# NOTE about model choice, choose a lighter model for testing request limits so it returns quickly.
# Ideally we find Phi-3/Phi-4 or Llama-3 are light enough for this purpose.

echo "------- SECURITY TEST SUITE -------"
echo "🦙 Testing Olla Request Size Limits"
echo "                            v1.1.12"
echo "-----------------------------------"
echo "Olla URL:         $OLLA_URL"
echo "Proxy Endpoint:   $PROXY_ENDPOINT"
echo "Health Endpoint:  $HEALTH_ENDPOINT"
echo "Model:            $MODEL_NAME"
echo "-----------------------------------"

TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=()

record_test_result() {
    local test_name="$1"
    local passed="$2"

    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    if [[ "$passed" == "true" ]]; then
        PASSED_TESTS=$((PASSED_TESTS + 1))
    else
        FAILED_TESTS+=("$test_name")
    fi
}

test_request() {
    local description="$1"
    local endpoint="$2"
    local data="$3"
    local expected_statuses="$4"  # Can be multiple like "200|502|413"
