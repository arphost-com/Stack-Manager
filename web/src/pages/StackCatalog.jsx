import { useEffect, useMemo, useState } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { projects, stackTemplates, system } from '../api/client';

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
  all: 'Every built-in stack. Use the search box or the category chips to narrow the list.',
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
  proxy: 'Reverse proxies, load balancers, and forward proxies — most also handle TLS termination.',
  queue: 'Message queues, brokers, and workflow orchestrators.',
  security: 'Authentication, SSO, VPNs, and security scanners.',
  web: 'Web servers and static hosting.',
};

const SUBCATEGORY_LABELS = {
  'llm-inference': 'LLM inference',
  'code-assistants': 'Code assistants',
  'personal-agents': 'Personal AI agents',
  'image-generation': 'Image generation',
  'voice-speech': 'Voice / speech',
  'vector-db': 'Vector DB',
  'workflow-rag': 'Workflow / RAG',
  'observability': 'Observability',
  'evals': 'Evals / testing',
  'search': 'Search',
};

const SUBCATEGORY_DESCRIPTIONS = {
  'all': 'Every AI template. Pick a sub-category to focus.',
  'llm-inference': 'Run large language models locally or expose an OpenAI-compatible API for other services to hit.',
  'code-assistants': 'In-editor and CLI AI coding assistants — pair one with an LLM inference stack for a fully local setup.',
  'personal-agents': 'Self-hosted personal AI agents that bridge Discord, Slack, WhatsApp, Signal, Telegram, iMessage and other chat apps to an LLM. OpenClaw-style.',
  'image-generation': 'Stable Diffusion and other text-to-image / image-to-image tools. Most benefit from an NVIDIA GPU.',
  'voice-speech': 'Text-to-speech, automatic speech recognition, and voice cloning servers.',
  'vector-db': 'Vector similarity databases for embeddings and RAG. Pair with an LLM inference stack.',
  'workflow-rag': 'RAG pipelines, chat platforms, memory layers, and LLM workflow builders for document ingestion, tools, and agent orchestration.',
  'observability': 'LLM tracing, prompt management, metrics, datasets, experiments, and evaluation workflows for running AI systems.',
  'evals': 'Prompt, model, RAG, and agent evaluation tools, including red-team and regression testing workflows.',
  'search': 'Full-text, hybrid search, crawlers, and web-context extraction tools for AI apps and agents.',
};

const SUBCATEGORY_ORDER = [
  'llm-inference',
  'code-assistants',
  'personal-agents',
  'image-generation',
  'voice-speech',
  'vector-db',
  'workflow-rag',
  'observability',
  'evals',
  'search',
];

const PAGE_SIZES = [24, 48, 96, 199];

function labelForCategory(cat) {
  return CATEGORY_LABELS[cat] || (cat ? cat.charAt(0).toUpperCase() + cat.slice(1) : cat);
}

function labelForSubcategory(sub) {
  return SUBCATEGORY_LABELS[sub] || sub;
}

const EMPTY_FORM = { name: '', compose_content: '', env_content: '', run_as_uid: '', run_as_gid: '', enforce_user: true, inactive: false, overwrite: false };

