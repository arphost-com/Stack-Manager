# Stack Manager

A **controller + agent** for running many Docker Compose stacks across a fleet of hosts. Ships as a single Go binary paired with a React web dashboard and a Bash CLI (`stack-manager.sh`), all in one repo.

Built for environments with **many stacks**, mixed lifecycle states, and special projects that need **custom update logic** (such as NetBox). Includes bulk actions, scheduled updates, backups to local paths / CIFS / NFS / FTP / SFTP / S3, per-project update policies, registry logins, DB checks, security scans, scoped per-project shell, and TLS out of the box.

Ships with a one-click **stack template catalog** spanning:

- **AI** — LLM runtimes, RAG pipelines, vector databases, code assistants, image generation, voice / speech
- **Web & CMS** — WordPress, static hosting, reverse proxies
- **Databases & queues** — Postgres, MariaDB, Redis, message brokers
- **DevOps** — monitoring, security, docs, automation, file sharing, media

Load a template, edit the `compose.yml` / `.env`, and deploy — or point the dashboard at a fleet of hosts through the built-in agent mode.

---

## Table of Contents

- [Key Design Rules](#key-design-rules)
- [Features](#features)
- [Requirements](#requirements)
- [Installation](#installation)
- [Web Dashboard](#web-dashboard)
- [Stack Catalog](#stack-catalog)
- [Project Layout](#project-layout)
- [Usage](#usage)
  - [Basic Commands](#basic-commands)
  - [Global Options](#global-options)
  - [Inactive Project Management](#inactive-project-management)
- [Custom Update Hooks](#custom-update-hooks)
- [Logging](#logging)
- [How It Works](#how-it-works)
  - [Project Discovery](#project-discovery)
  - [Project Name Resolution](#project-name-resolution)
  - [Command Execution Flow](#command-execution-flow)
  - [Signal Handling](#signal-handling)
- [Development Guide](#development-guide)
  - [Code Architecture](#code-architecture)
  - [Key Functions Reference](#key-functions-reference)
  - [Adding New Commands](#adding-new-commands)
  - [Testing Changes](#testing-changes)
- [Troubleshooting](#troubleshooting)
- [Exit Codes](#exit-codes)

---

## Key Design Rules

### Start / Stop / Status

Always use standard Docker Compose commands:
- `up` runs `docker compose up -d`
- `down` runs `docker compose down`
- `status` runs `docker compose ps`

### Update Behavior

The CLI `update` command has special override logic:

- If a project has a custom hook named `post-update_<project>.sh`, **only the hook is run** for updates
- Normal `docker compose pull` / `up -d` is **skipped** for that project
- Projects **without** a hook use normal update behavior (pull + up)

This makes it safe to manage projects like NetBox that can break if updated incorrectly.

The web/API surface also supports per-project update policies:

- `auto` - default. Build-only projects from GitHub/GitLab with no registry image are automatically marked `no_updates`.
- `allow` - always permit update actions.
- `no_updates` - skip update actions and save a skipped action session with the reason.

---

## Features

- **Auto-discovery** of Compose projects under a configurable root directory
- **Multiple compose file formats** supported:
  - `compose.yml`
  - `compose.yaml`
  - `docker-compose.yml`
  - `docker-compose.yaml`
- **Bulk operations**: `list`, `status`, `check`, `pull`, `update`, `up`, `restart`, `down`, `prune`
- **Per-project update hooks** for custom update logic
- **Inactive marker** (`.inactive` file) to exclude projects from operations
- **Safe failure handling** - one project failing does not stop the entire run
- **Optional timeouts** for pull/check operations to avoid hangs
- **Running-only filtering** - act only on projects with running containers
- **Dry-run mode** for safe simulation
- **Automatic logging** to timestamped files
- **Ctrl-C handling** - graceful interruption with summary output
- **Web dashboard** for project creation, one-click stack templates, management, updates, statistics, logging, backups, database checks, image-source classification, registry login, schedules, agents, and user management
- **Live action sessions** for update/pull/start/stop/restart with saved logs
- **Scheduled updates** for local projects and registered agents
- **MariaDB-backed state** for users, action history, project settings, agents, and schedules
- **Redis-backed sessions and cache** for project/image/job/settings/agent/schedule reads

---

## Requirements

| Dependency | Required | Notes |
|------------|----------|-------|
| Docker | Yes | Docker Engine must be running |
| Docker Compose v2 | Yes | Uses `docker compose` (not `docker-compose`) |
| timeout | Optional | From coreutils; enables `--timeout` for pull/check |
| Bash 4.0+ | Yes | Uses associative arrays and modern features |
| MariaDB | Web/API | Provided by `docker-compose.yml` |
| Redis | Web/API | Provided by `docker-compose.yml` |

Verify your Docker Compose version:
```bash
docker compose version
# Should show: Docker Compose version v2.x.x
```

---

## Installation

### Quick Install: Web Dashboard

```bash
# Clone the dashboard/API project
mkdir -p ~/docker
cd ~/docker
git clone https://github.com/arphost-com/Stack-Manager.git
cd Stack-Manager

# Generate .env, passwords, state directories, and printed login settings
./scripts/prepare-state.sh .env

# Start the web dashboard
docker compose --env-file .env up -d --build
```

Open the `HOST_URL` printed by `prepare-state.sh`, usually:

```bash
https://<server>:8993
```

For an existing install:

```bash
cd ~/docker/Stack-Manager
git pull
./scripts/prepare-state.sh .env
docker compose --env-file .env up -d --build
```

### CLI-Only Install

Use this only if you want the standalone Bash script without the web dashboard, MariaDB, Redis, agents, stack catalog, users, schedules, backups UI, or metrics history.

```bash
chmod +x stack-manager.sh
sudo install -m 0755 stack-manager.sh /usr/local/bin/stack-manager.sh
stack-manager.sh --help
```

---

## Web Dashboard

The dashboard runs as a Docker Compose stack with four services:

| Service | Port | Purpose |
|---------|------|---------|
| `server` | `8192` internal | Go API server |
| `web` | `${WEB_HTTP_PORT:-8193}:8080` and `${WEB_SSL_PORT:-8993}:8443` | React dashboard through nginx with HTTP redirect/ACME and HTTPS |
| `mariadb` | internal | Users, action history, project settings |
| `redis` | internal | Login sessions and cached project/image/job/settings data |

Example `.env`:

```bash
API_KEY=change-me-to-a-secure-key
ADMIN_USERNAME=admin
# ADMIN_PASSWORD=change-me-to-a-different-secure-password

DB_NAME=stack_manager
DB_USER=stack_manager
DB_PASSWORD=change-me-to-a-secure-database-password
DB_ROOT_PASSWORD=change-me-to-a-secure-root-database-password
REDIS_PASSWORD=change-me-to-a-secure-redis-password
REDIS_DB=0
CACHE_TTL_SECONDS=15
METRICS_REFRESH_MINUTES=15
WARM_CACHE_TTL_MINUTES=30

DOCKER_ROOT=/home/debian/docker
STATE_DIR=.stack-manager
BACKUP_TARGET_ROOT=.stack-manager/backup-targets
DOCKER_GID=998
SERVER_USER=1000:1000
WEB_HTTP_PORT=8193
WEB_SSL_PORT=8993
HOST_URL=https://docker01:8993
```

Environment reference:

| Variable | Default | Purpose |
|----------|---------|---------|
| `API_KEY` | none | Required legacy/API key used for API access and as the first-admin password fallback when `ADMIN_PASSWORD` is unset. Use a long random value. |
| `ADMIN_USERNAME` | `admin` | Username created only when the MariaDB users table is empty. |
| `ADMIN_PASSWORD` | empty | Optional first-admin password. If empty, the app uses `API_KEY` for initial bootstrap login. Rotate it after first login. |
| `DB_NAME` | `stack_manager` | MariaDB database name for users, action history, project settings, update policies, agents, and schedules. |
| `DB_USER` | `stack_manager` | MariaDB application user. |
| `DB_PASSWORD` | none | Required password for the MariaDB application user. |
| `DB_ROOT_PASSWORD` | none | Required MariaDB root password used by the MariaDB container during initialization. |
| `REDIS_PASSWORD` | none | Required password for Redis sessions and cache. |
| `REDIS_DB` | `0` | Redis logical database number. `0` means the first/default Redis DB. Keep `0` for the bundled dedicated Redis container; use another index only if intentionally sharing a Redis server with other apps. |
| `CACHE_TTL_SECONDS` | `15` | Time, in seconds, that project/image/job/settings reads may stay in Redis before refresh. Lower values show changes faster; higher values reduce Docker/API load. |
| `METRICS_REFRESH_MINUTES` | `15` | Background interval for project cache warmup, Docker stats snapshots, and metrics history collection. Values below 15 are raised to 15. |
| `WARM_CACHE_TTL_MINUTES` | `30` | Redis TTL for background-warmed project and image-source caches. Keep it at least twice the refresh interval for fastest dashboard loads. |
| `DOCKER_ROOT` | none | Required host directory containing the Docker Compose projects that Stack Manager should discover and manage. This can be any path on the host and is mounted into the server container at `/docker`. |
| `DOCKER_DAEMON_DIR` | `/etc/docker` | Host Docker daemon configuration directory used by Settings > Docker Settings when reading or writing `daemon.json`. |
| `STATE_DIR` | `.stack-manager` | Stack Manager persistent state directory. Relative paths are stored under the Stack Manager root beside `docker-compose.yml`. |
| `BACKUP_TARGET_ROOT` | `.stack-manager/backup-targets` | Host directory mounted into the server container at `/backup-targets` for UI-configured local, CIFS, NFS, and Linux mount backup endpoints. |
| `DOCKER_GID` | host Docker socket group | Group ID for `/var/run/docker.sock`. The non-root server user is added to this group so it can run Docker commands. |
| `SERVER_USER` | none | Required numeric UID:GID used to run the server container and own `STATE_DIR`, for example `1000:1000`. Use whichever host user/group should manage Docker. Set `0:0` only on hosts that intentionally require root compose management. |
| `WEB_HTTP_PORT` | `8193` | Host HTTP port for redirects and Let's Encrypt HTTP-01 challenges. Set to `80` for Let's Encrypt. |
| `WEB_SSL_PORT` | `8993` | Host HTTPS port for the dashboard. Set to `443` for Let's Encrypt. The API server stays internal on port `8192`. |
| `HOST_URL` | detected from hostname and `WEB_SSL_PORT` | Dashboard URL printed by setup for the operator. Edit it if users should open a different DNS name or reverse-proxy URL. |

`REDIS_DB` is not a MariaDB setting and does not create another Redis container. Redis has numbered logical databases inside one Redis instance; `0` is the normal default. Stack Manager stores login sessions and short-lived cache keys there. With the included dedicated Redis service, leave it at `0`.

`SERVER_USER` is host-specific. Different Linux boxes may need different numeric IDs, such as `0:0`, `1000:1000`, or `998:998`. Stack Manager uses that same UID:GID for the server process and for prepared state directory ownership so behavior stays consistent across hosts.

Start it:

```bash
./scripts/prepare-state.sh .env
docker compose --env-file .env up -d --build
```

If `.env` does not exist, `prepare-state.sh` creates it from `.env.example`. It fills missing or `change-me...` values with cryptographically random 36-character, shell-safe values for `API_KEY`, `ADMIN_PASSWORD`, `DB_PASSWORD`, `DB_ROOT_PASSWORD`, and `REDIS_PASSWORD`; generated values include punctuation while avoiding characters that break `.env`, MariaDB DSNs, or Compose parsing. It sets practical defaults for the other settings; writes everything back to `.env`; and prints the resulting settings, including `HOST_URL`. Edit host-specific values such as `DOCKER_ROOT`, `BACKUP_TARGET_ROOT`, `SERVER_USER`, `DOCKER_GID`, `WEB_HTTP_PORT`, `WEB_SSL_PORT`, and `HOST_URL` as needed for that box.

Open `https://<host>:8993` by default, or the `HOST_URL` printed by setup. If the MariaDB users table is empty, the first admin is created from `ADMIN_USERNAME` and `ADMIN_PASSWORD`. If `ADMIN_PASSWORD` is unset, the bootstrap password is `API_KEY`; rotate or add users from Settings after first login.

The preparation step is recommended on manual installs such as docker01. The Compose stack also runs a short `state-init` service that repairs app-writable state paths before the server starts. If `STATE_DIR` was already created as root, stop the stack and repair app state ownership once:

```bash
docker compose --env-file .env down
. ./.env
sudo chown -R "$(id -u):$(id -g)" "${STATE_DIR}"
./scripts/prepare-state.sh .env
docker compose --env-file .env up -d --build
```

Do not recursively chown `STATE_DIR/mariadb` or `STATE_DIR/redis` to the app user after those services have initialized; MariaDB and Redis need their own service-owned data files.

After `git pull`, use `docker compose --env-file .env up -d --build` instead of plain `docker compose up -d` when Dockerfiles or nginx/server config changed. Plain `up -d` can reuse old images and keep old nginx behavior.

Persistent state is stored under `STATE_DIR`:

| Path | Purpose |
|------|---------|
| `mariadb/` | MariaDB data for users, jobs, project settings, agents, schedules |
| `redis/` | Redis append-only data for sessions/cache |
| `hooks/` | Update hooks used by the API server |
| `backups/` | Project backups and database dumps |
| `backup-targets/` | Default host-backed mount for UI-configured local/CIFS/NFS backup destinations |
| `docker-config/` | Docker registry credentials from dashboard registry login |

Persistent state stays under the Stack Manager root. Managed Compose projects stay under `DOCKER_ROOT`, which can be any host directory chosen for that machine.

Legacy files from earlier versions are imported on startup if present:

- `/state/users.json`
- `/state/jobs/*.json`

The app no longer writes users, sessions, project settings, or action history as flat files.

### Background Cache And Metrics

Stack Manager warms project discovery, update-policy metadata, image-source metadata, and container stats in the background when the server starts and then every `METRICS_REFRESH_MINUTES`. Dashboard reads use Redis-cached summaries first, so normal page loads do not need to wait for every Docker inspection command.

Metrics are stored in MariaDB for historical graphing:

| Metric | Source |
|--------|--------|
| CPU and memory | `docker stats --no-stream` sampled in the background |
| Inbound/outbound traffic | Docker network RX/TX counters from `docker stats` |
| Backup count/bytes | Backup skill create events |
| Restore count/bytes | Backup skill restore events |
| Upload bytes | Backup endpoint copy/upload events |

The Project Detail Shell tab runs scoped Docker Compose troubleshooting commands from the selected project directory. It is not a raw host shell. Use `Recreate network + start` when Docker reports stale network errors such as `failed to set up container networking: network ... not found`.

### Root-Sensitive Projects

GitLab, PMM, and similar stacks may run containers as root internally or require root-owned data inside Docker volumes. Stack Manager does not need to run as host root for the containers themselves; Docker handles container users. Stack Manager does need filesystem access to the project directory and compose files under `DOCKER_ROOT`.

Recommended model for mixed hosts:

1. Run Stack Manager as the host service user, such as `SERVER_USER=1000:1000`.
2. Keep compose files and `.env` readable by that UID/GID, using group ownership or ACLs if needed.
3. Store application data in Docker named volumes where possible, not root-only bind paths.
4. For truly root-only project directories, run a separate root-capable Stack Manager agent with `SERVER_USER=0:0` and register it from the main server.

### Backup Endpoints

Backup endpoint definitions are managed in Settings, stored in MariaDB, and cached in Redis. Secrets are saved for use by the server but are not returned to the browser after save.

Endpoint types:

| Type | How it works |
|------|--------------|
| `Linux path` | Copies the local backup archive to an absolute path inside the server container. Use `/backup-targets/...` for the default host-backed path. |
| `Mounted path` | Same as Linux path, intended for any host mount exposed to the server container. |
| `CIFS mount` | Same as Linux path. Mount CIFS on the host first, then expose it through `BACKUP_TARGET_ROOT` or a compose override. |
| `NFS mount` | Same as Linux path. Mount NFS on the host first, then expose it through `BACKUP_TARGET_ROOT` or a compose override. |
| `FTP` | Uploads with `rclone` using host, port, username, password, and remote path fields from the UI. |
| `SFTP` | Uploads with `rclone` using password auth or an optional private-key file path available inside the server container. |
| `S3` | Uploads with `rclone` using bucket, prefix, region, endpoint, provider, and access/secret keys. |

Project backups are always created locally under `BACKUP_DIR` first. Choosing a destination in the Project Detail backup controls copies/uploads after the local archive is created. Local archives can be downloaded from the Backups tab. Remote-only FTP, SFTP, and S3 copies are not downloadable through the web UI unless they are also present on a mounted path the server can read.

If a non-root server user cannot read project data directories such as PostgreSQL, MariaDB, or Redis bind mounts, backup creation falls back to a short-lived root helper container through the Docker socket. This lets backups include mixed-UID project files without changing ownership on the host.

For a simple local or mounted destination, create the endpoint in Settings with:

```text
Type: Linux path or Mounted path
Path: /backup-targets
```

### Dashboard Update Policies

The dashboard can mark a project as not updateable when it is built directly from a Dockerfile in a GitHub/GitLab checkout and has no registry image to pull. Auto detection checks the project Git remote and parsed Compose image/build metadata.

| Mode | Behavior |
|------|----------|
| `auto` | Detect build-only GitHub/GitLab projects and mark them `no_updates`; otherwise allow updates |
| `allow` | Always run update actions |
| `no_updates` | Skip update actions and save a skipped action session with the configured reason |

Use the Project Detail overview page to view or override the policy.

### Stack Catalog

The dashboard includes a built-in stack catalog modeled after Docker GUI template flows such as Kitematic image browsing and Portainer app templates. Choosing a template does not deploy immediately; it loads editable `compose.yml` and `.env` content into the Create Project form so the operator can review paths, ports, passwords, and volumes first.

Catalog counts are intentionally balanced so each main category and each AI subcategory has a useful scan view. Templates are normal Compose projects after creation and can be edited later on disk like any other project.

![Personal AI agents catalog](docs/images/catalog-personal-ai-agents-dark.png)
![Web catalog](docs/images/catalog-web-dark.png)
![CMS catalog](docs/images/catalog-cms-dark.png)
![Database catalog](docs/images/catalog-database-light.png)
![Queue catalog](docs/images/catalog-queue-light.png)
![Dev Tools catalog](docs/images/catalog-devtools-light.png)

Main categories include:

| Category | What it covers |
| --- | --- |
| AI | LLM inference, code assistants, personal agents, image generation, voice/speech, vector DBs, workflow/RAG, observability, evals, and search/crawling |
| Web | Static web servers and lightweight HTTP test services |
| Proxy | Reverse proxies, TLS front ends, and identity-aware proxies |
| CMS | CMS, blog, headless CMS, and e-commerce stacks |
| Database | SQL, NoSQL, cache, graph, and database-admin stacks |
| Dev Tools | Git forges, CI/CD, browser IDEs, code quality, and diagram tools |
| Docs | Wikis, documentation, and knowledge-base tools |
| Files | File sharing, sync, object storage, and paperless document tools |
| Management | Docker and infrastructure management dashboards |
| Media | Media servers, libraries, and download automation |
| Monitoring | Metrics, uptime, dashboards, logs, and observability |
| Queue | Message queues, brokers, and workflow engines |
| Security | Auth, SSO, VPN, and security scanner starters |
| Automation | Task automation, workflow, and scheduled job tools |

AI subcategories include:

| Subcategory | What it covers |
| --- | --- |
| LLM inference | Ollama, Open WebUI, vLLM, Text Generation Inference, LiteLLM, LocalAI, and similar runtime/API stacks |
| Code assistants | Tabby, OpenHands, Continue, Aider, code-server + Ollama, and code model servers |
| Personal AI agents | OpenClaw-style chat gateways plus vetted personal agents such as Khoj |
| Image generation | ComfyUI, InvokeAI, Stable Diffusion Web UI, SD.Next, and LoRA/training tools |
| Voice / speech | Whisper, Piper, Coqui, MaryTTS, and speech service starters |
| Vector DB | Qdrant, Weaviate, Chroma, Milvus, LanceDB, pgvector, and related stores |
| Workflow / RAG | AnythingLLM, Flowise, Langflow, LibreChat, Onyx, DocsGPT, OpenMemory/Mem0, and other orchestration/document tools |
| Observability | Langfuse and Arize Phoenix for AI traces, prompts, experiments, datasets, and evals |
| Evals / testing | promptfoo for prompt, model, RAG, agent, and red-team regression tests |
| Search | Meilisearch, Typesense, OpenSearch, Firecrawl, Crawl4AI, and other search/crawling tools |

Recent AI additions:

| Template ID | Project | Best fit |
| --- | --- | --- |
| `librechat` | LibreChat | Multi-user AI chat, agents, MCP, files, and multi-provider routing |
| `onyx` | Onyx | Enterprise knowledge search/RAG with connectors |
| `khoj` | Khoj | Personal/team second brain and personal-agent workflows |
| `docsgpt` | DocsGPT | Private document assistants and enterprise search |
| `openmemory-mem0` | OpenMemory + Mem0 | Shared memory layer for agents and MCP clients |
| `langfuse` | Langfuse | LLM observability, prompt management, datasets, and evals |
| `phoenix` | Arize Phoenix | Lightweight AI tracing, experiments, and evaluations |
| `promptfoo` | promptfoo | Prompt/RAG/agent tests and red-team regression workflows |
| `firecrawl` | Firecrawl | Search, scrape, crawl, extract, and agent web context API |
| `crawl4ai` | Crawl4AI | Local crawler/scraper API for Markdown and RAG extraction |

See [AI Stack Catalog Notes](docs/AI_STACKS.md) for project-specific setup instructions, upstream links, and GPU/resource guidance.

### Docker Settings

Settings > Docker Settings reads and writes the Docker host's `daemon.json` using `DOCKER_DAEMON_DIR`, defaulting to `/etc/docker`. The tab exposes common daemon settings with tooltips:

- Log driver and log rotation defaults.
- Live restore.
- DNS servers.
- Default bridge network address pools.
- Registry mirrors and insecure registries.
- IPv6 defaults.
- Optional Docker TCP API hosts.
- Advanced raw JSON for daemon options not shown as form fields.

Saving creates a timestamped backup beside `daemon.json` when the file already exists. Docker must still be restarted on the host for changes to apply. Remote Docker TCP access is root-equivalent; prefer SSH, TLS on port `2376`, VPN-only binding, and firewall allowlists. For Docker Hub rate limits, login with `docker login` as the service user or configure trusted registry mirrors before rebuilding images.

### Scheduled Updates And Agents

Schedules are stored in MariaDB and cached in Redis. A schedule can target:

- `Local` - a project on the main Stack Manager host.
- A registered agent - a remote Stack Manager agent running on another Docker host.

Schedules support `update`, `pull`, `up`, `restart`, `down`, and `status`. Scheduled `update` respects the project update policy; projects marked `no_updates` record a skipped session instead of pulling.

Remote agents can run without opening any inbound port. Use callback mode when the remote Docker host can reach the controller over a VPN or outbound internet path, but the controller cannot call back into the remote host:

```bash
git clone https://github.com/arphost-com/Stack-Manager.git
cd Stack-Manager
./scripts/prepare-state.sh --agent .env
sed -i 's#^AGENT_CONTROLLER_URL=.*#AGENT_CONTROLLER_URL=https://docker02:8993#' .env
docker compose --env-file .env -f docker-compose.agent.yml up -d --build
```

Callback agent mode does not require MariaDB, Redis, the web UI, or a client-side HTTP listener on the remote host. The setup script writes `APP_MODE=agent-callback`, `AGENT_NAME`, `AGENT_TOKEN`, `AGENT_CONTROLLER_URL`, `AGENT_CHECKIN_SECONDS`, and `DOCKER_ROOT` into `.env`. Register the same `AGENT_NAME` and `AGENT_TOKEN` from Settings > Agents on the main server with mode `Outbound check-in`; leave Agent URL blank. The agent discovers local compose projects under `DOCKER_ROOT` and posts the inventory to `/api/v1/agent-checkin/projects` on the controller.

The older inbound HTTP agent is still supported for directly reachable hosts. Set `APP_MODE=agent`, keep `HOST_URL`/`AGENT_PORT`, and register `HOST_URL` as an `Inbound URL` agent when the controller can call `/agent/v1` on that host for project lists, jobs, logs, stats, registry login, and prune operations.

### Project Deletion

Directory deletion is intentionally gated:

1. Mark the project inactive first.
2. Click Delete.
3. Type the exact project name when prompted.

The API enforces the same rule and refuses to delete the configured Docker root or any path outside the configured `DOCKER_ROOT`. By default deletion runs `docker compose down` before removing the whole project directory.

### GitLab Pipeline

The GitLab pipeline treats docker02 as the dev environment:

- `deploy:docker02` runs automatically on the default branch after validation, tests, builds, and security scans pass.
- The deploy job preserves existing docker02 `.env` secrets or generates secure first-run values when GitLab CI variables are not set.
- `smoke:docker02` runs automatically after the dev deploy.
- `smoke:stack-template` is a manual test-server job. Set `STACK_TEMPLATE_TEST_HOST` to the test server IP, and set `STACK_TEMPLATE_ID` to one template ID, such as `openclaw`, to render its base `.env`, fill temporary test secrets, run `docker compose config`, start it, verify containers are still running, then tear it down.
- `smoke:stack-templates:all` is the manual sequential all-template version. It starts one rendered stack on the test server, verifies the containers, shuts it down with volumes removed, then moves to the next. Narrow it with `STACK_TEMPLATE_CATEGORY` or `STACK_TEMPLATE_SUBCATEGORY` when you want a smaller batch, such as `ai` / `personal-agents`.
- Template smoke jobs SSH to `root@STACK_TEMPLATE_TEST_HOST` by default. The test server must have Docker Compose installed and this public key in `/root/.ssh/authorized_keys`: `ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIHlfck6vWHItQJ9FjjI6hNCv/3OpwSGcqRyFlUirDmcx barry@arphost`. If the runner does not already have the matching private key, provide it as a masked `STACK_TEMPLATE_TEST_PRIVATE_KEY` CI variable.
- Set `STACK_TEMPLATE_SMOKE_MODE=config` on either manual job to validate Compose rendering only without starting containers.
- `push:github` is an optional manual production-style job that pushes the tested default branch to `arphost-com/Stack-Manager` with the masked `GITHUB_PAT` CI variable.
- If `push:github` is clicked before `GITHUB_PAT` is configured, the job fails with a clear message and does not push.
- After pushing, `push:github` verifies that GitHub `main` matches the GitLab commit SHA.

---

## Project Layout

### Directory Structure

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

---

## Usage

### Basic Commands

```bash
# List all discovered projects and their status
stack-manager.sh --root /docker list

# Show detailed container status per project
stack-manager.sh --root /docker status

# Check for image updates (pulls but doesn't restart)
stack-manager.sh --root /docker check

# Pull latest images for all projects
stack-manager.sh --root /docker pull

# Start all projects
stack-manager.sh --root /docker up

# Update all projects (uses hooks if present)
stack-manager.sh --root /docker update

# Restart all running projects
stack-manager.sh --root /docker restart

# Stop all projects
stack-manager.sh --root /docker down

# Prune unused Docker resources
stack-manager.sh --root /docker prune
```

### Operating on Specific Projects

```bash
# Update only specific projects
stack-manager.sh --root /docker update project-a project-b

# Restart a single project
stack-manager.sh --root /docker restart netbox-docker
```

### Global Options

| Option | Short | Description |
|--------|-------|-------------|
| `--root <path>` | `-r` | Root folder containing projects (default: `/docker`) |
| `--exclude <name>` | `-x` | Exclude project by folder name (repeatable) |
| `--only <name>` | `-o` | Only include specific project (repeatable) |
| `--include-inactive` | | Include projects marked with `.inactive` |
| `--only-inactive` | | Only operate on inactive projects |
| `--running-only` | | Only act on projects with running containers |
| `--timeout <seconds>` | | Timeout for pull/check operations (0 = disabled) |
| `--dry-run` | `-n` | Show actions without executing |
| `--prune` | `-p` | Run prune after the command |
| `--verbose` | `-v` | Enable verbose output |
| `--help` | `-h` | Show help message |

### Logging Options

| Option | Description |
|--------|-------------|
| `--log-dir <path>` | Directory for log files (default: ROOT) |
| `--log-file <path>` | Specific log file path |
| `--no-log` | Disable file logging (screen only) |
| `--log-off` | Same as `--no-log` |
| `--log-on` | Re-enable file logging |

### Hook Options

| Option | Description |
|--------|-------------|
| `--hooks-dir <path>` | Custom hooks directory (default: `<ROOT>/.stack-manager/hooks`) |
| `--no-hooks` | Disable all hooks |

### Inactive Project Management

```bash
# List all inactive projects
stack-manager.sh --root /docker inactive list

# Mark a project as inactive
stack-manager.sh --root /docker inactive on netbox-docker

# Mark a project as active again
stack-manager.sh --root /docker inactive off netbox-docker
```

---

## Custom Update Hooks

Hooks allow you to define custom update logic for specific projects.

### Hook Location

Default: `<ROOT>/.stack-manager/hooks/`

Override with: `--hooks-dir <path>`

### Hook Naming Convention

```
<phase>-<command>_<project>.sh
```

Where:
- `phase`: `pre` or `post`
- `command`: `update`, `pull`, `check`, `restart`, `down`, `status`, `list`, `up`
- `project`: folder name of the project

**Currently implemented:** Only `post-update_<project>.sh` overrides the update command.

### Hook Arguments

Hooks receive two arguments:
1. `$1` - Project name (folder name)
2. `$2` - Full path to project directory

### Example Hook: NetBox

Create `post-update_netbox-docker.sh`:

```bash
#!/usr/bin/env bash
set -u -o pipefail

PROJECT_NAME="$1"
PROJECT_DIR="$2"

cd "$PROJECT_DIR"

# NetBox requires specific update sequence
docker compose down
git checkout release
git pull -p origin release
docker compose pull
docker compose up -d

echo "NetBox update complete"
```

Make it executable:
```bash
chmod +x /docker/.stack-manager/hooks/post-update_netbox-docker.sh
```

### Important Hook Behavior

When a `post-update_<project>.sh` hook exists:
- The normal `docker compose pull` + `up -d` is **completely skipped**
- Only the hook script runs
- The hook is responsible for all update operations
- Hook failures are tracked and reported in the summary

---

## Logging

### Default Behavior

Logs are written to both screen and file. Default log location:

```
<ROOT>/stack-manager_YYYYmmdd_HHMMSS.log
```

### Examples

```bash
# Use custom log directory
stack-manager.sh --root /docker --log-dir /var/log update

# Use specific log file
stack-manager.sh --root /docker --log-file /tmp/update.log update

# Disable file logging (screen only)
stack-manager.sh --root /docker --no-log update
```

### Log File Fallback

If the specified log location is not writable, the script falls back to `$HOME/stack-manager_<timestamp>.log`.

---

## How It Works

### Project Discovery

1. The script scans the root directory for immediate subdirectories
2. Each subdirectory is checked for a compose file (in priority order)
3. Hidden directories (starting with `.`) are ignored
4. The root directory itself is also checked for a compose file

Function: `discover_projects()`

### Project Name Resolution

Docker Compose requires valid project names. The script handles this in two ways:

1. **Running projects**: Detects the existing compose project label from running containers
2. **New projects**: Sanitizes the folder name (lowercase, valid characters only)

This ensures consistent project naming even when folders have special characters.

Functions: `project_name_for_dir()`, `sanitize_project_name()`, `detect_running_project_label()`

### Command Execution Flow

1. Parse global options and command
2. Initialize logging
3. Initialize hooks directory
4. Discover all projects under root
5. Apply filters (inactive, only, exclude, CLI projects)
6. Execute command for each matching project
7. Track successes, failures, and skipped projects
8. Print summary
9. Exit with appropriate code

### Signal Handling

- **Ctrl-C (SIGINT)**: Sets `STOP_REQUESTED` flag
- Current operation completes
- Loop stops after current project
- Summary is printed
- Exit code 130

---

## Development Guide

### Code Architecture

The script is organized into logical sections:

```
1. Header & Configuration (lines 1-76)
   - Shebang and safety options
   - Default variables
   - Color definitions

2. Utility Functions (lines 78-86)
   - Logging helpers: log_hdr(), say(), warn(), err()
   - Binary checks: has_bin(), need_bin()
   - Array utilities: contains()

3. Core Functions (lines 88-428)
   - usage()
   - Compose file detection
   - Project name handling
   - Project discovery and filtering
   - Logging initialization
   - Hook management
   - Timeout handling
   - Command execution

4. Command Implementations (lines 466-671)
   - cmd_list(), cmd_status(), cmd_check()
   - cmd_pull(), cmd_up(), cmd_update()
   - cmd_restart(), cmd_down()
   - do_prune()

5. Inactive Subcommands (lines 676-730)
   - cmd_inactive_list()
   - cmd_inactive_on()
   - cmd_inactive_off()

6. Argument Parsing & Main (lines 735-835)
   - Option parsing loop
   - Command dispatch
   - Exit handling
```

### Key Functions Reference

| Function | Purpose |
|----------|---------|
| `discover_projects()` | Find all compose projects under root |
| `filter_projects()` | Apply CLI filters to project list |
| `compose_cmd()` | Build compose command string for a project |
| `run_project_op()` | Execute operation with logging and error handling |
| `hook_path()` | Resolve hook file path |
| `run_hook_if_present()` | Execute hook if it exists |
| `run_with_timeout()` | Execute command with optional timeout |
| `is_mutating_command()` | Check if command modifies state |
| `is_project_running()` | Check if project has running containers |

### Adding New Commands

1. Create a `cmd_<name>()` function:
```bash
cmd_mycommand() {
  local -a projects=("$@")
  for dir in "${projects[@]}"; do
    local ccmd; ccmd="$(compose_cmd "$dir" || true)"
    [[ -n "$ccmd" ]] || continue
    run_project_op "$dir" "MyCommand" "$ccmd <docker-compose-args>"
    (( STOP_REQUESTED )) && return 130
  done
}
```

2. Add to the case statement in main:
```bash
case "$CMD" in
  ...
  mycommand) cmd_mycommand "${PROJECTS[@]}" || rc=$? ;;
  ...
esac
```

3. If it's a mutating command, add to `is_mutating_command()`:
```bash
is_mutating_command() {
  case "${CMD:-}" in
    pull|update|restart|down|up|prune|mycommand) return 0 ;;
    *) return 1 ;;
  esac
}
```

4. Update `usage()` with documentation.

### Testing Changes

```bash
# Always test with dry-run first
./stack-manager.sh --root /docker --dry-run update

# Use verbose mode to see filtering decisions
./stack-manager.sh --root /docker --verbose --dry-run list

# Test on a single project
./stack-manager.sh --root /docker --dry-run update myproject

# Test with disabled hooks
./stack-manager.sh --root /docker --no-hooks --dry-run update
```

---

## Troubleshooting

### "No compose projects found under: /docker"

- Verify the root directory exists and contains subdirectories with compose files
- Check that compose files use one of the supported names
- Ensure you're not inside a hidden directory

### Timeouts not working

- Install `timeout` from coreutils: `apt install coreutils` or `brew install coreutils`
- The script warns if timeout is requested but not available

### Hook not running

- Verify hook is executable: `chmod +x hook.sh`
- Check hook naming: `post-update_<exact-folder-name>.sh`
- Ensure hooks are enabled (not using `--no-hooks`)
- Check hooks directory with `--verbose`

### Project name conflicts

- The script sanitizes folder names for Docker Compose compatibility
- Use `--verbose` to see the resolved project name
- Running containers use their existing label; stopped containers use sanitized name

### Permission denied for log file

- The script falls back to `$HOME` if configured log directory is not writable
- Use `--no-log` to disable file logging entirely
- Use `--log-file` to specify a writable location

---

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success - all operations completed |
| 1 | General error (missing arguments, unknown command, etc.) |
| 2 | One or more project operations failed |
| 130 | Interrupted by Ctrl-C |

---

## License

See repository for license information.
