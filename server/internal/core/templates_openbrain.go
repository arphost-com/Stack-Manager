package core

// OpenBrain templates: opinionated multi-service local-AI stacks. #1-#3 are
// deploy-verified end to end (containers boot cleanly with 0 restarts and the
// service endpoints respond). #4 (voice assistant) is a newer, broader stack —
// see docs/OPENBRAIN4.md for its post-deploy configuration guide. See
// web/src/pages/Documentation.jsx for the setup guides shown in the dashboard.
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
		{
			ID:          "openbrain-voice",
			Name:        "OpenBrain #4 — Voice Assistant (speech, web, code, memory)",
			Description: "The do-everything local AI: talk to it (Whisper STT + Kokoro/Piper TTS), search the web (SearXNG), write code (Code Llama), keep memory (mem0 + Qdrant), and automate tasks (n8n + Flowise) — all wired to Open WebUI and Ollama.",
			Category:    "ai",
			Subcategory: "voice-speech",
			Source:      "arphost",
			Image:       "ghcr.io/open-webui/open-webui:main",
			Tags:        []string{"ai", "voice", "stt", "tts", "whisper", "kokoro", "piper", "ollama", "codellama", "rag", "web-search", "searxng", "memory", "mem0", "n8n", "flowise", "qdrant"},
			ComposeContent: `services:
  ollama:
    image: ollama/ollama:latest
    restart: unless-stopped
    ports:
      - "${OLLAMA_PORT:-11434}:11434"
    volumes:
      - ollama-data:/root/.ollama
    networks: [openbrain]

  # Pulls the models once on first boot (a few GB): a general chat model, a code
  # model, and an embedding model for RAG/memory. Exits when done.
  ollama-init:
    image: ollama/ollama:latest
    restart: "no"
    depends_on:
      - ollama
    environment:
      OLLAMA_HOST: http://ollama:11434
    entrypoint: ["/bin/sh", "-c"]
    command:
      - |
        sleep 8
        ollama pull ${CHAT_MODEL:-llama3.1}
        ollama pull ${CODE_MODEL:-codellama}
        ollama pull ${EMBED_MODEL:-nomic-embed-text}
    networks: [openbrain]

  # Pre-downloads the Whisper model with a CURRENT huggingface client, then exits.
  # The faster-whisper-server image ships an old huggingface_hub that cannot fetch
  # models migrated to HuggingFace's Xet storage (the download fails with an S3
  # "AccessDenied"). This sidecar fetches the model into the shared cache first,
  # so whisper finds it locally and never has to hit Xet with its old client.
  whisper-init:
    image: python:3.12-slim
    restart: "no"
    environment:
      HF_HOME: /root/.cache/huggingface
      WHISPER_MODEL: ${WHISPER_MODEL:-Systran/faster-whisper-small}
    volumes:
      - whisper-cache:/root/.cache
    entrypoint: ["/bin/sh", "-c"]
    command:
      - |
        if find /root/.cache/huggingface/hub -name model.bin -size +1M 2>/dev/null | grep -q .; then
          echo "whisper model already cached; skipping download"; exit 0
        fi
        echo "installing hf client + downloading ${WHISPER_MODEL} ..."
        pip install --no-cache-dir -q "huggingface_hub[hf_xet]"
        hf download "${WHISPER_MODEL}"
        chmod -R a+rX /root/.cache/huggingface
        echo "whisper model cached"
    networks: [openbrain]

  # Speech-to-text: your voice recordings -> text. OpenAI-compatible /v1 API.
  whisper:
    image: fedirz/faster-whisper-server:latest-cpu
    restart: unless-stopped
    depends_on:
      whisper-init:
        condition: service_completed_successfully
    ports:
      - "${WHISPER_PORT:-8001}:8000"
    environment:
      WHISPER__MODEL: ${WHISPER_MODEL:-Systran/faster-whisper-small}
    volumes:
      - whisper-cache:/root/.cache
    networks: [openbrain]

  # Text-to-speech: replies read aloud. OpenAI-compatible /v1/audio/speech.
  kokoro:
    image: ghcr.io/remsky/kokoro-fastapi-cpu:latest
    restart: unless-stopped
    ports:
      - "${KOKORO_PORT:-8880}:8880"
    networks: [openbrain]

  # Voice-cloning TTS (XTTS-v2 + Piper) via openedai-speech. OpenAI-compatible,
  # so it's a drop-in alternative to Kokoro. Keep Kokoro as the fast default;
  # switch Open WebUI's Audio TTS to http://openedai-speech:8000/v1 (model
  # tts-1-hd, your custom voice) when you want a CLONED voice. To clone: drop a
  # clean 10-30s WAV in the openedai-voices volume and add it to
  # config/voice_to_speaker.yaml. GPU strongly recommended for XTTS.
  openedai-speech:
    image: ghcr.io/matatonic/openedai-speech:latest
    restart: unless-stopped
    ports:
      - "${OPENEDAI_PORT:-8003}:8000"
    volumes:
      - openedai-config:/app/config
      - openedai-voices:/app/voices
    networks: [openbrain]

  # Bonus TTS engine (Wyoming protocol) for Home Assistant / n8n voice flows.
  piper:
    image: rhasspy/wyoming-piper:latest
    restart: unless-stopped
    command: --voice ${PIPER_VOICE:-en_US-lessac-medium}
    ports:
      - "${PIPER_PORT:-10200}:10200"
    volumes:
      - piper-data:/data
    networks: [openbrain]

  # Private meta-search used for "search the web" answers.
  searxng:
    image: searxng/searxng:latest
    restart: unless-stopped
    ports:
      - "${SEARXNG_PORT:-8181}:8080"
    environment:
      SEARXNG_BASE_URL: http://localhost:${SEARXNG_PORT:-8181}/
    volumes:
      - searxng-data:/etc/searxng
    networks: [openbrain]

  # Vector store for RAG + memory. mem0 defaults to host "mem0_store".
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

  postgres:
    image: pgvector/pgvector:pg17
    restart: unless-stopped
    environment:
      POSTGRES_DB: openbrain
      POSTGRES_USER: openbrain
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-change-me}
    volumes:
      - postgres-data:/var/lib/postgresql/data
    networks: [openbrain]

  # The hub: chat + voice in/out + web search + RAG, wired to everything above.
  # Create the first admin account on first visit.
  open-webui:
    image: ghcr.io/open-webui/open-webui:main
    restart: unless-stopped
    ports:
      - "${WEBUI_PORT:-8080}:8080"
    environment:
      OLLAMA_BASE_URL: http://ollama:11434
      WEBUI_AUTH: "true"
      WEBUI_SECRET_KEY: ${WEBUI_SECRET_KEY:-change-me-openbrain}
      VECTOR_DB: qdrant
      QDRANT_URI: http://qdrant:6333
      RAG_EMBEDDING_ENGINE: ollama
      RAG_EMBEDDING_MODEL: ${EMBED_MODEL:-nomic-embed-text}
      ENABLE_RAG_WEB_SEARCH: "true"
      RAG_WEB_SEARCH_ENGINE: searxng
      SEARXNG_QUERY_URL: http://searxng:8080/search?q=<query>
      AUDIO_TTS_ENGINE: openai
      AUDIO_TTS_OPENAI_API_BASE_URL: http://kokoro:8880/v1
      AUDIO_TTS_OPENAI_API_KEY: none
      AUDIO_TTS_MODEL: kokoro
      AUDIO_TTS_VOICE: ${TTS_VOICE:-af_bella}
      AUDIO_STT_ENGINE: openai
      AUDIO_STT_OPENAI_API_BASE_URL: http://whisper:8000/v1
      AUDIO_STT_OPENAI_API_KEY: none
      AUDIO_STT_MODEL: ${WHISPER_MODEL:-Systran/faster-whisper-small}
    volumes:
      - openwebui-data:/app/backend/data
    depends_on:
      - ollama
      - qdrant
      - kokoro
      - whisper
      - searxng
    networks: [openbrain]

  # Persistent memory (mem0 / OpenMemory), vectors in Qdrant. For extraction,
  # set OPENAI_API_KEY or switch to local Ollama in its UI settings.
  mem0:
    image: mem0/openmemory-mcp:latest
    restart: unless-stopped
    ports:
      - "${MEM0_PORT:-8765}:8765"
    environment:
      USER: ${MEM0_USER:-admin}
      OPENAI_API_KEY: ${OPENAI_API_KEY:-}
      OLLAMA_HOST: http://ollama:11434
    volumes:
      - mem0-data:/usr/src/openmemory
    depends_on:
      - qdrant
      - ollama
    networks: [openbrain]

  # Visual agent/flow builder.
  flowise:
    image: flowiseai/flowise:latest
    restart: unless-stopped
    ports:
      - "${FLOWISE_PORT:-3001}:3000"
    environment:
      FLOWISE_USERNAME: ${FLOWISE_USERNAME:-admin}
      FLOWISE_PASSWORD: ${FLOWISE_PASSWORD:-change-me}
    volumes:
      - flowise-data:/root/.flowise
    depends_on:
      - ollama
      - qdrant
    networks: [openbrain]

  # Workflow automation for general tasks (persists to the shared Postgres).
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
    networks: [openbrain]

volumes:
  ollama-data:
  openwebui-data:
  qdrant-data:
  postgres-data:
  whisper-cache:
  piper-data:
  searxng-data:
  mem0-data:
  flowise-data:
  n8n-data:
  openedai-config:
  openedai-voices:

networks:
  openbrain:
`,
			EnvContent: "OLLAMA_PORT=11434\nWEBUI_PORT=8080\nWHISPER_PORT=8001\nKOKORO_PORT=8880\nOPENEDAI_PORT=8003\nPIPER_PORT=10200\nSEARXNG_PORT=8181\nQDRANT_PORT=6333\nFLOWISE_PORT=3001\nN8N_PORT=5678\nMEM0_PORT=8765\nPOSTGRES_PASSWORD=change-me\nWEBUI_SECRET_KEY=change-me-openbrain\nFLOWISE_USERNAME=admin\nFLOWISE_PASSWORD=change-me\nCHAT_MODEL=llama3.1\nCODE_MODEL=codellama\nEMBED_MODEL=nomic-embed-text\nWHISPER_MODEL=Systran/faster-whisper-small\nTTS_VOICE=af_bella\nPIPER_VOICE=en_US-lessac-medium\nOPENAI_API_KEY=\nMEM0_USER=admin\nN8N_HOST=localhost\n",
			Notes:      "HEAVY stack — give it plenty of RAM and (ideally) a GPU; enable GPU passthrough on Ollama for real speed. First boot pulls a few GB of models via ollama-init (chat + Code Llama + embeddings), so give it time. Open WebUI (WEBUI_PORT) is the hub: create the admin account, then click the mic to talk (Whisper transcribes) and the speaker to hear replies (Kokoro). Pick 'codellama' from the model menu for coding. VOICE CLONING: openedai-speech (OPENEDAI_PORT) is included as a second TTS — Kokoro stays the fast default; to speak in a CLONED voice, drop a clean 10-30s WAV in the openedai-voices volume, add it to config/voice_to_speaker.yaml, then set Open WebUI Audio TTS to http://openedai-speech:8000/v1 (model tts-1-hd, your voice name). GPU strongly recommended for XTTS. See docs/OPENBRAIN4-RECIPES.md. RAG document upload and memory use Qdrant automatically. ONE manual step for web search: SearXNG ships without JSON output — edit its settings.yml (Project > Config, or the searxng-data volume) to add 'formats: [html, json]' under 'search:' and restart searxng, then toggle Web Search in a chat. mem0/OpenMemory (MEM0_PORT) and Flowise/n8n are pre-wired to Ollama+Qdrant/Postgres; set OPENAI_API_KEY or point mem0's LLM at http://ollama:11434 for memory extraction. Change POSTGRES_PASSWORD/WEBUI_SECRET_KEY/FLOWISE_PASSWORD before exposing it.",
		},
	}
}
