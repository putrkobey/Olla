#!/usr/bin/env bash
# auth-headers-only.sh — proves Olla injects arbitrary static headers on
# outbound requests when only the headers: map is configured (no auth: block).
#
# The mock backend enforces a custom header, confirming that custom headers
# travel to the backend even when no structured auth block is present.
#
# Requires: go, curl, bash 4+
# Does NOT require Docker / AIMock.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

# shellcheck source=lib.sh
source "$SCRIPT_DIR/lib.sh"

OLLA_PORT="${OLLA_PORT:-40118}"
BACKEND_PORT="${BACKEND_PORT:-19913}"
OLLA_URL="http://127.0.0.1:${OLLA_PORT}"
BACKEND_URL="http://127.0.0.1:${BACKEND_PORT}"
OLLA_LOG="${TMPDIR:-/tmp}/olla-auth-headers.log"

CUSTOM_HEADER="X-Tenant-ID"
CUSTOM_VALUE="tenant-abc"

OLLA_PID=""
