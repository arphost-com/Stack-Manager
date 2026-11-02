# Stack Manager — Platform Guide

Operate Stack Manager itself, not the stacks it manages. For install and update
steps see [Installing Stack Manager](INSTALL.md), and for problems see
[Troubleshooting](TROUBLESHOOTING.md). For per-application stack docs, see the
Stack Catalog or the Documentation page in the UI.

- [Overview](#overview)
- [Requirements](#requirements)
- [Install and update](#install-and-update)
- [Manual upgrade from an old version](#manual-upgrade-from-an-old-version)
- [TLS / SSL](#tls--ssl)
- [Host helper scripts](#host-helper-scripts)
- [Timezone](#timezone)
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

- **Controller** (`APP_MODE` unset or `server`) — full stack with MariaDB + Redis, routes
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

## Install and update

The install, update, agent and CLI-only steps now live in one place,
[Installing Stack Manager](INSTALL.md). In short, run
`./scripts/prepare-state.sh .env` and then
`docker compose --env-file .env up -d --build` as a normal user in the `docker`
group, and update with **Settings > Update** or the same two commands after a
`git pull`. Hosts deployed by the GitLab pipeline are updated through the
pipeline.

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
   startup, and `prepare-state.sh` stamps the git commit into `.env`, so after
   any rebuild the footer reflects the deployed commit. If it looks stale, the
   images were not rebuilt. Run `docker compose --env-file .env up -d --build`.

5. **Host helpers** (gpu/os/update/csf/tz) are reinstalled automatically each
   time the stack starts, so a rebuild picks up changed helper scripts.

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

These are **installed automatically** every time the stack comes up — the
`state-init` sidecar copies them from `./scripts` to the host's
`/usr/local/sbin`, so the GPU/OS/Update/Firewall/Timezone panels activate on
first boot on every node with no manual step. `deploy.sh` also installs them.
Until a helper is present, its panel shows a one-time `sudo install …` hint.

---

## Timezone

**Settings → Timezone** sets the **host system clock** (via `timedatectl` on
Debian/Ubuntu, through the `stack-manager-tz` helper) — not just an app setting.
All Stack Manager containers mount `/etc/localtime` read-only, so after they
restart their logs, audit entries, backup filenames, and schedule times follow
the host's local time. The panel shows the host's current zone and, if the
helper isn't installed yet, the one-time install command.

---

## Backups

Project backups are always created locally under `BACKUP_DIR` first, then
copied/uploaded to the selected endpoint. Endpoint types (Linux path, Mounted
path, CIFS, NFS, FTP, SFTP, S3) are managed in **Settings → Backup Endpoints**;
secrets are never returned to the browser after save. If a non-root server user
can't read project data (Postgres/MariaDB/Redis bind mounts), backup falls back
to a short-lived root helper container — this is intentional; do not chown
project bind mounts to "fix" it.

Use the dedicated **Backups** page for single-project, selected-project, or all
active-project runs and for daily, weekly, monthly, or interval schedules.
Settings keeps the latest endpoint test result visible on the endpoint row so a
failed connection cannot be mistaken for a successful test.

---

## Base-image routing

Every Docker Hub base image in this repo is prefixed with `${BASE_IMAGE_PREFIX}`.
Empty (the default) pulls straight from Docker Hub. On a network with a registry
proxy, set e.g.
`BASE_IMAGE_PREFIX=10.10.10.96:8929/arphost/dependency_proxy/containers/library/`
(trailing slash required) to route every base image through the proxy and avoid
Docker Hub rate limits.

For docker02 GitLab deployments, the prefix defaults to empty and is not
preserved from an older `.env`. Set `STACK_MANAGER_BASE_IMAGE_PREFIX` in CI only
when the host has persistent pull credentials for that registry; job-scoped
dependency-proxy credentials cannot support later UI helper pulls or detached
rebuilds.

---

## Troubleshooting

See [Troubleshooting](TROUBLESHOOTING.md).
