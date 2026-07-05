# AGENTS.md

Repository guidance for coding agents working on Compose Manager.

## Project Overview

Compose Manager has two supported surfaces:

1. `compose-manager.sh` - Bash CLI for discovering and managing Docker Compose projects under a root directory.
2. `server/` + `web/` - Go REST API with a React dashboard for compose operations, logs, stats, backups, database checks, security scans, registry login, stack templates, schedules, agents, background metrics, scoped project shell, and project creation/deletion.

Use the existing patterns. Keep CLI and API behavior aligned when both surfaces implement the same operation.

## Build And Test

```bash
# CLI
bash -n compose-manager.sh
./compose-manager.sh --root /docker --dry-run update

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

`compose-manager.sh` is a single Bash tool. It discovers compose projects, applies filters, executes docker compose commands, runs hooks, and prints a summary.

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
- Use native `title` tooltips or an existing local tooltip pattern for operational controls.
- Show destructive actions distinctly.
- Avoid adding marketing/landing-page content.

## Docker Deployment

Services:

| Service | Port | Purpose |
| --- | --- | --- |
| `server` | 8192 internal | Go API server |
| `web` | `${WEB_PORT:-8193}:8080` | React SPA via nginx |
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
- `WEB_PORT`

The server container runs as the numeric host UID:GID configured by `SERVER_USER` and is added to the host Docker socket group via `DOCKER_GID`. For hosts that intentionally require root compose management, set `SERVER_USER=0:0`.
`SERVER_USER` is host-specific and may be `0:0`, `1000:1000`, `998:998`, or another numeric UID:GID depending on the box. Keep prepared `STATE_DIR` ownership aligned with this value.
For mixed-permission stacks such as GitLab or PMM, prefer keeping Compose Manager as the host service UID and making the compose project files readable by that UID/GID. Docker can still run the target containers as root. If a project directory must stay root-only, use a separate root-capable agent with `SERVER_USER=0:0`.

Docker credentials from registry login are stored under:

```text
/home/debian/docker/compose-manager/.compose-manager/docker-config
```

Store Compose Manager's own persistent state under the Compose Manager root. Managed projects live under the host-specific `DOCKER_ROOT`, which can be any directory selected for that machine. docker02 uses `/home/debian/docker` as its GitLab deploy target, but that is not a global default.

Manual installs should run `./scripts/prepare-state.sh .env` before `docker compose up`. The Compose stack also includes a `state-init` service that repairs `STATE_DIR` ownership before app services start. If state paths already exist with wrong ownership, stop the stack and repair only Compose Manager state with `sudo chown -R <uid>:<gid> "$STATE_DIR"` before restarting. Do not recursively chown `DOCKER_ROOT`; it contains managed projects that may have their own owners.
`prepare-state.sh` creates `.env` from `.env.example` if needed, generates random values for missing or `change-me...` secrets, writes all setup values back to `.env`, and prints the resulting settings including `HOST_URL`.

Runtime state:

- MariaDB data: `<compose-manager-root>/.compose-manager/mariadb`
- Redis append-only data: `<compose-manager-root>/.compose-manager/redis`
- Hooks: `<compose-manager-root>/.compose-manager/hooks`
- Backups and database dumps: `<compose-manager-root>/.compose-manager/backups`
- Docker registry credentials: `<compose-manager-root>/.compose-manager/docker-config`
- Legacy `/state/users.json` and `/state/jobs/*.json` are imported into MariaDB on startup if present; the app no longer writes users or action history as flat files.

## GitLab Pipeline

`.gitlab-ci.yml` targets the `docker02` shell runner by default.

- Validation runs `bash -n` and `docker compose config` with CI placeholder values.
- Tests run Go and web builds inside Docker containers so the shell runner does not need local Go or Node.
- Security jobs expect ARPHost runner tools on `PATH`: `semgrep`, `trivy`, `gitleaks`, `trufflehog`, and `dependency-check`.
- `deploy:docker02` is the automatic dev deployment for the default branch. It syncs the repo to `/home/debian/docker/compose-manager`, writes `.env`, and runs Compose as the `debian` user.
- `COMPOSE_MANAGER_API_KEY`, `COMPOSE_MANAGER_DB_PASSWORD`, `COMPOSE_MANAGER_DB_ROOT_PASSWORD`, and `COMPOSE_MANAGER_REDIS_PASSWORD` are optional for docker02 dev. If unset, the deploy job preserves existing `.env` values or generates secure first-run values.
- `smoke:docker02` runs automatically after dev deploy and reads the deployed API key from `/home/debian/docker/compose-manager/.env`.
- `push:github` is the optional manual production-style job. It pushes the tested default branch to `arphost-com/Compose-Manager` using masked `GITHUB_PAT` without blocking the green docker02 dev pipeline.
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
| `STATE_DIR` | `.compose-manager` | no | Host state directory before container mount. Relative paths resolve under the Compose Manager root; in Compose the server uses `/state` inside the container. |
| `PORT` | `8192` | no | API server listen port. |
| `HOOKS_DIR` | `<STATE_DIR>/hooks` | no | Directory for API update hooks. |
| `BACKUP_DIR` | `<STATE_DIR>/backups` | no | Directory for backups and database dumps. |
| `BACKUP_TARGET_ROOT` | `.compose-manager/backup-targets` | no | Host path mounted at `/backup-targets` for UI-configured local, CIFS, NFS, and Linux mount backup endpoints. |
| `DOCKER_CONFIG` | Docker default | no | Docker CLI config path for registry logins. |
| `HOST_URL` | detected by setup | no | Operator-facing dashboard URL printed by setup; not required by the server runtime. |

`REDIS_DB=0` is not related to MariaDB. It selects Redis logical database 0 inside one Redis instance. Use another number only if intentionally sharing a Redis server and you need Compose Manager keys isolated from other apps.

If the MariaDB `users` table is empty, the server creates the first admin from `ADMIN_USERNAME` and `ADMIN_PASSWORD`. If `ADMIN_PASSWORD` is empty, it uses `API_KEY` as the bootstrap password. Rotate or add users from the Settings page after first login.

Update policies:

- `auto`: default. Build-only projects from GitHub/GitLab with no registry image are automatically treated as `no_updates`.
- `allow`: always permit update actions.
- `no_updates`: skip update actions and record a skipped action session with the reason.

CLI config files load in this order:

1. `/etc/compose-manager.conf`
2. `~/.config/compose-manager.conf`

CLI variables include `ROOT`, `INACTIVE_MARKER`, `LOG_ENABLED`, `LOG_DIR`, `TIMEOUT_SECS`, `HOOKS_ENABLED`, and `HOOKS_DIR`.

## Git And Commits

Use `Co-Authored-By: BarryBot` in commit messages. Do not use "Codex" or "Claude" in commit metadata.