export default function StackCatalog() {
  const navigate = useNavigate();
  const [templates, setTemplates] = useState([]);
  const [query, setQuery] = useState('');
  const [category, setCategory] = useState('all');
  const [subcategory, setSubcategory] = useState('all');
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(24);
  const [message, setMessage] = useState(null);
  const [selected, setSelected] = useState(null);
  const [form, setForm] = useState(EMPTY_FORM);
  const [gpuInfo, setGpuInfo] = useState(null);
  const [submitting, setSubmitting] = useState(false);
  const [errorDetails, setErrorDetails] = useState('');

  const load = async () => {
    try {
      const [res, gpuRes] = await Promise.all([
        stackTemplates.list(),
        system.gpu().catch(() => ({ data: { available: false } })),
      ]);
      setTemplates(res.data || []);
      setGpuInfo(gpuRes.data || { available: false });
    } catch (err) {
      setMessage({ type: 'error', text: err.message });
      setErrorDetails(err.data?.output || err.data?.error || '');
    }
  };

  useEffect(() => { load(); }, []);
  useEffect(() => { setSubcategory('all'); }, [category]);
  useEffect(() => { setPage(1); }, [query, category, subcategory, pageSize]);
  useEffect(() => {
    if (!selected) return;
    const onKey = (event) => { if (event.key === 'Escape') closeModal(); };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [selected]);

  const categoryCounts = useMemo(() => {
    const counts = {};
    templates.forEach(t => { counts[t.category] = (counts[t.category] || 0) + 1; });
    return counts;
  }, [templates]);

  const categories = useMemo(() => {
    const set = new Set(templates.map(t => t.category));
    const order = ['ai', 'web', 'proxy', 'cms', 'database', 'devtools', 'docs', 'files', 'management', 'media', 'monitoring', 'queue', 'security', 'automation'];
    const known = order.filter(cat => set.has(cat));
    const extra = Array.from(set).filter(cat => !order.includes(cat)).sort();
    return ['all', ...known, ...extra];
  }, [templates]);

  const subcategoryCounts = useMemo(() => {
    const counts = {};
    templates.filter(t => t.category === 'ai').forEach(t => {
      const sub = t.subcategory || 'other';
      counts[sub] = (counts[sub] || 0) + 1;
    });
    return counts;
  }, [templates]);

  const aiSubcategories = useMemo(() => {
    const set = new Set(templates.filter(t => t.category === 'ai').map(t => t.subcategory || 'other'));
    const known = SUBCATEGORY_ORDER.filter(sub => set.has(sub));
    const extra = Array.from(set).filter(sub => !SUBCATEGORY_ORDER.includes(sub)).sort();
    return ['all', ...known, ...extra];
  }, [templates]);

  const filtered = templates.filter(template => {
    const q = query.trim().toLowerCase();
    if (category !== 'all' && template.category !== category) return false;
    if (category === 'ai' && subcategory !== 'all' && (template.subcategory || 'other') !== subcategory) return false;
    if (!q) return true;
    return [template.name, template.description, template.category, template.subcategory, ...(template.tags || [])].filter(Boolean).join(' ').toLowerCase().includes(q);
  });
  const pageCount = Math.max(1, Math.ceil(filtered.length / pageSize));
  const safePage = Math.min(page, pageCount);
  const start = filtered.length === 0 ? 0 : (safePage - 1) * pageSize + 1;
  const end = Math.min(filtered.length, safePage * pageSize);
  const pagedTemplates = filtered.slice((safePage - 1) * pageSize, safePage * pageSize);

  const gpuDeploySnippet = '    deploy:\n      resources:\n        reservations:\n          devices:\n            - driver: nvidia\n              count: all\n              capabilities: [gpu]\n';

  const openTemplate = (template) => {
    setSelected(template);
    let compose = template.compose_content;

    // Auto-inject GPU config for AI templates when a GPU is detected.
    const isAI = template.category === 'ai';
    const hasGPU = gpuInfo?.available;
    if (isAI && hasGPU && !compose.includes('capabilities: [gpu]')) {
      // Insert deploy block after the first service's restart line.
      compose = compose.replace(
        /(restart:\s*unless-stopped\n)/,
        '$1' + gpuDeploySnippet
      );
    }

    setForm({
      name: template.id,
      compose_content: compose,
      env_content: template.env_content || '',
      run_as_uid: '',
      run_as_gid: '',
      enforce_user: false,
      inactive: false,
      overwrite: false,
    });
    if (isAI && hasGPU) {
      setMessage({ type: 'success', text: `GPU detected (${gpuInfo.gpu_name || 'NVIDIA'}) — GPU acceleration enabled in compose.` });
    } else if (isAI && !hasGPU) {
      setMessage({ type: 'info', text: 'No GPU detected — this AI stack will run in CPU mode. Add GPU config manually if you attach one later.' });
    } else {
      setMessage(null);
    }
    setErrorDetails('');
  };

  const closeModal = () => {
    if (submitting) return;
    setSelected(null);
    setForm(EMPTY_FORM);
  };

  const spinItUp = async (event) => {
    event.preventDefault();
    const projectName = form.name.trim();
    setSubmitting(true);
    setErrorDetails('');
    setMessage({ type: 'running', text: form.inactive ? `Creating ${projectName}...` : `Creating and starting ${projectName}...` });
    try {
      await projects.create({ ...form, name: projectName });
      if (form.inactive) {
        navigate(`/projects/${encodeURIComponent(projectName)}`);
        return;
      }
      const res = await projects.startJob(projectName, 'up');
      const params = new URLSearchParams({ job: res.data.id, action: 'up' });
      navigate(`/projects/${encodeURIComponent(projectName)}?${params.toString()}`);
    } catch (err) {
      setMessage({ type: 'error', text: err.message });
      setErrorDetails(err.data?.output || err.data?.error || err.envelope?.error || '');
    } finally {
      setSubmitting(false);
    }
  };

  const activeCategoryDescription = CATEGORY_DESCRIPTIONS[category] || '';
  const activeSubDescription = category === 'ai' ? (SUBCATEGORY_DESCRIPTIONS[subcategory] || '') : '';

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-gray-950">Stack Catalog</h1>
          <p className="text-sm text-gray-600">{templates.length} apps across {Math.max(0, categories.length - 1)} categories. Click a stack to review compose.yml + .env, then Spin it Up.</p>
        </div>
        <button onClick={load} className="btn-secondary" title="Reload the built-in stack catalog.">Refresh</button>
      </div>

      {message && (
        <div className={`rounded border px-4 py-3 text-sm ${message.type === 'error' ? 'border-red-200 bg-red-50 text-red-900' : message.type === 'running' ? 'border-blue-200 bg-blue-50 text-blue-900' : 'border-green-200 bg-green-50 text-green-900'}`}>
          <div>{message.text}</div>
          {errorDetails && (
            <pre className="mt-2 max-h-72 overflow-auto whitespace-pre-wrap rounded bg-gray-950 p-3 font-mono text-xs text-gray-100">{errorDetails}</pre>
          )}
        </div>
      )}

      <div className="section-panel space-y-3">
        <input className="input w-full" value={query} onChange={e => setQuery(e.target.value)} placeholder="search templates, tags, categories" title="Filter catalog templates by name, description, tags, or (sub)category." />
        <div className="flex flex-wrap gap-2" title="Choose a category. AI has its own sub-category selector below.">
          {categories.map(cat => {
            const total = cat === 'all' ? templates.length : (categoryCounts[cat] || 0);
            const active = category === cat;
            return (
              <button
                key={cat}
                type="button"
                onClick={() => setCategory(cat)}
                title={cat === 'all' ? 'Show every category.' : `Show only ${labelForCategory(cat)} templates.`}
                className={`rounded-full border px-3 py-1 text-xs font-medium ${active ? 'border-blue-500 bg-blue-500 text-white' : 'border-gray-200 bg-white text-gray-700 hover:border-blue-300'}`}
              >
                {cat === 'all' ? 'All' : labelForCategory(cat)} <span className={`ml-1 ${active ? 'text-blue-100' : 'text-gray-400'}`}>{total}</span>
              </button>
            );
          })}
        </div>
        {category === 'ai' && (
          <div className="flex flex-wrap gap-2 border-t border-gray-100 pt-3" title="AI sub-categories.">
            {aiSubcategories.map(sub => {
              const total = sub === 'all' ? (categoryCounts.ai || 0) : (subcategoryCounts[sub] || 0);
              const active = subcategory === sub;
              return (
                <button
                  key={sub}
                  type="button"
                  onClick={() => setSubcategory(sub)}
                  title={sub === 'all' ? 'Show every AI template.' : `Show only AI ${labelForSubcategory(sub)} templates.`}
                  className={`rounded-full border px-3 py-1 text-xs font-medium ${active ? 'border-purple-500 bg-purple-500 text-white' : 'border-gray-200 bg-white text-gray-700 hover:border-purple-300'}`}
                >
                  {sub === 'all' ? 'All AI' : labelForSubcategory(sub)} <span className={`ml-1 ${active ? 'text-purple-100' : 'text-gray-400'}`}>{total}</span>
                </button>
              );
            })}
          </div>
        )}
        {(activeCategoryDescription || activeSubDescription) && (
          <div className="rounded-md border border-blue-100 bg-blue-50 px-3 py-2 text-sm text-blue-900">
            <div>{activeCategoryDescription}</div>
            {activeSubDescription && <div className="mt-1 text-blue-800">{activeSubDescription}</div>}
          </div>
        )}
      </div>

      <CatalogPager
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

      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
        {pagedTemplates.map(template => (
          <div key={template.id} role="button" tabIndex={0} onClick={() => openTemplate(template)} onKeyDown={e => { if (e.key === 'Enter') openTemplate(template); }} className="flex min-w-0 cursor-pointer flex-col rounded-md border border-gray-200 bg-white p-4 text-left text-sm shadow-sm hover:border-blue-300 focus:outline-none focus:ring-2 focus:ring-blue-200" title={`${template.name} — click to open the editor and spin it up.`}>
            <div className="flex items-start justify-between gap-2">
              <div className="min-w-0 flex-1">
                <div className="truncate font-medium text-gray-950" title={template.name}>{template.name}</div>
                <div className="mt-1 text-xs text-gray-500 line-clamp-2">{template.description}</div>
              </div>
              <div className="flex shrink-0 flex-col items-end gap-1">
                <Badge>{labelForCategory(template.category)}</Badge>
                {template.subcategory && <Badge tone="purple">{labelForSubcategory(template.subcategory)}</Badge>}
              </div>
            </div>
            <div className="mt-3 flex flex-wrap gap-1">
              {(template.tags || []).map(tag => <Badge key={tag} tone="cyan">{tag}</Badge>)}
            </div>
            <div className="mt-3 flex items-end justify-between gap-2">
              {template.image ? <div className="min-w-0 break-all font-mono text-xs text-gray-500">{template.image}</div> : <div />}
              <Link to={`/documentation?search=${encodeURIComponent(template.name)}`} onClick={e => e.stopPropagation()} className="shrink-0 text-xs text-blue-600 underline hover:text-blue-800" title={`View documentation for ${template.name}`}>Docs</Link>
            </div>
          </div>
        ))}
        {filtered.length === 0 && <div className="py-12 text-center text-sm text-gray-500 md:col-span-2 xl:col-span-3 2xl:col-span-4">No templates match the current filters.</div>}
      </div>

      {filtered.length > pageSize && (
        <CatalogPager
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

      {selected && (
        <div className="fixed inset-0 z-40 flex items-center justify-center bg-gray-950/60 p-4" onClick={closeModal} role="dialog" aria-modal="true">
          <div className="flex max-h-[90vh] w-full max-w-4xl flex-col overflow-hidden rounded-lg bg-white shadow-2xl" onClick={e => e.stopPropagation()}>
            <div className="flex items-start justify-between gap-3 border-b border-gray-200 px-5 py-4">
              <div>
                <div className="flex items-center gap-2">
                  <h2 className="text-lg font-semibold text-gray-950">{selected.name}</h2>
                  <Badge>{labelForCategory(selected.category)}</Badge>
                  {selected.subcategory && <Badge tone="purple">{labelForSubcategory(selected.subcategory)}</Badge>}
                </div>
                <p className="mt-1 text-sm text-gray-600">{selected.notes || selected.description}</p>
              </div>
              <button type="button" onClick={closeModal} className="btn-secondary" title="Close without creating (Esc).">Close</button>
            </div>

            <form onSubmit={spinItUp} className="flex flex-1 flex-col gap-4 overflow-y-auto p-5">
              <div className="grid gap-3 md:grid-cols-[1fr_auto_auto]">
                <label className="block text-sm" title="Folder name to create under the Docker root.">
                  <span className="mb-1 block font-medium text-gray-700">Project name</span>
                  <input required disabled={submitting} value={form.name} onChange={e => setForm({ ...form, name: e.target.value })} className="input" placeholder="example-stack" />
                </label>
                <label className="flex items-end gap-2 pb-2 text-sm text-gray-700" title="Create the folder but skip starting the stack until it's marked active.">
                  <input type="checkbox" disabled={submitting} checked={form.inactive} onChange={e => setForm({ ...form, inactive: e.target.checked })} />
                  Start inactive
                </label>
                <label className="flex items-end gap-2 pb-2 text-sm text-gray-700" title="Allow replacing compose.yml and .env if the folder already exists.">
                  <input type="checkbox" disabled={submitting} checked={form.overwrite} onChange={e => setForm({ ...form, overwrite: e.target.checked })} />
                  Overwrite existing
                </label>
              </div>
              <div className="grid gap-3 md:grid-cols-[1fr_1fr_auto]">
                <label className="block text-sm" title="Container user UID written to STACK_UID/PUID/UID/USER_UID. Leave blank to use the Stack Manager server user.">
                  <span className="mb-1 block font-medium text-gray-700">Run UID</span>
                  <input disabled={submitting} value={form.run_as_uid} onChange={e => setForm({ ...form, run_as_uid: e.target.value })} className="input" placeholder="auto" inputMode="numeric" />
                </label>
                <label className="block text-sm" title="Container group GID written to STACK_GID/PGID/GID/USER_GID. Leave blank to use the Stack Manager server group.">
                  <span className="mb-1 block font-medium text-gray-700">Run GID</span>
                  <input disabled={submitting} value={form.run_as_gid} onChange={e => setForm({ ...form, run_as_gid: e.target.value })} className="input" placeholder="auto" inputMode="numeric" />
                </label>
                <label className="flex items-end gap-2 pb-2 text-sm text-gray-700" title="Generate compose.override.yml so every service runs as the UID/GID above. Edit that file or .env later for stack-specific users.">
                  <input type="checkbox" disabled={submitting} checked={form.enforce_user} onChange={e => setForm({ ...form, enforce_user: e.target.checked })} />
                  Apply to services
                </label>
              </div>
              <label className="block text-sm" title="Editable compose.yml before creation.">
                <span className="mb-1 block font-medium text-gray-700">compose.yml</span>
                <textarea disabled={submitting} required className="textarea h-72 font-mono" value={form.compose_content} onChange={e => setForm({ ...form, compose_content: e.target.value })} />
              </label>
              <label className="block text-sm" title="Editable .env with default settings for this stack.">
                <span className="mb-1 block font-medium text-gray-700">.env</span>
                <textarea disabled={submitting} className="textarea h-40 font-mono" value={form.env_content} onChange={e => setForm({ ...form, env_content: e.target.value })} />
              </label>
              <div className="flex flex-wrap items-center justify-end gap-2 border-t border-gray-200 pt-3">
                <button type="button" onClick={closeModal} disabled={submitting} className="btn-secondary" title="Cancel without creating (Esc).">Cancel</button>
                <button type="submit" disabled={submitting} className="btn-primary" title="Create the project folder, write compose.yml + .env, and (unless Start inactive is checked) start it.">
                  {submitting ? 'Spinning it up...' : 'Spin it Up'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}

function CatalogPager({ total, filtered, start, end, page, pageCount, pageSize, setPage, setPageSize }) {
  return (
    <div className="section-panel flex flex-col gap-3 text-sm sm:flex-row sm:items-center sm:justify-between">
      <div className="text-gray-600">
        Showing <span className="font-medium text-gray-950">{start}-{end}</span> of <span className="font-medium text-gray-950">{filtered}</span> filtered apps
        {filtered !== total && <span> from {total} total</span>}
      </div>
      <div className="flex flex-wrap items-center gap-2">
        <select className="input max-w-[130px]" value={pageSize} onChange={e => setPageSize(Number(e.target.value))} title="Apps per page.">
          {PAGE_SIZES.map(size => <option key={size} value={size}>{size} apps</option>)}
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
    cyan: 'bg-cyan-100 text-cyan-800',
    purple: 'bg-purple-100 text-purple-800',
  };
  return <span className={`inline-flex rounded px-2 py-0.5 text-xs font-medium ${tones[tone] || tones.gray}`}>{children}</span>;
}
