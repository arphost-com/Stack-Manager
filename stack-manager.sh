#!/usr/bin/env bash
set -u -o pipefail

# -------------------------------------------------------------------
# stack-manager.sh (robust + hooks)
#
# Key behavior:
# - start/stop/status are ALWAYS normal compose actions:
#     up    -> docker compose up -d
#     down  -> docker compose down
#     status-> docker compose ps
# - update behavior:
#     If a custom hook exists:  post-update_<project>.sh
#       -> run ONLY the hook for update (skip normal compose pull/up)
#     Else:
#       -> normal compose pull + up -d
#
# Other features:
# - Discovers projects under ROOT with compose.yml / docker-compose.yml etc.
# - Optional logging to file (screen always).
# - Configurable log directory (default: ROOT).
# - Better `check` output: prints pull output and summarizes.
# - Supports .inactive marker with inactive on/off/list (skipped by default).
# - Supports --running-only for mutating commands.
# - Supports --timeout for pull/check to avoid hangs.
# - Sanitizes compose project name to avoid invalid project name errors.
#
# Hook convention:
#   <HOOKS_DIR>/<phase>-<command>_<project>.sh
#     phase: pre | post
#     command: update | pull | check | restart | down | status | list | up
#     project: folder name (basename of project dir)
#
# The special rule implemented here is ONLY for update:
#   post-update_<project>.sh overrides normal update flow.
#
# Configuration files (loaded in order, later values override):
#   /etc/stack-manager.conf
#   ~/.config/stack-manager.conf
# -------------------------------------------------------------------

# Default values (can be overridden by config files or CLI flags)
ROOT="/docker"
INACTIVE_MARKER=".inactive"

INCLUDE_INACTIVE=0
ONLY_INACTIVE=0
DRY_RUN=0
DO_PRUNE=0
VERBOSE=0
RUNNING_ONLY=0

# Logging
LOG_ENABLED=1          # can be disabled with --no-log / --log-off
LOG_DIR=""             # default = ROOT
LOG_FILE=""            # auto if empty

# Timeout seconds (0 = disabled)
TIMEOUT_SECS=0

# Hooks
HOOKS_ENABLED=1
HOOKS_DIR=""           # default: <ROOT>/.stack-manager/hooks

# Load config files if they exist
for _conf in /etc/stack-manager.conf ~/.config/stack-manager.conf; do
  # shellcheck disable=SC1090
  [[ -f "$_conf" ]] && source "$_conf"
done
unset _conf

STOP_REQUESTED=0

declare -a EXCLUDES=()
declare -a ONLY=()
declare -a CLI_PROJECTS=()

declare -a FAILURES=()
declare -a SUCCESSES=()
declare -a SKIPPED=()

# Colors
if [[ -t 1 ]]; then
  CYAN=$(tput setaf 6); MAGENTA=$(tput setaf 5); YELLOW=$(tput setaf 3)
  GREEN=$(tput setaf 2); RED=$(tput setaf 1); NC=$(tput sgr0)
else
  CYAN=""; MAGENTA=""; YELLOW=""; GREEN=""; RED=""; NC=""
fi

log_hdr() { echo -e "${GREEN}-----------------------------------------------------------------------------${NC}"; }
say()     { echo -e "$*"; }
warn()    { echo -e "${YELLOW}WARN:${NC} $*" >&2; }
err()     { echo -e "${RED}ERROR:${NC} $*" >&2; }

has_bin() { command -v "$1" >/dev/null 2>&1; }
need_bin() { has_bin "$1" || { err "Missing required command: $1"; exit 1; }; }

contains() { local needle="$1"; shift; for x in "$@"; do [[ "$x" == "$needle" ]] && return 0; done; return 1; }

