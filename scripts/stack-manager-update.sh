#!/usr/bin/env bash
# Stack Manager self-update helper.
#
# Updates the controller's own deployment: fetch latest, hard-reset to the
# tracked upstream (so an ad-hoc-dirtied deploy tree can't block the update),
# and rebuild + recreate the stack. Like the GPU/OS helpers it runs on the HOST
# via a privileged chroot /host container; the rebuild is DETACHED (setsid) so
# it survives this helper container exiting and the server/web containers
# restarting mid-update.
#
# Usage:
#   stack-manager-update.sh status    # branch, local vs upstream, commits behind
#   stack-manager-update.sh update    # fetch + reset --hard + up -d --build (detached)
set -euo pipefail

log() { printf '[self-update] %s\n' "$*"; }

# Find the host directory of the stack-manager compose project by reading the
# server container's compose working_dir label.
find_dir() {
  local cid
  cid=$(docker ps --filter 'label=com.docker.compose.service=server' --filter 'name=stack-manager' -q 2>/dev/null | head -1)
  [ -n "$cid" ] || cid=$(docker ps --filter 'name=stack-manager-server' -q 2>/dev/null | head -1)
  [ -n "$cid" ] || return 1
  docker inspect "$cid" --format '{{index .Config.Labels "com.docker.compose.project.working_dir"}}' 2>/dev/null
}

cmd_status() {
  local dir; dir=$(find_dir) || { echo 'error=cannot locate stack-manager deploy dir'; return 0; }
  [ -n "$dir" ] && cd "$dir" 2>/dev/null || { echo "error=deploy dir not found: $dir"; return 0; }
  git config --global --add safe.directory "$dir" 2>/dev/null || true
  git fetch --quiet origin 2>/dev/null || true
  echo "dir=$dir"
  echo "branch=$(git rev-parse --abbrev-ref HEAD 2>/dev/null)"
  echo "local=$(git rev-parse --short HEAD 2>/dev/null)"
  echo "remote=$(git rev-parse --short '@{u}' 2>/dev/null || echo unknown)"
  echo "behind=$(git rev-list --count 'HEAD..@{u}' 2>/dev/null || echo 0)"
}

cmd_update() {
  local dir; dir=$(find_dir) || { echo 'error=cannot locate stack-manager deploy dir'; return 1; }
  [ -n "$dir" ] || { echo 'error=empty deploy dir'; return 1; }
  local logf=/var/log/stack-manager-update.log
  log "starting detached self-update in $dir (log: $logf)"
  # Detach fully: new session, reparented to init when this helper exits, so the
  # (minutes-long) rebuild keeps going while server/web restart.
  setsid nohup bash -c "
    cd '$dir' || exit 1
    git config --global --add safe.directory '$dir' 2>/dev/null || true
    echo \"=== \$(date -u) self-update starting ===\"
    git fetch origin || exit 1
    git reset --hard '@{u}' || exit 1
    export VITE_GIT_SHA=\$(git rev-parse --short HEAD)
    docker compose --env-file .env up -d --build --remove-orphans
    echo \"=== \$(date -u) self-update done ===\"
  " >"$logf" 2>&1 &
  echo "update started (detached)"
}

case "${1:-status}" in
  status) cmd_status ;;
  update) cmd_update ;;
  *) echo "unknown command '${1:-}' (use status|update)" >&2; exit 1 ;;
esac
