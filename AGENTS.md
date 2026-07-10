# AGENTS.md

Repository guidance for coding agents working on Stack Manager.

## Project Overview

Stack Manager has two supported surfaces:

1. `stack-manager.sh` - Bash CLI for discovering and managing Docker Compose projects under a root directory.
2. `server/` + `web/` - Go REST API with a React dashboard for compose operations, logs, stats, backups, database checks, security scans, registry login, stack templates, schedules, agents, background metrics, scoped project shell, and project creation/deletion.

Use the existing patterns. Keep CLI and API behavior aligned when both surfaces implement the same operation.

## Build And Test

```bash
# CLI
bash -n stack-manager.sh
./stack-manager.sh --root /docker --dry-run update

# Server
cd server && go test ./...
cd server && go build ./cmd/server
cd server && API_KEY=dev-key-change-me-123 go run ./cmd/server

# Web
cd web && npm ci
cd web && npm run build
cd web && npm run dev

# Whole repo
make test
make build
make docker-build
```

`web/package-lock.json` is required. Do not remove it; Docker and `make build` rely on `npm ci`.

## Architecture

### CLI

`stack-manager.sh` is a single Bash tool. It discovers compose projects, applies filters, executes docker compose commands, runs hooks, and prints a summary.

Important CLI behavior:

- Update hook override: if `post-update_<project>.sh` exists in the hooks directory, `update` runs only that hook and skips normal pull/up.
- `-p` means `--prune`, not project.
- Flags must come before the command.
- Mutating commands must be listed in `is_mutating_command()`.

### API Server

Entry point: `server/cmd/server/main.go`

Main packages:

- `server/internal/core/` - discovery, compose execution, hooks, project creation/deletion, stack templates, scheduler, image source classification, registry login, update policy detection.
- `server/internal/handlers/` - project, system, registry, and skill HTTP handlers.
- `server/internal/middleware/` - API key auth.
- `server/internal/skills/` - modular skill registry and built-in skills.
- `server/internal/storage/` - MariaDB schema, Redis cache helpers, users/jobs/project settings/agents/schedules persistence, and legacy flat-file import.

Core routes are protected with login bearer sessions or legacy `X-API-Key`; `/health` and `POST /api/v1/auth/login` are public.

Core project routes:

```text
POST   /api/v1/projects
GET    /api/v1/projects
GET    /api/v1/projects/{name}
DELETE /api/v1/projects/{name}
GET    /api/v1/projects/{name}/images
GET    /api/v1/projects/{name}/status
GET    /api/v1/projects/{name}/update-policy
PUT    /api/v1/projects/{name}/update-policy
POST   /api/v1/projects/{name}/pull
POST   /api/v1/projects/{name}/up
POST   /api/v1/projects/{name}/down
POST   /api/v1/projects/{name}/update
POST   /api/v1/projects/{name}/restart
POST   /api/v1/projects/{name}/jobs/{action}
PUT    /api/v1/projects/{name}/inactive
GET    /api/v1/jobs
GET    /api/v1/jobs/{jobId}
GET    /api/v1/stack-templates
GET    /api/v1/stack-templates/{templateID}
GET    /api/v1/agents
POST   /api/v1/agents
DELETE /api/v1/agents/{agentID}
GET    /api/v1/agents/{agentID}/projects
POST   /api/v1/agents/{agentID}/commands   # queue a command for a callback agent
GET    /api/v1/agents/{agentID}/commands
ALL    /api/v1/agent-proxy/{agentID}/*      # forward to a peer /api/v1 or inbound agent /agent/v1
POST   /api/v1/agent-checkin/projects       # public: callback agent pushes inventory, gets queued commands
POST   /api/v1/agent-checkin/results        # public: callback agent reports command outcomes
GET    /api/v1/system/gpu                    # detect NVIDIA runtime/GPU
POST   /api/v1/system/gpu/test               # run a --gpus all nvidia-smi container
GET    /api/v1/system/info                   # server display name (setting > SERVER_DISPLAY_NAME > hostname)
PUT    /api/v1/system/info
GET    /api/v1/schedules
POST   /api/v1/schedules
DELETE /api/v1/schedules/{scheduleID}
POST   /api/v1/schedules/{scheduleID}/run
POST   /api/v1/projects/bulk/{action}
POST   /api/v1/prune
POST   /api/v1/registries/login
```

Project directory deletion must require the project to be inactive first, require exact-name confirmation, run only against a discovered project, and refuse to delete the configured root or paths outside `ROOT`.

