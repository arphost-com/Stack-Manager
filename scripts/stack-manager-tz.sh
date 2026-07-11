#!/usr/bin/env bash
# Stack Manager host-timezone helper.
#
# Gets/sets the HOST system timezone. Debian and Ubuntu both run systemd, so
# `timedatectl set-timezone` is the primary path; a symlink fallback covers any
# host without timedatectl. Runs on the HOST via the privileged chroot /host
# helper container (same pattern as the GPU/OS/self-update helpers).
#
# Usage:
#   stack-manager-tz.sh get            # prints tz=<current host zone>
#   stack-manager-tz.sh set <Zone>     # e.g. set America/New_York
set -euo pipefail

# Validate against the installed zoneinfo DB and a strict charset so nothing
# shell-unsafe is ever passed onward.
valid_tz() {
  local tz="$1"
  [ -n "$tz" ] || return 1
  printf '%s' "$tz" | grep -Eq '^[A-Za-z0-9][A-Za-z0-9_+/-]*$' || return 1
  [ -f "/usr/share/zoneinfo/$tz" ]
}

cmd_get() {
  if command -v timedatectl >/dev/null 2>&1; then
    local tz; tz=$(timedatectl show -p Timezone --value 2>/dev/null || true)
    [ -n "$tz" ] && { echo "tz=$tz"; return 0; }
  fi
  if [ -f /etc/timezone ]; then
    echo "tz=$(cat /etc/timezone)"; return 0
  fi
  if [ -L /etc/localtime ]; then
    echo "tz=$(readlink -f /etc/localtime | sed 's#.*/zoneinfo/##')"; return 0
  fi
  echo "tz=UTC"
}

cmd_set() {
  local tz="${1:-}"
  valid_tz "$tz" || { echo "error=invalid timezone: $tz"; exit 1; }
  if command -v timedatectl >/dev/null 2>&1; then
    if timedatectl set-timezone "$tz" 2>/dev/null; then
      echo "ok=$tz"; return 0
    fi
  fi
  # Fallback for hosts without timedatectl.
  ln -sf "/usr/share/zoneinfo/$tz" /etc/localtime
  printf '%s\n' "$tz" > /etc/timezone
  echo "ok=$tz"
}

case "${1:-get}" in
  get) cmd_get ;;
  set) shift; cmd_set "${1:-}" ;;
  *) echo "usage: $0 get|set <tz>" >&2; exit 1 ;;
esac
