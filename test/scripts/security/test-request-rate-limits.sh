#!/usr/bin/env bash

# Olla Request Rate Limit Security Test Suite
# [o] AI Enhanced: Claude[AI] 3.7, Github Copilot
#
# Run this after starting Olla with rate limiting configured

# 31-05-2025  [TF]  - Updated for golang.org/x/time/rate implementation,
#                     more aggressive testing, header detection fixes
#                     adds expectation to output for better CLI feedback (thanks to Claude[AI] + GW[VERY SMART HUMAN])
# 30-05-2025  [TF]  - Fixed test logic, improved burst handling, better timing
# 21-05-2025  [ML]  - Added more context to CLI output, shows when tests fail at the end
# 15-05-2025  [GW]  - Burst capacity test added and fix sleep timing for other tests
# 14-05-2025  [GW]  - Initial version built with Claude[AI] and Copilot

OLLA_URL="http://localhost:40114"
PROVIDER="${PROVIDER:-ollama}"
PROXY_ENDPOINT="/olla/${PROVIDER}/api/generate"
HEALTH_ENDPOINT="/internal/health"
MODEL_NAME="phi4:latest"
# NOTE about model choice, choose a lighter model for testing request limits so it returns quickly.
# Ideally we find Phi-3/Phi-4 or Llama-3 are light enough for this purpose.

export OLLA_SERVER_PER_IP_RATE_LIMIT="20"     # 20/minute for more aggressive testing
export OLLA_SERVER_RATE_BURST_SIZE="3"        # Small burst for clear testing
export OLLA_SERVER_HEALTH_RATE_LIMIT="60"     # 60/minute = 1 per second
export OLLA_SERVER_GLOBAL_RATE_LIMIT="0"      # Disable global limiting for cleaner tests

echo "------- SECURITY TEST SUITE -------"
echo "🦙 Testing Olla Request Rate Limit"
echo "                            v1.2.54"
echo "-----------------------------------"
echo "Olla URL:         $OLLA_URL"
echo "Proxy Endpoint:   $PROXY_ENDPOINT"
echo "Health Endpoint:  $HEALTH_ENDPOINT"
echo "Model:            $MODEL_NAME"
echo "-----------------------------------"
echo ""
echo "Test Configured for:"
echo "  - Per-IP Rate Limit: ${OLLA_SERVER_PER_IP_RATE_LIMIT:-20}  # 20/minute for more aggressive testing"
echo "  - Rate Burst Size:   ${OLLA_SERVER_RATE_BURST_SIZE:-3}    # Small burst for clear testing"
echo "  - Health Rate Limit: ${OLLA_SERVER_HEALTH_RATE_LIMIT:-60}  # 60/minute = 1 per second"
echo "  - Global Rate Limit: ${OLLA_SERVER_GLOBAL_RATE_LIMIT:-0}    # Disable global limiting for cleaner tests"
echo ""
echo "Set these and run olla:"
echo "  export OLLA_SERVER_PER_IP_RATE_LIMIT=\"20\""
echo "  export OLLA_SERVER_RATE_BURST_SIZE=\"3\""
echo "  export OLLA_SERVER_HEALTH_RATE_LIMIT=\"60\""
echo "  export OLLA_SERVER_GLOBAL_RATE_LIMIT=\"0\""
echo "  make run"
echo "-----------------------------------"

# Test tracking variables
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=()

# Function to record test result
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

# Function to make a single request and get detailed info
make_test_request() {
    local endpoint="$1"
    local client_ip="$2"

    local curl_cmd="curl -s -D - -o /dev/null -X POST -H \"Content-Type: application/json\" -d '{\"model\":\"$MODEL_NAME\",\"prompt\":\"test\"}'"

    if [[ -n "$client_ip" ]]; then
        curl_cmd="$curl_cmd -H \"X-Forwarded-For: $client_ip\""
    fi

    curl_cmd="$curl_cmd \"$OLLA_URL$endpoint\""

    local response=$(eval $curl_cmd 2>/dev/null)
    local http_status=$(echo "$response" | grep "HTTP/" | tail -1 | awk '{print $2}')
    local rate_limit=$(echo "$response" | grep -i "x-ratelimit-limit:" | cut -d: -f2 | tr -d ' \r')
    local rate_remaining=$(echo "$response" | grep -i "x-ratelimit-remaining:" | cut -d: -f2 | tr -d ' \r')
    local retry_after=$(echo "$response" | grep -i "retry-after:" | cut -d: -f2 | tr -d ' \r')

    echo "$http_status|$rate_limit|$rate_remaining|$retry_after"
}

# Test 1: Basic Rate Limiting Detection
test_basic_rate_limiting() {
    echo
    echo "Testing: Basic Rate Limiting"
    echo "Expected: Should see both successful requests (200) and rate limited requests (429)"
    echo "This proves rate limiting is active and working correctly"
    echo

    local requests=15
    local statuses=()
    local success_count=0
    local rate_limited_count=0
    local headers_present=false

    echo "Making $requests rapid requests..."

    for ((i=1; i<=requests; i++)); do
        local result=$(make_test_request "$PROXY_ENDPOINT" "")
        local status=$(echo "$result" | cut -d'|' -f1)
        local limit=$(echo "$result" | cut -d'|' -f2)
        local remaining=$(echo "$result" | cut -d'|' -f3)
        local retry_after=$(echo "$result" | cut -d'|' -f4)

        statuses+=("$status")
        echo "  Request $i: $status (Limit: $limit, Remaining: $remaining$(if [[ -n "$retry_after" ]]; then echo ", Retry: ${retry_after}s"; fi))"

        if [[ "$status" == "200" || "$status" == "502" ]]; then
            success_count=$((success_count + 1))
        elif [[ "$status" == "429" ]]; then
            rate_limited_count=$((rate_limited_count + 1))
        fi

        if [[ -n "$limit" ]]; then
            headers_present=true
        fi
    done

    echo
    echo "Results Analysis:"
    echo "  Successful requests: $success_count"
    echo "  Rate limited requests: $rate_limited_count"
    echo "  Headers present: $headers_present"
    echo "  Status sequence: ${statuses[*]}"

    # Success criteria: Must have both success AND rate limiting to prove it's working
    if [[ $success_count -gt 0 && $rate_limited_count -gt 0 && "$headers_present" == "true" ]]; then
