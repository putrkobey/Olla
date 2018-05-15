#!/bin/bash

# Test provider-specific routing endpoints
# Tests the new /olla/{provider}/api/* endpoints

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
CURL_TIMEOUT=30
VERBOSE="${VERBOSE:-false}"

# Track results
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

function banner() {
    echo -e "${PURPLE}╔═════════════════════════════════════════════════════════╗${RESET}"
    echo -e "${PURPLE}║${RESET}  ${CYAN}Olla Provider-Specific Routing Test${RESET}                    ${PURPLE}║${RESET}"
    echo -e "${PURPLE}╚═════════════════════════════════════════════════════════╝${RESET}"
    echo
}

# Test a specific endpoint
test_endpoint() {
    local endpoint=$1
    local description=$2
    local expected_format=$3
    
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    
    echo -e "\n${WHITE}Testing:${RESET} ${CYAN}$endpoint${RESET}"
    echo -e "${GREY}Description: $description${RESET}"
    echo -e "${GREY}Expected format: $expected_format${RESET}"
    
    local response
    local http_code
    
    response=$(curl -s -w "\n%{http_code}" \
        -X GET \
        -H "User-Agent: OllaProviderTest/1.0" \
        --max-time "$CURL_TIMEOUT" \
        "${TARGET_URL}${endpoint}" 2>&1)
    
    http_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | sed '$d')
    
    if [[ "$http_code" =~ ^2[0-9][0-9]$ ]]; then
        echo -e "  ${GREEN}✓ Success${RESET} (HTTP $http_code)"
        
        # Validate response format
        case "$expected_format" in
            "ollama")
                if echo "$body" | grep -q '"models"' && echo "$body" | grep -q '"name"'; then
                    echo -e "  ${GREEN}✓ Valid Ollama format${RESET}"
                    # Count models
                    model_count=$(echo "$body" | grep -o '"name"' | wc -l)
                    echo -e "  ${CYAN}→ Found $model_count Ollama models${RESET}"
                    PASSED_TESTS=$((PASSED_TESTS + 1))
                else
                    echo -e "  ${RED}✗ Invalid Ollama format${RESET}"
                    FAILED_TESTS=$((FAILED_TESTS + 1))
                fi
                ;;
            "openai")
                if echo "$body" | grep -q '"object":"list"' && echo "$body" | grep -q '"data"'; then
                    echo -e "  ${GREEN}✓ Valid OpenAI format${RESET}"
                    # Count models
                    model_count=$(echo "$body" | grep -o '"id"' | wc -l)
                    echo -e "  ${CYAN}→ Found $model_count models${RESET}"
                    PASSED_TESTS=$((PASSED_TESTS + 1))
                else
                    echo -e "  ${RED}✗ Invalid OpenAI format${RESET}"
                    FAILED_TESTS=$((FAILED_TESTS + 1))
                fi
                ;;
        esac
        
        if [ "$VERBOSE" = "true" ]; then
            echo -e "  ${GREY}Response preview: ${body:0:200}...${RESET}"
        fi
    else
        echo -e "  ${RED}✗ Failed${RESET} (HTTP $http_code)"
        FAILED_TESTS=$((FAILED_TESTS + 1))
        
        if [ "$http_code" = "404" ]; then
            echo -e "  ${YELLOW}→ No providers of this type available${RESET}"
        elif [ "$http_code" = "000" ]; then
            echo -e "  ${YELLOW}→ Connection timeout or DNS resolution failed${RESET}"
        elif [ -n "$body" ]; then
