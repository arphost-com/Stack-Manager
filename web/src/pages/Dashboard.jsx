import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { projects, jobs, skills as skillsApi, system, registries } from '../api/client';

const ACTIONS = [
  { key: 'update', label: 'Update', title: 'Pull newer images, then recreate containers. Projects with update hooks run only their hook.' },
  { key: 'pull', label: 'Pull', title: 'Download newer images without recreating containers.' },
  { key: 'up', label: 'Start', title: 'Run docker compose up -d for selected projects.' },
  { key: 'restart', label: 'Restart', title: 'Restart currently created containers.' },
  { key: 'down', label: 'Stop', title: 'Run docker compose down and remove project containers.' },
];

export default function Dashboard() {
  const [projectList, setProjectList] = useState([]);
  const [skillList, setSkillList] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [actionResult, setActionResult] = useState(null);
  const [selected, setSelected] = useState([]);
  const [filters, setFilters] = useState({ includeInactive: true, runningOnly: false, query: '' });
  const [timeout, setTimeoutValue] = useState(300);
  const [showCreate, setShowCreate] = useState(false);
  const [createForm, setCreateForm] = useState({
    name: '',
    compose_content: 'services:\n  app:\n    image: nginx:stable\n    restart: unless-stopped\n',
    env_content: '',
    inactive: false,
    overwrite: false,
  });
  const [registryForm, setRegistryForm] = useState({ registry: '', username: '', password: '' });

  const fetchData = async () => {
    try {
      setLoading(true);
      const [projRes, skillRes] = await Promise.all([
        projects.list({ include_inactive: filters.includeInactive ? 'true' : 'false', running_only: filters.runningOnly ? 'true' : 'false' }),
        skillsApi.list(),
      ]);
      setProjectList(projRes.data || []);
      setSkillList(skillRes.data || []);
      setError(null);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { fetchData(); }, [filters.includeInactive, filters.runningOnly]);

  const filteredProjects = projectList.filter((p) => {
    const q = filters.query.trim().toLowerCase();
    if (!q) return true;
    return p.name.toLowerCase().includes(q) || p.dir.toLowerCase().includes(q);
  });

  const running = projectList.filter(p => p.running).length;
  const inactive = projectList.filter(p => p.inactive).length;
  const customServices = projectList.reduce((sum, p) => sum + (p.image_sources || []).filter(s => s.source_type === 'custom').length, 0);
  const registryServices = projectList.reduce((sum, p) => sum + (p.image_sources || []).filter(s => s.source_type === 'registry').length, 0);

  const runAction = async (name, action) => {
    try {
      const res = await projects.startJob(name, action, timeout);
      setActionResult({ label: `${action} ${name}`, status: 'running', job: res.data });
      pollJob(res.data.id, `${action} ${name}`);
    } catch (err) {
      setActionResult({ label: `${action} ${name}`, status: 'error', error: err.message });
    }
  };

  const pollJob = (jobId, label) => {
    const tick = async () => {
      try {
        const res = await jobs.get(jobId);
        const job = res.data;
        setActionResult({ label, status: job.status === 'running' ? 'running' : job.success ? 'done' : 'error', job });
        if (job.status === 'running') {
          window.setTimeout(tick, 1500);
        } else {
          fetchData();
        }
      } catch (err) {
        setActionResult({ label, status: 'error', error: err.message });
      }
    };
    window.setTimeout(tick, 750);
  };

  const runBulk = async (action) => {
    const targets = selected.length > 0 ? selected : filteredProjects.map(p => p.name);
    try {
      setActionResult({ label: `${action} ${targets.length} project${targets.length === 1 ? '' : 's'}`, status: 'running' });
      const res = await projects.bulk(action, { projects: targets, timeout });
      setActionResult({ label: `bulk ${action}`, status: 'done', result: res.data });
      setSelected([]);
      fetchData();
    } catch (err) {
      setActionResult({ label: `bulk ${action}`, status: 'error', error: err.message });
    }
  };

  const createProject = async (event) => {
    event.preventDefault();
    try {
      setActionResult({ label: `create ${createForm.name}`, status: 'running' });
      await projects.create(createForm);
      setActionResult({ label: `create ${createForm.name}`, status: 'done' });
      setShowCreate(false);
      setCreateForm({ ...createForm, name: '', env_content: '' });
      fetchData();
    } catch (err) {
      setActionResult({ label: `create ${createForm.name}`, status: 'error', error: err.message });
    }
  };

  const loginRegistry = async (event) => {
    event.preventDefault();
    try {
      setActionResult({ label: `registry login ${registryForm.registry || 'Docker Hub'}`, status: 'running' });
      const res = await registries.login(registryForm);
      setActionResult({ label: 'registry login', status: 'done', result: res.data });
      setRegistryForm({ registry: registryForm.registry, username: registryForm.username, password: '' });
    } catch (err) {
      setActionResult({ label: 'registry login', status: 'error', error: err.message });
    }
  };

  const toggleSelected = (name) => {
    setSelected((current) => current.includes(name) ? current.filter(item => item !== name) : [...current, name]);
  };

  const toggleAll = () => {
    if (selected.length === filteredProjects.length) {
      setSelected([]);
    } else {
      setSelected(filteredProjects.map(p => p.name));
    }
  };

  if (loading) return <div className="text-center py-12 text-gray-500">Loading projects...</div>;
  if (error) return <div className="text-center py-12 text-red-700">Error: {error}</div>;

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-gray-950">Compose Manager</h1>
          <p className="text-sm text-gray-600">Manage compose projects from the configured Docker root.</p>
        </div>
        <div className="flex flex-wrap gap-2">
          <button title="Refresh project discovery, status, containers, hooks, and image sources." onClick={fetchData} className="btn-secondary">Refresh</button>
          <button title="Create a new project directory with a compose.yml file." onClick={() => setShowCreate(!showCreate)} className="btn-primary">Create Project</button>
          <button title="Remove unused Docker images, networks, and volumes on this host." onClick={async () => {
            setActionResult({ label: 'system prune', status: 'running' });
            try {
              const res = await system.prune();
              setActionResult({ label: 'system prune', status: 'done', result: res.data });
            } catch (err) {
              setActionResult({ label: 'system prune', status: 'error', error: err.message });
            }
          }} className="btn-danger">Prune</button>
        </div>
      </div>

      <div className="grid grid-cols-2 gap-3 lg:grid-cols-5">
        <StatCard label="Projects" value={projectList.length} />
        <StatCard label="Running" value={running} />
        <StatCard label="Inactive" value={inactive} />
        <StatCard label="Registry services" value={registryServices} />
        <StatCard label="Custom builds" value={customServices} />
      </div>

      {actionResult && <ActionResult result={actionResult} onDismiss={() => setActionResult(null)} />}

      {showCreate && (
        <form onSubmit={createProject} className="section-panel space-y-4">
          <div className="flex items-center justify-between">
            <h2 className="text-lg font-semibold text-gray-950">Create Project</h2>
            <button type="button" title="Close project creation." onClick={() => setShowCreate(false)} className="btn-secondary">Close</button>
          </div>
          <div className="grid gap-4 lg:grid-cols-[280px_1fr]">
            <div className="space-y-3">
              <Field label="Project name" title="Folder name under the configured root. Use letters, numbers, dots, underscores, or hyphens.">
                <input required value={createForm.name} onChange={e => setCreateForm({ ...createForm, name: e.target.value })} className="input" placeholder="example-stack" />
              </Field>
              <label className="flex items-center gap-2 text-sm text-gray-700" title="Create the project but exclude it from normal operations until activated.">
                <input type="checkbox" checked={createForm.inactive} onChange={e => setCreateForm({ ...createForm, inactive: e.target.checked })} />
                Start inactive
              </label>
              <label className="flex items-center gap-2 text-sm text-gray-700" title="Allow replacing compose.yml and .env if the folder already exists.">
                <input type="checkbox" checked={createForm.overwrite} onChange={e => setCreateForm({ ...createForm, overwrite: e.target.checked })} />
                Overwrite existing
              </label>
              <button type="submit" title="Create the compose project folder and files." className="btn-primary w-full">Create</button>
            </div>
            <div className="grid gap-4 lg:grid-cols-2">
              <Field label="compose.yml" title="Full Docker Compose YAML that will be written to compose.yml.">
                <textarea required value={createForm.compose_content} onChange={e => setCreateForm({ ...createForm, compose_content: e.target.value })} className="textarea h-64 font-mono" />
              </Field>
              <Field label=".env" title="Optional environment file written with owner-readable permissions.">
                <textarea value={createForm.env_content} onChange={e => setCreateForm({ ...createForm, env_content: e.target.value })} className="textarea h-64 font-mono" placeholder="KEY=value" />
              </Field>
            </div>
          </div>
        </form>
      )}

      <div className="section-panel">
        <div className="flex flex-col gap-4 xl:flex-row xl:items-end xl:justify-between">
          <div className="grid gap-3 sm:grid-cols-3 xl:w-[620px]">
            <Field label="Search" title="Filter projects by name or directory.">
              <input value={filters.query} onChange={e => setFilters({ ...filters, query: e.target.value })} className="input" placeholder="project or path" />
            </Field>
            <Field label="Timeout" title="Seconds to wait for pull or update operations before stopping the command.">
              <input type="number" min="0" value={timeout} onChange={e => setTimeoutValue(Number(e.target.value))} className="input" />
            </Field>
            <div className="flex items-end gap-4 pb-2">
              <label className="flex items-center gap-2 text-sm text-gray-700" title="Show projects marked with .inactive.">
                <input type="checkbox" checked={filters.includeInactive} onChange={e => setFilters({ ...filters, includeInactive: e.target.checked })} />
                Inactive
              </label>
              <label className="flex items-center gap-2 text-sm text-gray-700" title="Show only projects with running containers.">
                <input type="checkbox" checked={filters.runningOnly} onChange={e => setFilters({ ...filters, runningOnly: e.target.checked })} />
                Running
              </label>
            </div>
          </div>
          <div className="flex flex-wrap gap-2">
            {ACTIONS.map(action => (
              <button key={action.key} title={`${action.title} Applies to selected rows, or the current filtered list if none are selected.`} onClick={() => runBulk(action.key)} className={action.key === 'down' ? 'btn-danger' : 'btn-secondary'}>
                {action.label} {selected.length || filteredProjects.length}
              </button>
            ))}
          </div>
        </div>

        <div className="mt-4 overflow-x-auto">
          <table className="w-full min-w-[980px] text-left text-sm">
            <thead>
              <tr className="border-b border-gray-200 text-xs uppercase text-gray-500">
                <th className="w-10 py-2"><input title="Select or clear all visible projects." type="checkbox" checked={filteredProjects.length > 0 && selected.length === filteredProjects.length} onChange={toggleAll} /></th>
                <th className="py-2">Project</th>
                <th className="py-2">State</th>
                <th className="py-2">Sources</th>
                <th className="py-2">Containers</th>
                <th className="py-2">Directory</th>
                <th className="py-2 text-right">Actions</th>
              </tr>
            </thead>
            <tbody>
              {filteredProjects.map(p => (
                <tr key={p.name} className="border-b border-gray-100 align-top">
                  <td className="py-3"><input title={`Select ${p.name} for bulk actions.`} type="checkbox" checked={selected.includes(p.name)} onChange={() => toggleSelected(p.name)} /></td>
                  <td className="py-3">
                    <Link to={`/projects/${p.name}`} className="font-medium text-blue-700 hover:underline">{p.name}</Link>
                    <div className="mt-1 flex gap-1">
                      {p.inactive && <Badge tone="amber">inactive</Badge>}
                      {p.has_hook?.update && <Badge tone="cyan">update hook</Badge>}
                    </div>
                  </td>
                  <td className="py-3"><Badge tone={p.running ? 'green' : 'gray'}>{p.running ? 'running' : 'stopped'}</Badge></td>
                  <td className="py-3">
                    <SourceSummary sources={p.image_sources || []} />
                  </td>
                  <td className="py-3 text-gray-700">{p.containers?.length || 0}</td>
                  <td className="py-3 max-w-[300px] truncate font-mono text-xs text-gray-500" title={p.dir}>{p.dir}</td>
                  <td className="py-3">
                    <div className="flex justify-end gap-1">
                      {ACTIONS.map(action => (
                        <button key={action.key} title={action.title} onClick={() => runAction(p.name, action.key)} className={action.key === 'down' ? 'mini-danger' : 'mini-button'}>
                          {action.label}
                        </button>
                      ))}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          {filteredProjects.length === 0 && <div className="py-8 text-center text-gray-500">No projects match the current filters.</div>}
        </div>
      </div>

      <div className="grid gap-4 lg:grid-cols-[1fr_360px]">
        <form onSubmit={loginRegistry} className="section-panel space-y-3">
          <h2 className="text-lg font-semibold text-gray-950">Private Registry Login</h2>
          <div className="grid gap-3 md:grid-cols-3">
            <Field label="Registry" title="Docker registry host only, such as registry.gitlab.com. Leave blank for Docker Hub.">
              <input value={registryForm.registry} onChange={e => setRegistryForm({ ...registryForm, registry: e.target.value })} className="input" placeholder="registry.gitlab.com" />
            </Field>
            <Field label="Username" title="Registry username or deploy token username.">
              <input required value={registryForm.username} onChange={e => setRegistryForm({ ...registryForm, username: e.target.value })} className="input" />
            </Field>
            <Field label="Token or password" title="Sent to docker login via password-stdin. It is not stored by the web app after submission.">
              <input required type="password" value={registryForm.password} onChange={e => setRegistryForm({ ...registryForm, password: e.target.value })} className="input" />
            </Field>
          </div>
          <button type="submit" title="Authenticate Docker so private image pulls and updates can succeed." className="btn-secondary">Login Registry</button>
        </form>
        <div className="section-panel">
          <h2 className="text-lg font-semibold text-gray-950">Skills</h2>
          <div className="mt-3 space-y-2">
            {skillList.map(s => (
              <div key={s.name} className="flex items-start justify-between gap-3 border-b border-gray-100 pb-2 last:border-0">
                <div>
                  <div className="font-medium capitalize text-gray-900">{s.name}</div>
                  <div className="text-xs text-gray-500">{s.description}</div>
                </div>
                <Badge tone={s.healthy ? 'green' : 'red'}>{s.healthy ? 'healthy' : 'down'}</Badge>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}

function SourceSummary({ sources }) {
  const registry = sources.filter(s => s.source_type === 'registry').length;
  const custom = sources.filter(s => s.source_type === 'custom').length;
  const unknown = sources.filter(s => s.source_type === 'unknown').length;
  return (
    <div className="flex flex-wrap gap-1">
      {registry > 0 && <Badge tone="blue">{registry} registry</Badge>}
      {custom > 0 && <Badge tone="violet">{custom} custom</Badge>}
      {unknown > 0 && <Badge tone="gray">{unknown} unknown</Badge>}
      {sources.length === 0 && <span className="text-gray-400">not parsed</span>}
    </div>
  );
}

function StatCard({ label, value }) {
  return (
    <div className="section-panel py-4">
      <div className="text-2xl font-semibold text-gray-950">{value}</div>
      <div className="text-sm text-gray-500">{label}</div>
    </div>
  );
}

function Field({ label, title, children }) {
  return (
    <label className="block text-sm">
      <span className="mb-1 block font-medium text-gray-700" title={title}>{label}</span>
      {children}
    </label>
  );
}

function Badge({ tone = 'gray', children }) {
  const tones = {
    gray: 'bg-gray-100 text-gray-700',
    green: 'bg-green-100 text-green-800',
    red: 'bg-red-100 text-red-800',
    amber: 'bg-amber-100 text-amber-800',
    blue: 'bg-blue-100 text-blue-800',
    cyan: 'bg-cyan-100 text-cyan-800',
    violet: 'bg-violet-100 text-violet-800',
  };
  return <span className={`inline-flex rounded px-2 py-0.5 text-xs font-medium ${tones[tone] || tones.gray}`}>{children}</span>;
}

function ActionResult({ result, onDismiss }) {
  const tone = result.status === 'running' ? 'border-blue-200 bg-blue-50 text-blue-900' :
    result.status === 'error' ? 'border-red-200 bg-red-50 text-red-900' :
    'border-green-200 bg-green-50 text-green-900';
  return (
    <div className={`rounded border px-4 py-3 text-sm ${tone}`}>
      <div className="flex items-start justify-between gap-4">
        <div>
          <div className="font-medium">{result.status === 'running' ? `Running ${result.label}...` : result.status === 'error' ? `Error during ${result.label}` : `${result.label} completed`}</div>
          {result.error && <div className="mt-1">{result.error}</div>}
          {result.job && <div className="mt-1 text-xs">Session: <span className="font-mono">{result.job.id}</span> · {result.job.status}</div>}
          {(result.job?.output || result.result?.output) && (
            <pre className="mt-2 max-h-80 overflow-auto whitespace-pre-wrap rounded bg-gray-950 p-3 font-mono text-xs text-gray-100">
              {result.job?.output || result.result?.output}
            </pre>
          )}
        </div>
        <button title="Dismiss this message." onClick={onDismiss} className="text-sm underline">dismiss</button>
      </div>
    </div>
  );
}
