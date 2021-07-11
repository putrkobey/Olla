#!/bin/bash
# test-proxy-engine-profiles.sh
# Tests all combinations of proxy engines (olla/sherpa) and profiles (auto/standard/streaming)
# This script automates the testing of streaming behavior across different configurations

set -uo pipefail
# Temporarily disable -e to debug

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Source common functions and variables
source "$SCRIPT_DIR/_common.sh"
source "$SCRIPT_DIR/_olla.sh"
source "$SCRIPT_DIR/_streaming_tests.sh"

# Configuration
BASE_CONFIG=""
TEST_RESULTS_DIR=""
TEST_PORT=""

# Test configuration matrix
PROXY_ENGINES=("sherpa" "olla")
PROXY_PROFILES=("auto" "standard" "streaming")

# Function to parse command line arguments
parse_args() {
    while getopts "c:h" opt; do
        case ${opt} in
            c )
                BASE_CONFIG="$OPTARG"
                ;;
            h )
                echo "Usage: $0 -c <config_file>"
                echo "  -c: Base configuration file for Olla (relative to project root or absolute path)"
                echo "  -h: Show this help message"
                echo ""
                echo "Examples:"
                echo "  $0 -c config.yaml"
                echo "  $0 -c config.local.yaml"
                echo "  $0 -c config/config.local.yaml"
                echo "  $0 -c ../../../config/config.local.yaml"
                echo "  $0 -c /absolute/path/to/config.yaml"
                exit 0
                ;;
            \? )
                print_color "$RED" "Invalid option: -$OPTARG"
                exit 1
                ;;
        esac
    done
    
    if [[ -z "$BASE_CONFIG" ]]; then
        print_color "$RED" "ERROR: Configuration file required!"
        echo "Usage: $0 -c <config_file>"
        exit 1
    fi
    
    # Handle config path resolution
    if [[ ! "$BASE_CONFIG" = /* ]]; then
        # Not an absolute path, try to resolve it
        # First check if it exists relative to current directory
        if [[ -f "$BASE_CONFIG" ]]; then
            BASE_CONFIG="$(cd "$(dirname "$BASE_CONFIG")" && pwd)/$(basename "$BASE_CONFIG")"
        # Then check relative to project root
        elif [[ -f "$PROJECT_ROOT/$BASE_CONFIG" ]]; then
            BASE_CONFIG="$PROJECT_ROOT/$BASE_CONFIG"
        # Check in config directory
        elif [[ -f "$PROJECT_ROOT/config/$BASE_CONFIG" ]]; then
            BASE_CONFIG="$PROJECT_ROOT/config/$BASE_CONFIG"
        else
            print_color "$RED" "ERROR: Configuration file not found: $BASE_CONFIG"
            print_color "$YELLOW" "Searched in:"
            print_color "$GREY" "  - Current directory: $(pwd)"
            print_color "$GREY" "  - Project root: $PROJECT_ROOT"
            print_color "$GREY" "  - Config directory: $PROJECT_ROOT/config"
            exit 1
        fi
    fi
    
    if [[ ! -f "$BASE_CONFIG" ]]; then
        print_color "$RED" "ERROR: Configuration file not found: $BASE_CONFIG"
        exit 1
    fi
    
    print_color "$GREEN" "Using configuration: $BASE_CONFIG"
}