usage() {
  cat <<'EOF'
Usage:
  stack-manager.sh [global options] <command> [project ...]

Commands:
  list            List discovered projects + running containers per project
  status          Show 'docker compose ps' per project
  check           Check for image updates (pull output printed; no restart)
  pull            Pull images for projects
  up              docker compose up -d for projects
  update          Update projects:
                    - if post-update_<project>.sh exists => run ONLY that script
                    - else => docker compose pull + up -d
  restart         docker compose restart for projects
  down            docker compose down for projects
  prune           Prune docker images/networks/volumes (or use --prune after a command)

Inactive management:
  inactive list
  inactive on  <name>
  inactive off <name>

Global options:
  -r, --root <path>        Root folder containing projects (default: /docker)
  -x, --exclude <name>     Exclude by folder name (repeatable)
  -o, --only <name>        Only include folder name (repeatable)
      --include-inactive   Include projects with .inactive marker (otherwise skipped)
      --only-inactive      Only include projects with .inactive marker
      --running-only       Only act on projects that currently have running containers (mutating commands)
      --timeout <seconds>  Timeout for pull/check. 0 disables.

Logging options:
      --log-dir <path>     Log directory (default: ROOT)
      --log-file <path>    Log file path (overrides --log-dir)
      --no-log             Disable file logging (screen only)
      --log-off            Same as --no-log
      --log-on             Re-enable file logging (default)

Hooks:
      --hooks-dir <path>   Hooks directory (default: <ROOT>/.stack-manager/hooks)
      --no-hooks           Disable hooks

Other:
  -n, --dry-run            Show actions only
  -p, --prune              Run prune at the end
  -v, --verbose            More output
  -h, --help               Show help

Config files (loaded in order, later values override earlier):
  /etc/stack-manager.conf           System-wide config
  ~/.config/stack-manager.conf      User config

  Supported variables: ROOT, INACTIVE_MARKER, LOG_ENABLED, LOG_DIR,
  TIMEOUT_SECS, HOOKS_ENABLED, HOOKS_DIR

Examples:
  ./stack-manager.sh --root /docker list
  ./stack-manager.sh --root /docker check --timeout 180
  ./stack-manager.sh --root /docker up netbox-docker
  ./stack-manager.sh --root /docker update netbox-docker   # uses hook if present
EOF
}

# Ctrl-C handling
on_int() {
  STOP_REQUESTED=1
  echo
  warn "Ctrl-C received. Will stop after current project and print summary."
}
trap on_int INT

compose_file_for_dir() {
  local d="$1" f
  for f in "compose.yml" "compose.yaml" "docker-compose.yml" "docker-compose.yaml"; do
    [[ -f "$d/$f" ]] && { echo "$d/$f"; return 0; }
  done
  echo ""
}

is_inactive_project() { [[ -f "$1/$INACTIVE_MARKER" ]]; }

project_header() {
  local title="$1" dir="$2"
  log_hdr
  say "${YELLOW}${title}${NC}  ${MAGENTA}${dir}${NC}"
  log_hdr
}

sanitize_project_name() {
  local s="$1"
  s="$(echo "$s" | tr '[:upper:]' '[:lower:]')"
  s="$(echo "$s" | sed -E 's/[^a-z0-9_-]+/-/g')"
  s="$(echo "$s" | sed -E 's/^-+//; s/-+$//')"
  s="$(echo "$s" | sed -E 's/^[^a-z0-9]+//')"
  [[ -n "$s" ]] || s="p"
  echo "$s"
}

detect_running_project_label() {
  local dir="$1"
  local base; base="$(basename "$dir")"
  local cand1 cand2 found
  cand1="$(sanitize_project_name "$base")"
  cand2="$(echo "$base" | tr '[:upper:]' '[:lower:]')"

  found="$(docker ps --filter "label=com.docker.compose.project=${cand1}" --format '{{.Label "com.docker.compose.project"}}' | head -n 1 || true)"
  [[ -n "$found" ]] && { echo "$found"; return 0; }

  found="$(docker ps --filter "label=com.docker.compose.project=${cand2}" --format '{{.Label "com.docker.compose.project"}}' | head -n 1 || true)"
  [[ -n "$found" ]] && { echo "$found"; return 0; }

  echo ""
}

