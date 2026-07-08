# Stack Manager Agent Modes

Stack Manager agents can run on a remote Docker host without MariaDB, Redis, or the web UI. Use the same server binary or `docker-compose.agent.yml`; select behavior with `APP_MODE`.

## Outbound Only

Use this when the remote host can reach the controller, but the controller cannot reach the remote host because of NAT, ISP filtering, or firewall rules.

```bash
./scripts/prepare-state.sh --agent .env
sed -i 's#^AGENT_CONTROLLER_URL=.*#AGENT_CONTROLLER_URL=https://docker02:8993#' .env
docker compose --env-file .env -f docker-compose.agent.yml up -d --build
```

Equivalent direct binary form:

```bash
APP_MODE=agent-callback \
AGENT_NAME=my-client \
AGENT_TOKEN='replace-with-the-registered-token' \
AGENT_CONTROLLER_URL=https://docker02:8993 \
ROOT=/home/debian/docker \
./stack-manager-server
```

Register the agent in Settings > Agents as `Outbound check-in`, with the same `AGENT_NAME` and `AGENT_TOKEN`, and leave Agent URL blank.

## Inbound Only

Use this only when the controller can directly reach the remote host.

```bash
APP_MODE=agent \
AGENT_NAME=my-client \
AGENT_TOKEN='replace-with-the-registered-token' \
ROOT=/home/debian/docker \
PORT=8192 \
./stack-manager-server
```

With Compose:

```bash
APP_MODE=agent docker compose --env-file .env -f docker-compose.agent.yml up -d --build
```

Register the agent as `Inbound URL` with a URL such as `https://my-client.example.com:8192` and the same token.

## Both

Use this when you want outbound check-ins and also want the direct `/agent/v1` API available on networks where inbound access works.

```bash
APP_MODE=agent-both \
AGENT_NAME=my-client \
AGENT_TOKEN='replace-with-the-registered-token' \
AGENT_CONTROLLER_URL=https://docker02:8993 \
ROOT=/home/debian/docker \
PORT=8192 \
./stack-manager-server
```

With Compose:

```bash
APP_MODE=agent-both docker compose --env-file .env -f docker-compose.agent.yml up -d --build
```

For ISP-blocked clients, still register this as `Outbound check-in` and leave Agent URL blank. Scheduled remote actions currently require an inbound URL; outbound check-in currently provides inventory/visibility and last-seen status.
