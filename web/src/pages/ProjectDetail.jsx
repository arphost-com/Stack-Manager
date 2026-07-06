import { useState, useEffect, useRef } from 'react';
import { useParams, Link } from 'react-router-dom';
import { projects, jobs, debug as debugApi, security, backup, dbadmin, watch as watchApi } from '../api/client';

const TABS = ['overview', 'sessions', 'sources', 'watch', 'logs', 'stats', 'shell', 'security', 'backups', 'databases', 'inspect', 'processes'];
const ACTIONS = [
  { key: 'update', label: 'Update', title: 'Pull and recreate containers, unless an update hook exists.' },
  { key: 'pull', label: 'Pull', title: 'Pull images without restarting containers.' },
  { key: 'up', label: 'Start', title: 'Run docker compose up -d.' },
  { key: 'restart', label: 'Restart', title: 'Restart project containers.' },
  { key: 'down', label: 'Stop', title: 'Run docker compose down.' },
];

export default function ProjectDetail() {
  const { name } = useParams();
  const [project, setProject] = useState(null);
  const [activeTab, setActiveTab] = useState('overview');
  const [tabData, setTabData] = useState(null);
  const [loading, setLoading] = useState(true);
  const [tabLoading, setTabLoading] = useState(false);
  const [actionResult, setActionResult] = useState(null);
  const [timeout, setTimeoutValue] = useState(300);
  const [logOptions, setLogOptions] = useState({ tail: 200, container: '', watch: false });
  const [policyForm, setPolicyForm] = useState({ mode: 'auto', notes: '' });
  const [backupDestinationId, setBackupDestinationId] = useState('');
  const [backupDestinations, setBackupDestinations] = useState([]);
  const [shellForm, setShellForm] = useState({ command: 'ps', tail: 200, timeout: 300 });
  const [shellResult, setShellResult] = useState(null);

  const fetchProject = async () => {
    try {
      const [res, destinationRes] = await Promise.all([projects.get(name), backup.destinations()]);
      setProject(res.data);
      setBackupDestinations(destinationRes.data || []);
      setPolicyForm({
        mode: res.data.update_policy?.mode || 'auto',
        notes: res.data.update_policy?.notes || '',
      });
    } catch (err) {
      setActionResult({ status: 'error', label: 'load project', error: err.message });
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { fetchProject(); }, [name]);

  useEffect(() => {
    if (activeTab !== 'logs' || !logOptions.watch) return undefined;
    const id = window.setInterval(() => {
      loadTab('logs');
    }, 3000);
    return () => window.clearInterval(id);
  }, [activeTab, logOptions.watch, logOptions.tail, logOptions.container]);

  const loadTab = async (tab = activeTab) => {
    setActiveTab(tab);
    setTabData(null);
    if (tab === 'overview') return;
    setTabLoading(true);
    try {
      let res;
      switch (tab) {
        case 'sources': res = await projects.images(name); break;
        case 'sessions': res = await jobs.list(); break;
        case 'watch': res = await watchApi.list(name); break;
        case 'logs': res = await debugApi.logs(name, logOptions.tail, logOptions.container || undefined); break;
        case 'stats': res = await debugApi.stats(name); break;
        case 'shell': res = { data: shellResult }; break;
        case 'security': res = await security.scan(name); break;
        case 'backups': res = await backup.listProject(name); break;
        case 'databases': res = await dbadmin.health(name); break;
        case 'inspect': res = await debugApi.inspect(name); break;
        case 'processes': res = await debugApi.top(name); break;
        default: return;
      }
      setTabData(res.data);
    } catch (err) {
      setTabData({ error: err.message });
    } finally {
      setTabLoading(false);
    }
  };

  const runAction = async (action) => {
    try {
      const res = await projects.startJob(name, action, timeout);
      setActionResult({ status: 'running', label: action, job: res.data });
      pollJob(res.data.id, action);
    } catch (err) {
      setActionResult({ status: 'error', label: action, error: err.message });
    }
  };

  const pollJob = (jobId, label) => {
    const tick = async () => {
      try {
        const res = await jobs.get(jobId);
        const job = res.data;
        setActionResult({ status: job.status === 'running' ? 'running' : job.success ? 'done' : 'error', label, job });
        if (job.status === 'running') {
          window.setTimeout(tick, 1500);
        } else {
          fetchProject();
          if (activeTab !== 'overview') loadTab(activeTab);
        }
      } catch (err) {
        setActionResult({ status: 'error', label, error: err.message });
      }
    };
    window.setTimeout(tick, 750);
  };

  const toggleInactive = async () => {
    try {
      const next = !project.inactive;
      setActionResult({ status: 'running', label: next ? 'mark inactive' : 'mark active' });
      await projects.setInactive(name, next);
      setActionResult({ status: 'done', label: next ? 'mark inactive' : 'mark active' });
      fetchProject();
    } catch (err) {
      setActionResult({ status: 'error', label: 'inactive toggle', error: err.message });
    }
  };

  const saveUpdatePolicy = async (event) => {
    event.preventDefault();
    try {
      setActionResult({ status: 'running', label: 'update policy' });
      const res = await projects.setUpdatePolicy(name, policyForm);
      setProject({ ...project, update_policy: res.data });
      setActionResult({ status: 'done', label: 'update policy', result: res.data });
    } catch (err) {
      setActionResult({ status: 'error', label: 'update policy', error: err.message });
    }
  };

  const createBackup = async () => {
    try {
      setActionResult({ status: 'running', label: 'backup' });
      const destinationID = backupDestinationId ? Number(backupDestinationId) : undefined;
      const res = await backup.create(name, destinationID ? { destination_id: destinationID } : {});
      setActionResult({ status: 'done', label: 'backup', result: res.data });
      loadTab('backups');
    } catch (err) {
      setActionResult({ status: 'error', label: 'backup', error: err.message });
    }
  };

  const dumpDatabases = async () => {
    try {
      setActionResult({ status: 'running', label: 'database dump' });
      const res = await dbadmin.dump(name);
      setActionResult({ status: 'done', label: 'database dump', result: res.data });
    } catch (err) {
      setActionResult({ status: 'error', label: 'database dump', error: err.message });
    }
  };

  const runShell = async (event) => {
    event.preventDefault();
    setShellResult({ running: true, output: 'Running...' });
    try {
      const res = await debugApi.shell(name, shellForm);
      setShellResult(res.data);
      setTabData(res.data);
    } catch (err) {
      const failed = { success: false, error: err.message, output: err.message };
      setShellResult(failed);
      setTabData(failed);
    }
  };

  if (loading) return <div className="text-center py-12 text-gray-500">Loading project...</div>;
  if (!project) return <div className="text-center py-12 text-red-700">Project not found</div>;

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div>
          <Link to="/" className="text-sm text-blue-700 hover:underline">Back to dashboard</Link>
          <div className="mt-2 flex flex-wrap items-center gap-2">
            <h1 className="text-2xl font-semibold text-gray-950">{project.name}</h1>
            <Badge tone={project.running ? 'green' : 'gray'}>{project.running ? 'running' : 'stopped'}</Badge>
            {project.inactive && <Badge tone="amber">inactive</Badge>}
            {project.has_hook?.update && <Badge tone="cyan">update hook</Badge>}
            {project.update_policy?.effective_policy === 'no_updates' && <Badge tone="amber">no updates</Badge>}
          </div>
          <p className="mt-1 font-mono text-xs text-gray-500">{project.dir}</p>
        </div>
        <div className="flex flex-wrap gap-2">
          <label className="flex items-center gap-2 text-sm text-gray-700" title="Seconds before pull/update commands time out.">
            Timeout
            <input type="number" min="0" value={timeout} onChange={e => setTimeoutValue(Number(e.target.value))} className="input w-24" />
          </label>
          <button title={project.inactive ? 'Remove the .inactive marker so normal operations include this project.' : 'Create .inactive so normal operations skip this project.'} onClick={toggleInactive} className="btn-secondary">
            {project.inactive ? 'Activate' : 'Deactivate'}
          </button>
          <button title="Refresh project status, containers, hooks, and parsed sources." onClick={fetchProject} className="btn-secondary">Refresh</button>
        </div>
      </div>

      <div className="section-panel">
        <div className="flex flex-wrap gap-2">
          {ACTIONS.map(action => (
            <button key={action.key} title={action.key === 'update' && project.update_policy?.effective_policy === 'no_updates' ? 'Updates are disabled for this project; clicking records a skipped session.' : action.title} onClick={() => runAction(action.key)} className={action.key === 'down' ? 'btn-danger' : 'btn-secondary'}>
              {action.label}
            </button>
          ))}
          <select title="Optional backup endpoint to copy the backup to after it is created locally." value={backupDestinationId} onChange={e => setBackupDestinationId(e.target.value)} className="input max-w-[220px]">
            <option value="">Local only</option>
            {backupDestinations.filter(d => d.enabled).map(d => <option key={d.id} value={d.id}>{d.name} ({d.type})</option>)}
          </select>
          <button title="Create a tar.gz backup of the project directory. Choose a destination to also copy it to configured storage." onClick={createBackup} className="btn-secondary">Backup</button>
          <button title="Dump supported database containers in this project." onClick={dumpDatabases} className="btn-secondary">DB Dump</button>
          {project.is_git && (
            <button
              title="Run git pull --ff-only in the project directory. Only applies for projects that are a git checkout."
              onClick={async () => {
                setActionResult({ status: 'running', label: 'git pull' });
                try {
                  const res = await debugApi.shell(name, { command: 'git-pull', tail: 200, timeout: 120 });
                  const data = res.data || {};
                  setActionResult({ status: data.success ? 'done' : 'error', label: 'git pull', output: data.output, error: data.success ? '' : `exit ${data.exit_code}` });
                  fetchProject();
                } catch (err) {
                  setActionResult({ status: 'error', label: 'git pull', error: err.message });
                }
              }}
              className="btn-secondary"
            >
              git pull
            </button>
          )}
        </div>
      </div>

      {actionResult && <ActionResult result={actionResult} onDismiss={() => setActionResult(null)} />}

      <div className="section-panel">
        <div className="flex flex-wrap gap-1 border-b border-gray-200 pb-3">
          {TABS.map(tab => (
            <button key={tab} title={tabTitle(tab)} onClick={() => loadTab(tab)} className={`rounded-md px-3 py-1.5 text-sm capitalize ${activeTab === tab ? 'bg-blue-700 text-white' : 'text-gray-700 hover:bg-gray-100'}`}>
              {tab}
            </button>
          ))}
        </div>

        <div className="pt-4">
          {activeTab === 'overview' && <Overview project={project} policyForm={policyForm} setPolicyForm={setPolicyForm} saveUpdatePolicy={saveUpdatePolicy} />}

          {activeTab === 'logs' && (
            <div className="mb-4 flex flex-wrap items-end gap-3">
              <Field label="Tail" title="Number of log lines to fetch.">
                <input type="number" min="1" value={logOptions.tail} onChange={e => setLogOptions({ ...logOptions, tail: Number(e.target.value) })} className="input w-28" />
              </Field>
              <Field label="Container" title="Optional container filter. Only containers in this project are allowed.">
                <select value={logOptions.container} onChange={e => setLogOptions({ ...logOptions, container: e.target.value })} className="input w-72">
                  <option value="">All project containers</option>
                  {project.containers?.map(c => <option key={c.name} value={c.name}>{c.name}</option>)}
                </select>
              </Field>
              <button title="Reload logs with the current filters." onClick={() => loadTab('logs')} className="btn-secondary">Load Logs</button>
              <label className="flex items-center gap-2 pb-2 text-sm text-gray-700" title="Reload the current log view every 3 seconds.">
                <input type="checkbox" checked={logOptions.watch} onChange={e => setLogOptions({ ...logOptions, watch: e.target.checked })} />
                Watch
              </label>
            </div>
          )}

          {activeTab === 'shell' && (
            <ShellPanel form={shellForm} setForm={setShellForm} result={shellResult || tabData} runShell={runShell} />
          )}

          {tabLoading && <div className="py-8 text-center text-gray-500">Loading...</div>}
          {!tabLoading && tabData?.error && <div className="rounded border border-red-200 bg-red-50 p-3 text-sm text-red-800">{tabData.error}</div>}
          {!tabLoading && !tabData?.error && activeTab === 'sources' && <Sources data={tabData} />}
          {!tabLoading && !tabData?.error && activeTab === 'sessions' && <Sessions data={tabData} projectName={name} />}
          {!tabLoading && !tabData?.error && activeTab === 'watch' && <WatchTab data={tabData} projectName={name} reload={() => loadTab('watch')} />}
          {!tabLoading && !tabData?.error && activeTab === 'logs' && <Logs data={tabData} />}
          {!tabLoading && !tabData?.error && activeTab === 'stats' && <Stats data={tabData} />}
          {!tabLoading && !tabData?.error && activeTab === 'security' && <Security data={tabData} />}
          {!tabLoading && !tabData?.error && activeTab === 'backups' && <Backups data={tabData} projectName={name} reload={() => loadTab('backups')} setActionResult={setActionResult} />}
          {!tabLoading && !tabData?.error && activeTab === 'databases' && <Databases data={tabData} />}
          {!tabLoading && !tabData?.error && activeTab === 'inspect' && <JsonBlock value={tabData?.inspections || []} />}
          {!tabLoading && !tabData?.error && activeTab === 'processes' && <Processes data={tabData} />}
        </div>
      </div>
    </div>
  );
}

