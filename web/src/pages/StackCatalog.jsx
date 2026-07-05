import { useEffect, useMemo, useState } from 'react';
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

const SUBCATEGORY_LABELS = {
  'llm-inference': 'LLM inference',
  'image-generation': 'Image generation',
  'voice-speech': 'Voice / speech',
  'vector-db': 'Vector DB',
  'workflow-rag': 'Workflow / RAG',
  'code-assistants': 'Code assistants',
  'search': 'Search',
};

const SUBCATEGORY_ORDER = [
  'llm-inference',
  'code-assistants',
  'image-generation',
  'voice-speech',
  'vector-db',
  'workflow-rag',
  'search',
];

function labelForCategory(cat) {
  return CATEGORY_LABELS[cat] || (cat ? cat.charAt(0).toUpperCase() + cat.slice(1) : cat);
}

function labelForSubcategory(sub) {
  return SUBCATEGORY_LABELS[sub] || sub;
}

export default function StackCatalog() {
  const [templates, setTemplates] = useState([]);
  const [query, setQuery] = useState('');
  const [category, setCategory] = useState('all');
  const [subcategory, setSubcategory] = useState('all');
  const [message, setMessage] = useState(null);
  const [selected, setSelected] = useState(null);
  const [form, setForm] = useState({
    name: '',
    compose_content: '',
    env_content: '',
    inactive: false,
    overwrite: false,
  });

  const load = async () => {
    try {
      const res = await stackTemplates.list();
      setTemplates(res.data || []);
    } catch (err) {
      setMessage({ type: 'error', text: err.message });
    }
  };

  useEffect(() => { load(); }, []);

  useEffect(() => { setSubcategory('all'); }, [category]);

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

  const chooseTemplate = (template) => {
    setSelected(template);
    setForm({
      name: template.id,
      compose_content: template.compose_content,
      env_content: template.env_content || '',
      inactive: false,
      overwrite: false,
    });
    setMessage({ type: 'ok', text: `${template.name} loaded. Review compose.yml and .env before creating.` });
  };

  const createProject = async (event) => {
    event.preventDefault();
    setMessage({ type: 'running', text: `Creating ${form.name}...` });
    try {
      await projects.create(form);
      setMessage({ type: 'ok', text: `Created ${form.name}` });
      setSelected(null);
    } catch (err) {
      setMessage({ type: 'error', text: err.message });
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-gray-950">Stack Catalog</h1>
          <p className="text-sm text-gray-600">Pick a stack, edit its compose.yml and .env, then create it under the configured Docker root.</p>
        </div>
        <button onClick={load} className="btn-secondary" title="Reload the built-in stack catalog.">Refresh</button>
      </div>

      {message && <div className={`rounded border px-4 py-3 text-sm ${message.type === 'error' ? 'border-red-200 bg-red-50 text-red-900' : message.type === 'running' ? 'border-blue-200 bg-blue-50 text-blue-900' : 'border-green-200 bg-green-50 text-green-900'}`}>{message.text}</div>}

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
      </div>

      <div className="grid gap-4 lg:grid-cols-[1fr_520px]">
        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
          {filtered.map(template => (
            <button key={template.id} type="button" onClick={() => chooseTemplate(template)} className={`min-w-0 rounded-md border border-gray-200 bg-white p-4 text-left text-sm shadow-sm hover:border-blue-300 ${selected?.id === template.id ? 'border-blue-500 ring-2 ring-blue-100' : ''}`} title="Load this stack into the editor.">
              <div className="flex items-start justify-between gap-2">
                <div className="min-w-0">
                  <div className="break-words font-medium text-gray-950">{template.name}</div>
                  <div className="mt-1 text-xs text-gray-500">{template.description}</div>
                </div>
                <div className="flex shrink-0 flex-col items-end gap-1">
                  <Badge>{labelForCategory(template.category)}</Badge>
                  {template.subcategory && <Badge tone="purple">{labelForSubcategory(template.subcategory)}</Badge>}
                </div>
              </div>
              <div className="mt-3 flex flex-wrap gap-1">
                {(template.tags || []).map(tag => <Badge key={tag} tone="cyan">{tag}</Badge>)}
              </div>
              {template.image && <div className="mt-3 min-w-0 break-all font-mono text-xs text-gray-500">{template.image}</div>}
            </button>
          ))}
          {filtered.length === 0 && <div className="py-12 text-center text-sm text-gray-500 md:col-span-2 xl:col-span-3">No templates match the current filters.</div>}
        </div>

        <form onSubmit={createProject} className="section-panel space-y-3">
          <div>
            <h2 className="text-lg font-semibold text-gray-950">{selected ? selected.name : 'Template Editor'}</h2>
            <p className="text-sm text-gray-600">{selected ? selected.notes || selected.description : 'Select a template to load editable files.'}</p>
          </div>
          <input className="input" required disabled={!selected} value={form.name} onChange={e => setForm({ ...form, name: e.target.value })} placeholder="project name" title="Folder name to create under the Docker root." />
          <div className="flex flex-wrap gap-4 text-sm text-gray-700">
            <label className="flex items-center gap-2" title="Create with .inactive marker.">
              <input type="checkbox" disabled={!selected} checked={form.inactive} onChange={e => setForm({ ...form, inactive: e.target.checked })} />
              Start inactive
            </label>
            <label className="flex items-center gap-2" title="Allow replacing compose.yml and .env if the project exists.">
              <input type="checkbox" disabled={!selected} checked={form.overwrite} onChange={e => setForm({ ...form, overwrite: e.target.checked })} />
              Overwrite existing
            </label>
          </div>
          <label className="block text-sm">
            <span className="mb-1 block font-medium text-gray-700">compose.yml</span>
            <textarea disabled={!selected} required className="textarea h-72 font-mono" value={form.compose_content} onChange={e => setForm({ ...form, compose_content: e.target.value })} title="Editable compose.yml before creation." />
          </label>
          <label className="block text-sm">
            <span className="mb-1 block font-medium text-gray-700">.env</span>
            <textarea disabled={!selected} className="textarea h-40 font-mono" value={form.env_content} onChange={e => setForm({ ...form, env_content: e.target.value })} title="Editable .env with default settings for this stack." />
          </label>
          <button disabled={!selected} className="btn-primary w-full" title="Create the selected stack as a Compose Manager project.">Create Project</button>
        </form>
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