`APP_MODE=agent` runs the same server binary without MariaDB/Redis and exposes `/agent/v1` with bearer-token auth via `AGENT_TOKEN`. Keep controller routes under `/api/v1`; keep agent runtime routes under `/agent/v1`.

### Cross-server (peers, agents, command queue)

The dashboard's "All Servers" view aggregates the local controller plus every registered server. A server is one of:

- **Peer controller** (`mode=peer`) — another full controller, added by URL + API key. `handlers/peer.go` live-fetches its projects over its `/api/v1` (pinned to `source=local` to avoid mutual-peer recursion). Keep-alive is disabled and a 5-minute last-good snapshot is cached so a single failed poll doesn't blank the peer.
- **Inbound/both agent** — the controller reaches it directly; `handlers/agent_proxy.go` forwards project operations to its `/agent/v1`.
- **Callback agent** (`APP_MODE=agent-callback`) — push-only. It POSTs inventory to `/api/v1/agent-checkin/projects` and receives queued commands in the reply; it runs them via its engine and POSTs outcomes to `/api/v1/agent-checkin/results`. Commands live in the `agent_commands` table (`Enqueue/ClaimPending/SaveResult/List` in `storage/store.go`). Check-in auth matches the agent name to its stored token.

`agent_proxy.go` (`/api/v1/agent-proxy/{id}/*`) is the single forward path used by the web client's `projectsForSource`/`systemForSource`/`jobsForSource` helpers, so opening a peer/agent project routes to the owning host. All server-to-server clients (peer, agent-proxy, callback check-in) require **TLS 1.3** and skip cert verification (self-signed controllers).

The UI footer version comes from `web/package.json` + a git short SHA injected at build via `VITE_GIT_SHA` (Dockerfile build arg, set by CI) — never hand-edit it. `GET /api/v1/system/info` resolves the server display name from the stored setting, then `SERVER_DISPLAY_NAME`, then the OS hostname.

Response envelope:

```json
{"status":"ok","data":{},"timestamp":"..."}
{"status":"error","error":"message","timestamp":"..."}
```

### Skills

Every skill implements `server/internal/skills/registry.go:Skill`:

- `Name()`, `Description()`, `Version()`
- `Init(ctx, engine, cfg)`
- `RegisterRoutes(r chi.Router)`
- `HealthCheck(ctx)`, `Shutdown(ctx)`

Built-in skills:

- `security` - image scans and compose config audit.
- `debug` - logs, stats, inspect, events, top/processes, and scoped project shell troubleshooting.
- `backup` - create, list, download, restore, delete project backups, and manage backup endpoints.
- `dbadmin` - discover, health check, and dump supported DB containers.
- `frontend` - serves the React SPA when embedded.

When adding a skill, create `server/internal/skills/<name>/`, implement `Skill`, and register it in `cmd/server/main.go`.

### Web UI

React app in `web/src`.

Important files:

- `web/src/pages/Dashboard.jsx` - main management console, clickable summary filters, stack catalog, schedules, agents, filters, bulk actions, creation, deletion, registry login.
- `web/src/pages/ProjectDetail.jsx` - project actions, image sources, logs, stats, security, backups, DB checks, inspect, processes.
- `web/src/pages/Login.jsx` - username/password login.
- `web/src/pages/Settings.jsx` - account info, logout, and admin user management.
- `web/src/api/client.js` - API client.
- `web/src/index.css` - Tailwind component classes.

UI expectations:

- Keep controls practical and dense; this is an operations tool.
- Every form input needs a native `title` tooltip. In `web/src/pages/Settings.jsx` use the `Field` helper (which propagates `title` to the whole label and supports an optional `hint` for inline help under the input).
- When a client-side serializer converts free-form input to structured JSON (e.g. `default-address-pools`), throw on malformed input with a specific per-line message. Never silently drop fields — Docker Settings previously ate `default-address-pools` when a line missed its `,size` because the mapper filtered failed parses.
- Show destructive actions distinctly.
- Avoid adding marketing/landing-page content.

## Docker Deployment

Services:

| Service | Port | Purpose |
| --- | --- | --- |
| `server` | 8192 internal | Go API server |
| `web` | `${WEB_HTTP_PORT:-8193}:8080` (redirect + ACME) and `${WEB_SSL_PORT:-8993}:8443` (TLS) | React SPA via nginx; TLS terminated with self-signed cert generated on first boot in `${STATE_DIR}/ssl/`. Set both ports to 80/443 to run Let's Encrypt end-to-end. |
| `mariadb` | internal | Users, job history, project settings |
| `redis` | internal | Sessions and cached projects/images/jobs/settings |

