# OpenBrain #4 — Configure it once it's online

A start-to-finish setup guide for the **OpenBrain #4 — Voice Assistant** stack
(Open WebUI + Ollama + Whisper STT + Kokoro TTS + SearXNG + Qdrant + Postgres +
mem0/OpenMemory + Flowise + n8n + Piper).

Deploy it from **Stack Catalog → search "openbrain" → OpenBrain #4**. Everything
below is what to do *after* it's running. Replace `HOST` with your server's IP.

---

## 0. Before anything — let it warm up

First boot is heavy. The `ollama-init` sidecar pulls a few GB of models
(`llama3.1` + `codellama` + `nomic-embed-text`), and the images themselves are
large. Give it 5–20 minutes depending on bandwidth.

Watch progress from the project's **Logs** tab (or on the host):

```bash
docker compose logs -f ollama-init      # model pulls; exits 0 when done
docker compose ps                        # everything should be "Up"/"healthy"
```

Chat and code won't work until `ollama-init` has finished pulling.

> **GPU:** if you deployed on a GPU host, Ollama already has passthrough (AI
> templates enable it automatically). Verify with `docker exec <ollama> nvidia-smi`.
> Whisper/Kokoro run on CPU by design.

---

## Port map

| Service | URL | What it is |
|---|---|---|
| **Open WebUI** | `http://HOST:8080` | The hub — chat, voice, RAG, web search |
| Ollama | `http://HOST:11434` | LLM runtime (API) |
| Whisper (STT) | `http://HOST:8001` | Speech-to-text **API** — no homepage (use via Open WebUI; docs at `/docs`) |
| Kokoro (TTS) | `http://HOST:8880` | Text-to-speech **API** — no homepage (use via Open WebUI; try `/web` and `/docs`) |
| SearXNG | `http://HOST:8181` | Private web search |
| Qdrant | `http://HOST:6333` | Vector DB (RAG + memory) |
| Flowise | `http://HOST:3001` | Visual agent/flow builder |
| n8n | `http://HOST:5678` | Workflow automation |
| OpenMemory (mem0) | `http://HOST:8765` | Persistent memory API/UI |
| Piper | `HOST:10200` | Bonus TTS (Wyoming protocol) |

You only *log into* Open WebUI, Flowise, and n8n. The rest are wired internally
by Docker DNS (`http://ollama:11434`, `http://qdrant:6333`, …).

> **"Not Found" when I click a port?** That's normal — **Kokoro, Whisper,
> Ollama, and Qdrant are APIs, not websites.** Opening `http://HOST:8880/`
> returns `{"detail":"Not Found"}` because there's no page at the root; the TTS
> lives at `/v1/audio/speech`. **You don't open these directly — Open WebUI uses
> them for you** (mic = Whisper, speaker = Kokoro). To poke at them anyway:
> Kokoro has a voice **playground at `http://HOST:8880/web`** and API docs at
> `/docs`; Whisper's docs are at `http://HOST:8001/docs`. Only **Open WebUI**,
> **Flowise**, and **n8n** are real web UIs you open in a browser.

---

## 1. Change the secrets first

Before exposing this anywhere, edit the project's **`.env`** (Config tab) and set
real values, then restart:

- `POSTGRES_PASSWORD`
- `WEBUI_SECRET_KEY`
- `FLOWISE_PASSWORD`

Never put this stack on the public internet without a reverse proxy + auth (use
**Settings → Reverse Proxy** for HTTPS).

---

## 2. Open WebUI — the hub (`http://HOST:8080`)

1. Open it and **create the first account** — the first user becomes the admin.
2. Top-left model menu should list `llama3.1` and `codellama` (once
   `ollama-init` finished). If empty, wait for the pulls or pull manually:
   `docker exec <ollama-container> ollama pull llama3.1`.
3. Pick **`llama3.1`** for general chat, **`codellama`** for coding.

Everything below (voice, search, RAG) is already wired via environment variables
— you're just turning the switches on in the UI.

---

## 3. Voice — talk and listen

**Speech-to-text (your voice → text)** is pre-wired to Whisper, and
**text-to-speech (replies read aloud)** to Kokoro.

- **Talk to it:** click the **microphone** icon in the chat box, speak, stop —
  Whisper transcribes it into the prompt.
- **Hear replies:** click the **speaker** icon on a reply, or turn on
  auto-playback in **Settings → Audio**. Voice is Kokoro's `af_bella` by default
  (change `TTS_VOICE` in `.env`, e.g. `am_adam`, `af_sky`).
