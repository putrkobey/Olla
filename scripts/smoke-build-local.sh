#!/usr/bin/env bash
# Smoke check for `make build-local`: it must succeed from a pristine build/ and
# again over a build/ that already holds junk, and leave a runnable build/olla
# plus the staged build/docker-config.yaml. This guards the PR #239 regression
# where --output pointed at the build/ directory the goreleaser before-hooks
# create. Needs goreleaser and bun (the hooks run `make build-web`).
#
# Not wired into CI or `make ready`: it is slow, needs goreleaser and bun, and
# `make ci` already covers the plain build. It replaces ./build (gitignored
# build output) wholesale and removes it again on exit, so it refuses to run
# until you confirm with OLLA_SMOKE_OK=1. The script itself only deletes
# ./build, but `make build-local` runs `goreleaser --clean`, which also clears
# ./dist, as it always has.
set -euo pipefail

# A CDPATH match would make `cd scripts/..` land in another tree, and the
# cleanup below would then delete the wrong build/.
unset CDPATH
cd "$(dirname "$0")/.."

[[ -f makefile && -f go.mod ]] || { echo "smoke-build-local: $(pwd) is not the olla repo root" >&2; exit 2; }

if [[ "${OLLA_SMOKE_OK:-}" != "1" ]]; then
  echo "smoke-build-local: this deletes $(pwd)/build. Set OLLA_SMOKE_OK=1 to confirm:" >&2
  echo "  OLLA_SMOKE_OK=1 make smoke-build-local" >&2
  exit 2
fi