function Overview({ project, policyForm, setPolicyForm, saveUpdatePolicy }) {
  const policy = project.update_policy || {};
  return (
    <div className="space-y-4">
      <div className="grid gap-3 md:grid-cols-2">
        <Info label="Compose file" value={project.compose_file} />
        <Info label="Directory" value={project.dir} />
      </div>
      <form onSubmit={saveUpdatePolicy} className="rounded-md border border-gray-200 p-4">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
          <div className="grid flex-1 gap-3 md:grid-cols-[220px_1fr]">
            <Field label="Update policy" title="Auto marks build-only GitHub/GitLab projects with no registry image as no-updates. Manual modes override detection.">
              <select className="input" value={policyForm.mode} onChange={e => setPolicyForm({ ...policyForm, mode: e.target.value })}>
                <option value="auto">Auto detect</option>
                <option value="allow">Always allow updates</option>
                <option value="no_updates">No updates</option>
              </select>
            </Field>
            <Field label="Notes" title="Optional operator note for why this policy is set.">
              <input className="input" value={policyForm.notes} onChange={e => setPolicyForm({ ...policyForm, notes: e.target.value })} placeholder="optional note" />
            </Field>
          </div>
          <button className="btn-primary">Save Policy</button>
        </div>
        <div className="mt-3 grid gap-2 text-sm text-gray-600 md:grid-cols-2">
          <Info label="Effective policy" value={policy.effective_policy || 'allow'} />
          <Info label="Detected source" value={[policy.detected_source_type, policy.detected_source_url].filter(Boolean).join(' - ') || 'none'} />
          <Info label="Reason" value={policy.no_updates_reason || policy.detected_reason || 'updates allowed'} />
          <Info label="Mode" value={policy.mode || 'auto'} />
        </div>
      </form>
      <div>
        <h2 className="mb-2 text-lg font-semibold text-gray-950">Containers</h2>
        <div className="overflow-x-auto">
          <table className="w-full min-w-[720px] text-left text-sm">
            <thead><tr className="border-b border-gray-200 text-xs uppercase text-gray-500"><th className="py-2">Name</th><th>Image</th><th>State</th><th>Ports</th></tr></thead>
            <tbody>
              {project.containers?.map(c => (
                <tr key={c.name} className="border-b border-gray-100">
                  <td className="py-2 font-mono">{c.name}</td>
                  <td className="font-mono text-xs text-gray-600">{c.image}</td>
                  <td><Badge tone={c.state === 'running' ? 'green' : 'gray'}>{c.state}</Badge></td>
                  <td className="text-xs text-gray-500">{c.ports}</td>
                </tr>
              ))}
            </tbody>
          </table>
          {(!project.containers || project.containers.length === 0) && <div className="py-6 text-gray-500">No running containers found.</div>}
        </div>
      </div>
    </div>
  );
}

