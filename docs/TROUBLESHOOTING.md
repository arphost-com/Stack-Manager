# Troubleshooting Stack Manager

Find the symptom below. Most problems come down to the host firewall, the user
the install ran as, or a port another program already owns. For install and
update steps see [Installing Stack Manager](INSTALL.md).

- [Collect the basics first](#collect-the-basics-first)
- [The dashboard](#the-dashboard)
- [A stack you deployed](#a-stack-you-deployed)
- [Nginx Proxy Manager](#nginx-proxy-manager)
- [Updates and schedules](#updates-and-schedules)
- [Firewall messages](#firewall-messages)
- [Agents](#agents)
- [Updating Stack Manager itself](#updating-stack-manager-itself)

## Collect the basics first

Run these in the Stack Manager folder on the host. They change nothing.

```bash
docker compose --env-file .env ps
docker logs --tail 100 stack-manager-server-1
docker logs --tail 50 stack-manager-web-1
grep -E '^(DOCKER_ROOT|SERVER_USER|DOCKER_GID|WEB_SSL_PORT|WEB_HTTP_PORT)=' .env
```

The last line shows where Stack Manager looks for projects, which user it runs
as, and which ports it uses. It prints no secrets.

## The dashboard

### The page does not load from another machine

If `curl -k https://127.0.0.1:8993` works on the host but the browser cannot
connect, the host firewall is blocking 8993. Open 8993 and 8193 in whatever
manages the firewall.

- On a host with a control panel such as CloudPanel, Plesk or cPanel, open the
  ports in the panel. The panel rewrites the firewall and can drop rules added
  from the shell.
- On a plain host, use `ufw allow 8993/tcp` and `ufw allow 8193/tcp`, or the
  equivalent nftables rule.
- With ConfigServer csf, add the ports to `TCP_IN` under
  **Settings > Firewall** once you can reach the dashboard locally.

If curl fails on the host too, check `docker compose ps`. The web container
must be running and healthy.

### The browser warns about the certificate

That is the self-signed certificate created on first start. Accept it once, or
issue a real certificate under **Settings > SSL**. See TLS in the
[Platform Guide](PLATFORM.md#tls--ssl).

### `network stackmgr-net declared as external, but could not be found`

The setup script creates this network. Run `./scripts/prepare-state.sh .env`
again, or create it yourself with `docker network create stackmgr-net`, then
start the stack again.

### `permission denied` on the Docker socket

`DOCKER_GID` in `.env` must match the group that owns the socket. Compare it
with `stat -c '%g' /var/run/docker.sock`. Running
`./scripts/prepare-state.sh .env` sets it for you.

### Stack Manager runs as root and uses /root/docker

The setup script was run with sudo or as root, so `SERVER_USER` is `0:0` and
`DOCKER_ROOT` is `/root/docker`. That works, but it is rarely what you want.
To change it, stop the stack, set `SERVER_USER` and `DOCKER_ROOT` in `.env` to
the user and folder you want, run `./scripts/prepare-state.sh .env` as that
user, and start the stack again. Move existing project folders to the new
`DOCKER_ROOT` while their stacks are stopped.

### `permission denied` writing under .stack-manager

The state folder is owned by a different user than `SERVER_USER`, usually
because the stack once ran as root. Stop the stack, run
`sudo chown "$SERVER_USER" .stack-manager .stack-manager/ssl .stack-manager/logs`
for the folders named in the error, run `./scripts/prepare-state.sh .env`, then
start it again. Do not change the owner of `.stack-manager/mariadb` or
`.stack-manager/redis`, because the database containers own those files, and
never chown `DOCKER_ROOT` recursively.

### The footer shows an old version

The images were not rebuilt. Run
`docker compose --env-file .env up -d --build`. Plain `up -d` keeps the old
images.

### The disk is filling up

Stack Manager keeps local backups under `.stack-manager` and every update can
leave old images behind. Remove unused images with `docker image prune` on the
host and delete backups you no longer need from the **Backups** page. A full
system prune can be scheduled under **Settings > Schedules**, but it also
deletes unused volumes, so only use it when you are sure.

## A stack you deployed

### The containers run but the app does not load in the browser

Open the project's Overview. The Ports column shows the host ports to use.
If the app works on the host with `curl http://127.0.0.1:<port>` but not from
another machine, the host firewall is blocking that port. Open it the same way
as the dashboard ports above. Stack Manager starts containers but only opens
host firewall ports when csf is installed and you click **Open Ports**.

### The project uses host networking

Some templates, such as ARPVPN, Home Assistant and WireGuard tools, use
`network_mode: host`. Docker reports no port mappings for them, so the
Overview shows the ports the project declares instead and marks the container
as host network. These come from the numeric `*_PORT` values in the project's
`.env`, or from the image's own port list when the `.env` sets none. Change the
port in the project's `.env` and restart the project to move it, and open the
new port on the host firewall.

### Port is already allocated

Another container or program is using that host port. Change the port in the
project's `.env` and start the project again. `ss -ltnp | grep :<port>` on the
host shows who has it.

## Nginx Proxy Manager

Deploy it from **Settings > Reverse Proxy**, then connect it there with the
admin URL and login.

### NPM will not start, ports 80 or 443 are in use

NPM needs ports 80 and 443 on the host. A control panel such as CloudPanel,
Plesk or cPanel, or a system nginx or Apache, usually already owns them. Either
let that web server do the proxying instead of NPM, or move it off 80 and 443
first. Two programs cannot share those ports.

### A proxy host shows 502 Bad Gateway

The forward target is wrong. Inside the NPM container `localhost` means NPM
itself, so forward to the host's IP address and the app's host port as shown
in the project Overview. **Add to Proxy** on a project fills these in for you.

### The Let's Encrypt certificate will not issue

The domain's DNS A record must point at this host, and ports 80 and 443 must be
open on the host firewall and reachable from the internet. Check that the
domain resolves to the right address before you request the certificate.

## Updates and schedules

### An update failed with `post-update container verification failed`

After an update Stack Manager watches the containers. A container that
restarts, exits or turns unhealthy fails the update at once. A container whose
healthcheck is still starting is given up to 5 minutes, and if it is still
starting after that the job says how long it waited. Open the project's logs
to see why it did not become healthy.

### A job says `docker compose pull timed out after 5m0s`

The image took longer to download than the timeout allowed. Raise the timeout
on the schedule or the action, or use 0 for no limit on pulls of large images.

### A schedule turned itself off

Its project was deleted or no longer exists. Deleting a project turns off its
schedules, and a schedule whose project is missing records one failed run and
then turns itself off. Re-enable it after you restore the project.

### An update check shows `missing_local`

The project's image is not on the host, usually because the project was never
started or its image was removed. Start or pull the project. If the check also
shows `toomanyrequests`, Docker Hub rate limited the check. Log in to Docker
Hub under **Settings > Registry Logins**, or wait and check again.

## Firewall messages

### `csf is not installed` in the server log

This is normal on hosts that use ufw, nftables or a control panel firewall.
Stack Manager's firewall features only manage ConfigServer csf. Current
versions skip the login allowlist quietly when csf is missing. You only need to
act if you want csf, which you can install from **Settings > Firewall**.

### Containers lost networking after csf was removed

Removing csf flushes Docker's iptables rules. Run `sudo systemctl restart docker`
to rebuild them.

## Agents

### Check-ins return 401

The agent's name and token must match exactly what is registered under
**Settings > Agents** on the dashboard. Compare `AGENT_NAME` and `AGENT_TOKEN`
in the agent's `.env` with the registered values.

### The dashboard cannot reach an inbound agent

The inbound agent listener is plain HTTP on port 8192. Register it as
`http://<agent-host>:8192`, not https, and make sure that port is open from the
dashboard host. If the dashboard cannot reach the agent at all, use callback
mode instead.

### A callback agent never shows up

The agent must be able to reach the dashboard on 8993. Check
`AGENT_CONTROLLER_URL` in the agent's `.env` and test it from the agent host
with `curl -k <that URL>/health`.

## Updating Stack Manager itself

### Update now refuses to run

The host was deployed by the GitLab pipeline and has a
`.stack-manager-ci-deployed` marker. Update it through its pipeline.

### Update now logs a start line and then nothing

The update helper on that host is old. Start the stack once with
`docker compose --env-file .env up -d --build` so the current helpers are
installed, then try again.

### `deploy.sh` fails with `chmod: Operation not permitted`

An earlier update ran as root and left root-owned files. Change the owner of
the Stack Manager folder back to your user, leaving `.stack-manager` alone,
then try again.

### Docker Settings shows `daemon.json is not valid JSON: invalid character 'U'`

This only happens the first time the helper image is pulled. Refresh the page
once the pull has finished.
