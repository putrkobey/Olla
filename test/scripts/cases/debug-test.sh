#!/bin/bash
# Debug version of the test script

set -euo pipefail

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Source common functions
source "$SCRIPT_DIR/_common.sh"
source "$SCRIPT_DIR/_olla.sh"

echo "Script directory: $SCRIPT_DIR"
echo "Project root: $PROJECT_ROOT"
echo ""

# Test matrix
PROXY_ENGINES=("sherpa" "olla")
PROXY_PROFILES=("auto" "standard" "streaming")