function Sources({ data }) {
  const images = data?.images || [];
  return (
    <div className="overflow-x-auto">
      <table className="w-full min-w-[820px] text-left text-sm">
        <thead><tr className="border-b border-gray-200 text-xs uppercase text-gray-500"><th className="py-2">Service</th><th>Source</th><th>Image</th><th>Registry</th><th>Access</th><th>Message</th></tr></thead>
        <tbody>
          {images.map(img => (
            <tr key={img.service} className="border-b border-gray-100 align-top">
              <td className="py-2 font-medium">{img.service}</td>
              <td><Badge tone={img.source_type === 'custom' ? 'violet' : 'blue'}>{img.source_type}</Badge></td>
              <td className="font-mono text-xs text-gray-600">{img.image || img.build_context}</td>
              <td className="font-mono text-xs text-gray-600">{img.registry}</td>
              <td><Badge tone={accessTone(img.access)}>{img.access || 'not checked'}</Badge></td>
              <td className="max-w-[360px] text-xs text-gray-500">{img.message}</td>
            </tr>
          ))}
        </tbody>
      </table>
      {images.length === 0 && <div className="py-6 text-gray-500">No image metadata was parsed.</div>}
    </div>
  );
}

function Sessions({ data, projectName }) {
  const [openId, setOpenId] = useState('');
  const sessions = (Array.isArray(data) ? data : []).filter(job => job.project === projectName)
    .sort((a, b) => new Date(b.started_at) - new Date(a.started_at));
  const open = sessions.find(job => job.id === openId) || sessions[0];
  return (
    <div className="grid gap-4 lg:grid-cols-[320px_1fr]">
      <div className="space-y-2">
        {sessions.map(job => (
          <button key={job.id} onClick={() => setOpenId(job.id)} className={`block w-full rounded-md border p-3 text-left text-sm ${open?.id === job.id ? 'border-blue-300 bg-blue-50' : 'border-gray-200 bg-white hover:bg-gray-50'}`}>
            <div className="flex items-center justify-between gap-2">
              <span className="font-medium">{job.action}</span>
              <Badge tone={job.status === 'running' ? 'blue' : job.success ? 'green' : 'red'}>{job.status}</Badge>
            </div>
            <div className="mt-1 font-mono text-xs text-gray-500">{job.id}</div>
            <div className="mt-1 text-xs text-gray-500">{new Date(job.started_at).toLocaleString()}</div>
          </button>
        ))}
        {sessions.length === 0 && <div className="py-6 text-gray-500">No action sessions saved for this project.</div>}
      </div>
      <div>
        {open ? (
          <>
            <div className="mb-2 flex flex-wrap items-center gap-2 text-sm">
              <Badge tone={open.status === 'running' ? 'blue' : open.success ? 'green' : 'red'}>{open.status}</Badge>
              <span className="font-medium">{open.action}</span>
              <span className="font-mono text-xs text-gray-500">{open.id}</span>
              {open.duration && <span className="text-xs text-gray-500">{open.duration}</span>}
            </div>
            <pre className="max-h-[560px] overflow-auto rounded-md bg-gray-950 p-4 font-mono text-xs text-gray-100 whitespace-pre-wrap">{open.output || 'No output yet.'}</pre>
          </>
        ) : null}
      </div>
    </div>
  );
}

