# Stack Manager

Manage all your Docker Compose stacks from one dashboard. Discover, deploy, update, back up, and monitor projects across a fleet of hosts — with a 200-template catalog, scheduled updates, live shell access, firewall management, two-factor auth, and agents that phone home from behind NAT.

![Dashboard](docs/images/dashboard-dark.png)

---

## Highlights

- **200+ one-click stack templates** — AI, databases, CMS, monitoring, proxies, dev tools, media, and more. Pick a template, review the compose and env, and spin it up.
- **Fleet management with agents** — register remote Docker hosts as outbound (phone-home), inbound, or combined agents. Schedule actions across all of them from one controller.
- **Scheduled updates** — daily at 03:00, weekly on Saturday, monthly on the 15th, or every N minutes. Per-project update policies prevent accidental breakage.
- **Backup to anywhere** — local paths, CIFS, NFS, FTP, SFTP (with in-browser SSH key generation), and S3. Automatic local archive + remote copy.
- **Live in-browser shell** — xterm.js terminal that opens a real interactive session inside any running container via WebSocket + docker exec.
- **Firewall management** — install, configure, and monitor ConfigServer Firewall (csf/lfd) from the dashboard. Login IPs are auto-allowlisted.
- **Two-factor authentication** — TOTP (Google Authenticator / Authy) with QR enrollment, backup codes, and per-user enable/disable.
- **Self-signed TLS out of the box** — HTTPS on first boot with zero config. Optional Let's Encrypt or Nginx Proxy Manager for real domains.
- **Docker daemon settings** — edit `daemon.json` from the browser with tooltips, backups, and teardown guidance for network changes.
- **Security scans** — image vulnerability scanning and compose audit from the project detail page.
- **Database checks** — health checks and SQL dumps for MariaDB, MySQL, and PostgreSQL containers.
- **Audit log** — every action recorded with actor, project, result, and timestamp.

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

> Add your own screenshots below. Suggested shots:

![Stack Catalog](docs/images/catalog-personal-ai-agents-dark.png)

![Project Detail](docs/images/project-detail-placeholder.png)

![Settings](docs/images/settings-placeholder.png)

---

## Features in Detail

### Dashboard

The main page shows every discovered Compose project with live state, image sources, update availability, and one-click actions: start, stop, restart, update, pull, backup, and delete. Filter by running, stopped, inactive, or projects with available updates. Bulk actions apply to the filtered list or a manual selection.

![Dashboard filters](docs/images/dashboard-filters-placeholder.png)

### Stack Catalog (200+ Templates)

Browse and deploy from a curated catalog organized into 14 categories and 10 AI subcategories:

| Category | Examples |
|----------|----------|
| **AI** | Ollama + Open WebUI, LibreChat, AnythingLLM, ComfyUI, Whisper, Langfuse, promptfoo |
| **Web & Proxy** | Nginx, Caddy, Traefik, Nginx Proxy Manager, Apache |
| **CMS** | WordPress, Ghost, Strapi, Directus, Payload, Concrete CMS |
| **Database** | PostgreSQL, MariaDB, Redis, MongoDB, CockroachDB, ClickHouse |
| **Dev Tools** | Gitea, Forgejo, code-server, Jenkins, Drone CI, Draw.io |
| **Monitoring** | Grafana + Prometheus, Uptime Kuma, Beszel, Netdata, cAdvisor |
| **Docs** | BookStack, Docmost, DokuWiki, Wiki.js, Outline |
| **Media** | Jellyfin, Plex, Audiobookshelf, Calibre-Web |
| **Security** | Authelia, Keycloak, WireGuard, CrowdSec, Vaultwarden |
| **Files** | Nextcloud, Seafile, MinIO, Paperless-ngx |
| **Queue** | RabbitMQ, Apache ActiveMQ, Beanstalkd, Faktory |
| **Automation** | n8n, Huginn, Cronicle, Changedetection.io |

Templates load into an editable Create Project form — review ports, volumes, passwords, and env vars before deploying. Nothing deploys until you click Create.

### Scheduled Updates

Set human-readable schedules from Settings or the Dashboard:

- **Daily** at a specific time (UTC)
- **Weekly** on a chosen day at a specific time
- **Monthly** on a day-of-month at a specific time
- **Every N minutes** for custom intervals

Each schedule targets a project (local or on a registered agent) and runs update, pull, up, restart, down, or status. Projects with a `no_updates` policy record a skipped session instead of pulling.

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

### Fleet Agents

Manage Docker hosts across your network from one controller. Three modes cover every network topology:

| Mode | How it works | Needs inbound port? |
|------|-------------|---------------------|
| **Outbound check-in** | Agent phones home to the controller. Best behind NAT. | No |
| **Inbound listener** | Controller reaches out to the agent. | Yes |
| **Both** | Combined — works whether the host is reachable or not. | Yes |

