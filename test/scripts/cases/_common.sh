#!/bin/bash
# _common.sh
# Common functions and variables for test scripts

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

# Common paths
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
TEST_CONFIG="config/config.test.local.yaml"
OLLA_BINARY="$PROJECT_ROOT/olla"

# Function to print colored output
print_color() {
    local color=$1
    local message=$2
    echo -e "${color}${message}${RESET}"
    # Force flush output
    if [[ -t 1 ]]; then
        tput sgr0 2>/dev/null || true
    fi
}

# Function to print section header
print_header() {
    local title=$1
    print_color "$PURPLE" "============================================================"
    print_color "$PURPLE" "  $title"
    print_color "$PURPLE" "============================================================"
}

# Function to print subsection
print_section() {
    local title=$1
    print_color "$WHITE" "\n$title"
    print_color "$GREY" "----------------------------------------"
}

# Function to check and activate virtual environment
check_venv() {
    # If already in a venv, use it
    if [[ -n "${VIRTUAL_ENV:-}" ]]; then
        print_color "$GREEN" "✓ Virtual environment detected: $VIRTUAL_ENV"
        return 0
    fi
    
    # Look for .venv in test/scripts directory
    local venv_path="$PROJECT_ROOT/test/scripts/.venv"
    
    if [[ -d "$venv_path" ]]; then
        print_color "$YELLOW" "Found virtual environment at: $venv_path"
        print_color "$YELLOW" "Activating virtual environment..."
        
        # Activate based on OS
        if [[ "$OSTYPE" == "msys" || "$OSTYPE" == "win32" ]]; then
