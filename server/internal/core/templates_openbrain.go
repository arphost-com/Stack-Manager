package core

// OpenBrain templates: opinionated multi-service local-AI stacks. All three are
// deploy-verified end to end (containers boot cleanly with 0 restarts and the
// service endpoints respond). See web/src/pages/Documentation.jsx for the full
// setup guides shown in the dashboard.
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
			ComposeContent: `services:
  ollama:
    image: ollama/ollama:latest
    restart: unless-stopped
    ports:
      - "${OLLAMA_PORT:-11434}:11434"
    volumes:
      - ollama-data:/root/.ollama
    networks:
      - openbrain

  open-webui:
    image: ghcr.io/open-webui/open-webui:main
    restart: unless-stopped
    ports:
      - "${WEBUI_PORT:-8080}:8080"
    environment:
      OLLAMA_BASE_URL: http://ollama:11434
      WEBUI_AUTH: "true"
    volumes:
      - openwebui-data:/app/backend/data
    depends_on:
      - ollama
    networks:
      - openbrain

  postgres:
    image: postgres:17
    restart: unless-stopped
    environment:
      POSTGRES_DB: openbrain
      POSTGRES_USER: openbrain
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-change-me}
    volumes:
      - postgres-data:/var/lib/postgresql/data
    networks:
      - openbrain

  neo4j:
    image: neo4j:5-community
    restart: unless-stopped
    ports:
      - "${NEO4J_HTTP_PORT:-7474}:7474"
      - "${NEO4J_BOLT_PORT:-7687}:7687"
    environment:
      NEO4J_AUTH: neo4j/${NEO4J_PASSWORD:-change-me}
      NEO4J_PLUGINS: '["apoc"]'
    volumes:
      - neo4j-data:/data
    networks:
      - openbrain

  temporal:
    image: temporalio/auto-setup:latest
    restart: unless-stopped
    ports:
      - "${TEMPORAL_PORT:-7233}:7233"
    environment:
      DB: postgres12
      DB_PORT: "5432"
      POSTGRES_USER: temporal
      POSTGRES_PWD: ${POSTGRES_PASSWORD:-change-me}
      POSTGRES_SEEDS: temporal-db
    depends_on:
      - temporal-db
    networks:
      - openbrain

  temporal-db:
    image: postgres:17
    restart: unless-stopped
    environment:
      POSTGRES_DB: temporal
      POSTGRES_USER: temporal
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-change-me}
    volumes:
      - temporal-db-data:/var/lib/postgresql/data
    networks:
      - openbrain

  temporal-ui:
    image: temporalio/ui:latest
    restart: unless-stopped
    ports:
      - "${TEMPORAL_UI_PORT:-8233}:8080"
    environment:
      TEMPORAL_ADDRESS: temporal:7233
    depends_on:
      - temporal
    networks:
      - openbrain

volumes:
  ollama-data:
  openwebui-data:
  postgres-data:
  temporal-db-data:
  neo4j-data:

networks:
  openbrain:
`,
			EnvContent: "OLLAMA_PORT=11434\nWEBUI_PORT=8080\nNEO4J_HTTP_PORT=7474\nNEO4J_BOLT_PORT=7687\nTEMPORAL_PORT=7233\nTEMPORAL_UI_PORT=8233\nPOSTGRES_PASSWORD=change-me\nNEO4J_PASSWORD=change-me\n",
			Notes:      "All 7 services verified booting cleanly (Temporal connects to its Postgres, UI on TEMPORAL_UI_PORT, Neo4j graph on 7474/7687, Ollama API on 11434). Change POSTGRES_PASSWORD and NEO4J_PASSWORD before starting. Pull a model after boot: docker exec <ollama-container> ollama pull llama3.1. Open WebUI (WEBUI_PORT) is the chat UI; create the first admin account on first visit.",
		},
		{
			ID:          "openbrain-mem0",
			Name:        "OpenBrain #2 — Agent + Memory + Automation",
			Description: "Ollama + Open WebUI + Qdrant + mem0 (OpenMemory) + Neo4j + n8n for agents with durable vector memory and workflow automation.",
			Category:    "ai",
			Subcategory: "workflow-rag",
			Source:      "arphost",
			Image:       "ollama/ollama:latest",
			Tags:        []string{"ai", "agents", "memory", "automation", "ollama", "mem0", "qdrant", "n8n"},
			ComposeContent: `services:
  ollama:
    image: ollama/ollama:latest
    restart: unless-stopped
    ports:
      - "${OLLAMA_PORT:-11434}:11434"
    volumes:
      - ollama-data:/root/.ollama
    networks:
      - openbrain

  open-webui:
    image: ghcr.io/open-webui/open-webui:main
    restart: unless-stopped
    ports:
      - "${WEBUI_PORT:-8080}:8080"
    environment:
      OLLAMA_BASE_URL: http://ollama:11434
      WEBUI_AUTH: "true"
    volumes:
      - openwebui-data:/app/backend/data
    depends_on:
      - ollama
    networks:
      - openbrain

  postgres:
    image: pgvector/pgvector:pg17
    restart: unless-stopped
    environment:
      POSTGRES_DB: openbrain
      POSTGRES_USER: openbrain
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-change-me}
    volumes:
      - postgres-data:/var/lib/postgresql/data
    networks:
      - openbrain

  # Vector memory store. mem0 (OpenMemory) defaults to reaching Qdrant at the
  # host "mem0_store", so we expose that name as a network alias.
  qdrant:
    image: qdrant/qdrant:latest
    restart: unless-stopped
    ports:
      - "${QDRANT_PORT:-6333}:6333"
    volumes:
      - qdrant-data:/qdrant/storage
    networks:
      openbrain:
        aliases:
          - mem0_store

  neo4j:
    image: neo4j:5-community
    restart: unless-stopped
    ports:
      - "${NEO4J_HTTP_PORT:-7474}:7474"
      - "${NEO4J_BOLT_PORT:-7687}:7687"
    environment:
      NEO4J_AUTH: neo4j/${NEO4J_PASSWORD:-change-me}
      NEO4J_PLUGINS: '["apoc"]'
    volumes:
      - neo4j-data:/data
    networks:
      - openbrain

  # OpenMemory (mem0) durable memory API. Listens on 8765 inside the container.
  # Uses Qdrant (mem0_store) for vectors. Memory extraction needs an LLM: set
  # OPENAI_API_KEY, or switch the LLM/embedder to the local Ollama in the
  # OpenMemory UI (Settings) using base URL http://ollama:11434.
  mem0:
    image: mem0/openmemory-mcp:latest
    restart: unless-stopped
    ports:
      - "${MEM0_PORT:-8000}:8765"
    environment:
      USER: ${MEM0_USER:-admin}
      OPENAI_API_KEY: ${OPENAI_API_KEY:-}
      OLLAMA_HOST: http://ollama:11434
    volumes:
      - mem0-data:/usr/src/openmemory
    depends_on:
      - qdrant
      - ollama
    networks:
      - openbrain

  # Workflow automation. N8N_SECURE_COOKIE=false is required to log in over
  # plain HTTP by IP (the default install). Persists to the shared Postgres.
  n8n:
    image: n8nio/n8n:latest
    restart: unless-stopped
    ports:
      - "${N8N_PORT:-5678}:5678"
    environment:
      N8N_SECURE_COOKIE: "false"
      N8N_HOST: ${N8N_HOST:-localhost}
      N8N_PORT: "5678"
      N8N_PROTOCOL: http
      DB_TYPE: postgresdb
      DB_POSTGRESDB_HOST: postgres
      DB_POSTGRESDB_PORT: "5432"
      DB_POSTGRESDB_DATABASE: openbrain
      DB_POSTGRESDB_USER: openbrain
      DB_POSTGRESDB_PASSWORD: ${POSTGRES_PASSWORD:-change-me}
    volumes:
      - n8n-data:/home/node/.n8n
    depends_on:
      - postgres
    networks:
      - openbrain

volumes:
  ollama-data:
  openwebui-data:
  postgres-data:
  qdrant-data:
  neo4j-data:
  mem0-data:
  n8n-data:

networks:
  openbrain:
`,
			EnvContent: "OLLAMA_PORT=11435\nWEBUI_PORT=8081\nNEO4J_HTTP_PORT=7475\nNEO4J_BOLT_PORT=7688\nQDRANT_PORT=6333\nMEM0_PORT=8000\nN8N_PORT=5678\nPOSTGRES_PASSWORD=change-me\nNEO4J_PASSWORD=change-me\nMEM0_USER=admin\nN8N_HOST=localhost\nOPENAI_API_KEY=\n",
			Notes:      "Verified boot: mem0 (OpenMemory) API on MEM0_PORT, Qdrant vector store on QDRANT_PORT, n8n on N8N_PORT persisting to Postgres. Change all passwords first. mem0 needs an LLM to extract/embed memories: set OPENAI_API_KEY (works out of the box), or switch mem0's LLM+embedder to the bundled Ollama in the OpenMemory Settings UI (base URL http://ollama:11434, embedder nomic-embed-text with 768 dims). n8n login works over plain HTTP by IP because N8N_SECURE_COOKIE is false. Neo4j is included for optional graph memory.",
		},
		{
			ID:          "openbrain-flowise",
			Name:        "OpenBrain #3 — Visual Agent Builder",
			Description: "Ollama + Open WebUI + Flowise + Qdrant + Postgres for building low-code LLM agents and RAG chains visually.",
			Category:    "ai",
			Subcategory: "workflow-rag",
			Source:      "arphost",
			Image:       "flowiseai/flowise:latest",
			Tags:        []string{"ai", "agents", "rag", "low-code", "ollama", "flowise", "qdrant"},
			ComposeContent: `services:
  ollama:
    image: ollama/ollama:latest
    restart: unless-stopped
    ports:
      - "${OLLAMA_PORT:-11434}:11434"
    volumes:
      - ollama-data:/root/.ollama
    networks:
      - openbrain

  open-webui:
    image: ghcr.io/open-webui/open-webui:main
    restart: unless-stopped
    ports:
      - "${WEBUI_PORT:-8080}:8080"
    environment:
      OLLAMA_BASE_URL: http://ollama:11434
      WEBUI_AUTH: "true"
    volumes:
      - openwebui-data:/app/backend/data
    depends_on:
      - ollama
    networks:
      - openbrain

  qdrant:
    image: qdrant/qdrant:latest
    restart: unless-stopped
    ports:
      - "${QDRANT_PORT:-6333}:6333"
    volumes:
      - qdrant-data:/qdrant/storage
    networks:
      - openbrain

  postgres:
    image: postgres:17
    restart: unless-stopped
    environment:
      POSTGRES_DB: flowise
      POSTGRES_USER: flowise
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-change-me}
    volumes:
      - postgres-data:/var/lib/postgresql/data
    networks:
      - openbrain

  # Flowise — low-code visual builder for LLM agents and RAG chains. Connect it
  # to Ollama (http://ollama:11434) and Qdrant (http://qdrant:6333) from inside
  # the flows. Persists to Postgres.
  flowise:
    image: flowiseai/flowise:latest
    restart: unless-stopped
    ports:
      - "${FLOWISE_PORT:-3000}:3000"
    environment:
      FLOWISE_USERNAME: ${FLOWISE_USERNAME:-admin}
      FLOWISE_PASSWORD: ${FLOWISE_PASSWORD:-change-me}
      DATABASE_TYPE: postgres
      DATABASE_HOST: postgres
      DATABASE_PORT: "5432"
      DATABASE_NAME: flowise
      DATABASE_USER: flowise
      DATABASE_PASSWORD: ${POSTGRES_PASSWORD:-change-me}
    volumes:
      - flowise-data:/root/.flowise
    depends_on:
      - postgres
      - ollama
    networks:
      - openbrain

volumes:
  ollama-data:
  openwebui-data:
  qdrant-data:
  postgres-data:
  flowise-data:

networks:
  openbrain:
`,
			EnvContent: "OLLAMA_PORT=11436\nWEBUI_PORT=8082\nQDRANT_PORT=6335\nFLOWISE_PORT=3000\nPOSTGRES_PASSWORD=change-me\nFLOWISE_USERNAME=admin\nFLOWISE_PASSWORD=change-me\n",
			Notes:      "Verified boot: Flowise on FLOWISE_PORT (persists to Postgres), Qdrant on QDRANT_PORT, Ollama on 11434, Open WebUI on WEBUI_PORT. Change POSTGRES_PASSWORD and FLOWISE_PASSWORD first. In Flowise, add an Ollama chat model (base URL http://ollama:11434) and a Qdrant vector store (http://qdrant:6333) inside your chatflows. Pull a model first: docker exec <ollama-container> ollama pull llama3.1.",
		},
	}
}