project_name_for_dir() {
  local dir="$1"
  local base; base="$(basename "$dir")"
  local existing
  existing="$(detect_running_project_label "$dir")"
  if [[ -n "$existing" ]]; then
    echo "$existing"
  else
    echo "$(sanitize_project_name "$base")"
  fi
}

compose_cmd() {
  local dir="$1"
  local cf; cf="$(compose_file_for_dir "$dir")"
  [[ -n "$cf" ]] || return 1

  local pname; pname="$(project_name_for_dir "$dir")"
  echo "COMPOSE_PROGRESS=plain docker compose --ansi never -f \"$cf\" -p \"$pname\""
}

running_container_lines_for_dir() {
  local dir="$1"
  local pname; pname="$(project_name_for_dir "$dir")"
  docker ps --filter "label=com.docker.compose.project=${pname}" --format '  - {{.Names}}  ({{.Image}})' 2>/dev/null || true
}

is_project_running() {
  local dir="$1"
  local pname; pname="$(project_name_for_dir "$dir")"
  local one
  one="$(docker ps --filter "label=com.docker.compose.project=${pname}" --format '{{.ID}}' | head -n 1 || true)"
  [[ -n "$one" ]]
}

discover_projects() {
  local root="$1"
  [[ -d "$root" ]] || { err "Root does not exist: $root"; exit 1; }

  local cf
  cf="$(compose_file_for_dir "$root")"
  [[ -n "$cf" ]] && echo "$root"

  local d
  while IFS= read -r -d '' d; do
    cf="$(compose_file_for_dir "$d")"
    [[ -n "$cf" ]] && echo "$d"
  done < <(find "$root" -mindepth 1 -maxdepth 1 -type d ! -name '.*' -print0 | sort -z)
}

