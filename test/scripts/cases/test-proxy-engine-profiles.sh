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


# Function to create test configuration for engine and profile
create_engine_profile_config() {
    local engine=$1
    local profile=$2
    
    print_color "$CYAN" "Creating config: engine=$engine, profile=$profile, host=localhost, port=$TEST_PORT"
    
    # Use the create_test_config function from _olla.sh with yq modifications
    if command_exists yq; then
        create_test_config "$BASE_CONFIG" ".proxy.engine = \"$engine\" | .proxy.profile = \"$profile\" | .server.host = \"localhost\" | .server.port = $TEST_PORT"
    else
        # Use awk to modify YAML
        print_color "$YELLOW" "Using awk to modify YAML configuration"
        cp "$BASE_CONFIG" "$PROJECT_ROOT/$TEST_CONFIG"
        
        # Create a temporary file with all modifications
        awk -v engine="$engine" -v profile="$profile" -v port="$TEST_PORT" '
        BEGIN { in_server = 0; in_proxy = 0 }
        
        # Handle server section
        /^server:/ { 
            in_server = 1
            in_proxy = 0
            print
            next
        }
        
        # Handle proxy section
        /^proxy:/ { 
            in_proxy = 1
            in_server = 0
            print
            next
        }
        
        # Reset section flags when we hit a new top-level key
        /^[a-zA-Z]/ && !/^[[:space:]]/ {
            in_server = 0
            in_proxy = 0
        }
        
        # Update values in server section
        in_server && /^[[:space:]]+host:/ {
            sub(/host:.*/, "host: \"localhost\"")
        }
        in_server && /^[[:space:]]+port:/ {
            sub(/port:.*/, "port: " port)
        }
        
        # Update values in proxy section
        in_proxy && /^[[:space:]]+engine:/ {
            sub(/engine:.*/, "engine: \"" engine "\"")
        }
        in_proxy && /^[[:space:]]+profile:/ {
            sub(/profile:.*/, "profile: \"" profile "\"")
        }
        
        { print }
        ' "$PROJECT_ROOT/$TEST_CONFIG" > "$PROJECT_ROOT/$TEST_CONFIG.tmp" && \
        mv "$PROJECT_ROOT/$TEST_CONFIG.tmp" "$PROJECT_ROOT/$TEST_CONFIG"
        
        # Verify the changes
        local actual_port=$(grep -A10 "^server:" "$PROJECT_ROOT/$TEST_CONFIG" | grep "port:" | head -1 | awk '{print $2}')
        local actual_engine=$(grep -A10 "^proxy:" "$PROJECT_ROOT/$TEST_CONFIG" | grep "engine:" | head -1 | sed 's/.*engine:[[:space:]]*"\?\([^"]*\)"\?.*/\1/')
        local actual_profile=$(grep -A10 "^proxy:" "$PROJECT_ROOT/$TEST_CONFIG" | grep "profile:" | head -1 | sed 's/.*profile:[[:space:]]*"\?\([^"]*\)"\?.*/\1/')
        
        print_color "$CYAN" "Configuration changes:"
        print_color "$CYAN" "  Port: $actual_port (expected: $TEST_PORT)"
        print_color "$CYAN" "  Engine: $actual_engine (expected: $engine)"
        print_color "$CYAN" "  Profile: $actual_profile (expected: $profile)"
        
        if [[ "$actual_port" == "$TEST_PORT" ]]; then
            print_color "$GREEN" "✓ Configuration updated successfully"
        else
            print_color "$RED" "ERROR: Failed to update port correctly"
        fi
        
        # Show what we created
        print_color "$CYAN" "Test configuration server section:"
        grep -A5 "^server:" "$PROJECT_ROOT/$TEST_CONFIG" 2>/dev/null | sed 's/^/  /'
        
        print_color "$CYAN" "Test configuration proxy section:"
        grep -A10 "^proxy:" "$PROJECT_ROOT/$TEST_CONFIG" 2>/dev/null | grep -E "^proxy:|engine:|profile:" | sed 's/^/  /'
    fi
}

# Function to start Olla with custom log file
start_olla_with_log() {
    local engine=$1
    local profile=$2
    local log_file="$TEST_RESULTS_DIR/olla-$engine-$profile.log"
    
    # Call the start_olla function from _olla.sh
    start_olla "$TEST_CONFIG" "$log_file"
}




# Function to run a single test configuration
run_test_case() {
    local engine=$1
    local profile=$2
    local test_start=$(date +%s)
    
    print_section "Testing: $engine engine with $profile profile"
    
    # Create test case directory structure
    local test_case_dir="$TEST_RESULTS_DIR/$engine-$profile"
    mkdir -p "$test_case_dir/config"
    mkdir -p "$test_case_dir/logs"
    
    # Create test configuration
    if ! create_engine_profile_config "$engine" "$profile"; then
        print_color "$RED" "Failed to create configuration for $engine/$profile"
        return 1
    fi
    
    # Copy the test configuration to the test case folder
    cp "$PROJECT_ROOT/$TEST_CONFIG" "$test_case_dir/config/config.yaml"
    print_color "$CYAN" "Saved test config to: $test_case_dir/config/config.yaml"
    
    # Start Olla with the log in the test case directory
    print_color "$YELLOW" "Starting Olla with $engine/$profile configuration..."
    local log_file="$test_case_dir/logs/olla.log"
    if ! start_olla "$TEST_CONFIG" "$log_file"; then
        print_color "$RED" "Failed to start Olla for $engine/$profile"
        return 1
    fi
    
    # Check for phi model
    print_color "$YELLOW" "Checking for phi models..."
    if ! check_phi_models; then
        print_color "$RED" "No phi models available, skipping tests"
        stop_olla
        return 1
    fi
    
    # Run streaming tests
    print_color "$YELLOW" "Running streaming test suite..."
    if run_all_streaming_tests "$test_case_dir/logs"; then
        print_color "$GREEN" "All streaming tests passed"
    else
        print_color "$YELLOW" "Some streaming tests failed (check logs)"
    fi
    
    # Stop Olla
    print_color "$YELLOW" "Stopping Olla..."
    stop_olla
    
    # Calculate test duration
    local test_end=$(date +%s)
    local duration=$((test_end - test_start))
    
    # Create a summary file for this test case
    {
        echo "Test Case: $engine/$profile"
        echo "Duration: ${duration}s"
        echo "Binary: ../bin/$(ls "$TEST_RESULTS_DIR/bin" | head -1)"
        echo "Config: config.yaml"
        echo "Port: $TEST_PORT"
        echo "Date: $(date)"
    } > "$test_case_dir/summary.txt"
    
    print_color "$GREEN" "✓ Completed $engine/$profile in ${duration}s"
    print_color "$GREY" "  Test case directory: $test_case_dir/"
    
    # Small delay to ensure clean shutdown
    sleep 2
}

# Function to generate summary report
generate_summary() {
    local report_file="$TEST_RESULTS_DIR/summary.txt"
    
    print_header "Test Summary"
