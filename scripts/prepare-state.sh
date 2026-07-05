#!/usr/bin/env sh
set -eu

script_dir="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
project_root="$(dirname "${script_dir}")"
env_file="${1:-${project_root}/.env}"
example_env="${project_root}/.env.example"

rand_secret() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex 32
  else
    dd if=/dev/urandom bs=48 count=1 2>/dev/null | base64 | tr -dc 'A-Za-z0-9' | cut -c 1-48
  fi
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
    "" | change-me* | /path/to/* | "http://change-me"*)
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

detect_host_url() {
  port="$1"
  host="$(hostname -f 2>/dev/null || hostname 2>/dev/null || printf '%s' localhost)"
  case "${host}" in
    "") host="localhost" ;;
  esac
  printf 'http://%s:%s\n' "${host}" "${port}"
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

ensure_setting ADMIN_USERNAME admin
ensure_setting DB_NAME compose_manager
ensure_setting DB_USER compose_manager
ensure_setting REDIS_DB 0
ensure_setting CACHE_TTL_SECONDS 15
ensure_setting DOCKER_ROOT "${HOME}/docker"
ensure_setting STATE_DIR .compose-manager
if [ "${created_env}" -eq 1 ]; then
  set_env_value DOCKER_GID "$(detect_docker_gid)"
  set_env_value SERVER_USER "$(id -u):$(id -g)"
  printf '%s\n' "Set host-specific DOCKER_GID and SERVER_USER in ${env_file}."
else
  ensure_setting DOCKER_GID "$(detect_docker_gid)"
  ensure_setting SERVER_USER "$(id -u):$(id -g)"
fi
ensure_setting WEB_PORT 8193
ensure_secret API_KEY
ensure_secret ADMIN_PASSWORD
ensure_secret DB_PASSWORD
ensure_secret DB_ROOT_PASSWORD
ensure_secret REDIS_PASSWORD
ensure_setting HOST_URL "$(detect_host_url "$(env_value WEB_PORT)")"
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

state_dir="${STATE_DIR:-.compose-manager}"
case "${state_dir}" in
  /*) ;;
  *) state_dir="${project_root}/${state_dir}" ;;
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
  "${state_dir}/mariadb" \
  "${state_dir}/redis"

if [ "$(id -u)" -eq 0 ]; then
  chown -R "${state_uid}:${state_gid}" "${state_dir}"
else
  current_owner="$(stat_owner "${state_dir}")"
  expected_owner="${state_uid}:${state_gid}"
  if [ "${current_owner}" != "${expected_owner}" ]; then
    printf '%s\n' "State directory ${state_dir} is owned by ${current_owner}, expected ${expected_owner}."
    printf '%s\n' "Run: sudo chown -R ${expected_owner} ${state_dir}"
    exit 1
  fi
fi

find "${state_dir}" -type d -exec chmod 2770 {} +
find "${state_dir}" -type f -exec chmod 660 {} +

printf '%s\n' "Prepared ${state_dir} for ${state_uid}:${state_gid}."

printf '%s\n' ""
printf '%s\n' "Compose Manager settings written to ${env_file}:"
for key in \
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
  DOCKER_ROOT \
  STATE_DIR \
  DOCKER_GID \
  SERVER_USER \
  WEB_PORT
do
  printf '%s=%s\n' "${key}" "$(env_value "${key}")"
done
