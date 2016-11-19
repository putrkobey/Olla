#!/bin/bash

# Olla Model Routing Test Script
# Tests model routing with different models and capabilities
# Now displays response headers showing which endpoint handled each request
####
# Usage: ./test-model-routing.sh [additional_model1] [additional_model2] ...
# Configuration: Set TARGET_URL environment variable if needed
# 
# Response headers displayed:
#   X-Olla-Endpoint: The endpoint that handled the request
#   X-Olla-Model: The model name if specified in the request
#   X-Olla-Backend-Type: The backend type (ollama, openai, lmstudio)
#   X-Olla-Request-ID: Unique request identifier for correlation
#   X-Olla-Response-Time: Total response time including streaming
#   X-Served-By: Server identification header

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
WHITE='\033[1;37m'
GREY='\033[0;37m'
RESET='\033[0m'

# Configuration
TARGET_URL="${TARGET_URL:-http://localhost:40114}"
# Default to Ollama provider - users can override with environment variable
PROVIDER="${PROVIDER:-openai}"
PROXY_ENDPOINT="/olla/${PROVIDER}/"
CURL_TIMEOUT=180
VERBOSE="${VERBOSE:-false}"

# Arrays for models
declare -a AVAILABLE_MODELS=()
declare -a ADDITIONAL_MODELS=()
declare -a ALL_MODELS=()

# Test cases with different capabilities
declare -A TEST_CASES
TEST_CASES["simple_chat"]='{"model": "MODEL_NAME", "messages": [{"role": "user", "content": "Say hello"}], "max_tokens": 50}'
TEST_CASES["vision_request"]='{"model": "MODEL_NAME", "messages": [{"role": "user", "content": [{"type": "text", "text": "What do you see?"}, {"type": "image_url", "image_url": {"url": "https://example.com/test.jpg"}}]}], "max_tokens": 50}'
TEST_CASES["function_call"]='{"model": "MODEL_NAME", "messages": [{"role": "user", "content": "Get the weather"}], "tools": [{"type": "function", "function": {"name": "get_weather", "description": "Get weather"}}], "max_tokens": 50}'
TEST_CASES["code_request"]='{"model": "MODEL_NAME", "messages": [{"role": "system", "content": "You are a code assistant"}, {"role": "user", "content": "Write a function"}], "max_tokens": 100}'
TEST_CASES["embedding_request"]='{"model": "MODEL_NAME", "input": "This is a test sentence for embeddings"}'

# Track results
TOTAL_TESTS=0
SUCCESSFUL_ROUTES=0
FAILED_ROUTES=0

# Track endpoint usage
declare -A ENDPOINT_USAGE
declare -A ENDPOINT_SUCCESS
declare -A ENDPOINT_FAILURE

# Track script timing
SCRIPT_START_TIME=$(date +%s)

function banner() {
    echo -e "${PURPLE}╔═════════════════════════════════════════════════════════╗${RESET}"
    echo -e "${PURPLE}║${RESET}  ${CYAN}Olla Model Routing Test${RESET}                                ${PURPLE}║${RESET}"
    echo -e "${PURPLE}╚═════════════════════════════════════════════════════════╝${RESET}"
    echo
}