Required environment:

- `API_KEY`
- `DB_PASSWORD`
- `DB_ROOT_PASSWORD`
- `REDIS_PASSWORD`
- `DOCKER_ROOT`
- `DOCKER_GID`
- `SERVER_USER`
- `WEB_HTTP_PORT`
- `WEB_SSL_PORT`

The server container runs as the numeric host UID:GID configured by `SERVER_USER` and is added to the host Docker socket group via `DOCKER_GID`. For hosts that intentionally require root compose management, set `SERVER_USER=0:0`.
`SERVER_USER` is host-specific and may be `0:0`, `1000:1000`, `998:998`, or another numeric UID:GID depending on the box. Keep prepared `STATE_DIR` ownership aligned with this value.
For mixed-permission stacks such as GitLab or PMM, prefer keeping Stack Manager as the host service UID and making the compose project files readable by that UID/GID. Docker can still run the target containers as root. If a project directory must stay root-only, use a separate root-capable agent with `SERVER_USER=0:0`.

Docker credentials from registry login are stored under:

```text
/home/debian/docker/stack-manager/.stack-manager/docker-config
```

Store Stack Manager's own persistent state under the Stack Manager root. Managed projects live under the host-specific `DOCKER_ROOT`, which can be any directory selected for that machine. docker02 uses `/home/debian/docker` as its GitLab deploy target, but that is not a global default.

Manual installs should run `./scripts/prepare-state.sh .env` before `docker compose up`. The Compose stack also includes a `state-init` service that repairs `STATE_DIR` ownership before app services start. If state paths already exist with wrong ownership, stop the stack and repair only Stack Manager state with `sudo chown -R <uid>:<gid> "$STATE_DIR"` before restarting. Do not recursively chown `DOCKER_ROOT`; it contains managed projects that may have their own owners.
`prepare-state.sh` creates `.env` from `.env.example` if needed, generates random values for missing or `change-me...` secrets, writes all setup values back to `.env`, and prints the resulting settings including `HOST_URL`.

Runtime state:

- MariaDB data: `<stack-manager-root>/.stack-manager/mariadb`
- Redis append-only data: `<stack-manager-root>/.stack-manager/redis`
- Hooks: `<stack-manager-root>/.stack-manager/hooks`
- Backups and database dumps: `<stack-manager-root>/.stack-manager/backups`
- Docker registry credentials: `<stack-manager-root>/.stack-manager/docker-config`
- Legacy `/state/users.json` and `/state/jobs/*.json` are imported into MariaDB on startup if present; the app no longer writes users or action history as flat files.

## GitLab Pipeline

`.gitlab-ci.yml` targets the `docker02` shell runner by default.

- Validation runs `bash -n` and `docker compose config` with CI placeholder values.
- Tests run Go and web builds inside Docker containers so the shell runner does not need local Go or Node.
- Security jobs expect ARPHost runner tools on `PATH`: `semgrep`, `trivy`, `gitleaks`, `trufflehog`, and `dependency-check`.
- `deploy:docker02` is the automatic dev deployment for the default branch. It syncs the repo to `/home/debian/docker/stack-manager`, writes `.env`, and runs Compose as the `debian` user.
- `STACK_MANAGER_API_KEY`, `STACK_MANAGER_DB_PASSWORD`, `STACK_MANAGER_DB_ROOT_PASSWORD`, and `STACK_MANAGER_REDIS_PASSWORD` are optional for docker02 dev. If unset, the deploy job preserves existing `.env` values or generates secure first-run values.
- `smoke:docker02` runs automatically after dev deploy and reads the deployed API key from `/home/debian/docker/stack-manager/.env`.
- `push:github` is the optional manual production-style job. It pushes the tested default branch to `arphost-com/Stack-Manager` using masked `GITHUB_PAT` without blocking the green docker02 dev pipeline.
- If `push:github` is run without `GITHUB_PAT`, it fails with a clear message and does not push.
- After pushing, `push:github` verifies that GitHub `main` matches the GitLab commit SHA.

## Security Guardrails

- Never use `shell=True` or equivalent shell string execution for subprocess calls.
- Do not hardcode credentials or weak default passwords.
- Use `docker login --password-stdin` for registry auth.
- Validate project names and backup IDs before using them in filesystem paths.
- Do not allow logs/inspect/top access to containers outside the selected project.
- Keep authenticated API errors in the standard JSON envelope.
- Prefer non-root containers and absolute `WORKDIR`.
- Do not disable CSRF or mark request-driven template output as raw HTML.