- **Full call mode:** the headphone/call button gives a hands-free
  listen-and-respond loop.

If STT/TTS don't respond, confirm the `whisper` and `kokoro` containers are
healthy (they download their models on first use too).

---

## 4. Web search (the one manual step)

SearXNG ships **without JSON output**, which Open WebUI needs. Enable it once:

1. On the host, edit `searxng-data`'s `settings.yml` (or use the project's file
   tools) and under `search:` add `json` to `formats`:

   ```yaml
   search:
     formats:
       - html
       - json
   ```

2. Restart SearXNG: `docker compose restart searxng`.
3. In Open WebUI, open a chat's **+ / tools** and toggle **Web Search** on
   (it's already pointed at `http://searxng:8080`). Ask something current and it
   will cite web results.

---

## 5. Documents & RAG (automatic)

RAG uses **Qdrant** for vectors and **Ollama** (`nomic-embed-text`) for
embeddings — no setup. In a chat, click **+** → upload a file (or add a
Knowledge collection under **Workspace → Knowledge**), then reference it with
`#` in your prompt. Answers are grounded in your docs.

---

## 6. Persistent memory — mem0 / OpenMemory (`http://HOST:8765`)

OpenMemory stores long-term memories in Qdrant. It needs an LLM to extract
memories — pick one:

- **Local (free):** in the OpenMemory UI **Settings**, set the LLM + embedder to
  the local Ollama, base URL `http://ollama:11434` (already reachable in the
  network), model `llama3.1`.
- **OpenAI:** set `OPENAI_API_KEY` in the project's `.env` and restart `mem0`.

To have chats *use* memory, connect OpenMemory to Open WebUI as an MCP/tool (or
call its API from n8n/Flowise).

---

## 7. Flowise — build agents (`http://HOST:3001`)

1. Log in with `FLOWISE_USERNAME` / `FLOWISE_PASSWORD` (from `.env`).
2. In a chatflow, add a **ChatOllama** node → Base URL `http://ollama:11434`,
   model `llama3.1` (or `codellama`).
3. For RAG, add a **Qdrant** vector store node → `http://qdrant:6333`.
4. Deploy the chatflow; Flowise exposes an OpenAI-compatible prediction endpoint
   you can also add to Open WebUI as a custom model connection.

---

## 8. n8n — automate general tasks (`http://HOST:5678`)

1. Create the owner account on first visit (it persists to the shared Postgres).
2. Build workflows that hit the local services over Docker DNS:
   `http://ollama:11434` (LLM), `http://searxng:8080/search?q=...&format=json`
   (search), `http://qdrant:6333` (vectors), the OpenMemory API, or Flowise.
3. Triggers (webhooks, schedules, chat) let it run tasks on your behalf.

---

## 9. Piper (optional bonus TTS)

`piper` speaks the **Wyoming** protocol on `HOST:10200` — not HTTP, so Open WebUI
uses Kokoro instead. Piper is there for **Home Assistant** voice pipelines or
n8n/Wyoming flows. Change the voice with `PIPER_VOICE` in `.env`.

---

## Quick end-to-end test

1. Open `http://HOST:8080`, sign in, pick `llama3.1`.
2. Click the **mic**, say "what's the capital of France," stop → it transcribes,
   answers, and (speaker on) reads it back. ✅ voice in + out.
3. Toggle **Web Search**, ask "top AI news today" → cited web results. ✅ search.
4. Switch to `codellama`, ask for a Python function → code. ✅ code.
5. Upload a PDF, ask about it → grounded answer. ✅ RAG/memory.

---

## Troubleshooting

- **No models in the menu / "model not found":** `ollama-init` is still pulling,
  or failed — check `docker compose logs ollama-init`, or pull manually.
- **Voice does nothing:** `whisper`/`kokoro` still downloading their models, or
  the browser blocked mic access (needs HTTPS or `localhost`; put it behind the
  reverse proxy for mic access from other machines).
- **Web search returns nothing:** the SearXNG JSON step (section 4) isn't done,
  or you didn't restart searxng.
- **Memory not saving:** OpenMemory has no LLM configured — do section 6.
- **Slow chat:** no GPU on Ollama — enable **GPU passthrough** (Settings → GPU +
  the Enable GPU action) or pick a smaller model.
- **It's a big stack:** give the host plenty of RAM (16 GB+) and a GPU for real
  speed. Run one heavy AI stack per GPU at a time.
