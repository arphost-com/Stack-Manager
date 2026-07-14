# Stack Manager

Manage all your Docker Compose stacks from one dashboard. Discover, deploy, update, back up, and monitor projects across a fleet of hosts — with a 200+ template catalog, scheduled updates, live shell access, firewall management, two-factor auth, and agents that phone home from behind NAT.

<!-- Replace with your own screenshot: docs/images/dashboard-dark.png -->

---

## Highlights

- **200+ one-click stack templates** — AI, databases, CMS, monitoring, proxies, dev tools, media, and more. Pick a template, review the compose and env, and spin it up.
- **In-browser config editor** — edit compose.yml, .env, Caddyfile, and other project files directly from the dashboard with automatic .bak backups.
- **Fleet management with agents & peers** — register remote Docker hosts as outbound (phone-home), inbound, or combined agents, or add another full install as a **peer controller**. The "All Servers" view shows every connected host; open and manage their projects, and act across all of them from one controller. Behind-NAT **callback agents** are managed through a command queue that runs on their next check-in. All cross-server traffic uses TLS 1.3.
- **GPU for AI stacks** — Settings > GPU detects the host GPU, one-click-installs the NVIDIA driver + toolkit, and runs a real `--gpus all` test container (nvidia-smi) to prove passthrough works. "Add GPU passthrough" is a checkbox on both the Stack Catalog and the Create Project form (baked in before deploy), or an **Enable GPU** action after the fact.
- **Per-stack volumes & networks** — inspect a project's Docker volumes and networks (with in-use containers) and safely delete them, scoped to that stack, from its detail page.
- **One-click self-update** — Settings > Update pulls and rebuilds the controller on the host (detached, survives the restart) and shows **what's in the update** (the pending commit subjects) before you run it.
- **Scheduled updates** — daily at 03:00, weekly on Saturday, monthly on the 15th, or every N minutes. Per-project update policies prevent accidental breakage.
- **Backup to anywhere** — local paths, CIFS, NFS, FTP, SFTP (with in-browser SSH key generation), and S3. Automatic local archive + remote copy.
- **Live in-browser shell** — xterm.js terminal with real PTY support that opens an interactive session inside any running container via WebSocket + docker exec. Tab completion, arrow keys, colors, and resize — as fast as SSH.
- **Firewall management** — install, configure, and monitor ConfigServer Firewall (csf/lfd) from the dashboard with structured port forms, testing-mode toggle, allow/deny lists, config editor, and live log viewer. Login IPs are auto-allowlisted. Docker compatibility is auto-configured.
- **Reverse proxy integration** — one-click **deploy** Nginx Proxy Manager, then add proxied domains from the dashboard: per-project **Add to Proxy**, a one-click **proxy the Stack Manager UI** target, and auto-filled forwards for running projects. Let's Encrypt stays separate for non-proxy installs.
- **Two-factor authentication** — TOTP (Google Authenticator / Authy) with QR enrollment, backup codes, and per-user enable/disable.
- **Self-signed TLS out of the box** — HTTPS on first boot with zero config. Optional Let's Encrypt or Nginx Proxy Manager for real domains.
- **General settings in the browser** — change ports, cache TTLs, host URL, a friendly **server display name** (shown in the server selector instead of the IP), the **timezone** (which sets the host system clock via `timedatectl` so all containers follow it), and roll the API key from Settings > General without touching .env or SSH.
- **Multiple Docker roots** — discover projects across more than one host directory via `EXTRA_DOCKER_ROOTS`.
- **Docker daemon settings** — edit `daemon.json` from the browser with tooltips, backups, and teardown guidance for network changes.
- **Security scans** — image vulnerability scanning and compose audit from the project detail page.
- **Database checks** — health checks and SQL dumps for MariaDB, MySQL, and PostgreSQL containers.
- **Audit log** — every mutating action recorded with actor, project, result, and timestamp, with quick "Updates run" / "Backups run" presets and an **Activity log** link from each project that deep-links to its entries.

---

## Quick Start

```bash
mkdir -p ~/docker && cd ~/docker
git clone https://github.com/arphost-com/Stack-Manager.git stack-manager
cd stack-manager
./scripts/prepare-state.sh .env
docker compose --env-file .env up -d --build
```

Open the URL printed by the setup script (default `https://<your-ip>:8993`). All passwords and keys are auto-generated on first run.

