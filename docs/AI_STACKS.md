# AI Stack Catalog Notes

Stack Manager includes AI templates only when the upstream project has credible ownership, visible use, and a clear self-hosting path. These templates are starters: open the template, review `compose.yml` and `.env`, set secrets, then create the project.

## Docker Compose Basics

Stack Manager writes each stack as a normal Docker Compose project. Compose defines services, networks, and volumes in one YAML file and uses commands such as `docker compose up -d`, `docker compose ps`, `docker compose logs`, and `docker compose down` to manage lifecycle. Official docs: https://docs.docker.com/compose/

GPU access is configured per service in Compose with `deploy.resources.reservations.devices` and `capabilities: [gpu]`. Docker requires `capabilities` for GPU reservations, and `count` or `device_ids` can target a specific GPU. Official GPU docs: https://docs.docker.com/compose/how-tos/gpu-support/

You don't have to write that block by hand: **Settings > GPU** detects the host GPU and **Run GPU test** launches a throwaway `--gpus all` container running `nvidia-smi` to confirm the full passthrough chain (driver + nvidia-container-toolkit + runtime) works. When you open an AI stack template from the catalog with a GPU present, the **Add GPU passthrough** checkbox injects (or removes) the device block for you.

On a one-GPU server, do not run multiple heavy AI projects at the same time if they each expect GPU inference. Ollama, vLLM, image generation, voice models, Onyx model services, and similar workloads will compete for the same VRAM. Start one GPU-heavy stack, verify it, shut it down, then start the next unless the GPU has enough memory and the templates are explicitly pinned to separate devices.

## Vetted AI Additions

| Stack | Template ID | Best use | Why it made the cut |
| --- | --- | --- | --- |
| LibreChat | `librechat` | Multi-user AI chat, agents, files, MCP, multi-provider routing | Large active upstream, official self-hosting compose, broad provider support |
| Onyx | `onyx` | Enterprise knowledge search and RAG with connectors | Mature self-hosted platform, official Compose deployment, connector/search focus |
| Khoj | `khoj` | Personal/team second brain, web/doc answers, automations | Active project, official Docker Compose, strong personal-agent fit |
| DocsGPT | `docsgpt` | Private document assistants and enterprise search | Established repo, official hub compose, document-first UX |
| OpenMemory + Mem0 | `openmemory-mem0` | Shared memory layer for agents and MCP clients | High-signal memory project, self-hosted OpenMemory stack, Qdrant storage |
| Langfuse | `langfuse` | LLM observability, prompts, metrics, datasets, evals | Production-grade observability project with official self-host compose |
| Arize Phoenix | `phoenix` | Lightweight AI tracing, experiments, datasets, evaluations | Strong observability/eval project, simple container deployment |
| promptfoo | `promptfoo` | Prompt/RAG/agent evals and red-team testing | Official self-host image, widely used eval/security workflow |
| Firecrawl | `firecrawl` | Web search, scrape, crawl, and agent web context API | Large active project, official self-host docs, good agent data source |
| Crawl4AI | `crawl4ai` | Local crawler/scraper API for Markdown and RAG extraction | Active crawler project with hardened Docker API server |

## Project-Specific How-To

### LibreChat (`librechat`)

Use LibreChat when the customer wants a shared ChatGPT-style app with OpenAI, Anthropic, local OpenAI-compatible providers, agents, MCP, file upload, search, and admin controls.

1. Open Stack Catalog > AI > Workflow / RAG > LibreChat.
2. Set `MEILI_MASTER_KEY`, `JWT_SECRET`, `JWT_REFRESH_SECRET`, `CREDS_KEY`, `CREDS_IV`, `ADMIN_PANEL_SESSION_SECRET`, and `LIBRECHAT_VECTOR_DB_PASSWORD`.
3. Add at least one provider key such as `OPENAI_API_KEY` or `ANTHROPIC_API_KEY`, or configure a local OpenAI-compatible endpoint after first boot.
4. Start the stack, open `LIBRECHAT_PORT`, then open the admin panel on `LIBRECHAT_ADMIN_PORT` for administration.
5. Put it behind HTTPS before users upload files or create agents.

Sources: https://github.com/danny-avila/LibreChat and https://docs.librechat.ai/

### Onyx (`onyx`)

Use Onyx for enterprise search/RAG where connectors, document indexing, and workplace knowledge search matter more than a simple chat UI.

1. Open Stack Catalog > AI > Workflow / RAG > Onyx.
2. Set `ONYX_POSTGRES_PASSWORD`, `ONYX_OPENSEARCH_PASSWORD`, `ONYX_S3_ACCESS_KEY`, and `ONYX_S3_SECRET_KEY`.
3. Allocate enough memory for OpenSearch and the model services before indexing large document sources.
4. Start the stack and open `ONYX_PORT`.
5. Use its admin/settings flow to configure auth and connectors before production exposure.

Sources: https://github.com/onyx-dot-app/onyx and https://docs.onyx.app/

### Khoj (`khoj`)

Use Khoj as a personal or small-team AI second brain with docs, web answers, custom agents, scheduled automations, and local/cloud LLM support.

1. Open Stack Catalog > AI > Personal AI agents > Khoj.
2. Set `KHOJ_POSTGRES_PASSWORD`, `KHOJ_DJANGO_SECRET_KEY`, `KHOJ_ADMIN_EMAIL`, and `KHOJ_ADMIN_PASSWORD`.
3. Add a provider key or set `OPENAI_BASE_URL` and `KHOJ_DEFAULT_CHAT_MODEL` for a local OpenAI-compatible model server.
4. Start the stack and open `KHOJ_PORT`.
5. Keep `KHOJ_DOMAIN`/reverse-proxy settings aligned if exposing it outside localhost.

