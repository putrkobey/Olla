#!/usr/bin/env bash
# Test script for OpenCode + LM Studio + Olla integration.
# Verifies connectivity from the host: LM Studio, the Olla container,
# Olla health, endpoint status, the OpenAI models endpoint, and a chat
# completion.

set -e

# Colours for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No colour

# Configuration
LM_STUDIO_HOST="${LM_STUDIO_HOST:-localhost}"
LM_STUDIO_PORT="${LM_STUDIO_PORT:-1234}"
OLLA_HOST="${OLLA_HOST:-localhost}"
OLLA_PORT="${OLLA_PORT:-40114}"

echo -e "${BLUE}=====================================${NC}"
echo -e "${BLUE}OpenCode + LM Studio Integration Test${NC}"
echo -e "${BLUE}=====================================${NC}"
echo ""

# Function to print test results
print_result() {
    if [ "$1" -eq 0 ]; then
        echo -e "${GREEN}PASS${NC}: $2"
    else
        echo -e "${RED}FAIL${NC}: $2"
        if [ -n "$3" ]; then
            echo -e "${YELLOW}  $3${NC}"
        fi
    fi
}

# Test 1: Check if LM Studio is running on host
echo -e "${BLUE}[1/6]${NC} Checking LM Studio connectivity on host..."
if curl -s -f "http://${LM_STUDIO_HOST}:${LM_STUDIO_PORT}/v1/models" > /dev/null 2>&1; then
    print_result 0 "LM Studio is running on ${LM_STUDIO_HOST}:${LM_STUDIO_PORT}"
    LM_STUDIO_MODELS=$(curl -s "http://${LM_STUDIO_HOST}:${LM_STUDIO_PORT}/v1/models" | jq -r '.data[].id' 2>/dev/null || echo "")
    if [ -n "$LM_STUDIO_MODELS" ]; then
        echo -e "  ${GREEN}Models loaded:${NC}"
        echo "$LM_STUDIO_MODELS" | sed 's/^/    - /'
    else
        echo -e "  ${YELLOW}Warning: No models loaded in LM Studio${NC}"
    fi
else
    print_result 1 "LM Studio is not reachable" \
        "Start LM Studio and enable server mode (Developer -> Server -> Start Server)"