---

## Screenshots

Existing catalog screenshots:

![AI Stack Catalog](docs/images/catalog-personal-ai-agents-dark.png)
![Web Catalog](docs/images/catalog-web-dark.png)
![CMS Catalog](docs/images/catalog-cms-dark.png)
![Database Catalog](docs/images/catalog-database-light.png)
![Queue Catalog](docs/images/catalog-queue-light.png)
![Dev Tools Catalog](docs/images/catalog-devtools-light.png)

![Project Detail — Backups](docs/images/project-detail-backups.png)

<!-- Add more screenshots as you take them:
![Dashboard](docs/images/dashboard-dark.png)
![Project Detail](docs/images/project-detail.png)
![Settings General](docs/images/settings-general.png)
![Firewall Panel](docs/images/firewall-panel.png)
![Interactive Shell](docs/images/interactive-shell.png)
![Config Editor](docs/images/config-editor.png)
![TOTP Enrollment](docs/images/totp-enrollment.png)
![Reverse Proxy](docs/images/reverse-proxy.png)
![Scheduled Updates](docs/images/scheduled-updates.png)
-->

---

## Features in Detail

### Dashboard

The main page shows every discovered Compose project with live state, image sources, update availability, and one-click actions: start, stop, restart, update, pull, backup, and delete. Filter by running, stopped, inactive, or projects with available updates. Bulk actions apply to the filtered list or a manual selection.

### Stack Catalog (200+ Templates)

Browse and deploy from a curated catalog organized into 18 categories and 10 AI subcategories:

| Category | Examples |
|----------|----------|
| **AI** | Ollama + Open WebUI, OpenBrain agent stacks (workflow / memory / visual builder), Voice Assistant (Ollama + Open WebUI + Kokoro TTS, voice pre-wired), LibreChat, AnythingLLM, ComfyUI, Whisper, Langfuse |
| **Web & Proxy** | Nginx, Caddy, Traefik, Nginx Proxy Manager, Apache, ProxyForge (SOCKS5 + HTTP proxy admin) |
| **CMS** | WordPress, Ghost, Strapi, Directus, Payload, Concrete CMS |
| **Database** | PostgreSQL, MariaDB, Redis, MongoDB, CockroachDB, ClickHouse |
| **Dev Tools** | Gitea, Forgejo, code-server, Jenkins, Drone CI, Draw.io |
| **Monitoring** | Grafana + Prometheus, Uptime Kuma, Beszel, Netdata, cAdvisor |
| **Docs** | BookStack, Docmost, DokuWiki, Wiki.js, Outline |
| **Media** | Jellyfin, Plex, Radarr, Sonarr, Prowlarr, qBittorrent, Jellyseerr, Immich, Tube Archivist, ErsatzTV, Unpackerr |
| **Gaming** | EmulatorJS, RomM, Sunshine (Moonlight game streaming) |
| **Remote** | Webtop (full Linux desktop in browser), Neko (shared virtual browser), Apache Guacamole, RustDesk, Firefox |
| **Security** | Authelia, Keycloak, CrowdSec, Vaultwarden, ARPVPN, wg-easy, WireGuard-UI, Headscale, Pritunl |
| **Files** | Nextcloud, Seafile, MinIO, Paperless-ngx, ConvertX |
| **Finance** | Actual Budget, Firefly III, Wallos |
| **Productivity** | Karakeep, Linkwarden, Memos, Vikunja |
| **Queue** | RabbitMQ, Apache ActiveMQ, Beanstalkd, Faktory |
| **Automation** | n8n, Huginn, Cronicle, Changedetection.io, ntfy |

Templates load into an editable Create Project form — review ports, volumes, passwords, and env vars before deploying. Nothing deploys until you click Create. Templates that need a config file ship a working starter config embedded in the compose (via `configs:`), so they boot out of the box and you can edit the config any time from a project's Config tab.

### In-Browser Config Editor

The **Config** tab on each project detail page lets you edit compose.yml, .env, Dockerfile, Caddyfile, nginx.conf, and other config files directly from the browser. Every save creates a `.bak` backup. A hint reminds you to restart the stack after compose changes.

### Scheduled Updates

Set human-readable schedules from Settings or the Dashboard:

- **Daily** at a specific time (UTC)
- **Weekly** on a chosen day at a specific time
- **Monthly** on a day-of-month at a specific time
- **Every N minutes** for custom intervals

