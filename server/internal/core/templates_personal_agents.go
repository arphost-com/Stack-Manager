package core

// personalAgentStackTemplates: AI personal agents - self-hosted gateways that
// bridge chat apps, tools, memory, and LLM-backed autonomous agents. The list is
// intentionally capped to the best nine deployable options for Stack Manager.
func personalAgentStackTemplates() []StackTemplate {
	return []StackTemplate{
		{
			ID: "openclaw", Name: "OpenClaw",
			Description: "Popular local-first personal AI assistant with a gateway for WhatsApp, Telegram, Slack, Discord, Signal, iMessage and many other channels.",
			Category:    "ai", Subcategory: "personal-agents",
			Source: "npm", Image: "node:24-bookworm-slim",
			Tags: []string{"ai", "personal-agent", "chat-gateway", "openclaw"},
			ComposeContent: `services:
  openclaw:
    image: node:24-bookworm-slim
    working_dir: /root
    command: >-
      sh -c "npm install -g openclaw@${OPENCLAW_VERSION:-latest} &&
             openclaw gateway --host 0.0.0.0 --port 18789 --verbose"
    environment:
      OPENAI_API_KEY: ${OPENAI_API_KEY:-}
      ANTHROPIC_API_KEY: ${ANTHROPIC_API_KEY:-}
    ports:
      - "${OPENCLAW_PORT:-18789}:18789"
    volumes:
      - openclaw-home:/root
    restart: unless-stopped
volumes:
  openclaw-home:
`,
			EnvContent: "OPENCLAW_PORT=18789\nOPENCLAW_VERSION=latest\nOPENAI_API_KEY=\nANTHROPIC_API_KEY=\n",
			Notes:      "Use OpenClaw onboarding and pairing before exposing the gateway. Put it behind HTTPS and keep default pairing/allowlist protections enabled.",
		},
		{
			ID: "zeroclaw", Name: "ZeroClaw",
			Description: "Small, security-first personal agent runtime with supervised autonomy, workspace boundaries, command policy, and OS/container sandbox options.",
			Category:    "ai", Subcategory: "personal-agents",
			Source: "github", Image: "ghcr.io/zeroclaw-labs/zeroclaw:latest",
			Tags: []string{"ai", "personal-agent", "security", "rust"},
			ComposeContent: `services:
  zeroclaw:
    image: ghcr.io/zeroclaw-labs/zeroclaw:latest
    command: ["zeroclaw", "gateway", "--host", "0.0.0.0", "--port", "18791"]
    environment:
      ZEROCLAW_TOKEN: ${ZEROCLAW_TOKEN:?set ZEROCLAW_TOKEN in .env}
      OPENAI_API_KEY: ${OPENAI_API_KEY:-}
      ANTHROPIC_API_KEY: ${ANTHROPIC_API_KEY:-}
      OLLAMA_BASE_URL: ${OLLAMA_BASE_URL:-}
    ports:
      - "${ZEROCLAW_PORT:-18791}:18791"
    volumes:
      - zeroclaw-data:/data
    restart: unless-stopped
volumes:
  zeroclaw-data:
`,
			EnvContent: "ZEROCLAW_PORT=18791\nZEROCLAW_TOKEN=\nOPENAI_API_KEY=\nANTHROPIC_API_KEY=\nOLLAMA_BASE_URL=\n",
			Notes:      "Best fit when the customer needs a safer default posture. Keep supervised mode on until site-specific policies are reviewed.",
		},
		{
			ID: "nanoclaw", Name: "NanoClaw",
			Description: "Lightweight secure OpenClaw-style agent aimed at simple personal and team deployments with isolated execution.",
			Category:    "ai", Subcategory: "personal-agents",
			Source: "npm", Image: "node:24-bookworm-slim",
			Tags: []string{"ai", "personal-agent", "chat-gateway", "sandboxed"},
			ComposeContent: `services:
  nanoclaw:
    image: node:24-bookworm-slim
    working_dir: /root
    command: >-
      sh -c "npm install -g nanoclaw@${NANOCLAW_VERSION:-latest} &&
             nanoclaw gateway --host 0.0.0.0 --port 18790"
    environment:
      NANOCLAW_AUTH_TOKEN: ${NANOCLAW_TOKEN:?set NANOCLAW_TOKEN in .env}
      OPENAI_API_KEY: ${OPENAI_API_KEY:-}
      ANTHROPIC_API_KEY: ${ANTHROPIC_API_KEY:-}
    ports:
      - "${NANOCLAW_PORT:-18790}:18790"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - nanoclaw-home:/root
    restart: unless-stopped
volumes:
  nanoclaw-home:
`,
			EnvContent: "NANOCLAW_PORT=18790\nNANOCLAW_VERSION=latest\nNANOCLAW_TOKEN=\nOPENAI_API_KEY=\nANTHROPIC_API_KEY=\n",
			Notes:      "This stack mounts the Docker socket so NanoClaw can launch isolated workers. Use only on hosts where that trust boundary is acceptable.",
		},
		{
			ID: "moltworker", Name: "Moltworker",
			Description: "Cloudflare-maintained OpenClaw worker/sandbox gateway for customers who want a Cloudflare-edge deployment path.",
			Category:    "ai", Subcategory: "personal-agents",
			Source: "github", Image: "node:24-bookworm-slim",
			Tags: []string{"ai", "personal-agent", "openclaw", "cloudflare"},
			ComposeContent: `services:
  moltworker:
    image: node:24-bookworm-slim
    working_dir: /workspace
    command: >-
      sh -c "npm install -g wrangler &&
             npm create cloudflare@latest moltworker -- --template cloudflare/moltworker --deploy=false &&
             cd moltworker && npm install && npm run dev -- --ip 0.0.0.0 --port 18796"
    environment:
      MOLTBOT_GATEWAY_TOKEN: ${MOLTBOT_GATEWAY_TOKEN:?set MOLTBOT_GATEWAY_TOKEN in .env}
      CLOUDFLARE_ACCOUNT_ID: ${CLOUDFLARE_ACCOUNT_ID:-}
      CLOUDFLARE_API_TOKEN: ${CLOUDFLARE_API_TOKEN:-}
    ports:
      - "${MOLTWORKER_PORT:-18796}:18796"
    volumes:
      - moltworker-workspace:/workspace
    restart: unless-stopped
volumes:
  moltworker-workspace:
`,
			EnvContent: "MOLTWORKER_PORT=18796\nMOLTBOT_GATEWAY_TOKEN=\nCLOUDFLARE_ACCOUNT_ID=\nCLOUDFLARE_API_TOKEN=\n",
			Notes:      "Local dev mode is provided for Stack Manager. For production, deploy to Cloudflare Workers and protect admin/API paths with Cloudflare Access.",
		},
		{
			ID: "letta-agent", Name: "Letta Agent",
			Description: "Stateful agent platform with advanced memory, self-improvement, tools, subagents, and local or self-hosted server options.",
			Category:    "ai", Subcategory: "personal-agents",
			Source: "docker-hub", Image: "letta/letta:latest",
			Tags: []string{"ai", "personal-agent", "memory", "stateful-agent"},
			ComposeContent: `services:
  letta:
    image: letta/letta:latest
    environment:
      LETTA_SERVER_PASSWORD: ${LETTA_SERVER_PASSWORD:?set LETTA_SERVER_PASSWORD in .env}
      LETTA_PG_HOST: postgres
      LETTA_PG_PORT: 5432
      LETTA_PG_DB: letta
      LETTA_PG_USER: letta
      LETTA_PG_PASSWORD: ${LETTA_POSTGRES_PASSWORD:?set LETTA_POSTGRES_PASSWORD in .env}
      OPENAI_API_KEY: ${OPENAI_API_KEY:-}
      ANTHROPIC_API_KEY: ${ANTHROPIC_API_KEY:-}
      OLLAMA_BASE_URL: ${OLLAMA_BASE_URL:-}
    ports:
      - "${LETTA_PORT:-8283}:8283"
    volumes:
      - letta-data:/root/.letta
    depends_on:
      - postgres
    restart: unless-stopped
  postgres:
    image: pgvector/pgvector:pg17
    environment:
      POSTGRES_DB: letta
      POSTGRES_USER: letta
      POSTGRES_PASSWORD: ${LETTA_POSTGRES_PASSWORD:?set LETTA_POSTGRES_PASSWORD in .env}
    volumes:
      - letta-postgres:/var/lib/postgresql/data
    restart: unless-stopped
volumes:
  letta-data:
  letta-postgres:
`,
			EnvContent: "LETTA_PORT=8283\nLETTA_SERVER_PASSWORD=\nLETTA_POSTGRES_PASSWORD=\nOPENAI_API_KEY=\nANTHROPIC_API_KEY=\nOLLAMA_BASE_URL=\n",
			Notes:      "Use Letta when durable memory and self-improving agents matter more than chat-channel breadth. Set one provider key or an Ollama URL before creating agents.",
		},
		{
			ID: "hermes-agent", Name: "Hermes Agent",
			Description: "Autonomous AI agent gateway by NousResearch with OpenAI-compatible API, web dashboard, and multi-channel support.",
			Category:    "ai", Subcategory: "personal-agents",
			Source: "docker-hub", Image: "nousresearch/hermes-agent:latest",
			Tags: []string{"ai", "personal-agent", "gateway", "autonomous"},
			ComposeContent: "services:\n  hermes:\n    image: nousresearch/hermes-agent:latest\n    command: [\"gateway\", \"run\"]\n    restart: unless-stopped\n    ports:\n      - \"${HERMES_API_PORT:-8642}:8642\"\n      - \"${HERMES_DASHBOARD_PORT:-9119}:9119\"\n    environment:\n      HERMES_DASHBOARD: \"1\"\n      HERMES_LLM_API_KEY: ${HERMES_API_KEY:-}\n      ANTHROPIC_API_KEY: ${ANTHROPIC_API_KEY:-}\n    volumes:\n      - hermes-data:/opt/data\nvolumes:\n  hermes-data:\n",
			EnvContent: "HERMES_API_PORT=8642\nHERMES_DASHBOARD_PORT=9119\nHERMES_API_KEY=\nANTHROPIC_API_KEY=\n",
			Notes:      "Set HERMES_API_KEY or ANTHROPIC_API_KEY before starting. API at port 8642, dashboard at port 9119.",
		},
	}
}