## Configuration

Server env vars:

| Variable | Default | Required | Purpose |
| --- | --- | --- | --- |
| `BASE_IMAGE_PREFIX` | empty | no | Prefix prepended to every Docker Hub base image (`FROM` lines, compose `image:` refs, runtime `alpine:3.22` helpers). Empty = pull straight from Docker Hub. On docker02 the pipeline sets `10.10.10.96:8929/arphost/dependency_proxy/containers/library/` to route through the GitLab dependency proxy. Must end with a trailing slash. |
| `API_KEY` | none | yes | Legacy/API key and first-admin password fallback when `ADMIN_PASSWORD` is unset. |
| `ADMIN_USERNAME` | `admin` | no | First admin username when the MariaDB users table is empty. |
| `ADMIN_PASSWORD` | API key bootstrap fallback | no | Optional first admin password. Rotate or replace it after first login. |
| `DATABASE_DSN` | derived from DB vars | no | Full MariaDB DSN override. Usually leave unset in Compose. |
| `DB_HOST` | none | yes if `DATABASE_DSN` unset | MariaDB host, normally `mariadb` in Compose. |
| `DB_PORT` | `3306` | no | MariaDB port. |
| `DB_NAME` | none | yes if `DATABASE_DSN` unset | MariaDB database for users, jobs, project settings, and update policies. |
| `DB_USER` | none | yes if `DATABASE_DSN` unset | MariaDB application user. |
| `DB_PASSWORD` | none | yes if `DATABASE_DSN` unset | MariaDB application password. |
| `REDIS_ADDR` | `redis:6379` | no | Redis host and port, normally the bundled Redis service. |
| `REDIS_PASSWORD` | none | yes | Redis password for sessions and cache. |
| `REDIS_DB` | `0` | no | Redis logical database index. `0` is the first/default Redis DB; keep it for the bundled dedicated Redis service. |
| `CACHE_TTL_SECONDS` | `15` | no | Redis cache TTL for project/image/job/settings reads. Lower values refresh faster; higher values reduce Docker/API load. |
| `METRICS_REFRESH_MINUTES` | `15` | no | Background interval for project cache warmup and Docker stats snapshots. Minimum 15. |
| `WARM_CACHE_TTL_MINUTES` | `30` | no | Redis TTL for background-warmed project/image metadata. |
| `ROOT` | `/docker` | no | In-container path where managed Compose projects are mounted. |
| `STATE_DIR` | `.stack-manager` | no | Host state directory before container mount. Relative paths resolve under the Stack Manager root; in Compose the server uses `/state` inside the container. |
| `PORT` | `8192` | no | API server listen port. |
| `HOOKS_DIR` | `<STATE_DIR>/hooks` | no | Directory for API update hooks. |
| `BACKUP_DIR` | `<STATE_DIR>/backups` | no | Directory for backups and database dumps. |
| `BACKUP_TARGET_ROOT` | `.stack-manager/backup-targets` | no | Host path mounted at `/backup-targets` for UI-configured local, CIFS, NFS, and Linux mount backup endpoints. |
| `DOCKER_CONFIG` | Docker default | no | Docker CLI config path for registry logins. |
| `HOST_URL` | detected by setup | no | Operator-facing dashboard URL printed by setup; not required by the server runtime. |

`REDIS_DB=0` is not related to MariaDB. It selects Redis logical database 0 inside one Redis instance. Use another number only if intentionally sharing a Redis server and you need Stack Manager keys isolated from other apps.

If the MariaDB `users` table is empty, the server creates the first admin from `ADMIN_USERNAME` and `ADMIN_PASSWORD`. If `ADMIN_PASSWORD` is empty, it uses `API_KEY` as the bootstrap password. Rotate or add users from the Settings page after first login.

Update policies:

- `auto`: default. Build-only projects from GitHub/GitLab with no registry image are automatically treated as `no_updates`.
- `allow`: always permit update actions.
- `no_updates`: skip update actions and record a skipped action session with the reason.

CLI config files load in this order:

1. `/etc/stack-manager.conf`
2. `~/.config/stack-manager.conf`

CLI variables include `ROOT`, `INACTIVE_MARKER`, `LOG_ENABLED`, `LOG_DIR`, `TIMEOUT_SECS`, `HOOKS_ENABLED`, and `HOOKS_DIR`.

## Git And Commits

Use `Co-Authored-By: BarryBot` in commit messages. Do not use "Codex" or "Claude" in commit metadata.
