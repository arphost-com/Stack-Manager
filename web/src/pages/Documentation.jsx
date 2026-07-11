import { useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { projects, stackTemplates } from '../api/client';

const CATEGORY_LABELS = {
  ai: 'AI',
  automation: 'Automation',
  cms: 'CMS',
  database: 'Database',
  devtools: 'Dev Tools',
  docs: 'Docs',
  files: 'Files',
  management: 'Management',
  media: 'Media',
  monitoring: 'Monitoring',
  proxy: 'Proxy',
  queue: 'Queue',
  security: 'Security',
  web: 'Web',
};

const CATEGORY_DESCRIPTIONS = {
  all: 'Every built-in stack. Search by app, image, category, tag, environment key, or setup note.',
  ai: 'AI/ML stacks: LLM inference, image and voice generation, vector databases, RAG workflows, search, code assistants, and personal agent gateways.',
  automation: 'Task automation, workflow engines, cron schedulers, and notification systems.',
  cms: 'Content management systems, blog platforms, and e-commerce storefronts.',
  database: 'SQL, NoSQL, and graph databases with persistent volumes ready to be shared with other stacks.',
  devtools: 'Developer tools: CI/CD servers, Git forges, in-browser IDEs, code intelligence, and diagram editors.',
  docs: 'Wikis, technical documentation, and knowledge bases.',
  files: 'File sync, share, and object storage servers.',
  management: 'Docker and infrastructure management dashboards.',
  media: 'Media servers, libraries, and download automation.',
  monitoring: 'Metrics, uptime, logging, and observability.',
  proxy: 'Reverse proxies, load balancers, and forward proxies.',
  queue: 'Message queues, brokers, and workflow orchestrators.',
  security: 'Authentication, SSO, VPNs, and security scanners.',
  web: 'Web servers and static hosting.',
};

const CATEGORY_ORDER = ['ai', 'web', 'proxy', 'cms', 'database', 'devtools', 'docs', 'files', 'management', 'media', 'monitoring', 'queue', 'security', 'automation'];
const PAGE_SIZES = [20, 40, 80, 199];

const enhancedGuides = {
  librechat: {
    fit: 'Multi-user AI chat, agents, MCP, files, and multi-provider routing.',
    setup: [
      'Set MEILI_MASTER_KEY, JWT secrets, credential encryption values, admin panel secret, and vector DB password.',
      'Add at least one provider key or configure a local OpenAI-compatible endpoint after first boot.',
      'Open LIBRECHAT_PORT for users and LIBRECHAT_ADMIN_PORT for administration.',
    ],
    caution: 'Put it behind HTTPS before uploads, agents, or multi-user use.',
    links: [
      ['GitHub', 'https://github.com/danny-avila/LibreChat'],
      ['Docs', 'https://docs.librechat.ai/'],
    ],
  },
  onyx: {
    fit: 'Enterprise knowledge search and RAG with connectors.',
    setup: [
      'Set Postgres, OpenSearch, and S3/MinIO secrets before launch.',
      'Start with enough memory for OpenSearch and the model services.',
      'Configure auth and connectors in the app before indexing broad sources.',
    ],
    caution: 'This is one of the heavier templates; do not colocate it with other GPU-heavy AI stacks on a single small server.',
    links: [
      ['GitHub', 'https://github.com/onyx-dot-app/onyx'],
      ['Docs', 'https://docs.onyx.app/'],
    ],
  },
  khoj: {
    fit: 'Personal or small-team second brain with docs, web answers, agents, and automations.',
    setup: [
      'Set Postgres password, Django secret, admin email, and admin password.',
      'Add a hosted provider key or set OPENAI_BASE_URL and KHOJ_DEFAULT_CHAT_MODEL for a local model.',
      'Open KHOJ_PORT after startup and finish onboarding.',
    ],
    caution: 'Keep reverse-proxy domain settings aligned if exposing it outside localhost.',
    links: [
      ['GitHub', 'https://github.com/khoj-ai/khoj'],
      ['Docs', 'https://docs.khoj.dev/'],
    ],
  },
  docsgpt: {
    fit: 'Private document Q&A, support assistants, and enterprise search.',
    setup: [
      'Set DOCSGPT_POSTGRES_PASSWORD.',
      'Set at least one model provider key before expecting generated answers.',
      'Keep DOCSGPT_API_PUBLIC_URL reachable from user browsers.',
    ],
    caution: 'Pin a tested image tag before production because upstream compose examples commonly use develop images.',
    links: [['GitHub', 'https://github.com/arc53/DocsGPT']],
  },
  'openmemory-mem0': {
    fit: 'Durable memory API for agents and MCP clients.',
    setup: [
      'Set OPENMEMORY_USER and OPENMEMORY_API_KEY.',
      'Set OPENAI_API_KEY if the memory extraction path uses OpenAI models.',
      'Point MCP clients at OPENMEMORY_API_PORT and use OPENMEMORY_UI_PORT for inspection.',
    ],
    caution: 'Memory APIs can expose sensitive user context; keep them private or behind access control.',
    links: [
      ['GitHub', 'https://github.com/mem0ai/mem0'],
      ['Docs', 'https://docs.mem0.ai/'],
    ],
  },
  langfuse: {
    fit: 'LLM traces, prompt management, datasets, metrics, playgrounds, and eval workflows.',
    setup: [
      'Generate NEXTAUTH_SECRET, SALT, ENCRYPTION_KEY, and service passwords.',
      'Start the stack and create an org/project.',
      'Copy Langfuse keys into apps that should report traces.',
    ],
    caution: 'Back up Postgres and ClickHouse before relying on it for production telemetry.',
    links: [
      ['GitHub', 'https://github.com/langfuse/langfuse'],
      ['Docs', 'https://langfuse.com/docs'],
    ],
  },
  phoenix: {
    fit: 'Lightweight AI tracing, experiments, datasets, prompt work, and evaluations.',
    setup: [
      'Set PHOENIX_POSTGRES_PASSWORD.',
      'Start the stack and open PHOENIX_PORT.',
      'Point OpenTelemetry instrumentation at PHOENIX_OTLP_GRPC_PORT.',
    ],
    caution: 'Place the UI behind auth if it leaves a trusted network.',
    links: [
      ['GitHub', 'https://github.com/Arize-ai/phoenix'],
      ['Docs', 'https://phoenix.arize.com/'],
    ],
  },
  promptfoo: {
    fit: 'Prompt, RAG, model, and agent evals, including red-team regression checks.',
    setup: [
      'Set provider keys used by evals.',
      'Start the stack and open PROMPTFOO_PORT.',
      'Keep test outputs in the included persistent volume.',
    ],
    caution: 'The open self-host image does not include built-in auth; protect it before exposure.',
    links: [
      ['GitHub', 'https://github.com/promptfoo/promptfoo'],
      ['Self-hosting', 'https://www.promptfoo.dev/docs/usage/self-hosting/'],
    ],
  },
  firecrawl: {
    fit: 'Search, scrape, crawl, extract, screenshots, and agent web context API.',
    setup: [
      'Set FIRECRAWL_POSTGRES_PASSWORD and BULL_AUTH_KEY.',
      'Add model provider settings only when using AI extraction features.',
      'Call the API on FIRECRAWL_PORT after startup.',
    ],
    caution: 'Self-hosting does not include Firecrawl cloud anti-blocking; use proxies, rate limits, and private access.',
    links: [
      ['GitHub', 'https://github.com/firecrawl/firecrawl'],
      ['Self-hosting', 'https://github.com/firecrawl/firecrawl/blob/main/SELF_HOST.md'],
    ],
  },
  crawl4ai: {
    fit: 'Local crawler/scraper API that emits LLM-ready Markdown for RAG workflows.',
    setup: [
      'Set CRAWL4AI_API_TOKEN.',
      'Add provider keys only when using LLM extraction.',
      'Call the API on CRAWL4AI_PORT after the health check passes.',
    ],
    caution: 'Keep token auth enabled and avoid direct public exposure.',
    links: [
      ['GitHub', 'https://github.com/unclecode/crawl4ai'],
      ['Docs', 'https://docs.crawl4ai.com/'],
    ],
  },
  rhasspy: {
    fit: 'Fully offline voice assistant with wake word, speech-to-text, intent recognition, and text-to-speech — all local.',
    setup: [
      'Open the web UI at RHASSPY_PORT (default 12101).',
      'Select your language profile and download the required models from the settings page.',
      'Configure wake word (porcupine, snowboy, or raven), speech-to-text (kaldi, deepspeech, or pocketsphinx), and TTS (larynx, flite, or pico).',
      'Test with the web UI microphone button before connecting to Home Assistant or other integrations.',
    ],
    caution: 'Microphone access requires the /dev/snd device mount. Remove it for headless/API-only use.',
    links: [
      ['Docs', 'https://rhasspy.readthedocs.io/'],
      ['GitHub', 'https://github.com/rhasspy/rhasspy'],
    ],
  },
  'whisper-asr': {
    fit: 'OpenAI Whisper speech-to-text with a REST API for transcription and translation.',
    setup: [
      'Start the container — it downloads the Whisper model on first run (small by default).',
      'Send audio files via POST to /asr for transcription.',
      'Change ASR_MODEL to medium or large for better accuracy (requires more RAM/VRAM).',
    ],
    caution: 'Large models need 4+ GB RAM. GPU recommended for real-time transcription.',
    links: [
      ['GitHub', 'https://github.com/ahmetoner/whisper-asr-webservice'],
    ],
  },
  'piper-tts': {
    fit: 'Fast local text-to-speech over the Wyoming protocol — pairs with Home Assistant.',
    setup: [
      'Start the container and it auto-downloads the default English voice.',
      'Connect from Home Assistant via the Wyoming integration at the configured port.',
      'Add --voice flag to switch voices. See piper.rhasspy.org for available voice models.',
    ],
    caution: 'CPU-only but very fast. One voice per container instance.',
    links: [
      ['Voices', 'https://rhasspy.github.io/piper-samples/'],
      ['GitHub', 'https://github.com/rhasspy/piper'],
    ],
  },
  'voice-assistant': {
    fit: 'Talk to a local LLM by voice. Ollama for the model, Open WebUI as the chat/voice UI, and Kokoro for text-to-speech — pre-wired so voice works on first login with no manual audio setup. Deploy-verified: all services boot clean, Kokoro synthesizes speech, and Open WebUI reaches it.',
    setup: [
      'Deploy the stack. All three services come up on their own; no passwords to set for the voice pieces.',
      'Open WebUI: http://<host>:WEBUI_PORT — create the admin account on first visit. Ollama is auto-connected.',
      'Pull a model: docker exec <ollama-container> ollama pull llama3.1 (or any Ollama model).',
      'Start a chat, then click the headphone/call icon to talk. Speech-to-text uses Open WebUI\'s built-in Whisper (it downloads a small model on first mic use); text-to-speech is already routed to the local Kokoro server.',
      'Change the voice by setting TTS_VOICE in .env (af_sky, af_bella, am_adam, bf_emma, and more) and restarting Open WebUI. Everything stays editable in Admin > Settings > Audio.',
      'Kokoro TTS API (OpenAI-compatible) is also directly usable at http://<host>:KOKORO_PORT/v1/audio/speech if you want to wire other apps to it.',
    ],
    caution: 'Plan for 6+ GB RAM. TTS is fully local (Kokoro bundles its model). STT downloads a Whisper model on first use. GPU passthrough for Ollama makes responses much faster but is optional.',
    links: [
      ['Open WebUI', 'https://docs.openwebui.com/'],
      ['Open WebUI Audio config', 'https://docs.openwebui.com/troubleshooting/audio/'],
      ['Kokoro-FastAPI', 'https://github.com/remsky/Kokoro-FastAPI'],
      ['Ollama', 'https://ollama.com/'],
    ],
  },
  'openbrain': {
    fit: 'Full AI agent development stack: Ollama for local LLMs, Open WebUI for chat, Neo4j for knowledge graphs, Temporal for durable workflows.',
    setup: [
      'Set POSTGRES_PASSWORD and NEO4J_PASSWORD in .env before starting.',
      'After boot, pull a model: docker exec <ollama-container> ollama pull llama3.1 (also pull nomic-embed-text for embeddings).',
      'Open WebUI: https://<host>:WEBUI_PORT — create an admin account on first visit. Ollama is auto-connected.',
      'Temporal UI: https://<host>:TEMPORAL_UI_PORT — view and manage workflow executions. No login required.',
      'Neo4j Browser: https://<host>:NEO4J_HTTP_PORT — login with neo4j / your NEO4J_PASSWORD. Use Cypher queries to explore the knowledge graph.',
      'Build agents: create a Python venv, install langgraph + graphiti-core + temporalio, and connect to the services at their internal ports.',
      'Graphiti connection: from graphiti_core import Graphiti; g = Graphiti("bolt://<host>:7687", "neo4j", "YOUR_PASSWORD")',
      'Temporal connection: from temporalio.client import Client; client = await Client.connect("<host>:7233")',
      'Ollama API: curl http://<host>:11434/api/generate -d \'{"model":"llama3.1","prompt":"hello"}\'',
    ],
    caution: 'Heavy stack — recommend 4 vCPU / 16 GB RAM minimum. GPU passthrough recommended for Ollama. All services run on an internal Docker network; only the UI ports are exposed.',
    links: [
      ['Ollama', 'https://ollama.com/'],
      ['Open WebUI', 'https://docs.openwebui.com/'],
      ['LangGraph', 'https://langchain-ai.github.io/langgraph/'],
      ['Graphiti', 'https://github.com/getzep/graphiti'],
      ['Temporal', 'https://docs.temporal.io/'],
      ['Neo4j', 'https://neo4j.com/docs/'],
    ],
  },
  'openbrain-mem0': {
    fit: 'Agent stack with durable vector memory (mem0/OpenMemory backed by Qdrant), an optional knowledge graph (Neo4j), and workflow automation (n8n). The "memory + automation" flavour of OpenBrain. Deploy-verified: all services boot cleanly and mem0 connects to Qdrant.',
    setup: [
      'Set POSTGRES_PASSWORD and NEO4J_PASSWORD in .env before starting. Optionally set OPENAI_API_KEY (see the memory note below).',
      'Pull models after boot: docker exec <ollama-container> ollama pull llama3.1 (chat) and ollama pull nomic-embed-text (embeddings for the offline memory path).',
      'Open WebUI: http://<host>:WEBUI_PORT — create an admin account on first visit. Ollama is auto-connected.',
      'mem0 / OpenMemory API + UI: http://<host>:MEM0_PORT (the container listens on 8765 internally; the compose maps it for you). Open /docs for the REST API.',
      'Qdrant vector store: http://<host>:QDRANT_PORT/dashboard — this is where mem0 stores memory vectors. mem0 reaches it internally as the host "mem0_store".',
      'n8n automation: http://<host>:N8N_PORT — create the owner account on first visit (n8n removed static basic-auth; N8N_SECURE_COOKIE is set false so login works over plain HTTP by IP). Workflows persist to Postgres.',
      'MEMORY — pick one: (a) set OPENAI_API_KEY in .env and mem0 works out of the box (text-embedding-3-small, 1536-dim); or (b) fully offline: in OpenMemory > Settings set the LLM to Ollama (model llama3.1, base URL http://ollama:11434) and the embedder to Ollama (model nomic-embed-text) with embedding dimensions 768, then restart the mem0 container so it recreates the Qdrant collection at the matching size.',
      'Neo4j Browser (optional graph memory): http://<host>:NEO4J_HTTP_PORT — login neo4j / your NEO4J_PASSWORD.',
      'Example n8n flow: HTTP trigger → call Ollama for reasoning → POST the result to the mem0 API → later query mem0 for relevant memories → respond.',
    ],
    caution: 'Heavier than OpenBrain #1 — plan for 8+ GB RAM. mem0 cannot extract or embed memories without an LLM: it needs either OPENAI_API_KEY or the local-Ollama configuration above. If you switch embedder models after memories exist, delete the Qdrant "openmemory" collection and restart mem0 so it recreates at the new vector size.',
    links: [
      ['mem0 / OpenMemory', 'https://docs.mem0.ai/'],
      ['Qdrant', 'https://qdrant.tech/documentation/'],
      ['n8n', 'https://docs.n8n.io/'],
      ['Open WebUI', 'https://docs.openwebui.com/'],
      ['Ollama', 'https://ollama.com/'],
      ['Neo4j', 'https://neo4j.com/docs/'],
    ],
  },
  'openbrain-flowise': {
    fit: 'Low-code visual builder for LLM agents and RAG chains: Flowise for the drag-and-drop flow canvas, Ollama for local models, Qdrant for vector storage, Open WebUI for direct chat. Deploy-verified: all services boot cleanly (Flowise reports "all initialization steps completed").',
    setup: [
      'Set POSTGRES_PASSWORD and FLOWISE_PASSWORD in .env before starting.',
      'Pull a model after boot: docker exec <ollama-container> ollama pull llama3.1 (and ollama pull nomic-embed-text for RAG embeddings).',
      'Flowise: http://<host>:FLOWISE_PORT — log in with FLOWISE_USERNAME / FLOWISE_PASSWORD. Flows persist to Postgres.',
      'In a chatflow, add a "ChatOllama" node and set its Base URL to http://ollama:11434 and the model name you pulled.',
      'For RAG, add a "Qdrant" vector-store node pointing at http://qdrant:6333 and an "Ollama Embeddings" node (nomic-embed-text). Upload documents to build a knowledge base.',
      'Open WebUI: http://<host>:WEBUI_PORT — a ready-made chat UI against the same Ollama if you just want to talk to a model.',
      'Qdrant dashboard: http://<host>:QDRANT_PORT/dashboard — inspect collections and vectors created by your flows.',
    ],
    caution: 'Plan for 6+ GB RAM. All internal service URLs (http://ollama:11434, http://qdrant:6333) use Docker DNS names and only work when referenced from inside the flows — not from your browser. GPU passthrough recommended for Ollama.',
    links: [
      ['Flowise', 'https://docs.flowiseai.com/'],
      ['Qdrant', 'https://qdrant.tech/documentation/'],
      ['Ollama', 'https://ollama.com/'],
      ['Open WebUI', 'https://docs.openwebui.com/'],
    ],
  },
  radarr: {
    fit: 'Automated movie downloading and management — monitors RSS feeds and grabs releases.',
    setup: [
      'Open the web UI at RADARR_PORT (default 7878).',
      'Add a download client (NZBGet, SABnzbd, qBittorrent, or Transmission).',
      'Add indexers via Prowlarr or manually in Settings > Indexers.',
      'Add movies and Radarr will search for and download them automatically.',
    ],
    caution: 'Requires a working download client and indexers. Configure quality profiles before adding movies.',
    links: [
      ['Docs', 'https://wiki.servarr.com/radarr'],
      ['GitHub', 'https://github.com/Radarr/Radarr'],
    ],
  },
  sonarr: {
    fit: 'Automated TV show downloading and management — same ecosystem as Radarr for series.',
    setup: [
      'Open the web UI at SONARR_PORT (default 8989).',
      'Add a download client and indexers (same as Radarr).',
      'Add TV series and Sonarr monitors for new episodes automatically.',
    ],
    caution: 'Same requirements as Radarr. Use Prowlarr to manage indexers across Sonarr + Radarr.',
    links: [
      ['Docs', 'https://wiki.servarr.com/sonarr'],
    ],
  },
  jellyfin: {
    fit: 'Free and open-source media server — stream movies, TV, music, and photos to any device.',
    setup: [
      'Open the web UI at the configured port and complete the setup wizard.',
      'Add media library paths pointing to your mounted volumes.',
      'Install client apps on your devices (Roku, Fire TV, iOS, Android, web browser).',
    ],
    caution: 'Hardware transcoding needs GPU passthrough. Without it, direct play only.',
    links: [
      ['Docs', 'https://jellyfin.org/docs/'],
      ['Clients', 'https://jellyfin.org/downloads/clients/'],
    ],
  },
  'hermes-agent': {
    fit: 'Autonomous AI agent gateway by NousResearch with OpenAI-compatible API and web dashboard.',
    setup: [
      'Set HERMES_API_KEY or ANTHROPIC_API_KEY before starting.',
      'API available at port 8642 (OpenAI-compatible).',
      'Dashboard at port 9119 when HERMES_DASHBOARD=1.',
    ],
    caution: 'Needs an LLM provider key to function. The agent runs autonomously once configured.',
    links: [
      ['Docs', 'https://hermes-agent.nousresearch.com/docs/user-guide/docker'],
      ['GitHub', 'https://github.com/NousResearch/hermes-agent'],
    ],
  },
};

export default function Documentation() {
  const [templates, setTemplates] = useState([]);
  const [projectList, setProjectList] = useState([]);
  const [query, setQuery] = useState('');
  const [category, setCategory] = useState('all');
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [error, setError] = useState('');
  const [docsTab, setDocsTab] = useState(() => {
    try { return localStorage.getItem('cm_docs_tab') || 'current'; } catch { return 'current'; }
  });
  const changeDocsTab = (value) => {
    setDocsTab(value);
    try { localStorage.setItem('cm_docs_tab', value); } catch {}
  };

  const load = async () => {
    try {
      const [templateRes, projectRes] = await Promise.all([
        stackTemplates.list(),
        projects.list({ include_inactive: true }),
      ]);
      setTemplates(templateRes.data || []);
      setProjectList(projectRes.data || []);
      setError('');
    } catch (err) {
      setError(err.message);
    }
  };

  useEffect(() => { load(); }, []);
  useEffect(() => { setPage(1); }, [query, category, pageSize]);

  const categoryCounts = useMemo(() => {
    const counts = {};
    templates.forEach(template => { counts[template.category] = (counts[template.category] || 0) + 1; });
    return counts;
  }, [templates]);

  const categories = useMemo(() => {
    const set = new Set(templates.map(template => template.category));
    const known = CATEGORY_ORDER.filter(cat => set.has(cat));
    const extra = Array.from(set).filter(cat => !CATEGORY_ORDER.includes(cat)).sort();
    return ['all', ...known, ...extra];
  }, [templates]);

  const filtered = templates.filter(template => {
    const guide = buildGuide(template);
    const q = query.trim().toLowerCase();
    if (category !== 'all' && template.category !== category) return false;
    if (!q) return true;
    return [
      template.name,
      template.id,
      template.description,
      template.category,
      template.subcategory,
      template.image,
      template.source,
      ...(template.tags || []),
      ...guide.setup,
      guide.caution,
    ].filter(Boolean).join(' ').toLowerCase().includes(q);
  });

  const pageCount = Math.max(1, Math.ceil(filtered.length / pageSize));
  const safePage = Math.min(page, pageCount);
  const start = filtered.length === 0 ? 0 : (safePage - 1) * pageSize + 1;
  const end = Math.min(filtered.length, safePage * pageSize);
  const pagedTemplates = filtered.slice((safePage - 1) * pageSize, safePage * pageSize);

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-gray-950">Documentation</h1>
          <p className="text-sm text-gray-600">{projectList.length} current stack docs and {templates.length} catalog docs across {Math.max(0, categories.length - 1)} categories.</p>
        </div>
        <button onClick={load} className="btn-secondary" title="Reload stack documentation from the built-in catalog.">Refresh</button>
      </div>

      {error && <div className="rounded border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-900">{error}</div>}

      <div className="flex flex-wrap gap-2" role="tablist" aria-label="Documentation sections">
        <button
          type="button"
          role="tab"
          aria-selected={docsTab === 'current'}
          onClick={() => changeDocsTab('current')}
          className={docsTab === 'current' ? 'btn-primary' : 'btn-secondary'}
          title="Docs for stacks that are currently discovered on this host."
        >
          Current Stacks <span className="ml-1 text-xs opacity-80">({projectList.length})</span>
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={docsTab === 'catalog'}
          onClick={() => changeDocsTab('catalog')}
          className={docsTab === 'catalog' ? 'btn-primary' : 'btn-secondary'}
          title="Docs for the built-in stack catalog."
        >
          Stack Catalog <span className="ml-1 text-xs opacity-80">({templates.length})</span>
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={docsTab === 'stackmanager'}
          onClick={() => changeDocsTab('stackmanager')}
          className={docsTab === 'stackmanager' ? 'btn-primary' : 'btn-secondary'}
          title="How to install, upgrade, and use Stack Manager itself, plus the API reference."
        >
          Stack Manager
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={docsTab === 'compose'}
          onClick={() => changeDocsTab('compose')}
          className={docsTab === 'compose' ? 'btn-primary' : 'btn-secondary'}
          title="Docker Compose command cheatsheet and how-tos."
        >
          Docker Compose
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={docsTab === 'gpu'}
          onClick={() => changeDocsTab('gpu')}
          className={docsTab === 'gpu' ? 'btn-primary' : 'btn-secondary'}
          title="GPU support: detection, passthrough, drivers, and the single-GPU rule."
        >
          GPU
        </button>
      </div>

      {(docsTab === 'current' || docsTab === 'catalog') && (
        <section className="section-panel">
          <input
            className="input w-full"
            value={query}
            onChange={e => setQuery(e.target.value)}
            placeholder={docsTab === 'current' ? 'search your current stacks by name' : 'search catalog: app, image, env key, tag, category, setup note'}
            title="Search the current tab."
          />
        </section>
      )}

      {docsTab === 'current' && (
        <section className="space-y-3">
          <div>
            <h2 className="text-lg font-semibold text-gray-950">Current Stacks</h2>
            <p className="text-sm text-gray-600">Every discovered project has a generated project guide, plus any README or docs files found in the stack directory.</p>
          </div>
          <div className="grid gap-3 lg:grid-cols-2">
            {projectList.filter(p => { const q = query.trim().toLowerCase(); return !q || (p.name || '').toLowerCase().includes(q); }).map(project => <CurrentStackDoc key={project.name} project={project} />)}
            {projectList.length === 0 && <div className="py-8 text-center text-sm text-gray-500 lg:col-span-2">No current stacks were discovered.</div>}
          </div>
        </section>
      )}

      {docsTab === 'catalog' && (
        <>
          <section className="section-panel space-y-3">
            <div className="flex flex-wrap gap-2" title="Choose a documentation category.">
              {categories.map(cat => {
                const total = cat === 'all' ? templates.length : (categoryCounts[cat] || 0);
                const active = category === cat;
                return (
                  <button
                    key={cat}
                    type="button"
                    onClick={() => setCategory(cat)}
                    title={cat === 'all' ? 'Show every stack doc.' : `Show only ${labelForCategory(cat)} docs.`}
                    className={`rounded-full border px-3 py-1 text-xs font-medium ${active ? 'border-blue-500 bg-blue-500 text-white' : 'border-gray-200 bg-white text-gray-700 hover:border-blue-300'}`}
                  >
                    {cat === 'all' ? 'All' : labelForCategory(cat)} <span className={`ml-1 ${active ? 'text-blue-100' : 'text-gray-400'}`}>{total}</span>
                  </button>
                );
              })}
            </div>
            <div className="rounded-md border border-blue-100 bg-blue-50 px-3 py-2 text-sm text-blue-900">
              {CATEGORY_DESCRIPTIONS[category] || CATEGORY_DESCRIPTIONS.all}
            </div>
          </section>

          <DocPager
            total={templates.length}
            filtered={filtered.length}
            start={start}
            end={end}
            page={safePage}
            pageCount={pageCount}
            pageSize={pageSize}
            setPage={setPage}
            setPageSize={setPageSize}
          />

          <section className="grid gap-3 lg:grid-cols-2">
            {pagedTemplates.map(template => (
              <ProjectDoc key={template.id} template={template} />
            ))}
            {filtered.length === 0 && <div className="py-12 text-center text-sm text-gray-500 lg:col-span-2">No documentation entries match the current filters.</div>}
          </section>

          {filtered.length > pageSize && (
            <DocPager
              total={templates.length}
              filtered={filtered.length}
              start={start}
              end={end}
              page={safePage}
              pageCount={pageCount}
              pageSize={pageSize}
              setPage={setPage}
              setPageSize={setPageSize}
            />
          )}
        </>
      )}

      {docsTab === 'stackmanager' && <StackManagerDocs />}
      {docsTab === 'compose' && <DockerComposeDocs />}
      {docsTab === 'gpu' && <GpuDocs />}
    </div>
  );
}

function CurrentStackDoc({ project }) {
  const docs = project.documentation || [];
  const guide = docs.find(doc => doc.path === '_stack-manager/project-guide.md') || docs[0];
  const projectHref = `/projects/${encodeURIComponent(project.name)}?tab=docs`;
  return (
    <article className="rounded-md border border-gray-200 bg-white p-4 shadow-sm">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div className="min-w-0">
          <h3 className="font-semibold text-gray-950">{project.name}</h3>
          <div className="mt-1 truncate font-mono text-xs text-gray-500" title={project.dir}>{project.dir}</div>
        </div>
        <div className="flex flex-wrap justify-end gap-2">
          <Badge>{project.running ? 'running' : 'stopped'}</Badge>
          {project.inactive && <Badge tone="amber">inactive</Badge>}
        </div>
      </div>
      <div className="mt-3 flex flex-wrap gap-2 text-xs text-gray-600">
        <span className="rounded bg-gray-100 px-2 py-1">{docs.length || 1} docs</span>
        {project.containers?.length > 0 && <span className="rounded bg-gray-100 px-2 py-1">{project.containers.length} containers</span>}
        {project.image_sources?.length > 0 && <span className="rounded bg-gray-100 px-2 py-1">{project.image_sources.length} image sources</span>}
      </div>
      <p className="mt-3 text-sm leading-6 text-gray-600">
        {guide ? `${guide.title} is available for this stack.` : 'A generated project guide is available for this stack.'}
      </p>
      {docs.length > 0 && (
        <div className="mt-3 flex flex-wrap gap-2">
          {docs.slice(0, 4).map(doc => (
            <span key={doc.path} className="rounded border border-gray-200 bg-gray-50 px-2 py-1 font-mono text-xs text-gray-600" title={doc.path}>
              {doc.title}
            </span>
          ))}
          {docs.length > 4 && <span className="rounded border border-gray-200 bg-gray-50 px-2 py-1 text-xs text-gray-500">+{docs.length - 4} more</span>}
        </div>
      )}
      <div className="mt-4">
        <Link to={projectHref} className="inline-flex rounded-md border border-blue-200 bg-blue-50 px-3 py-2 text-sm font-medium text-blue-800 hover:bg-blue-100" title={`Open documentation for ${project.name}.`}>
          Open docs
        </Link>
      </div>
    </article>
  );
}

function ProjectDoc({ template }) {
  const guide = buildGuide(template);
  return (
    <article className="rounded-md border border-gray-200 bg-white p-4 shadow-sm">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div className="min-w-0">
          <h3 className="font-semibold text-gray-950">{template.name}</h3>
          <div className="mt-1 font-mono text-xs text-gray-500">{template.id}</div>
        </div>
        <div className="flex flex-wrap justify-end gap-2">
          <Badge>{labelForCategory(template.category)}</Badge>
          {template.subcategory && <Badge tone="purple">{template.subcategory}</Badge>}
        </div>
      </div>
      <p className="mt-3 text-sm leading-6 text-gray-600">{guide.fit}</p>
      {template.image && <div className="mt-3 break-all rounded bg-gray-50 px-3 py-2 font-mono text-xs text-gray-600">{template.image}</div>}
      <ul className="mt-3 list-disc space-y-1 pl-5 text-sm leading-6 text-gray-700">
        {guide.setup.map(item => <li key={item}>{item}</li>)}
      </ul>
      <p className="mt-3 rounded-md border border-blue-100 bg-blue-50 px-3 py-2 text-sm leading-6 text-blue-900">{guide.caution}</p>
      <div className="mt-3 flex flex-wrap gap-2">
        {guide.links.map(([label, href]) => <DocLink key={href} href={href} compact>{label}</DocLink>)}
      </div>
    </article>
  );
}

function buildGuide(template) {
  const enhanced = enhancedGuides[template.id] || {};
  const envKeys = parseEnvKeys(template.env_content || '');
  return {
    fit: enhanced.fit || template.description || `${template.name} stack template.`,
    setup: enhanced.setup || defaultSetup(template, envKeys),
    caution: enhanced.caution || template.notes || defaultCaution(template),
    links: enhanced.links || defaultLinks(template),
  };
}

function defaultSetup(template, envKeys) {
  const setup = [];
  if (envKeys.length > 0) {
    const preview = envKeys.slice(0, 8).join(', ');
    const suffix = envKeys.length > 8 ? `, and ${envKeys.length - 8} more` : '';
    setup.push(`Review .env values before launch: ${preview}${suffix}.`);
  } else {
    setup.push('Review the compose.yml before launch; this template has no default .env values.');
  }
  if (template.image) {
    setup.push(`Confirm the image/tag is appropriate for your host: ${template.image}.`);
  }
  setup.push('Use Spin it Up from the Stack Catalog, then check logs and container health from the project page.');
  return setup;
}

function defaultCaution(template) {
  if (template.category === 'database') return 'Back up the data volume before upgrades or destructive maintenance.';
  if (template.category === 'proxy' || template.category === 'security') return 'Review ports, trusted networks, and HTTPS/auth settings before exposing it outside a private network.';
  if (template.category === 'ai') return 'AI workloads can be CPU, RAM, disk, or GPU intensive; start one heavy stack at a time on small hosts.';
  return 'Review exposed ports, bind mounts, credentials, and persistent volumes before production use.';
}

function defaultLinks(template) {
  const links = [];
  const imageLink = imageDocsLink(template.image);
  if (imageLink) links.push(['Image', imageLink]);
  return links.length > 0 ? links : [['Docker Compose', 'https://docs.docker.com/compose/']];
}

function parseEnvKeys(content) {
  return content.split('\n')
    .map(line => line.trim())
    .filter(line => line && !line.startsWith('#') && line.includes('='))
    .map(line => line.replace(/^export\s+/, '').split('=')[0].trim())
    .filter(Boolean);
}

function imageDocsLink(image = '') {
  const clean = image.split('@')[0].split(':')[0];
  if (!clean) return '';
  if (clean.startsWith('ghcr.io/')) return `https://${clean.replace(/^ghcr\.io\//, 'github.com/')}`;
  if (clean.startsWith('lscr.io/linuxserver/')) return `https://docs.linuxserver.io/images/docker-${clean.split('/').pop()}/`;
  if (clean.includes('/')) return `https://hub.docker.com/r/${clean}`;
  return `https://hub.docker.com/_/${clean}`;
}

function labelForCategory(cat) {
  return CATEGORY_LABELS[cat] || (cat ? cat.charAt(0).toUpperCase() + cat.slice(1) : cat);
}

function DocPager({ total, filtered, start, end, page, pageCount, pageSize, setPage, setPageSize }) {
  return (
    <div className="section-panel flex flex-col gap-3 text-sm sm:flex-row sm:items-center sm:justify-between">
      <div className="text-gray-600">
        Showing <span className="font-medium text-gray-950">{start}-{end}</span> of <span className="font-medium text-gray-950">{filtered}</span> docs
        {filtered !== total && <span> from {total} total</span>}
      </div>
      <div className="flex flex-wrap items-center gap-2">
        <select className="input max-w-[130px]" value={pageSize} onChange={e => setPageSize(Number(e.target.value))} title="Docs per page.">
          {PAGE_SIZES.map(size => <option key={size} value={size}>{size} docs</option>)}
        </select>
        <button type="button" className="mini-button" disabled={page <= 1} onClick={() => setPage(1)} title="First page.">First</button>
        <button type="button" className="mini-button" disabled={page <= 1} onClick={() => setPage(page - 1)} title="Previous page.">Prev</button>
        <span className="text-xs text-gray-500">Page {page} of {pageCount}</span>
        <button type="button" className="mini-button" disabled={page >= pageCount} onClick={() => setPage(page + 1)} title="Next page.">Next</button>
        <button type="button" className="mini-button" disabled={page >= pageCount} onClick={() => setPage(pageCount)} title="Last page.">Last</button>
      </div>
    </div>
  );
}

