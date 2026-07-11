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

docker compose --env-file .env up -d --build "$@"

# Install/refresh the host helper scripts (GPU / OS updates / self-update /
# firewall) so their Settings panels work without a manual SSH step. Deploy
# users have passwordless sudo; failures here are non-fatal.
for h in gpu os update csf tz; do
  src="scripts/stack-manager-${h}.sh"
  [ -f "${src}" ] || continue
  if sudo install -m 750 "${src}" "/usr/local/sbin/stack-manager-${h}" 2>/dev/null; then
    echo "[deploy] installed host helper: stack-manager-${h}"
  fi
done

# Stamp the running server's version into the DB so the footer reflects the
# deployed commit (read from /system/info), independent of the frontend build.
base="$(sed -n 's/.*"version": *"\([^"]*\)".*/\1/p' web/package.json | head -1)"
version="${base}+${VITE_GIT_SHA:-nogit}"
key="$(grep -E '^API_KEY=' .env | cut -d= -f2- || true)"
port="$(grep -E '^WEB_SSL_PORT=' .env | cut -d= -f2- || true)"; port="${port:-8993}"
if [ -n "${key}" ]; then
  echo "[deploy] waiting for the stack to come up to stamp version ${version}…"
  i=0
  while [ "${i}" -lt 30 ]; do
    curl -sk -m5 -o /dev/null "https://localhost:${port}/health" 2>/dev/null && break
    i=$((i + 1)); sleep 2
  done
  curl -sk -m10 -H "X-API-Key: ${key}" -H 'Content-Type: application/json' \
    -X PUT "https://localhost:${port}/api/v1/system/info" \
    -d "{\"app_version\":\"${version}\"}" >/dev/null 2>&1 \
    && echo "[deploy] stamped version ${version}" \
    || echo "[deploy] version stamp skipped (server not reachable yet)"
fi
