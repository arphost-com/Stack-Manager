# Stack Manager Agent Modes

Stack Manager agents can run on a remote Docker host without MariaDB, Redis, or the web UI. Use the same server binary or `docker-compose.agent.yml`; select behavior with `APP_MODE`. The step by step install is in [Installing Stack Manager](INSTALL.md#install-an-agent-on-another-host).

The Compose commands below are the normal way to run an agent. The direct binary forms need a `stack-manager-server` binary built with `make build-linux` and are mainly useful for development.

## Outbound Only

Use this when the remote host can reach the controller, but the controller cannot reach the remote host because of NAT, ISP filtering, or firewall rules.

```bash
./scripts/prepare-state.sh --agent --mode callback --controller https://docker02:8993 .env
docker compose --env-file .env -f docker-compose.agent.yml up -d --build
```

`prepare-state.sh --agent` fills the agent `.env` completely: pass
`--controller <url>` (or answer the prompt when run in a terminal) so it never
leaves a `change-me` placeholder, and it prints the exact `AGENT_NAME` and
`AGENT_TOKEN` to register on the controller. The name and token must match on
both sides or check-in returns 401. Check-ins go to the controller over HTTPS.

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
./scripts/prepare-state.sh --agent --mode inbound .env
docker compose --env-file .env -f docker-compose.agent.yml up -d --build
```

Register the agent as `Inbound URL` with a URL such as `http://my-client.example.com:8192` and the same token. The inbound listener is plain HTTP, so keep port 8192 reachable only from the controller, for example over a VPN or a firewall rule limited to the controller's address.

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
./scripts/prepare-state.sh --agent --mode both --controller https://docker02:8993 .env
docker compose --env-file .env -f docker-compose.agent.yml up -d --build
```

For ISP-blocked clients, still register this as `Outbound check-in` and leave Agent URL blank.

## Command queue (callback agents)

Callback agents are push-only, so the controller can't drive them live. Instead
it **queues** commands and the agent runs them on its next check-in:

- Opening a callback agent's project on the controller loads it from the last
  check-in snapshot and shows a **Queued commands** panel.
- Lifecycle actions (up / down / pull / update / restart) are enqueued; the
  agent claims them on check-in, runs them via its engine, and reports the
  result back. Status moves `pending → dispatched → done/failed` with output.
- API: `POST /api/v1/agents/{id}/commands` to queue, `GET` to list. The agent
  posts outcomes to `POST /api/v1/agent-checkin/results`.
- Tune responsiveness with `AGENT_CHECKIN_SECONDS` (default 60).

Inbound/both agents and peers are still managed live (no queue needed).
