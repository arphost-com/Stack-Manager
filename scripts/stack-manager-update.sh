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
  echo "dir=$dir"
  git config --global --add safe.directory "$dir" 2>/dev/null || true
  # A CI/rsync-deployed host has no .git, so self-update (fetch + reset) can't
  # work — report that clearly instead of blank branch/local fields.
  if ! git rev-parse --git-dir >/dev/null 2>&1; then
    echo "vcs=none"
    echo "note=This deploy tree is not a git checkout (deployed by CI/rsync). Update it through your pipeline, not this button."
    return 0
  fi
  git fetch --quiet origin 2>/dev/null || true
  echo "branch=$(git rev-parse --abbrev-ref HEAD 2>/dev/null)"
  echo "local=$(git rev-parse --short HEAD 2>/dev/null)"
  echo "remote=$(git rev-parse --short '@{u}' 2>/dev/null || echo unknown)"
  echo "behind=$(git rev-list --count 'HEAD..@{u}' 2>/dev/null || echo 0)"
  # One change=<subject> line per pending commit (newest first), so the update
  # panel can show what the update contains.
  git log --format='change=%s' 'HEAD..@{u}' 2>/dev/null | head -50
}

cmd_update() {
  local dir; dir=$(find_dir) || { echo 'error=cannot locate stack-manager deploy dir'; return 1; }
  [ -n "$dir" ] || { echo 'error=empty deploy dir'; return 1; }
  # Refuse on a non-git (CI/rsync) tree instead of failing mid-rebuild.
  if ! git -C "$dir" rev-parse --git-dir >/dev/null 2>&1; then
    echo 'error=this deploy tree is not a git checkout (deployed by CI/rsync); update it through your pipeline, not this button'
    return 1
  fi
  local logf=/var/log/stack-manager-update.log

  # The rebuild takes minutes and RESTARTS the server/web containers, so it must
  # outlive both this helper and the containers that triggered it. This helper
  # runs inside a `docker run --rm` container; when that container exits, Docker
  # SIGKILLs its entire cgroup. setsid/nohup escape the controlling terminal but
  # NOT the cgroup, so a backgrounded child dies with the container (symptom: the
  # log shows only the start banner). systemd-run launches the rebuild as a
  # transient unit owned by the host's systemd (PID 1), fully outside this
  # container's cgroup, so it survives.
  # The script redirects its OWN stdout/stderr to the log via `exec` on line 1,
  # so callers just run `bash -c "$script"` with no outer redirect or brace
  # group. (A brace-group wrapper `{ $script ; }` breaks because $script ends in
  # a newline, leaving a bare `;` that bash rejects under systemd-run/ExecStart.)
  # Pass safe.directory inline on every git call: the systemd unit runs as root
  # with a fresh env (no HOME), so `git config --global` writes nowhere useful and
  # root would refuse the deploy tree if it's owned by another user ("dubious
  # ownership"). Inline -c needs no config file or HOME.
  local git="git -c safe.directory='$dir'"
  local script="exec >'$logf' 2>&1
    cd '$dir' || exit 1
    echo \"=== \$(date -u) self-update starting ===\"
    $git fetch origin || { echo 'ERROR: git fetch failed'; exit 1; }
    $git reset --hard '@{u}' || { echo 'ERROR: git reset failed'; exit 1; }
    # This unit runs as root, so git just wrote root-owned objects/files. Re-own
    # the tree to the deploy user (skipping the state dir, whose mariadb/redis
    # files are service-owned) so a later non-root deploy.sh doesn't break on
    # 'Operation not permitted'. No-op when the tree is already root-owned.
    ownerid=\$(stat -c '%U:%G' '$dir' 2>/dev/null || true)
    [ -n \"\$ownerid\" ] && [ \"\$ownerid\" != 'root:root' ] && find '$dir' -name .stack-manager -prune -o -user root -exec chown \"\$ownerid\" {} + 2>/dev/null || true
    export VITE_GIT_SHA=\$($git rev-parse --short HEAD)
    if [ -f .env ]; then
      if grep -q '^VITE_GIT_SHA=' .env; then
        sed -i \"s/^VITE_GIT_SHA=.*/VITE_GIT_SHA=\$VITE_GIT_SHA/\" .env
      else
        printf '\\nVITE_GIT_SHA=%s\\n' \"\$VITE_GIT_SHA\" >> .env
      fi
      [ -n \"\$ownerid\" ] && [ \"\$ownerid\" != 'root:root' ] && chown \"\$ownerid\" .env 2>/dev/null || true
    fi
    echo \"rebuilding at \$VITE_GIT_SHA\"
    docker compose --env-file .env up -d --build --remove-orphans || { echo 'ERROR: compose up failed'; exit 1; }
    echo \"=== \$(date -u) self-update done ===\""

  if command -v systemd-run >/dev/null 2>&1; then
    # Clear any prior transient unit of this name so a stale/failed one can't
    # block relaunch, then run detached under systemd (--collect auto-removes it
    # when it finishes).
    systemctl reset-failed stack-manager-selfupdate.service 2>/dev/null || true
    if systemd-run --collect --unit=stack-manager-selfupdate \
        --setenv=HOME=/root \
        --setenv=PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin \
        bash -c "$script" >/dev/null 2>&1; then
      log "starting self-update in $dir via systemd (log: $logf)"
      echo "update started (systemd transient unit)"
      return 0
    fi
    log "systemd-run unavailable/failed; falling back to setsid"
  fi

  # Fallback for non-systemd hosts: setsid detached. Less robust across the
  # helper-container exit, but the best available without systemd.
  log "starting detached self-update in $dir (log: $logf)"
  setsid nohup bash -c "$script" >/dev/null 2>&1 &
  echo "update started (detached fallback)"
}

case "${1:-status}" in
  status) cmd_status ;;
  update) cmd_update ;;
  *) echo "unknown command '${1:-}' (use status|update)" >&2; exit 1 ;;
esac
