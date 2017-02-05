#!/usr/bin/env bash
# auth-env-fatal.sh — proves Olla refuses to start when a config references an
# environment variable that is not set.
#
# This is the most important safety test: a missing credential must produce a
# loud startup failure, not a silent zero-value that sends unauthenticated
# requests to production backends.
#
# The script asserts:
#   1. Olla exits non-zero within a few seconds (fatal startup error).
#   2. The error output mentions the endpoint name so the operator knows which
#      endpoint's config is broken.
#
# Requires: go, bash 4+
# Does NOT require Docker / AIMock or a running backend.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

# shellcheck source=lib.sh
source "$SCRIPT_DIR/lib.sh"

OLLA_PORT="${OLLA_PORT:-40119}"
# Port chosen to avoid collision with other auth test scripts; a backend is
# not actually required because Olla must fail before it ever connects.
BACKEND_PORT="${BACKEND_PORT:-19914}"
OLLA_LOG="${TMPDIR:-/tmp}/olla-auth-env-fatal.log"

# Deliberately unset — must NOT be present when Olla starts.
# Use seconds + PID for uniqueness; %N (nanoseconds) is GNU-only and fails on BSD/macOS.
MISSING_VAR="OLLA_TEST_MISSING_TOKEN_$(date +%s)_$$"
unset "$MISSING_VAR" 2>/dev/null || true

ENDPOINT_NAME="mock-env-fatal"

echo "=== auth-env-fatal: missing env var must abort startup ==="
echo "Missing var: \${${MISSING_VAR}}"
echo "Endpoint name in config: ${ENDPOINT_NAME}"
echo

CONFIG=$(mktemp "${TMPDIR:-/tmp}/olla-auth-env-fatal-XXXXXX.yaml")
cat >"$CONFIG" <<YAML
server:
