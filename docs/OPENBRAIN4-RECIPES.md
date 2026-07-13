# OpenBrain #4 — Recipes

Practical workflows on the OpenBrain #4 voice stack. The headline one first:
**audio file → text → rewrite → new audio.**

Replace `HOST` with your server IP. Ports are the OpenBrain #4 defaults:

| Piece | External (from your laptop) | Internal (from n8n/Flowise) |
|---|---|---|
| Whisper (STT) | `http://HOST:8001/v1` | `http://whisper:8000/v1` |
| Ollama (LLM) | `http://HOST:11434` | `http://ollama:11434` |
| Kokoro (TTS, fast) | `http://HOST:8880/v1` | `http://kokoro:8880/v1` |
| openedai-speech (TTS + cloning) | `http://HOST:8003/v1` | `http://openedai-speech:8000/v1` |

All three speak the **OpenAI-compatible** API, so any tool that talks to OpenAI works.

---

## Recipe 1 — Transcribe → rewrite → re-record

### A. One command (a shell script)

Save this as `revoice.sh`, `chmod +x revoice.sh`, run `./revoice.sh input.mp3`.

```bash
#!/usr/bin/env bash
# Usage: ./revoice.sh input.mp3 [output.mp3]
set -euo pipefail
HOST="${HOST:-127.0.0.1}"
IN="$1"; OUT="${2:-rewritten.mp3}"
STYLE="${STYLE:-Rewrite this clearly and professionally, keep the meaning, fix grammar and filler words}"
VOICE="${VOICE:-af_bella}"

echo "1/3 transcribing $IN ..."
TEXT=$(curl -s -F "file=@${IN}" -F "model=Systran/faster-whisper-small" \
  "http://${HOST}:8001/v1/audio/transcriptions" | python3 -c 'import json,sys;print(json.load(sys.stdin)["text"])')
echo "   → $TEXT"

echo "2/3 rewriting ..."
REWRITE=$(curl -s "http://${HOST}:11434/api/generate" -d "$(python3 -c '
import json,sys
print(json.dumps({"model":"llama3.1","stream":False,
 "prompt": sys.argv[1] + ":\n\n" + sys.argv[2]}))' "$STYLE" "$TEXT")" \
  | python3 -c 'import json,sys;print(json.load(sys.stdin)["response"].strip())')
echo "   → $REWRITE"

echo "3/3 synthesizing $OUT ..."
curl -s "http://${HOST}:8880/v1/audio/speech" \
  -H "Content-Type: application/json" \
  -d "$(python3 -c 'import json,sys;print(json.dumps({"model":"kokoro","voice":sys.argv[1],"input":sys.argv[2],"response_format":"mp3"}))' "$VOICE" "$REWRITE")" \
  --output "$OUT"
echo "Done → $OUT"
```

Tweak with env vars, e.g.
`STYLE="Rewrite as a friendly 30-second radio ad" VOICE="am_adam" ./revoice.sh clip.wav ad.mp3`.

### B. The three raw calls (if you want to see each step)

```bash
# 1) audio -> text
curl -s -F file=@input.mp3 -F model=Systran/faster-whisper-small \
  http://HOST:8001/v1/audio/transcriptions
# -> {"text":"...the transcript..."}

# 2) text -> rewritten text (llama3.1)
curl -s http://HOST:11434/api/generate -d '{
  "model":"llama3.1","stream":false,
  "prompt":"Rewrite this clearly and fix grammar:\n\n<paste transcript>"
}'
# -> {"response":"...rewritten text..."}

# 3) text -> new audio (Kokoro), saved to out.mp3
curl -s http://HOST:8880/v1/audio/speech \
  -H "Content-Type: application/json" \
  -d '{"model":"kokoro","voice":"af_bella","input":"<paste rewritten text>","response_format":"mp3"}' \
  --output out.mp3
```

Voices: `af_bella`, `af_sky`, `af_nicole`, `am_adam`, `am_michael`, … (Kokoro's voice list).

### C. In Open WebUI (no terminal)

1. Start a chat, click **+** and **upload the audio file** — Open WebUI transcribes it (Whisper) into the message.
2. Type your instruction, e.g. *"Rewrite the transcript above as clean meeting notes."*
3. Click the **speaker** icon on the reply to hear it (Kokoro), or **⋯ → download audio** to save the new recording.

### D. Automated in n8n (reusable pipeline)

Build once, reuse for every file. Nodes:

1. **Trigger** — *Local File Trigger* (watch a folder) or *Webhook* (POST a file).
2. **HTTP Request → Whisper**: `POST http://whisper:8000/v1/audio/transcriptions`, body type *n8n binary file* field `file`, plus `model=Systran/faster-whisper-small`. Take `{{$json.text}}`.
3. **HTTP Request → Ollama**: `POST http://ollama:11434/api/generate`, JSON `{"model":"llama3.1","stream":false,"prompt":"Rewrite clearly:\n\n{{$json.text}}"}`. Take `{{$json.response}}`.
4. **HTTP Request → Kokoro**: `POST http://kokoro:8880/v1/audio/speech`, JSON `{"model":"kokoro","voice":"af_bella","input":"{{$json.response}}","response_format":"mp3"}`, response format **File**.
5. **Write Binary File** (or return via the webhook) → your new `.mp3`.

Use the **internal** DNS names above — n8n runs inside the same Docker network.

---

## Recipe 2 — Clone a voice from an MP3 (openedai-speech / XTTS)

**Kokoro can't clone voices** — it has a fixed set of ~50 built-in voices. OpenBrain
#4 now also ships **openedai-speech** (`OPENEDAI_PORT`, default `8003`), an
OpenAI-compatible TTS that wraps **XTTS-v2** and *can* clone a voice from a short
sample. Keep Kokoro as the fast default; reach for openedai-speech when you want a
cloned voice.

> ⚠️ Only clone voices you have permission to use. XTTS is CPU-slow — enable **GPU
> passthrough** on this project for usable speed.

### A. Add your reference voice

1. Get a **clean 10-30s WAV** of the target voice (one speaker, no music/noise).
   Convert an mp3 if needed: `ffmpeg -i sample.mp3 -ar 24000 -ac 1 myvoice.wav`.
2. Copy it into the **`openedai-voices`** volume, e.g. from the host:
   ```bash
   # find the volume mountpoint, then drop the wav in
   docker cp myvoice.wav openbrain-voice-openedai-speech-1:/app/voices/myvoice.wav
   ```
3. Register it as a custom voice. Edit **`/app/config/voice_to_speaker.yaml`** in
   the `openedai-config` volume (create it if missing) and add:
   ```yaml
   tts-1-hd:
     myvoice:
       model: xtts_v2.0.2
       speaker: /app/voices/myvoice.wav
   ```
4. Restart the service: `docker restart openbrain-voice-openedai-speech-1`.

### B. Use it — curl

```bash
curl -s http://HOST:8003/v1/audio/speech \
  -H "Content-Type: application/json" \
  -d '{"model":"tts-1-hd","voice":"myvoice","input":"Hello, this is my cloned voice.","response_format":"mp3"}' \
  --output cloned.mp3
```

### C. Use it — in Open WebUI

**Settings → Audio → TTS**: engine **OpenAI**, base URL
`http://openedai-speech:8000/v1` (internal) or `http://HOST:8003/v1`, model
**`tts-1-hd`**, voice **`myvoice`**. Now the speaker button reads replies in the
cloned voice. Switch the voice back to a Kokoro one (base URL
`http://kokoro:8880/v1`, model `kokoro`) any time you want the fast default.

### D. Clone + re-record (drop-in for Recipe 1)

To make Recipe 1's `revoice.sh` output in the cloned voice, point step 3 at
openedai-speech instead of Kokoro:

```bash
TTS_URL="http://HOST:8003/v1/audio/speech" TTS_MODEL="tts-1-hd" VOICE="myvoice" ./revoice.sh input.mp3 out.mp3
```
(adjust `revoice.sh` step 3 to use `$TTS_URL`/`$TTS_MODEL` — same JSON body).

**Want just the cloning TTS, no full stack?** Deploy the standalone
**openedai-speech (TTS + voice cloning)** template from the Stack Catalog.

---

## More ideas (same three building blocks + search + memory)

The stack gives you STT (Whisper), an LLM (llama3.1 / codellama), TTS (Kokoro),
web search (SearXNG), vectors (Qdrant), memory (mem0), and glue (n8n/Flowise).
Mix and match:

1. **Meeting notes + spoken recap** — recording → transcript → LLM ("summary +
   action items with owners") → save the notes → optional Kokoro recap you can
   listen to on the commute.
2. **Translate a recording** — audio → text → LLM ("translate to Spanish") →
   Kokoro. Instant dubbed voiceover in another language.
3. **Article → audiobook** — paste/scrape text → LLM ("split into natural
   paragraphs") → loop each chunk through Kokoro → concatenate the mp3s with
   `ffmpeg`. Turn any blog post or PDF into a podcast.
4. **Voice web-research** — speak a question (Whisper) → SearXNG web search →
   LLM answers with citations → Kokoro reads it back. Hands-free "what's the
   latest on X?".
5. **Voice journal with memory** — daily voice note → transcript → store in
   **mem0** → later ask "what did I say about the budget last week?" and it
   recalls across entries.
6. **Searchable recording archive (RAG)** — batch-transcribe a folder of calls →
   embed the transcripts in **Qdrant** (Open WebUI Knowledge) → ask questions
   across *all* of them, grounded, with sources.
7. **Podcast / ad producer** — outline → LLM writes the script → assign
   different Kokoro voices per speaker → stitch into a two-voice episode.
8. **Code by voice** — dictate intent (Whisper) → **codellama** → get a function.
   Great for hands-busy moments; pair with Flowise for a repeatable agent.
9. **Clean-up dictation** — ramble into the mic → LLM strips filler/"um", fixes
   grammar, formats as an email or ticket → send via an n8n step.
10. **Content repurposing** — one recording → n8n fans it out to: a summary, a
    tweet thread, show-notes, *and* a re-recorded intro — in one run.

Every one of these is the same pattern: **Whisper in → LLM in the middle →
Kokoro out**, optionally with SearXNG (facts), Qdrant (your docs), or mem0
(your history) bolted on. n8n or Flowise turn any of them into a one-click or
scheduled pipeline.

---

## Tips

- **Better transcripts:** set `WHISPER_MODEL=Systran/faster-whisper-medium` (or
  `-large-v3`) in the project `.env` for accuracy (slower / more RAM).
- **Faster LLM:** enable **GPU passthrough** on Ollama (Settings → GPU) — this is
  the biggest speed lever for the rewrite step.
- **Concatenate audio:** `ffmpeg -f concat -safe 0 -i list.txt -c copy full.mp3`.
- **Long text → TTS:** split into ≤ ~1–2k character chunks before Kokoro, then
  concatenate; very long single requests can time out.
- **From inside the network** (n8n/Flowise) always use `whisper:8000`,
  `ollama:11434`, `kokoro:8880`; from your laptop use `HOST:8001/11434/8880`.
