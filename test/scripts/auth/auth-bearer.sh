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
