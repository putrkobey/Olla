#!/bin/bash
# Test script for Claude Code + Ollama + Olla setup

set -e  # Exit on error

OLLA_URL="http://localhost:40114"
ANTHROPIC_URL="${OLLA_URL}/olla/anthropic/v1"

echo "Testing Olla + Ollama + Anthropic Translation..."
echo ""

# Colours for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test 1: Health check
echo -e "${YELLOW}Test 1: Checking Olla health...${NC}"
if curl -sf "${OLLA_URL}/internal/health" > /dev/null; then
    echo -e "${GREEN}  Olla is healthy${NC}"
else
    echo -e "${RED}  Olla health check failed${NC}"
    exit 1
fi
echo ""

# Test 2: List models
echo -e "${YELLOW}Test 2: Listing available models...${NC}"
MODELS=$(curl -s "${ANTHROPIC_URL}/models")
if echo "$MODELS" | jq -e '.data | length > 0' > /dev/null 2>&1; then
    echo -e "${GREEN}  Models available:${NC}"
    echo "$MODELS" | jq -r '.data[].id' | sed 's/^/    - /'
