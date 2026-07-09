package core

func openbrainTemplates() []StackTemplate {
	return []StackTemplate{
		{
			ID:          "openbrain",
			Name:        "OpenBrain #1 — Agent + Workflow",
			Description: "Ollama + Open WebUI + Postgres + Neo4j + Temporal for durable agent workflows with graph knowledge.",
			Category:    "ai",
			Subcategory: "workflow-rag",
			Source:      "arphost",
			Image:       "ollama/ollama:latest",
			Tags:        []string{"ai", "agents", "workflow", "knowledge-graph", "ollama", "temporal", "neo4j"},
			ComposeContent: "services:\n  ollama:\n    image: ollama/ollama:latest\n    restart: unless-stopped\n    ports:\n      - \"${OLLAMA_PORT:-11434}:11434\"\n    volumes:\n      - ollama-data:/root/.ollama\n    networks:\n      - openbrain\n\n  open-webui:\n    image: ghcr.io/open-webui/open-webui:main\n    restart: unless-stopped\n    ports:\n      - \"${WEBUI_PORT:-8080}:8080\"\n    environment:\n      OLLAMA_BASE_URL: http://ollama:11434\n      WEBUI_AUTH: \"true\"\n    volumes:\n      - openwebui-data:/app/backend/data\n    depends_on:\n      - ollama\n    networks:\n      - openbrain\n\n  postgres:\n    image: postgres:17\n    restart: unless-stopped\n    environment:\n      POSTGRES_DB: openbrain\n      POSTGRES_USER: openbrain\n      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-change-me}\n    volumes:\n      - postgres-data:/var/lib/postgresql/data\n    networks:\n      - openbrain\n\n  neo4j:\n    image: neo4j:5-community\n    restart: unless-stopped\n    ports:\n      - \"${NEO4J_HTTP_PORT:-7474}:7474\"\n      - \"${NEO4J_BOLT_PORT:-7687}:7687\"\n    environment:\n      NEO4J_AUTH: neo4j/${NEO4J_PASSWORD:-change-me}\n      NEO4J_PLUGINS: '[\"apoc\"]'\n    volumes:\n      - neo4j-data:/data\n    networks:\n      - openbrain\n\n  temporal:\n    image: temporalio/auto-setup:latest\n    restart: unless-stopped\n    ports:\n      - \"${TEMPORAL_PORT:-7233}:7233\"\n    environment:\n      DB: postgres12\n      DB_PORT: \"5432\"\n      POSTGRES_USER: temporal\n      POSTGRES_PWD: ${POSTGRES_PASSWORD:-change-me}\n      POSTGRES_SEEDS: temporal-db\n    depends_on:\n      - temporal-db\n    networks:\n      - openbrain\n\n  temporal-db:\n    image: postgres:17\n    restart: unless-stopped\n    environment:\n      POSTGRES_DB: temporal\n      POSTGRES_USER: temporal\n      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-change-me}\n    volumes:\n      - temporal-db-data:/var/lib/postgresql/data\n    networks:\n      - openbrain\n\n  temporal-ui:\n    image: temporalio/ui:latest\n    restart: unless-stopped\n    ports:\n      - \"${TEMPORAL_UI_PORT:-8233}:8080\"\n    environment:\n      TEMPORAL_ADDRESS: temporal:7233\n    depends_on:\n      - temporal\n    networks:\n      - openbrain\n\nvolumes:\n  ollama-data:\n  openwebui-data:\n  postgres-data:\n  temporal-db-data:\n  neo4j-data:\n\nnetworks:\n  openbrain:\n",
			EnvContent: "OLLAMA_PORT=11434\nWEBUI_PORT=8080\nNEO4J_HTTP_PORT=7474\nNEO4J_BOLT_PORT=7687\nTEMPORAL_PORT=7233\nTEMPORAL_UI_PORT=8233\nPOSTGRES_PASSWORD=change-me\nNEO4J_PASSWORD=change-me\n",
			Notes:      "Change POSTGRES_PASSWORD and NEO4J_PASSWORD before starting. Pull a model after boot: docker exec <ollama-container> ollama pull llama3.1. Install LangGraph + Graphiti in a Python venv for agent development.",
		},
		{
			ID:          "openbrain-mem0",
			Name:        "OpenBrain #2 — Agent + Memory + Automation",
			Description: "Ollama + Open WebUI + Postgres/pgvector + Mem0 + Neo4j + n8n for agents with durable memory and workflow automation.",
			Category:    "ai",
			Subcategory: "workflow-rag",
			Source:      "arphost",
			Image:       "ollama/ollama:latest",
			Tags:        []string{"ai", "agents", "memory", "automation", "ollama", "mem0", "n8n", "neo4j"},
			ComposeContent: "services:\n  ollama:\n    image: ollama/ollama:latest\n    restart: unless-stopped\n    ports:\n      - \"${OLLAMA_PORT:-11434}:11434\"\n    volumes:\n      - ollama-data:/root/.ollama\n    networks:\n      - openbrain\n\n  open-webui:\n    image: ghcr.io/open-webui/open-webui:main\n    restart: unless-stopped\n    ports:\n      - \"${WEBUI_PORT:-8080}:8080\"\n    environment:\n      OLLAMA_BASE_URL: http://ollama:11434\n      WEBUI_AUTH: \"true\"\n    volumes:\n      - openwebui-data:/app/backend/data\n    depends_on:\n      - ollama\n    networks:\n      - openbrain\n\n  postgres:\n    image: pgvector/pgvector:pg17\n    restart: unless-stopped\n    environment:\n      POSTGRES_DB: openbrain\n      POSTGRES_USER: openbrain\n      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-change-me}\n    volumes:\n      - postgres-data:/var/lib/postgresql/data\n    networks:\n      - openbrain\n\n  neo4j:\n    image: neo4j:5-community\n    restart: unless-stopped\n    ports:\n      - \"${NEO4J_HTTP_PORT:-7474}:7474\"\n      - \"${NEO4J_BOLT_PORT:-7687}:7687\"\n    environment:\n      NEO4J_AUTH: neo4j/${NEO4J_PASSWORD:-change-me}\n      NEO4J_PLUGINS: '[\"apoc\"]'\n    volumes:\n      - neo4j-data:/data\n    networks:\n      - openbrain\n\n  mem0:\n    image: mem0/openmemory-mcp:latest\n    restart: unless-stopped\n    ports:\n      - \"${MEM0_PORT:-8000}:8000\"\n    environment:\n      OPENMEMORY_USER: admin\n      OPENMEMORY_API_KEY: ${MEM0_API_KEY:-change-me}\n      OPENAI_API_KEY: ${OPENAI_API_KEY:-}\n    volumes:\n      - mem0-data:/data\n    depends_on:\n      - postgres\n      - neo4j\n    networks:\n      - openbrain\n\n  n8n:\n    image: n8nio/n8n:latest\n    restart: unless-stopped\n    ports:\n      - \"${N8N_PORT:-5678}:5678\"\n    environment:\n      N8N_BASIC_AUTH_ACTIVE: \"true\"\n      N8N_BASIC_AUTH_USER: admin\n      N8N_BASIC_AUTH_PASSWORD: ${N8N_PASSWORD:-change-me}\n    volumes:\n      - n8n-data:/home/node/.n8n\n    networks:\n      - openbrain\n\nvolumes:\n  ollama-data:\n  openwebui-data:\n  postgres-data:\n  neo4j-data:\n  mem0-data:\n  n8n-data:\n\nnetworks:\n  openbrain:\n",
			EnvContent: "OLLAMA_PORT=11434\nWEBUI_PORT=8080\nNEO4J_HTTP_PORT=7474\nNEO4J_BOLT_PORT=7687\nMEM0_PORT=8000\nN8N_PORT=5678\nPOSTGRES_PASSWORD=change-me\nNEO4J_PASSWORD=change-me\nMEM0_API_KEY=change-me\nN8N_PASSWORD=change-me\nOPENAI_API_KEY=\n",
			Notes:      "Change all passwords before starting. Uses pgvector for vector storage. Pull a model: docker exec <ollama-container> ollama pull llama3.1. Mem0 needs OPENAI_API_KEY or a local model endpoint for memory extraction.",
		},
	}
}
