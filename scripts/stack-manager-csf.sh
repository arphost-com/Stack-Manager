#!/usr/bin/env bash
# stack-manager-csf.sh — narrow-scope root helper for driving CSF/LFD from
# the Stack Manager server container. Every operation the server needs on
# the host firewall goes through one of the subcommands below; the server
# account is granted passwordless sudo for THIS SCRIPT ONLY (see
# stack-manager-csf.sudoers.example). Nothing here forwards free-form
# arguments to a shell — inputs are validated against strict patterns.
set -euo pipefail

CSF_BIN="${STACK_MANAGER_CSF_BIN:-/usr/sbin/csf}"
CSF6_BIN="${STACK_MANAGER_CSF6_BIN:-/usr/sbin/csf6}"
CSF_ETC="${STACK_MANAGER_CSF_ETC:-/etc/csf}"
LFD_LOG="${STACK_MANAGER_LFD_LOG:-/var/log/lfd.log}"
UPSTREAM_URL="${STACK_MANAGER_CSF_URL:-https://github.com/Black-HOST/csf.git}"

usage() {
  cat >&2 <<'EOF'
Usage: stack-manager-csf <subcommand> [args...]

Discovery / status:
  version                    Print csf version + running mode.
  status                     Print status summary (installed, testing mode, iptables rules count).
  list-allow                 Print /etc/csf/csf.allow (whitelist).
  list-deny                  Print /etc/csf/csf.deny (blocklist).
  list-tempbans              Print current temporary bans (`csf -t`).
  tail-log [lines]           Print last N (default 200) lines of lfd.log.

IP management (IPv4 or IPv6; comment required):
  allow-ip <ip> <comment>    Permanently allow an IP.
  deny-ip <ip> <comment>     Permanently deny an IP.
  remove-ip <ip>             Remove <ip> from allow and deny lists.

Config:
  read-config <name>         Read one of: csf.conf, csf.allow, csf.deny, csf.ignore, csf.pignore
  write-config <name>        Read the new content from stdin. Backup created under state dir.

Lifecycle:
  restart                    csf -r (reload firewall).
  reload-lfd                 Restart the lfd daemon.
  install                    Clone + install CSF from the pinned upstream repo.
  uninstall                  Run /etc/csf/uninstall.sh.
EOF
}

log() { printf '[stack-manager-csf] %s\n' "$*" >&2; }
die() { log "ERROR: $*"; exit 2; }

# --- input validation ---------------------------------------------------------

IP4_RE='^([0-9]{1,3}\.){3}[0-9]{1,3}(/[0-9]{1,2})?$'
IP6_RE='^[0-9A-Fa-f:]+(/[0-9]{1,3})?$'
COMMENT_RE='^[A-Za-z0-9 ._@:/-]{1,120}$'
CONFIG_ALLOW='csf.conf csf.allow csf.deny csf.ignore csf.pignore'

validate_ip() {
  local ip="$1"
  if [[ "$ip" =~ $IP4_RE ]] || [[ "$ip" =~ $IP6_RE ]]; then
    return 0
  fi
  die "invalid IP: $ip"
}

validate_comment() {
  local c="$1"
  [[ "$c" =~ $COMMENT_RE ]] || die "invalid comment (allowed: letters, digits, space, . _ @ : / -; max 120 chars)"
}

validate_config_name() {
  local name="$1"
  for allowed in $CONFIG_ALLOW; do
    if [[ "$name" == "$allowed" ]]; then return 0; fi
  done
  die "config name not in allowlist: $name"
}

is_ipv6() {
  [[ "$1" == *:* ]]
}

require_csf() {
  [[ -x "$CSF_BIN" ]] || die "csf is not installed at $CSF_BIN"
}

# --- subcommands --------------------------------------------------------------

cmd_version() {
  if [[ ! -x "$CSF_BIN" ]]; then
    printf 'not_installed\n'
    return 0
  fi
  "$CSF_BIN" -v 2>&1 || true
}

cmd_status() {
  if [[ ! -x "$CSF_BIN" ]]; then
    printf 'installed=false\n'
    return 0
  fi
  printf 'installed=true\n'
  local testing
  testing="$(awk -F'=' '/^TESTING[[:space:]]*=/{gsub(/[" ]/,"",$2); print $2; exit}' "$CSF_ETC/csf.conf" 2>/dev/null || printf '?')"
  printf 'testing_mode=%s\n' "$testing"
  local rules
  rules="$( { iptables -S 2>/dev/null; ip6tables -S 2>/dev/null; } | wc -l | awk '{print $1}')"
  printf 'iptables_rules=%s\n' "$rules"
  if systemctl is-active --quiet lfd 2>/dev/null; then
    printf 'lfd_active=true\n'
  else
    printf 'lfd_active=false\n'
  fi
}

cmd_list_allow() {
  require_csf
  [[ -f "$CSF_ETC/csf.allow" ]] && cat "$CSF_ETC/csf.allow" || true
}

cmd_list_deny() {
  require_csf
  [[ -f "$CSF_ETC/csf.deny" ]] && cat "$CSF_ETC/csf.deny" || true
}

cmd_list_tempbans() {
  require_csf
  "$CSF_BIN" -t 2>&1 || true
}

cmd_tail_log() {
  local n="${1:-200}"
  [[ "$n" =~ ^[0-9]+$ ]] || die "lines must be numeric"
  (( n <= 5000 )) || die "lines cap is 5000"
  [[ -f "$LFD_LOG" ]] || { printf 'lfd log not found at %s\n' "$LFD_LOG"; return 0; }
  tail -n "$n" "$LFD_LOG"
}

