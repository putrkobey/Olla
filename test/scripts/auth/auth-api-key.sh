#!/usr/bin/env bash
# auth-api-key.sh — proves Olla injects api_key credentials on outbound requests.
#
# Happy path: correct key in the configured header → 200.
# Failure path: wrong key → mock backend returns 401 → Olla surfaces non-200.
#
# Requires: go, curl, bash 4+
# Does NOT require Docker / AIMock.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

# shellcheck source=lib.sh
source "$SCRIPT_DIR/lib.sh"

OLLA_PORT="${OLLA_PORT:-40116}"
BACKEND_PORT="${BACKEND_PORT:-19911}"
OLLA_URL="http://127.0.0.1:${OLLA_PORT}"
BACKEND_URL="http://127.0.0.1:${BACKEND_PORT}"
OLLA_LOG="${TMPDIR:-/tmp}/olla-auth-apikey.log"

GOOD_KEY="sk-apikey-test-abc123"
KEY_HEADER="X-Api-Key"

OLLA_PID=""
