# Stack Manager — Platform Guide

Install, upgrade, and operate Stack Manager itself (not the stacks it manages).
For per-application stack docs, see the **Stack Catalog** / the Documentation
page in the UI.

- [Overview](#overview)
- [Requirements](#requirements)
- [Install](#install)
- [Upgrade](#upgrade)
- [Manual upgrade from an old version](#manual-upgrade-from-an-old-version)
- [TLS / SSL](#tls--ssl)
- [Host helper scripts](#host-helper-scripts)
- [Backups](#backups)
- [Base-image routing](#base-image-routing)
- [Troubleshooting](#troubleshooting)

---

## Overview

Stack Manager ships two surfaces from one repo:

- **`stack-manager.sh`** — a single-file Bash CLI for discovering and managing
  Compose projects under a root directory. No dependencies beyond Docker Compose.
- **`server/` (Go API) + `web/` (React SPA)** — the dashboard: logs, stats,
  backups, DB checks, security scans, registry login, templates, schedules,
  agents, per-project shell, and per-stack volumes/networks.

The same Go binary runs in two modes:

- **Controller** (`APP_MODE` unset) — full stack with MariaDB + Redis, routes
  under `/api/v1`, session/bearer auth.
- **Agent** (`APP_MODE=agent*`) — no MariaDB/Redis/UI, routes under `/agent/v1`,
  bearer-token auth via `AGENT_TOKEN`. Registered from a controller's
  **Settings → Agents**.

Two directories you should never confuse:

| | |
|---|---|
| **`STATE_DIR`** (default `.stack-manager/`) | Stack Manager's own state: MariaDB data, Redis AOF, backups, registry docker-config, and TLS material under `ssl/`. |
| **`DOCKER_ROOT`** | The host directory holding the managed Compose projects. Host-specific; can be any path. |

---

## Requirements

- Linux host with Docker Engine + the Docker Compose plugin.
- The server user needs access to the Docker socket (`DOCKER_GID` must match
  `stat -c '%g' /var/run/docker.sock`).
- Ports for the dashboard (default `8193` HTTP → `8993` HTTPS).
- `git` on the host if you want the UI "Update now" button or `deploy.sh`.

---

## Install

### Web dashboard (recommended)

```bash
mkdir -p ~/docker && cd ~/docker
git clone https://github.com/arphost-com/Stack-Manager.git stack-manager
cd stack-manager
./scripts/prepare-state.sh .env
docker compose --env-file .env up -d --build
```

`prepare-state.sh` creates `.env` from `.env.example`, replaces every
`change-me…` placeholder with a random 36-char secret, detects `DOCKER_GID` and
`SERVER_USER` from the host, prepares the state directories with correct
ownership, and prints the login URL + credentials. **Re-running it never
overwrites existing secrets.**

Prefer `./scripts/deploy.sh` over a bare `docker compose up` when you want the
UI footer version stamped and the host helper scripts installed automatically —
see [Upgrade](#upgrade).

### CLI-only (no dashboard)

```bash
chmod +x stack-manager.sh
./stack-manager.sh --root ~/docker list
./stack-manager.sh --root ~/docker --dry-run update
```

Flags must come **before** the command. `-p` is `--prune`, not `--project`.

### Agent (managed node)

On the node to be managed:

```bash
git clone https://github.com/arphost-com/Stack-Manager.git stack-manager
cd stack-manager
./scripts/prepare-state.sh --agent --mode callback .env   # or --mode inbound / both
# Set DOCKER_ROOT and AGENT_CONTROLLER_URL in .env, then:
docker compose --env-file .env -f docker-compose.agent.yml up -d --build
```

`prepare-state.sh --agent` writes `APP_MODE`, `AGENT_NAME`, `AGENT_TOKEN`, and
`AGENT_PORT`, and prints the values to register in the controller's
**Settings → Agents**. Agent modes: `callback` (outbound check-in, best behind
NAT), `inbound` (controller reaches the agent), or `both`.

---

## Upgrade

Three supported paths, all equivalent in result:

### 1. UI — Settings → Update → "Update now" (no SSH)

Pulls the latest code and rebuilds the stack **detached on the host** so it
survives the server/web containers restarting. The panel shows how many commits
you're behind and **what's in the update** (the pending commit subjects). One
time per host, install the update helper (the panel shows the exact command):

```bash
sudo install -m 750 scripts/stack-manager-update.sh /usr/local/sbin/stack-manager-update
```

### 2. `deploy.sh` (SSH, recommended for a full refresh)

```bash
cd ~/docker/stack-manager
git pull            # or: git fetch && git reset --hard origin/main
./scripts/deploy.sh
```

`deploy.sh` runs `prepare-state.sh`, rebuilds with the correct version stamp
(`VITE_GIT_SHA`), **installs/refreshes all host helper scripts**
(gpu/os/update/csf/tz), and records the deployed version in the DB. Use this —
not a bare `docker compose up` — after pulling, or the web bundle can serve a
**stale cached build** (wrong footer version, and a stale frontend against a
newer server shows as "invalid request body").

### 3. Manual

```bash
cd ~/docker/stack-manager
git pull
./scripts/prepare-state.sh .env
docker compose --env-file .env up -d --build
```

> After changing Dockerfiles, nginx, or server config, always use
> `up -d --build` (not plain `up -d`) — plain `up -d` reuses old images.

---

## Manual upgrade from an old version

Bringing a long-stale install current. The app is designed to make this safe,
but a few things are easy to get wrong:

1. **Preserve your secrets.** Never regenerate `.env` from scratch.
   `prepare-state.sh` only fills `change-me…` placeholders and leaves existing
   secrets alone, so re-running it is safe. If you must edit `.env`, keep
   `API_KEY`, `DB_PASSWORD`, `DB_ROOT_PASSWORD`, `REDIS_PASSWORD`, and
   `ADMIN_PASSWORD`.

2. **Settings moved into the database.** Newer versions store runtime settings
   (API key, app version, timezone, `CACHE_TTL_SECONDS`, `DOCKER_DAEMON_DIR`,
   `HOST_URL`, extra roots, metrics/cache intervals) in the DB, **seeded from
   `.env` on first boot**. So your existing `.env` API key keeps working after
   the upgrade — the DB is seeded from it. After upgrading you can roll the API
   key and edit these values under **Settings → General** (they no longer need
   to live in `.env`).

3. **Do not chown the state data dirs.** If `STATE_DIR` ownership is wrong after
   a stack came up as root: stop the stack, `chown -R "$SERVER_USER"
   "$STATE_DIR"`, re-run `./scripts/prepare-state.sh .env`, and bring it back
   up. **Never** chown `STATE_DIR/mariadb` or `STATE_DIR/redis` after those
   services initialized — MariaDB/Redis own their data files. Never recursively
   chown `DOCKER_ROOT` (it holds other people's projects).

4. **Version footer.** The server stamps its own build version into the DB on
   startup, so after any rebuild the footer reflects the deployed commit. If it
   looks stale, you rebuilt with a bare `docker compose` (no `VITE_GIT_SHA`) —
   redeploy with `./scripts/deploy.sh` or the UI Update button.

5. **Reinstall the host helpers** (gpu/os/update/csf/tz) if their scripts
   changed — `deploy.sh` does this for you; otherwise
   `sudo install -m 750 scripts/stack-manager-<name>.sh /usr/local/sbin/stack-manager-<name>`.

6. **If CSF (firewall) was uninstalled**, Docker's iptables NAT chain is
   flushed with it — `systemctl restart docker` (the CSF helper now schedules
   this automatically on uninstall).

7. **Ports / TLS.** To move from the default self-signed HTTPS to Let's Encrypt,
   set `WEB_HTTP_PORT=80` / `WEB_SSL_PORT=443`, redeploy, then issue via
   **Settings → SSL**. See [TLS / SSL](#tls--ssl).

---

## TLS / SSL

The web container terminates TLS. Default install serves HTTP on `WEB_HTTP_PORT`
(8193) redirecting to HTTPS on `WEB_SSL_PORT` (8993) with a **self-signed cert
generated on first boot**. Cert files live at `${STATE_DIR}/ssl/fullchain.pem`
and `privkey.pem`; `${STATE_DIR}/ssl/mode` records `self-signed` vs
`letsencrypt`.

To use Let's Encrypt: set `WEB_HTTP_PORT=80` and `WEB_SSL_PORT=443`, redeploy,
then **Settings → SSL** issues via HTTP-01 (the host must be reachable on 80 for
a real domain). The HTTP server block always serves
`/.well-known/acme-challenge/` from `${STATE_DIR}/ssl/acme-webroot/` so renewals
work in either mode. Never delete or chown `ssl/` while nginx is running.

---

## Host helper scripts

Some panels drive the host through small root helpers at
`/usr/local/sbin/stack-manager-<name>`, run via a privileged `chroot /host`
container:

| Helper | Powers |
|---|---|
| `stack-manager-gpu` | GPU detect / test / one-click NVIDIA install |
| `stack-manager-os` | OS package updates (apt) |
| `stack-manager-update` | Self-update (pull + rebuild, detached via systemd) |
| `stack-manager-csf` | ConfigServer csf/lfd firewall |
| `stack-manager-tz` | Host system timezone (Debian + Ubuntu) |

`deploy.sh` installs/refreshes all of them. Until a helper is installed, its
panel shows a one-time `sudo install …` hint instead of its controls.

---

## Backups

Project backups are always created locally under `BACKUP_DIR` first, then
copied/uploaded to the selected endpoint. Endpoint types (Linux path, Mounted
path, CIFS, NFS, FTP, SFTP, S3) are managed in **Settings → Backup Endpoints**;
secrets are never returned to the browser after save. If a non-root server user
can't read project data (Postgres/MariaDB/Redis bind mounts), backup falls back
to a short-lived root helper container — this is intentional; do not chown
project bind mounts to "fix" it.

---

## Base-image routing

Every Docker Hub base image in this repo is prefixed with `${BASE_IMAGE_PREFIX}`.
Empty (the default) pulls straight from Docker Hub. On a network with a registry
proxy, set e.g.
`BASE_IMAGE_PREFIX=10.10.10.96:8929/arphost/dependency_proxy/containers/library/`
(trailing slash required) to route every base image through the proxy and avoid
Docker Hub rate limits.

---

## Troubleshooting

- **Footer version looks stale / "invalid request body" after an update** — the
  web bundle was rebuilt without `VITE_GIT_SHA` (bare `docker compose`). Redeploy
  with `./scripts/deploy.sh` or the UI Update button.
- **"Update now" logs a start line then nothing** — the update helper on that
  host is an old build; reinstall it (`deploy.sh` or the `sudo install` command).
  The current helper runs the rebuild under systemd so it survives the restart.
- **`deploy.sh` fails with `chmod: Operation not permitted`** — a prior root
  self-update left root-owned files. `sudo chown -R <deploy-user>:<deploy-user>`
  the tree **excluding `.stack-manager`**, then retry. The current self-update
  re-owns the tree automatically.
- **Docker Settings: `daemon.json is not valid JSON: invalid character 'U'`** —
  transient on first use of the helper image (Docker's "Unable to find image…"
  pull line); refresh once the image is cached. Fixed in current builds.
- **Web container won't bind ports after CSF was removed** —
  `systemctl restart docker` to rebuild Docker's iptables chains.