Each schedule targets a project (local or on a registered agent) and runs update, pull, up, restart, down, or status. Projects with a `no_updates` policy record a skipped session instead of pulling.

Projects with an enabled scheduled update are automatically excluded from the nightly background update check — no wasted Docker Hub pulls for manifests that the scheduler will check anyway.

### Backup Endpoints

Create backups from the project detail page or on a schedule. Archives are created locally first, then copied to any configured endpoint:

| Endpoint Type | Authentication |
|---------------|----------------|
| Linux / mounted path | Filesystem access |
| CIFS / NFS mount | Host-side mount |
| FTP | Username + password |
| SFTP | Password or SSH key (paste or generate from the browser) |
| S3 | Access key + secret key |

SFTP endpoints support in-browser Ed25519 key generation: click **Generate**, save the endpoint, copy the displayed public key into the remote server's `authorized_keys`. No host filesystem access needed.

### Fleet Agents & Peer Controllers

Manage Docker hosts across your network from one controller. The dashboard's **Server** selector shows **All Servers** — the local host plus every connected server — and you can filter to any single one. Add servers in Settings > Agents:

| Mode | How it works | Needs inbound port? | Manage projects? |
|------|-------------|---------------------|------------------|
| **Outbound check-in** (agent) | Agent phones home to the controller. Best behind NAT. | No | Yes, via queued commands |
| **Inbound listener** (agent) | Controller reaches out to the agent. | Yes | Yes, live |
| **Both** (agent) | Combined — works whether the host is reachable or not. | Yes | Yes, live |
| **Peer controller** | Another full Stack Manager. You add it by its URL + API key; the controller live-fetches its projects into the "All Servers" view. | Reachable over HTTPS | Yes, live |

Agents are a lightweight runtime (no database, Redis, or UI) installed with `./scripts/prepare-state.sh --agent --mode <callback\|inbound\|both> --controller https://<controller>:8993` — which auto-generates the `.env` (including `AGENT_TOKEN`), fills the controller URL (no `change-me` left behind), and prints the exact name/token to register. **Peer controllers** are two full installs that each add the other as a peer, so both dashboards see and (via the agent proxy) act on both hosts over direct HTTPS. All server-to-server traffic uses TLS 1.3.

Open any project in the "All Servers" view — including ones on a peer or agent. Peer/inbound projects are managed live. **Callback agents** can't be reached inbound, so opening their project shows a **Queued commands** panel: your up/down/pull/update/restart actions are queued and run on the agent's next check-in, with the output reported back. When a specific server is selected in the dropdown, bulk actions, Create Project, and Prune target *that* server.

### Interactive Shell

Open a live terminal inside any running container from the Shell tab. Powered by xterm.js + WebSocket + `docker exec` with a real PTY (via `creack/pty`). Full terminal support: colored prompt, tab completion, arrow keys, Ctrl-C, history, and automatic resize. Pick a container from the dropdown, click Connect, and type commands. Sessions are auth-gated and scoped to the project's containers.

### Firewall (ConfigServer csf/lfd)

Settings > Firewall provides full management of ConfigServer Firewall on the host. The server uses `nsenter` into the host's PID namespace so CSF operations (including `csf -r` which flushes all iptables) survive without killing the server container.

