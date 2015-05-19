#!/bin/bash

# Olla Provider-Specific Model Endpoints Test Script
# Tests the new provider-specific model listing endpoints
####
# Usage: ./test-provider-models.sh
# Configuration: Set TARGET_URL environment variable if needed

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
    echo -e "${PURPLE}║${RESET}  ${CYAN}Olla Provider Model Endpoints Test${RESET}                     ${PURPLE}║${RESET}"
    echo -e "${PURPLE}╚═════════════════════════════════════════════════════════╝${RESET}"
    echo
    echo -e "${WHITE}Expected Behavior:${RESET}"
    echo -e "${GREY}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}"
    echo -e "${CYAN}/olla/ollama/api/tags${RESET}        ${BLUE}→${RESET} Ollama models (native format)"
    echo -e "${CYAN}/olla/ollama/v1/models${RESET}       ${BLUE}→${RESET} Ollama models (OpenAI format)"
    echo -e "${CYAN}/olla/lmstudio/v1/models${RESET}     ${BLUE}→${RESET} LM Studio models (OpenAI format)"
    echo -e "${CYAN}/olla/lmstudio/api/v0/*${RESET}      ${BLUE}→${RESET} LM Studio models (enhanced format)"
    echo -e "${CYAN}/olla/openai/v1/models${RESET}       ${BLUE}→${RESET} OpenAI-compatible models"
    echo -e "${CYAN}/olla/vllm/v1/models${RESET}         ${BLUE}→${RESET} vLLM models (enhanced metadata)"
    echo -e "${GREY}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}"
    echo -e "${CYAN}/olla/models${RESET}                 ${BLUE}→${RESET} Olla Unified format (default)"
    echo -e "${CYAN}/olla/models?format=unified${RESET}  ${BLUE}→${RESET} Olla Unified format (explicit)"
    echo -e "${CYAN}/olla/models?format=openai${RESET}   ${BLUE}→${RESET} OpenAI format (all models)"
    echo -e "${CYAN}/olla/models?format=ollama${RESET}   ${BLUE}→${RESET} Ollama format (filters to Ollama only)"
    echo -e "${CYAN}/olla/models?format=lmstudio${RESET} ${BLUE}→${RESET} LM Studio format (filters to LM Studio only)"
    echo -e "${CYAN}/olla/models?format=vllm${RESET}     ${BLUE}→${RESET} vLLM format (filters to vLLM only)"
    echo
}

usage() {
    echo -e "${WHITE}Usage:${RESET} $0"
    echo
    echo -e "${YELLOW}Environment Variables:${RESET}"
    echo -e "  TARGET_URL      Olla proxy URL (default: http://localhost:40114)"
    echo -e "  VERBOSE         Show detailed request/response info (default: false)"
    echo
    echo -e "${YELLOW}Description:${RESET}"
    echo -e "  This script tests the provider-specific model listing endpoints"
    echo -e "  to ensure they return models in the correct format for each provider."
    echo
}

test_endpoint() {
    local endpoint=$1
    local expected_format=$2
    local description=$3
    
    echo -e "\n${WHITE}Testing:${RESET} ${CYAN}$endpoint${RESET}"
    echo -e "${GREY}Expected format: $expected_format${RESET}"
    echo -e "${GREY}Description: $description${RESET}"
    
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    
    # Make the request
    local response
    local http_code
    
    response=$(curl -s -w "\n%{http_code}" \
        -X GET \
        -H "User-Agent: OllaProviderTest/1.0" \
        --max-time "$CURL_TIMEOUT" \
        "${TARGET_URL}${endpoint}" 2>&1)
    
    http_code=$(echo "$response" | tail -n1)
