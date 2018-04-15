#!/bin/bash
# _olla.sh
# Olla-specific helper functions for test scripts

# Global variable for Olla PID
OLLA_PID=""

# Function to get git version info for binary naming
get_git_version() {
    local current_dir=$(pwd)
    cd "$PROJECT_ROOT"
    
    # Get short hash
    local git_hash=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
    
    # Check if working directory is clean
    if [[ -n "$(git status --porcelain 2>/dev/null)" ]]; then
        echo "olla-${git_hash}-wip"
    else
        echo "olla-${git_hash}"
    fi
    
    cd "$current_dir"
}

# Function to build Olla
build_olla() {
    print_color "$YELLOW" "Building Olla from source..."
    
    # Save current directory
    local current_dir=$(pwd)
    
    # Change to project root
    cd "$PROJECT_ROOT"
    
    # Clean previous build
    if [[ -f "$OLLA_BINARY" ]]; then
        rm -f "$OLLA_BINARY"
    fi
    
    # Build with go
    if command_exists go; then
        print_color "$CYAN" "Building with Go from: $(pwd)"
        
        # Ensure we're in the right directory and use proper go build command
        if [[ -f "main.go" ]]; then
            go build -o olla .
        elif [[ -f "cmd/olla/main.go" ]]; then
            go build -o olla ./cmd/olla
        else
            print_color "$RED" "ERROR: Cannot find main.go or cmd/olla/main.go from $(pwd)"
            cd "$current_dir"
            return 1
        fi
        
        if [[ -f "$OLLA_BINARY" ]]; then
            print_color "$GREEN" "✓ Olla built successfully"
            cd "$current_dir"
            return 0
        else
            print_color "$RED" "ERROR: Failed to build Olla"
            cd "$current_dir"
            return 1
        fi
    else
        print_color "$RED" "ERROR: Go is not installed"
        print_color "$YELLOW" "Please install Go from https://golang.org/dl/"
        cd "$current_dir"
        return 1
    fi
}

# Function to create test configuration from base config
create_test_config() {
    local base_config=$1
    local modifications=$2  # String of yq modifications or sed commands
    
    if [[ ! -f "$base_config" ]]; then
        print_color "$RED" "ERROR: Base configuration not found: $base_config"
        return 1
    fi
    
    print_color "$CYAN" "Creating test configuration..."
    
    # Ensure config directory exists
    mkdir -p "$PROJECT_ROOT/config"
    
    # Use yq if available for reliable YAML manipulation
    if command_exists yq; then
        eval "yq eval '$modifications' \"$base_config\"" > "$PROJECT_ROOT/$TEST_CONFIG"
    else
        # Fallback to sed (less reliable but works for simple cases)
        cp "$base_config" "$PROJECT_ROOT/$TEST_CONFIG"
        
        # Apply modifications using sed
        # This is a simplified approach - real implementation would need better parsing
        print_color "$YELLOW" "Warning: Using sed for YAML manipulation (install yq for better reliability)"
        eval "$modifications"
    fi
    
    if [[ -f "$PROJECT_ROOT/$TEST_CONFIG" ]]; then
        print_color "$GREEN" "✓ Test configuration created at: $PROJECT_ROOT/$TEST_CONFIG"
        return 0
    else
        print_color "$RED" "ERROR: Failed to create test configuration"
        return 1
    fi
}

# Function to start Olla with configuration
start_olla() {
    local config_file="${1:-$TEST_CONFIG}"
    local log_file="${2:-olla.log}"
    
    print_color "$YELLOW" "Starting Olla..."
    
    # Save current directory
    local current_dir=$(pwd)
    cd "$PROJECT_ROOT"
    
    # Check if port is available
    local port="${TEST_PORT:-40114}"
    if ! is_port_available $port; then
        print_color "$RED" "ERROR: Port $port is already in use"
        cd "$current_dir"
        return 1
    fi
    
    # Ensure binary exists
    if [[ ! -f "$OLLA_BINARY" ]]; then
        print_color "$RED" "ERROR: Olla binary not found at $OLLA_BINARY"
        cd "$current_dir"
        return 1
    fi
    
    # Start Olla in background
    local port="${TEST_PORT:-40114}"
    
    # Ensure config file is absolute path
    if [[ ! "$config_file" = /* ]]; then
        config_file="$PROJECT_ROOT/$config_file"
    fi
    
    print_color "$CYAN" "Config file: $config_file"
    print_color "$CYAN" "Expected port: $port"
    
    # Verify config has correct port
    if [[ -f "$config_file" ]]; then
        local config_port=$(grep -E "^\s*port:" "$config_file" | awk '{print $2}')
        print_color "$CYAN" "Port in config file: $config_port"
        if [[ "$config_port" != "$port" ]]; then
            print_color "$YELLOW" "WARNING: Config port ($config_port) doesn't match expected port ($port)"
        fi
    fi
    
