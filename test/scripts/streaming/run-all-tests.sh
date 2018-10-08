#!/bin/bash
# Run all streaming tests

set -e

echo "===================================="
echo "    Running All Streaming Tests     "
echo "===================================="
echo

# Activate virtual environment if it exists
if [ -d "../.venv" ]; then
    if [ -f "../.venv/Scripts/activate" ]; then
        # Windows
        source ../.venv/Scripts/activate
    elif [ -f "../.venv/bin/activate" ]; then
        # Unix/macOS
        source ../.venv/bin/activate
    fi
fi
