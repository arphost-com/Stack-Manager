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
    working_dir: /srv/app
    # There is no turnkey Strapi image (apps are project-specific), so scaffold a
    # fresh Strapi app into the volume on first run, then start it. First boot
    # takes a few minutes (npm install + scaffold); after that it starts fast.
    command:
      - sh
      - -c
      - |
        if [ ! -f package.json ]; then
          echo "Scaffolding Strapi (first run, a few minutes)..."
          npx -y create-strapi-app@latest . --quickstart --no-run --skip-cloud --use-npm
        fi
        exec npm run develop
    environment:
      HOST: 0.0.0.0
      PORT: "1337"
    ports:
      - "${STRAPI_PORT:-1337}:1337"
    volumes:
      - strapi-data:/srv/app
    restart: unless-stopped
volumes:
  strapi-data:
`,
			EnvContent: "STRAPI_PORT=1337\n",
			Notes:      "First boot scaffolds a fresh Strapi app into the volume and can take several minutes (watch the logs). Then open the admin at /admin to create your first admin user. Uses the scaffold's default SQLite; switch to Postgres in the generated .env for production.",
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
			ID: "jackett", Name: "Jackett", Description: "Indexer proxy giving Sonarr/Radarr access to torrent and Usenet trackers.",
			Category: "media",
			Source:   "linuxserver", Image: "lscr.io/linuxserver/jackett:latest",
			Tags: []string{"media", "indexer", "arr"},
			ComposeContent: `services:
  jackett:
    image: lscr.io/linuxserver/jackett:latest
    environment:
      - PUID=${PUID:-1000}
      - PGID=${PGID:-1000}
      - TZ=${TZ:-Etc/UTC}
    ports:
      - "${JACKETT_PORT:-9117}:9117"
    volumes:
      - jackett-config:/config
    restart: unless-stopped
volumes:
  jackett-config:
`,
			EnvContent: "PUID=1000\nPGID=1000\nTZ=Etc/UTC\nJACKETT_PORT=9117\n",
			Notes:      "Prowlarr is the more modern option, but Jackett is still widely used. Add trackers in the UI, then feed the Torznab URLs to Sonarr/Radarr.",
		},
		{
			ID: "flaresolverr", Name: "FlareSolverr", Description: "Proxy that solves Cloudflare / DDoS-Guard challenges for indexers.",
			Category: "media",
			Source:   "docker-hub", Image: "ghcr.io/flaresolverr/flaresolverr:latest",
			Tags: []string{"media", "indexer", "cloudflare"},
			ComposeContent: `services:
  flaresolverr:
    image: ghcr.io/flaresolverr/flaresolverr:latest
    environment:
      - LOG_LEVEL=info
      - TZ=${TZ:-Etc/UTC}
    ports:
      - "${FLARESOLVERR_PORT:-8191}:8191"
    restart: unless-stopped
`,
			EnvContent: "TZ=Etc/UTC\nFLARESOLVERR_PORT=8191\n",
			Notes:      "Add as a proxy in Prowlarr/Jackett (http://<host>:8191) so Cloudflare-protected indexers work. No persistent data.",
		},
		{
			ID: "deluge", Name: "Deluge", Description: "Lightweight BitTorrent client with a web UI.",
			Category: "media",
			Source:   "linuxserver", Image: "lscr.io/linuxserver/deluge:latest",
			Tags: []string{"media", "download", "torrent"},
			ComposeContent: `services:
  deluge:
    image: lscr.io/linuxserver/deluge:latest
    environment:
      - PUID=${PUID:-1000}
      - PGID=${PGID:-1000}
      - TZ=${TZ:-Etc/UTC}
    ports:
      - "${DELUGE_PORT:-8112}:8112"
      - "6881:6881"
      - "6881:6881/udp"
    volumes:
      - deluge-config:/config
      - ${DOWNLOADS_DIR:-./downloads}:/downloads
    restart: unless-stopped
volumes:
  deluge-config:
`,
			EnvContent: "PUID=1000\nPGID=1000\nTZ=Etc/UTC\nDELUGE_PORT=8112\nDOWNLOADS_DIR=./downloads\n",
			Notes:      "Default web UI password is 'deluge' — change it on first login. Add it as a download client in Sonarr/Radarr.",
		},
		{
			ID: "ombi", Name: "Ombi", Description: "Request platform for Plex/Emby/Jellyfin users (movies, TV, and music).",
			Category: "media",
			Source:   "linuxserver", Image: "lscr.io/linuxserver/ombi:latest",
			Tags: []string{"media", "requests"},
			ComposeContent: `services:
  ombi:
    image: lscr.io/linuxserver/ombi:latest
    environment:
      - PUID=${PUID:-1000}
      - PGID=${PGID:-1000}
      - TZ=${TZ:-Etc/UTC}
    ports:
      - "${OMBI_PORT:-3579}:3579"
    volumes:
      - ombi-config:/config
    restart: unless-stopped
volumes:
  ombi-config:
`,
			EnvContent: "PUID=1000\nPGID=1000\nTZ=Etc/UTC\nOMBI_PORT=3579\n",
			Notes:      "An alternative to Jellyseerr with music-request support. Connect your media server and *arr apps in Settings.",
		},
		{
			ID: "komga", Name: "Komga", Description: "Comics, manga, and digital-book server with a web reader and OPDS.",
			Category: "media",
			Source:   "docker-hub", Image: "gotson/komga:latest",
			Tags: []string{"media", "comics", "manga", "books"},
			ComposeContent: `services:
  komga:
    image: gotson/komga:latest
    environment:
      - TZ=${TZ:-Etc/UTC}
    ports:
      - "${KOMGA_PORT:-25600}:25600"
    volumes:
      - komga-config:/config
      - ${COMICS_DIR:-./comics}:/data
    restart: unless-stopped
volumes:
  komga-config:
`,
			EnvContent: "TZ=Etc/UTC\nKOMGA_PORT=25600\nCOMICS_DIR=./comics\n",
			Notes:      "Point /data at your comics/manga library, create an admin on first launch, then read in-browser or via any OPDS app.",
		},
		{
			ID: "kavita", Name: "Kavita", Description: "Fast self-hosted library for comics, manga, and ebooks with a web reader.",
			Category: "media",
			Source:   "docker-hub", Image: "jvmilazz0/kavita:latest",
			Tags: []string{"media", "books", "comics", "manga", "ebooks"},
			ComposeContent: `services:
  kavita:
    image: jvmilazz0/kavita:latest
    environment:
      - TZ=${TZ:-Etc/UTC}
    ports:
      - "${KAVITA_PORT:-5000}:5000"
    volumes:
      - kavita-config:/kavita/config
      - ${BOOKS_DIR:-./books}:/data
    restart: unless-stopped
volumes:
  kavita-config:
`,
			EnvContent: "TZ=Etc/UTC\nKAVITA_PORT=5000\nBOOKS_DIR=./books\n",
			Notes:      "Add your ebook/comic folders as libraries after the first-run setup wizard.",
		},
		{
			ID: "tdarr", Name: "Tdarr", Description: "Distributed media transcoding and health-checking (audio/video) with a web UI.",
			Category: "media",
			Source:   "docker-hub", Image: "ghcr.io/haveagitgat/tdarr:latest",
			Tags: []string{"media", "transcode", "ffmpeg"},
			ComposeContent: `services:
  tdarr:
    image: ghcr.io/haveagitgat/tdarr:latest
    environment:
      - TZ=${TZ:-Etc/UTC}
      - PUID=${PUID:-1000}
      - PGID=${PGID:-1000}
      - internalNode=true
      - serverIP=0.0.0.0
      - serverPort=8266
      - webUIPort=8265
    ports:
      - "${TDARR_WEBUI_PORT:-8265}:8265"
      - "${TDARR_SERVER_PORT:-8266}:8266"
    volumes:
      - tdarr-server:/app/server
      - tdarr-configs:/app/configs
      - tdarr-logs:/app/logs
      - ${MEDIA_DIR:-./media}:/media
      - ${TRANSCODE_DIR:-./transcode}:/temp
    restart: unless-stopped
volumes:
  tdarr-server:
  tdarr-configs:
  tdarr-logs:
`,
			EnvContent: "TZ=Etc/UTC\nPUID=1000\nPGID=1000\nTDARR_WEBUI_PORT=8265\nTDARR_SERVER_PORT=8266\nMEDIA_DIR=./media\nTRANSCODE_DIR=./transcode\n",
			Notes:      "For GPU transcoding, add GPU passthrough (Settings > GPU or the per-project Enable GPU). Point /media at your library.",
		},
		{
			ID: "openedai-speech", Name: "openedai-speech (TTS + voice cloning)",
			Description: "OpenAI-compatible text-to-speech: fast Piper voices plus XTTS-v2 voice cloning from a reference sample. Drop-in TTS for Open WebUI.",
			Category:    "ai",
			Subcategory: "voice-speech",
			Source:      "docker-hub", Image: "ghcr.io/matatonic/openedai-speech:latest",
			Tags: []string{"ai", "tts", "voice", "cloning", "xtts", "piper", "openai-compatible"},
			ComposeContent: `services:
  openedai-speech:
    image: ghcr.io/matatonic/openedai-speech:latest
    restart: unless-stopped
    ports:
      - "${TTS_PORT:-8000}:8000"
    volumes:
      - openedai-config:/app/config
      - openedai-voices:/app/voices
volumes:
  openedai-config:
  openedai-voices:
`,
			EnvContent: "TTS_PORT=8000\n",
			Notes:      "OpenAI-compatible TTS at /v1/audio/speech (no homepage at /). Model tts-1 = fast Piper voices; tts-1-hd = XTTS-v2 (quality + voice cloning; GPU strongly recommended — enable GPU passthrough). To CLONE a voice: put a clean 10-30s WAV in the openedai-voices volume and add a custom voice in config/voice_to_speaker.yaml pointing at it, then request that voice name. In Open WebUI set Audio TTS to OpenAI, base URL http://<host>:TTS_PORT/v1 (or http://openedai-speech:8000/v1 from inside a network), model tts-1-hd, voice = your custom name. Only clone voices you have rights to use.",
		},

		// ---- Media: gaps filled (photos, YouTube, live-TV, arr companions, users, analytics) ----
		{
			ID: "immich", Name: "Immich", Description: "Self-hosted photo and video backup and management with mobile apps and ML search.",
			Category:    "media",
			Subcategory: "photos",
			Source:      "docker-hub", Image: "ghcr.io/immich-app/immich-server:release",
			Tags: []string{"media", "photos", "video", "backup", "google-photos-alternative"},
			ComposeContent: `services:
  immich-server:
    image: ghcr.io/immich-app/immich-server:${IMMICH_VERSION:-release}
    ports:
      - "${IMMICH_PORT:-2283}:2283"
    volumes:
      - ./media:/data
      - /etc/localtime:/etc/localtime:ro
    environment:
      DB_HOSTNAME: database
      DB_USERNAME: ${DB_USERNAME:-postgres}
      DB_PASSWORD: ${DB_PASSWORD}
      DB_DATABASE_NAME: ${DB_DATABASE_NAME:-immich}
      REDIS_HOSTNAME: redis
    depends_on:
      - redis
      - database
    restart: unless-stopped
  immich-machine-learning:
    image: ghcr.io/immich-app/immich-machine-learning:${IMMICH_VERSION:-release}
    volumes:
      - model-cache:/cache
    restart: unless-stopped
  redis:
    image: docker.io/valkey/valkey:9
    volumes:
      - redis-data:/data
    restart: unless-stopped
  database:
    image: ghcr.io/immich-app/postgres:14-vectorchord0.4.3-pgvectors0.2.0
    environment:
      POSTGRES_USER: ${DB_USERNAME:-postgres}
      POSTGRES_PASSWORD: ${DB_PASSWORD}
      POSTGRES_DB: ${DB_DATABASE_NAME:-immich}
      POSTGRES_INITDB_ARGS: "--data-checksums"
    volumes:
      - db-data:/var/lib/postgresql/data
    restart: unless-stopped
volumes:
  model-cache:
  redis-data:
  db-data:
`,
			EnvContent: "IMMICH_VERSION=release\nIMMICH_PORT=2283\nDB_USERNAME=postgres\nDB_DATABASE_NAME=immich\nDB_PASSWORD=change-me-immich-db-password\n",
			Notes:      "Set DB_PASSWORD before first boot. The library lives under ./media (bind mount); DB/cache use named volumes. Web UI + mobile apps on IMMICH_PORT (2283) — create the admin account on first visit. Uses the current official Postgres (VectorChord) and Valkey images. GPU is optional but speeds up ML search/face recognition.",
		},
		{
			ID: "metube", Name: "MeTube", Description: "Web GUI for yt-dlp to download video and audio from YouTube and many other sites.",
			Category:    "media",
			Subcategory: "youtube",
			Source:      "docker-hub", Image: "ghcr.io/alexta69/metube:latest",
			Tags: []string{"media", "youtube", "yt-dlp", "downloader"},
			ComposeContent: `services:
  metube:
    image: ghcr.io/alexta69/metube:latest
    ports:
      - "${METUBE_PORT:-8081}:8081"
    volumes:
      - ./media:/downloads
    environment:
      UID: ${METUBE_UID:-1000}
      GID: ${METUBE_GID:-1000}
    restart: unless-stopped
`,
			EnvContent: "METUBE_PORT=8081\nMETUBE_UID=1000\nMETUBE_GID=1000\n",
			Notes:      "No dependencies or secrets. Paste a URL in the web UI (METUBE_PORT) and files land under ./media. Optional YTDL_OPTIONS env (JSON) for format/quality defaults.",
		},
		{
			ID: "pinchflat", Name: "Pinchflat", Description: "Self-hosted YouTube channel/playlist DVR that auto-downloads and archives new videos.",
			Category:    "media",
			Subcategory: "youtube",
			Source:      "docker-hub", Image: "ghcr.io/kieraneglin/pinchflat:latest",
			Tags: []string{"media", "youtube", "dvr", "archive", "yt-dlp"},
			ComposeContent: `services:
  pinchflat:
    image: ghcr.io/kieraneglin/pinchflat:latest
    restart: unless-stopped
    ports:
      - "${PINCHFLAT_PORT:-8945}:8945"
    environment:
      - TZ=${TZ:-Etc/UTC}
    volumes:
      - pinchflat_config:/config
      - ./media:/downloads
volumes:
  pinchflat_config:
`,
			EnvContent: "PINCHFLAT_PORT=8945\nTZ=Etc/UTC\n",
			Notes:      "Subscribe to channels/playlists in the web UI (PINCHFLAT_PORT 8945) and it downloads new uploads on a schedule into ./media. Set TZ for correct scheduling. No built-in auth — put behind a reverse proxy if exposed.",
		},
		{
			ID: "tube-archivist", Name: "Tube Archivist", Description: "Self-hosted YouTube media server: subscribe, download, index and stream channels with search.",
			Category:    "media",
			Subcategory: "youtube",
			Source:      "docker-hub", Image: "bbilly1/tubearchivist:latest",
			Tags: []string{"media", "youtube", "archive", "search", "jellyfin"},
			ComposeContent: `services:
  tubearchivist:
    image: bbilly1/tubearchivist:latest
    ports:
      - "${TA_PORT:-8000}:8000"
    volumes:
      - ./media:/youtube
      - cache:/cache
    environment:
      ES_URL: http://archivist-es:9200
      REDIS_CON: redis://archivist-redis:6379
      HOST_UID: ${HOST_UID:-1000}
      HOST_GID: ${HOST_GID:-1000}
      TA_HOST: ${TA_HOST}
      TA_USERNAME: ${TA_USERNAME}
      TA_PASSWORD: ${TA_PASSWORD}
      ELASTIC_PASSWORD: ${ELASTIC_PASSWORD}
      TZ: ${TZ:-Etc/UTC}
    depends_on:
      - archivist-es
      - archivist-redis
    restart: unless-stopped
  archivist-redis:
    image: redis:latest
    volumes:
      - redis:/data
    depends_on:
      - archivist-es
    restart: unless-stopped
  archivist-es:
    image: docker.elastic.co/elasticsearch/elasticsearch:8.19.0
    environment:
      ELASTIC_PASSWORD: ${ELASTIC_PASSWORD}
      discovery.type: single-node
      xpack.security.enabled: "true"
      ES_JAVA_OPTS: "-Xms512m -Xmx512m"
    ulimits:
      memlock:
        soft: -1
        hard: -1
    volumes:
      - es:/usr/share/elasticsearch/data
    restart: unless-stopped
volumes:
  cache:
  redis:
  es:
`,
			EnvContent: "TA_PORT=8000\nTA_HOST=http://localhost:8000\nHOST_UID=1000\nHOST_GID=1000\nTZ=Etc/UTC\nTA_USERNAME=tubearchivist\nTA_PASSWORD=change-me-ta-password\nELASTIC_PASSWORD=change-me-elastic-password\n",
			Notes:      "Heavier: bundles Elasticsearch + Redis. Set TA_HOST to the actual browser URL/IP (e.g. http://192.168.1.10:8000), and change TA_PASSWORD + ELASTIC_PASSWORD before first boot. First login uses TA_USERNAME/TA_PASSWORD. Integrates with Jellyfin.",
		},
		{
			ID: "jellystat", Name: "Jellystat", Description: "Statistics and watch-history dashboard for a Jellyfin server (a Tautulli for Jellyfin).",
			Category:    "media",
			Subcategory: "monitoring",
			Source:      "docker-hub", Image: "cyfershepard/jellystat:latest",
			Tags: []string{"media", "jellyfin", "stats", "analytics", "monitoring"},
			ComposeContent: `services:
  jellystat-db:
    image: postgres:18.1
    restart: unless-stopped
    shm_size: 1gb
    environment:
      - POSTGRES_USER=${POSTGRES_USER:-jellystat}
      - POSTGRES_PASSWORD=${POSTGRES_PASSWORD:-change-me-db-password}
    volumes:
      - jellystat_pgdata:/var/lib/postgresql
  jellystat:
    image: cyfershepard/jellystat:latest
    restart: unless-stopped
    depends_on:
      - jellystat-db
    ports:
      - "${JELLYSTAT_PORT:-3000}:3000"
    environment:
      - POSTGRES_USER=${POSTGRES_USER:-jellystat}
      - POSTGRES_PASSWORD=${POSTGRES_PASSWORD:-change-me-db-password}
      - POSTGRES_IP=jellystat-db
      - POSTGRES_PORT=5432
      - JWT_SECRET=${JWT_SECRET:-change-me-jwt-secret}
      - TZ=${TZ:-Etc/UTC}
    volumes:
      - jellystat_backup:/app/backend/backup-data
volumes:
  jellystat_pgdata:
  jellystat_backup:
