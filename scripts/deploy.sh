#!/usr/bin/env bash
# Rebuild and restart the Stack Manager stack, stamping the UI footer version
# with the current commit SHA. Use this instead of a bare `docker compose up`
# so the version reflects the deployed code (the Docker build context has no
# .git, so the SHA must be passed in from the host, where git is available).
#
# Equivalent to `make docker-up` but needs no `make`.
#
# Usage: ./scripts/deploy.sh            # from the repo root
set -euo pipefail
cd "$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"

VITE_GIT_SHA="$(git rev-parse --short HEAD 2>/dev/null || echo '')"
export VITE_GIT_SHA
echo "[deploy] building stack at version $(cat web/package.json | sed -n 's/.*\"version\": *\"\([^\"]*\)\".*/\1/p' | head -1)+${VITE_GIT_SHA:-nogit}"

# Prepare state (idempotent: fills .env, fixes ownership) if the script exists.
if [ -x ./scripts/prepare-state.sh ]; then
  ./scripts/prepare-state.sh .env >/dev/null 2>&1 || true
fi

exec docker compose --env-file .env up -d --build "$@"
