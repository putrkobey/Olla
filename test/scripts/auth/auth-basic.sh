#!/usr/bin/env bash
# auth-basic.sh — proves Olla injects HTTP Basic auth credentials on outbound
# requests.
#
# The mock backend enforces the pre-encoded "Authorization: Basic <b64>" value.
# The script computes the expected value once so we can assert it without
# duplicating encoding logic.
#
# Requires: go, curl, bash 4+, base64 (coreutils or macOS)
# Does NOT require Docker / AIMock.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

# shellcheck source=lib.sh
source "$SCRIPT_DIR/lib.sh"

OLLA_PORT="${OLLA_PORT:-40117}"
BACKEND_PORT="${BACKEND_PORT:-19912}"
OLLA_URL="http://127.0.0.1:${OLLA_PORT}"
BACKEND_URL="http://127.0.0.1:${BACKEND_PORT}"
OLLA_LOG="${TMPDIR:-/tmp}/olla-auth-basic.log"

USERNAME="testuser"
PASSWORD="testpass99"
# Pre-compute the exact Authorization header value Olla will send.
ENCODED=$(printf '%s:%s' "$USERNAME" "$PASSWORD" | base64 | tr -d '\n')
EXPECTED_AUTH_VALUE="Basic ${ENCODED}"

OLLA_PID=""
BACKEND_PID=""

cleanup() {
    kill_proc "$OLLA_PID"
    kill_proc "$BACKEND_PID"
}
trap cleanup EXIT INT TERM

echo "=== auth-basic: outbound HTTP Basic auth injection ==="
echo "Backend: ${BACKEND_URL}  Olla: ${OLLA_URL}"
echo

free_port "$BACKEND_PORT"
go run "$REPO_ROOT/test/cmd/mockbackend" \
    --addr "127.0.0.1:${BACKEND_PORT}" \
    --require-header "Authorization" \
    --require-value "${EXPECTED_AUTH_VALUE}" \
    >"${TMPDIR:-/tmp}/mockbackend-basic.log" 2>&1 &
BACKEND_PID=$!
wait_for_mockbackend "$BACKEND_URL" 15

CONFIG=$(mktemp "${TMPDIR:-/tmp}/olla-auth-basic-XXXXXX.yaml")
cat >"$CONFIG" <<YAML
server:
  host: "127.0.0.1"
  port: ${OLLA_PORT}
  read_timeout: 10s
  write_timeout: 0s
  shutdown_timeout: 2s
  request_logging: false
  request_limits:
    max_body_size: 10485760
    max_header_size: 524288
  rate_limits:
    global_requests_per_minute: 0
    per_ip_requests_per_minute: 0
    health_requests_per_minute: 0
    burst_size: 50
proxy:
  engine: "olla"
