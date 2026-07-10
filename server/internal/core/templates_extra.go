package core

// catalogFillTemplates brings each stack category (and each AI sub-category)
// up to at least 9 built-in templates. Kept in a separate file so the primary
// templates.go stays readable. Every template here should be a working Compose
// starter that the operator can then edit before "Create Project".
func catalogFillTemplates() []StackTemplate {
	return []StackTemplate{
		// ---- AI: code-assistants ----
		{
			ID: "tabby", Name: "Tabby", Description: "Self-hosted AI coding assistant server.",
			Category: "ai", Subcategory: "code-assistants",
			Source: "docker-hub", Image: "tabbyml/tabby:latest",
			Tags: []string{"ai", "code-assistant", "gpu"},
			ComposeContent: `services:
  tabby:
    image: tabbyml/tabby:latest
    command: ["serve", "--model", "${TABBY_MODEL:-StarCoder-1B}", "--device", "${TABBY_DEVICE:-cpu}"]
    ports:
      - "${TABBY_PORT:-8083}:8080"
    volumes:
      - tabby-data:/data
    restart: unless-stopped
volumes:
  tabby-data:
`,
			EnvContent: "TABBY_PORT=8083\nTABBY_MODEL=StarCoder-1B\nTABBY_DEVICE=cpu\n",
			Notes:      "Change TABBY_DEVICE to cuda on hosts with an NVIDIA GPU (also add --gpus all to compose).",
		},
		{
			ID: "refact", Name: "Refact Self-Hosted", Description: "Open-source coding assistant with fine-tuning and RAG.",
			Category: "ai", Subcategory: "code-assistants",
			Source: "docker-hub", Image: "smallcloud/refact_self_hosting:latest",
			Tags: []string{"ai", "code-assistant"},
			ComposeContent: `services:
  refact:
    image: smallcloud/refact_self_hosting:latest
    ports:
      - "${REFACT_PORT:-8008}:8008"
    volumes:
      - refact-perm:/perm_storage
    restart: unless-stopped
volumes:
  refact-perm:
`,
			EnvContent: "REFACT_PORT=8008\n",
			Notes:      "First-run creates an admin account through the web UI.",
		},
		{
			ID: "openhands", Name: "OpenHands (formerly OpenDevin)", Description: "Autonomous coding agent that plans and executes tasks.",
			Category: "ai", Subcategory: "code-assistants",
			Source: "docker-hub", Image: "ghcr.io/openhands/openhands:0.61",
			Tags: []string{"ai", "code-assistant", "agent"},
			ComposeContent: `services:
  openhands:
    image: ghcr.io/openhands/openhands:0.61
    environment:
      SANDBOX_RUNTIME_CONTAINER_IMAGE: ghcr.io/openhands/runtime:0.61-nikolaik
      LLM_API_KEY: ${LLM_API_KEY:-}
      LLM_MODEL: ${LLM_MODEL:-anthropic/claude-3-5-sonnet-latest}
    ports:
      - "${OPENHANDS_PORT:-3000}:3000"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - openhands-state:/.openhands-state
    restart: unless-stopped
volumes:
  openhands-state:
`,
			EnvContent: "OPENHANDS_PORT=3000\nLLM_API_KEY=\nLLM_MODEL=anthropic/claude-3-5-sonnet-latest\n",
			Notes:      "Provide LLM_API_KEY for the model provider. The agent spawns sandbox containers through the mounted Docker socket.",
		},
		{
			ID: "aider", Name: "Aider CLI Container", Description: "Terminal-based AI pair programmer packaged as a long-lived shell container.",
			Category: "ai", Subcategory: "code-assistants",
			Source: "docker-hub", Image: "paulgauthier/aider:latest",
			Tags: []string{"ai", "code-assistant", "cli"},
			ComposeContent: `services:
  aider:
    image: paulgauthier/aider:latest
    stdin_open: true
    tty: true
    environment:
      OPENAI_API_KEY: ${OPENAI_API_KEY:-}
      ANTHROPIC_API_KEY: ${ANTHROPIC_API_KEY:-}
    volumes:
      - ./workspace:/app
    working_dir: /app
    command: sleep infinity
    restart: unless-stopped
`,
			EnvContent: "OPENAI_API_KEY=\nANTHROPIC_API_KEY=\n",
			Notes:      "Attach with 'docker compose exec aider aider' once the container is up. Place your git repo under ./workspace.",
		},
		{
			ID: "codellama-ollama", Name: "Ollama + Code Llama", Description: "Ollama preloaded with a Code Llama model for local IDE integrations.",
			Category: "ai", Subcategory: "code-assistants",
			Source: "docker-hub", Image: "ollama/ollama:latest",
			Tags: []string{"ai", "code-assistant", "ollama"},
			ComposeContent: `services:
  ollama:
    image: ollama/ollama:latest
    ports:
      - "${OLLAMA_PORT:-11434}:11434"
    volumes:
      - ollama-models:/root/.ollama
    restart: unless-stopped
  bootstrap:
    image: ollama/ollama:latest
    depends_on:
      - ollama
    entrypoint: ["sh", "-c", "sleep 5 && ollama pull ${OLLAMA_CODE_MODEL:-codellama:7b-instruct}"]
    environment:
      OLLAMA_HOST: http://ollama:11434
volumes:
  ollama-models:
`,
			EnvContent: "OLLAMA_PORT=11434\nOLLAMA_CODE_MODEL=codellama:7b-instruct\n",
			Notes:      "Point Continue, Twinny, or another OpenAI-API client at http://<host>:11434.",
		},
		{
			ID: "sglang-code", Name: "SGLang Code Server", Description: "SGLang inference server for code models (OpenAI-compatible API).",
			Category: "ai", Subcategory: "code-assistants",
			Source: "docker-hub", Image: "lmsysorg/sglang:latest",
			Tags: []string{"ai", "code-assistant", "inference"},
			ComposeContent: `services:
  sglang:
    image: lmsysorg/sglang:latest
    command: >-
      python -m sglang.launch_server
      --model-path ${SGLANG_MODEL:-Qwen/Qwen2.5-Coder-7B-Instruct}
      --host 0.0.0.0 --port 30000
    ports:
      - "${SGLANG_PORT:-30000}:30000"
    volumes:
      - sglang-cache:/root/.cache/huggingface
    restart: unless-stopped
volumes:
  sglang-cache:
`,
			EnvContent: "SGLANG_PORT=30000\nSGLANG_MODEL=Qwen/Qwen2.5-Coder-7B-Instruct\n",
			Notes:      "Requires a GPU; add 'deploy.resources.reservations.devices' with nvidia driver for CUDA hosts.",
		},
		{
			ID: "code-server-ai", Name: "code-server + Ollama", Description: "VS Code in the browser with a local Ollama sidecar for AI extensions.",
			Category: "ai", Subcategory: "code-assistants",
			Source: "docker-hub", Image: "linuxserver/code-server:latest",
			Tags: []string{"ai", "code-assistant", "ide"},
			ComposeContent: `services:
  code:
    image: linuxserver/code-server:latest
    environment:
      PUID: ${PUID:-1000}
      PGID: ${PGID:-1000}
      PASSWORD: ${CODE_PASSWORD:?set CODE_PASSWORD in .env}
    ports:
      - "${CODE_PORT:-8443}:8443"
    volumes:
      - ./config:/config
      - ./workspace:/config/workspace
    restart: unless-stopped
  ollama:
    image: ollama/ollama:latest
    volumes:
      - ollama-models:/root/.ollama
    restart: unless-stopped
volumes:
  ollama-models:
`,
			EnvContent: "PUID=1000\nPGID=1000\nCODE_PASSWORD=change-me-please\nCODE_PORT=8443\n",
			Notes:      "Install Continue or Twinny in code-server and point it at http://ollama:11434.",
		},

		// ---- AI: image-generation (need +7) ----
		{
			ID: "invokeai", Name: "InvokeAI", Description: "Stable Diffusion creative studio with node graph and gallery UI.",
			Category: "ai", Subcategory: "image-generation",
			Source: "docker-hub", Image: "ghcr.io/invoke-ai/invokeai:latest",
			Tags: []string{"ai", "image", "stable-diffusion"},
			ComposeContent: `services:
  invokeai:
    image: ghcr.io/invoke-ai/invokeai:latest
    ports:
      - "${INVOKEAI_PORT:-9090}:9090"
    volumes:
      - invokeai-data:/invokeai
    restart: unless-stopped
volumes:
  invokeai-data:
`,
			EnvContent: "INVOKEAI_PORT=9090\n",
			Notes:      "GPU strongly recommended. Add 'deploy.resources.reservations.devices' with nvidia driver for CUDA hosts.",
		},
		{
			ID: "fooocus", Name: "Fooocus", Description: "Streamlined Stable Diffusion UI focused on ease of use.",
			Category: "ai", Subcategory: "image-generation",
			Source: "docker-hub", Image: "ghcr.io/lllyasviel/fooocus:latest",
			Tags: []string{"ai", "image", "stable-diffusion"},
			ComposeContent: `services:
  fooocus:
    image: ghcr.io/lllyasviel/fooocus:latest
    ports:
      - "${FOOOCUS_PORT:-7865}:7865"
    volumes:
      - fooocus-models:/app/models
      - fooocus-outputs:/app/outputs
    restart: unless-stopped
volumes:
  fooocus-models:
  fooocus-outputs:
`,
			EnvContent: "FOOOCUS_PORT=7865\n",
			Notes:      "First run downloads several GB of model weights.",
		},
		{
			ID: "sd-webui-forge", Name: "SD WebUI Forge", Description: "AUTOMATIC1111 fork optimized for speed and memory.",
			Category: "ai", Subcategory: "image-generation",
			Source: "docker-hub", Image: "nykk3/stable-diffusion-webui-forge:latest",
			Tags: []string{"ai", "image", "stable-diffusion"},
			ComposeContent: `services:
  forge:
    image: nykk3/stable-diffusion-webui-forge:latest
    ports:
      - "${FORGE_PORT:-7861}:7861"
    volumes:
      - forge-data:/app/data
    restart: unless-stopped
volumes:
  forge-data:
`,
			EnvContent: "FORGE_PORT=7861\n",
			Notes:      "Bring your own SD checkpoints under the mounted data volume.",
		},
		{
			ID: "rembg", Name: "rembg (background removal)", Description: "Web/API server for removing image backgrounds with u2net.",
			Category: "ai", Subcategory: "image-generation",
			Source: "docker-hub", Image: "danielgatis/rembg:latest",
			Tags: []string{"ai", "image", "utility"},
			ComposeContent: `services:
  rembg:
    image: danielgatis/rembg:latest
    command: ["s"]
    ports:
      - "${REMBG_PORT:-7000}:7000"
    restart: unless-stopped
`,
			EnvContent: "REMBG_PORT=7000\n",
			Notes:      "Endpoint at /api/remove — POST an image to receive a PNG with the background removed.",
		},

		// ---- AI: voice-speech (need +8) ----
		{
			ID: "piper-tts", Name: "Piper TTS (Wyoming)", Description: "Fast local text-to-speech, exposed over the Wyoming protocol.",
			Category: "ai", Subcategory: "voice-speech",
			Source: "docker-hub", Image: "rhasspy/wyoming-piper:latest",
			Tags: []string{"ai", "tts", "voice"},
			ComposeContent: `services:
  piper:
    image: rhasspy/wyoming-piper:latest
    command: ["--voice", "${PIPER_VOICE:-en_US-lessac-medium}"]
    ports:
      - "${PIPER_PORT:-10200}:10200"
    volumes:
      - piper-data:/data
    restart: unless-stopped
volumes:
  piper-data:
`,
			EnvContent: "PIPER_PORT=10200\nPIPER_VOICE=en_US-lessac-medium\n",
			Notes:      "Integrates with Home Assistant, Rhasspy, and other Wyoming clients.",
		},
		{
			ID: "coqui-tts", Name: "Coqui TTS", Description: "Coqui TTS server with a REST API for many voices.",
			Category: "ai", Subcategory: "voice-speech",
			Source: "docker-hub", Image: "ghcr.io/coqui-ai/tts:latest",
			Tags: []string{"ai", "tts", "voice"},
			ComposeContent: `services:
  coqui:
    image: ghcr.io/coqui-ai/tts:latest
    command: ["python3", "TTS/server/server.py", "--model_name", "${TTS_MODEL:-tts_models/en/vctk/vits}"]
    ports:
      - "${TTS_PORT:-5002}:5002"
    volumes:
      - coqui-models:/root/.local/share/tts
    restart: unless-stopped
volumes:
  coqui-models:
`,
			EnvContent: "TTS_PORT=5002\nTTS_MODEL=tts_models/en/vctk/vits\n",
			Notes:      "First run downloads the requested model. GPU is optional but improves latency.",
		},
		{
			ID: "whisper-cpp", Name: "whisper.cpp Server", Description: "C++ inference server for OpenAI Whisper models.",
			Category: "ai", Subcategory: "voice-speech",
			Source: "docker-hub", Image: "ghcr.io/ggml-org/whisper.cpp:main",
			Tags: []string{"ai", "asr", "voice"},
			ComposeContent: `services:
  whisper:
    image: ghcr.io/ggml-org/whisper.cpp:main
    command: ["whisper-server -l ${WHISPER_LANG:-en} -m /models/${WHISPER_MODEL:-ggml-base.en.bin} --host 0.0.0.0 --port 8080"]
    ports:
      - "${WHISPER_PORT:-8081}:8080"
    volumes:
      - whisper-models:/models
    restart: unless-stopped
volumes:
  whisper-models:
`,
			EnvContent: "WHISPER_PORT=8081\nWHISPER_LANG=en\nWHISPER_MODEL=ggml-base.en.bin\n",
			Notes:      "Download the requested ggml model into the whisper-models volume before first request. Image moved to the ggml-org namespace; the server binary is launched via the bash -c entrypoint.",
		},
		{
			ID: "faster-whisper", Name: "faster-whisper Server", Description: "GPU-optimized Whisper reimplementation with an OpenAI-compatible API.",
			Category: "ai", Subcategory: "voice-speech",
			Source: "docker-hub", Image: "fedirz/faster-whisper-server:latest-cpu",
			Tags: []string{"ai", "asr", "voice"},
			ComposeContent: `services:
  fasterwhisper:
    image: fedirz/faster-whisper-server:latest-cpu
    environment:
      WHISPER_MODEL: ${WHISPER_MODEL:-Systran/faster-whisper-medium}
      COMPUTE_TYPE: ${COMPUTE_TYPE:-int8}
    ports:
      - "${FASTER_WHISPER_PORT:-8082}:8000"
    volumes:
      - fw-cache:/root/.cache
    restart: unless-stopped
volumes:
  fw-cache:
`,
			EnvContent: "FASTER_WHISPER_PORT=8082\nWHISPER_MODEL=Systran/faster-whisper-medium\nCOMPUTE_TYPE=int8\n",
			Notes:      "Swap COMPUTE_TYPE to float16 and the image tag to latest-cuda on GPU hosts. Upstream renamed the project to speaches; this is the final faster-whisper-server build.",
		},
		{
			ID: "xtts-server", Name: "XTTS v2 Server", Description: "Coqui XTTS-v2 streaming voice cloning server.",
			Category: "ai", Subcategory: "voice-speech",
			Source: "docker-hub", Image: "erew123/alltalk_tts:latest",
			Tags: []string{"ai", "tts", "voice-clone"},
			ComposeContent: `services:
  xtts:
    image: erew123/alltalk_tts:latest
    ports:
      - "${XTTS_PORT:-7851}:7851"
    volumes:
      - xtts-models:/app/models
    restart: unless-stopped
volumes:
  xtts-models:
`,
			EnvContent: "XTTS_PORT=7851\n",
			Notes:      "First run downloads XTTS weights; keep the volume around for reuse.",
		},

		{
			ID: "voice-assistant", Name: "Voice Assistant (Ollama + Open WebUI + Kokoro TTS)",
			Description: "Talk to a local LLM by voice. Open WebUI is pre-wired to Kokoro for text-to-speech and its built-in Whisper for speech-to-text — voice works on first login, no manual audio setup.",
			Category:    "ai", Subcategory: "voice-speech",
			Source: "arphost", Image: "ghcr.io/open-webui/open-webui:main",
			Tags: []string{"ai", "voice", "tts", "stt", "ollama", "open-webui", "kokoro"},
			ComposeContent: `services:
  ollama:
    image: ollama/ollama:latest
    restart: unless-stopped
    ports:
      - "${OLLAMA_PORT:-11434}:11434"
    volumes:
      - ollama-data:/root/.ollama
    networks:
      - voice

  # OpenAI-compatible TTS with the voice model bundled in the image (no manual
  # weight download). Open WebUI calls it at /v1/audio/speech.
  kokoro:
    image: ghcr.io/remsky/kokoro-fastapi-cpu:latest
    restart: unless-stopped
    ports:
      - "${KOKORO_PORT:-8880}:8880"
    networks:
      - voice

  open-webui:
    image: ghcr.io/open-webui/open-webui:main
    restart: unless-stopped
    ports:
      - "${WEBUI_PORT:-8080}:8080"
    environment:
      OLLAMA_BASE_URL: http://ollama:11434
      WEBUI_AUTH: "true"
      # Fully-integrated voice, applied at startup (no Admin > Audio setup):
      #  - TTS  -> local Kokoro (OpenAI-compatible)
      #  - STT  -> Open WebUI's built-in local Whisper (default engine)
      AUDIO_TTS_ENGINE: openai
      AUDIO_TTS_OPENAI_API_BASE_URL: http://kokoro:8880/v1
      AUDIO_TTS_OPENAI_API_KEY: ${TTS_API_KEY:-sk-local-no-auth}
      AUDIO_TTS_MODEL: kokoro
      AUDIO_TTS_VOICE: ${TTS_VOICE:-af_sky}
    volumes:
      - openwebui-data:/app/backend/data
    depends_on:
      - ollama
      - kokoro
    networks:
      - voice

volumes:
  ollama-data:
  openwebui-data:

networks:
  voice:
`,
			EnvContent: "OLLAMA_PORT=11437\nWEBUI_PORT=8083\nKOKORO_PORT=8880\nTTS_VOICE=af_sky\nTTS_API_KEY=sk-local-no-auth\n",
			Notes:      "Deploy-verified: all 3 services boot clean, Kokoro synthesizes speech, and Open WebUI reaches it. After boot: create the admin account on first visit, then pull a model (docker exec <ollama-container> ollama pull llama3.1). Click the headphone/call icon in a chat to talk. TTS is already set to Kokoro; change the voice via TTS_VOICE (af_sky, af_bella, am_adam, bf_emma, ...). STT uses Open WebUI's built-in Whisper (downloads a small model on first mic use). Everything stays editable in Admin > Settings > Audio.",
		},

		{
			ID: "rhasspy", Name: "Rhasspy Voice Assistant", Description: "Fully offline private voice assistant with web UI, wake word, speech-to-text, intent recognition, and text-to-speech.",
			Category: "ai", Subcategory: "voice-speech",
			Source: "docker-hub", Image: "rhasspy/rhasspy:latest",
			Tags:           []string{"ai", "voice", "assistant", "offline", "stt", "tts"},
			ComposeContent: "services:\n  rhasspy:\n    image: rhasspy/rhasspy:latest\n    restart: unless-stopped\n    ports:\n      - \"${RHASSPY_PORT:-12101}:12101\"\n    volumes:\n      - rhasspy-profiles:/profiles\n    devices:\n      - /dev/snd:/dev/snd\n    command: [\"--user-profiles\", \"/profiles\", \"--profile\", \"en\"]\nvolumes:\n  rhasspy-profiles:\n",
			EnvContent:     "RHASSPY_PORT=12101\n",
			Notes:          "Web UI at port 12101. The /dev/snd device mount is for microphone access — remove it if running headless (HTTP API only). Change --profile to your language (en, de, fr, etc.).",
		},

		// ---- AI: vector-db (need +6) ----
		{
			ID: "milvus", Name: "Milvus Standalone", Description: "High-performance open-source vector database (standalone mode).",
			Category: "ai", Subcategory: "vector-db",
			Source: "docker-hub", Image: "milvusdb/milvus:latest",
			Tags: []string{"ai", "vector-db"},
			ComposeContent: `services:
  etcd:
    image: quay.io/coreos/etcd:v3.5.5
    command: etcd -advertise-client-urls=http://etcd:2379 -listen-client-urls=http://0.0.0.0:2379 --data-dir /etcd
    volumes:
      - etcd-data:/etcd
  minio:
    image: minio/minio:latest
    command: minio server /data
    environment:
      MINIO_ROOT_USER: ${MINIO_USER:-minioadmin}
      MINIO_ROOT_PASSWORD: ${MINIO_PASSWORD:-minioadmin}
    volumes:
      - minio-data:/data
  milvus:
    image: milvusdb/milvus:latest
    command: milvus run standalone
    environment:
      ETCD_ENDPOINTS: etcd:2379
      MINIO_ADDRESS: minio:9000
    ports:
      - "${MILVUS_PORT:-19530}:19530"
    depends_on:
      - etcd
      - minio
    volumes:
      - milvus-data:/var/lib/milvus
    restart: unless-stopped
volumes:
  etcd-data:
  minio-data:
  milvus-data:
`,
			EnvContent: "MILVUS_PORT=19530\nMINIO_USER=minioadmin\nMINIO_PASSWORD=minioadmin\n",
			Notes:      "Change the MinIO credentials before starting.",
		},
		{
			ID: "pgvector", Name: "Postgres + pgvector", Description: "PostgreSQL with the pgvector extension for vector similarity search.",
			Category: "ai", Subcategory: "vector-db",
			Source: "docker-hub", Image: "pgvector/pgvector:pg16",
			Tags: []string{"ai", "vector-db", "postgres"},
			ComposeContent: `services:
  pg:
    image: pgvector/pgvector:pg16
    environment:
      POSTGRES_USER: ${PG_USER:-vectors}
      POSTGRES_PASSWORD: ${PG_PASSWORD:?set PG_PASSWORD in .env}
      POSTGRES_DB: ${PG_DB:-vectors}
    ports:
      - "${PG_PORT:-5433}:5432"
    volumes:
      - pgvector-data:/var/lib/postgresql/data
    restart: unless-stopped
volumes:
  pgvector-data:
`,
			EnvContent: "PG_USER=vectors\nPG_PASSWORD=change-me\nPG_DB=vectors\nPG_PORT=5433\n",
			Notes:      "Run 'CREATE EXTENSION vector;' inside the database on first connect.",
		},
		{
			ID: "vespa", Name: "Vespa", Description: "Yahoo/Verizon Media's search + vector engine.",
			Category: "ai", Subcategory: "vector-db",
			Source: "docker-hub", Image: "vespaengine/vespa:latest",
			Tags: []string{"ai", "vector-db", "search"},
			ComposeContent: `services:
  vespa:
    image: vespaengine/vespa:latest
    ports:
      - "${VESPA_PORT:-8085}:8080"
      - "${VESPA_ADMIN:-19071}:19071"
    volumes:
      - vespa-data:/opt/vespa/var
    restart: unless-stopped
volumes:
  vespa-data:
`,
			EnvContent: "VESPA_PORT=8085\nVESPA_ADMIN=19071\n",
			Notes:      "Deploy an application package with the Vespa CLI once the container is healthy.",
		},
		{
			ID: "marqo", Name: "Marqo", Description: "End-to-end tensor search engine (vector + BM25).",
			Category: "ai", Subcategory: "vector-db",
			Source: "docker-hub", Image: "marqoai/marqo:latest",
			Tags: []string{"ai", "vector-db"},
			ComposeContent: `services:
  marqo:
    image: marqoai/marqo:latest
    ports:
      - "${MARQO_PORT:-8882}:8882"
    volumes:
      - marqo-data:/opt/vespa/var
    restart: unless-stopped
volumes:
  marqo-data:
`,
			EnvContent: "MARQO_PORT=8882\n",
			Notes:      "GPU acceleration is supported on CUDA hosts with 'deploy.resources.reservations.devices'.",
		},
		{
			ID: "redis-stack", Name: "Redis Stack", Description: "Redis with RediSearch, RedisJSON, and vector search modules.",
			Category: "ai", Subcategory: "vector-db",
			Source: "docker-hub", Image: "redis/redis-stack:latest",
			Tags: []string{"ai", "vector-db", "redis"},
			ComposeContent: `services:
  redis:
    image: redis/redis-stack:latest
    environment:
      REDIS_ARGS: --requirepass ${REDIS_PASSWORD:?set REDIS_PASSWORD in .env}
    ports:
      - "${REDIS_PORT:-6380}:6379"
      - "${REDIS_INSIGHT_PORT:-8001}:8001"
    volumes:
      - redis-stack-data:/data
    restart: unless-stopped
volumes:
  redis-stack-data:
`,
			EnvContent: "REDIS_PASSWORD=change-me\nREDIS_PORT=6380\nREDIS_INSIGHT_PORT=8001\n",
			Notes:      "RedisInsight web UI is on the insight port.",
		},
		{
			ID: "opensearch-vectors", Name: "OpenSearch (vector-ready)", Description: "OpenSearch single-node with the k-NN plugin for vector search.",
			Category: "ai", Subcategory: "vector-db",
			Source: "docker-hub", Image: "opensearchproject/opensearch:latest",
			Tags: []string{"ai", "vector-db", "search"},
			ComposeContent: `services:
  opensearch:
    image: opensearchproject/opensearch:latest
    environment:
      discovery.type: single-node
      OPENSEARCH_INITIAL_ADMIN_PASSWORD: ${OPENSEARCH_PASSWORD:?set OPENSEARCH_PASSWORD in .env}
    ports:
      - "${OPENSEARCH_PORT:-9200}:9200"
    volumes:
      - opensearch-data:/usr/share/opensearch/data
    ulimits:
      memlock: {soft: -1, hard: -1}
      nofile:  {soft: 65536, hard: 65536}
    restart: unless-stopped
volumes:
  opensearch-data:
`,
			EnvContent: "OPENSEARCH_PORT=9200\nOPENSEARCH_PASSWORD=change-me-2Chars!\n",
			Notes:      "Password must meet OpenSearch complexity rules (upper, lower, digit, symbol).",
		},

		// ---- AI: workflow-rag (need +5) ----
		{
			ID: "ragflow", Name: "RAGFlow", Description: "Enterprise RAG platform with document parsing pipelines.",
			Category: "ai", Subcategory: "workflow-rag",
			Source: "docker-hub", Image: "infiniflow/ragflow:latest",
			Tags: []string{"ai", "rag"},
			ComposeContent: `services:
  ragflow:
    image: infiniflow/ragflow:latest
    ports:
      - "${RAGFLOW_PORT:-9380}:9380"
    volumes:
      - ragflow-data:/ragflow
    restart: unless-stopped
volumes:
  ragflow-data:
`,
			EnvContent: "RAGFLOW_PORT=9380\n",
			Notes:      "Provide external ES/Redis endpoints in the RAGFlow config for production use.",
		},
		{
			ID: "haystack", Name: "Haystack REST API", Description: "deepset Haystack pipeline server with an OpenAPI interface.",
			Category: "ai", Subcategory: "workflow-rag",
			Source: "docker-hub", Image: "deepset/haystack:cpu",
			Tags: []string{"ai", "rag", "pipeline"},
			ComposeContent: `services:
  haystack:
    image: deepset/haystack:cpu
    command: ["gunicorn", "-b", "0.0.0.0:8000", "rest_api.application:app", "--workers", "1", "--timeout", "180"]
    ports:
      - "${HAYSTACK_PORT:-8000}:8000"
    volumes:
      - haystack-data:/data
    restart: unless-stopped
volumes:
  haystack-data:
`,
			EnvContent: "HAYSTACK_PORT=8000\n",
			Notes:      "Swap image tag to gpu for CUDA hosts.",
		},
		{
			ID: "dify", Name: "Dify", Description: "LLM app development platform (studio + orchestration).",
			Category: "ai", Subcategory: "workflow-rag",
			Source: "docker-hub", Image: "langgenius/dify-web:latest",
			Tags: []string{"ai", "workflow", "rag"},
			ComposeContent: `services:
  dify-api:
    image: langgenius/dify-api:latest
    environment:
      MODE: api
      SECRET_KEY: ${DIFY_SECRET:?set DIFY_SECRET in .env}
    volumes:
      - dify-data:/app/api/storage
    restart: unless-stopped
  dify-web:
    image: langgenius/dify-web:latest
    ports:
      - "${DIFY_PORT:-3010}:3000"
    depends_on:
      - dify-api
    restart: unless-stopped
volumes:
  dify-data:
`,
			EnvContent: "DIFY_PORT=3010\nDIFY_SECRET=change-me-32-chars-minimum\n",
			Notes:      "Dify normally ships with its own compose bundle (postgres/redis); add those services to persist state.",
		},
		{
			ID: "verba", Name: "Verba (Weaviate RAG)", Description: "Weaviate's RAG chatbot with a polished UI.",
			Category: "ai", Subcategory: "workflow-rag",
			Source: "docker-hub", Image: "semitechnologies/verba:latest",
			Tags: []string{"ai", "rag", "chatbot"},
			ComposeContent: `services:
  verba:
    image: semitechnologies/verba:latest
    environment:
      OPENAI_API_KEY: ${OPENAI_API_KEY:-}
      WEAVIATE_URL: ${WEAVIATE_URL:-http://weaviate:8080}
    ports:
      - "${VERBA_PORT:-8000}:8000"
    restart: unless-stopped
`,
			EnvContent: "VERBA_PORT=8000\nOPENAI_API_KEY=\nWEAVIATE_URL=http://weaviate:8080\n",
			Notes:      "Point WEAVIATE_URL at your existing Weaviate template.",
		},

		// ---- AI: search (need +8) ----
		{
			ID: "meilisearch", Name: "Meilisearch", Description: "Lightning-fast typo-tolerant search engine.",
			Category: "ai", Subcategory: "search",
			Source: "docker-hub", Image: "getmeili/meilisearch:latest",
			Tags: []string{"search"},
			ComposeContent: `services:
  meili:
    image: getmeili/meilisearch:latest
    environment:
      MEILI_MASTER_KEY: ${MEILI_KEY:?set MEILI_KEY in .env}
    ports:
      - "${MEILI_PORT:-7700}:7700"
    volumes:
      - meili-data:/meili_data
    restart: unless-stopped
volumes:
  meili-data:
`,
			EnvContent: "MEILI_PORT=7700\nMEILI_KEY=change-me-32-chars-minimum\n",
			Notes:      "The master key gates all API access; rotate carefully.",
		},
		{
			ID: "typesense", Name: "Typesense", Description: "Open-source, faceted search server with typo tolerance.",
			Category: "ai", Subcategory: "search",
			Source: "docker-hub", Image: "typesense/typesense:30.1",
			Tags: []string{"search"},
			ComposeContent: `services:
  typesense:
    image: typesense/typesense:30.1
    command: >-
      --data-dir /data
      --api-key=${TYPESENSE_KEY:?set TYPESENSE_KEY in .env}
      --enable-cors
    ports:
      - "${TYPESENSE_PORT:-8108}:8108"
    volumes:
      - typesense-data:/data
    restart: unless-stopped
volumes:
  typesense-data:
`,
			EnvContent: "TYPESENSE_PORT=8108\nTYPESENSE_KEY=change-me-32-chars-minimum\n",
			Notes:      "API key protects both admin and search endpoints; use collection-scoped keys in clients.",
		},
		{
			ID: "elasticsearch", Name: "Elasticsearch (single-node)", Description: "Elasticsearch single-node cluster for development or small workloads.",
			Category: "ai", Subcategory: "search",
			Source: "docker-hub", Image: "elasticsearch:8.14.3",
			Tags: []string{"search"},
			ComposeContent: `services:
  elastic:
    image: elasticsearch:8.14.3
    environment:
      discovery.type: single-node
      xpack.security.enabled: "false"
      ES_JAVA_OPTS: -Xms1g -Xmx1g
    ports:
      - "${ELASTIC_PORT:-9201}:9200"
    volumes:
      - elastic-data:/usr/share/elasticsearch/data
    ulimits:
      memlock: {soft: -1, hard: -1}
    restart: unless-stopped
volumes:
  elastic-data:
`,
			EnvContent: "ELASTIC_PORT=9201\n",
			Notes:      "Set xpack.security.enabled: true and configure auth before exposing this outside a trusted network.",
		},
		{
			ID: "opensearch", Name: "OpenSearch (single-node)", Description: "OpenSearch single-node cluster for full-text and vector search.",
			Category: "ai", Subcategory: "search",
			Source: "docker-hub", Image: "opensearchproject/opensearch:latest",
			Tags: []string{"search"},
			ComposeContent: `services:
  opensearch:
    image: opensearchproject/opensearch:latest
    environment:
      discovery.type: single-node
      OPENSEARCH_INITIAL_ADMIN_PASSWORD: ${OPENSEARCH_PASSWORD:?set OPENSEARCH_PASSWORD in .env}
    ports:
      - "${OPENSEARCH_PORT:-9202}:9200"
    volumes:
      - opensearch-search-data:/usr/share/opensearch/data
    restart: unless-stopped
volumes:
  opensearch-search-data:
`,
			EnvContent: "OPENSEARCH_PORT=9202\nOPENSEARCH_PASSWORD=change-me-2Chars!\n",
			Notes:      "Admin password must satisfy OpenSearch complexity rules.",
		},
		{
			ID: "zincsearch", Name: "ZincSearch", Description: "Lightweight Elasticsearch alternative in a single binary.",
			Category: "ai", Subcategory: "search",
			Source: "docker-hub", Image: "public.ecr.aws/zinclabs/zincsearch:latest",
			Tags: []string{"search"},
			ComposeContent: `services:
  zinc:
    image: public.ecr.aws/zinclabs/zincsearch:latest
    environment:
      ZINC_FIRST_ADMIN_USER: ${ZINC_USER:-admin}
      ZINC_FIRST_ADMIN_PASSWORD: ${ZINC_PASSWORD:?set ZINC_PASSWORD in .env}
    ports:
      - "${ZINC_PORT:-4080}:4080"
    volumes:
      - zinc-data:/data
    restart: unless-stopped
volumes:
  zinc-data:
`,
			EnvContent: "ZINC_PORT=4080\nZINC_USER=admin\nZINC_PASSWORD=change-me\n",
			Notes:      "Web UI at http://<host>:4080.",
		},
		{
			ID: "sonic", Name: "Sonic", Description: "Fast, lightweight schema-less search backend.",
			Category: "ai", Subcategory: "search",
			Source: "docker-hub", Image: "valeriansaliou/sonic:latest",
			Tags: []string{"search"},
			ComposeContent: `services:
  sonic:
    image: valeriansaliou/sonic:latest
    ports:
      - "${SONIC_PORT:-1491}:1491"
    volumes:
      - sonic-store:/var/lib/sonic/store
      - ./config.cfg:/etc/sonic.cfg:ro
    restart: unless-stopped
volumes:
  sonic-store:
`,
			EnvContent: "SONIC_PORT=1491\n",
			Notes:      "Create config.cfg beside compose.yml with the auth_password before starting.",
		},
		{
			ID: "manticore", Name: "Manticore Search", Description: "SphinxSearch fork with full-text, vector, and SQL support.",
			Category: "ai", Subcategory: "search",
			Source: "docker-hub", Image: "manticoresearch/manticore:latest",
			Tags: []string{"search"},
			ComposeContent: `services:
  manticore:
    image: manticoresearch/manticore:latest
    ports:
      - "${MANTICORE_PORT:-9308}:9308"
      - "${MANTICORE_SQL:-9306}:9306"
    volumes:
      - manticore-data:/var/lib/manticore
    restart: unless-stopped
volumes:
  manticore-data:
`,
			EnvContent: "MANTICORE_PORT=9308\nMANTICORE_SQL=9306\n",
			Notes:      "HTTP API on 9308; MySQL wire protocol on 9306.",
		},
		{
			ID: "perplexica", Name: "Perplexica", Description: "Open-source alternative to Perplexity AI (SearXNG + LLM).",
			Category: "ai", Subcategory: "search",
			Source: "docker-hub", Image: "itzcrazykns1337/perplexica:latest",
			Tags: []string{"ai", "search"},
			ComposeContent: `services:
  perplexica:
    image: itzcrazykns1337/perplexica:latest
    environment:
      SEARXNG_API_URL: ${SEARXNG_URL:-http://searxng:8080}
      OPENAI_API_KEY: ${OPENAI_API_KEY:-}
    ports:
      - "${PERPLEXICA_PORT:-3005}:3000"
    restart: unless-stopped
`,
			EnvContent: "PERPLEXICA_PORT=3005\nSEARXNG_URL=http://searxng:8080\nOPENAI_API_KEY=\n",
			Notes:      "Pair with the SearXNG template on the same network.",
		},

		// ---- Non-AI: automation +3 ----
		{
			ID: "huginn", Name: "Huginn", Description: "Self-hosted scenario builder for scraping and automation.",
			Category: "automation",
			Source:   "docker-hub", Image: "huginn/huginn",
			Tags: []string{"automation", "scraping"},
			ComposeContent: `services:
  huginn:
    image: huginn/huginn
    # The all-in-one huginn/huginn image bundles its own MySQL. Do not set
    # HUGINN_DATABASE_ADAPTER=sqlite3 -- huginn only supports mysql2/postgresql
    # and that override crash-loops the container.
    ports:
      - "${HUGINN_PORT:-3010}:3000"
    volumes:
      - huginn-data:/var/lib/mysql
    restart: unless-stopped
volumes:
  huginn-data:
`,
			EnvContent: "HUGINN_PORT=3010\n",
			Notes:      "Swap to MySQL/Postgres for production instead of the bundled sqlite.",
		},
		{
			ID: "gotify", Name: "Gotify", Description: "Self-hosted push notifications server.",
			Category: "automation",
			Source:   "docker-hub", Image: "gotify/server:latest",
			Tags: []string{"automation", "notifications"},
			ComposeContent: `services:
  gotify:
    image: gotify/server:latest
    ports:
      - "${GOTIFY_PORT:-8083}:80"
    volumes:
      - gotify-data:/app/data
    restart: unless-stopped
volumes:
  gotify-data:
`,
			EnvContent: "GOTIFY_PORT=8083\n",
			Notes:      "Default login is admin/admin — change immediately after first launch.",
		},
		{
			ID: "cronicle", Name: "Cronicle", Description: "Multi-server cron scheduler with a web UI.",
			Category: "automation",
			Source:   "docker-hub", Image: "ghcr.io/cronicle-edge/cronicle-edge:latest",
			Tags: []string{"automation", "cron"},
			ComposeContent: `services:
  cronicle:
    image: ghcr.io/cronicle-edge/cronicle-edge:latest
    # The image ships no default command (just tini), so it must be told to run
    # the foreground manager, or it prints tini help and exits.
    command: ["/opt/cronicle/bin/manager"]
    environment:
      CRONICLE_secret_key: ${CRONICLE_SECRET:-change-me-to-a-random-secret}
      CRONICLE_foreground: "1"
      CRONICLE_echo: "1"
    ports:
      - "${CRONICLE_PORT:-3012}:3012"
    volumes:
      - cronicle-data:/opt/cronicle/data
      - cronicle-logs:/opt/cronicle/logs
    restart: unless-stopped
volumes:
  cronicle-data:
  cronicle-logs:
`,
			EnvContent: "CRONICLE_PORT=3012\nCRONICLE_SECRET=change-me-to-a-random-secret\n",
			Notes:      "Boots the foreground manager and the web UI on CRONICLE_PORT. Change CRONICLE_SECRET to a random value before real use. Default admin login is admin / admin.",
		},

		// ---- Non-AI: cms +3 ----
		{
			ID: "strapi", Name: "Strapi Headless CMS", Description: "Open-source headless CMS with an admin panel.",
			Category: "cms",
			Source:   "docker-hub", Image: "node:24-bookworm-slim",
			Tags: []string{"cms", "headless"},
			ComposeContent: `services:
  strapi:
    image: node:24-bookworm-slim
    environment:
      DATABASE_CLIENT: sqlite
      DATABASE_FILENAME: /srv/app/data/db.sqlite
    ports:
      - "${STRAPI_PORT:-1337}:1337"
    volumes:
      - strapi-data:/srv/app
    restart: unless-stopped
volumes:
  strapi-data:
`,
			EnvContent: "STRAPI_PORT=1337\n",
			Notes:      "Use Postgres for production; sqlite is fine for dev only.",
		},
		{
			ID: "prestashop", Name: "PrestaShop", Description: "Open-source e-commerce platform.",
			Category: "cms",
			Source:   "docker-hub", Image: "prestashop/prestashop:latest",
			Tags: []string{"cms", "ecommerce"},
			ComposeContent: `services:
  prestashop:
    image: prestashop/prestashop:latest
    environment:
      DB_SERVER: db
      DB_USER: ${PS_DB_USER:-prestashop}
      DB_PASSWD: ${PS_DB_PASSWORD:?set PS_DB_PASSWORD in .env}
      DB_NAME: ${PS_DB_NAME:-prestashop}
    ports:
      - "${PS_PORT:-8086}:80"
    depends_on:
      - db
    restart: unless-stopped
  db:
    image: mariadb:11.4
    environment:
      MARIADB_ROOT_PASSWORD: ${PS_DB_ROOT:?set PS_DB_ROOT in .env}
      MARIADB_DATABASE: ${PS_DB_NAME:-prestashop}
      MARIADB_USER: ${PS_DB_USER:-prestashop}
      MARIADB_PASSWORD: ${PS_DB_PASSWORD:?set PS_DB_PASSWORD in .env}
    volumes:
      - prestashop-db:/var/lib/mysql
volumes:
  prestashop-db:
`,
			EnvContent: "PS_PORT=8086\nPS_DB_USER=prestashop\nPS_DB_NAME=prestashop\nPS_DB_PASSWORD=change-me\nPS_DB_ROOT=change-me-root\n",
			Notes:      "PrestaShop's installer runs on first web request.",
		},

		// ---- Non-AI: database +3 ----
		{
			ID: "mariadb", Name: "MariaDB", Description: "MariaDB 11.4 with persistent volume.",
			Category: "database",
			Source:   "docker-hub-official", Image: "mariadb:11.4",
			Tags: []string{"database", "sql", "mysql"},
			ComposeContent: `services:
  mariadb:
    image: mariadb:11.4
    environment:
      MARIADB_ROOT_PASSWORD: ${MARIADB_ROOT_PASSWORD:?set MARIADB_ROOT_PASSWORD in .env}
      MARIADB_DATABASE: ${MARIADB_DATABASE:-app}
      MARIADB_USER: ${MARIADB_USER:-app}
      MARIADB_PASSWORD: ${MARIADB_PASSWORD:?set MARIADB_PASSWORD in .env}
    ports:
      - "${MARIADB_PORT:-3307}:3306"
    volumes:
      - mariadb-data:/var/lib/mysql
    restart: unless-stopped
volumes:
  mariadb-data:
`,
			EnvContent: "MARIADB_PORT=3307\nMARIADB_DATABASE=app\nMARIADB_USER=app\nMARIADB_PASSWORD=change-me\nMARIADB_ROOT_PASSWORD=change-me-root\n",
			Notes:      "Wire another compose project at mariadb:3306 by joining the same network.",
		},
		{
			ID: "cockroachdb", Name: "CockroachDB (single-node)", Description: "Distributed SQL database in single-node insecure mode.",
			Category: "database",
			Source:   "docker-hub-official", Image: "cockroachdb/cockroach:latest",
			Tags: []string{"database", "sql", "distributed"},
			ComposeContent: `services:
  cockroach:
    image: cockroachdb/cockroach:latest
    command: start-single-node --insecure --advertise-addr=cockroach
    ports:
      - "${COCKROACH_SQL:-26257}:26257"
      - "${COCKROACH_UI:-8087}:8080"
    volumes:
      - cockroach-data:/cockroach/cockroach-data
    restart: unless-stopped
volumes:
  cockroach-data:
`,
			EnvContent: "COCKROACH_SQL=26257\nCOCKROACH_UI=8087\n",
			Notes:      "Do not use --insecure in production; enable TLS via 'cockroach cert' first.",
		},
		{
			ID: "neo4j", Name: "Neo4j Community", Description: "Graph database with the Neo4j Browser UI.",
			Category: "database",
			Source:   "docker-hub-official", Image: "neo4j:5",
			Tags: []string{"database", "graph"},
			ComposeContent: `services:
  neo4j:
    image: neo4j:5
    environment:
      NEO4J_AUTH: neo4j/${NEO4J_PASSWORD:?set NEO4J_PASSWORD (min 8 chars) in .env}
    ports:
      - "${NEO4J_HTTP:-7474}:7474"
      - "${NEO4J_BOLT:-7687}:7687"
    volumes:
      - neo4j-data:/data
    restart: unless-stopped
volumes:
  neo4j-data:
`,
			EnvContent: "NEO4J_HTTP=7474\nNEO4J_BOLT=7687\nNEO4J_PASSWORD=change-me-8chars\n",
			Notes:      "Neo4j rejects passwords shorter than 8 characters.",
		},

		// ---- Non-AI: devtools +3 ----
		{
			ID: "drawio", Name: "draw.io (diagrams.net)", Description: "Self-hosted diagram editor.",
			Category: "devtools",
			Source:   "docker-hub", Image: "jgraph/drawio:latest",
			Tags: []string{"devtools", "diagrams"},
			ComposeContent: `services:
  drawio:
    image: jgraph/drawio:latest
    ports:
      - "${DRAWIO_PORT:-8088}:8080"
    restart: unless-stopped
`,
			EnvContent: "DRAWIO_PORT=8088\n",
			Notes:      "Runs entirely client-side once loaded; no server storage.",
		},
		{
			ID: "woodpecker", Name: "Woodpecker CI Server", Description: "Lightweight open-source CI/CD server.",
			Category: "devtools",
			Source:   "docker-hub", Image: "woodpeckerci/woodpecker-server:latest",
			Tags: []string{"devtools", "ci"},
			ComposeContent: `services:
  woodpecker:
    image: woodpeckerci/woodpecker-server:latest
    environment:
      WOODPECKER_OPEN: "true"
      WOODPECKER_HOST: ${WOODPECKER_HOST:-http://localhost:8089}
      WOODPECKER_AGENT_SECRET: ${WOODPECKER_SECRET:?set WOODPECKER_SECRET in .env}
      WOODPECKER_GITEA: "true"
      WOODPECKER_GITEA_URL: ${GITEA_URL:-}
      WOODPECKER_GITEA_CLIENT: ${GITEA_CLIENT:-}
      WOODPECKER_GITEA_SECRET: ${GITEA_SECRET:-}
    ports:
      - "${WOODPECKER_PORT:-8089}:8000"
    volumes:
      - woodpecker-data:/var/lib/woodpecker
    restart: unless-stopped
volumes:
  woodpecker-data:
`,
			EnvContent: "WOODPECKER_PORT=8089\nWOODPECKER_HOST=http://localhost:8089\nWOODPECKER_SECRET=change-me-32-chars\nGITEA_URL=\nGITEA_CLIENT=\nGITEA_SECRET=\n",
			Notes:      "Also run at least one woodpecker-agent instance to actually execute jobs.",
		},
		{
			ID: "drone", Name: "Drone CI Server", Description: "Container-native CI/CD platform.",
			Category: "devtools",
			Source:   "docker-hub", Image: "drone/drone:2",
			Tags: []string{"devtools", "ci"},
			ComposeContent: `services:
  drone:
    image: drone/drone:2
    environment:
      DRONE_SERVER_HOST: ${DRONE_HOST:-drone.local}
      DRONE_SERVER_PROTO: ${DRONE_PROTO:-http}
      DRONE_RPC_SECRET: ${DRONE_SECRET:-change-me-to-a-32-char-secret}
      DRONE_GITEA_SERVER: ${GITEA_URL:-}
      DRONE_GITEA_CLIENT_ID: ${GITEA_CLIENT:-}
      DRONE_GITEA_CLIENT_SECRET: ${GITEA_SECRET:-}
    ports:
      - "${DRONE_PORT:-8090}:80"
    volumes:
      - drone-data:/data
    restart: unless-stopped
volumes:
  drone-data:
`,
			EnvContent: "DRONE_PORT=8090\nDRONE_HOST=drone.local\nDRONE_PROTO=http\nDRONE_SECRET=change-me-32-chars-minimum\nGITEA_URL=\nGITEA_CLIENT=\nGITEA_SECRET=\n",
			Notes:      "Pair with drone/drone-runner-docker to run jobs.",
		},

		// ---- Non-AI: docs +3 ----
		{
			ID: "outline", Name: "Outline Wiki", Description: "Team knowledge base with markdown editor.",
			Category: "docs",
			Source:   "docker-hub", Image: "outlinewiki/outline:latest",
			Tags: []string{"docs", "wiki"},
			ComposeContent: `services:
  outline:
    image: outlinewiki/outline:latest
    environment:
      SECRET_KEY: ${OUTLINE_SECRET:?set OUTLINE_SECRET (32+ chars hex) in .env}
      UTILS_SECRET: ${OUTLINE_UTILS_SECRET:?set OUTLINE_UTILS_SECRET (32+ chars hex) in .env}
      URL: ${OUTLINE_URL:-http://localhost:3020}
    ports:
      - "${OUTLINE_PORT:-3020}:3000"
    volumes:
      - outline-data:/var/lib/outline/data
    restart: unless-stopped
volumes:
  outline-data:
`,
			EnvContent: "OUTLINE_PORT=3020\nOUTLINE_URL=http://localhost:3020\nOUTLINE_SECRET=change-me-32-chars-hex\nOUTLINE_UTILS_SECRET=change-me-32-chars-hex\n",
			Notes:      "Outline needs Postgres, Redis, and an SSO provider; add those services for production.",
		},
		{
			ID: "docmost", Name: "Docmost", Description: "Open-source collaborative wiki and documentation platform.",
			Category: "docs",
			Source:   "docker-hub", Image: "docmost/docmost:latest",
			Tags: []string{"docs", "wiki"},
			ComposeContent: `services:
  docmost:
    image: docmost/docmost:latest
    environment:
      APP_SECRET: ${DOCMOST_SECRET:?set DOCMOST_SECRET (32+ chars) in .env}
      DATABASE_URL: postgres://docmost:${DOCMOST_DB_PASSWORD:?set DOCMOST_DB_PASSWORD in .env}@db:5432/docmost
    ports:
      - "${DOCMOST_PORT:-3030}:3000"
    depends_on:
      - db
    restart: unless-stopped
  db:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: docmost
      POSTGRES_PASSWORD: ${DOCMOST_DB_PASSWORD:?set DOCMOST_DB_PASSWORD in .env}
      POSTGRES_DB: docmost
    volumes:
      - docmost-db:/var/lib/postgresql/data
volumes:
  docmost-db:
`,
			EnvContent: "DOCMOST_PORT=3030\nDOCMOST_SECRET=change-me-32-chars-minimum\nDOCMOST_DB_PASSWORD=change-me\n",
			Notes:      "Enable SMTP for signup emails via Docmost's environment vars.",
		},
		{
			ID: "mkdocs-material", Name: "MkDocs Material", Description: "Static site generator for technical documentation.",
			Category: "docs",
			Source:   "docker-hub", Image: "squidfunk/mkdocs-material:latest",
			Tags: []string{"docs", "static"},
			ComposeContent: `services:
  mkdocs:
    image: squidfunk/mkdocs-material:latest
    # Seed a starter mkdocs.yml + docs/index.md on first run if the mounted
    # folder is empty, then serve. Without this, "mkdocs serve" crash-loops on
    # a fresh deploy with "Config file 'mkdocs.yml' does not exist". Everything
    # is written to ./docs on the host, so edit it there afterward.
    entrypoint: ["sh", "-c"]
    command:
      - |
        [ -f mkdocs.yml ] || printf 'site_name: My Docs\ntheme:\n  name: material\n' > mkdocs.yml
        [ -f docs/index.md ] || { mkdir -p docs; printf '# Welcome\n\nEdit mkdocs.yml and the files under docs/ to build your site.\n' > docs/index.md; }
        exec mkdocs serve --dev-addr=0.0.0.0:8000
    ports:
      - "${MKDOCS_PORT:-8000}:8000"
    volumes:
      - ./docs:/docs
    restart: unless-stopped
`,
			EnvContent: "MKDOCS_PORT=8000\n",
			Notes:      "Boots with a starter mkdocs.yml + docs/index.md seeded into ./docs on first run (only if missing). Edit those files on the host; live reload picks up changes.",
		},

		// ---- Non-AI: files +3 ----
		{
			ID: "seafile", Name: "Seafile", Description: "Self-hosted file sync + sharing (community edition).",
			Category: "files",
			Source:   "docker-hub", Image: "seafileltd/seafile-mc:latest",
			Tags: []string{"files", "sync"},
			ComposeContent: `services:
  seafile:
    image: seafileltd/seafile-mc:latest
    environment:
      DB_HOST: db
      DB_ROOT_PASSWD: ${SEAFILE_DB_ROOT:?set SEAFILE_DB_ROOT in .env}
      TIME_ZONE: ${TZ:-UTC}
      SEAFILE_ADMIN_EMAIL: ${SEAFILE_ADMIN_EMAIL:-admin@example.com}
      SEAFILE_ADMIN_PASSWORD: ${SEAFILE_ADMIN_PASSWORD:?set SEAFILE_ADMIN_PASSWORD in .env}
    ports:
      - "${SEAFILE_PORT:-8091}:80"
    volumes:
      - seafile-data:/shared
    depends_on:
      - db
    restart: unless-stopped
  db:
    image: mariadb:11.4
    environment:
      MARIADB_ROOT_PASSWORD: ${SEAFILE_DB_ROOT:?set SEAFILE_DB_ROOT in .env}
    volumes:
      - seafile-db:/var/lib/mysql
volumes:
  seafile-data:
  seafile-db:
`,
			EnvContent: "SEAFILE_PORT=8091\nSEAFILE_DB_ROOT=change-me\nSEAFILE_ADMIN_EMAIL=admin@example.com\nSEAFILE_ADMIN_PASSWORD=change-me\nTZ=UTC\n",
			Notes:      "Also mount a Memcached container for production performance.",
		},
		{
			ID: "owncloud", Name: "ownCloud", Description: "Enterprise-flavored file collaboration server.",
			Category: "files",
			Source:   "docker-hub", Image: "owncloud/server:latest",
			Tags: []string{"files", "sync"},
			ComposeContent: `services:
  owncloud:
    image: owncloud/server:latest
    environment:
      OWNCLOUD_DOMAIN: ${OWNCLOUD_DOMAIN:-localhost:8092}
      OWNCLOUD_DB_TYPE: sqlite
      OWNCLOUD_ADMIN_USERNAME: ${OWNCLOUD_ADMIN:-admin}
      OWNCLOUD_ADMIN_PASSWORD: ${OWNCLOUD_ADMIN_PASSWORD:?set OWNCLOUD_ADMIN_PASSWORD in .env}
    ports:
      - "${OWNCLOUD_PORT:-8092}:8080"
    volumes:
      - owncloud-data:/mnt/data
    restart: unless-stopped
volumes:
  owncloud-data:
`,
			EnvContent: "OWNCLOUD_PORT=8092\nOWNCLOUD_DOMAIN=localhost:8092\nOWNCLOUD_ADMIN=admin\nOWNCLOUD_ADMIN_PASSWORD=change-me\n",
			Notes:      "Use MariaDB + Redis for anything larger than a home lab.",
		},
		{
			ID: "pydio-cells", Name: "Pydio Cells", Description: "Modern file sharing and collaboration platform.",
			Category: "files",
			Source:   "docker-hub", Image: "pydio/cells:latest",
			Tags: []string{"files", "sync"},
			ComposeContent: `services:
  cells:
    image: pydio/cells:latest
    environment:
      CELLS_BIND: 0.0.0.0:8093
      CELLS_EXTERNAL: ${CELLS_EXTERNAL:-http://localhost:8093}
    ports:
      - "${CELLS_PORT:-8093}:8093"
    volumes:
      - cells-data:/var/cells
    restart: unless-stopped
volumes:
  cells-data:
`,
			EnvContent: "CELLS_PORT=8093\nCELLS_EXTERNAL=http://localhost:8093\n",
			Notes:      "First-run setup wizard prompts for admin credentials and DB.",
		},

		// ---- Non-AI: management +3 ----
		{
			ID: "yacht", Name: "Yacht", Description: "Web UI for managing Docker containers with app templates.",
			Category: "management",
			Source:   "docker-hub", Image: "selfhostedpro/yacht:latest",
			Tags: []string{"management", "docker"},
			ComposeContent: `services:
  yacht:
    image: selfhostedpro/yacht:latest
    ports:
      - "${YACHT_PORT:-8094}:8000"
    volumes:
      - yacht-data:/config
      - /var/run/docker.sock:/var/run/docker.sock
    restart: unless-stopped
volumes:
  yacht-data:
`,
			EnvContent: "YACHT_PORT=8094\n",
			Notes:      "Default admin/pass=admin@yacht.local / pass. Change immediately.",
		},
		{
			ID: "rundeck", Name: "Rundeck", Description: "Job scheduler and runbook automation.",
			Category: "management",
			Source:   "docker-hub", Image: "rundeck/rundeck:6.0.0",
			Tags: []string{"management", "automation"},
			ComposeContent: `services:
  rundeck:
    image: rundeck/rundeck:6.0.0
    environment:
      RUNDECK_GRAILS_URL: ${RUNDECK_URL:-http://localhost:8095}
    ports:
      - "${RUNDECK_PORT:-8095}:4440"
    volumes:
      - rundeck-data:/home/rundeck/server/data
    restart: unless-stopped
volumes:
  rundeck-data:
`,
			EnvContent: "RUNDECK_PORT=8095\nRUNDECK_URL=http://localhost:8095\n",
			Notes:      "Default login is admin / admin — change immediately.",
		},
		{
			ID: "semaphore-ui", Name: "Semaphore UI", Description: "Web UI for running Ansible playbooks, Terraform, and shell scripts.",
			Category: "management",
			Source:   "docker-hub", Image: "semaphoreui/semaphore:latest",
			Tags: []string{"management", "ansible"},
			ComposeContent: `services:
  semaphore:
    image: semaphoreui/semaphore:latest
    environment:
      SEMAPHORE_DB_DIALECT: bolt
      SEMAPHORE_ADMIN: ${SEMAPHORE_ADMIN:-admin}
      SEMAPHORE_ADMIN_EMAIL: ${SEMAPHORE_EMAIL:-admin@example.com}
      SEMAPHORE_ADMIN_NAME: ${SEMAPHORE_NAME:-Admin}
      SEMAPHORE_ADMIN_PASSWORD: ${SEMAPHORE_PASSWORD:?set SEMAPHORE_PASSWORD in .env}
    ports:
      - "${SEMAPHORE_PORT:-3096}:3000"
    volumes:
      - semaphore-data:/etc/semaphore
    restart: unless-stopped
volumes:
  semaphore-data:
`,
			EnvContent: "SEMAPHORE_PORT=3096\nSEMAPHORE_ADMIN=admin\nSEMAPHORE_EMAIL=admin@example.com\nSEMAPHORE_NAME=Admin\nSEMAPHORE_PASSWORD=change-me\n",
			Notes:      "Switch SEMAPHORE_DB_DIALECT to postgres or mysql for real deployments.",
		},

		// ---- Non-AI: media +3 ----
		{
			ID: "sonarr", Name: "Sonarr", Description: "TV series library and download automation.",
			Category: "media",
			Source:   "docker-hub", Image: "linuxserver/sonarr:latest",
			Tags: []string{"media", "arr"},
			ComposeContent: `services:
  sonarr:
    image: linuxserver/sonarr:latest
    environment:
      PUID: ${PUID:-1000}
      PGID: ${PGID:-1000}
      TZ: ${TZ:-UTC}
    ports:
      - "${SONARR_PORT:-8989}:8989"
    volumes:
      - ./config:/config
      - ./tv:/tv
      - ./downloads:/downloads
    restart: unless-stopped
`,
			EnvContent: "PUID=1000\nPGID=1000\nTZ=UTC\nSONARR_PORT=8989\n",
			Notes:      "Bind mount TV library and downloads paths as appropriate.",
		},
		{
			ID: "radarr", Name: "Radarr", Description: "Movie library and download automation.",
			Category: "media",
			Source:   "docker-hub", Image: "linuxserver/radarr:latest",
			Tags: []string{"media", "arr"},
			ComposeContent: `services:
  radarr:
    image: linuxserver/radarr:latest
    environment:
      PUID: ${PUID:-1000}
      PGID: ${PGID:-1000}
      TZ: ${TZ:-UTC}
    ports:
      - "${RADARR_PORT:-7878}:7878"
    volumes:
      - ./config:/config
      - ./movies:/movies
      - ./downloads:/downloads
    restart: unless-stopped
`,
			EnvContent: "PUID=1000\nPGID=1000\nTZ=UTC\nRADARR_PORT=7878\n",
			Notes:      "Same style as Sonarr; run both plus qBittorrent/SABnzbd together.",
		},
		{
			ID: "overseerr", Name: "Overseerr", Description: "Request management for Plex / Jellyfin users.",
			Category: "media",
			Source:   "docker-hub", Image: "sctx/overseerr:latest",
			Tags: []string{"media", "requests"},
			ComposeContent: `services:
  overseerr:
    image: sctx/overseerr:latest
    environment:
      TZ: ${TZ:-UTC}
    ports:
      - "${OVERSEERR_PORT:-5055}:5055"
    volumes:
      - overseerr-config:/app/config
    restart: unless-stopped
volumes:
  overseerr-config:
`,
			EnvContent: "TZ=UTC\nOVERSEERR_PORT=5055\n",
			Notes:      "Setup wizard connects to Plex/Jellyfin and Sonarr/Radarr on first launch.",
		},

		// ---- Non-AI: monitoring +3 ----
		{
			ID: "cadvisor", Name: "cAdvisor", Description: "Container resource usage and performance metrics.",
			Category: "monitoring",
			Source:   "docker-hub", Image: "gcr.io/cadvisor/cadvisor:latest",
			Tags: []string{"monitoring", "containers"},
			ComposeContent: `services:
  cadvisor:
    image: gcr.io/cadvisor/cadvisor:latest
    ports:
      - "${CADVISOR_PORT:-8098}:8080"
    volumes:
      - /:/rootfs:ro
      - /var/run:/var/run:ro
      - /sys:/sys:ro
      - /var/lib/docker/:/var/lib/docker:ro
      - /dev/disk/:/dev/disk:ro
    privileged: true
    restart: unless-stopped
`,
			EnvContent: "CADVISOR_PORT=8098\n",
			Notes:      "Exposes /metrics for Prometheus scraping.",
		},
		{
			ID: "node-exporter", Name: "Prometheus Node Exporter", Description: "Host-level metrics exporter for Prometheus.",
			Category: "monitoring",
			Source:   "docker-hub-official", Image: "prom/node-exporter:latest",
			Tags: []string{"monitoring", "prometheus"},
			ComposeContent: `services:
  node-exporter:
    image: prom/node-exporter:latest
    command:
      - --path.rootfs=/host
    network_mode: host
    pid: host
    volumes:
      - /:/host:ro,rslave
    restart: unless-stopped
`,
			EnvContent: "",
			Notes:      "Exposes metrics on 9100 via host networking. Scrape it from Prometheus.",
		},
		{
			ID: "blackbox-exporter", Name: "Prometheus Blackbox Exporter", Description: "Probes HTTP, TCP, DNS, and ICMP endpoints for Prometheus.",
			Category: "monitoring",
			Source:   "docker-hub-official", Image: "prom/blackbox-exporter:latest",
			Tags: []string{"monitoring", "prometheus"},
			ComposeContent: `services:
  blackbox:
    image: prom/blackbox-exporter:latest
    ports:
      - "${BLACKBOX_PORT:-9115}:9115"
    command: ["--config.file=/config/blackbox.yml"]
    configs:
      - source: blackbox_config
        target: /config/blackbox.yml
    restart: unless-stopped
configs:
  # Working default probe modules so the exporter boots out of the box. Edit
  # this inline config (Config tab) to add or change probes.
  blackbox_config:
    content: |
      modules:
        http_2xx:
          prober: http
          timeout: 5s
          http:
            method: GET
        tcp_connect:
          prober: tcp
        icmp:
          prober: icmp
        dns_udp:
          prober: dns
          dns:
            query_name: example.com
            query_type: A
`,
			EnvContent: "BLACKBOX_PORT=9115\n",
			Notes:      "Ships with default http/tcp/icmp/dns probe modules so it boots immediately. Edit the inline blackbox config (Config tab) to customize probes.",
		},

		// ---- Non-AI: queue +3 ----
		{
			ID: "temporal", Name: "Temporal (dev)", Description: "Temporal workflow engine dev server with built-in UI.",
			Category: "queue",
			Source:   "docker-hub", Image: "temporalio/temporal:latest",
			Tags: []string{"queue", "workflows"},
			ComposeContent: `services:
  # Single-container dev Temporal (server + Web UI, built-in storage). The old
  # auto-setup image needs an external Postgres/MySQL and does not support
  # DB=sqlite; server start-dev is the supported lightweight option.
  temporal:
    image: temporalio/temporal:latest
    command: ["server", "start-dev", "--ip", "0.0.0.0", "--ui-ip", "0.0.0.0", "--ui-port", "8233"]
    ports:
      - "${TEMPORAL_PORT:-7233}:7233"
      - "${TEMPORAL_UI_PORT:-8233}:8233"
    volumes:
      - temporal-data:/home/temporal/.config
    restart: unless-stopped
volumes:
  temporal-data:
`,
			EnvContent: "TEMPORAL_PORT=7233\nTEMPORAL_UI_PORT=8233\n",
			Notes:      "Dev server on TEMPORAL_PORT (gRPC) with the Web UI on TEMPORAL_UI_PORT. Uses built-in storage — for production run the temporalio/auto-setup image against Postgres or MySQL instead.",
		},
		{
			ID: "faktory", Name: "Faktory", Description: "Language-agnostic background job server.",
			Category: "queue",
			Source:   "docker-hub", Image: "contribsys/faktory:latest",
			Tags: []string{"queue", "jobs"},
			ComposeContent: `services:
  faktory:
    image: contribsys/faktory:latest
    command: /faktory -e production -b :7419 -w :7420
    environment:
      FAKTORY_PASSWORD: ${FAKTORY_PASSWORD:?set FAKTORY_PASSWORD in .env}
    ports:
      - "${FAKTORY_PORT:-7419}:7419"
      - "${FAKTORY_UI_PORT:-7420}:7420"
    volumes:
      - faktory-data:/var/lib/faktory
    restart: unless-stopped
volumes:
  faktory-data:
`,
			EnvContent: "FAKTORY_PORT=7419\nFAKTORY_UI_PORT=7420\nFAKTORY_PASSWORD=change-me\n",
			Notes:      "Web UI at :7420 uses HTTP Basic with FAKTORY_PASSWORD.",
		},
		{
			ID: "activemq", Name: "Apache ActiveMQ Classic", Description: "Popular JMS broker with STOMP, AMQP, and MQTT support.",
			Category: "queue",
			Source:   "docker-hub-official", Image: "apache/activemq-classic:latest",
			Tags: []string{"queue", "jms"},
			ComposeContent: `services:
  activemq:
    image: apache/activemq-classic:latest
    ports:
      - "${ACTIVEMQ_TCP:-61616}:61616"
      - "${ACTIVEMQ_UI:-8161}:8161"
    volumes:
      - activemq-data:/opt/apache-activemq/data
    restart: unless-stopped
volumes:
  activemq-data:
`,
			EnvContent: "ACTIVEMQ_TCP=61616\nACTIVEMQ_UI=8161\n",
			Notes:      "Default admin credentials are admin/admin — change in conf/jetty-realm.properties.",
		},

		// ---- Non-AI: security +3 ----
		{
			ID: "fail2ban", Name: "fail2ban", Description: "Log-scanning intrusion prevention.",
			Category: "security",
			Source:   "docker-hub", Image: "crazymax/fail2ban:latest",
			Tags: []string{"security", "ips"},
			ComposeContent: `services:
  fail2ban:
    image: crazymax/fail2ban:latest
    network_mode: host
    cap_add:
      - NET_ADMIN
      - NET_RAW
    environment:
      TZ: ${TZ:-UTC}
    volumes:
      - fail2ban-data:/data
      - /var/log:/var/log:ro
    restart: unless-stopped
volumes:
  fail2ban-data:
`,
			EnvContent: "TZ=UTC\n",
			Notes:      "Provide jail.local under the data volume.",
		},
		{
			ID: "wazuh-manager", Name: "Wazuh Manager", Description: "Open-source security monitoring / SIEM manager.",
			Category: "security",
			Source:   "docker-hub", Image: "wazuh/wazuh-manager:4.14.6",
			Tags: []string{"security", "siem"},
			ComposeContent: `services:
  wazuh:
    image: wazuh/wazuh-manager:4.14.6
    ports:
      - "${WAZUH_AGENTS:-1514}:1514"
      - "${WAZUH_ENROLLMENT:-1515}:1515"
      - "${WAZUH_API:-55000}:55000"
    volumes:
      - wazuh-config:/var/ossec/etc
      - wazuh-logs:/var/ossec/logs
    restart: unless-stopped
volumes:
  wazuh-config:
  wazuh-logs:
`,
			EnvContent: "WAZUH_AGENTS=1514\nWAZUH_ENROLLMENT=1515\nWAZUH_API=55000\n",
			Notes:      "Deploy the Wazuh indexer and dashboard separately for the full stack.",
		},
		{
			ID: "certbot", Name: "certbot", Description: "Let's Encrypt cert issuer (interactive mode).",
			Category: "security",
			Source:   "docker-hub", Image: "certbot/certbot:latest",
			Tags: []string{"security", "tls"},
			ComposeContent: `services:
  certbot:
    image: certbot/certbot:latest
    entrypoint: sh -c "sleep infinity"
    volumes:
      - ./certs:/etc/letsencrypt
      - ./www:/var/www/certbot
    restart: unless-stopped
`,
			EnvContent: "",
			Notes:      "Exec certbot manually: 'docker compose exec certbot certbot certonly --webroot -w /var/www/certbot -d example.com'.",
		},

		// ---- Non-AI: web +3 ----
		{
			ID: "openresty", Name: "OpenResty", Description: "Nginx + LuaJIT distribution for scripted request handling.",
			Category: "web",
			Source:   "docker-hub", Image: "openresty/openresty:alpine",
			Tags: []string{"web", "nginx", "lua"},
			ComposeContent: `services:
  openresty:
    image: openresty/openresty:alpine
    ports:
      - "${OPENRESTY_PORT:-8081}:80"
    volumes:
      - ./conf.d:/etc/nginx/conf.d:ro
      - ./html:/usr/local/openresty/nginx/html:ro
    restart: unless-stopped
`,
			EnvContent: "OPENRESTY_PORT=8081\n",
			Notes:      "Drop *.conf files under ./conf.d to configure server blocks.",
		},
		{
			ID: "hugo-server", Name: "Hugo Static Server", Description: "Hugo dev server with live reload for a static site.",
			Category: "web",
			Source:   "docker-hub", Image: "hugomods/hugo:exts",
			Tags: []string{"web", "static", "hugo"},
			// The old klakegg/hugo image is archived/unmaintained; hugomods/hugo
			// is the maintained replacement. On first run, seed a minimal site
			// (config + a home page + a home layout) into the mounted ./site so
			// the server boots and renders instead of crash-looping on an empty
			// dir. Edit ./site on the host or drop in a real Hugo theme.
			ComposeContent: `services:
  hugo:
    image: hugomods/hugo:exts
    entrypoint: ["sh", "-c"]
    command:
      - |
        [ -f hugo.toml ] || [ -f config.toml ] || [ -f hugo.yaml ] || printf 'baseURL = "http://localhost:1313/"\ntitle = "My Hugo Site"\n' > hugo.toml
        [ -f content/_index.md ] || { mkdir -p content; printf '# Welcome\n\nEdit hugo.toml, add a theme, and put content under content/.\n' > content/_index.md; }
        [ -f layouts/index.html ] || { mkdir -p layouts; printf '<!doctype html><html><head><title>{{ .Site.Title }}</title></head><body><h1>{{ .Site.Title }}</h1>{{ .Content }}</body></html>\n' > layouts/index.html; }
        exec hugo server --bind 0.0.0.0 -p 1313 --disableFastRender
    ports:
      - "${HUGO_PORT:-1313}:1313"
    volumes:
      - ./site:/src
    restart: unless-stopped
`,
			EnvContent: "HUGO_PORT=1313\n",
			Notes:      "Boots a live-reload Hugo dev server with a seeded starter site under ./site (config + home page + a minimal home layout). Edit ./site on the host, or drop in a real Hugo theme and set 'theme' in hugo.toml.",
		},
		{
			ID: "caddy-file-browser", Name: "Caddy Static + Auth", Description: "Caddy 2 serving files with basicauth from Caddyfile.",
			Category: "web",
			Source:   "docker-hub-official", Image: "caddy:latest",
			Tags: []string{"web", "static"},
			ComposeContent: `services:
  caddy:
    image: caddy:latest
    ports:
      - "${CADDY_HTTP:-8082}:80"
      - "${CADDY_HTTPS:-8443}:443"
    configs:
      - source: caddyfile
        target: /etc/caddy/Caddyfile
    volumes:
      - ./html:/usr/share/caddy
      - caddy-data:/data
      - caddy-config:/config
    restart: unless-stopped
volumes:
  caddy-data:
  caddy-config:
configs:
  # Starter Caddyfile: serve the ./html folder with a file browser on :80. Add a
  # 'basicauth' block (edit the Config tab) to gate access, or point it at your
  # own domain to get automatic HTTPS.
  caddyfile:
    content: |
      :80 {
        root * /usr/share/caddy
        file_server browse
      }
`,
			EnvContent: "CADDY_HTTP=8082\nCADDY_HTTPS=8443\n",
			Notes:      "Boots serving ./html with directory browsing on the HTTP port. Add a basicauth block or a real domain (for auto-HTTPS) by editing the inline Caddyfile (Config tab).",
		},

		// ---- Non-AI: proxy +7 ----
		// nginx-proxy-manager is defined in templates.go with correct env var names.
		{
			ID: "haproxy", Name: "HAProxy", Description: "Reliable, high-performance L4/L7 load balancer.",
			Category: "proxy",
			Source:   "docker-hub-official", Image: "haproxy:lts-alpine",
			Tags: []string{"proxy", "loadbalancer"},
			ComposeContent: `services:
  haproxy:
    image: haproxy:lts-alpine
    ports:
      - "${HAPROXY_PORT:-8083}:80"
      - "${HAPROXY_STATS:-8404}:8404"
    configs:
      - source: haproxy_config
        target: /usr/local/etc/haproxy/haproxy.cfg
    restart: unless-stopped
configs:
  # Working starter config: a stats page on 8404 and an empty backend on 80.
  # Add real servers to the "servers" backend (Config tab) to load-balance.
  haproxy_config:
    content: |
      global
        log stdout format raw local0
      defaults
        mode http
        log global
        timeout connect 5s
        timeout client 30s
        timeout server 30s
      frontend stats
        bind *:8404
        stats enable
        stats uri /
        stats refresh 10s
      frontend http_in
        bind *:80
        default_backend servers
      backend servers
        balance roundrobin
        # server app1 10.0.0.10:8080 check
`,
			EnvContent: "HAPROXY_PORT=8083\nHAPROXY_STATS=8404\n",
			Notes:      "Boots with a stats page (HAPROXY_STATS port) and an empty backend. Add real servers to the 'servers' backend in the inline config (Config tab).",
		},
		{
			ID: "squid", Name: "Squid Proxy", Description: "Caching HTTP forward proxy.",
			Category: "proxy",
			Source:   "docker-hub", Image: "ubuntu/squid:latest",
			Tags: []string{"proxy", "forward-proxy"},
			ComposeContent: `services:
  squid:
    image: ubuntu/squid:latest
    ports:
      - "${SQUID_PORT:-3128}:3128"
    configs:
      - source: squid_config
        target: /etc/squid/squid.conf
    volumes:
      - squid-cache:/var/spool/squid
    restart: unless-stopped
volumes:
  squid-cache:
configs:
  # Starter config that allows private (RFC1918) networks only and denies the
  # rest, so it is not an open proxy. Edit the ACLs in the Config tab.
  squid_config:
    content: |
      http_port 3128
      acl localnet src 10.0.0.0/8
      acl localnet src 172.16.0.0/12
      acl localnet src 192.168.0.0/16
      acl SSL_ports port 443
      acl Safe_ports port 80 443 21 70 210 1025-65535
      acl CONNECT method CONNECT
      http_access deny !Safe_ports
      http_access deny CONNECT !SSL_ports
      http_access allow localhost
      http_access allow localnet
      http_access deny all
      coredump_dir /var/spool/squid
`,
			EnvContent: "SQUID_PORT=3128\n",
			Notes:      "Boots with a safe starter config that allows only private networks (not an open proxy). Edit the ACLs in the inline config (Config tab).",
		},
		{
			ID: "envoy", Name: "Envoy Proxy", Description: "Cloud-native L4/L7 proxy for service meshes and edge.",
			Category: "proxy",
			Source:   "docker-hub", Image: "envoyproxy/envoy:v1.31-latest",
			Tags: []string{"proxy", "servicemesh"},
			ComposeContent: `services:
  envoy:
    image: envoyproxy/envoy:v1.31-latest
    ports:
      - "${ENVOY_PORT:-10000}:10000"
      - "${ENVOY_ADMIN:-9901}:9901"
    configs:
      - source: envoy_config
        target: /etc/envoy/envoy.yaml
    restart: unless-stopped
configs:
  # Minimal working config: admin on 9901 and a listener on 10000 that returns
  # 200. Replace the direct_response route with real clusters/routes.
  envoy_config:
    content: |
      admin:
        address:
          socket_address: { address: 0.0.0.0, port_value: 9901 }
      static_resources:
        listeners:
        - name: listener_0
          address:
            socket_address: { address: 0.0.0.0, port_value: 10000 }
          filter_chains:
          - filters:
            - name: envoy.filters.network.http_connection_manager
              typed_config:
                "@type": type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager
                stat_prefix: ingress_http
                http_filters:
                - name: envoy.filters.http.router
                  typed_config:
                    "@type": type.googleapis.com/envoy.extensions.filters.http.router.v3.Router
                route_config:
                  name: local_route
                  virtual_hosts:
                  - name: local_service
                    domains: ["*"]
                    routes:
                    - match: { prefix: "/" }
                      direct_response:
                        status: 200
                        body: { inline_string: "Envoy is running. Edit envoy.yaml to add clusters and routes.\n" }
`,
			EnvContent: "ENVOY_PORT=10000\nENVOY_ADMIN=9901\n",
			Notes:      "Boots with admin on ENVOY_ADMIN and a listener on ENVOY_PORT that returns 200. Replace the direct_response route with real clusters/routes in the inline config (Config tab).",
		},
		{
			ID: "swag", Name: "SWAG (LinuxServer.io)", Description: "Reverse proxy with automatic Let's Encrypt certs and fail2ban.",
			Category: "proxy",
			Source:   "docker-hub", Image: "linuxserver/swag:latest",
			Tags: []string{"proxy", "reverse-proxy", "tls"},
			ComposeContent: `services:
  swag:
    image: linuxserver/swag:latest
    cap_add:
      - NET_ADMIN
    environment:
      PUID: ${PUID:-1000}
      PGID: ${PGID:-1000}
      TZ: ${TZ:-UTC}
      URL: ${SWAG_URL:?set SWAG_URL to your domain in .env}
      VALIDATION: http
    ports:
      - "${SWAG_HTTP:-80}:80"
      - "${SWAG_HTTPS:-443}:443"
    volumes:
      - swag-config:/config
    restart: unless-stopped
volumes:
  swag-config:
`,
			EnvContent: "PUID=1000\nPGID=1000\nTZ=UTC\nSWAG_URL=example.com\nSWAG_HTTP=80\nSWAG_HTTPS=443\n",
			Notes:      "Use dns- validation for wildcard certs; see LinuxServer docs.",
		},
		{
			ID: "zoraxy", Name: "Zoraxy", Description: "All-in-one reverse proxy with GUI, TLS, and IP filtering.",
			Category: "proxy",
			Source:   "docker-hub", Image: "zoraxydocker/zoraxy:latest",
			Tags: []string{"proxy", "reverse-proxy"},
			ComposeContent: `services:
  zoraxy:
    image: zoraxydocker/zoraxy:latest
    ports:
      - "${ZORAXY_HTTP:-80}:80"
      - "${ZORAXY_HTTPS:-443}:443"
      - "${ZORAXY_MGMT:-8000}:8000"
    volumes:
      - zoraxy-config:/opt/zoraxy/config
    restart: unless-stopped
volumes:
  zoraxy-config:
`,
			EnvContent: "ZORAXY_HTTP=80\nZORAXY_HTTPS=443\nZORAXY_MGMT=8000\n",
			Notes:      "Set up admin credentials via the management port on first launch.",
		},
		{
			ID: "pomerium", Name: "Pomerium", Description: "Identity-aware access proxy for internal services.",
			Category: "proxy",
			Source:   "docker-hub", Image: "pomerium/pomerium:latest",
			Tags: []string{"proxy", "zero-trust"},
			ComposeContent: `services:
  pomerium:
    image: pomerium/pomerium:latest
    ports:
      - "${POMERIUM_HTTPS:-443}:443"
    configs:
      - source: pomerium_config
        target: /pomerium/config.yaml
    restart: unless-stopped
configs:
  # Starter config so Pomerium boots. It is NON-FUNCTIONAL until you set a real
  # identity provider and REGENERATE the secrets below:
  #   head -c32 /dev/urandom | base64
  pomerium_config:
    content: |
      authenticate_service_url: https://authenticate.localhost.pomerium.io
      # PLACEHOLDER secrets — REGENERATE both before exposing this proxy.
      shared_secret: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
      cookie_secret: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
      insecure_server: true
      address: ":443"
      routes:
        - from: https://verify.localhost.pomerium.io
          to: https://verify.pomerium.com
          allow_public_unauthenticated_access: true
          preserve_host_header: true
`,
			EnvContent: "POMERIUM_HTTPS=443\n",
			Notes:      "Boots with a placeholder starter config. It does NOTHING useful until you configure a real identity provider and REGENERATE shared_secret + cookie_secret (head -c32 /dev/urandom | base64) and your routes in the inline config (Config tab).",
		},
	}
}