filter_projects() {
  local -a projects=("$@")
  local -a out=()

  for p in "${projects[@]}"; do
    local base inactive
    base="$(basename "$p")"
    inactive=0
    is_inactive_project "$p" && inactive=1

    if (( ONLY_INACTIVE )); then
      (( inactive )) || { (( VERBOSE )) && warn "Skipping $base (not inactive)"; continue; }
    else
      if (( inactive )) && (( ! INCLUDE_INACTIVE )); then
        (( VERBOSE )) && warn "Skipping $base (marked inactive)"
        continue
      fi
    fi

    if [[ ${#CLI_PROJECTS[@]} -gt 0 ]]; then
      contains "$base" "${CLI_PROJECTS[@]}" || { (( VERBOSE )) && warn "Skipping $base (not in CLI list)"; continue; }
    fi

    if [[ ${#ONLY[@]} -gt 0 ]]; then
      contains "$base" "${ONLY[@]}" || { (( VERBOSE )) && warn "Skipping $base (not in --only list)"; continue; }
    fi

    if [[ ${#EXCLUDES[@]} -gt 0 ]]; then
      contains "$base" "${EXCLUDES[@]}" && { (( VERBOSE )) && warn "Skipping $base (excluded)"; continue; }
    fi

    out+=("$p")
  done

  [[ ${#out[@]} -gt 0 ]] && printf '%s\n' "${out[@]}"
}

project_dir_by_name() {
  local name="$1"
  local -a all=()
  while IFS= read -r p; do all+=("$p"); done < <(discover_projects "$ROOT")
  for p in "${all[@]}"; do
    [[ "$(basename "$p")" == "$name" ]] && { echo "$p"; return 0; }
  done
  echo ""
}

init_logging() {
  if (( ! LOG_ENABLED )); then
    say "${CYAN}Logging:${NC} file logging disabled (screen only)"
    return 0
  fi

  if [[ -z "$LOG_DIR" ]]; then
    LOG_DIR="$ROOT"
  fi

  if [[ -z "$LOG_FILE" ]]; then
    mkdir -p "$LOG_DIR" 2>/dev/null || true
    LOG_FILE="${LOG_DIR}/stack-manager_$(date +%Y%m%d_%H%M%S).log"
  else
    mkdir -p "$(dirname "$LOG_FILE")" 2>/dev/null || true
  fi

  touch "$LOG_FILE" 2>/dev/null || {
    local fallback="${HOME}/stack-manager_$(date +%Y%m%d_%H%M%S).log"
    warn "Cannot write log file at '$LOG_FILE'. Falling back to '$fallback'."
    LOG_FILE="$fallback"
    touch "$LOG_FILE" || { err "Cannot write any log file."; exit 1; }
  }

  exec > >(tee -a "$LOG_FILE") 2>&1
  say "${CYAN}Log file:${NC} ${MAGENTA}${LOG_FILE}${NC}"
}

init_hooks() {
  if [[ -z "$HOOKS_DIR" ]]; then
    HOOKS_DIR="${ROOT}/.stack-manager/hooks"
  fi
  if (( HOOKS_ENABLED )); then
    mkdir -p "$HOOKS_DIR" 2>/dev/null || true
  fi
}

hook_path() {
  # <phase> <command> <project>
  local phase="$1" cmd="$2" project="$3"
  echo "${HOOKS_DIR}/${phase}-${cmd}_${project}.sh"
}

run_hook_if_present() {
  # Args: <phase> <command> <project_dir>
  local phase="$1" cmd="$2" dir="$3"
  local project; project="$(basename "$dir")"

  (( HOOKS_ENABLED )) || return 0

  local hook; hook="$(hook_path "$phase" "$cmd" "$project")"
  if [[ -x "$hook" ]]; then
    project_header "Hook (${phase}-${cmd}): ${project}" "$hook"
    if (( DRY_RUN )); then
      say "${CYAN}[dry-run]${NC} $hook $project $dir"
      return 0
    fi
    "$hook" "$project" "$dir" || {
      local rc=$?
      say "${RED}HOOK FAILED${NC} (${phase}-${cmd}) project=${project} exit_code=${rc}"
      FAILURES+=("${project} (Hook ${phase}-${cmd}) exit_code=${rc}")
    }
  fi
  return 0
}

run_with_timeout() {
  local secs="$1"; shift
  local cmd="$*"

  if (( secs <= 0 )); then
    bash -lc "$cmd"
    return $?
  fi

  if ! has_bin timeout; then
    warn "'timeout' not found; running without timeout."
    bash -lc "$cmd"
    return $?
  fi

  timeout --preserve-status --kill-after=10s "${secs}s" bash -lc "$cmd"
  return $?
}

is_mutating_command() {
  case "${CMD:-}" in
    pull|update|restart|down|up|prune) return 0 ;;
    *) return 1 ;;
  esac
}

run_project_op() {
  local dir="$1" label="$2" cmd="$3" tmo="${4:-0}"
  local name; name="$(basename "$dir")"

  if (( RUNNING_ONLY )) && is_mutating_command; then
    if ! is_project_running "$dir"; then
      (( VERBOSE )) && warn "Skipping $name ($label) due to --running-only (not running)."
      SKIPPED+=("$name ($label) [not running]")
      return 0
    fi
  fi

  project_header "${label}: ${name}" "$dir"

  if (( DRY_RUN )); then
    say "${CYAN}[dry-run]${NC} $cmd"
    SUCCESSES+=("$name ($label) [dry-run]")
    return 0
  fi

  local rc=0
  say "${CYAN}Command:${NC} $cmd"
  if (( tmo > 0 )); then
    run_with_timeout "$tmo" "$cmd" || rc=$?
  else
    bash -lc "$cmd" || rc=$?
  fi

  if (( rc == 0 )); then
    say "${GREEN}OK${NC} ($label)"
    SUCCESSES+=("$name ($label)")
  else
    if (( rc == 124 || rc == 137 )); then
      say "${RED}FAILED${NC} ($label) exit_code=${rc} (timeout)"
      FAILURES+=("$name ($label) exit_code=${rc} (timeout)")
    else
      say "${RED}FAILED${NC} ($label) exit_code=${rc}"
      FAILURES+=("$name ($label) exit_code=${rc}")
    fi
  fi

  (( STOP_REQUESTED )) && return 130
  return "$rc"
}

print_summary() {
  log_hdr
  say "${YELLOW}Summary${NC}"
  log_hdr

  if [[ ${#SUCCESSES[@]} -gt 0 ]]; then
    say "${GREEN}Succeeded:${NC}"
    for s in "${SUCCESSES[@]}"; do echo "  - $s"; done
  else
    say "${YELLOW}Succeeded:${NC} none"
  fi

  echo
  if [[ ${#SKIPPED[@]} -gt 0 ]]; then
    say "${CYAN}Skipped:${NC}"
    for s in "${SKIPPED[@]}"; do echo "  - $s"; done
  fi

  echo
  if [[ ${#FAILURES[@]} -gt 0 ]]; then
    say "${RED}Failed:${NC}"
    for f in "${FAILURES[@]}"; do echo "  - $f"; done
    echo
    if (( LOG_ENABLED )); then
      say "${YELLOW}Log:${NC} ${MAGENTA}${LOG_FILE}${NC}"
    fi
  else
    say "${GREEN}Failed:${NC} none"
  fi

  log_hdr
}

# -----------------------------
# Commands
# -----------------------------
cmd_list() {
  local -a projects=("$@")
  say "${YELLOW}Compose projects under:${NC} ${MAGENTA}${ROOT}${NC}"
  say "${YELLOW}Inactive marker:${NC} ${MAGENTA}${INACTIVE_MARKER}${NC} (skipped by default)"
  if (( LOG_ENABLED )); then
    say "${YELLOW}Logs:${NC} ${MAGENTA}${LOG_DIR:-$ROOT}${NC}"
  else
    say "${YELLOW}Logs:${NC} disabled"
  fi
  if (( HOOKS_ENABLED )); then
    say "${YELLOW}Hooks:${NC} ${MAGENTA}${HOOKS_DIR}${NC}"
    say "${YELLOW}Update override hook name:${NC} post-update_<project>.sh"
  else
    say "${YELLOW}Hooks:${NC} disabled"
  fi
  log_hdr

  for dir in "${projects[@]}"; do
    local name; name="$(basename "$dir")"
    local inactive_tag=""
    is_inactive_project "$dir" && inactive_tag=" ${YELLOW}[inactive]${NC}"

    local hook_tag=""
    if (( HOOKS_ENABLED )) && [[ -x "$(hook_path "post" "update" "$name")" ]]; then
      hook_tag=" ${CYAN}[hook-update]${NC}"
    fi

    project_header "${name}${inactive_tag}${hook_tag}" "$dir"

    if is_project_running "$dir"; then
      say "${CYAN}Running containers:${NC}"
      local lines; lines="$(running_container_lines_for_dir "$dir")"
      [[ -n "$lines" ]] && echo "$lines" || say "${YELLOW}Running, but could not enumerate via labels. Try 'status'.${NC}"
    else
      say "${YELLOW}Status:${NC} not running"
    fi

    (( STOP_REQUESTED )) && return 130
  done
  return 0
}

cmd_status() {
  local -a projects=("$@")
  for dir in "${projects[@]}"; do
    local ccmd; ccmd="$(compose_cmd "$dir" || true)"
    [[ -n "$ccmd" ]] || continue
    run_project_op "$dir" "Status" "$ccmd ps" || true
    (( STOP_REQUESTED )) && return 130
  done
  return 0
}

cmd_check() {
  local -a projects=("$@")

  say "${YELLOW}Checking for image updates (pull + output).${NC}"
  say "${CYAN}Note:${NC} This may download newer images, but does NOT restart containers."
  if (( TIMEOUT_SECS > 0 )); then
    say "${CYAN}Timeout:${NC} ${TIMEOUT_SECS}s (pull operations)"
  fi
  log_hdr

  for dir in "${projects[@]}"; do
    local name; name="$(basename "$dir")"
    local ccmd; ccmd="$(compose_cmd "$dir" || true)"
    [[ -n "$ccmd" ]] || continue

    project_header "Check: ${name}" "$dir"

    if (( DRY_RUN )); then
      say "${CYAN}[dry-run]${NC} $ccmd pull"
      SUCCESSES+=("$name (Check) [dry-run]")
      continue
    fi

    local out rc=0
    if (( TIMEOUT_SECS > 0 )); then
      out="$(run_with_timeout "$TIMEOUT_SECS" "$ccmd pull" 2>&1)" || rc=$?
    else
      out="$(bash -lc "$ccmd pull" 2>&1)" || rc=$?
    fi

    echo "$out"

    if (( rc != 0 )); then
      if (( rc == 124 || rc == 137 )); then
        say "${RED}FAILED${NC} (Check) exit_code=${rc} (timeout)"
        FAILURES+=("$name (Check) exit_code=${rc} (timeout)")
      else
        say "${RED}FAILED${NC} (Check) exit_code=${rc}"
        FAILURES+=("$name (Check) exit_code=${rc}")
      fi
      say "${YELLOW}Last lines:${NC}"
      echo "$out" | tail -n 30
    else
      if echo "$out" | grep -qi 'Downloaded newer image'; then
        say "${GREEN}Result:${NC} updates found (newer images pulled)"
      elif echo "$out" | grep -qiE 'Image is up to date|Already exists|Pull complete|Downloaded'; then
        say "${GREEN}Result:${NC} pull completed (likely up-to-date)"
      else
        say "${YELLOW}Result:${NC} pull succeeded but output didn’t indicate changes"
      fi
      SUCCESSES+=("$name (Check)")
    fi

    (( STOP_REQUESTED )) && return 130
  done
  return 0
}

cmd_pull() {
  local -a projects=("$@")
  for dir in "${projects[@]}"; do
    local ccmd; ccmd="$(compose_cmd "$dir" || true)"
    [[ -n "$ccmd" ]] || continue
    run_project_op "$dir" "Pull" "$ccmd pull" "$TIMEOUT_SECS" || true
    (( STOP_REQUESTED )) && return 130
  done
  return 0
}

cmd_up() {
  local -a projects=("$@")
  for dir in "${projects[@]}"; do
    local ccmd; ccmd="$(compose_cmd "$dir" || true)"
    [[ -n "$ccmd" ]] || continue
    run_project_op "$dir" "Up" "$ccmd up -d" || true
    (( STOP_REQUESTED )) && return 130
  done
  return 0
}

cmd_update() {
  local -a projects=("$@")
  for dir in "${projects[@]}"; do
    local name; name="$(basename "$dir")"
    local ccmd; ccmd="$(compose_cmd "$dir" || true)"
    [[ -n "$ccmd" ]] || continue

    # If a custom post-update hook exists, use it INSTEAD of compose pull/up.
    local hook; hook="$(hook_path "post" "update" "$name")"
    if (( HOOKS_ENABLED )) && [[ -x "$hook" ]]; then
      project_header "Update (hook): ${name}" "$dir"
      say "${CYAN}Custom update hook detected:${NC} $hook"
      say "${YELLOW}Using hook for update; skipping normal compose pull/up for this project.${NC}"

      if (( DRY_RUN )); then
        say "${CYAN}[dry-run]${NC} $hook $name $dir"
        SUCCESSES+=("$name (Hook post-update) [dry-run]")
      else
        local rc=0
        "$hook" "$name" "$dir" || rc=$?
        if (( rc != 0 )); then
          say "${RED}HOOK FAILED${NC} project=${name} exit_code=${rc}"
          FAILURES+=("${name} (Hook post-update) exit_code=${rc}")
        else
          say "${GREEN}Hook OK${NC}"
          SUCCESSES+=("$name (Hook post-update)")
        fi
      fi

      (( STOP_REQUESTED )) && return 130
      continue
    fi

    # No hook: normal update flow
    project_header "Update: ${name}" "$dir"

    local pull_ok=1
    say "${CYAN}Step 1/2:${NC} Pulling images..."
    run_project_op "$dir" "Pull" "$ccmd pull" "$TIMEOUT_SECS" || pull_ok=0
    (( STOP_REQUESTED )) && return 130

    if (( pull_ok )); then
      say "${CYAN}Step 2/2:${NC} Bringing up containers..."
      run_project_op "$dir" "Up" "$ccmd up -d" || true
    else
      say "${YELLOW}Skipping 'up' for ${name} because pull failed${NC}"
      SKIPPED+=("$name (Up) [pull failed]")
    fi
    (( STOP_REQUESTED )) && return 130

    # Explicitly continue to next project regardless of success/failure
    continue
  done
}

cmd_restart() {
  local -a projects=("$@")
  for dir in "${projects[@]}"; do
    local ccmd; ccmd="$(compose_cmd "$dir" || true)"
    [[ -n "$ccmd" ]] || continue
    run_project_op "$dir" "Restart" "$ccmd restart" || true
    (( STOP_REQUESTED )) && return 130
  done
  return 0
}

cmd_down() {
  local -a projects=("$@")
  for dir in "${projects[@]}"; do
    local ccmd; ccmd="$(compose_cmd "$dir" || true)"
    [[ -n "$ccmd" ]] || continue
    run_project_op "$dir" "Down" "$ccmd down" || true
    (( STOP_REQUESTED )) && return 130
  done
  return 0
}

do_prune() {
  log_hdr
  say "${YELLOW}Pruning docker resources (images, networks, volumes)...${NC}"
  log_hdr

  if (( DRY_RUN )); then
    say "${CYAN}[dry-run]${NC} docker image prune"
    say "${CYAN}[dry-run]${NC} docker network prune"
    say "${CYAN}[dry-run]${NC} docker volume prune"
    return 0
  fi

  docker image prune -f || true
  docker network prune -f || true
  docker volume prune -f || true
}

# -----------------------------
# Inactive subcommands
# -----------------------------
cmd_inactive_list() {
  local -a all=()
  while IFS= read -r p; do all+=("$p"); done < <(discover_projects "$ROOT")

  say "${YELLOW}Inactive projects under:${NC} ${MAGENTA}${ROOT}${NC}"
  log_hdr

  local found=0
  for dir in "${all[@]}"; do
    if is_inactive_project "$dir"; then
      found=1
      echo "  - $(basename "$dir")  ($dir)"
    fi
  done

  (( found )) || say "${CYAN}None marked inactive.${NC}"
}

cmd_inactive_on() {
  local name="${1:-}"
  [[ -n "$name" ]] || { err "inactive on requires a project name"; exit 1; }

  local dir
  dir="$(project_dir_by_name "$name")"
  [[ -n "$dir" ]] || { err "Project not found under $ROOT: $name"; exit 1; }

  if (( DRY_RUN )); then
    say "${CYAN}[dry-run]${NC} would create: $dir/$INACTIVE_MARKER"
    return 0
  fi

  touch "$dir/$INACTIVE_MARKER"
  say "${GREEN}Marked inactive:${NC} ${MAGENTA}$name${NC} (created $INACTIVE_MARKER)"
}

cmd_inactive_off() {
  local name="${1:-}"
  [[ -n "$name" ]] || { err "inactive off requires a project name"; exit 1; }

  local dir
  dir="$(project_dir_by_name "$name")"
  [[ -n "$dir" ]] || { err "Project not found under $ROOT: $name"; exit 1; }

  if (( DRY_RUN )); then
    say "${CYAN}[dry-run]${NC} would remove: $dir/$INACTIVE_MARKER"
    return 0
  fi

  if [[ -f "$dir/$INACTIVE_MARKER" ]]; then
    rm -f "$dir/$INACTIVE_MARKER"
    say "${GREEN}Marked active:${NC} ${MAGENTA}$name${NC} (removed $INACTIVE_MARKER)"
  else
    say "${CYAN}Already active:${NC} ${MAGENTA}$name${NC} (no $INACTIVE_MARKER found)"
  fi
}

# -----------------------------
# Parse args
# -----------------------------
if [[ $# -lt 1 ]]; then usage; exit 1; fi

while [[ $# -gt 0 ]]; do
  case "${1:-}" in
    -r|--root) ROOT="${2:-}"; shift 2;;
    -x|--exclude) EXCLUDES+=("${2:-}"); shift 2;;
    -o|--only) ONLY+=("${2:-}"); shift 2;;
    --include-inactive) INCLUDE_INACTIVE=1; shift;;
    --only-inactive) ONLY_INACTIVE=1; shift;;
    --running-only) RUNNING_ONLY=1; shift;;
    --timeout) TIMEOUT_SECS="${2:-0}"; shift 2;;

    --log-dir) LOG_DIR="${2:-}"; shift 2;;
    --log-file) LOG_FILE="${2:-}"; shift 2;;
    --no-log|--log-off) LOG_ENABLED=0; shift;;
    --log-on) LOG_ENABLED=1; shift;;

    --hooks-dir) HOOKS_DIR="${2:-}"; shift 2;;
    --no-hooks) HOOKS_ENABLED=0; shift;;

    -n|--dry-run) DRY_RUN=1; shift;;
    -p|--prune) DO_PRUNE=1; shift;;
    -v|--verbose) VERBOSE=1; shift;;
    -h|--help) usage; exit 0;;
    --) shift; break;;
    -*)
      err "Unknown option: $1"
      usage
      exit 1
      ;;
    *)
      break
      ;;
  esac
done

CMD="${1:-}"; shift || true

need_bin docker
init_logging
init_hooks

if (( TIMEOUT_SECS > 0 )) && ! has_bin timeout; then
  warn "You set --timeout ${TIMEOUT_SECS}, but 'timeout' is not installed. Timeouts will be ignored."
fi

# inactive command
if [[ "$CMD" == "inactive" ]]; then
  sub="${1:-}"; shift || true
  case "$sub" in
    list) cmd_inactive_list ;;
    on)   cmd_inactive_on "${1:-}" ;;
    off)  cmd_inactive_off "${1:-}" ;;
    *) err "Unknown inactive subcommand: ${sub:-<none>}"; usage; exit 1 ;;
  esac
  exit 0