cmd_allow_ip() {
  local ip="$1" comment="$2"
  validate_ip "$ip"
  validate_comment "$comment"
  require_csf
  if is_ipv6 "$ip"; then
    [[ -x "$CSF6_BIN" ]] || die "csf6 not found; IPv6 support disabled on this host"
    "$CSF6_BIN" -a "$ip" "$comment"
  else
    "$CSF_BIN" -a "$ip" "$comment"
  fi
}

cmd_deny_ip() {
  local ip="$1" comment="$2"
  validate_ip "$ip"
  validate_comment "$comment"
  require_csf
  if is_ipv6 "$ip"; then
    [[ -x "$CSF6_BIN" ]] || die "csf6 not found; IPv6 support disabled on this host"
    "$CSF6_BIN" -d "$ip" "$comment"
  else
    "$CSF_BIN" -d "$ip" "$comment"
  fi
}

cmd_remove_ip() {
  local ip="$1"
  validate_ip "$ip"
  require_csf
  if is_ipv6 "$ip"; then
    [[ -x "$CSF6_BIN" ]] && "$CSF6_BIN" -ar "$ip" 2>/dev/null || true
    [[ -x "$CSF6_BIN" ]] && "$CSF6_BIN" -dr "$ip" 2>/dev/null || true
  else
    "$CSF_BIN" -ar "$ip" 2>/dev/null || true
    "$CSF_BIN" -dr "$ip" 2>/dev/null || true
  fi
}

cmd_read_config() {
  local name="$1"
  validate_config_name "$name"
  require_csf
  local path="$CSF_ETC/$name"
  [[ -f "$path" ]] || { printf ''; return 0; }
  cat "$path"
}

cmd_write_config() {
  local name="$1"
  validate_config_name "$name"
  require_csf
  local path="$CSF_ETC/$name"
  local backup_dir="${STACK_MANAGER_CSF_BACKUP_DIR:-/var/backups/stack-manager-csf}"
  mkdir -p "$backup_dir"
  chmod 700 "$backup_dir"
  local ts
  ts="$(date -u +%Y%m%dT%H%M%SZ)"
  if [[ -f "$path" ]]; then
    cp -a "$path" "$backup_dir/${name}.${ts}"
  fi
  local tmp
  tmp="$(mktemp "${backup_dir}/.${name}.new.XXXXXX")"
  cat > "$tmp"
  # Basic sanity: reject binary content.
  if grep -q $'\x00' "$tmp"; then
    rm -f "$tmp"
    die "config content contains NUL byte"
  fi
  chmod 600 "$tmp"
  mv "$tmp" "$path"
  printf 'wrote %s (backup: %s/%s.%s)\n' "$path" "$backup_dir" "$name" "$ts"
}

cmd_restart() {
  require_csf
  "$CSF_BIN" -r
}

cmd_reload_lfd() {
  systemctl restart lfd
}

cmd_install() {
  if [[ -x "$CSF_BIN" ]]; then
    printf 'already installed: %s\n' "$("$CSF_BIN" -v 2>&1 | head -1)"
    return 0
  fi
  local workdir=""
  # ${workdir:-} guard keeps `set -u` happy if the trap fires before
  # mktemp completes (mktemp itself fails on a full /tmp, etc.).
  trap 'rm -rf "${workdir:-}"' EXIT
  workdir="$(mktemp -d /tmp/stack-manager-csf-install.XXXXXX)"
  git clone --depth 1 "$UPSTREAM_URL" "$workdir/csf" >&2
  # Look for the installer by file existence, not executable bit —
  # `git clone` on some hosts strips +x from tracked files. csf is
  # designed to be run via `sh install.sh` anyway.
  local installer=""
  if [[ -f "$workdir/csf/install.sh" ]]; then
    installer="install.sh"
  elif [[ -f "$workdir/csf/install.generic.sh" ]]; then
    installer="install.generic.sh"
  else
    die "no installer found in upstream repo (tried install.sh and install.generic.sh under $workdir/csf)"
  fi
  ( cd "$workdir/csf" && sh "$installer" )
  [[ -x "$CSF_BIN" ]] || die "install did not produce $CSF_BIN"
  "$CSF_BIN" -v
}

cmd_uninstall() {
  local uninst="$CSF_ETC/uninstall.sh"
  [[ -x "$uninst" ]] || die "uninstall script not found at $uninst"
  sh "$uninst"
}

# --- dispatch -----------------------------------------------------------------

sub="${1:-}"
if [[ -z "$sub" || "$sub" == "-h" || "$sub" == "--help" ]]; then
  usage
  exit 0
fi
shift || true

case "$sub" in
  version)         cmd_version                 ;;
  status)          cmd_status                  ;;
  list-allow)      cmd_list_allow              ;;
  list-deny)       cmd_list_deny               ;;
  list-tempbans)   cmd_list_tempbans           ;;
  tail-log)        cmd_tail_log "$@"           ;;
  allow-ip)        [[ $# -ge 2 ]] || die "allow-ip requires <ip> <comment>"; cmd_allow_ip "$1" "$2" ;;
  deny-ip)         [[ $# -ge 2 ]] || die "deny-ip requires <ip> <comment>"; cmd_deny_ip "$1" "$2" ;;
  remove-ip)       [[ $# -ge 1 ]] || die "remove-ip requires <ip>"; cmd_remove_ip "$1" ;;
  read-config)     [[ $# -ge 1 ]] || die "read-config requires <name>"; cmd_read_config "$1" ;;
  write-config)    [[ $# -ge 1 ]] || die "write-config requires <name>"; cmd_write_config "$1" ;;
  restart)         cmd_restart                 ;;
  reload-lfd)      cmd_reload_lfd              ;;
  install)         cmd_install                 ;;
  uninstall)       cmd_uninstall               ;;
  *)               usage; exit 2               ;;
esac
