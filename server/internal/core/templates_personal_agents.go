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
			ID: "nanobot", Name: "Nanobot",
			Description: "Lightweight open-source AI agent for tools, chats and workflows, with OpenRouter-style provider configuration.",
			Category:    "ai", Subcategory: "personal-agents",
			Source: "github", Image: "ghcr.io/hkuds/nanobot:latest",
			Tags: []string{"ai", "personal-agent", "workflow", "chat"},
			ComposeContent: `services:
  nanobot:
    image: ghcr.io/hkuds/nanobot:latest
    command: ["nanobot", "serve", "--host", "0.0.0.0", "--port", "8080"]
    environment:
      NANOBOT_TOKEN: ${NANOBOT_TOKEN:?set NANOBOT_TOKEN in .env}
      OPENROUTER_API_KEY: ${OPENROUTER_API_KEY:-}
      NANOBOT_MODEL: ${NANOBOT_MODEL:-anthropic/claude-opus-4-6}
    ports:
      - "${NANOBOT_PORT:-18792}:8080"
    volumes:
      - nanobot-data:/data
    restart: unless-stopped
volumes:
  nanobot-data:
`,
			EnvContent: "NANOBOT_PORT=18792\nNANOBOT_TOKEN=\nOPENROUTER_API_KEY=\nNANOBOT_MODEL=anthropic/claude-opus-4-6\n",
			Notes:      "Run the upstream onboarding flow after first start if the image expects local config files instead of environment-only setup.",
		},
		{
			ID: "hermes-agent", Name: "Hermes Agent",
			Description: "Nous Research personal agent focused on learning from completed tasks and turning successful patterns into reusable skills.",
			Category:    "ai", Subcategory: "personal-agents",
			Source: "github", Image: "ghcr.io/nousresearch/hermes-agent:latest",
			Tags: []string{"ai", "personal-agent", "memory", "nous"},
			ComposeContent: `services:
  hermes:
    image: ghcr.io/nousresearch/hermes-agent:latest
    command: ["hermes", "gateway", "--host", "0.0.0.0", "--port", "8080"]
    environment:
      HERMES_LLM_PROVIDER: ${HERMES_PROVIDER:-openai}
      HERMES_LLM_API_KEY: ${HERMES_API_KEY:?set HERMES_API_KEY in .env}
    ports:
      - "${HERMES_PORT:-18793}:8080"
    volumes:
      - hermes-data:/data
    restart: unless-stopped
volumes:
  hermes-data:
`,
			EnvContent: "HERMES_PORT=18793\nHERMES_PROVIDER=openai\nHERMES_API_KEY=\n",
			Notes:      "Good second choice next to OpenClaw for solo-founder workflows where persistent improvement and reusable skills matter.",
		},
		{
			ID: "qwenpaw", Name: "QwenPaw",
			Description: "Qwen and AgentScope personal assistant with local/cloud deployment, built-in memory, sandbox controls, and multi-channel support.",
			Category:    "ai", Subcategory: "personal-agents",
			Source: "github", Image: "ghcr.io/agentscope-ai/qwenpaw:latest",
			Tags: []string{"ai", "personal-agent", "qwen", "multi-channel"},
			ComposeContent: `services:
  qwenpaw:
    image: ghcr.io/agentscope-ai/qwenpaw:latest
    environment:
      QWEN_API_KEY: ${QWEN_API_KEY:-}
      OPENAI_API_KEY: ${OPENAI_API_KEY:-}
      OLLAMA_BASE_URL: ${OLLAMA_BASE_URL:-}
    ports:
      - "${QWENPAW_PORT:-18794}:8080"
    volumes:
      - qwenpaw-data:/data
    restart: unless-stopped
volumes:
  qwenpaw-data:
`,
			EnvContent: "QWENPAW_PORT=18794\nQWEN_API_KEY=\nOPENAI_API_KEY=\nOLLAMA_BASE_URL=\n",
			Notes:      "Strong option for Qwen-first customers or customers who need DingTalk, Lark, WeChat, QQ, Discord, Telegram, and iMessage coverage.",
		},
		{
			ID: "openjarvis", Name: "OpenJarvis",
			Description: "Personal AI for personal devices with persistent autonomous mode and code/task execution options.",
			Category:    "ai", Subcategory: "personal-agents",
			Source: "github", Image: "ghcr.io/open-jarvis/openjarvis:latest",
			Tags: []string{"ai", "personal-agent", "local-first"},
			ComposeContent: `services:
  openjarvis:
    image: ghcr.io/open-jarvis/openjarvis:latest
    environment:
      OPENJARVIS_TOKEN: ${OPENJARVIS_TOKEN:?set OPENJARVIS_TOKEN in .env}
      OPENAI_API_KEY: ${OPENAI_API_KEY:-}
      ANTHROPIC_API_KEY: ${ANTHROPIC_API_KEY:-}
    ports:
      - "${OPENJARVIS_PORT:-18795}:8080"
    volumes:
      - openjarvis-data:/data
    restart: unless-stopped
volumes:
  openjarvis-data:
`,
			EnvContent: "OPENJARVIS_PORT=18795\nOPENJARVIS_TOKEN=\nOPENAI_API_KEY=\nANTHROPIC_API_KEY=\n",
			Notes:      "Pair with Ollama or a local inference stack for customer-owned data flows.",
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
	}
}
