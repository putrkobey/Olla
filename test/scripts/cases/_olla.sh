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
    
