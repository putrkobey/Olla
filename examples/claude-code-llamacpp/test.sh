#!/bin/bash
# Test script for Claude Code + llama.cpp + Olla setup

set -e

OLLA_URL="http://localhost:40114"
ANTHROPIC_URL="${OLLA_URL}/olla/anthropic/v1"
LLAMACPP_URL="http://localhost:8080"

echo "Testing Olla + llama.cpp + Anthropic Translation..."
echo ""

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Test 1: llama.cpp health
echo -e "${YELLOW}Test 1: Checking llama.cpp health...${NC}"
if curl -sf "${LLAMACPP_URL}/health" > /dev/null; then
    echo -e "${GREEN}  llama.cpp is healthy${NC}"
else
    echo -e "${RED}  llama.cpp health check failed${NC}"
    echo "  Possible causes:"
    echo "    - Model still loading (wait 30-60 seconds and retry)"
    echo "    - Model file path incorrect in compose.yaml"
    echo "    - Insufficient memory for model"
    exit 1
fi
echo ""

# Test 2: Olla health
echo -e "${YELLOW}Test 2: Checking Olla health...${NC}"
if curl -sf "${OLLA_URL}/internal/health" > /dev/null; then
    echo -e "${GREEN}  Olla is healthy${NC}"
else
    echo -e "${RED}  Olla health check failed${NC}"
    exit 1
fi
echo ""

# Test 3: List models via Anthropic endpoint
echo -e "${YELLOW}Test 3: Listing available models...${NC}"
MODELS=$(curl -s "${ANTHROPIC_URL}/models")
if echo "$MODELS" | jq -e '.data | length > 0' > /dev/null 2>&1; then
    echo -e "${GREEN}  Models available:${NC}"
    echo "$MODELS" | jq -r '.data[].id' | sed 's/^/    - /'
else
    echo -e "${RED}  No models available${NC}"
    echo "  Check llama.cpp logs:"
    echo "    docker compose logs llama-cpp"
    exit 1
fi
