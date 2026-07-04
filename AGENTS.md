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
cd server && API_KEY=dev-key go run ./cmd/server

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

- `server/internal/core/` - discovery, compose execution, hooks, project creation, image source classification, registry login.
- `server/internal/handlers/` - project, system, registry, and skill HTTP handlers.
- `server/internal/middleware/` - API key auth.
- `server/internal/skills/` - modular skill registry and built-in skills.

Core routes are protected with `X-API-Key`; `/health` is public.

Core project routes:

```text
POST   /api/v1/projects
GET    /api/v1/projects
GET    /api/v1/projects/{name}
GET    /api/v1/projects/{name}/images
GET    /api/v1/projects/{name}/status
POST   /api/v1/projects/{name}/pull
POST   /api/v1/projects/{name}/up
POST   /api/v1/projects/{name}/down
POST   /api/v1/projects/{name}/update
POST   /api/v1/projects/{name}/restart
PUT    /api/v1/projects/{name}/inactive
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
- `web/src/pages/Settings.jsx` - browser-stored API key.
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

Required environment:

- `API_KEY`
- `DOCKER_ROOT`
- `DOCKER_GID`
- `SERVER_USER`
- `WEB_PORT`

The server image runs as non-root by default (`SERVER_USER=1001:1001`) and is added to the host Docker socket group via `DOCKER_GID`. For hosts that intentionally require root compose management, set `SERVER_USER=0:0`.

Docker credentials from registry login are stored under:

```text
/docker/.compose-manager/docker-config
```

## GitLab Pipeline

`.gitlab-ci.yml` targets the `docker02` shell runner by default.

- Validation runs `bash -n` and `docker compose config` with CI placeholder values.
- Tests run Go and web builds inside Docker containers so the shell runner does not need local Go or Node.
- Security jobs expect ARPHost runner tools on `PATH`: `semgrep`, `trivy`, `gitleaks`, `trufflehog`, and `dependency-check`.
- `deploy:docker02` is manual-only. It syncs the repo to `/home/debian/docker/compose-manager`, writes `.env` from CI variables, and runs Compose as the `debian` user.
- `COMPOSE_MANAGER_API_KEY` must be set as a masked GitLab CI/CD variable.
- `smoke:docker02` runs after a completed manual deploy.

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
| `ROOT` | `/docker` | no |
| `PORT` | `8192` | no |
| `HOOKS_DIR` | `<ROOT>/.compose-manager/hooks` | no |
| `BACKUP_DIR` | `<ROOT>/.compose-manager/backups` | no |
| `DOCKER_CONFIG` | Docker default | no |

CLI config files load in this order:

1. `/etc/compose-manager.conf`
2. `~/.config/compose-manager.conf`

CLI variables include `ROOT`, `INACTIVE_MARKER`, `LOG_ENABLED`, `LOG_DIR`, `TIMEOUT_SECS`, `HOOKS_ENABLED`, and `HOOKS_DIR`.

## Git And Commits

Use `Co-Authored-By: BarryBot` in commit messages. Do not use "Codex" or "Claude" in commit metadata.
