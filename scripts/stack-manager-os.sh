#!/usr/bin/env bash
# Stack Manager base-OS update helper (Debian / Ubuntu).
#
# Runs apt package management on the HOST. Like the GPU helper, the Stack
# Manager server invokes this inside the host namespaces via a privileged
# chroot /host container, because the app itself runs in a container.
#
# Usage:
#   stack-manager-os.sh status            # apt-get update + list upgradable
#   stack-manager-os.sh upgrade           # update + dist-upgrade + autoremove
#   stack-manager-os.sh autoremove        # remove unused packages only
#   stack-manager-os.sh search <term>     # apt-cache search (read-only)
#   stack-manager-os.sh install <pkg>     # apt-get install one package
#
# The server validates <term>/<pkg> before calling, but this script also
# refuses anything outside a safe character set as defense in depth, and always
# passes them to apt as arguments (never through a shell).
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

log() { printf '[os-update] %s\n' "$*"; }
die() { printf '[os-update] ERROR: %s\n' "$*" >&2; exit 1; }

require_root() { [ "$(id -u)" -eq 0 ] || die "must run as root"; }
require_apt() { command -v apt-get >/dev/null 2>&1 || die "apt-get not found (Debian/Ubuntu only)"; }

# Package/search tokens: letters, digits, and . _ + - : / ~ only.
safe_token() {
  case "$1" in
    '' ) die "empty argument" ;;
    *[!A-Za-z0-9._+:/~-]* ) die "illegal characters in '$1'" ;;
    * ) : ;;
  esac
}

cmd_status() {
  require_apt
  log "apt-get update"
  apt-get update -q
  echo "--- upgradable ---"
  # apt list is noisy on stderr ("WARNING: apt does not have a stable CLI"); keep stdout.
  apt list --upgradable 2>/dev/null | grep -v '^Listing' || true
  local n
  n=$(apt list --upgradable 2>/dev/null | grep -vc '^Listing' || true)
  echo "upgradable_count=${n}"
}

cmd_upgrade() {
  require_root; require_apt
  log "apt-get update"; apt-get update -q
  log "apt-get dist-upgrade"; apt-get -y -q dist-upgrade
  log "apt-get autoremove"; apt-get -y -q autoremove
  log "upgrade complete"
}

cmd_autoremove() {
  require_root; require_apt
  log "apt-get autoremove"; apt-get -y -q autoremove
  log "autoremove complete"
}

cmd_search() {
  require_apt
  safe_token "${1:-}"
  # Read-only; cap output so a broad term can't flood the UI.
  apt-cache search -- "$1" | head -100
}

cmd_install() {
  require_root; require_apt
  safe_token "${1:-}"
  log "apt-get install $1"
  apt-get update -q
  apt-get -y -q install -- "$1"
  log "install complete"
}

case "${1:-status}" in
  status) cmd_status ;;
  upgrade) cmd_upgrade ;;
  autoremove) cmd_autoremove ;;
  search) shift; cmd_search "${1:-}" ;;
  install) shift; cmd_install "${1:-}" ;;
  *) die "unknown command '${1:-}' (use status|upgrade|autoremove|search|install)" ;;
esac