fi

CLI_PROJECTS=("$@")

ALL_PROJECTS=()
while IFS= read -r p; do ALL_PROJECTS+=("$p"); done < <(discover_projects "$ROOT")
[[ ${#ALL_PROJECTS[@]} -gt 0 ]] || { err "No compose projects found under: $ROOT"; exit 1; }

PROJECTS=()
while IFS= read -r p; do PROJECTS+=("$p"); done < <(filter_projects "${ALL_PROJECTS[@]}")
[[ ${#PROJECTS[@]} -gt 0 ]] || { err "No matching projects after filters."; exit 1; }

rc=0
case "$CMD" in
  list)    cmd_list "${PROJECTS[@]}" || rc=$? ;;
  status)  cmd_status "${PROJECTS[@]}" || rc=$? ;;
  check)   cmd_check "${PROJECTS[@]}" || rc=$? ;;
  pull)    cmd_pull "${PROJECTS[@]}" || rc=$? ;;
  up)      cmd_up "${PROJECTS[@]}" || rc=$? ;;
  update)  cmd_update "${PROJECTS[@]}" || rc=$? ;;
  restart) cmd_restart "${PROJECTS[@]}" || rc=$? ;;
  down)    cmd_down "${PROJECTS[@]}" || rc=$? ;;
  prune)   do_prune || true ;;
  *)
    err "Unknown command: $CMD"
    usage
    exit 1
    ;;
esac

if (( DO_PRUNE )) && [[ "$CMD" != "prune" ]]; then
  do_prune || true
fi

print_summary

if (( STOP_REQUESTED )); then
  exit 130
fi

if [[ ${#FAILURES[@]} -gt 0 ]]; then
  exit 2
fi

exit "${rc}"