function Logs({ data }) {
  const text = Array.isArray(data) ? data.map(l => l.output).join('\n') : JSON.stringify(data, null, 2);
  return <pre className="max-h-[560px] overflow-auto rounded-md bg-gray-950 p-4 font-mono text-xs text-gray-100 whitespace-pre-wrap">{text}</pre>;
}

function Stats({ data }) {
  return (
    <div className="grid gap-2">
      {data?.stats?.map((s, i) => (
        <div key={i} className="grid gap-2 rounded-md border border-gray-200 p-3 text-sm md:grid-cols-6">
          <span className="font-mono">{s.container}</span>
          <span>CPU {s.cpu}</span>
          <span>Memory {s.memory}</span>
          <span>Mem {s.mem_percent}</span>
          <span>Net {s.net_io}</span>
          <span>PIDs {s.pids}</span>
        </div>
      ))}
      {(!data?.stats || data.stats.length === 0) && <div className="py-6 text-gray-500">No running containers to measure.</div>}
    </div>
  );
}

function ShellPanel({ form, setForm, result, runShell }) {
  const commands = [
    { value: 'ps', label: 'Compose ps', title: 'Show project container state.' },
    { value: 'config', label: 'Validate config', title: 'Run docker compose config.' },
    { value: 'up', label: 'Start', title: 'Run docker compose up -d.' },
    { value: 'recreate', label: 'Recreate network + start', title: 'Run docker compose down --remove-orphans, then docker compose up -d. Useful for stale Docker network errors.' },
    { value: 'pull', label: 'Pull', title: 'Run docker compose pull.' },
    { value: 'restart', label: 'Restart', title: 'Run docker compose restart.' },
    { value: 'logs', label: 'Logs', title: 'Run docker compose logs with timestamps.' },
    { value: 'down', label: 'Stop', title: 'Run docker compose down.' },
    { value: 'git-status', label: 'git status', title: 'git status --short --branch (requires a .git repo in the project dir).' },
    { value: 'git-fetch', label: 'git fetch', title: 'git fetch --all --prune (requires a .git repo).' },
    { value: 'git-pull', label: 'git pull', title: 'git pull --ff-only in the project directory (requires a .git repo).' },
    { value: 'git-log', label: 'git log (last 20)', title: 'git log --oneline --decorate -n 20.' },
    { value: 'git-remote', label: 'git remote -v', title: 'Show configured git remotes and URLs.' },
  ];
  return (
    <div className="space-y-4">
      <form onSubmit={runShell} className="grid gap-3 md:grid-cols-[260px_120px_120px_auto] md:items-end">
        <Field label="Command" title="Scoped troubleshooting command. Raw shell input is not accepted.">
          <select className="input" value={form.command} onChange={e => setForm({ ...form, command: e.target.value })}>
            {commands.map(command => <option key={command.value} value={command.value}>{command.label}</option>)}
          </select>
        </Field>
        <Field label="Tail" title="Log lines for the Logs command.">
          <input className="input" type="number" min="1" max="2000" value={form.tail} onChange={e => setForm({ ...form, tail: Number(e.target.value) })} />
        </Field>
        <Field label="Timeout" title="Seconds before the troubleshooting command is stopped.">
          <input className="input" type="number" min="1" max="600" value={form.timeout} onChange={e => setForm({ ...form, timeout: Number(e.target.value) })} />
        </Field>
        <button className="btn-secondary" title={commands.find(c => c.value === form.command)?.title || 'Run command'}>Run</button>
      </form>
      <div className="rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800">
        Commands run from this project directory using Docker Compose. For root-owned projects, make the compose files readable by the Stack Manager service UID or manage them from a root-capable agent.
      </div>
      {result && (
        <div>
          <div className="mb-2 flex items-center gap-2 text-sm">
            <Badge tone={result.success ? 'green' : result.running ? 'blue' : 'red'}>{result.running ? 'running' : result.success ? 'success' : 'failed'}</Badge>
            {typeof result.exit_code === 'number' && <span className="text-gray-500">exit {result.exit_code}</span>}
          </div>
          <pre className="max-h-[560px] overflow-auto rounded-md bg-gray-950 p-4 font-mono text-xs text-gray-100 whitespace-pre-wrap">{result.output || result.error || ''}</pre>
        </div>
      )}
    </div>
  );
}