The Settings > Agents page shows setup commands with Copy buttons, a controller URL field, and a mode selector that updates the sample commands live. Agents need no database, Redis, or web UI.

### Interactive Shell

Open a live terminal inside any running container from the Shell tab. Powered by xterm.js + WebSocket + `docker exec`. Pick a container from the dropdown, click Connect, and type commands. Sessions are auth-gated and scoped to the project's containers.

### Firewall (CSF)

Settings > Firewall manages ConfigServer Firewall on the host:

- **Install / uninstall** csf from the browser
- **Status dashboard** — installed, testing mode, LFD active, iptables rule count
- **Allow / deny lists** — add, remove, view entries (IPv4 and IPv6)
- **Config editor** — edit `csf.conf`, `csf.allow`, `csf.deny`, `csf.ignore`, `csf.pignore` with timestamped backups
- **Log viewer** — tail `/var/log/lfd.log` with configurable line count
- **Auto-allowlist on login** — every successful dashboard login runs `csf -a` for the caller's IP

One-time host setup: `sudo install -m 750 scripts/stack-manager-csf.sh /usr/local/sbin/stack-manager-csf`

### Two-Factor Authentication

Protect accounts with TOTP (Google Authenticator, Authy, or any compatible app):

1. Go to **Settings > Account > Set up 2FA**
2. Scan the QR code with your authenticator app
3. Save the 8 backup codes somewhere safe
4. Enter a 6-digit code to verify and enable

Once enabled, login requires a code after the password step. Admins can reset another user's 2FA from the Users tab.

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
- **Nginx Proxy Manager** — deploy from the Stack Catalog and proxy individual projects with per-domain LE certs (see Settings > Reverse Proxy for setup steps)

### Reverse Proxy

Settings > Reverse Proxy explains how to set up Nginx Proxy Manager alongside Stack Manager for domain-based HTTPS. Only needed when you have real domain names — private networks and IP-only hosts should use the built-in self-signed cert.

### Documentation

The Documentation page has two tabs:

- **Current Stacks** — auto-generated project guides for every discovered project, plus any README or docs files found in the stack directory
- **Stack Catalog** — searchable documentation for all 200 catalog templates, with setup steps, env key references, caution notes, and upstream links

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
| Docs | Auto-generated project guide + any README/docs files |
| Sessions | Action history with full output logs |
| Sources | Image origin, registry, access status per service |
| Watch | Live startup log streaming (Up + Watch) with color-coded services |
| Logs | Docker compose logs with timestamps |
| Stats | CPU, memory, network I/O per container |
| Shell | Scoped compose commands + interactive terminal |
| Security | Image scan + compose audit results |
| Backups | Create, restore, download, delete archives |
| Databases | Health checks and SQL dumps for supported engines |
| Inspect | Raw `docker inspect` JSON |
| Processes | `docker top` output |

---

## Architecture

```
Browser ──HTTPS──> nginx (web container, TLS termination)
                      │
                      ├── /api/* ──> Go server (port 8192)
                      │                 ├── MariaDB (users, jobs, settings, schedules, audit)
                      │                 ├── Redis (sessions, cache)
                      │                 └── Docker socket (/var/run/docker.sock)
                      │
                      └── /* ──> React SPA (hashed assets, no-cache index.html)
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
| `STATE_DIR` | `.stack-manager` | Persistent state (MariaDB, Redis, hooks, backups, SSL) |
| `SERVER_USER` | detected | UID:GID for the server container |
| `DOCKER_GID` | detected | Docker socket group ID |
| `WEB_SSL_PORT` | `8993` | HTTPS port (set to `443` for standard HTTPS) |
| `WEB_HTTP_PORT` | `8193` | HTTP redirect port (set to `80` for Let's Encrypt) |
| `HOST_URL` | detected | Dashboard URL shown in setup output |
| `CACHE_TTL_SECONDS` | `15` | Redis cache TTL for project state |
| `METRICS_REFRESH_MINUTES` | `15` | Background discovery and stats interval |

---

## Agent Installation

See [Agent Modes](docs/AGENT_MODES.md) for the full protocol reference, or use the guided setup in **Settings > Agents** which generates copy-paste commands for each mode.

```bash
git clone https://github.com/arphost-com/Stack-Manager.git stack-manager
cd stack-manager
./scripts/prepare-state.sh --agent .env
# Edit .env: set DOCKER_ROOT and AGENT_CONTROLLER_URL
docker compose --env-file .env -f docker-compose.agent.yml up -d --build
```

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

Stack Manager warms project discovery, update-policy metadata, image-source metadata, and container stats in the background every `METRICS_REFRESH_MINUTES`. Dashboard reads use Redis-cached summaries so normal page loads do not wait for Docker inspection commands.

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