usage() {
    echo -e "${WHITE}Usage:${RESET} $0 [additional_model1] [additional_model2] ..."
    echo
    echo -e "${YELLOW}Environment Variables:${RESET}"
    echo -e "  TARGET_URL      Olla proxy URL (default: http://localhost:40114)"
    echo -e "  PROVIDER        Provider to test (default: openai) - ollama, lmstudio, openai, vllm"
    echo -e "  VERBOSE         Show detailed request/response info (default: false)"
    echo
    echo -e "${YELLOW}Description:${RESET}"
    echo -e "  This script automatically discovers available models from Olla"
    echo -e "  and tests routing for each model with appropriate capabilities."
    echo -e "  You can also specify additional models to test as arguments."
    echo
    echo -e "${YELLOW}Response Headers:${RESET}"
    echo -e "  The script now displays routing information from response headers:"
    echo -e "  • X-Olla-Endpoint - Which backend endpoint handled the request"
    echo -e "  • X-Olla-Model - The model used (if specified in request)"
    echo -e "  • X-Olla-Backend-Type - Backend platform (ollama, openai, lmstudio)"
    echo -e "  • X-Olla-Request-ID - Unique request ID for correlation"
    echo -e "  • X-Olla-Response-Time - Total time including streaming"
    echo -e "  • X-Served-By - Server identification"
    echo
    echo -e "${YELLOW}Examples:${RESET}"
    echo -e "  $0                               # Test only discovered models (OpenAI)"
    echo -e "  PROVIDER=ollama $0               # Test Ollama endpoints"
    echo -e "  PROVIDER=lmstudio $0             # Test LM Studio endpoints"
    echo -e "  PROVIDER=vllm $0                 # Test vLLM endpoints"
    echo -e "  $0 gpt-4-turbo claude-3-sonnet   # Test discovered + additional models"
    echo -e "  VERBOSE=true $0                  # Show detailed debug information"
    echo
}