function Badge({ tone = 'gray', children }) {
  const tones = {
    gray: 'bg-gray-100 text-gray-700',
    amber: 'bg-amber-100 text-amber-800',
    purple: 'bg-purple-100 text-purple-800',
  };
  return <span className={`inline-flex rounded px-2 py-0.5 text-xs font-medium ${tones[tone] || tones.gray}`}>{children}</span>;
}

function DocLink({ href, children, compact = false }) {
  return (
    <a
      href={href}
      target="_blank"
      rel="noreferrer"
      className={`inline-flex rounded-md border border-blue-200 bg-blue-50 font-medium text-blue-800 hover:bg-blue-100 ${compact ? 'px-2 py-1 text-xs' : 'px-3 py-2 text-sm'}`}
      title={`Open ${children} in a new tab.`}
    >
      {children}
    </a>
  );
}

function DocSection({ title, children }) {
  return (
    <section className="section-panel space-y-2">
      <h2 className="text-lg font-semibold text-gray-950">{title}</h2>
      {children}
    </section>
  );
}

function Code({ children }) {
  return (
    <pre className="overflow-x-auto rounded-md bg-gray-950 p-3 font-mono text-xs leading-relaxed text-gray-100">{children}</pre>
  );
}

const GH_BASE = 'https://github.com/arphost-com/Stack-Manager/blob/main';

