#!/usr/bin/env bash
# Stack Manager base-OS update helper (Debian / Ubuntu).
#
# Runs apt package management on the HOST. Like the GPU helper, the Stack
# Manager server invokes this inside the host namespaces via a privileged
# chroot /host container, because the app itself runs in a container.
#
# Usage:
#   stack-manager-os.sh status            # apt-get update + list upgradable
#   stack-manager-os.sh upgrade-start     # start a detached upgrade unit
#   stack-manager-os.sh upgrade-status    # report detached upgrade state/log
#   stack-manager-os.sh autoremove        # remove unused packages only
#   stack-manager-os.sh search <term>     # apt-cache search (read-only)
#   stack-manager-os.sh install <pkg>     # apt-get install one package
#
# The server validates <term>/<pkg> before calling, but this script also
# refuses anything outside a safe character set as defense in depth, and always
# passes them to apt as arguments (never through a shell).
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

state_dir=/var/lib/stack-manager/os-update
state_file=${state_dir}/status
log_file=${state_dir}/upgrade.log
lock_file=${state_dir}/upgrade.lock
unit_name=stack-manager-os-upgrade.service

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

write_upgrade_state() {
  local state=$1 exit_code=${2:-} started_at=${3:-} finished_at=${4:-}
  mkdir -p "${state_dir}"
  chmod 700 "${state_dir}"
  local tmp
  tmp=$(mktemp "${state_dir}/status.XXXXXX")
  {
    printf 'state=%s\n' "${state}"
    printf 'exit_code=%s\n' "${exit_code}"
    printf 'started_at=%s\n' "${started_at}"
    printf 'finished_at=%s\n' "${finished_at}"
  } >"${tmp}"
  chmod 600 "${tmp}"
  mv -f "${tmp}" "${state_file}"
}

cmd_upgrade_run() {
  require_root; require_apt
  mkdir -p "${state_dir}"
  chmod 700 "${state_dir}"
  exec 9>"${lock_file}"
  flock -n 9 || die "another OS upgrade is already running"
  local started_at finished_at exit_code
  started_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  : >"${log_file}"
  chmod 600 "${log_file}"
  write_upgrade_state running '' "${started_at}" ''
  set +e
  ( set -e; cmd_upgrade ) >>"${log_file}" 2>&1
  exit_code=$?
  set -e
  finished_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  if [ "${exit_code}" -eq 0 ]; then
    write_upgrade_state completed "${exit_code}" "${started_at}" "${finished_at}"
  else
    write_upgrade_state failed "${exit_code}" "${started_at}" "${finished_at}"
  fi
  return "${exit_code}"
}

cmd_upgrade_start() {
  require_root; require_apt
  command -v systemd-run >/dev/null 2>&1 || die "systemd-run not found"
  mkdir -p "${state_dir}"
  chmod 700 "${state_dir}"
  if systemctl is-active --quiet "${unit_name}"; then
    die "an OS upgrade is already running"
  fi
  write_upgrade_state queued '' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" ''
  if ! systemd-run --quiet --collect --no-block --unit="${unit_name%.service}" \
    /usr/local/sbin/stack-manager-os upgrade-run; then
    write_upgrade_state failed 1 "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    die "failed to start detached OS upgrade unit"
  fi
  log "detached upgrade started as ${unit_name}"
}

cmd_upgrade_status() {
  require_root
  if [ -f "${state_file}" ]; then
    cat "${state_file}"
  else
    printf 'state=idle\nexit_code=\nstarted_at=\nfinished_at=\n'
  fi
  echo '--- output ---'
  if [ -f "${log_file}" ]; then
    tail -200 "${log_file}"
  fi
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
  upgrade) cmd_upgrade_start ;;
  upgrade-start) cmd_upgrade_start ;;
  upgrade-run) cmd_upgrade_run ;;
  upgrade-status) cmd_upgrade_status ;;
  autoremove) cmd_autoremove ;;
  search) shift; cmd_search "${1:-}" ;;
  install) shift; cmd_install "${1:-}" ;;
  *) die "unknown command '${1:-}' (use status|upgrade-start|upgrade-status|autoremove|search|install)" ;;
esac