# Fetch available models from Olla
fetch_available_models() {
    echo -e "${YELLOW}Fetching available models from Olla...${RESET}"
    
    local response
    local http_code
    
    response=$(curl -s -w "\n%{http_code}" \
        -X GET \
        -H "User-Agent: OllaRoutingTest/1.0" \
        --max-time "$CURL_TIMEOUT" \
        "${TARGET_URL}/olla/models" 2>&1)
    
    http_code=$(echo "$response" | tail -n1)
    
    if [[ "$http_code" =~ ^2[0-9][0-9]$ ]]; then
        # Extract model IDs from JSON response
        local model_data=$(echo "$response" | sed '$d')
        
        # Parse JSON to get model IDs (works with simple JSON structure)
        while IFS= read -r model_id; do
            if [ -n "$model_id" ]; then
                AVAILABLE_MODELS+=("$model_id")
            fi
        done < <(echo "$model_data" | grep -o '"id"[[:space:]]*:[[:space:]]*"[^"]*"' | sed 's/.*: *"\([^"]*\)".*/\1/')
        
        echo -e "${GREEN}✓ Found ${#AVAILABLE_MODELS[@]} models in Olla${RESET}"
        
        # Show discovered models
        if [ ${#AVAILABLE_MODELS[@]} -gt 0 ]; then
            echo -e "${GREY}Available models:${RESET}"
            for model in "${AVAILABLE_MODELS[@]}"; do
                echo -e "  • ${CYAN}$model${RESET}"
            done
        fi
    else
        echo -e "${RED}✗ Failed to fetch models (HTTP $http_code)${RESET}"
        echo -e "${YELLOW}Will proceed with additional models only${RESET}"
    fi
    echo
}

test_model_routing() {
    local model=$1
    local test_name=$2
    local request_body=$3
    local endpoint=$4
    
    # Replace MODEL_NAME with actual model
    request_body="${request_body//MODEL_NAME/$model}"
    
    echo -e "\n${WHITE}Testing:${RESET} ${CYAN}$model${RESET} - ${YELLOW}$test_name${RESET}"
    echo -e "${GREY}Endpoint: ${endpoint}${RESET}"
    
    # Make the request and capture all details
    local response_file="/tmp/olla_test_response_$$"
    local headers_file="/tmp/olla_test_headers_$$"
    local http_code
    local response_time
    
    # Use curl with more detailed output and capture headers
    # Note: --trace-ascii shows trailers which -D doesn't capture
    local curl_output=$(curl -s -w "\n%{http_code}\n%{time_total}" \
        -o "$response_file" \
        -D "$headers_file" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "User-Agent: OllaRoutingTest/1.0" \
        -d "$request_body" \
        --max-time "$CURL_TIMEOUT" \
        "${TARGET_URL}${PROXY_ENDPOINT}${endpoint}" 2>&1)
    
    # Extract HTTP code and response time from the last two lines
    http_code=$(echo "$curl_output" | tail -n2 | head -n1)
    response_time=$(echo "$curl_output" | tail -n1)
    
    # Read response body
    local response=""
    if [ -f "$response_file" ]; then
        response=$(cat "$response_file")
        rm -f "$response_file"
    fi
    
    # Parse response headers
    local olla_endpoint=""
    local olla_model=""
    local olla_backend_type=""
    local olla_request_id=""
    local olla_response_time=""
    local served_by=""
    if [ -f "$headers_file" ]; then
        olla_endpoint=$(grep -i "^X-Olla-Endpoint:" "$headers_file" | cut -d' ' -f2- | tr -d '\r' || true)
        olla_model=$(grep -i "^X-Olla-Model:" "$headers_file" | cut -d' ' -f2- | tr -d '\r' || true)
        olla_backend_type=$(grep -i "^X-Olla-Backend-Type:" "$headers_file" | cut -d' ' -f2- | tr -d '\r' || true)
        olla_request_id=$(grep -i "^X-Olla-Request-ID:" "$headers_file" | cut -d' ' -f2- | tr -d '\r' || true)
        olla_response_time=$(grep -i "^X-Olla-Response-Time:" "$headers_file" | cut -d' ' -f2- | tr -d '\r' || true)
        served_by=$(grep -i "^X-Served-By:" "$headers_file" | cut -d' ' -f2- | tr -d '\r' || true)
        rm -f "$headers_file"
    fi
    
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    
    # Check response
    if [[ "$http_code" =~ ^2[0-9][0-9]$ ]]; then
        echo -e "  ${GREEN}✓ Success${RESET} (HTTP $http_code) - ${GREY}${response_time}s${RESET}"
        SUCCESSFUL_ROUTES=$((SUCCESSFUL_ROUTES + 1))
        
        # Display routing headers
        if [ -n "$olla_endpoint" ]; then
            if [ -n "$olla_backend_type" ]; then
                echo -e "  ${CYAN}→ Routed to:${RESET} ${WHITE}$olla_endpoint${RESET} (${GREY}$olla_backend_type${RESET})"
            else
                echo -e "  ${CYAN}→ Routed to:${RESET} ${WHITE}$olla_endpoint${RESET}"
            fi
            
            # Track endpoint usage for summary
            ENDPOINT_USAGE["$olla_endpoint"]=$((${ENDPOINT_USAGE["$olla_endpoint"]:-0} + 1))
            ENDPOINT_SUCCESS["$olla_endpoint"]=$((${ENDPOINT_SUCCESS["$olla_endpoint"]:-0} + 1))
        fi
        if [ -n "$olla_model" ]; then
            echo -e "  ${CYAN}→ Model:${RESET} ${WHITE}$olla_model${RESET}"
        fi
        if [ -n "$olla_request_id" ]; then
            echo -e "  ${CYAN}→ Request ID:${RESET} ${GREY}$olla_request_id${RESET}"
        fi
        if [ -n "$olla_response_time" ]; then
            echo -e "  ${CYAN}→ Response Time:${RESET} ${YELLOW}$olla_response_time${RESET}"
        elif [ -n "$response_time" ]; then
            # Fallback to curl's measured time if header not available
            echo -e "  ${CYAN}→ Response Time:${RESET} ${YELLOW}${response_time}s${RESET} (client-side)"
        fi
        if [ -n "$served_by" ]; then
            echo -e "  ${CYAN}→ Served by:${RESET} ${GREY}$served_by${RESET}"
        fi
        
        # Debug mode - show all Olla headers if verbose
        if [ "$VERBOSE" = "true" ] && [ -f "$headers_file" ]; then
            echo -e "  ${GREY}Headers received:${RESET}"
            grep -i "^X-Olla-" "$headers_file" | while read -r header; do
                echo -e "    ${GREY}$header${RESET}"
            done
        fi
        
        # Fallback to response body if headers not present
