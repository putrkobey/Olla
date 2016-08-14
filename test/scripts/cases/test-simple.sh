#!/bin/bash
# Simple test to verify the test matrix works

# Test matrix
PROXY_ENGINES=("sherpa" "olla")
PROXY_PROFILES=("auto" "standard" "streaming")

echo "Starting test matrix..."

total=$((${#PROXY_ENGINES[@]} * ${#PROXY_PROFILES[@]}))