function StackManagerDocs() {
  return (
    <div className="space-y-4">
      <DocSection title="What Stack Manager is">
        <p className="text-sm text-gray-700">A dashboard + CLI for discovering and running Docker Compose stacks on a fleet of hosts. One Go binary runs as a <strong>controller</strong> (full UI + MariaDB/Redis, <code className="rounded bg-gray-100 px-1">/api/v1</code>) or an <strong>agent</strong> (<code className="rounded bg-gray-100 px-1">/agent/v1</code>, bearer-token). Two dirs never to confuse: <code className="rounded bg-gray-100 px-1">STATE_DIR</code> (Stack Manager's own DB/backups/TLS) and <code className="rounded bg-gray-100 px-1">DOCKER_ROOT</code> (the managed projects).</p>
        <p className="text-sm text-gray-600">Full guide: <DocLink href={`${GH_BASE}/docs/PLATFORM.md`} compact>docs/PLATFORM.md</DocLink> · <DocLink href={`${GH_BASE}/README.md`} compact>README</DocLink> · <DocLink href={`${GH_BASE}/docs/AGENT_MODES.md`} compact>Agent modes</DocLink></p>
      </DocSection>

      <DocSection title="Platform features">
        <ul className="list-disc space-y-1.5 pl-5 text-sm leading-6 text-gray-700">
          <li><strong>Manage multiple servers</strong> — the dashboard&rsquo;s <em>All Servers</em> selector shows the local controller plus every connected server. Add them in Settings &rsquo; Agents as a <em>peer</em> (another full controller, by URL + API key), an <em>inbound/both agent</em> (managed live), or a <em>callback agent</em> (behind NAT — pushes inventory and runs queued commands on check-in).</li>
          <li><strong>Open &amp; manage remote projects</strong> — click any project in All Servers to open it, even on a peer/agent. Peer/inbound projects manage live; callback-agent projects open from the last check-in with a Queued Commands panel. Bulk actions, Create Project, and Prune target the selected server.</li>
          <li><strong>Reverse proxy (NPM)</strong> — Settings &rsquo; Reverse Proxy deploys Nginx Proxy Manager in one click; add proxied domains, one-click &ldquo;Proxy the Stack Manager UI,&rdquo; or per-project &ldquo;Add to Proxy.&rdquo; Let&rsquo;s Encrypt lives under Settings &rsquo; SSL for non-proxy installs.</li>
          <li><strong>Firewall</strong> — per-project <em>Open Ports (CSF)</em> adds published TCP ports to the host ConfigServer firewall and reloads it.</li>
          <li><strong>Volumes &amp; networks</strong> — each project&rsquo;s Volumes and Networks tabs list what it owns (with in-use containers) and offer a guarded delete scoped to that stack.</li>
          <li><strong>Naming &amp; clickable ports</strong> — set a friendly server name in Settings &rsquo; General; a project&rsquo;s published ports become clickable links on its Overview.</li>
        </ul>
      </DocSection>

      <DocSection title="Install">
        <p className="text-sm text-gray-700">Web dashboard (recommended):</p>
        <Code>{`mkdir -p ~/docker && cd ~/docker
git clone https://github.com/arphost-com/Stack-Manager.git stack-manager
cd stack-manager
./scripts/prepare-state.sh .env
docker compose --env-file .env up -d --build`}</Code>
        <p className="text-sm text-gray-600"><code className="rounded bg-gray-100 px-1">prepare-state.sh</code> generates random secrets, detects <code className="rounded bg-gray-100 px-1">DOCKER_GID</code>/<code className="rounded bg-gray-100 px-1">SERVER_USER</code>, and prints the login URL — re-running it never overwrites existing secrets. Default dashboard: HTTP 8193 → HTTPS 8993 (self-signed on first boot).</p>
        <p className="text-sm text-gray-700 pt-1">Agent (managed node):</p>
        <Code>{`./scripts/prepare-state.sh --agent --mode callback .env   # or inbound / both
# set DOCKER_ROOT + AGENT_CONTROLLER_URL in .env, then:
docker compose --env-file .env -f docker-compose.agent.yml up -d --build`}</Code>
      </DocSection>

      <DocSection title="Upgrade">
        <ul className="list-disc space-y-1 pl-5 text-sm text-gray-700">
          <li><strong>UI (no SSH):</strong> Settings → Update → <em>Update now</em>. Shows how far behind you are and what's in the update. Install the helper once: <code className="rounded bg-gray-100 px-1 text-xs">sudo install -m 750 scripts/stack-manager-update.sh /usr/local/sbin/stack-manager-update</code></li>
          <li><strong>deploy.sh (recommended full refresh):</strong> <code className="rounded bg-gray-100 px-1">git pull &amp;&amp; ./scripts/deploy.sh</code> — rebuilds with the correct version stamp, installs all host helpers, records the version in the DB.</li>
          <li><strong>Manual:</strong> <code className="rounded bg-gray-100 px-1">git pull &amp;&amp; ./scripts/prepare-state.sh .env &amp;&amp; docker compose --env-file .env up -d --build</code></li>
        </ul>
        <p className="text-sm text-amber-700">Always use <code className="rounded bg-white/60 px-1">up -d --build</code> after pulling — a bare <code className="rounded bg-white/60 px-1">up -d</code> reuses stale images (wrong footer version / "invalid request body").</p>
      </DocSection>

      <DocSection title="Manual upgrade from an old version">
        <ul className="list-disc space-y-1 pl-5 text-sm text-gray-700">
          <li><strong>Keep your secrets</strong> — never regenerate <code className="rounded bg-gray-100 px-1">.env</code>; re-running prepare-state only fills placeholders.</li>
          <li><strong>Settings moved to the DB</strong> (API key, version, timezone, cache TTL, daemon dir, host URL) — seeded from <code className="rounded bg-gray-100 px-1">.env</code> on first boot, so your existing API key keeps working. Edit them under Settings → General afterward.</li>
          <li><strong>Never chown</strong> <code className="rounded bg-gray-100 px-1">STATE_DIR/mariadb</code> or <code className="rounded bg-gray-100 px-1">/redis</code> after init, and never recursively chown <code className="rounded bg-gray-100 px-1">DOCKER_ROOT</code>.</li>
          <li><strong>If CSF was removed</strong>, run <code className="rounded bg-gray-100 px-1">systemctl restart docker</code> (Docker's iptables NAT chain gets flushed with it).</li>
        </ul>
        <p className="text-sm text-gray-600">Details + troubleshooting: <DocLink href={`${GH_BASE}/docs/PLATFORM.md`} compact>docs/PLATFORM.md</DocLink></p>
      </DocSection>

      <DocSection title="API reference">
        <p className="text-sm text-gray-700">REST under <code className="rounded bg-gray-100 px-1">/api/v1</code>. <code className="rounded bg-gray-100 px-1">GET /health</code> and <code className="rounded bg-gray-100 px-1">POST /api/v1/auth/login</code> are public; everything else needs a <strong>login session cookie</strong> or the legacy <code className="rounded bg-gray-100 px-1">X-API-Key</code> header. Some routes (users, docker daemon, OS/self-update, SSL) require an admin.</p>
        <p className="text-sm text-gray-700">Every response uses one envelope:</p>
        <Code>{`{ "status": "ok",    "data": { ... }, "timestamp": "..." }
{ "status": "error", "error": "message", "timestamp": "..." }`}</Code>
        <p className="text-sm text-gray-700">Example (API-key auth):</p>
        <Code>{`curl -sk -H "X-API-Key: $API_KEY" https://HOST:8993/api/v1/projects
curl -sk -H "X-API-Key: $API_KEY" -X POST https://HOST:8993/api/v1/projects/myapp/up
curl -sk -H "X-API-Key: $API_KEY" https://HOST:8993/api/v1/projects/myapp/volumes`}</Code>
        <div className="overflow-x-auto">
          <table className="min-w-full text-sm">
            <thead><tr className="border-b border-gray-200 text-left text-xs uppercase text-gray-500"><th className="py-2 pr-4">Group</th><th className="py-2 pr-4">Routes</th></tr></thead>
            <tbody className="align-top">
              {[
                ['Auth / users', 'POST /auth/login · /auth/totp/login · GET /auth/me · POST /auth/logout · /auth/change-password · TOTP enroll/verify/disable · GET/POST/DELETE /users'],
                ['Projects', 'GET /projects · POST /projects · GET|DELETE /projects/{name} · POST /projects/{name}/{up|down|pull|restart|update} · POST /projects/bulk/{action}'],
                ['Per-project', 'GET /projects/{name}/{status|images|logs*|stats*|files} · shell/exec (WS) · watch · /update-policy · /inactive'],
                ['Volumes / networks', 'GET /projects/{name}/volumes · DELETE …/volumes/{volume} · GET /projects/{name}/networks · DELETE …/networks/{network}'],
                ['Agents / schedules', 'GET|POST /agents · GET /agents/{id}/projects · POST|GET /agents/{id}/commands · GET|POST /schedules · POST /schedules/{id}/run'],
                ['System / settings', 'GET|PUT /settings/env · POST /settings/env/roll-api-key · GET|POST /settings/ssl/* · GET /system/{gpu|os|update|tz} (+ POST actions) · GET|PUT /docker/daemon · GET /metrics/* · GET /audit'],
                ['Skills', 'GET /scan/{name} (security) · backup, dbadmin, debug (inspect/top/logs/stats), firewall'],
                ['Registry / proxy / templates', 'POST /registries/login · GET /stack-templates · /proxy/{status|deploy|hosts}'],
              ].map(([g, r]) => (
                <tr key={g} className="border-b border-gray-100"><td className="py-2 pr-4 font-medium text-gray-900 whitespace-nowrap">{g}</td><td className="py-2 pr-4 font-mono text-xs text-gray-600">{r}</td></tr>
              ))}
            </tbody>
          </table>
        </div>
        <p className="text-xs text-gray-500">Agents expose a parallel read/action subset under <code className="rounded bg-gray-100 px-1">/agent/v1</code> (bearer <code className="rounded bg-gray-100 px-1">AGENT_TOKEN</code>). The controller proxies to them at <code className="rounded bg-gray-100 px-1">/agent-proxy/{'{'}agentId{'}'}/…</code>.</p>
      </DocSection>
    </div>
  );
}

const COMPOSE_COMMANDS = [
  ['Start / update a stack', 'docker compose --env-file .env up -d --build', 'Build (if needed) and start in the background. --build picks up Dockerfile/context changes; omit it to just (re)create.'],
  ['Stop and remove', 'docker compose down', 'Stop and remove containers + the default network. Add -v to also delete named volumes (destroys data).'],
  ['Just stop / start', 'docker compose stop   |   docker compose start', 'Stop or start existing containers without recreating them.'],
  ['Restart', 'docker compose restart [service]', 'Restart all services or one; no config re-read.'],
  ['Pull newer images', 'docker compose pull', 'Fetch newer images without recreating. Follow with up -d to apply.'],
  ['Status', 'docker compose ps        docker compose ls', 'ps = this project’s containers; ls = all compose projects on the host.'],
  ['Logs', 'docker compose logs -f --tail=200 [service]', 'Follow logs; --tail limits history. Drop -f for a one-shot dump.'],
  ['Shell into a container', 'docker compose exec <service> sh', 'Open a shell in a running service (use bash if present).'],
  ['Run a one-off', 'docker compose run --rm <service> <cmd>', 'Run a throwaway container for a command (migrations, CLI tools).'],
  ['Validate / view merged config', 'docker compose config', 'Render the fully-merged compose file with variables resolved. Great for debugging .env interpolation.'],
  ['Rebuild one service', 'docker compose up -d --build --no-deps <service>', 'Rebuild + recreate a single service without touching its dependencies.'],
  ['Recreate cleanly', 'docker compose up -d --force-recreate --remove-orphans', 'Force new containers and drop containers for services no longer in the file.'],
];

function GpuDocs() {
  return (
    <div className="space-y-4">
      <section className="section-panel space-y-2 border-amber-200 bg-amber-50">
        <h2 className="text-lg font-semibold text-amber-900">Single-GPU rule</h2>
        <p className="text-sm leading-6 text-amber-900">
          Do not run multiple heavy AI projects on one GPU at the same time. LLM inference, image generation, voice models, and larger RAG/search stacks fight over VRAM and will crash or starve each other. On a one-GPU host: start one AI stack, verify it, shut it down, then start the next — unless each service is sized and pinned to a specific available GPU.
        </p>
      </section>

      <DocSection title="Detect &amp; test the GPU">
        <p className="text-sm leading-6 text-gray-700">
          <Link to="/settings" className="text-blue-700 underline">Settings &rsquo; GPU</Link> detects the host GPU (vendor, model, driver, CUDA). <strong>Run GPU test</strong> launches a real <code className="rounded bg-gray-100 px-1">--gpus all</code> container and runs <code className="rounded bg-gray-100 px-1">nvidia-smi</code> inside it, so you know passthrough actually works end to end — not just that a driver is present.
        </p>
      </DocSection>

      <DocSection title="One-click driver + toolkit install">
        <p className="text-sm leading-6 text-gray-700">
          If the GPU stack isn&rsquo;t set up, <strong>Settings &rsquo; GPU</strong> can install the NVIDIA driver and the NVIDIA Container Toolkit on the host (Debian &amp; Ubuntu) and wire Docker&rsquo;s runtime. On Secure Boot systems the DKMS module is signed with a MOK — you&rsquo;ll need to <strong>enroll the key at the console</strong> (Proxmox noVNC or a monitor) on the next reboot, then the driver loads.
        </p>
        <ul className="list-disc space-y-1 pl-5 text-sm text-gray-700">
          <li>Detect &rarr; Install &rarr; reboot (enroll MOK if Secure Boot) &rarr; Run GPU test.</li>
          <li>The install runs detached on the host and survives the dashboard restart.</li>
        </ul>
      </DocSection>

      <DocSection title="Give a stack the GPU">
        <p className="text-sm leading-6 text-gray-700">
          When you open an AI template with a GPU present, an <strong>Add GPU passthrough</strong> checkbox injects the compose device reservation for you. For an existing project, the <strong>Enable GPU</strong> action on its Overview adds the <code className="rounded bg-gray-100 px-1">deploy.resources.reservations.devices</code> block and restarts it. Tdarr and other transcoders benefit from the same passthrough.
        </p>
      </DocSection>

      <DocSection title="Multiple GPUs">
        <p className="text-sm leading-6 text-gray-700">
          With more than one GPU you can run stacks concurrently if each is pinned to a specific device (e.g. <code className="rounded bg-gray-100 px-1">NVIDIA_VISIBLE_DEVICES=0</code> / <code className="rounded bg-gray-100 px-1">device_ids: ['1']</code>) and sized to fit that card&rsquo;s VRAM. Watch <code className="rounded bg-gray-100 px-1">nvidia-smi</code> for contention.
        </p>
        <div className="flex flex-wrap gap-2">
          <DocLink href="https://docs.docker.com/compose/how-tos/gpu-support/" compact>Compose GPU support</DocLink>
          <DocLink href="https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/" compact>NVIDIA Container Toolkit</DocLink>
        </div>
      </DocSection>
    </div>
  );
}

function DockerComposeDocs() {
  return (
    <div className="space-y-4">
      <DocSection title="Docker Compose">
        <p className="text-sm leading-6 text-gray-700">
          Stack Manager runs everything as plain Docker Compose projects. Pick a template from the Stack Catalog, set its <code className="rounded bg-gray-100 px-1">.env</code>, and Spin it Up — or manage any stack by hand with the commands below. Each project is just a folder with a <code className="rounded bg-gray-100 px-1">compose.yml</code> under your <code className="rounded bg-gray-100 px-1">DOCKER_ROOT</code>.
        </p>
        <div className="flex flex-wrap gap-2">
          <DocLink href="https://docs.docker.com/compose/" compact>Docker Compose docs</DocLink>
          <DocLink href="https://docs.docker.com/compose/how-tos/gpu-support/" compact>Compose GPU support</DocLink>
        </div>
      </DocSection>
      <DocSection title="Docker Compose cheatsheet">
        <p className="text-sm text-gray-600">The commands Stack Manager runs under the hood — handy from the per-project <strong>Shell</strong> tab or an SSH session in the project directory. Run them from the folder that holds <code className="rounded bg-gray-100 px-1">compose.yml</code>.</p>
        <div className="overflow-x-auto">
          <table className="min-w-full text-sm">
            <thead><tr className="border-b border-gray-200 text-left text-xs uppercase text-gray-500"><th className="py-2 pr-4">Task</th><th className="py-2 pr-4">Command</th><th className="py-2 pr-4">Notes</th></tr></thead>
            <tbody className="align-top">
              {COMPOSE_COMMANDS.map(([task, cmd, note]) => (
                <tr key={task} className="border-b border-gray-100">
                  <td className="py-2 pr-4 font-medium text-gray-900 whitespace-nowrap">{task}</td>
                  <td className="py-2 pr-4"><code className="whitespace-pre rounded bg-gray-950 px-1.5 py-0.5 font-mono text-xs text-gray-100">{cmd}</code></td>
                  <td className="py-2 pr-4 text-xs text-gray-600">{note}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </DocSection>

      <DocSection title="Volumes, networks & cleanup">
        <Code>{`docker compose ps -q | xargs docker inspect --format '{{.Name}}'   # container names
docker volume ls  --filter label=com.docker.compose.project=<project>
docker network ls --filter label=com.docker.compose.project=<project>
docker volume rm  <name>       # fails while a container uses it
docker system df               # disk used by images/containers/volumes
docker system prune            # remove unused data (add --volumes to include volumes)`}</Code>
        <p className="text-sm text-gray-600">In the dashboard, a stack's volumes and networks (with in-use containers and a guarded delete) are on the project's <strong>Volumes</strong> and <strong>Networks</strong> tabs. Host-wide cleanup is under Dashboard → Prune.</p>
      </DocSection>

      <DocSection title="Stack Manager equivalents">
        <p className="text-sm text-gray-700">The bundled CLI mirrors these across every discovered project (flags come <em>before</em> the command):</p>
        <Code>{`./stack-manager.sh --root /docker list
./stack-manager.sh --root /docker --dry-run update
./stack-manager.sh --root /docker update <project>`}</Code>
        <p className="text-sm text-gray-600">A per-project <code className="rounded bg-gray-100 px-1">&lt;hooks-dir&gt;/post-update_&lt;project&gt;.sh</code> overrides the generic pull+up for stacks that need a specific sequence.</p>
      </DocSection>

      <DocSection title="Official references">
        <div className="flex flex-wrap gap-2">
          <DocLink href="https://docs.docker.com/compose/">Compose overview</DocLink>
          <DocLink href="https://docs.docker.com/reference/cli/docker/compose/">Compose CLI reference</DocLink>
          <DocLink href="https://docs.docker.com/reference/compose-file/">Compose file spec</DocLink>
          <DocLink href="https://docs.docker.com/engine/reference/commandline/cli/">Docker CLI</DocLink>
        </div>
      </DocSection>
    </div>
  );
}
