#!/bin/bash

# Olla Load Test Script
# Tests the proxy endpoint with configurable concurrency and duration
# Reads config from .env file, takes duration and concurrency as args
####
# Usage: ./test-load-limits.sh <duration> <concurrency>
# Example: ./test-load-limits.sh 300 10 # Defaults: 5 minutes duration, 10 concurrent workers

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
WHITE='\033[1;37m'
GREY='\033[0;37m'
BOLD='\033[1m'
DIM='\033[2m'
RESET='\033[0m'

# Global variables for tracking
TOTAL_REQUESTS=0
SUCCESSFUL_REQUESTS=0
FAILED_REQUESTS=0
TOTAL_RESPONSE_TIME=0
MIN_RESPONSE_TIME=999999
MAX_RESPONSE_TIME=0
START_TIME=""
STATS_DIR=""
WORKER_PIDS=""

# Configuration defaults
PROVIDER="${PROVIDER:-ollama}"
PROXY_ENDPOINT="/olla/${PROVIDER}/"
TARGET_URL=""
RATE_LIMIT_DELAY=0
MODEL_NAME="phi4:latest"
DURATION=60
CONCURRENCY=5
CURL_MAX_TIME=300

# Test questions for the model
QUESTIONS=(
    "What is the capital of Australia?"
    "Explain quantum computing in simple terms"
    "Write a haiku about coffee"
    "What are the benefits of renewable energy?"
    "How does machine learning work?"
    "Describe the water cycle"
    "What is the difference between AI and ML?"
    "List 3 interesting facts about space"
    "How do you make a perfect cup of tea?"
    "What is the meaning of life?"
    "Explain photosynthesis briefly"
    "What are the main programming paradigms?"
    "How does the internet work?"
    "What is climate change?"
    "Describe how a computer processor works"
    "What are the health benefits of exercise?"
    "How do vaccines work?"
    "What is blockchain technology?"
    "Explain the concept of time zones"
    "What makes a good story?"
)

function banner() {
    echo -e "${PURPLE}╔═════════════════════════════════════════════════════════╗${RESET}"
    echo -e "${PURPLE}║${RESET}  ${CYAN}🦙 Olla Load Test${RESET} ${GREY}- Continuous Proxy Testing${RESET}           ${PURPLE}║${RESET}"
    echo -e "${PURPLE}╚═════════════════════════════════════════════════════════╝${RESET}"
    echo
}

usage() {
    echo -e "${WHITE}Usage:${RESET} $0 <duration> <concurrency>"
    echo
    echo -e "${YELLOW}Arguments:${RESET}"
    echo -e "  duration     Test duration in seconds"
    echo -e "  concurrency  Number of concurrent workers"
    echo
    echo -e "${YELLOW}Configuration (.env file):${RESET}"
    echo -e "  TARGET_URL      Olla proxy URL (e.g., http://localhost:40114)"
    echo -e "  RATE_LIMIT      Delay between requests in seconds (default: 0)"
    echo -e "  MODEL_NAME      Model to test with (default: phi4:latest)"
    echo
    echo -e "${YELLOW}Environment Variables:${RESET}"
    echo -e "  PROVIDER        Provider to test (default: ollama) - ollama, lmstudio, openai, vllm"
    echo
    echo -e "${YELLOW}Example:${RESET}"
    echo -e "  $0 300 10   # Run for 5 minutes with 10 concurrent workers"
    echo
}

load_env() {
    if [ ! -f ".env" ]; then
        echo -e "${RED}ERROR:${RESET} .env file not found"
        echo
        echo -e "${YELLOW}Create a .env file with:${RESET}"
        echo -e "TARGET_URL=http://localhost:40114"
        echo -e "RATE_LIMIT=0.1"
        echo -e "MODEL_NAME=phi4:latest"
        exit 1
    fi

    # Load .env file the simple way
    while IFS='=' read -r key value; do
        # Skip comments and empty lines
        case "$key" in
            '#'*|'') continue ;;
        esac

        # Remove quotes if present
        value=$(echo "$value" | sed 's/^["'\'']//' | sed 's/["'\'']$//')

        case "$key" in
            'TARGET_URL') TARGET_URL="$value" ;;
            'RATE_LIMIT') RATE_LIMIT_DELAY="$value" ;;
            'MODEL_NAME') MODEL_NAME="$value" ;;
        esac
    done < .env

    if [ -z "$TARGET_URL" ]; then
        echo -e "${RED}ERROR:${RESET} TARGET_URL not set in .env file"
        exit 1
    fi
}

validate_args() {
    if [ $# -ne 2 ]; then
        usage
        exit 1
    fi

    if ! echo "$1" | grep -q '^[0-9]\+$' || [ "$1" -le 0 ]; then
        echo -e "${RED}ERROR:${RESET} Duration must be a positive integer"
        exit 1
    fi

    if ! echo "$2" | grep -q '^[0-9]\+$' || [ "$2" -le 0 ]; then
        echo -e "${RED}ERROR:${RESET} Concurrency must be a positive integer"
        exit 1
    fi

    DURATION=$1
    CONCURRENCY=$2
}

setup_temp_files() {
    STATS_DIR="/tmp/olla_load_test_$$"
    mkdir -p "$STATS_DIR"

    # Initialise stats
    echo "0" > "$STATS_DIR/total"
    echo "0" > "$STATS_DIR/success"
    echo "0" > "$STATS_DIR/failed"
    echo "0" > "$STATS_DIR/total_time"
    echo "999999" > "$STATS_DIR/min_time"
    echo "0" > "$STATS_DIR/max_time"
    echo "" > "$STATS_DIR/errors"
    echo "" > "$STATS_DIR/pids"
}

cleanup() {
    echo
    echo -e "${YELLOW}Cleaning up...${RESET}"

    # Kill all worker processes
    if [ -f "$STATS_DIR/pids" ]; then
        while read -r pid; do
