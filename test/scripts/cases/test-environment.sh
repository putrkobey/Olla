#!/bin/bash
# Test script to verify environment setup

set -euo pipefail

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Source common functions
source "$SCRIPT_DIR/_common.sh"

print_header "Environment Test"

# Show paths
print_section "Path Information"
echo "Script Directory: $SCRIPT_DIR"
echo "Project Root: $PROJECT_ROOT"
echo "Current Directory: $(pwd)"
