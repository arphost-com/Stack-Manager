# AGENTS.md

Repository guidance for coding agents working on Compose Manager.

## Project Overview

Compose Manager has two supported surfaces:

1. `compose-manager.sh` - Bash CLI for discovering and managing Docker Compose projects under a root directory.
2. `server/` + `web/` - Go REST API with a React dashboard for compose operations, logs, stats, backups, database checks, security scans, registry login, and project creation.

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

- `server/internal/core/` - discovery, compose execution, hooks, project creation, image source classification, registry login, update policy detection.
- `server/internal/handlers/` - project, system, registry, and skill HTTP handlers.
- `server/internal/middleware/` - API key auth.
- `server/internal/skills/` - modular skill registry and built-in skills.
- `server/internal/storage/` - MariaDB schema, Redis cache helpers, users/jobs/project settings persistence, and legacy flat-file import.

Core routes are protected with login bearer sessions or legacy `X-API-Key`; `/health` and `POST /api/v1/auth/login` are public.

Core project routes:

```text
POST   /api/v1/projects
GET    /api/v1/projects
GET    /api/v1/projects/{name}
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
POST   /api/v1/projects/bulk/{action}
POST   /api/v1/prune
POST   /api/v1/registries/login
```

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
- `debug` - logs, stats, inspect, events, top/processes.
- `backup` - create, list, restore, delete project backups.
- `dbadmin` - discover, health check, and dump supported DB containers.
- `frontend` - serves the React SPA when embedded.

When adding a skill, create `server/internal/skills/<name>/`, implement `Skill`, and register it in `cmd/server/main.go`.

### Web UI

React app in `web/src`.

Important files:

- `web/src/pages/Dashboard.jsx` - main management console, filters, bulk actions, creation, registry login.
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

The server image runs as non-root by default (`SERVER_USER=1001:1001`) and is added to the host Docker socket group via `DOCKER_GID`. For hosts that intentionally require root compose management, set `SERVER_USER=0:0`.

Docker credentials from registry login are stored under:

```text
/home/debian/.compose-manager/docker-config
```

Do not store Compose Manager's own persistent state under the managed Docker root. On docker02 the app checkout is `/home/debian/docker/compose-manager`, while persistent state is `/home/debian/.compose-manager` and is mounted into the containers at `/state`.

Runtime state:

- MariaDB data: `/home/debian/.compose-manager/mariadb`
- Redis append-only data: `/home/debian/.compose-manager/redis`
- Hooks: `/home/debian/.compose-manager/hooks`
- Backups and database dumps: `/home/debian/.compose-manager/backups`
- Docker registry credentials: `/home/debian/.compose-manager/docker-config`
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

| Variable | Default | Required |
| --- | --- | --- |
| `API_KEY` | none | yes |
| `ADMIN_USERNAME` | `admin` | no |
| `ADMIN_PASSWORD` | API key bootstrap fallback | no |
| `DATABASE_DSN` | derived from DB vars | no |
| `DB_HOST` | none | yes if `DATABASE_DSN` unset |
| `DB_PORT` | `3306` | no |
| `DB_NAME` | none | yes if `DATABASE_DSN` unset |
| `DB_USER` | none | yes if `DATABASE_DSN` unset |
| `DB_PASSWORD` | none | yes if `DATABASE_DSN` unset |
| `REDIS_ADDR` | `redis:6379` | no |
| `REDIS_PASSWORD` | none | yes |
| `REDIS_DB` | `0` | no |
| `CACHE_TTL_SECONDS` | `15` | no |
| `ROOT` | `/docker` | no |
| `STATE_DIR` | `$HOME/.compose-manager` | no |
| `PORT` | `8192` | no |
| `HOOKS_DIR` | `<STATE_DIR>/hooks` | no |
| `BACKUP_DIR` | `<STATE_DIR>/backups` | no |
| `DOCKER_CONFIG` | Docker default | no |

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
