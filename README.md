# Compose Manager

A CLI and web dashboard for managing **multiple Docker Compose projects** stored under a single root directory.

Designed for environments with **many stacks**, mixed lifecycle states, and special projects that require **custom update logic** (such as NetBox).

---

## Table of Contents

- [Key Design Rules](#key-design-rules)
- [Features](#features)
- [Requirements](#requirements)
- [Installation](#installation)
- [Web Dashboard](#web-dashboard)
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
- **Web dashboard** for project creation, management, updates, statistics, logging, backups, database checks, image-source classification, registry login, and user management
- **Live action sessions** for update/pull/start/stop/restart with saved logs
- **MariaDB-backed state** for users, action history, and project settings
- **Redis-backed sessions and cache** for project/image/job/settings reads

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

### Quick Install

```bash
# Download and install to /usr/local/bin
chmod +x compose-manager.sh
sudo install -m 0755 compose-manager.sh /usr/local/bin/compose-manager.sh
```

### Verify Installation

```bash
compose-manager.sh --help
```

### Optional: Create Hooks Directory

```bash
mkdir -p /docker/.compose-manager/hooks
```

---

## Web Dashboard

The dashboard runs as a Docker Compose stack with four services:

| Service | Port | Purpose |
|---------|------|---------|
| `server` | `8192` internal | Go API server |
| `web` | `${WEB_PORT:-8193}:8080` | React dashboard through nginx |
| `mariadb` | internal | Users, action history, project settings |
| `redis` | internal | Login sessions and cached project/image/job/settings data |

Example `.env`:

```bash
API_KEY=change-me-to-a-secure-key
ADMIN_USERNAME=admin
# ADMIN_PASSWORD=change-me-to-a-different-secure-password

DB_NAME=compose_manager
DB_USER=compose_manager
DB_PASSWORD=change-me-to-a-secure-database-password
DB_ROOT_PASSWORD=change-me-to-a-secure-root-database-password
REDIS_PASSWORD=change-me-to-a-secure-redis-password
REDIS_DB=0
CACHE_TTL_SECONDS=15

DOCKER_ROOT=/home/debian/docker
STATE_DIR=.compose-manager
DOCKER_GID=998
SERVER_USER=1000:1000
WEB_PORT=8193
HOST_URL=http://docker01:8193
```

Environment reference:

| Variable | Default | Purpose |
|----------|---------|---------|
| `API_KEY` | none | Required legacy/API key used for API access and as the first-admin password fallback when `ADMIN_PASSWORD` is unset. Use a long random value. |
| `ADMIN_USERNAME` | `admin` | Username created only when the MariaDB users table is empty. |
| `ADMIN_PASSWORD` | empty | Optional first-admin password. If empty, the app uses `API_KEY` for initial bootstrap login. Rotate it after first login. |
| `DB_NAME` | `compose_manager` | MariaDB database name for users, action history, project settings, and update policies. |
| `DB_USER` | `compose_manager` | MariaDB application user. |
| `DB_PASSWORD` | none | Required password for the MariaDB application user. |
| `DB_ROOT_PASSWORD` | none | Required MariaDB root password used by the MariaDB container during initialization. |
| `REDIS_PASSWORD` | none | Required password for Redis sessions and cache. |
| `REDIS_DB` | `0` | Redis logical database number. `0` means the first/default Redis DB. Keep `0` for the bundled dedicated Redis container; use another index only if intentionally sharing a Redis server with other apps. |
| `CACHE_TTL_SECONDS` | `15` | Time, in seconds, that project/image/job/settings reads may stay in Redis before refresh. Lower values show changes faster; higher values reduce Docker/API load. |
| `DOCKER_ROOT` | none | Required host directory containing the Docker Compose projects that Compose Manager should discover and manage. This can be any path on the host and is mounted into the server container at `/docker`. |
| `STATE_DIR` | `.compose-manager` | Compose Manager persistent state directory. Relative paths are stored under the Compose Manager root beside `docker-compose.yml`. |
| `DOCKER_GID` | host Docker socket group | Group ID for `/var/run/docker.sock`. The non-root server user is added to this group so it can run Docker commands. |
| `SERVER_USER` | none | Required numeric UID:GID used to run the server container and own `STATE_DIR`, for example `1000:1000`. Use whichever host user/group should manage Docker. Set `0:0` only on hosts that intentionally require root compose management. |
| `WEB_PORT` | `8193` | Host port for the web dashboard. The API server stays internal on port `8192`. |
| `HOST_URL` | detected from hostname and `WEB_PORT` | Dashboard URL printed by setup for the operator. Edit it if users should open a different DNS name or reverse-proxy URL. |

`REDIS_DB` is not a MariaDB setting and does not create another Redis container. Redis has numbered logical databases inside one Redis instance; `0` is the normal default. Compose Manager stores login sessions and short-lived cache keys there. With the included dedicated Redis service, leave it at `0`.

`SERVER_USER` is host-specific. Different Linux boxes may need different numeric IDs, such as `0:0`, `1000:1000`, or `998:998`. Compose Manager uses that same UID:GID for the server process and for prepared state directory ownership so behavior stays consistent across hosts.

Start it:

```bash
./scripts/prepare-state.sh .env
docker compose --env-file .env up -d --build
```

If `.env` does not exist, `prepare-state.sh` creates it from `.env.example`. It fills missing or `change-me...` values with random values for `API_KEY`, `ADMIN_PASSWORD`, `DB_PASSWORD`, `DB_ROOT_PASSWORD`, and `REDIS_PASSWORD`; sets practical defaults for the other settings; writes everything back to `.env`; and prints the resulting settings, including `HOST_URL`. Edit host-specific values such as `DOCKER_ROOT`, `SERVER_USER`, `DOCKER_GID`, `WEB_PORT`, and `HOST_URL` as needed for that box.

Open `http://<host>:8193`. If the MariaDB users table is empty, the first admin is created from `ADMIN_USERNAME` and `ADMIN_PASSWORD`. If `ADMIN_PASSWORD` is unset, the bootstrap password is `API_KEY`; rotate or add users from Settings after first login.

The preparation step is required on manual installs such as docker01. The Compose file uses `create_host_path: false` for bind mounts so Docker will not silently create `STATE_DIR` as `root`. If `STATE_DIR` was already created as root, stop the stack and repair ownership once:

```bash
docker compose --env-file .env down
. ./.env
sudo chown -R "$(id -u):$(id -g)" "${STATE_DIR}"
./scripts/prepare-state.sh .env
docker compose --env-file .env up -d --build
```

After `git pull`, use `docker compose --env-file .env up -d --build` instead of plain `docker compose up -d` when Dockerfiles or nginx/server config changed. Plain `up -d` can reuse old images and keep old nginx behavior.

Persistent state is stored under `STATE_DIR`:

| Path | Purpose |
|------|---------|
| `mariadb/` | MariaDB data for users, jobs, project settings |
| `redis/` | Redis append-only data for sessions/cache |
| `hooks/` | Update hooks used by the API server |
| `backups/` | Project backups and database dumps |
| `docker-config/` | Docker registry credentials from dashboard registry login |

Persistent state stays under the Compose Manager root. Managed Compose projects stay under `DOCKER_ROOT`, which can be any host directory chosen for that machine.

Legacy files from earlier versions are imported on startup if present:

- `/state/users.json`
- `/state/jobs/*.json`

The app no longer writes users, sessions, project settings, or action history as flat files.

### Dashboard Update Policies

The dashboard can mark a project as not updateable when it is built directly from a Dockerfile in a GitHub/GitLab checkout and has no registry image to pull. Auto detection checks the project Git remote and parsed Compose image/build metadata.

| Mode | Behavior |
|------|----------|
| `auto` | Detect build-only GitHub/GitLab projects and mark them `no_updates`; otherwise allow updates |
| `allow` | Always run update actions |
| `no_updates` | Skip update actions and save a skipped action session with the configured reason |

Use the Project Detail overview page to view or override the policy.

### GitLab Pipeline

The GitLab pipeline treats docker02 as the dev environment:

- `deploy:docker02` runs automatically on the default branch after validation, tests, builds, and security scans pass.
- The deploy job preserves existing docker02 `.env` secrets or generates secure first-run values when GitLab CI variables are not set.
- `smoke:docker02` runs automatically after the dev deploy.
- `push:github` is an optional manual production-style job that pushes the tested default branch to `arphost-com/Compose-Manager` with the masked `GITHUB_PAT` CI variable.
- If `push:github` is clicked before `GITHUB_PAT` is configured, the job fails with a clear message and does not push.
- After pushing, `push:github` verifies that GitHub `main` matches the GitLab commit SHA.

---

## Project Layout

### Directory Structure

The CLI expects projects organized under a root directory:

```
/docker/                          # Root directory (configurable with --root)
├── .compose-manager/             # Configuration directory
│   └── hooks/                    # Custom update hooks
│       └── post-update_netbox-docker.sh
├── project-a/
│   └── compose.yml
├── project-b/
│   ├── compose.yml
│   └── .inactive                 # Marker file - project is skipped
├── netbox-docker/
│   └── docker-compose.yml
└── compose-manager_20240115_143022.log  # Auto-generated log file
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
compose-manager.sh --root /docker list

# Show detailed container status per project
compose-manager.sh --root /docker status

# Check for image updates (pulls but doesn't restart)
compose-manager.sh --root /docker check

# Pull latest images for all projects
compose-manager.sh --root /docker pull

# Start all projects
compose-manager.sh --root /docker up

# Update all projects (uses hooks if present)
compose-manager.sh --root /docker update

# Restart all running projects
compose-manager.sh --root /docker restart

# Stop all projects
compose-manager.sh --root /docker down

# Prune unused Docker resources
compose-manager.sh --root /docker prune
```

### Operating on Specific Projects

```bash
# Update only specific projects
compose-manager.sh --root /docker update project-a project-b

# Restart a single project
compose-manager.sh --root /docker restart netbox-docker
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
| `--hooks-dir <path>` | Custom hooks directory (default: `<ROOT>/.compose-manager/hooks`) |
| `--no-hooks` | Disable all hooks |

### Inactive Project Management

```bash
# List all inactive projects
compose-manager.sh --root /docker inactive list

# Mark a project as inactive
compose-manager.sh --root /docker inactive on netbox-docker

# Mark a project as active again
compose-manager.sh --root /docker inactive off netbox-docker
```

---

## Custom Update Hooks

Hooks allow you to define custom update logic for specific projects.

### Hook Location

Default: `<ROOT>/.compose-manager/hooks/`

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
chmod +x /docker/.compose-manager/hooks/post-update_netbox-docker.sh
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
<ROOT>/compose-manager_YYYYmmdd_HHMMSS.log
```

### Examples

```bash
# Use custom log directory
compose-manager.sh --root /docker --log-dir /var/log update

# Use specific log file
compose-manager.sh --root /docker --log-file /tmp/update.log update

# Disable file logging (screen only)
compose-manager.sh --root /docker --no-log update
```

### Log File Fallback

If the specified log location is not writable, the script falls back to `$HOME/compose-manager_<timestamp>.log`.

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
./compose-manager.sh --root /docker --dry-run update

# Use verbose mode to see filtering decisions
./compose-manager.sh --root /docker --verbose --dry-run list

# Test on a single project
./compose-manager.sh --root /docker --dry-run update myproject

# Test with disabled hooks
./compose-manager.sh --root /docker --no-hooks --dry-run update
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