function Security({ data }) {
  return (
    <div>
      <div className="mb-4 flex flex-wrap gap-2">
        {Object.entries(data?.summary || {}).map(([severity, count]) => <Badge key={severity} tone={severityTone(severity)}>{severity}: {count}</Badge>)}
      </div>
      <div className="space-y-2">
        {data?.findings?.map((f, i) => (
          <div key={i} className="rounded-md border border-gray-200 p-3 text-sm">
            <Badge tone={severityTone(f.severity)}>{f.severity}</Badge>
            <span className="ml-2 text-xs text-gray-500">{f.category}</span>
            <div className="mt-1">{f.description}</div>
          </div>
        ))}
        {(!data?.findings || data.findings.length === 0) && <div className="py-6 text-green-700">No security findings returned.</div>}
      </div>
    </div>
  );
}

function Backups({ data, projectName, reload, setActionResult }) {
  const downloadBackup = async (id) => {
    try {
      setActionResult({ status: 'running', label: `download ${id}` });
      await backup.download(id);
      setActionResult({ status: 'done', label: `download ${id}` });
    } catch (err) {
      setActionResult({ status: 'error', label: `download ${id}`, error: err.message });
    }
  };
  const restoreBackup = async (id) => {
    try {
      setActionResult({ status: 'running', label: `restore ${id}` });
      const res = await backup.restore(projectName, id);
      setActionResult({ status: 'done', label: `restore ${id}`, result: res.data });
      reload();
    } catch (err) {
      setActionResult({ status: 'error', label: `restore ${id}`, error: err.message });
    }
  };
  const deleteBackup = async (id) => {
    try {
      setActionResult({ status: 'running', label: `delete ${id}` });
      const res = await backup.delete(id);
      setActionResult({ status: 'done', label: `delete ${id}`, result: res.data });
      reload();
    } catch (err) {
      setActionResult({ status: 'error', label: `delete ${id}`, error: err.message });
    }
  };
  return (
    <div className="space-y-2">
      {Array.isArray(data) && data.map(b => (
        <div key={b.id} className="flex flex-col gap-2 rounded-md border border-gray-200 p-3 text-sm md:flex-row md:items-center md:justify-between">
          <div>
            <div className="font-mono">{b.id}</div>
            <div className="text-xs text-gray-500">{(b.size_bytes / 1024 / 1024).toFixed(1)} MB · {new Date(b.created_at).toLocaleString()}</div>
          </div>
          <div className="flex gap-2">
            <button title="Download this local backup archive from the Stack Manager server." onClick={() => downloadBackup(b.id)} className="mini-button">Download</button>
            <button title="Restore this backup, stop running containers first, then start the project." onClick={() => restoreBackup(b.id)} className="mini-button">Restore</button>
            <button title="Delete this backup archive." onClick={() => deleteBackup(b.id)} className="mini-danger">Delete</button>
          </div>
        </div>
      ))}
      {(!data || data.length === 0) && <div className="py-6 text-gray-500">No backups found.</div>}
    </div>
  );
}

