# Installing Stack Manager

This is the one place that describes how to install and update Stack Manager
itself. The README, the Platform Guide and the in-app Documentation page link
here. If something goes wrong, see [Troubleshooting](TROUBLESHOOTING.md).

- [Before you start](#before-you-start)
- [Install the dashboard](#install-the-dashboard)
- [Log in and open the firewall](#log-in-and-open-the-firewall)
- [Update Stack Manager](#update-stack-manager)
- [Install an agent on another host](#install-an-agent-on-another-host)
- [Use the CLI only](#use-the-cli-only)
- [What the setup script does](#what-the-setup-script-does)

## Before you start

You need a Linux host with Docker Engine, the Docker Compose plugin and git.

Run the install as a normal user that is in the `docker` group, not with sudo
and not as root. Check with `docker ps`. If that works without sudo you are
ready. If it does not, add the user to the group with
`sudo usermod -aG docker $USER`, then log out and back in.

This matters because the setup script takes its defaults from whoever runs it.
Run it as `debian` and Stack Manager runs as that user and manages projects in
`/home/debian/docker`. Run it with sudo and it runs as root and manages
`/root/docker` instead, which is usually not what you want. The only step that
ever asks for sudo is a DNS fix on hosts that use systemd-resolved, and the
script asks for it itself.

The dashboard listens on two ports.

| Port | Use |
|---|---|
| 8993 | HTTPS dashboard |
| 8193 | HTTP, redirects to HTTPS and serves Let's Encrypt challenges |

Agents listen on 8192 when they run in inbound mode. Nothing else is published.

## Install the dashboard

```bash
mkdir -p ~/docker && cd ~/docker
git clone https://github.com/arphost-com/Stack-Manager.git stack-manager
cd stack-manager
./scripts/prepare-state.sh .env
docker compose --env-file .env up -d --build
```

The first build takes a few minutes. When the setup script finishes it prints
the dashboard URL and the admin login. Save that login. You can change the
password after you sign in.

`make docker-up` runs the same two commands if you prefer make.

If your managed projects live somewhere other than `~/docker`, edit
`DOCKER_ROOT` in `.env` and run `./scripts/prepare-state.sh .env` again before
`docker compose up`. The script creates the folder for you. Extra folders to
scan can go in `EXTRA_DOCKER_ROOTS`, separated by commas.

## Log in and open the firewall

Browse to `https://<host-ip>:8993`. The first certificate is self-signed, so
the browser will warn once. To use a real certificate, see TLS in the
[Platform Guide](PLATFORM.md#tls--ssl).

If the page does not load from another machine, the host firewall is almost
always the reason. Open 8993 and 8193 in whatever manages the firewall on that
host. On a plain host that is usually `ufw` or nftables. On a host with a
control panel such as CloudPanel, Plesk or cPanel, open the ports in the panel,
because those panels rewrite the firewall and can remove rules added from the
shell. Stack Manager only manages the firewall itself when ConfigServer csf is
installed.

The same applies to every stack you deploy. Stack Manager starts the
containers, but it does not open their ports on the host firewall unless csf is
installed and you use the Open Ports button.

## Update Stack Manager

Pick the way that matches how the host was installed.

### Hosts installed from git, as above

Use **Settings > Update > Update now** in the dashboard. It shows the pending
changes, then pulls and rebuilds in the background on the host, so it survives
the dashboard restarting. You can also do the same by hand.

```bash
cd ~/docker/stack-manager
git pull
./scripts/prepare-state.sh .env
docker compose --env-file .env up -d --build
```

Always use `up -d --build`. Plain `up -d` keeps running the old images. Running
`prepare-state.sh` again is safe because it never replaces existing secrets.

`./scripts/deploy.sh` also still works. It does the same steps, refreshes the
host helper scripts with sudo and records the version. It is optional now,
because the server records its own version on startup and the helpers are
installed automatically every time the stack starts.

### Hosts deployed by the GitLab pipeline

These hosts carry a `.stack-manager-ci-deployed` marker and refuse
**Update now**, because a git reset there would roll back to a stale mirror.
Update them by running their pipeline instead.

## Install an agent on another host

An agent lets one dashboard manage Docker projects on another host. It has no
dashboard, database or Redis of its own.

On the new host, as a normal user in the `docker` group,

```bash
mkdir -p ~/docker && cd ~/docker
git clone https://github.com/arphost-com/Stack-Manager.git stack-manager
cd stack-manager
./scripts/prepare-state.sh --agent --mode callback --controller https://<dashboard-host>:8993 .env
docker compose --env-file .env -f docker-compose.agent.yml up -d --build
```

The script prints an agent name and token. On the dashboard, open
**Settings > Agents > Add Agent** and enter that name and token. The two must
match exactly or the agent's check-ins are rejected.

Choose the mode by which side can reach the other.

| Mode | When to use it | Register it as |
|---|---|---|
| `callback` | The agent can reach the dashboard but not the other way round, for example behind NAT. This is the safest default. | Outbound check-in, Agent URL left blank |
| `inbound` | The dashboard can reach the agent on port 8192. | Inbound URL, `http://<agent-host>:8192` |
| `both` | You want check-ins and direct access. | Outbound check-in, unless inbound access is reliable |

The inbound agent listener is plain HTTP on 8192, so use an `http://` URL for
it and keep that port reachable only from the dashboard host.
`--controller` is not needed for `inbound` mode. For details and the command
queue see [Agent Modes](AGENT_MODES.md).

To link two full dashboards as peers instead, install the dashboard on both
hosts and add each one to the other under **Settings > Agents** with its URL
and API key.

## Use the CLI only

`stack-manager.sh` is a single Bash script that manages Compose projects
without the dashboard.

```bash
./stack-manager.sh --root ~/docker list
./stack-manager.sh --root ~/docker --dry-run update
```

Options go before the command. Without `--root` the CLI uses `/docker`, which
is different from the dashboard default of `~/docker`, so pass `--root` or set
`ROOT` in `/etc/stack-manager.conf` or `~/.config/stack-manager.conf`. Run
`./stack-manager.sh --help` for every option.

## What the setup script does

`scripts/prepare-state.sh` is safe to run any number of times.

- It copies `.env.example` to `.env` if there is no `.env` yet.
- It replaces every `change-me` placeholder with a random secret and never
  touches a secret that is already set.
- It detects `DOCKER_GID` from the Docker socket and sets `SERVER_USER` to the
  user running it.
- It creates `DOCKER_ROOT` and the state folder `.stack-manager` with the right
  owner.
- It creates the `stackmgr-net` Docker network, which the dashboard needs.
- It stamps the current git commit into `.env` and installs a git hook that
  keeps it current after each `git pull`.
- It prints the dashboard URL and the admin login.

All settings live in `.env`. Many runtime settings are copied into the database
on first start and can be changed later under **Settings > General**.
