#!/usr/bin/env bash
# Shared helpers for auth test scripts.
# Source this file; do not execute it directly.

# Colour codes — match the project's ANSI palette
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RESET='\033[0m'

TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=()

pass() {
    local name="$1"
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    PASSED_TESTS=$((PASSED_TESTS + 1))
    printf "${GREEN}PASS${RESET}: %s\n" "$name"
}

fail() {
    local name="$1"
    local reason="${2:-}"
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    FAILED_TESTS+=("$name")
    if [[ -n "$reason" ]]; then
        printf "${RED}FAIL${RESET}: %s (%s)\n" "$name" "$reason" >&2
    else
        printf "${RED}FAIL${RESET}: %s\n" "$name" >&2
    fi
}

# summarise prints the final PASS/FAIL summary line and exits with the
# appropriate code.  Call from a trap or at the end of each script.
summarise() {
    local script_name="${1:-test}"
    echo
    echo "Results: ${PASSED_TESTS}/${TOTAL_TESTS} passed"
    if [[ ${#FAILED_TESTS[@]} -eq 0 ]]; then
        printf "${GREEN}PASS${RESET}: %s\n" "$script_name"
        return 0
    else
        printf "${RED}FAIL${RESET}: %s\n" "$script_name"
        return 1
    fi
}

# wait_for_url polls until the URL returns HTTP 200 or the timeout is reached.
wait_for_url() {
    local url="$1"
    local timeout="${2:-15}"
    local attempt=0
    until curl -sf --max-time 2 "$url" >/dev/null 2>&1; do
        attempt=$((attempt + 1))
        if [[ $attempt -ge $timeout ]]; then
            printf "${RED}ERROR${RESET}: %s did not become available within %ss\n" "$url" "$timeout" >&2
            return 1
        fi
        sleep 1
    done
}

# wait_for_mockbackend polls the unauthenticated /health endpoint on the mock
# backend. Use this instead of wait_for_url for mockbackend instances because
# the auth-enforced /v1/models path returns 401 which curl -sf treats as failure.
wait_for_mockbackend() {
    local base_url="$1"
