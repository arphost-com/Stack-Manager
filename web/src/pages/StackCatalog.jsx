import { useEffect, useMemo, useState } from 'react';
import { projects, stackTemplates } from '../api/client';

export default function StackCatalog() {
  const [templates, setTemplates] = useState([]);
  const [query, setQuery] = useState('');
  const [category, setCategory] = useState('all');
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

  const categories = useMemo(() => ['all', ...Array.from(new Set(templates.map(t => t.category))).sort()], [templates]);
  const filtered = templates.filter(template => {
    const q = query.trim().toLowerCase();
    if (category !== 'all' && template.category !== category) return false;
    if (!q) return true;
    return [template.name, template.description, template.category, ...(template.tags || [])].join(' ').toLowerCase().includes(q);
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

      <div className="section-panel">
        <div className="grid gap-3 md:grid-cols-[1fr_220px]">
          <input className="input" value={query} onChange={e => setQuery(e.target.value)} placeholder="search templates, tags, categories" title="Filter catalog templates." />
          <select className="input" value={category} onChange={e => setCategory(e.target.value)} title="Filter by category.">
            {categories.map(item => <option key={item} value={item}>{item}</option>)}
          </select>
        </div>
      </div>

      <div className="grid gap-4 lg:grid-cols-[1fr_520px]">
        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
          {filtered.map(template => (
            <button key={template.id} type="button" onClick={() => chooseTemplate(template)} className={`rounded-md border border-gray-200 bg-white p-4 text-left text-sm shadow-sm hover:border-blue-300 ${selected?.id === template.id ? 'border-blue-500 ring-2 ring-blue-100' : ''}`} title="Load this stack into the editor.">
              <div className="flex items-start justify-between gap-2">
                <div>
                  <div className="font-medium text-gray-950">{template.name}</div>
                  <div className="mt-1 text-xs text-gray-500">{template.description}</div>
                </div>
                <Badge>{template.category}</Badge>
              </div>
              <div className="mt-3 flex flex-wrap gap-1">
                {(template.tags || []).map(tag => <Badge key={tag} tone="cyan">{tag}</Badge>)}
              </div>
              {template.image && <div className="mt-3 font-mono text-xs text-gray-500">{template.image}</div>}
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
            <textarea disabled={!selected} required className="textarea h-72 font-mono" value={form.compose_content} onChange={e => setForm({ ...form, compose_content: e.target.value })} />
          </label>
          <label className="block text-sm">
            <span className="mb-1 block font-medium text-gray-700">.env</span>
            <textarea disabled={!selected} className="textarea h-40 font-mono" value={form.env_content} onChange={e => setForm({ ...form, env_content: e.target.value })} />
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
  };
  return <span className={`inline-flex rounded px-2 py-0.5 text-xs font-medium ${tones[tone] || tones.gray}`}>{children}</span>;
}
