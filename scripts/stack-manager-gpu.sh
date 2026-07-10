#!/usr/bin/env bash
# Stack Manager GPU host helper.
#
# Installs the NVIDIA container-GPU stack (driver + nvidia-container-toolkit)
# and registers the Docker `nvidia` runtime on Debian and Ubuntu hosts, so
# containers can use `--gpus all` / deploy.resources.reservations.devices.
#
# This runs on the HOST (the Stack Manager server invokes it inside the host
# namespaces via a privileged helper, since driver install + reboot can't be
# done from inside the app container). It is deliberately idempotent and logs
# each step so the UI can stream progress.
#
# Usage:
#   stack-manager-gpu.sh status     # report distro, driver, toolkit, runtime
#   stack-manager-gpu.sh install    # install toolkit + driver, configure docker
#   stack-manager-gpu.sh toolkit    # install only the container toolkit + runtime
#   stack-manager-gpu.sh uninstall  # remove driver + toolkit (for testing)
#   stack-manager-gpu.sh reboot     # reboot the host
set -euo pipefail

log() { printf '[gpu-setup] %s\n' "$*"; }
die() { printf '[gpu-setup] ERROR: %s\n' "$*" >&2; exit 1; }

require_root() { [ "$(id -u)" -eq 0 ] || die "must run as root"; }

detect_os() {
  # Sets OS_ID (debian|ubuntu) and OS_CODENAME.
  [ -r /etc/os-release ] || die "cannot read /etc/os-release"
  # shellcheck disable=SC1091
  . /etc/os-release
  OS_ID="${ID:-}"
  OS_CODENAME="${VERSION_CODENAME:-}"
  case "$OS_ID" in
    ubuntu | debian) : ;;
    *) die "unsupported distro '$OS_ID' (only debian and ubuntu are supported)" ;;
  esac
}

have() { command -v "$1" >/dev/null 2>&1; }

secureboot_state() {
  # echoes: enabled | disabled | unknown
  if have mokutil; then
    case "$(mokutil --sb-state 2>/dev/null)" in
      *enabled*) echo enabled ;;
      *disabled*) echo disabled ;;
      *) echo unknown ;;
    esac
  else
    echo unknown
  fi
}

driver_loaded() { lsmod 2>/dev/null | grep -q '^nvidia'; }

cmd_status() {
  detect_os
  echo "os=$OS_ID codename=$OS_CODENAME kernel=$(uname -r)"
  if have nvidia-smi && nvidia-smi >/dev/null 2>&1; then
    echo "driver=working $(nvidia-smi --query-gpu=name,driver_version --format=csv,noheader 2>/dev/null | head -1)"
  elif ls /lib/modules/"$(uname -r)"/updates/dkms/nvidia.ko* >/dev/null 2>&1 || dpkg -l 2>/dev/null | grep -q nvidia-dkms; then
    echo "driver=built-not-loaded"
  else
    echo "driver=missing"
  fi
  have nvidia-ctk && echo "toolkit=installed" || echo "toolkit=missing"
  docker info --format '{{.Runtimes}}' 2>/dev/null | grep -q nvidia && echo "runtime=nvidia-registered" || echo "runtime=missing"
  echo "secureboot=$(secureboot_state)"
  driver_loaded && echo "module=loaded" || echo "module=not-loaded"
  # Pending MOK enrollment (module signing key waiting for console confirm).
  if have mokutil && mokutil --list-new 2>/dev/null | grep -qiE 'Module Signature|CN='; then
    echo "mok=pending-enroll"
  fi
  lspci 2>/dev/null | grep -iE 'nvidia' | sed 's/^/gpu=/' || true
}

# handle_secureboot stages MOK enrollment when Secure Boot is on and the DKMS
# module is signed with a machine-owner key that isn't enrolled yet. The final
# enroll step must be confirmed at the host/VM console (Secure Boot requirement).
handle_secureboot() {
  [ "$(secureboot_state)" = "enabled" ] || { log "Secure Boot off — no MOK step needed"; return 0; }
  driver_loaded && { log "module already loaded"; return 0; }
  local mok=/var/lib/shim-signed/mok/MOK.der
  [ -f "$mok" ] || { log "Secure Boot on but no MOK cert at $mok — module may be unsigned"; return 0; }
  if mokutil --list-new 2>/dev/null | grep -qiE 'Module Signature|CN='; then
    log "MOK enrollment already staged — confirm it at the console"
    return 0
  fi
  local pw="${GPU_MOK_PASSWORD:-nvidia}"
  log "staging MOK enrollment (one-time password: $pw)"
  printf '%s\n%s\n' "$pw" "$pw" | mokutil --import "$mok" >/dev/null 2>&1 || true
  log "MOK staged. Reboot, then at the 'Perform MOK Management' console screen choose Enroll MOK and enter: $pw"
}