`,
			EnvContent: "JELLYSTAT_PORT=3000\nPOSTGRES_USER=jellystat\nPOSTGRES_PASSWORD=change-me-db-password\nJWT_SECRET=change-me-jwt-secret\nTZ=Etc/UTC\n",
			Notes:      "Change POSTGRES_PASSWORD and JWT_SECRET before first boot. Create the admin account on first login (JELLYSTAT_PORT 3000), then add your Jellyfin server URL + API key in the UI.",
		},
		{
			ID: "unpackerr", Name: "Unpackerr", Description: "Headless daemon that auto-extracts compressed downloads for Sonarr/Radarr/Lidarr/Readarr.",
			Category:    "media",
			Subcategory: "automation",
			Source:      "docker-hub", Image: "golift/unpackerr:latest",
			Tags: []string{"media", "arr", "automation", "companion"},
			ComposeContent: `services:
  unpackerr:
    image: golift/unpackerr:latest
    restart: unless-stopped
    user: "${PUID:-1000}:${PGID:-1000}"
    volumes:
      - ./media:/downloads
    environment:
      - TZ=${TZ:-Etc/UTC}
      - UN_LOG_FILE=/downloads/unpackerr.log
      - UN_SONARR_0_URL=${UN_SONARR_0_URL:-http://sonarr:8989}
      - UN_SONARR_0_API_KEY=${UN_SONARR_0_API_KEY:-change-me-sonarr-api-key}
      - UN_RADARR_0_URL=${UN_RADARR_0_URL:-http://radarr:7878}
      - UN_RADARR_0_API_KEY=${UN_RADARR_0_API_KEY:-change-me-radarr-api-key}
`,
			EnvContent: "PUID=1000\nPGID=1000\nTZ=Etc/UTC\nUN_SONARR_0_URL=http://sonarr:8989\nUN_SONARR_0_API_KEY=change-me-sonarr-api-key\nUN_RADARR_0_URL=http://radarr:7878\nUN_RADARR_0_API_KEY=change-me-radarr-api-key\n",
			Notes:      "No web UI — a background helper. Fill in your Sonarr/Radarr API keys and make sure ./media matches the same download path Sonarr/Radarr use, so it can see and extract completed archives. Add UN_LIDARR_0_* / UN_READARR_0_* as needed.",
		},
		{
			ID: "recyclarr", Name: "Recyclarr", Description: "Syncs TRaSH-Guides custom formats and quality profiles into Sonarr and Radarr on a schedule.",
			Category:    "media",
			Subcategory: "automation",
			Source:      "docker-hub", Image: "ghcr.io/recyclarr/recyclarr:8",
			Tags: []string{"media", "arr", "automation", "trash-guides"},
			ComposeContent: `services:
  recyclarr:
    image: ghcr.io/recyclarr/recyclarr:8
    restart: unless-stopped
    environment:
      - TZ=${TZ:-Etc/UTC}
      - CRON_SCHEDULE=${RECYCLARR_CRON:-@daily}
    volumes:
      - recyclarr_config:/config
volumes:
  recyclarr_config:
`,
			EnvContent: "TZ=Etc/UTC\nRECYCLARR_CRON=@daily\n",
			Notes:      "No web UI — runs on a cron (default daily). REQUIRED post-deploy step: create /config/recyclarr.yml in the recyclarr_config volume with your Sonarr/Radarr base_url + api_key (see recyclarr.dev), or it does nothing.",
		},
		{
			ID: "autobrr", Name: "autobrr", Description: "Real-time IRC/RSS filter that grabs torrents and hands releases to your download client.",
			Category:    "media",
			Subcategory: "automation",
			Source:      "docker-hub", Image: "ghcr.io/autobrr/autobrr:latest",
			Tags: []string{"media", "torrent", "automation", "irc", "rss"},
			ComposeContent: `services:
  autobrr:
    image: ghcr.io/autobrr/autobrr:latest
    restart: unless-stopped
    environment:
      - TZ=${TZ:-Etc/UTC}
    volumes:
      - autobrr_config:/config
    ports:
      - "${AUTOBRR_PORT:-7474}:7474"
volumes:
  autobrr_config:
`,
			EnvContent: "TZ=Etc/UTC\nAUTOBRR_PORT=7474\n",
			Notes:      "Single container (SQLite, no external DB). Open the web UI (AUTOBRR_PORT 7474) to create the admin user, then add indexers, IRC networks, and filters. Config persists in autobrr_config.",
		},
		{
			ID: "ersatztv", Name: "ErsatzTV", Description: "Builds custom live-TV channels from your library, streamed to Plex/Jellyfin/Emby via HDHR/M3U.",
			Category:    "media",
			Subcategory: "live-tv",
			Source:      "docker-hub", Image: "ghcr.io/ersatztv/legacy:latest",
			Tags: []string{"media", "live-tv", "iptv", "plex", "jellyfin"},
			ComposeContent: `services:
  ersatztv:
    image: ghcr.io/ersatztv/legacy:latest
    restart: unless-stopped
    environment:
      - TZ=${TZ:-Etc/UTC}
    ports:
      - "${ERSATZTV_PORT:-8409}:8409"
    volumes:
      - ersatztv_config:/config
      - ./media:/data:ro
    tmpfs:
      - /transcode
volumes:
  ersatztv_config:
`,
			EnvContent: "TZ=Etc/UTC\nERSATZTV_PORT=8409\n",
			Notes:      "Add ./media as a Local library in the UI (ERSATZTV_PORT 8409), then build channels/schedules. Point Plex/Jellyfin/Emby Live TV at ErsatzTV's HDHR or M3U URL. For hardware transcoding pass /dev/dri (VAAPI/QSV) or an NVIDIA GPU.",
		},
		{
			ID: "threadfin", Name: "Threadfin", Description: "IPTV/M3U proxy and EPG manager that feeds live TV into Plex, Jellyfin, and Emby.",
			Category:    "media",
			Subcategory: "live-tv",
			Source:      "docker-hub", Image: "fyb3roptik/threadfin:latest",
			Tags: []string{"media", "live-tv", "iptv", "m3u", "epg"},
			ComposeContent: `services:
  threadfin:
    image: fyb3roptik/threadfin:latest
    restart: unless-stopped
    ports:
      - "${THREADFIN_PORT:-34400}:34400"
    environment:
      - PUID=${PUID:-1000}
      - PGID=${PGID:-1000}
      - TZ=${TZ:-Etc/UTC}
    volumes:
      - threadfin_conf:/home/threadfin/conf
      - threadfin_temp:/tmp/threadfin
volumes:
  threadfin_conf:
  threadfin_temp:
`,
			EnvContent: "THREADFIN_PORT=34400\nPUID=1000\nPGID=1000\nTZ=Etc/UTC\n",
			Notes:      "Open the web UI (THREADFIN_PORT 34400), add an M3U playlist URL and an XMLTV EPG, then map/filter channels. Point your media server's Live TV/DVR at Threadfin's M3U + EPG endpoints.",
		},
		{
			ID: "maintainerr", Name: "Maintainerr", Description: "Rule-based library maintenance that finds and cleans up stale media in Plex/Jellyfin/Emby.",
			Category:    "media",
			Subcategory: "automation",
			Source:      "docker-hub", Image: "ghcr.io/maintainerr/maintainerr:latest",
			Tags: []string{"media", "plex", "cleanup", "automation", "overseerr"},
			ComposeContent: `services:
  maintainerr:
    image: ghcr.io/maintainerr/maintainerr:latest
    restart: unless-stopped
    user: "${PUID:-1000}:${PGID:-1000}"
    ports:
      - "${MAINTAINERR_PORT:-6246}:6246"
    environment:
      - TZ=${TZ:-Etc/UTC}
    volumes:
      - maintainerr_data:/opt/data
volumes:
  maintainerr_data:
`,
			EnvContent: "MAINTAINERR_PORT=6246\nPUID=1000\nPGID=1000\nTZ=Etc/UTC\n",
			Notes:      "Open the web UI (MAINTAINERR_PORT 6246), connect Plex and Overseerr/Jellyseerr, then build rules (e.g. delete watched content older than N days). Data persists in maintainerr_data.",
		},
		{
			ID: "wizarr", Name: "Wizarr", Description: "Invitation and user-management portal for onboarding users to Jellyfin, Plex, and Emby.",
			Category:    "media",
			Subcategory: "users",
			Source:      "docker-hub", Image: "ghcr.io/wizarrrr/wizarr:latest",
			Tags: []string{"media", "users", "invitations", "jellyfin", "plex"},
			ComposeContent: `services:
  wizarr:
    image: ghcr.io/wizarrrr/wizarr:latest
    restart: unless-stopped
    ports:
      - "${WIZARR_PORT:-5690}:5690"
    environment:
      - PUID=${PUID:-1000}
      - PGID=${PGID:-1000}
      - TZ=${TZ:-Etc/UTC}
      - DISABLE_BUILTIN_AUTH=${DISABLE_BUILTIN_AUTH:-false}
    volumes:
      - wizarr_data:/data
volumes:
  wizarr_data:
`,
			EnvContent: "WIZARR_PORT=5690\nPUID=1000\nPGID=1000\nTZ=Etc/UTC\nDISABLE_BUILTIN_AUTH=false\n",
			Notes:      "Complete the setup wizard in the web UI (WIZARR_PORT 5690) to connect your media server (admin URL + API key), then hand out invitation links. Data persists in wizarr_data. Only set DISABLE_BUILTIN_AUTH=true when fronting with an external auth proxy.",
		},

		// ---- Cool & new: gaming, remote, utilities, finance, productivity ----
		{
			ID: "romm", Name: "RomM", Description: "Self-hosted ROM manager with EmulatorJS play-in-browser, metadata scraping, and save sync.",
			Category:    "gaming",
			Source:      "docker-hub", Image: "rommapp/romm:latest",
			Tags: []string{"gaming", "roms", "emulation", "emulatorjs"},
			ComposeContent: `services:
  romm:
    image: rommapp/romm:latest
    restart: unless-stopped
    environment:
      - DB_HOST=romm-db
      - DB_NAME=romm
      - DB_USER=romm-user
      - DB_PASSWD=${DB_PASSWD}
      - ROMM_AUTH_SECRET_KEY=${ROMM_AUTH_SECRET_KEY}
    volumes:
      - romm_resources:/romm/resources
      - romm_redis_data:/redis-data
      - ./library:/romm/library
      - ./assets:/romm/assets
      - ./config:/romm/config
    ports:
      - "${ROMM_PORT:-8080}:8080"
    depends_on:
      romm-db:
        condition: service_healthy
        restart: true
  romm-db:
    image: mariadb:11
    restart: unless-stopped
    environment:
      - MARIADB_ROOT_PASSWORD=${MARIADB_ROOT_PASSWORD}
      - MARIADB_DATABASE=romm
      - MARIADB_USER=romm-user
      - MARIADB_PASSWORD=${DB_PASSWD}
    volumes:
      - mysql_data:/var/lib/mysql
    healthcheck:
      test: ["CMD", "healthcheck.sh", "--connect", "--innodb_initialized"]
      start_period: 30s
      start_interval: 10s
      interval: 10s
      timeout: 5s
      retries: 5
volumes:
  mysql_data:
  romm_resources:
  romm_redis_data:
`,
			EnvContent: "ROMM_PORT=8080\nDB_PASSWD=change-me-db-password\nMARIADB_ROOT_PASSWORD=change-me-root-password\nROMM_AUTH_SECRET_KEY=change-me-run-openssl-rand-hex-32\n",
			Notes:      "Pairs with EmulatorJS to play in-browser. Set DB_PASSWD, MARIADB_ROOT_PASSWORD, and ROMM_AUTH_SECRET_KEY (openssl rand -hex 32) before first boot. Put ROMs under ./library. Optional IGDB/ScreenScraper/SteamGridDB keys improve metadata.",
		},
		{
			ID: "neko", Name: "Neko", Description: "Self-hosted virtual browser streamed over WebRTC for shared/collaborative viewing.",
			Category:    "remote",
			Source:      "docker-hub", Image: "ghcr.io/m1k1o/neko/firefox:latest",
			Tags: []string{"remote", "browser", "webrtc", "watch-party"},
			ComposeContent: `services:
  neko:
    image: ghcr.io/m1k1o/neko/firefox:latest
    restart: unless-stopped
    shm_size: "2gb"
    ports:
      - "${NEKO_PORT:-8080}:8080"
      - "52000-52100:52000-52100/udp"
    environment:
      NEKO_DESKTOP_SCREEN: 1920x1080@30
      NEKO_MEMBER_MULTIUSER_USER_PASSWORD: ${NEKO_USER_PASSWORD}
      NEKO_MEMBER_MULTIUSER_ADMIN_PASSWORD: ${NEKO_ADMIN_PASSWORD}
      NEKO_WEBRTC_EPR: 52000-52100
      NEKO_WEBRTC_ICELITE: 1
`,
			EnvContent: "NEKO_PORT=8080\nNEKO_USER_PASSWORD=change-me-user-password\nNEKO_ADMIN_PASSWORD=change-me-admin-password\n",
			Notes:      "Swap the image flavor (/chromium, /brave, /kde, ...) for other apps. The UDP 52000-52100 range must be published and match NEKO_WEBRTC_EPR. Behind NAT/public IP add NEKO_NAT1TO1 with your public IP (or drop ICELITE). shm_size 2gb needed for the browser.",
		},
		{
			ID: "guacamole", Name: "Apache Guacamole", Description: "Clientless RDP/VNC/SSH remote-desktop gateway accessed entirely from the browser.",
			Category:    "remote",
			Source:      "docker-hub", Image: "guacamole/guacamole:1.6.0",
			Tags: []string{"remote", "rdp", "vnc", "ssh", "gateway"},
			ComposeContent: `services:
  guac-init:
    image: guacamole/guacamole:1.6.0
    entrypoint: ["/bin/sh", "-c"]
    command:
      - test -s /init/initdb.sql || /opt/guacamole/bin/initdb.sh --postgresql > /init/initdb.sql
    volumes:
      - guac-init:/init
    restart: "no"
  postgres:
    image: postgres:15-alpine
    restart: unless-stopped
    environment:
      POSTGRES_DB: guacamole_db
      POSTGRES_USER: guacamole_user
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
    volumes:
      - guac_pg:/var/lib/postgresql/data
      - guac-init:/docker-entrypoint-initdb.d:ro
    depends_on:
      guac-init:
        condition: service_completed_successfully
  guacd:
    image: guacamole/guacd:1.6.0
    restart: unless-stopped
  guacamole:
    image: guacamole/guacamole:1.6.0
    restart: unless-stopped
    depends_on:
      - guacd
      - postgres
    environment:
      GUACD_HOSTNAME: guacd
      POSTGRESQL_HOSTNAME: postgres
      POSTGRESQL_PORT: 5432
      POSTGRESQL_DATABASE: guacamole_db
      POSTGRESQL_USERNAME: guacamole_user
      POSTGRESQL_PASSWORD: ${POSTGRES_PASSWORD}
    ports:
      - "${GUAC_PORT:-8080}:8080"
volumes:
  guac_pg:
  guac-init:
`,
			EnvContent: "GUAC_PORT=8080\nPOSTGRES_PASSWORD=change-me-postgres-password\n",
			Notes:      "Web UI is at /guacamole/ (e.g. http://host:8080/guacamole/). The guac-init sidecar generates the DB schema automatically before Postgres first-boots, so it is one-click. Default login guacadmin / guacadmin — change it immediately. Set POSTGRES_PASSWORD before first boot.",
		},
		{
			ID: "rustdesk", Name: "RustDesk Server", Description: "Self-hosted RustDesk signal + relay server for private remote-desktop connections (TeamViewer alt).",
			Category:    "remote",
			Source:      "docker-hub", Image: "rustdesk/rustdesk-server:latest",
			Tags: []string{"remote", "remote-desktop", "rustdesk"},
			ComposeContent: `services:
  hbbs:
    image: rustdesk/rustdesk-server:latest
    restart: unless-stopped
    command: hbbs -r ${RUSTDESK_RELAY_HOST}:21117
    ports:
      - "21115:21115"
      - "21116:21116"
      - "21116:21116/udp"
      - "21118:21118"
    volumes:
      - ./data:/root
    depends_on:
      - hbbr
    networks:
      - rustdesk-net
  hbbr:
    image: rustdesk/rustdesk-server:latest
    restart: unless-stopped
    command: hbbr
    ports:
      - "21117:21117"
      - "21119:21119"
    volumes:
      - ./data:/root
    networks:
      - rustdesk-net
networks:
  rustdesk-net:
`,
			EnvContent: "RUSTDESK_RELAY_HOST=change-me-your-public-host-or-ip\n",
			Notes:      "Headless (no web UI). Set RUSTDESK_RELAY_HOST to your public host/IP. On first boot hbbs writes a keypair to ./data — read ./data/id_ed25519.pub and enter it in RustDesk clients as the key. Ports 21115-21119 must be reachable; UDP 21116 is required for discovery.",
		},
		{
			ID: "cobalt", Name: "Cobalt", Description: "Self-hosted social-media/video downloader backend exposing a clean JSON download API.",
			Category:    "media",
			Subcategory: "youtube",
			Source:      "docker-hub", Image: "ghcr.io/imputnet/cobalt:11",
			Tags: []string{"media", "downloader", "video", "api"},
			ComposeContent: `services:
  cobalt:
    image: ghcr.io/imputnet/cobalt:11
    init: true
    read_only: true
    restart: unless-stopped
    ports:
      - "${COBALT_PORT:-9000}:9000"
    environment:
      API_URL: "${API_URL}"
`,
			EnvContent: "COBALT_PORT=9000\nAPI_URL=http://localhost:9000/\n",
			Notes:      "JSON API only (no bundled web UI). Set API_URL to the exact public URL the API is reached at (scheme+host, trailing slash) or downloads fail. No auth by default — put behind a reverse proxy/allowlist. For YouTube add the yt-session-generator helper.",
		},
		{
			ID: "convertx", Name: "ConvertX", Description: "Self-hosted file converter supporting 1000+ formats (documents, images, video, audio).",
			Category:    "files",
			Source:      "docker-hub", Image: "ghcr.io/c4illin/convertx:main",
			Tags: []string{"files", "converter", "utility"},
			ComposeContent: `services:
  convertx:
    image: ghcr.io/c4illin/convertx:main
    restart: unless-stopped
    ports:
      - "${CONVERTX_PORT:-3000}:3000"
    environment:
      JWT_SECRET: "${JWT_SECRET}"
      ACCOUNT_REGISTRATION: "false"
    volumes:
      - ./data:/app/data
`,
			EnvContent: "CONVERTX_PORT=3000\nJWT_SECRET=change-me-long-random-jwt-secret\n",
			Notes:      "First account created becomes admin. Set JWT_SECRET (long random string) before first boot. Set HTTP_ALLOWED=true only when serving over plain HTTP/localhost. Leave ACCOUNT_REGISTRATION=false after the first user to lock signups.",
		},
		{
			ID: "cyberchef", Name: "CyberChef", Description: "The Cyber Swiss-army knife: encryption, encoding, compression, and data analysis in the browser.",
			Category:    "devtools",
			Source:      "docker-hub", Image: "ghcr.io/gchq/cyberchef:latest",
			Tags: []string{"devtools", "security", "encoding", "utility"},
			ComposeContent: `services:
  cyberchef:
    image: ghcr.io/gchq/cyberchef:latest
    restart: unless-stopped
    ports:
      - "${CYBERCHEF_PORT:-8000}:8080"
`,
			EnvContent: "CYBERCHEF_PORT=8000\n",
			Notes:      "Fully static/stateless — no DB, secrets, or volumes. Serves the SPA over HTTP on container port 8080; put behind your own TLS/reverse proxy. Nothing to configure on first run.",
		},
		{
			ID: "karakeep", Name: "Karakeep", Description: "Self-hosted AI bookmarking (formerly Hoarder) with full-text search and auto-tagging.",
			Category:    "productivity",
			Source:      "docker-hub", Image: "ghcr.io/karakeep-app/karakeep:release",
			Tags: []string{"productivity", "bookmarks", "ai", "search"},
			ComposeContent: `services:
  web:
    image: ghcr.io/karakeep-app/karakeep:${KARAKEEP_VERSION:-release}
    restart: unless-stopped
    ports:
      - "${KARAKEEP_PORT:-3000}:3000"
    environment:
      DATA_DIR: /data
      MEILI_ADDR: http://meilisearch:7700
      MEILI_MASTER_KEY: "${MEILI_MASTER_KEY}"
      BROWSER_WEB_URL: http://chrome:9222
      NEXTAUTH_URL: "${NEXTAUTH_URL}"
      NEXTAUTH_SECRET: "${NEXTAUTH_SECRET}"
    volumes:
      - data:/data
    depends_on:
      - meilisearch
      - chrome
  chrome:
    image: gcr.io/zenika-hub/alpine-chrome:124
    restart: unless-stopped
    command:
      - --no-sandbox
      - --disable-gpu
      - --disable-dev-shm-usage
      - --remote-debugging-address=0.0.0.0
      - --remote-debugging-port=9222
      - --hide-scrollbars
  meilisearch:
    image: getmeili/meilisearch:v1.41.0
    restart: unless-stopped
    environment:
      MEILI_NO_ANALYTICS: "true"
      MEILI_MASTER_KEY: "${MEILI_MASTER_KEY}"
    volumes:
      - meilisearch:/meili_data
volumes:
  data:
  meilisearch:
`,
			EnvContent: "KARAKEEP_PORT=3000\nKARAKEEP_VERSION=release\nNEXTAUTH_URL=http://localhost:3000\nNEXTAUTH_SECRET=change-me-openssl-rand-base64-36\nMEILI_MASTER_KEY=change-me-openssl-rand-base64-36\n",
			Notes:      "Set NEXTAUTH_SECRET and MEILI_MASTER_KEY (both openssl rand -base64 36) and NEXTAUTH_URL to your server URL before first boot. Optional AI auto-tagging: add OPENAI_API_KEY, or point at your Ollama with OLLAMA_BASE_URL + INFERENCE_TEXT_MODEL (great with OpenBrain).",
		},
		{
			ID: "linkwarden", Name: "Linkwarden", Description: "Self-hosted bookmark manager that archives full-page snapshots (screenshot/PDF/HTML) of links.",
			Category:    "productivity",
			Source:      "docker-hub", Image: "ghcr.io/linkwarden/linkwarden:latest",
			Tags: []string{"productivity", "bookmarks", "archive"},
			ComposeContent: `services:
  postgres:
    image: postgres:16-alpine
    restart: unless-stopped
    environment:
      - POSTGRES_PASSWORD=${POSTGRES_PASSWORD}
    volumes:
      - pgdata:/var/lib/postgresql/data
  linkwarden:
    image: ghcr.io/linkwarden/linkwarden:latest
    restart: unless-stopped
    depends_on:
      - postgres
    ports:
      - "${LINKWARDEN_PORT:-3000}:3000"
    environment:
      - NEXTAUTH_URL=${NEXTAUTH_URL:-http://localhost:3000/api/v1/auth}
      - NEXTAUTH_SECRET=${NEXTAUTH_SECRET}
      - DATABASE_URL=postgresql://postgres:${POSTGRES_PASSWORD}@postgres:5432/postgres
    volumes:
      - ./data:/data/data
volumes:
  pgdata:
`,
			EnvContent: "LINKWARDEN_PORT=3000\nNEXTAUTH_URL=http://localhost:3000/api/v1/auth\nNEXTAUTH_SECRET=change-me-nextauth-secret\nPOSTGRES_PASSWORD=change-me-postgres-password\n",
			Notes:      "Set NEXTAUTH_SECRET (openssl rand -base64 32) and POSTGRES_PASSWORD before first boot, and NEXTAUTH_URL to http://<host>:3000/api/v1/auth. Add Meilisearch (MEILI_MASTER_KEY + MEILI_HOST) for full-text search.",
		},
		{
			ID: "memos", Name: "Memos", Description: "Lightweight, markdown-native self-hosted note-taking and microblog tool.",
			Category:    "productivity",
			Source:      "docker-hub", Image: "neosmemo/memos:stable",
			Tags: []string{"productivity", "notes", "markdown"},
			ComposeContent: `services:
  memos:
    image: neosmemo/memos:stable
    restart: unless-stopped
    ports:
      - "${MEMOS_PORT:-5230}:5230"
    environment:
      MEMOS_DRIVER: sqlite
      MEMOS_INSTANCE_URL: ${MEMOS_INSTANCE_URL:-http://localhost:5230}
    volumes:
      - memos_data:/var/opt/memos
volumes:
  memos_data:
`,
			EnvContent: "MEMOS_PORT=5230\nMEMOS_INSTANCE_URL=http://localhost:5230\n",
			Notes:      "SQLite by default in /var/opt/memos (persisted). No required secrets — create the first admin account in the UI on first run. Set MEMOS_INSTANCE_URL to your public URL for correct links.",
		},
		{
			ID: "vikunja", Name: "Vikunja", Description: "Self-hosted to-do list and project management (API + frontend in one image).",
			Category:    "productivity",
			Source:      "docker-hub", Image: "vikunja/vikunja:latest",
			Tags: []string{"productivity", "todo", "tasks", "projects"},
			ComposeContent: `services:
  vikunja:
    image: vikunja/vikunja:latest
    restart: unless-stopped
    ports:
      - "${VIKUNJA_PORT:-3456}:3456"
    environment:
      VIKUNJA_SERVICE_PUBLICURL: ${VIKUNJA_PUBLICURL:-http://localhost:3456}
      VIKUNJA_SERVICE_SECRET: ${VIKUNJA_SERVICE_SECRET}
      VIKUNJA_DATABASE_TYPE: postgres
      VIKUNJA_DATABASE_HOST: db
      VIKUNJA_DATABASE_DATABASE: ${DB_DATABASE:-vikunja}
      VIKUNJA_DATABASE_USER: ${DB_USERNAME:-vikunja}
      VIKUNJA_DATABASE_PASSWORD: ${DB_PASSWORD}
    volumes:
      - ./data/files:/app/vikunja/files
    depends_on:
      db:
        condition: service_healthy
  db:
    image: postgres:18
    restart: unless-stopped
    environment:
      POSTGRES_DB: ${DB_DATABASE:-vikunja}
      POSTGRES_USER: ${DB_USERNAME:-vikunja}
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - vikunja_db:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -h localhost -U $$POSTGRES_USER"]
      interval: 2s
      start_period: 30s
volumes:
  vikunja_db:
`,
			EnvContent: "VIKUNJA_PORT=3456\nVIKUNJA_PUBLICURL=http://localhost:3456\nVIKUNJA_SERVICE_SECRET=change-me-random-jwt-secret\nDB_DATABASE=vikunja\nDB_USERNAME=vikunja\nDB_PASSWORD=change-me-db-password\n",
			Notes:      "Single merged image serves both API and UI on 3456. Set VIKUNJA_SERVICE_PUBLICURL to your reachable URL and a strong VIKUNJA_SERVICE_SECRET (JWT signing), plus DB_PASSWORD, before first boot. Register the first user in the UI.",
		},
		{
			ID: "dawarich", Name: "Dawarich", Description: "Self-hosted location-history tracker and Google Maps Timeline replacement.",
			Category:    "productivity",
			Source:      "docker-hub", Image: "freikin/dawarich:latest",
			Tags: []string{"productivity", "location", "maps", "self-tracking"},
			ComposeContent: `services:
  dawarich_redis:
    image: redis:7.4-alpine
    command: redis-server
    restart: unless-stopped
    volumes:
      - dawarich_shared:/data
    healthcheck:
      test: ["CMD", "redis-cli", "--raw", "incr", "ping"]
      interval: 10s
      retries: 5
  dawarich_db:
    image: postgis/postgis:17-3.5-alpine
    restart: unless-stopped
    environment:
      POSTGRES_USER: "${DATABASE_USERNAME:-postgres}"
      POSTGRES_PASSWORD: "${DATABASE_PASSWORD}"
      POSTGRES_DB: "${DATABASE_NAME:-dawarich_development}"
    volumes:
      - dawarich_db_data:/var/lib/postgresql/data
      - dawarich_shared:/var/shared
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${DATABASE_USERNAME:-postgres} -d ${DATABASE_NAME:-dawarich_development}"]
      interval: 10s
      retries: 5
  dawarich_app:
    image: freikin/dawarich:latest
    entrypoint: web-entrypoint.sh
    command: ["bin/rails", "server", "-p", "3000", "-b", "0.0.0.0"]
    restart: unless-stopped
    ports:
      - "${DAWARICH_APP_PORT:-3000}:3000"
    environment:
      RAILS_ENV: production
      DATABASE_HOST: dawarich_db
      DATABASE_USERNAME: "${DATABASE_USERNAME:-postgres}"
      DATABASE_PASSWORD: "${DATABASE_PASSWORD}"
      DATABASE_NAME: "${DATABASE_NAME:-dawarich_development}"
      REDIS_URL: redis://dawarich_redis:6379/0
      SECRET_KEY_BASE: "${SECRET_KEY_BASE}"
      APPLICATION_HOSTS: "${APPLICATION_HOSTS:-localhost,127.0.0.1}"
      TIME_ZONE: "${TIME_ZONE:-Europe/London}"
    volumes:
      - dawarich_public:/var/app/public
      - dawarich_watched:/var/app/tmp/imports/watched
      - dawarich_storage:/var/app/storage
    depends_on:
      dawarich_db:
        condition: service_healthy
      dawarich_redis:
        condition: service_healthy
    healthcheck:
      test: ["CMD-SHELL", "wget -qO - http://127.0.0.1:3000/api/v1/health | grep -q ok"]
      interval: 10s
      retries: 5
  dawarich_sidekiq:
    image: freikin/dawarich:latest
    entrypoint: sidekiq-entrypoint.sh
    command: ["sidekiq"]
    restart: unless-stopped
    environment:
      RAILS_ENV: production
      DATABASE_HOST: dawarich_db
      DATABASE_USERNAME: "${DATABASE_USERNAME:-postgres}"
      DATABASE_PASSWORD: "${DATABASE_PASSWORD}"
      DATABASE_NAME: "${DATABASE_NAME:-dawarich_development}"
      REDIS_URL: redis://dawarich_redis:6379/0
      SECRET_KEY_BASE: "${SECRET_KEY_BASE}"
      APPLICATION_HOSTS: "${APPLICATION_HOSTS:-localhost,127.0.0.1}"
      TIME_ZONE: "${TIME_ZONE:-Europe/London}"
    volumes:
      - dawarich_public:/var/app/public
      - dawarich_watched:/var/app/tmp/imports/watched
      - dawarich_storage:/var/app/storage
    depends_on:
      dawarich_app:
        condition: service_healthy
    healthcheck:
      test: ["CMD-SHELL", "pgrep -f sidekiq"]
      interval: 10s
      retries: 5
volumes:
  dawarich_db_data:
  dawarich_shared:
  dawarich_public:
  dawarich_watched:
  dawarich_storage:
`,
			EnvContent: "DAWARICH_APP_PORT=3000\nDATABASE_USERNAME=postgres\nDATABASE_PASSWORD=change-me-db-password\nDATABASE_NAME=dawarich_development\nAPPLICATION_HOSTS=localhost,127.0.0.1\nTIME_ZONE=Europe/London\nSECRET_KEY_BASE=change-me-openssl-rand-hex-64\n",
			Notes:      "Set SECRET_KEY_BASE (openssl rand -hex 64), DATABASE_PASSWORD, and APPLICATION_HOSTS (your host/domain, or requests are rejected) before first boot. DB must be PostGIS. App + sidekiq share the same image/env, differing by entrypoint.",
		},
		{
			ID: "ntfy", Name: "ntfy", Description: "Self-hosted pub-sub push-notification server with web UI, REST API, and mobile apps.",
			Category:    "automation",
			Source:      "docker-hub", Image: "binwiederhier/ntfy:v2.11.0",
			Tags: []string{"automation", "notifications", "push", "api"},
			ComposeContent: `services:
  ntfy:
    image: binwiederhier/ntfy:v2.11.0
    command:
      - serve
    restart: unless-stopped
    ports:
      - "${NTFY_PORT:-8080}:80"
    environment:
      TZ: "${TZ:-UTC}"
      NTFY_BASE_URL: "${NTFY_BASE_URL}"
      NTFY_CACHE_FILE: /var/cache/ntfy/cache.db
    volumes:
      - cache:/var/cache/ntfy
      - ./ntfy-config:/etc/ntfy
volumes:
  cache:
`,
			EnvContent: "NTFY_PORT=8080\nTZ=UTC\nNTFY_BASE_URL=http://localhost:8080\n",
			Notes:      "Web UI + API both on container port 80. Set NTFY_BASE_URL to your public URL. Anonymous read/write by default; to lock down, add auth-default-access: deny-all to a server.yml in ./ntfy-config. NTFY_CACHE_FILE persists messages.",
		},
		{
			ID: "scrutiny", Name: "Scrutiny", Description: "Hard-drive SMART monitoring dashboard with historical health/temperature tracking and alerts.",
			Category:    "monitoring",
			Source:      "docker-hub", Image: "ghcr.io/analogj/scrutiny:master-omnibus",
			Tags: []string{"monitoring", "smart", "disk", "homelab"},
			ComposeContent: `services:
  scrutiny:
    image: ghcr.io/analogj/scrutiny:master-omnibus
    restart: unless-stopped
    cap_add:
      - SYS_RAWIO
    ports:
      - "${SCRUTINY_PORT:-8080}:8080"
    volumes:
      - scrutiny_config:/opt/scrutiny/config
      - scrutiny_influxdb:/opt/scrutiny/influxdb
      - /run/udev:/run/udev:ro
    devices:
      - "/dev/sda:/dev/sda"
      - "/dev/sdb:/dev/sdb"
volumes:
  scrutiny_config:
  scrutiny_influxdb:
`,
			EnvContent: "SCRUTINY_PORT=8080\n",
			Notes:      "Requires privileged host access: cap_add SYS_RAWIO (add SYS_ADMIN for NVMe) and each drive passed via devices:. EDIT the devices: list to match your host's actual drives (/dev/sda, /dev/nvme0, ...) — it cannot be fully zero-config. InfluxDB is bundled (omnibus).",
		},
		{
			ID: "speedtest-tracker", Name: "Speedtest Tracker", Description: "Scheduled internet speed-test logging with a web dashboard, history, and charts.",
			Category:    "monitoring",
			Source:      "linuxserver", Image: "lscr.io/linuxserver/speedtest-tracker:latest",
			Tags: []string{"monitoring", "speedtest", "network"},
			ComposeContent: `services:
  speedtest-tracker:
    image: lscr.io/linuxserver/speedtest-tracker:latest
    restart: unless-stopped
    ports:
      - "${SPEEDTEST_PORT:-8765}:80"
    environment:
      - PUID=1000
      - PGID=1000
      - TZ=${TZ:-Etc/UTC}
      - APP_KEY=${APP_KEY}
      - APP_URL=${APP_URL:-http://localhost:8765}
      - DB_CONNECTION=sqlite
      - SPEEDTEST_SCHEDULE=${SPEEDTEST_SCHEDULE:-}
    volumes:
      - speedtest_config:/config
volumes:
  speedtest_config:
`,
			EnvContent: "SPEEDTEST_PORT=8765\nTZ=Etc/UTC\nAPP_URL=http://localhost:8765\nSPEEDTEST_SCHEDULE=\nAPP_KEY=base64:change-me-generate-with-openssl-rand-base64-32\n",
			Notes:      "Set APP_KEY: generate with echo \"base64:$(openssl rand -base64 32)\" and paste the full base64: value before first boot. Default login admin@example.com / password (change immediately). SQLite by default; set APP_URL to your access URL.",
		},
		{
			ID: "actual-budget", Name: "Actual Budget", Description: "Self-hosted envelope/zero-based budgeting app with a fast local-first sync server.",
			Category:    "finance",
			Source:      "docker-hub", Image: "actualbudget/actual-server:latest",
			Tags: []string{"finance", "budgeting", "money"},
			ComposeContent: `services:
  actual:
    image: actualbudget/actual-server:latest
    restart: unless-stopped
    ports:
      - "${ACTUAL_PORT:-5006}:5006"
    volumes:
      - ./data:/data
`,
			EnvContent: "ACTUAL_PORT=5006\n",
			Notes:      "Single container, no external DB (data in /data). No required secret env — set a server password in the web UI on first launch. Optional HTTPS via ACTUAL_HTTPS_KEY/ACTUAL_HTTPS_CERT.",
		},
		{
			ID: "wallos", Name: "Wallos", Description: "Open-source self-hosted personal subscription and recurring-expense tracker.",
			Category:    "finance",
			Source:      "docker-hub", Image: "bellamy/wallos:latest",
			Tags: []string{"finance", "subscriptions", "budgeting"},
			ComposeContent: `services:
  wallos:
    image: bellamy/wallos:latest
    restart: unless-stopped
    ports:
      - "${WALLOS_PORT:-8282}:80"
    environment:
      TZ: ${TZ:-Etc/UTC}
    volumes:
      - ./data/db:/var/www/html/db
      - ./data/logos:/var/www/html/images/uploads/logos
`,
			EnvContent: "WALLOS_PORT=8282\nTZ=Etc/UTC\n",
			Notes:      "Bundled SQLite — no external DB. First run: open the port and register the first admin user in the UI. Persist ./data/db and ./data/logos or you lose data. OIDC/email notifications are optional.",
		},
		{
			ID: "firefly-iii", Name: "Firefly III", Description: "Self-hosted personal finance manager with double-entry accounting, budgets, and reports.",
			Category:    "finance",
			Source:      "docker-hub", Image: "fireflyiii/core:latest",
			Tags: []string{"finance", "accounting", "budgeting"},
			ComposeContent: `services:
  app:
    image: fireflyiii/core:latest
    restart: unless-stopped
    ports:
      - "${FF_PORT:-8080}:8080"
    environment:
      APP_KEY: ${APP_KEY}
      APP_URL: ${APP_URL:-http://localhost:8080}
      TZ: ${TZ:-Etc/UTC}
      DB_CONNECTION: mysql
      DB_HOST: db
      DB_PORT: 3306
      DB_DATABASE: ${DB_DATABASE:-firefly}
      DB_USERNAME: ${DB_USERNAME:-firefly}
      DB_PASSWORD: ${DB_PASSWORD}
    volumes:
      - firefly_upload:/var/www/html/storage/upload
    depends_on:
      - db
  db:
    image: mariadb:lts
    restart: unless-stopped
    environment:
      MYSQL_RANDOM_ROOT_PASSWORD: "yes"
      MYSQL_DATABASE: ${DB_DATABASE:-firefly}
      MYSQL_USER: ${DB_USERNAME:-firefly}
      MYSQL_PASSWORD: ${DB_PASSWORD}
    volumes:
      - firefly_db:/var/lib/mysql
volumes:
  firefly_upload:
  firefly_db:
`,
			EnvContent: "FF_PORT=8080\nAPP_URL=http://localhost:8080\nTZ=Etc/UTC\nAPP_KEY=change-me-32-char-app-key-000000\nDB_DATABASE=firefly\nDB_USERNAME=firefly\nDB_PASSWORD=change-me-db-password\n",
			Notes:      "APP_KEY must be EXACTLY 32 chars — generate: head /dev/urandom | LC_ALL=C tr -dc 'A-Za-z0-9' | head -c 32; echo. Keep it unchanged across upgrades (it encrypts stored data). Set APP_URL and DB_PASSWORD before first boot.",
		},

		// ---- VPN / proxy management (ARPHost products + WireGuard UIs) ----
		{
			ID: "arpvpn", Name: "ARPVPN", Description: "ARPHost's WireGuard VPN management web GUI — users, roles, live traffic graphs, and a full API.",
			Category:    "security",
			Subcategory: "vpn",
			Source:      "official-github", Image: "10.10.10.96:5050/arphost/arpvpn:v2-latest",
			Tags: []string{"security", "vpn", "wireguard", "arphost", "gui"},
			ComposeContent: `services:
  arpvpn:
    image: ${ARPVPN_IMAGE:-10.10.10.96:5050/arphost/arpvpn:v2-latest}
    user: "${ARPVPN_RUNTIME_USER:-arpvpn}"
    environment:
      ARPVPN_CONTAINER_NAME: "${ARPVPN_CONTAINER_NAME:-arpvpn}"
      ARPVPN_COOKIE_SUFFIX: "${ARPVPN_COOKIE_SUFFIX:-}"
      ARPVPN_SECURE_COOKIES: "${ARPVPN_SECURE_COOKIES:-0}"
      ARPVPN_HTTP_PORT: "${ARPVPN_HTTP_PORT:-8085}"
      ARPVPN_HTTPS_PORT: "${ARPVPN_HTTPS_PORT:-8086}"
    command:
      - /bin/bash
      - -lc
      - |
        set -euo pipefail
        if [ ! -w /data ]; then
          echo "ERROR: /data is not writable by runtime user UID:GID $(id -u):$(id -g)."
          ls -ld /data || true
          exit 1
        fi
        if ! sudo -n /usr/bin/wg genkey >/dev/null 2>&1; then
          echo "ERROR: WireGuard command preflight failed for $(id -u):$(id -g)."
          echo "Use ARPVPN_RUNTIME_USER=arpvpn, or rebuild image with matching ARPVPN_UID/ARPVPN_GID."
          exit 1
        fi
        if [ ! -f /data/uwsgi.yaml ] && [ -d /var/www/arpvpn/data ]; then
          if find /var/www/arpvpn/data -mindepth 1 -maxdepth 1 | read -r _; then
            cp -r /var/www/arpvpn/data/. /data/
          fi
        fi
        if [ -f /data/uwsgi.yaml ]; then
          sed -i -E 's|^([[:space:]]*pyargv:)[[:space:]]*data[[:space:]]*$|\1 /data|' /data/uwsgi.yaml || true
          sed -i -E "s|^([[:space:]]*http-socket:)[[:space:]]*0\\.0\\.0\\.0:[0-9]+[[:space:]]*$|\\1 0.0.0.0:$${ARPVPN_HTTP_PORT}|" /data/uwsgi.yaml || true
          sed -i -E "s|^([[:space:]]*https-socket:)[[:space:]]*0\\.0\\.0\\.0:[0-9]+(,.*)$|\\1 0.0.0.0:$${ARPVPN_HTTPS_PORT}\\2|" /data/uwsgi.yaml || true
        fi
        exec /usr/bin/uwsgi --yaml /data/uwsgi.yaml
    cap_add:
      - NET_ADMIN
      - NET_RAW
    volumes:
      - ${DATA_FOLDER:-./data}:/data
    network_mode: host
    restart: unless-stopped
`,
			EnvContent: "ARPVPN_IMAGE=10.10.10.96:5050/arphost/arpvpn:v2-latest\nARPVPN_RUNTIME_USER=arpvpn\nARPVPN_HTTP_PORT=8085\nARPVPN_HTTPS_PORT=8086\nARPVPN_SECURE_COOKIES=0\nDATA_FOLDER=./data\n",
			Notes:      "ARPHost's own WireGuard GUI. Uses host networking + NET_ADMIN/NET_RAW; web UI on ARPVPN_HTTP_PORT (8085), HTTPS on 8086. The image lives on the ARPHost GitLab registry — run docker login 10.10.10.96:5050 first (Settings > Registry Login). Config persists under DATA_FOLDER. Default login is in the project docs.",
		},
		{
			ID: "wg-easy", Name: "wg-easy", Description: "The simplest self-hosted WireGuard VPN with a clean web UI for managing peers.",
			Category:    "security",
			Subcategory: "vpn",
			Source:      "docker-hub", Image: "ghcr.io/wg-easy/wg-easy:15",
			Tags: []string{"security", "vpn", "wireguard", "gui"},
			ComposeContent: `services:
  wg-easy:
    image: ghcr.io/wg-easy/wg-easy:15
    restart: unless-stopped
    environment:
      - INIT_ENABLED=${WG_INIT_ENABLED:-false}
      - INIT_HOST=${WG_HOST:-}
      - INIT_PORT=${WG_PORT:-51820}
      - INIT_USERNAME=${WG_ADMIN_USER:-admin}
      - INIT_PASSWORD=${WG_ADMIN_PASSWORD:-}
    volumes:
      - etc_wireguard:/etc/wireguard
      - /lib/modules:/lib/modules:ro
    ports:
      - "${WG_PORT:-51820}:51820/udp"
      - "${WG_UI_PORT:-51821}:51821/tcp"
    cap_add:
      - NET_ADMIN
      - SYS_MODULE
    sysctls:
      - net.ipv4.ip_forward=1
      - net.ipv4.conf.all.src_valid_mark=1
volumes:
  etc_wireguard:
`,
			EnvContent: "WG_PORT=51820\nWG_UI_PORT=51821\nWG_INIT_ENABLED=false\nWG_HOST=vpn.example.com\nWG_ADMIN_USER=admin\nWG_ADMIN_PASSWORD=change-me-admin-pass\n",
			Notes:      "v15: by default you complete a first-run setup wizard in the web UI (WG_UI_PORT 51821) — no secret env needed. For unattended setup, set WG_INIT_ENABLED=true plus WG_HOST/WG_ADMIN_PASSWORD. Needs NET_ADMIN + SYS_MODULE and ip_forward sysctls; UDP 51820 must be reachable. Pinned to tag 15 (not latest).",
		},
		{
			ID: "wireguard-ui", Name: "WireGuard-UI", Description: "Web UI for WireGuard paired with the linuxserver/wireguard data-plane container.",
			Category:    "security",
			Subcategory: "vpn",
			Source:      "docker-hub", Image: "ngoduykhanh/wireguard-ui:latest",
			Tags: []string{"security", "vpn", "wireguard", "gui"},
			ComposeContent: `services:
  wireguard:
    image: linuxserver/wireguard:latest
    restart: unless-stopped
    cap_add:
      - NET_ADMIN
    sysctls:
      - net.ipv4.conf.all.src_valid_mark=1
      - net.ipv4.ip_forward=1
    volumes:
      - wg_config:/config
    ports:
      - "${WGUI_PORT:-5000}:5000"
      - "${WG_PORT:-51820}:51820/udp"
  wireguard-ui:
    image: ngoduykhanh/wireguard-ui:latest
    restart: unless-stopped
    depends_on:
      - wireguard
    cap_add:
      - NET_ADMIN
    network_mode: service:wireguard
    environment:
      - SESSION_SECRET=${WGUI_SESSION_SECRET}
      - WGUI_USERNAME=${WGUI_USERNAME:-admin}
      - WGUI_PASSWORD=${WGUI_PASSWORD}
      - WGUI_MANAGE_START=true
      - WGUI_MANAGE_RESTART=true
    volumes:
      - wgui_db:/app/db
      - wg_config:/etc/wireguard
volumes:
  wg_config:
  wgui_db:
`,
			EnvContent: "WGUI_PORT=5000\nWG_PORT=51820\nWGUI_USERNAME=admin\nWGUI_PASSWORD=change-me-admin-pass\nWGUI_SESSION_SECRET=change-me-openssl-rand-hex-32\n",
			Notes:      "The UI joins the wireguard container's network namespace, so BOTH the UI (WGUI_PORT 5000) and WG (51820/udp) ports are published on the wireguard service. Set WGUI_PASSWORD and SESSION_SECRET (openssl rand -hex 32) before first boot. The shared config volume is how the UI hands configs to the data plane.",
		},
		{
			ID: "headscale", Name: "Headscale", Description: "Self-hosted, open-source implementation of the Tailscale control server for your own mesh.",
			Category:    "security",
			Subcategory: "vpn",
			Source:      "docker-hub", Image: "headscale/headscale:0.29.2",
			Tags: []string{"security", "vpn", "wireguard", "tailscale", "mesh"},
			ComposeContent: `services:
  headscale:
    image: headscale/headscale:0.29.2
    restart: unless-stopped
    command: serve
    ports:
      - "${HEADSCALE_PORT:-8080}:8080"
      - "127.0.0.1:${HEADSCALE_METRICS_PORT:-9090}:9090"
    configs:
      - source: headscale_config
        target: /etc/headscale/config.yaml
    volumes:
      - headscale_data:/var/lib/headscale
  headscale-ui:
    image: ghcr.io/gurucomputing/headscale-ui:latest
    restart: unless-stopped
    ports:
      - "${HEADSCALE_UI_PORT:-8081}:80"
configs:
  headscale_config:
    content: |
      server_url: ${HEADSCALE_SERVER_URL:-http://127.0.0.1:8080}
      listen_addr: 0.0.0.0:8080
      metrics_listen_addr: 127.0.0.1:9090
      grpc_listen_addr: 0.0.0.0:50443
      grpc_allow_insecure: false
      noise:
        private_key_path: /var/lib/headscale/noise_private.key
      prefixes:
        v4: 100.64.0.0/10
        v6: fd7a:115c:a1e0::/48
        allocation: sequential
      derp:
        server:
          enabled: false
        urls:
          - https://controlplane.tailscale.com/derpmap/default
        auto_update_enabled: true
        update_frequency: 24h
      disable_check_updates: false
      ephemeral_node_inactivity_timeout: 30m
      database:
        type: sqlite
        sqlite:
          path: /var/lib/headscale/db.sqlite
      log:
        level: info
      dns:
        magic_dns: true
        base_domain: headscale.internal
        nameservers:
          global:
            - 1.1.1.1
volumes:
  headscale_data:
`,
			EnvContent: "HEADSCALE_PORT=8080\nHEADSCALE_METRICS_PORT=9090\nHEADSCALE_UI_PORT=8081\nHEADSCALE_SERVER_URL=http://127.0.0.1:8080\n",
			Notes:      "Control plane only (no NET_ADMIN). A minimal config.yaml is shipped via configs: — set HEADSCALE_SERVER_URL to the URL clients will reach (e.g. https://your-host) before deploying. Clients: tailscale up --login-server=<url>. Make a pre-auth key with: docker exec <ctr> headscale preauthkeys create --user <u>. The UI (HEADSCALE_UI_PORT 8081) needs an API key: headscale apikeys create.",
		},
		{
			ID: "pritunl", Name: "Pritunl", Description: "Enterprise OpenVPN/WireGuard server with a web admin console (community Docker image + MongoDB).",
			Category:    "security",
			Subcategory: "vpn",
			Source:      "docker-hub", Image: "ghcr.io/jippi/docker-pritunl:latest",
			Tags: []string{"security", "vpn", "openvpn", "wireguard"},
			ComposeContent: `services:
  mongo:
    image: mongo:7
    restart: unless-stopped
    volumes:
      - mongo_data:/data/db
  pritunl:
    image: ghcr.io/jippi/docker-pritunl:latest
    restart: unless-stopped
    depends_on:
      - mongo
    environment:
      - PRITUNL_MONGODB_URI=mongodb://mongo:27017/pritunl
    cap_add:
      - NET_ADMIN
    devices:
      - /dev/net/tun:/dev/net/tun
    volumes:
      - pritunl_data:/var/lib/pritunl
    ports:
      - "${PRITUNL_HTTPS_PORT:-1443}:443/tcp"
      - "${PRITUNL_HTTP_PORT:-1480}:80/tcp"
      - "${PRITUNL_VPN_PORT:-1194}:1194/udp"
      - "${PRITUNL_VPN_PORT:-1194}:1194/tcp"
volumes:
  mongo_data:
  pritunl_data:
`,
			EnvContent: "PRITUNL_HTTPS_PORT=1443\nPRITUNL_HTTP_PORT=1480\nPRITUNL_VPN_PORT=1194\n",
			Notes:      "Community image (no official Pritunl Docker image). Admin console on PRITUNL_HTTPS_PORT (1443). First run: docker exec <pritunl> pritunl setup-key (DB setup token) and docker exec <pritunl> pritunl default-password (initial admin login). Needs NET_ADMIN + /dev/net/tun.",
		},
		{
			ID: "proxyforge", Name: "ProxyForge", Description: "ARPHost's multi-tenant web admin for SOCKS5 (Dante) + HTTP CONNECT (Squid) proxies, one credential for both.",
			Category:    "proxy",
			Source:      "official-github", Image: "proxyforge-frontend:local",
			Tags: []string{"proxy", "socks5", "squid", "dante", "arphost", "multi-tenant"},
			ComposeContent: `services:
  init-data:
    image: busybox:1
    command: >-
      sh -c "mkdir -p /data /certs &&
             chown 1001:1001 /data /certs &&
             chmod 0755 /data /certs"
    user: "0:0"
    volumes:
      - ${DATA_FOLDER:-./data}:/data
      - ./certs:/certs
    restart: "no"
  backend:
    image: proxyforge-backend:local
    build:
      context: "https://github.com/arphost-com/proxyforge.git#main:backend"
      dockerfile: Dockerfile
    restart: unless-stopped
    environment:
      PROXYFORGE_JWT_SECRET: ${PROXYFORGE_JWT_SECRET}
      PROXYFORGE_BOOTSTRAP_ADMIN_EMAIL: ${PROXYFORGE_BOOTSTRAP_ADMIN_EMAIL}
      PROXYFORGE_BOOTSTRAP_ADMIN_PASSWORD: ${PROXYFORGE_BOOTSTRAP_ADMIN_PASSWORD}
      PROXYFORGE_COOKIE_SECURE: ${PROXYFORGE_COOKIE_SECURE:-false}
      PROXYFORGE_SSL_MODE: ${PROXYFORGE_SSL_MODE:-disabled}
      PROXYFORGE_DATA_DIR: /data
      PROXYFORGE_DANTE_DIR: /etc/dante
      PROXYFORGE_DANTE_PIDFILE: /etc/dante/danted.pid
      PROXYFORGE_SQUID_PIDFILE: /etc/dante/squid.pid
      PROXYFORGE_SQUID_HTTP_PORT: ${PROXYFORGE_SQUID_HTTP_PORT:-3128}
      PROXYFORGE_PUBLIC_PROXY_HOST: ${PROXYFORGE_PUBLIC_PROXY_HOST:-}
    volumes:
      - ${DATA_FOLDER:-./data}:/data
      - dante-config:/etc/dante
      - proxyforge-logs:/var/log/proxyforge:ro
      - ./certs:/etc/proxyforge/certs
      - acme-challenge:/var/www/acme
    networks:
      - proxyforge
    pid: "service:dante"
    depends_on:
      dante:
        condition: service_started
      squid:
        condition: service_started
      init-data:
        condition: service_completed_successfully
  dante:
    image: proxyforge-dante:local
    build:
      context: "https://github.com/arphost-com/proxyforge.git#main:dante"
      dockerfile: Dockerfile
    restart: unless-stopped
    ports:
      - "${PROXYFORGE_SOCKS_PORT:-1080}:1080"
    volumes:
      - dante-config:/etc/dante
    networks:
      - proxyforge
  squid:
    image: proxyforge-squid:local
    build:
      context: "https://github.com/arphost-com/proxyforge.git#main:squid"
      dockerfile: Dockerfile
    restart: unless-stopped
    ports:
      - "${PROXYFORGE_SQUID_HTTP_PORT:-3128}:3128"
    volumes:
      - dante-config:/etc/dante
      - proxyforge-logs:/var/log/proxyforge
    networks:
      - proxyforge
    pid: "service:dante"
    depends_on:
      dante:
        condition: service_started
  frontend:
    image: proxyforge-frontend:local
    build:
      context: "https://github.com/arphost-com/proxyforge.git#main:frontend"
      dockerfile: Dockerfile
    restart: unless-stopped
    environment:
      PROXYFORGE_SSL_MODE: ${PROXYFORGE_SSL_MODE:-disabled}
    ports:
      - "${PROXYFORGE_HTTP_PORT:-8088}:8080"
      - "${PROXYFORGE_HTTPS_PORT:-8443}:8443"
    volumes:
      - ./certs:/etc/proxyforge/certs
      - acme-challenge:/var/www/acme
    networks:
      - proxyforge
    depends_on:
      backend:
        condition: service_healthy
      init-data:
        condition: service_completed_successfully
volumes:
  dante-config:
  proxyforge-logs:
  acme-challenge:
networks:
  proxyforge:
    driver: bridge
`,
			EnvContent: "PROXYFORGE_HTTP_PORT=8088\nPROXYFORGE_HTTPS_PORT=8443\nPROXYFORGE_SOCKS_PORT=1080\nPROXYFORGE_SQUID_HTTP_PORT=3128\nPROXYFORGE_SSL_MODE=disabled\nPROXYFORGE_PUBLIC_PROXY_HOST=\nDATA_FOLDER=./data\nPROXYFORGE_JWT_SECRET=change-me-openssl-rand-hex-32\nPROXYFORGE_BOOTSTRAP_ADMIN_EMAIL=admin@example.com\nPROXYFORGE_BOOTSTRAP_ADMIN_PASSWORD=change-me-on-first-login\n",
			Notes:      "ARPHost product. No published images — Compose builds the 4 images from the public GitHub repo on first up (needs build tools + internet). Set PROXYFORGE_JWT_SECRET (openssl rand -hex 32) and the bootstrap admin email/password before deploy. Web admin on PROXYFORGE_HTTP_PORT (8088); SOCKS5 on 1080, HTTP-CONNECT on 3128.",
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
