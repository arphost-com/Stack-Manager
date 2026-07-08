#!/usr/bin/env sh
set -eu

script_dir="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
project_root="$(dirname "${script_dir}")"
agent_mode=0
case "${1:-}" in
  --agent)
    agent_mode=1
    shift
    ;;
esac
env_file="${1:-${project_root}/.env}"
example_env="${project_root}/.env.example"
case "${env_file}" in
  /*) ;;
  *) env_file="$(pwd)/${env_file}" ;;
esac

rand_secret() {
  if command -v openssl >/dev/null 2>&1; then
    base="$(openssl rand -base64 48 | tr -dc 'A-Za-z0-9' | cut -c 1-32)"
  else
    base="$(dd if=/dev/urandom bs=48 count=1 2>/dev/null | base64 | tr -dc 'A-Za-z0-9' | cut -c 1-32)"
  fi
  printf '%s._%%-\n' "${base}"
}

env_value() {
  key="$1"
  awk -F= -v key="${key}" '$1 == key { sub(/^[^=]*=/, ""); print; exit }' "${env_file}" 2>/dev/null || true
}

set_env_value() {
  key="$1"
  value="$2"
  tmp="$(mktemp)"
  awk -v key="${key}" -v value="${value}" '
    $0 ~ "^[#[:space:]]*" key "=" && done == 0 {
      print key "=" value
      done = 1
      next
    }
    { print }
    END {
      if (done == 0) {
        print key "=" value
      }
    }
  ' "${env_file}" > "${tmp}"
  mv "${tmp}" "${env_file}"
}

ensure_secret() {
  key="$1"
  value="$(env_value "${key}")"
  case "${value}" in
    "" | change-me*)
      set_env_value "${key}" "$(rand_secret)"
      printf '%s\n' "Generated ${key} in ${env_file}."
      ;;
  esac
}

setting_needs_value() {
  value="$1"
  case "${value}" in
    "" | change-me* | /path/to/* | "http://change-me"* | "https://change-me"*)
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

ensure_setting() {
  key="$1"
  default_value="$2"
  value="$(env_value "${key}")"
  if setting_needs_value "${value}"; then
    set_env_value "${key}" "${default_value}"
    printf '%s\n' "Set ${key}=${default_value} in ${env_file}."
  fi
}

detect_docker_gid() {
  if [ -S /var/run/docker.sock ]; then
    stat -c '%g' /var/run/docker.sock 2>/dev/null || stat -f '%g' /var/run/docker.sock
  else
    printf '%s\n' "0"
  fi
}

detect_primary_ip() {
  # Prefer the source IP of the default route - that is the address a
  # browser on the same network would use to reach this box, and it
  # avoids DNS surprises with the hostname.
  ip="$(ip route get 1.1.1.1 2>/dev/null | awk '{for(i=1;i<=NF;i++) if($i=="src"){print $(i+1); exit}}')"
  if [ -z "${ip}" ]; then
    ip="$(hostname -I 2>/dev/null | awk '{print $1}')"
  fi
  if [ -z "${ip}" ]; then
    ip="$(hostname 2>/dev/null || printf '%s' localhost)"
  fi
  printf '%s' "${ip}"
}

detect_host_url() {
  port="$1"
  host="$(detect_primary_ip)"
  case "${host}" in
    "") host="localhost" ;;
  esac
  printf 'http://%s:%s\n' "${host}" "${port}"
}

detect_host_url_https() {
  port="$1"
  host="$(detect_primary_ip)"
  case "${host}" in
    "") host="localhost" ;;
  esac
  printf 'https://%s:%s\n' "${host}" "${port}"
}

detect_agent_name() {
  hostname -s 2>/dev/null || hostname 2>/dev/null || printf '%s\n' compose-agent
}

created_env=0
if [ ! -f "${env_file}" ]; then
  if [ ! -f "${example_env}" ]; then
    printf '%s\n' "${env_file} does not exist and ${example_env} was not found."
    exit 1
  fi
  cp "${example_env}" "${env_file}"
  chmod 600 "${env_file}"
  created_env=1
  printf '%s\n' "Created ${env_file} from .env.example."
fi

if [ "${agent_mode}" -eq 1 ]; then
  # APP_MODE default for agent installs is outbound check-in. Operators can
  # override to "agent" (inbound listener) or "agent-both" (combined) either
  # inline (APP_MODE=agent docker compose ... up -d) or by editing .env.
  ensure_setting APP_MODE agent-callback
  ensure_setting AGENT_NAME "$(detect_agent_name)"
  ensure_secret AGENT_TOKEN
  ensure_setting AGENT_PORT 8192
  # Force-set instead of ensure_setting so the .env.example placeholder is
  # replaced; operators still need to point this at their real controller.
  set_env_value AGENT_CONTROLLER_URL https://change-me:8993
  ensure_setting AGENT_CHECKIN_SECONDS 60
else
  set_env_value APP_MODE server
  ensure_setting ADMIN_USERNAME admin
  ensure_setting DB_NAME stack_manager
  ensure_setting DB_USER stack_manager
  ensure_setting REDIS_DB 0
  ensure_setting CACHE_TTL_SECONDS 15
  ensure_setting METRICS_REFRESH_MINUTES 15
  ensure_setting WARM_CACHE_TTL_MINUTES 30
fi
ensure_setting DOCKER_ROOT "${HOME}/docker"
if [ "${agent_mode}" -eq 1 ]; then
  ensure_setting ROOT "$(env_value DOCKER_ROOT)"
fi
ensure_setting STATE_DIR .stack-manager
ensure_setting BACKUP_TARGET_ROOT .stack-manager/backup-targets
if [ "${created_env}" -eq 1 ]; then
  set_env_value DOCKER_GID "$(detect_docker_gid)"
  set_env_value SERVER_USER "$(id -u):$(id -g)"
  printf '%s\n' "Set host-specific DOCKER_GID and SERVER_USER in ${env_file}."
else
  ensure_setting DOCKER_GID "$(detect_docker_gid)"
  ensure_setting SERVER_USER "$(id -u):$(id -g)"
fi
if [ "${agent_mode}" -eq 1 ]; then
  ensure_setting HOST_URL "$(detect_host_url "$(env_value AGENT_PORT)")"
else
  ensure_setting WEB_HTTP_PORT 8193
  ensure_setting WEB_SSL_PORT 8993
  ensure_secret API_KEY
  ensure_secret ADMIN_PASSWORD
  ensure_secret DB_PASSWORD
  ensure_secret DB_ROOT_PASSWORD
  ensure_secret REDIS_PASSWORD
  ensure_setting HOST_URL "$(detect_host_url_https "$(env_value WEB_SSL_PORT)")"
fi
chmod 600 "${env_file}"

if [ -f "${env_file}" ]; then
  set -a
  # shellcheck disable=SC1090
  . "${env_file}"
  set +a
fi

default_uid="$(id -u)"
default_gid="$(id -g)"
server_user="${SERVER_USER:-${default_uid}:${default_gid}}"
case "${server_user}" in
  *:*) ;;
  *)
    printf '%s\n' "SERVER_USER must be a numeric UID:GID pair, such as 0:0, 1000:1000, or 998:998."
    exit 1
    ;;
esac
state_uid="${STATE_UID:-${server_user%%:*}}"
state_gid="${STATE_GID:-${server_user#*:}}"
case "${state_uid}:${state_gid}" in
  *[!0-9:]* | :* | *: | *:*:*)
    printf '%s\n' "SERVER_USER must be a numeric UID:GID pair, such as 0:0, 1000:1000, or 998:998."
    exit 1
    ;;
esac

state_dir="${STATE_DIR:-.stack-manager}"
case "${state_dir}" in
  /*) ;;
  *) state_dir="${project_root}/${state_dir}" ;;
esac
backup_target_root="${BACKUP_TARGET_ROOT:-.stack-manager/backup-targets}"
case "${backup_target_root}" in
  /*) ;;
  *) backup_target_root="${project_root}/${backup_target_root}" ;;
esac
if [ -z "${DOCKER_ROOT:-}" ]; then
  printf '%s\n' "DOCKER_ROOT is required. Set it in ${env_file} to the directory containing managed Compose projects."
  exit 1
fi
docker_root="${DOCKER_ROOT}"

stat_owner() {
  stat -c '%u:%g' "$1" 2>/dev/null || stat -f '%u:%g' "$1"
}

mkdir -p \
  "${docker_root}" \
  "${state_dir}" \
  "${state_dir}/hooks" \
  "${state_dir}/backups" \
  "${state_dir}/backups/db-dumps" \
  "${state_dir}/docker-config" \
  "${state_dir}/jobs" \
  "${state_dir}/ssl" \
  "${state_dir}/ssl/acme-webroot" \
  "${backup_target_root}"

if [ "${agent_mode}" -eq 0 ]; then
  mkdir -p "${state_dir}/mariadb" "${state_dir}/redis"
fi

if [ "$(id -u)" -eq 0 ]; then
  chown "${state_uid}:${state_gid}" "${state_dir}"
  chown -R "${state_uid}:${state_gid}" "${state_dir}/hooks" "${state_dir}/backups" "${state_dir}/docker-config" "${state_dir}/jobs" "${state_dir}/ssl" "${backup_target_root}"
else
  current_owner="$(stat_owner "${state_dir}")"
  expected_owner="${state_uid}:${state_gid}"
  if [ "${current_owner}" != "${expected_owner}" ]; then
    printf '%s\n' "State directory ${state_dir} is owned by ${current_owner}, expected ${expected_owner}."
    printf '%s\n' "Run: sudo chown -R ${expected_owner} ${state_dir}"
    exit 1
  fi
fi

find "${state_dir}" "${state_dir}/hooks" "${state_dir}/backups" "${state_dir}/docker-config" "${state_dir}/jobs" "${backup_target_root}" -type d -exec chmod 2770 {} +
find "${state_dir}/hooks" "${state_dir}/backups" "${state_dir}/docker-config" "${state_dir}/jobs" "${backup_target_root}" -type f -exec chmod 660 {} +
# SSL dir: needs to be readable by both the Go server (SERVER_USER) and the
# web container (also SERVER_USER now), with the ACME webroot writable so
# certbot helper containers can drop challenge files.
chmod 2775 "${state_dir}/ssl" "${state_dir}/ssl/acme-webroot"

printf '%s\n' "Prepared ${state_dir} for ${state_uid}:${state_gid}."
printf '%s\n' "Prepared ${backup_target_root} for backup endpoint mounts."

printf '%s\n' ""
printf '%s\n' "Stack Manager settings written to ${env_file}:"
if [ "${agent_mode}" -eq 1 ]; then
  for key in \
    APP_MODE \
    HOST_URL \
    AGENT_NAME \
    AGENT_TOKEN \
    AGENT_PORT \
    AGENT_CONTROLLER_URL \
    AGENT_CHECKIN_SECONDS \
    DOCKER_ROOT \
    ROOT \
    STATE_DIR \
    BACKUP_TARGET_ROOT \
    DOCKER_GID \
    SERVER_USER
  do
    printf '%s=%s\n' "${key}" "$(env_value "${key}")"
  done
else
  for key in \
    APP_MODE \
    HOST_URL \
    API_KEY \
    ADMIN_USERNAME \
    ADMIN_PASSWORD \
    DB_NAME \
    DB_USER \
    DB_PASSWORD \
    DB_ROOT_PASSWORD \
    REDIS_PASSWORD \
    REDIS_DB \
    CACHE_TTL_SECONDS \
    METRICS_REFRESH_MINUTES \
    WARM_CACHE_TTL_MINUTES \
    DOCKER_ROOT \
    STATE_DIR \
    BACKUP_TARGET_ROOT \
    DOCKER_GID \
    SERVER_USER \
    WEB_HTTP_PORT \
    WEB_SSL_PORT
  do
    printf '%s=%s\n' "${key}" "$(env_value "${key}")"
  done
fi

# Friendly login summary. Prefer the HOST_URL the operator (or CI) already
# picked; otherwise build one from the detected FQDN so it never falls back
# to 127.0.0.1 or localhost when a real hostname exists.
login_url="$(env_value HOST_URL)"
if [ -z "${login_url}" ] || [ "${login_url}" = "http://change-me:8993" ] || [ "${login_url}" = "https://change-me:8993" ]; then
  login_url="$(detect_host_url_https "$(env_value WEB_SSL_PORT)")"
fi

printf '\n'
printf '============================================================\n'
if [ "${agent_mode}" -eq 1 ]; then
  printf 'Stack Manager AGENT is ready.\n'
  printf '\n'
  printf 'Agent URL:   %s\n' "${login_url}"
  printf 'Agent name:  %s\n' "$(env_value AGENT_NAME)"
  printf 'Agent token: %s\n' "$(env_value AGENT_TOKEN)"
  printf '\n'
  printf 'Register from the controller: Settings > Agents.\n'
else
  printf 'Stack Manager is ready.\n'
  printf '\n'
  printf 'Dashboard URL: %s\n' "${login_url}"
  printf 'Login:         %s\n' "$(env_value ADMIN_USERNAME)"
  printf 'Password:      %s\n' "$(env_value ADMIN_PASSWORD)"
  printf '\n'
  printf 'API key (for scripts): %s\n' "$(env_value API_KEY)"
fi
printf '============================================================\n'