Sources: https://github.com/khoj-ai/khoj and https://docs.khoj.dev/

### DocsGPT (`docsgpt`)

Use DocsGPT for document Q&A, internal support assistants, and private search over uploaded documentation.

1. Open Stack Catalog > AI > Workflow / RAG > DocsGPT.
2. Set `DOCSGPT_POSTGRES_PASSWORD`.
3. Set `OPENAI_API_KEY` or another supported provider key before expecting answer generation.
4. Start the stack, open `DOCSGPT_FRONTEND_PORT`, and keep `DOCSGPT_API_PUBLIC_URL` aligned with the API URL users can reach.
5. Pin a known-good image tag before production; the upstream hub compose currently uses `develop` images.

Sources: https://github.com/arc53/DocsGPT

### OpenMemory + Mem0 (`openmemory-mem0`)

Use OpenMemory/Mem0 when agents need a durable user/session/agent memory API that can be reused across tools.

1. Open Stack Catalog > AI > Workflow / RAG > OpenMemory + Mem0.
2. Set `OPENMEMORY_USER` and `OPENMEMORY_API_KEY`.
3. Add `OPENAI_API_KEY` if your memory extraction path uses OpenAI models.
4. Start the stack, open `OPENMEMORY_UI_PORT`, and point MCP clients at `OPENMEMORY_API_PORT`.
5. Keep the API private or add reverse-proxy auth; memory APIs can expose sensitive personal or customer context.

Sources: https://github.com/mem0ai/mem0 and https://docs.mem0.ai/

### Langfuse (`langfuse`)

Use Langfuse for LLM tracing, prompt management, metrics, datasets, playgrounds, and eval workflows across production AI apps.

1. Open Stack Catalog > AI > Observability > Langfuse.
2. Generate and set `NEXTAUTH_SECRET`, `LANGFUSE_SALT`, `LANGFUSE_ENCRYPTION_KEY`, `LANGFUSE_POSTGRES_PASSWORD`, `LANGFUSE_REDIS_PASSWORD`, `LANGFUSE_CLICKHOUSE_PASSWORD`, `LANGFUSE_MINIO_USER`, and `LANGFUSE_MINIO_PASSWORD`.
3. Start the stack and open `LANGFUSE_PORT`.
4. Create an org/project and copy the public/secret keys into apps that should report traces.
5. Give ClickHouse and Postgres persistent disk and backups before using it for production telemetry.

Sources: https://github.com/langfuse/langfuse and https://langfuse.com/docs

### Arize Phoenix (`phoenix`)

Use Phoenix when you want a lighter observability/eval server for OpenTelemetry traces, experiments, datasets, prompt work, and troubleshooting.

1. Open Stack Catalog > AI > Observability > Arize Phoenix.
2. Set `PHOENIX_POSTGRES_PASSWORD`.
3. Start the stack, open `PHOENIX_PORT`, and point app instrumentation at `PHOENIX_OTLP_GRPC_PORT`.
4. Use Phoenix datasets and experiments to compare prompt or retrieval changes.
5. Put the UI behind auth if it leaves a trusted network.

Sources: https://github.com/Arize-ai/phoenix and https://phoenix.arize.com/

### promptfoo (`promptfoo`)

Use promptfoo for repeatable prompt, RAG, agent, and model tests, including red-team cases and regression checks.

1. Open Stack Catalog > AI > Evals / testing > promptfoo.
2. Set provider keys such as `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, or `GOOGLE_API_KEY` if you want evals from the web UI.
3. Start the stack and open `PROMPTFOO_PORT`.
4. Persist eval data in the included Docker volume.
5. The upstream self-hosted open image has no built-in auth and is intended for individual/experimental use, so place it behind access control.

Sources: https://github.com/promptfoo/promptfoo and https://www.promptfoo.dev/docs/usage/self-hosting/

### Firecrawl (`firecrawl`)

Use Firecrawl when agents need web search, page scraping, crawling, extraction, screenshots, or clean Markdown from websites.

1. Open Stack Catalog > AI > Search > Firecrawl.
2. Set `FIRECRAWL_POSTGRES_PASSWORD` and `BULL_AUTH_KEY`.
3. Add `OPENAI_API_KEY`, `OPENAI_BASE_URL`, `OLLAMA_BASE_URL`, and model values only if using AI extraction features.
4. Start the stack and call the API on `FIRECRAWL_PORT`.
5. Self-hosting does not include Firecrawl cloud's advanced anti-blocking layer, so use proxies and rate limits where needed.

Sources: https://github.com/firecrawl/firecrawl and https://github.com/firecrawl/firecrawl/blob/main/SELF_HOST.md

### Crawl4AI (`crawl4ai`)

Use Crawl4AI for a local crawler/scraper API that emits LLM-ready Markdown and supports RAG extraction workflows.

1. Open Stack Catalog > AI > Search > Crawl4AI.
2. Set `CRAWL4AI_API_TOKEN`.
3. Add provider keys only for LLM extraction features.
4. Start the stack and call the API on `CRAWL4AI_PORT`.
5. Keep the API token enabled and avoid exposing the API directly to the public internet.

Sources: https://github.com/unclecode/crawl4ai and https://docs.crawl4ai.com/