add_toolkit_repo() {
  local key=/usr/share/keyrings/nvidia-container-toolkit-keyring.gpg
  log "adding nvidia-container-toolkit apt repo"
  curl -fsSL https://nvidia.github.io/libnvidia-container/gpgkey | gpg --dearmor -o "$key"
  curl -s -L https://nvidia.github.io/libnvidia-container/stable/deb/nvidia-container-toolkit.list \
    | sed "s#deb https://#deb [signed-by=$key] https://#g" \
    > /etc/apt/sources.list.d/nvidia-container-toolkit.list
  apt-get update -q
}

install_toolkit() {
  have nvidia-ctk && { log "toolkit already installed"; } || {
    add_toolkit_repo
    log "installing nvidia-container-toolkit"
    DEBIAN_FRONTEND=noninteractive apt-get install -y -q nvidia-container-toolkit
  }
  log "configuring docker nvidia runtime"
  nvidia-ctk runtime configure --runtime=docker
  # NOTE: docker is restarted later via schedule_docker_restart, not here —
  # this helper usually runs inside a container, and restarting dockerd mid-run
  # would kill it before the remaining steps finish.
}

# schedule_docker_restart restarts docker a few seconds later, in the
# background, so it fires AFTER this (containerized) helper exits.
schedule_docker_restart() {
  log "scheduling docker restart (applies the nvidia runtime)"
  nohup sh -c 'sleep 3; systemctl restart docker' >/dev/null 2>&1 &
}

install_driver() {
  if have nvidia-smi && nvidia-smi >/dev/null 2>&1; then
    log "driver already working"; return 0
  fi
  detect_os
  # Kernel headers are needed to build the module.
  DEBIAN_FRONTEND=noninteractive apt-get install -y -q "linux-headers-$(uname -r)" || true
  if [ "$OS_ID" = "ubuntu" ]; then
    log "installing NVIDIA driver via ubuntu-drivers (recommended)"
    DEBIAN_FRONTEND=noninteractive apt-get install -y -q ubuntu-drivers-common
    ubuntu-drivers install || DEBIAN_FRONTEND=noninteractive apt-get install -y -q nvidia-driver-535
  else
    # Debian: driver lives in contrib/non-free.
    log "enabling contrib/non-free and installing nvidia-driver (Debian)"
    if ! grep -rhqE 'non-free' /etc/apt/sources.list /etc/apt/sources.list.d/ 2>/dev/null; then
      sed -i 's/ main$/ main contrib non-free non-free-firmware/' /etc/apt/sources.list || true
    fi
    apt-get update -q
    DEBIAN_FRONTEND=noninteractive apt-get install -y -q nvidia-driver
  fi
}

cmd_install() {
  require_root
  detect_os
  log "starting GPU setup on $OS_ID $OS_CODENAME"
  install_toolkit
  install_driver
  handle_secureboot
  schedule_docker_restart
  log "install complete — a reboot is required to load the NVIDIA kernel module"
}

cmd_uninstall() {
  require_root
  log "removing NVIDIA driver and toolkit (test teardown)"
  DEBIAN_FRONTEND=noninteractive apt-get purge -y -q 'nvidia-*' 'libnvidia-*' nvidia-container-toolkit nvidia-container-toolkit-base libnvidia-container1 libnvidia-container-tools 2>/dev/null || true
  DEBIAN_FRONTEND=noninteractive apt-get autoremove -y -q || true
  rm -f /etc/apt/sources.list.d/nvidia-container-toolkit.list /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg
  log "uninstall complete — reboot to fully unload the module"
}

cmd_reboot() { require_root; log "rebooting host"; ( sleep 2; systemctl reboot ) & }

case "${1:-status}" in
  status) cmd_status ;;
  install) cmd_install ;;
  toolkit) require_root; install_toolkit; schedule_docker_restart ;;
  uninstall) cmd_uninstall ;;
  reboot) cmd_reboot ;;
  *) die "unknown command '${1:-}' (use status|install|toolkit|uninstall|reboot)" ;;
esac
