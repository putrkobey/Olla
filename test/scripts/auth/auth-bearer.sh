#!/usr/bin/env bash
# auth-bearer.sh — proves Olla injects bearer tokens on outbound requests.
#
# Happy path: correct token reaches the mock backend → 200.
# Failure path: wrong token in config → mock backend rejects it → Olla returns
# a non-200 (502 or 401 propagated).
#
# Requires: go, curl, bash 4+
# Does NOT require Docker / AIMock.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

# shellcheck source=lib.sh
source "$SCRIPT_DIR/lib.sh"

OLLA_PORT="${OLLA_PORT:-40115}"
BACKEND_PORT="${BACKEND_PORT:-19910}"
OLLA_URL="http://127.0.0.1:${OLLA_PORT}"
BACKEND_URL="http://127.0.0.1:${BACKEND_PORT}"
OLLA_LOG="${TMPDIR:-/tmp}/olla-auth-bearer.log"

GOOD_TOKEN="test-token-bearer-abc123"

OLLA_PID=""
BACKEND_PID=""

cleanup() {
    kill_proc "$OLLA_PID"
    kill_proc "$BACKEND_PID"
}
trap cleanup EXIT INT TERM

echo "=== auth-bearer: outbound bearer token injection ==="
echo "Backend: ${BACKEND_URL}  Olla: ${OLLA_URL}"
echo

# ── start mock backend ────────────────────────────────────────────────────────
free_port "$BACKEND_PORT"
go run "$REPO_ROOT/test/cmd/mockbackend" \
    --addr "127.0.0.1:${BACKEND_PORT}" \
    --require-header "Authorization" \
    --require-value "Bearer ${GOOD_TOKEN}" \
    >"${TMPDIR:-/tmp}/mockbackend-bearer.log" 2>&1 &
BACKEND_PID=$!
wait_for_mockbackend "$BACKEND_URL" 15

# ── write a per-run config with the correct token ────────────────────────────
CONFIG=$(mktemp "${TMPDIR:-/tmp}/olla-auth-bearer-XXXXXX.yaml")
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
  profile: "auto"
  load_balancer: "priority"
  connection_timeout: 5s
  response_timeout: 10s
  read_timeout: 10s
discovery:
  type: "static"
  refresh_interval: 10s
  static:
    endpoints:
      - url: "${BACKEND_URL}"
        name: "mock-bearer"
        type: "openai-compatible"
        priority: 100
        model_url: "/v1/models"
        health_check_url: "/v1/models"
        check_interval: 5s
        check_timeout: 2s
        auth:
          type: bearer
          token: "${GOOD_TOKEN}"
  model_discovery:
    enabled: false
model_registry:
  type: "memory"
