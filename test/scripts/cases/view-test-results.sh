#!/bin/bash
# view-test-results.sh
# View results from the latest test run

set -euo pipefail

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Source common functions
source "$SCRIPT_DIR/_common.sh"

# Find the latest test results directory
find_latest_results() {
    local results_base="$PROJECT_ROOT/test/results"
    if [[ ! -d "$results_base" ]]; then
        return 1
    fi
    
    local latest=$(ls -dt "$results_base"/test-results-* 2>/dev/null | head -1)
    if [[ -n "$latest" ]]; then
        echo "$latest"
    else
        return 1
    fi
}

# Main execution