function Databases({ data }) {
  return (
    <div className="space-y-2">
      {data?.checks?.map((c, i) => (
        <div key={i} className="rounded-md border border-gray-200 p-3 text-sm">
          <Badge tone={c.healthy ? 'green' : 'red'}>{c.healthy ? 'healthy' : 'unhealthy'}</Badge>
          <span className="ml-2 font-mono">{c.container}</span>
          <span className="ml-2 text-gray-500">{c.engine}</span>
          {c.output && <pre className="mt-2 whitespace-pre-wrap font-mono text-xs text-gray-500">{c.output}</pre>}
        </div>
      ))}
      {(!data?.checks || data.checks.length === 0) && <div className="py-6 text-gray-500">No supported database containers found.</div>}
    </div>
  );
}

function Processes({ data }) {
  return (
    <div className="space-y-3">
      {data?.processes?.map(p => (
        <div key={p.container}>
          <div className="mb-1 font-mono text-sm">{p.container}</div>
          <pre className="max-h-72 overflow-auto rounded-md bg-gray-950 p-3 font-mono text-xs text-gray-100 whitespace-pre-wrap">{p.output}</pre>
        </div>
      ))}
      {(!data?.processes || data.processes.length === 0) && <div className="py-6 text-gray-500">No process output returned.</div>}
    </div>
  );
}