**Status dashboard:**
- Installed / not installed indicator with one-click Install from [Black-HOST/csf](https://github.com/Black-HOST/csf)
- Testing mode, LFD active, iptables rule count
- CSF version

**Firewall Settings panel:**
- **Testing mode** checkbox — disable after verifying your port rules
- **Docker mode** checkbox — auto-set during install on Docker hosts; ensures CSF accommodates Docker's iptables chains
- **SYN flood protection** checkbox
- **Syslog restrict** level (0–3)
- **TCP IN / TCP OUT / UDP IN / UDP OUT** — comma-separated port fields with hints explaining what each direction means
- Save + Restart csf buttons

**IP management:**
- Your detected IP shown with an **Add my IP** button
- Manual allow / deny form for any IPv4 or IPv6 address with a comment
- Allow list and deny list tables with per-entry Remove buttons
- Auto-allowlist on login — every successful dashboard login runs `csf -a` for the caller's IP

**Per-project port opening:**
- Any project has an **Open Ports (CSF)** button that adds the project's published TCP ports inbound to the host firewall (`TCP_IN`) and reloads CSF, backing up `csf.conf` first. Requires the firewall helper (`stack-manager-csf`) installed on that host.

**Config editor:**
- In-browser editor for `csf.conf`, `csf.allow`, `csf.deny`, `csf.ignore`, `csf.pignore`
- Every save creates a timestamped backup under `/var/backups/stack-manager-csf/`

**Log viewer:**
- Tail `/var/log/lfd.log` with configurable line count (up to 5000)

**Docker compatibility:**
- Install auto-sets `DOCKER = "1"` in `csf.conf`
- Writes `csfpre.sh` (saves iptables state) and `csfpost.sh` (restarts Docker after CSF reload) so `csf -r` never breaks container networking
- Installs `unzip`, `perl`, and `iptables` as prerequisites

**One-time host setup:**

```bash
sudo install -m 750 scripts/stack-manager-csf.sh /usr/local/sbin/stack-manager-csf
```

If the helper is not installed, the Firewall panel shows an amber install-command card with a Copy button instead of an error.

### Reverse Proxy (Nginx Proxy Manager)

Settings > Reverse Proxy deploys and manages Nginx Proxy Manager from the dashboard:

- **Deploy Nginx Proxy Manager** — one click stands up NPM from the built-in template and prefills the connection form with its admin URL and default login
- **Connect** with the NPM admin URL, email, and password (the auth request sends only `identity` + `secret`, which NPM's schema requires)
- **Add proxied domains** from a form — running projects appear as chips that auto-fill the forward target, plus a one-click **Stack Manager UI** target to proxy the dashboard itself
- **Add to Proxy (NPM)** on any project — one click creates a proxy host forwarding to the host + the project's first published port (domain and SSL editable in NPM after)
- **List / delete proxy hosts** from the table
- Let's Encrypt stays separate under Settings > SSL for installs that don't use the proxy; private networks and IP-only hosts can keep the built-in self-signed cert

### Two-Factor Authentication

Protect accounts with TOTP (Google Authenticator, Authy, or any compatible app):

1. Go to **Settings > Account > Set up 2FA**
2. Scan the QR code with your authenticator app
3. Save the 8 backup codes somewhere safe
4. Enter a 6-digit code to verify and enable

Once enabled, login requires a code after the password step. Admins can reset another user's 2FA from the Users tab.

### General Settings

Settings > General lets admins change `.env` values from the browser:

- **Ports** — WEB_SSL_PORT (default 8993), WEB_HTTP_PORT (default 8193)
- **Cache and refresh** — CACHE_TTL_SECONDS, METRICS_REFRESH_MINUTES, WARM_CACHE_TTL_MINUTES
- **Host URL** — the dashboard URL shown in setup output and agent commands
- **Extra Docker roots** — comma-separated additional directories to discover projects in
- **Roll API key** — generate a new API key with one click (takes effect on restart)

Port changes require a full `docker compose --env-file .env up -d` restart.

### Docker Settings

Edit the host Docker `daemon.json` from Settings > Docker Settings:

- Log driver and rotation
- DNS servers
- Default address pools
- Registry mirrors and insecure registries
- Live restore and IPv6
- Remote Docker TCP hosts (with security warnings)
- Raw JSON for advanced options

Every save creates a timestamped backup. Network-field changes show a full teardown guide.

### SSL / TLS

HTTPS works on first boot with an auto-generated self-signed certificate. For real domains:

- **Self-signed** — regenerate with custom CN and SANs from the SSL panel
- **Let's Encrypt** — set ports 80/443 and issue via HTTP-01 from the SSL panel
- **Nginx Proxy Manager** — deploy from the Stack Catalog and manage from Settings > Reverse Proxy

### Documentation

The Documentation page has two tabs:

- **Current Stacks** — auto-generated project guides for every discovered project, plus any README or docs files found in the stack directory
- **Stack Catalog** — searchable documentation for all 200+ catalog templates, with setup steps, env key references, caution notes, and upstream links

### Security & Audit

- **Image scanning** — Trivy vulnerability reports per project
- **Compose audit** — checks for privileged containers, host networking, missing healthchecks
- **Audit log** — every dashboard action logged with actor, project, IP, duration, and result
- **Per-project update policies** — auto-detect build-only repos and block accidental updates

### Project Detail

Each project has a dedicated page with tabs:

| Tab | What it shows |
|-----|--------------|
| Overview | Containers, state, update policy, image sources, compose hooks |
| Config | In-browser editor for compose.yml, .env, and other config files |
| Docs | Auto-generated project guide + any README/docs files |
| Sessions | Action history with full output logs and sticky-follow scroll |
| Sources | Image origin, registry, access status per service |
| Watch | Live startup log streaming (Up + Watch) with color-coded services |
| Logs | Docker compose logs with timestamps |
| Stats | CPU, memory, network I/O per container |
| Shell | Scoped compose commands + interactive xterm.js terminal |
| Security | Image scan + compose audit results |
| Backups | Create, restore, download, delete archives |
| Databases | Health checks and SQL dumps for supported engines |
| Inspect | Raw `docker inspect` JSON |
| Processes | `docker top` output |

The action bar also has per-project one-click buttons: **Update / Pull / Start / Restart / Stop**, **Backup**, **DB Dump**, **Open Ports (CSF)** (opens the project's published ports in the host firewall), and **Add to Proxy (NPM)** (creates an Nginx Proxy Manager host for the project). Backups warn before restore/delete, and stopping is styled as a destructive action.

---

## Architecture

```
Browser ──HTTPS──> nginx (web container, TLS termination)
                      │
                      ├── /api/* ──> Go server (port 8192, pid:host, nsenter for firewall)
                      │                 ├── MariaDB (users, jobs, settings, schedules, audit)
                      │                 ├── Redis (sessions, cache)
                      │                 ├── Docker socket (/var/run/docker.sock)
                      │                 └── WebSocket (/api/v1/projects/*/shell/exec)
                      │
                      └── /* ──> React SPA (hashed assets with immutable cache,
                                           no-cache index.html for instant deploys)
```

The same Go binary runs in two modes:

| Mode | What it does |
|------|-------------|
| **Controller** (`APP_MODE=server`) | Full stack: dashboard, API, MariaDB, Redis, agents, schedules |
| **Agent** (`APP_MODE=agent-callback/agent/agent-both`) | Lightweight: discovers local projects, reports to the controller |

---

## Installation

### Web Dashboard (recommended)

```bash
mkdir -p ~/docker && cd ~/docker
git clone https://github.com/arphost-com/Stack-Manager.git stack-manager
cd stack-manager
./scripts/prepare-state.sh .env
docker compose --env-file .env up -d --build
```

`prepare-state.sh` generates cryptographically random passwords for every secret, detects `DOCKER_GID` and `SERVER_USER` from the host, and prints the login URL + credentials. Re-running it never overwrites existing secrets.

### Updating

```bash
cd ~/docker/stack-manager
git pull
./scripts/prepare-state.sh .env
docker compose --env-file .env up -d --build
```

### CLI-Only (no dashboard)

```bash
chmod +x stack-manager.sh
./stack-manager.sh --root ~/docker list
./stack-manager.sh --root ~/docker update
```

The CLI is a single Bash script with no dependencies beyond Docker Compose. It supports all lifecycle commands, bulk operations, custom update hooks, inactive markers, timeouts, dry-run mode, and logging.

---

## Environment Reference

| Variable | Default | Purpose |
|----------|---------|---------|
| `API_KEY` | generated | API access key and bootstrap password fallback |
| `ADMIN_USERNAME` | `admin` | First admin username |
| `ADMIN_PASSWORD` | generated | First admin password (rotate after first login) |
| `DB_PASSWORD` | generated | MariaDB application password |
| `DB_ROOT_PASSWORD` | generated | MariaDB root password |
| `REDIS_PASSWORD` | generated | Redis password |
| `DOCKER_ROOT` | `~/docker` | Host directory containing managed Compose projects |
| `EXTRA_DOCKER_ROOTS` | empty | Comma-separated additional directories to scan for projects |
| `STATE_DIR` | `.stack-manager` | Persistent state (MariaDB, Redis, hooks, backups, SSL) |
| `SERVER_USER` | detected | UID:GID for the server container |
| `DOCKER_GID` | detected | Docker socket group ID |
| `WEB_SSL_PORT` | `8993` | HTTPS port (set to `443` for standard HTTPS) |
| `WEB_HTTP_PORT` | `8193` | HTTP redirect port (set to `80` for Let's Encrypt) |
| `HOST_URL` | detected | Dashboard URL shown in setup output |
| `CACHE_TTL_SECONDS` | `15` | Redis cache TTL for project state |
| `METRICS_REFRESH_MINUTES` | `15` | Background discovery and stats interval |
| `WARM_CACHE_TTL_MINUTES` | `30` | Redis TTL for background-warmed caches |

---

## Agent Installation

See [Agent Modes](docs/AGENT_MODES.md) for the full protocol reference, or use the guided setup in **Settings > Agents** which generates copy-paste commands for each mode.

```bash
git clone https://github.com/arphost-com/Stack-Manager.git stack-manager
cd stack-manager
# --mode is callback (outbound), inbound, or both. This writes the correct
# APP_MODE and auto-generates AGENT_TOKEN, AGENT_NAME, AGENT_PORT into .env,
# then prints the values to register in Settings > Agents.
./scripts/prepare-state.sh --agent --mode callback .env
# Edit .env: set DOCKER_ROOT and AGENT_CONTROLLER_URL to your controller's URL
docker compose --env-file .env -f docker-compose.agent.yml up -d --build
```

Then register the agent from the controller: **Settings > Agents > Add Agent** (name, mode, and the printed `AGENT_TOKEN`). For a **peer controller** instead, run a full install on both hosts and add each as a peer of the other (URL + API key) under Settings > Agents.

---

## CLI Reference

```bash
stack-manager.sh [options] <command> [projects...]

Commands:
  list      Show discovered projects
  status    Container status per project
  check     Check for image updates
  pull      Pull latest images
  up        Start all projects
  update    Pull + recreate (respects hooks)
  restart   Restart running projects
  down      Stop all projects
  prune     Remove unused Docker resources

Options:
  --root <dir>     Project root directory
  --running        Only act on running projects
  --dry-run        Show what would run without executing
  --timeout <sec>  Timeout for pull/check operations
  --prune          Run docker system prune after operations
  --log-dir <dir>  Custom log output directory
  --no-log         Disable automatic logging
```

---

## Development

```bash
# Run tests
cd server && go test ./...
bash -n stack-manager.sh

# Build
make build           # Go + web
make build-linux     # Cross-compile for linux/amd64
make docker-up       # Full stack with docker compose

# Single test
cd server && go test ./internal/core -run TestName
```

---

## Technical Reference

Everything below is the full technical detail for operators, contributors, and anyone scripting against the CLI or API.

### Project Layout

The CLI expects projects organized under a root directory:

```
/docker/                          # Root directory (configurable with --root)
├── .stack-manager/             # Configuration directory
│   └── hooks/                    # Custom update hooks
│       └── post-update_netbox-docker.sh
├── project-a/
│   └── compose.yml
├── project-b/
│   ├── compose.yml
│   └── .inactive                 # Marker file - project is skipped
├── netbox-docker/
│   └── docker-compose.yml
└── stack-manager_20240115_143022.log  # Auto-generated log file
```

### Compose File Detection

For each subdirectory, the script looks for compose files in this order:
1. `compose.yml`
2. `compose.yaml`
3. `docker-compose.yml`
4. `docker-compose.yaml`

The first match is used.

### Update Hook Override

If `<hooks-dir>/post-update_<project>.sh` exists, `update` runs only that hook and skips the normal `docker compose pull` + `up -d`. This exists so projects like NetBox that require a specific upgrade sequence do not get broken by generic pull/up.

### Update Policies

Per-project setting stored in MariaDB, cached in Redis:

| Mode | Behavior |
|------|----------|
| `auto` (default) | Build-only GitHub/GitLab projects (no registry image) are auto-treated as `no_updates`; others allow updates |
| `allow` | Always run update actions |
| `no_updates` | Skip update actions and record a skipped action session with the configured reason |

Scheduled updates and bulk actions respect this policy.

### Project Deletion Guardrails

Directory deletion (CLI and API) enforces:

1. Project must be marked inactive first.
2. Exact project-name confirmation required.
3. Target must be a discovered project.
4. Refuse to delete the configured `DOCKER_ROOT` or any path outside it.

By default deletion runs `docker compose down` before removing the project directory.

### Backup Endpoint Details

Project backups are always created locally under `BACKUP_DIR` first, then copied/uploaded to the selected endpoint. Endpoint types (`Linux path`, `Mounted path`, `CIFS mount`, `NFS mount`, `FTP`, `SFTP`, `S3`) are managed in Settings, stored in MariaDB, and secrets are never returned to the browser after save. FTP/SFTP/S3 use `rclone`; local/mounted just copy under `/backup-targets`.

If a non-root server user cannot read project data (e.g. Postgres/MariaDB/Redis bind mounts), backup falls back to a short-lived root helper container through the Docker socket.

### Background Cache And Metrics

Stack Manager warms project discovery, update-policy metadata, image-source metadata, and container stats in the background every `METRICS_REFRESH_MINUTES`. The projects:list cache TTL is capped at 30 minutes regardless of the metrics interval so a very large METRICS_REFRESH_MINUTES cannot freeze the dashboard STATE column. Dashboard reads use Redis-cached summaries so normal page loads do not wait for Docker inspection commands.

Metrics stored in MariaDB for historical graphing:

| Metric | Source |
|--------|--------|
| CPU and memory | `docker stats --no-stream` sampled in the background |
| Inbound/outbound traffic | Docker network RX/TX counters |
| Backup count/bytes | Backup skill create events |
| Restore count/bytes | Backup skill restore events |
| Upload bytes | Backup endpoint copy/upload events |

### Root-Sensitive Projects

GitLab, PMM, and similar stacks may run containers as root internally or require root-owned data. Stack Manager does not need to run as host root for the containers themselves; Docker handles container users. Stack Manager needs filesystem access to the project directory and compose files under `DOCKER_ROOT`.

Recommended model for mixed hosts:

1. Run Stack Manager as the host service user (`SERVER_USER=1000:1000`).
2. Keep compose files and `.env` readable by that UID/GID.
3. Store application data in Docker named volumes where possible.
4. For root-only project directories, run a separate root-capable Stack Manager agent with `SERVER_USER=0:0` and register it from the main server.

### Persistent State

| Path | Purpose |
|------|---------|
| `mariadb/` | MariaDB data for users, jobs, project settings, agents, schedules |
| `redis/` | Redis append-only data for sessions/cache |
| `hooks/` | Update hooks used by the API server |
| `backups/` | Project backups and database dumps |
| `backup-targets/` | Default host-backed mount for UI-configured backup destinations |
| `docker-config/` | Docker registry credentials from dashboard registry login |
| `ssl/` | TLS certificates (self-signed or Let's Encrypt) |

### Inactive Project Management

```bash
# Mark a project inactive (creates .inactive file)
touch /docker/project-name/.inactive

# Re-activate
rm /docker/project-name/.inactive

# List only active projects
stack-manager.sh --root /docker list
```

Inactive projects are excluded from `status`, `check`, `pull`, `update`, `up`, `restart`, `down`, and `prune` unless targeted by name.

### Custom Update Hooks

```bash
# Create a hook for a project
cat > ~/.stack-manager/hooks/post-update_netbox-docker.sh << 'EOF'
#!/bin/bash
cd "$PROJECT_DIR"
git pull
docker compose pull
docker compose up -d
EOF
chmod +x ~/.stack-manager/hooks/post-update_netbox-docker.sh
```

When `update` runs for `netbox-docker`, only this hook executes. Normal pull/up is skipped.

### Logging

All operations are logged to timestamped files:

```
/docker/stack-manager_20240115_143022.log
```

Override the log directory with `--log-dir` or disable with `--no-log`.

### Signal Handling

Ctrl-C during a batch run interrupts the current project and prints a summary of completed projects. No projects after the current one are started.

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | All operations completed |
| 1 | One or more operations failed |
| 2 | Invalid arguments or configuration |
| 130 | Interrupted by Ctrl-C |

### GitLab Pipeline

The GitLab pipeline treats docker02 as the dev environment:

- `deploy:docker02` runs automatically on the default branch after validation, tests, builds, and security scans pass.
- The deploy job preserves existing `.env` secrets or generates secure first-run values.
- `smoke:docker02` runs automatically after deploy.
- `smoke:stack-template` and `smoke:stack-templates:all` are manual test-server jobs. Set `STACK_TEMPLATE_TEST_HOST` to the test server IP. Use `--skip` to exclude specific templates.
- `push:github` is an optional manual job that mirrors tested `main` to GitHub.

---

## License

See [LICENSE](LICENSE) for details.
