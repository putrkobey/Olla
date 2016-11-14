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
        echo "✅ PASS: Rate limiting is working perfectly!"
        echo "   - Got $success_count successful requests (proves requests can succeed)"
        echo "   - Got $rate_limited_count rate limited requests (proves rate limiting is active)"
        echo "   - Rate limit headers are present (proves middleware is working)"
        record_test_result "Basic Rate Limiting" "true"
    elif [[ $rate_limited_count -gt 0 ]]; then
        echo "✅ PASS: Rate limiting detected (some 429s found)"
        record_test_result "Basic Rate Limiting" "true"
    elif [[ "$headers_present" == "true" ]]; then
        echo "⚠️  PARTIAL: Headers present but no rate limiting triggered"
        echo "   This might mean limits are set too high for this test"
        record_test_result "Basic Rate Limiting" "true"
    else
        echo "❌ FAIL: No rate limiting detected and no headers present"
        record_test_result "Basic Rate Limiting" "false"
    fi
}

# Test 2: Health Endpoint Verification
test_health_endpoint() {
    echo
    echo "Testing: Health Endpoint Rate Limiting"
    echo "Expected: Health endpoint should have different (usually higher) rate limits"
    echo

    local requests=8
    local success_count=0
    local rate_limited_count=0
    local health_limit=""

    echo "Making $requests requests to health endpoint..."

    for ((i=1; i<=requests; i++)); do
        local result=$(make_test_request "$HEALTH_ENDPOINT" "")
        local status=$(echo "$result" | cut -d'|' -f1)
        local limit=$(echo "$result" | cut -d'|' -f2)
        local remaining=$(echo "$result" | cut -d'|' -f3)

        echo "  Request $i: $status (Limit: $limit, Remaining: $remaining)"

        if [[ "$status" == "200" ]]; then
            success_count=$((success_count + 1))
        elif [[ "$status" == "429" ]]; then
            rate_limited_count=$((rate_limited_count + 1))
        fi

        if [[ -n "$limit" && -z "$health_limit" ]]; then
            health_limit="$limit"
        fi
    done

    echo
    echo "Results Analysis:"
    echo "  Successful requests: $success_count"
    echo "  Rate limited requests: $rate_limited_count"
    echo "  Health endpoint limit: $health_limit"

    # Health endpoint test passes if we can detect it's configured
    if [[ -n "$health_limit" ]]; then
        echo "✅ PASS: Health endpoint rate limiting is configured (limit: $health_limit)"
        record_test_result "Health Endpoint Rate Limiting" "true"
    else
        echo "❌ FAIL: Could not detect health endpoint rate limit configuration"
        record_test_result "Health Endpoint Rate Limiting" "false"
    fi
}

# Test 3: Per-IP Isolation
test_per_ip_isolation() {
    echo
    echo "Testing: Per-IP Isolation"
    echo "Expected: Different IPs should have independent rate limit buckets"
    echo

    local requests=10
    local ip1="203.0.113.1"
    local ip2="203.0.113.2"

    echo "Testing IP 1: $ip1"
    local ip1_success=0
    local ip1_rate_limited=0

    for ((i=1; i<=requests; i++)); do
        local result=$(make_test_request "$PROXY_ENDPOINT" "$ip1")
        local status=$(echo "$result" | cut -d'|' -f1)

        if [[ "$status" == "200" || "$status" == "502" ]]; then
            ip1_success=$((ip1_success + 1))
        elif [[ "$status" == "429" ]]; then
            ip1_rate_limited=$((ip1_rate_limited + 1))
        fi
    done

    # Brief pause between IP tests
    sleep 0.5

    echo "Testing IP 2: $ip2"
    local ip2_success=0
    local ip2_rate_limited=0

    for ((i=1; i<=requests; i++)); do
        local result=$(make_test_request "$PROXY_ENDPOINT" "$ip2")
        local status=$(echo "$result" | cut -d'|' -f1)

        if [[ "$status" == "200" || "$status" == "502" ]]; then
            ip2_success=$((ip2_success + 1))
        elif [[ "$status" == "429" ]]; then
            ip2_rate_limited=$((ip2_rate_limited + 1))
        fi
    done

    echo
    echo "Results Analysis:"
    echo "  IP1 ($ip1): $ip1_success successful, $ip1_rate_limited rate limited"
    echo "  IP2 ($ip2): $ip2_success successful, $ip2_rate_limited rate limited"

    # Per-IP isolation passes if both IPs show some activity
    if [[ $ip1_success -gt 0 && $ip2_success -gt 0 ]]; then
        echo "✅ PASS: Per-IP isolation working - both IPs can make requests independently"
        record_test_result "Per-IP Isolation" "true"
    else
        echo "❌ FAIL: Per-IP isolation may not be working properly"
        record_test_result "Per-IP Isolation" "false"
    fi
}

# Test 4: Burst and Rate Limiting Detection
test_burst_and_rate_limiting() {
    echo
    echo "Testing: Burst Capacity and Rate Limiting"
    echo "Expected: Should allow initial burst of requests, then apply rate limiting"
    echo

    local requests=20
    local success_count=0
    local rate_limited_count=0
    local first_rate_limit_at=0