function Info({ label, value }) {
  return (
    <div className="rounded-md border border-gray-200 p-3">
      <div className="text-xs uppercase text-gray-500">{label}</div>
      <div className="mt-1 break-all font-mono text-sm text-gray-800">{value}</div>
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

function JsonBlock({ value }) {
  return <pre className="max-h-[560px] overflow-auto rounded-md bg-gray-950 p-4 font-mono text-xs text-gray-100 whitespace-pre-wrap">{JSON.stringify(value, null, 2)}</pre>;
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
          {result.result?.destination && (
            <div className="mt-1 text-xs">
              Destination: <span className="font-mono">{result.result.destination.destination_name}</span> · {result.result.destination.success ? 'copied' : result.result.destination.error || 'failed'}
              {result.result.destination.target && <span> · <span className="font-mono">{result.result.destination.target}</span></span>}
            </div>
          )}
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

function tabTitle(tab) {
  const titles = {
    overview: 'Project files and running containers.',
    sources: 'Classify custom builds, public registry images, and private images.',
    watch: 'Up + Watch: run docker compose up -d then tail the live startup log. Refresh-safe.',
    logs: 'Read docker compose logs with optional container and tail filters.',
    stats: 'Show docker stats for running containers.',
    shell: 'Run scoped troubleshooting commands in this project directory.',
    security: 'Scan images and audit compose configuration.',
    backups: 'List, restore, and delete project backups.',
    databases: 'Check supported database containers.',
    inspect: 'Show docker inspect JSON.',
    processes: 'Show docker top output.',
    sessions: 'Show live and completed action output sessions.',
  };
  return titles[tab] || tab;
}

const SERVICE_COLORS = ['text-emerald-400', 'text-sky-400', 'text-amber-400', 'text-fuchsia-400', 'text-orange-400', 'text-lime-400', 'text-rose-400', 'text-cyan-400', 'text-indigo-400', 'text-yellow-400'];

function serviceColorFor(service) {
  if (!service) return 'text-gray-300';
  let hash = 0;
  for (let i = 0; i < service.length; i++) hash = (hash * 31 + service.charCodeAt(i)) & 0xffffffff;
  return SERVICE_COLORS[Math.abs(hash) % SERVICE_COLORS.length];
}

function parseWatchLine(line) {
  const match = line.match(/^([a-zA-Z0-9._-]+-\d+)\s*\|\s*(.*)$/);
  if (match) return { service: match[1], text: match[2] };
  return { service: '', text: line };
}

function WatchTab({ data, projectName, reload }) {
  const sessions = Array.isArray(data) ? data : [];
  const [selectedId, setSelectedId] = useState(sessions[0]?.id || '');
  const [lines, setLines] = useState([]);
  const [autoScroll, setAutoScroll] = useState(true);
  const [streaming, setStreaming] = useState(false);
  const [error, setError] = useState('');
  const [starting, setStarting] = useState(false);
  const preRef = useRef(null);
  const esRef = useRef(null);

  useEffect(() => {
    if (sessions.length && !selectedId) setSelectedId(sessions[0].id);
  }, [sessions, selectedId]);

  useEffect(() => {
    if (!selectedId) return undefined;
    setLines([]);
    setError('');
    setStreaming(true);
    const url = watchApi.streamUrl(projectName, selectedId);
    const es = new EventSource(url, { withCredentials: false });
    esRef.current = es;
    es.onmessage = ev => {
      setLines(prev => [...prev, parseWatchLine(ev.data)]);
    };
    es.addEventListener('end', () => {
      setStreaming(false);
      es.close();
    });
    es.addEventListener('error', ev => {
      setError('stream error');
      setStreaming(false);
      es.close();
    });
    es.onerror = () => {
      setStreaming(false);
      es.close();
    };
    return () => {
      es.close();
      esRef.current = null;
    };
  }, [selectedId, projectName]);

  useEffect(() => {
    if (!autoScroll) return;
    if (preRef.current) preRef.current.scrollTop = preRef.current.scrollHeight;
  }, [lines, autoScroll]);

  const startNew = async () => {
    setStarting(true);
    setError('');
    try {
      const res = await watchApi.start(projectName);
      const newId = res.data?.session?.id;
      await reload();
      if (newId) setSelectedId(newId);
    } catch (err) {
      setError(err.message);
    } finally {
      setStarting(false);
    }
  };

  const stopSession = async () => {
    if (!selectedId) return;
    try { await watchApi.stop(projectName, selectedId); } catch (err) { setError(err.message); }
    reload();
  };

  const selected = sessions.find(s => s.id === selectedId);

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-end gap-3">
        <button className="btn-primary" onClick={startNew} disabled={starting} title="Run docker compose up -d and start streaming the live startup log. Every service line is color-coded so multi-container startups are easy to read.">
          {starting ? 'Starting…' : 'Up + Watch'}
        </button>
        <Field label="Past sessions" title="Every Up + Watch stores its log on disk. Reopen any past session to replay the exact output.">
          <select className="input w-72" value={selectedId} onChange={e => setSelectedId(e.target.value)}>
            {sessions.length === 0 && <option value="">No sessions yet</option>}
            {sessions.map(s => (
              <option key={s.id} value={s.id}>
                {new Date(s.started_at).toLocaleString()} · {s.running ? 'live' : `exit ${s.exit_code}`}
              </option>
            ))}
          </select>
        </Field>
        {selected?.running && (
          <button className="btn-secondary" onClick={stopSession} title="Kill the log follower for this session. The log file stays on disk.">Stop follower</button>
        )}
        <label className="ml-auto flex items-center gap-2 pb-2 text-sm text-gray-700" title="Uncheck to read history without the view jumping.">
          <input type="checkbox" checked={autoScroll} onChange={e => setAutoScroll(e.target.checked)} />
          Auto-scroll
        </label>
      </div>
      {error && <div className="rounded border border-red-200 bg-red-50 p-3 text-sm text-red-800">{error}</div>}
      {selected && (
        <div className="flex flex-wrap gap-3 text-xs text-gray-500">
          <span>Session <code className="rounded bg-gray-100 px-1 py-0.5">{selected.id}</code></span>
          <span>{selected.running ? <span className="rounded bg-green-100 px-1.5 py-0.5 text-green-800">streaming</span> : <span>ended · exit {selected.exit_code}</span>}</span>
          {selected.size_bytes > 0 && <span>{selected.size_bytes.toLocaleString()} bytes</span>}
        </div>
      )}
      <pre ref={preRef} className="max-h-[560px] overflow-auto rounded-md bg-gray-950 p-4 font-mono text-xs leading-relaxed">
        {lines.length === 0 ? (
          <span className="text-gray-500">{streaming ? 'Waiting for first log line…' : selectedId ? 'Empty log for this session.' : 'Click Up + Watch to start a new session.'}</span>
        ) : lines.map((line, idx) => (
          <div key={idx}>
            {line.service && <span className={`${serviceColorFor(line.service)} font-semibold`}>{line.service.padEnd(28, ' ')}</span>}
            <span className="text-gray-100">{line.service ? ' | ' : ''}{line.text}</span>
          </div>
        ))}
      </pre>
    </div>
  );
}

function accessTone(access) {
  if (access === 'public' || access === 'private-authenticated' || access === 'local-build') return 'green';
  if (access === 'private-login-required') return 'amber';
  if (access === 'not-found' || access === 'inaccessible') return 'red';
  return 'gray';
}

function severityTone(severity) {
  if (severity === 'critical' || severity === 'high') return 'red';
  if (severity === 'medium') return 'amber';
  if (severity === 'low') return 'blue';
  return 'gray';
}
