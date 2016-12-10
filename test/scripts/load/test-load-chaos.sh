#!/bin/bash

# Olla Chaos Load Test Script (The Benny KaosTheory Edition)
# Simulates partial-stream readers to stress proxy with early disconnects.
# Useful for stress testing connection handling, stream cancellation and 
# robustness under mid-stream disconnects.
#
# Supports a --chaos flag that introduces randomised read sizes, malformed 
# JSON payloads, and jitter between requests.
# 
# Run this alone or along with test-load-limits.sh to simulate real-world
# scenarios where clients may disconnect mid-stream or send malformed data.
####
# Requirements: jq, curl
####
# Usage: ./test-load-stream.sh [duration] [concurrency] [--chaos]
# - duration: Duration of the test in seconds (default: 60)
# - concurrency: Number of concurrent workers (default: 10)
# - --chaos: Enable chaos mode with random read bytes and malformed data
#
# Example: ./test-load-stream.sh 60 10 # Defaults: 60 seconds duration, 10 concurrent workers

set -euo pipefail
