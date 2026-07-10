import { useState, useEffect, useRef, useCallback } from 'react';
import { useParams, Link, useLocation, useNavigate } from 'react-router-dom';
import { projects, projectsForSource, agents as agentsApi, jobs, jobsForSource, debug as debugApi, security, backup, dbadmin, watch as watchApi, firewall as firewallApi, proxy as proxyApi } from '../api/client';

// parsePublishedPorts pulls unique host-published TCP ports from a project's
// containers (Docker port strings like "0.0.0.0:8080->80/tcp").
function parsePublishedPorts(project) {
  const ports = new Set();
  for (const c of project?.containers || []) {
    for (const mapping of (c.ports || '').split(',')) {
      const m = mapping.trim().match(/:(\d+)->\d+\/tcp/);
      if (m && m[1] !== '0') ports.add(m[1]);
    }
  }
  return [...ports];
}
import { useFollowingScroll } from '../hooks/useFollowingScroll';
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import '@xterm/xterm/css/xterm.css';

const TABS = ['overview', 'config', 'docs', 'sessions', 'sources', 'watch', 'logs', 'stats', 'shell', 'security', 'backups', 'databases', 'inspect', 'processes'];
const ACTIONS = [
  { key: 'update', label: 'Update', title: 'Pull and recreate containers, unless an update hook exists.' },
  { key: 'pull', label: 'Pull', title: 'Pull images without restarting containers.' },
  { key: 'up', label: 'Start', title: 'Run docker compose up -d.' },
  { key: 'restart', label: 'Restart', title: 'Restart project containers.' },
  { key: 'down', label: 'Stop', title: 'Run docker compose down.' },
];

const updateBlockedReason = (project) => {
  if (project.update_policy?.effective_policy === 'no_updates') return project.update_policy?.no_updates_reason || 'Updates are disabled for this project.';
  if (!project.update_status?.checked) return 'No update check has run yet.';
  if (!project.update_status?.available) return 'No image updates were available at the last check.';
  return '';
};

const canRunImageUpdate = (project) => updateBlockedReason(project) === '';

const updateStatusLabel = (project) => {
  const status = project.update_status || {};
  if (project.update_policy?.effective_policy === 'no_updates') return 'disabled';
  if (!status.checked) return 'not checked';
  if (status.available) return `${status.count || 1} available`;
  if (status.error) return 'check warning';
  return 'current';
};

const updateStatusTone = (project) => {
  const status = project.update_status || {};
  if (project.update_policy?.effective_policy === 'no_updates') return 'amber';
  if (!status.checked) return 'gray';
  if (status.available) return 'green';
  if (status.error) return 'amber';
  return 'gray';
};

export default function ProjectDetail() {
  const { name } = useParams();
  const location = useLocation();
  const navigate = useNavigate();
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
  const startupJobRef = useRef('');
  // A project may live on a peer controller. When it does, every project-level
  // call is routed through the local controller's agent-proxy to that peer
  // (papiRef). sourceInfo.agentId is null for local projects. Tabs that stream
  // (logs/stats/shell) or use skills (security/backups/databases) aren't proxied
  // yet, so they show a notice for remote projects.
  const papiRef = useRef(projects);
  const [sourceInfo, setSourceInfo] = useState({ resolved: false, agentId: null, name: '', callback: false });
  const [agentCommands, setAgentCommands] = useState([]);
  const isRemote = !!sourceInfo.agentId;
  // Callback agents are push-only: we can't call them live, so their project is
  // read from the last check-in snapshot and actions are queued for the agent to
  // run on its next check-in.
  const isCallback = sourceInfo.callback;
  const REMOTE_ONLY_LOCAL_TABS = ['logs', 'stats', 'shell', 'security', 'backups', 'databases', 'inspect', 'processes', 'watch'];
  // Tracks the last location.search we applied a tab from, so the URL effect
  // fires only on real URL changes — not every time activeTab changes (which
  // used to snap the user back to the URL's ?tab= value on any tab click).
  const appliedSearchRef = useRef(null);

  // Resolve which controller owns this project and return a project API scoped
  // to it. Prefers the ?source=<agentName> hint from the dashboard link; falls
  // back to searching all servers when the page is opened by a bare URL.
  const resolveProjectApi = async () => {
    const params = new URLSearchParams(location.search);
    const sourceName = params.get('source') || '';
    if (sourceName && sourceName !== 'local') {
      const list = await agentsApi.list().catch(() => null);
      const agent = list?.data?.find(a => a.name === sourceName);
      if (agent) return agentTarget(agent);
    }
    return { api: projects, agentId: null, name: '', callback: false };
  };

  // agentTarget decides how to talk to a project's owning agent: peers and
  // inbound/both agents are called live through the proxy; callback agents are
  // push-only, so their project comes from the check-in snapshot and actions are
  // queued.
  const agentTarget = (agent) => {
    const live = !!agent.base_url && agent.mode !== 'callback';
    return {
      api: live ? projectsForSource(agent.id) : projects,
      agentId: agent.id,
      name: agent.name,
      callback: !live,
    };
  };

  const loadAgentCommands = async (agentId) => {
    const res = await agentsApi.commands(agentId, name).catch(() => ({ data: [] }));
    setAgentCommands(res.data || []);
  };

  const fetchProject = async () => {
    try {
      let { api, agentId, name: srcName, callback } = await resolveProjectApi();
      papiRef.current = api;
      let res;
      if (callback) {
        // Read the project from the agent's last check-in snapshot.
        const snap = await agentsApi.projects(agentId).catch(() => ({ data: [] }));
        const found = (snap.data || []).find(p => p.name === name);
        if (!found) throw new Error(`"${name}" isn't in ${srcName}'s last check-in yet.`);
        res = { data: found };
      } else {
        try {
          res = await api.get(name);
        } catch (err) {
          // Bare-URL fallback: a local miss means the project lives on a peer/agent.
          if (!agentId && err.status === 404) {
            const [all, agentList] = await Promise.all([
              projects.list({ source: 'all', include_inactive: 'true' }).catch(() => null),
              agentsApi.list().catch(() => null),
            ]);
            const owner = all?.data?.find(p => p.name === name && p.source_host && p.source_host !== 'local');
            const agent = owner && agentList?.data?.find(a => a.name === owner.source_host);
            if (agent) {
              ({ api, agentId, name: srcName, callback } = agentTarget(agent));
              papiRef.current = api;
              if (callback) {
                const snap = await agentsApi.projects(agentId).catch(() => ({ data: [] }));
                const found = (snap.data || []).find(p => p.name === name);
                if (!found) throw err;
                res = { data: found };
              } else {
                res = await api.get(name);
              }
            } else {
              throw err;
            }
          } else {
            throw err;
          }
        }
      }
      setSourceInfo({ resolved: true, agentId, name: srcName, callback });
      setProject(res.data);
      if (callback && agentId) loadAgentCommands(agentId);
      const destinationRes = await backup.destinations().catch(() => ({ data: [] }));
      setBackupDestinations(destinationRes.data || []);
      setPolicyForm({
        mode: res.data.update_policy?.mode || 'auto',
        notes: res.data.update_policy?.notes || '',
      });
    } catch (err) {
      setSourceInfo(s => ({ ...s, resolved: true }));
      setActionResult({ status: 'error', label: 'load project', error: err.message });
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { fetchProject(); }, [name, location.search]);

  useEffect(() => {
    const params = new URLSearchParams(location.search);
    const jobId = params.get('job');
    if (!jobId || startupJobRef.current === jobId) return;
    startupJobRef.current = jobId;
    const label = params.get('action') || 'up';
    setActiveTab('sessions');
    setActionResult({ status: 'running', label, job: { id: jobId, status: 'running' } });
    pollJob(jobId, label);
  }, [location.search]);

  useEffect(() => {
    // Only react to genuine URL changes. Keying off activeTab here caused the
    // tab bar to be unusable: clicking a tab set activeTab, re-ran this effect,
    // and the still-present ?tab= in the URL yanked the user back.
    if (appliedSearchRef.current === location.search) return;
    appliedSearchRef.current = location.search;
    const params = new URLSearchParams(location.search);
    if (params.get('job')) return;
    const tab = params.get('tab');
    if (tab && TABS.includes(tab)) loadTab(tab);
  }, [location.search]);

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
    // Callback agents expose only the snapshot: everything else needs a live
    // connection we don't have. Actions are queued from the overview instead.
    if (isCallback) {
      setTabData({ error: `${sourceInfo.name} is a check-in (callback) agent. Only the overview and queued actions are available here — run logs, shell, and scans on ${sourceInfo.name} directly.` });
      return;
    }
    // Streaming and skill tabs aren't proxied to peers yet — show a clear
    // message (via the standard error panel) instead of a confusing local
    // failure when viewing a remote project.
    if (isRemote && REMOTE_ONLY_LOCAL_TABS.includes(tab)) {
      setTabData({ error: `The ${tab} view isn't proxied to remote servers yet. Open ${name} on ${sourceInfo.name || 'the peer'} directly for logs, shell, scans, and backups.` });
      return;
    }
    setTabLoading(true);
    try {
      let res;
      switch (tab) {
        case 'docs': res = await papiRef.current.docs(name); break;
        case 'sources': res = await papiRef.current.images(name); break;
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

  // Tab clicks go through here so the URL's ?tab= stays in sync with the view
  // (reloads and bookmarks land on the right tab). We preempt appliedSearchRef
  // to the new search string so the URL effect doesn't re-load the same tab.
  const selectTab = (tab) => {
    const params = new URLSearchParams(location.search);
    params.delete('job');
    if (tab === 'overview') params.delete('tab'); else params.set('tab', tab);
    const search = params.toString() ? `?${params.toString()}` : '';
    appliedSearchRef.current = search;
    loadTab(tab);
    navigate(`${location.pathname}${search}`, { replace: true });
  };

  // pollAgentCommands refreshes the queued-command list for a while so the
  // pending -> dispatched -> done transition is visible without a manual reload.
  const pollAgentCommands = () => {
    let n = 0;
    const id = window.setInterval(() => {
      n += 1;
      loadAgentCommands(sourceInfo.agentId);
      if (n >= 20) window.clearInterval(id);
    }, 5000);
  };

  const runAction = async (action) => {
    if (isCallback) {
      // Callback agents can't be driven live; queue the action for the agent to
      // run on its next check-in.
      try {
        await agentsApi.enqueueCommand(sourceInfo.agentId, { project: name, action, params: JSON.stringify({ timeout }) });
        setActionResult({ status: 'done', label: `queued ${action}`, result: { output: `Queued "${action}" for ${sourceInfo.name}. It runs on the agent's next check-in (usually within a minute).` } });
        loadAgentCommands(sourceInfo.agentId);
        pollAgentCommands();
      } catch (err) {
        setActionResult({ status: 'error', label: `queue ${action}`, error: err.message });
      }
      return;
    }
    try {
      const res = await papiRef.current.startJob(name, action, timeout);
      setActionResult({ status: 'running', label: action, job: res.data });
      pollJob(res.data.id, action);
    } catch (err) {
      setActionResult({ status: 'error', label: action, error: err.message });
    }
  };

  // Open this project's published ports in the host CSF firewall.
  const openFirewallPorts = async () => {
    const ports = parsePublishedPorts(project);
    if (ports.length === 0) {
      setActionResult({ status: 'error', label: 'firewall ports', error: 'No published TCP ports found for this project.' });
      return;
    }
    if (!window.confirm(`Open ${ports.length} port${ports.length === 1 ? '' : 's'} (${ports.join(', ')}) inbound in the host CSF firewall?`)) return;
    try {
      setActionResult({ status: 'running', label: 'firewall ports' });
      const res = await firewallApi.allowPorts(ports, 'tcp');
      setActionResult({ status: 'done', label: 'firewall ports', result: res.data });
    } catch (err) {
      setActionResult({ status: 'error', label: 'firewall ports', error: err.message });
    }
  };

  // Create an NPM proxy host for this project (domain = project name, editable
  // in NPM after). Forwards to this host and the project's first published port.
  const addToProxy = async () => {
    const ports = parsePublishedPorts(project);
    if (ports.length === 0) {
      setActionResult({ status: 'error', label: 'add to proxy', error: 'No published TCP ports found for this project.' });
      return;
    }
    const status = await proxyApi.status().catch(() => ({ data: {} }));
    if (!status.data?.connected) {
      setActionResult({ status: 'error', label: 'add to proxy', error: 'Nginx Proxy Manager is not connected. Set it up in Settings > Reverse Proxy first.' });
      return;
    }
    const port = ports[0];
    if (!window.confirm(`Create an NPM proxy host for "${name}" forwarding to ${window.location.hostname}:${port}? You can set the real domain and SSL in NPM afterward.`)) return;
    try {
      setActionResult({ status: 'running', label: 'add to proxy' });
      await proxyApi.createHost({
        domain_names: [name], forward_scheme: 'http', forward_host: window.location.hostname, forward_port: Number(port),
        enabled: true, block_exploits: true, allow_websocket_upgrade: true, access_list_id: 0, certificate_id: 0,
        meta: { letsencrypt_agree: false, dns_challenge: false }, advanced_config: '', locations: [],
        caching_enabled: false, ssl_forced: false, http2_support: false, hsts_enabled: false, hsts_subdomains: false,
      });
      setActionResult({ status: 'done', label: 'add to proxy', result: { output: `Created proxy host "${name}" -> ${window.location.hostname}:${port}. Edit the domain and enable SSL in NPM.` } });
    } catch (err) {
      setActionResult({ status: 'error', label: 'add to proxy', error: err.message });
    }
  };

  const pollJob = (jobId, label) => {
    const japi = jobsForSource(sourceInfo.agentId);
    const tick = async () => {
      try {
        const res = await japi.get(jobId);
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
      await papiRef.current.setInactive(name, next);
      setActionResult({ status: 'done', label: next ? 'mark inactive' : 'mark active' });
      fetchProject();
    } catch (err) {
      setActionResult({ status: 'error', label: 'inactive toggle', error: err.message });
    }
  };

  const deleteProject = async () => {
    const confirmName = window.prompt(`Type ${name} to delete the whole project directory. This runs docker compose down first and then removes ${project?.dir || 'the project folder'} — data volumes referenced by the compose file are kept unless you pass down -v elsewhere.`);
    if (confirmName !== name) {
      setActionResult({ status: 'error', label: `delete ${name}`, error: 'Project name confirmation did not match.' });
      return;
    }
    try {
      setActionResult({ status: 'running', label: `delete ${name}` });
      await papiRef.current.delete(name, { confirm_name: confirmName, stop_first: true });
      // Navigate back to the Dashboard; the current page 404s once the
      // directory is gone.
      navigate('/');
    } catch (err) {
      setActionResult({ status: 'error', label: `delete ${name}`, error: err.message, result: err.data });
      fetchProject();
    }
  };

  const saveUpdatePolicy = async (event) => {
    event.preventDefault();
    try {
      setActionResult({ status: 'running', label: 'update policy' });
      const res = await papiRef.current.setUpdatePolicy(name, policyForm);
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
            {isRemote && (
              <span title={isCallback
                ? `This project runs on "${sourceInfo.name}", a check-in (callback) agent. It's shown from the agent's last check-in; actions are queued and run on the agent's next check-in.`
                : `This project runs on the peer/agent "${sourceInfo.name}". Lifecycle actions, config, files, sources, and docs are proxied to it. Logs, shell, scans, and backups aren't proxied yet — open it on ${sourceInfo.name} directly.`}
                className="inline-flex items-center gap-1 rounded-md bg-violet-100 px-2 py-0.5 text-xs font-semibold text-violet-800 ring-1 ring-violet-200">
                on {sourceInfo.name}{isCallback ? ' · check-in' : ''}
              </span>
            )}
            <Badge tone={project.running ? 'green' : 'gray'}>{project.running ? 'running' : 'stopped'}</Badge>
            {project.inactive && <Badge tone="amber">inactive</Badge>}
            {project.has_hook?.update && <Badge tone="cyan">update hook</Badge>}
            {project.update_policy?.effective_policy === 'no_updates' && <Badge tone="amber">no updates</Badge>}
            <Badge tone={updateStatusTone(project)}>{updateStatusLabel(project)}</Badge>
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
          {project.inactive && (
            <button title="Delete the whole project directory after exact-name confirmation. Runs docker compose down first. Volumes referenced by the compose file are kept." onClick={deleteProject} className="btn-danger">
              Delete
            </button>
          )}
          <button title="Refresh project status, containers, hooks, and parsed sources." onClick={fetchProject} className="btn-secondary">Refresh</button>
        </div>
      </div>

      <div className="section-panel">
        <div className="flex flex-wrap gap-2">
          {ACTIONS.map(action => {
            const imageActionBlocked = (action.key === 'update' || action.key === 'pull') && !canRunImageUpdate(project);
            return (
            <button key={action.key} disabled={imageActionBlocked} title={imageActionBlocked ? updateBlockedReason(project) : action.title} onClick={() => runAction(action.key)} className={`${action.key === 'down' ? 'btn-danger' : 'btn-secondary'} ${imageActionBlocked ? 'opacity-50' : ''}`}>
              {action.label}
            </button>
          )})}
          <select title="Optional backup endpoint to copy the backup to after it is created locally." value={backupDestinationId} onChange={e => setBackupDestinationId(e.target.value)} className="input max-w-[220px]">
            <option value="">Local only</option>
            {backupDestinations.filter(d => d.enabled).map(d => <option key={d.id} value={d.id}>{d.name} ({d.type})</option>)}
          </select>
          {/* Backup, DB dump, firewall and proxy act on the controller's own
              host, so they're hidden for projects that live on a peer — run
              them from that peer's dashboard instead. */}
          {!isRemote && <button title="Create a tar.gz backup of the project directory. Choose a destination to also copy it to configured storage." onClick={createBackup} className="btn-secondary">Backup</button>}
          {!isRemote && <button title="Dump supported database containers in this project." onClick={dumpDatabases} className="btn-secondary">DB Dump</button>}
          {!isRemote && <button title="Open this project's published TCP ports inbound in the host CSF firewall." onClick={openFirewallPorts} className="btn-secondary">Open Ports (CSF)</button>}
          {!isRemote && <button title="Create an Nginx Proxy Manager proxy host for this project (requires NPM connected in Settings). Domain and SSL are editable in NPM after." onClick={addToProxy} className="btn-secondary">Add to Proxy (NPM)</button>}
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
            <button key={tab} title={tabTitle(tab)} onClick={() => selectTab(tab)} className={`rounded-md px-3 py-1.5 text-sm capitalize ${activeTab === tab ? 'bg-blue-700 text-white' : 'text-gray-700 hover:bg-gray-100'}`}>
              {tab}
            </button>
          ))}
        </div>

        <div className="pt-4">
          {activeTab === 'overview' && <Overview project={project} policyForm={policyForm} setPolicyForm={setPolicyForm} saveUpdatePolicy={saveUpdatePolicy} />}
          {activeTab === 'overview' && isCallback && (
            <QueuedCommands commands={agentCommands} agentName={sourceInfo.name} onRefresh={() => loadAgentCommands(sourceInfo.agentId)} />
          )}
          {activeTab === 'config' && <ConfigEditor projectName={name} sourceAgentId={sourceInfo.agentId} />}

          {activeTab === 'docs' && <ProjectDocs docs={tabData || project.documentation || []} projectName={name} sourceAgentId={sourceInfo.agentId} />}

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

          {activeTab === 'shell' && !isRemote && (
            <div className="space-y-4">
              <ShellPanel form={shellForm} setForm={setShellForm} result={shellResult || tabData} runShell={runShell} />
              <InteractiveShell projectName={name} />
            </div>
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
        <Info label="Update status" value={updateStatusLabel(project)} />
        <Info label="Last update check" value={project.update_status?.checked_at ? formatDate(project.update_status.checked_at) : 'never'} />
      </div>
      {project.update_status?.images?.some(check => check.update_available) && (
        <div className="rounded-md border border-green-200 bg-green-50 p-4">
          <h2 className="text-sm font-semibold text-green-950">Available image updates</h2>
          <div className="mt-2 space-y-2 text-xs">
            {project.update_status.images.filter(check => check.update_available).map(check => (
              <div key={`${check.service}:${check.image}`}>
                <div className="font-medium text-green-950">{check.service}</div>
                <div className="break-all font-mono text-green-800">{check.image}</div>
              </div>
            ))}
          </div>
        </div>
      )}
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
            <thead><tr className="border-b border-gray-200 text-xs uppercase text-gray-500"><th className="py-2">Name</th><th>Image</th><th>State</th><th title="Published host ports are clickable links that open the service in a new tab. Locked/grey ports are internal to the Docker network and not reachable from a browser.">Ports</th></tr></thead>
            <tbody>
              {project.containers?.map(c => (
                <tr key={c.name} className="border-b border-gray-100">
                  <td className="py-2 font-mono">{c.name}</td>
                  <td className="font-mono text-xs text-gray-600">{c.image}</td>
                  <td><Badge tone={c.state === 'running' ? 'green' : 'gray'}>{c.state}</Badge></td>
                  <td className="text-xs"><ContainerPorts ports={c.ports} state={c.state} /></td>
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

const HTTPS_HOST_PORTS = new Set(['443', '8443', '8993', '9443']);

// ExternalLinkIcon / LockIcon are tiny inline glyphs so a port chip reads at a
// glance as either "opens in a new tab" or "internal, not reachable".
const ExternalLinkIcon = ({ className = 'h-3 w-3' }) => (
  <svg viewBox="0 0 24 24" className={className} fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round">
    <path d="M14 4h6v6" /><path d="M20 4 10 14" /><path d="M19 14v5a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V6a1 1 0 0 1 1-1h5" />
  </svg>
);
const LockIcon = ({ className = 'h-3 w-3' }) => (
  <svg viewBox="0 0 24 24" className={className} fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round">
    <rect x="4" y="11" width="16" height="9" rx="2" /><path d="M8 11V7a4 4 0 0 1 8 0v4" />
  </svg>
);

// ContainerPorts renders a container's port mappings as clear, self-describing
// chips: published host ports become blue "open in a new tab" links (external
// link glyph); ports that are only exposed on the internal Docker network get a
// muted lock chip that explains why they aren't clickable. Deduplicates the
// IPv4/IPv6 pair Docker prints for the same host port.
function ContainerPorts({ ports, state }) {
  if (!ports) return <span className="text-gray-400">—</span>;
  const publishedSeen = new Set();
  const internalSeen = new Set();
  const published = [];
  const internal = [];
  for (const raw of ports.split(',')) {
    const part = raw.trim();
    // Published: "0.0.0.0:8080->80/tcp" or "[::]:8080->80/tcp"
    const pub = part.match(/(?:\d+\.\d+\.\d+\.\d+|\[[^\]]+\]):(\d+)->\d+\/(tcp|udp)/);
    if (pub) {
      const [, hostPort, proto] = pub;
      if (publishedSeen.has(hostPort)) continue;
      publishedSeen.add(hostPort);
      published.push({ hostPort, proto });
      continue;
    }
    // Exposed only: "5432/tcp" (reachable on the Docker network, not the host)
    const exp = part.match(/^(\d+)\/(tcp|udp)$/);
    if (exp) {
      const key = `${exp[1]}/${exp[2]}`;
      if (internalSeen.has(key)) continue;
      internalSeen.add(key);
      internal.push({ port: exp[1], proto: exp[2] });
    }
  }
  if (published.length === 0 && internal.length === 0) {
    return <span className="font-mono text-xs text-gray-500">{ports}</span>;
  }
  const live = state === 'running';
  return (
    <div className="flex flex-wrap gap-1.5">
      {published.map(({ hostPort, proto }) => {
        if (proto === 'udp') {
          return (
            <span key={`u${hostPort}`} title={`UDP port ${hostPort} is published on the host but UDP services don't open in a browser.`}
              className="inline-flex items-center gap-1 rounded-md bg-gray-100 px-2 py-0.5 font-mono text-xs text-gray-600 ring-1 ring-gray-200">
              {hostPort}<span className="font-sans text-[9px] uppercase tracking-wide text-gray-400">udp</span>
            </span>
          );
        }
        const scheme = HTTPS_HOST_PORTS.has(hostPort) ? 'https' : 'http';
        const url = `${scheme}://${window.location.hostname}:${hostPort}`;
        return (
          <a key={hostPort} href={live ? url : undefined} target="_blank" rel="noreferrer"
            aria-disabled={!live}
            title={live ? `Open ${url} in a new tab` : `${url} — container isn't running`}
            className={`group inline-flex items-center gap-1 rounded-md px-2 py-0.5 font-mono text-xs ring-1 transition-colors ${live
              ? 'bg-blue-50 text-blue-700 ring-blue-200 hover:bg-blue-100 hover:ring-blue-300'
              : 'pointer-events-none bg-gray-100 text-gray-400 ring-gray-200'}`}>
            {hostPort}
            <ExternalLinkIcon className="h-3 w-3 opacity-50 transition-opacity group-hover:opacity-100" />
          </a>
        );
      })}
      {internal.map(({ port, proto }) => (
        <span key={`i${port}${proto}`}
          title={`Container port ${port}/${proto} is exposed on the internal Docker network only — it isn't published to the host, so there's no browser link. Add a ports: mapping in compose.yml to expose it.`}
          className="inline-flex items-center gap-1 rounded-md bg-gray-50 px-2 py-0.5 font-mono text-xs text-gray-400 ring-1 ring-gray-200">
          <LockIcon className="h-3 w-3 opacity-70" />
          {port}<span className="font-sans text-[9px] uppercase tracking-wide text-gray-300">{proto}</span>
        </span>
      ))}
    </div>
  );
}

// QueuedCommands shows the command queue for a callback-agent project: actions
// the controller has queued, and their status/output once the agent runs them.
function QueuedCommands({ commands, agentName, onRefresh }) {
  const tone = (c) => {
    if (c.status === 'done') return c.success ? 'bg-green-100 text-green-800' : 'bg-red-100 text-red-800';
    if (c.status === 'error') return 'bg-red-100 text-red-800';
    if (c.status === 'dispatched') return 'bg-amber-100 text-amber-800';
    return 'bg-gray-100 text-gray-600';
  };
  const label = (c) => (c.status === 'done' ? (c.success ? 'done' : 'failed') : c.status);
  return (
    <div className="section-panel mt-6">
      <div className="mb-2 flex items-center justify-between">
        <h2 className="text-lg font-semibold text-gray-950">Queued commands</h2>
        <button onClick={onRefresh} className="btn-secondary text-sm">Refresh</button>
      </div>
      <p className="mb-3 text-sm text-gray-500">Actions run on {agentName}&rsquo;s next check-in (usually within a minute); results appear here.</p>
      {(!commands || commands.length === 0) && (
        <div className="py-4 text-sm text-gray-500">No commands queued yet — use the action buttons above to queue one.</div>
      )}
      {commands && commands.length > 0 && (
        <div className="space-y-2">
          {commands.map((c) => (
            <div key={c.id} className="rounded-md border border-gray-200 p-3">
              <div className="flex items-center gap-2">
                <span className="font-mono text-sm font-semibold text-gray-900">{c.action}</span>
                <span className={`rounded px-2 py-0.5 text-xs font-medium ${tone(c)}`}>{label(c)}</span>
                <span className="ml-auto text-xs text-gray-400">{c.updated_at ? new Date(c.updated_at).toLocaleString() : ''}</span>
              </div>
              {c.output && (c.status === 'done' || c.status === 'error') && (
                <pre className="mt-2 max-h-48 overflow-auto whitespace-pre-wrap rounded bg-gray-950 p-2 text-xs text-gray-100">{c.output}</pre>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function ProjectDocs({ docs, projectName, sourceAgentId }) {
  const api = projectsForSource(sourceAgentId);
  const availableDocs = Array.isArray(docs) ? docs : [];
  const [selectedPath, setSelectedPath] = useState('');
  const [docContent, setDocContent] = useState(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    if (availableDocs.length === 0) {
      setSelectedPath('');
      setDocContent(null);
      return;
    }
    if (!selectedPath || !availableDocs.some(doc => doc.path === selectedPath)) {
      setSelectedPath(availableDocs[0].path);
    }
  }, [availableDocs, selectedPath]);

  useEffect(() => {
    if (!selectedPath) return undefined;
    let cancelled = false;
    setLoading(true);
    setError('');
    api.docContent(projectName, selectedPath)
      .then(res => {
        if (!cancelled) setDocContent(res.data);
      })
      .catch(err => {
        if (!cancelled) {
          setError(err.message);
          setDocContent(null);
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => { cancelled = true; };
  }, [projectName, selectedPath]);

  if (availableDocs.length === 0) {
    return (
      <div className="rounded-md border border-gray-200 p-4 text-sm text-gray-600">
        No project documentation files were found. Add a README file or Markdown/text files under <code className="rounded bg-gray-100 px-1 py-0.5">docs/</code>, <code className="rounded bg-gray-100 px-1 py-0.5">doc/</code>, or <code className="rounded bg-gray-100 px-1 py-0.5">documentation/</code> inside this project directory.
      </div>
    );
  }

  const selected = availableDocs.find(doc => doc.path === selectedPath);

  return (
    <div className="grid gap-4 lg:grid-cols-[280px_1fr]">
      <div className="space-y-2">
        {availableDocs.map(doc => (
          <button
            key={doc.path}
            type="button"
            onClick={() => setSelectedPath(doc.path)}
            className={`block w-full rounded-md border p-3 text-left text-sm ${selectedPath === doc.path ? 'border-blue-300 bg-blue-50' : 'border-gray-200 bg-white hover:bg-gray-50'}`}
            title={`View ${doc.path}`}
          >
            <div className="font-medium text-gray-950">{doc.title}</div>
            <div className="mt-1 break-all font-mono text-xs text-gray-500">{doc.path}</div>
            <div className="mt-1 text-xs text-gray-500">{formatBytes(doc.size_bytes)} · {formatDate(doc.updated_at)}</div>
          </button>
        ))}
      </div>
      <div>
        {selected && (
          <div className="mb-2 flex flex-wrap items-center gap-2 text-sm">
            <Badge tone="blue">{selected.file_name}</Badge>
            <span className="break-all font-mono text-xs text-gray-500">{selected.path}</span>
          </div>
        )}
        {loading && <div className="py-8 text-center text-gray-500">Loading documentation...</div>}
        {error && <div className="rounded border border-red-200 bg-red-50 p-3 text-sm text-red-800">{error}</div>}
        {!loading && !error && (
          <pre className="max-h-[640px] overflow-auto rounded-md bg-gray-950 p-4 font-mono text-xs leading-6 text-gray-100 whitespace-pre-wrap">{docContent?.content || 'No content.'}</pre>
        )}
      </div>
    </div>
  );
}

function ConfigEditor({ projectName, sourceAgentId }) {
  const api = projectsForSource(sourceAgentId);
  const [files, setFiles] = useState([]);
  const [selectedFile, setSelectedFile] = useState('');
  const [content, setContent] = useState('');
  const [dirty, setDirty] = useState(false);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState('');

  useEffect(() => {
    const load = async () => {
      try {
        const res = await api.files(projectName);
        const all = (res.data || []).filter(f => f.editable);
        setFiles(all);
        if (all.length > 0 && !selectedFile) {
          const compose = all.find(f => f.name === 'compose.yml' || f.name === 'docker-compose.yml');
          setSelectedFile((compose || all[0]).name);
        }
      } catch {}
    };
    load();
  }, [projectName]);

  useEffect(() => {
    if (!selectedFile) return;
    const load = async () => {
      try {
        const res = await api.fileContent(projectName, selectedFile);
        setContent(res.data?.content || '');
        setDirty(false);
        setMessage('');
      } catch (err) {
        setContent('');
        setMessage('Failed to load: ' + err.message);
      }
    };
    load();
  }, [selectedFile, projectName]);

  const save = async () => {
    setSaving(true);
    setMessage('');
    try {
      const res = await api.saveFile(projectName, selectedFile, content);
      setDirty(false);
      setMessage(res.data?.hint || 'Saved.');
    } catch (err) {
      setMessage('Error: ' + err.message);
    }
    setSaving(false);
  };

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-end gap-2">
        <Field label="File" title="Select a project config file to edit.">
          <select className="input w-64" value={selectedFile} onChange={e => { setSelectedFile(e.target.value); }}>
            {files.map(f => <option key={f.name} value={f.name}>{f.name} ({(f.size / 1024).toFixed(1)} KB)</option>)}
          </select>
        </Field>
        <button className="btn-primary" disabled={!dirty || saving} onClick={save} title="Save the file. A .bak backup is created automatically.">{saving ? 'Saving...' : 'Save'}</button>
        <button className="btn-secondary" disabled={!dirty} onClick={() => { setSelectedFile(selectedFile); }} title="Reload the file and discard changes.">Discard</button>
      </div>
      {message && <div className="rounded-md border border-blue-200 bg-blue-50 px-3 py-2 text-sm text-blue-900">{message}</div>}
      {dirty && <div className="text-xs text-amber-700">Unsaved changes.</div>}
      <textarea
        className="w-full rounded-md border border-gray-700 bg-gray-950 p-3 font-mono text-xs leading-relaxed text-gray-100"
        rows={24}
        spellCheck={false}
        value={content}
        onChange={e => { setContent(e.target.value); setDirty(true); }}
      />
      <div className="rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800">
        After saving, run <code className="rounded bg-white/60 px-1">docker compose up -d</code> from the Shell tab or click Start/Restart to apply compose changes. <code className="rounded bg-white/60 px-1">.env</code> changes apply on the next container recreate.
      </div>
    </div>
  );
}

function InteractiveShell({ projectName }) {
  const [containers, setContainers] = useState([]);
  const [selectedContainer, setSelectedContainer] = useState('');
  const [connected, setConnected] = useState(false);
  const termRef = useRef(null);
  const termElRef = useRef(null);
  const wsRef = useRef(null);
  const fitRef = useRef(null);

  useEffect(() => {
    const load = async () => {
      try {
        const res = await projects.get(projectName);
        const running = (res.data?.containers || []).filter(c => c.state === 'running');
        setContainers(running);
        if (running.length > 0 && !selectedContainer) setSelectedContainer(running[0].name);
      } catch {}
    };
    load();
  }, [projectName]);

  const connect = useCallback(() => {
    if (!selectedContainer || connected) return;
    const term = new Terminal({
      cursorBlink: true,
      fontSize: 13,
      fontFamily: 'Menlo, Monaco, "Courier New", monospace',
      theme: { background: '#0a0a0f', foreground: '#e5e7eb' },
    });
    const fit = new FitAddon();
    term.loadAddon(fit);
    if (termElRef.current) {
      term.open(termElRef.current);
      fit.fit();
    }
    termRef.current = term;
    fitRef.current = fit;

    const token = localStorage.getItem('cm_token') || '';
    const proto = window.location.protocol === 'https:' ? 'wss' : 'ws';
    const url = `${proto}://${window.location.host}/api/v1/projects/${encodeURIComponent(projectName)}/shell/exec?container=${encodeURIComponent(selectedContainer)}&token=${encodeURIComponent(token)}`;
    const ws = new WebSocket(url);
    ws.binaryType = 'arraybuffer';
    wsRef.current = ws;

    const sendResize = () => {
      if (ws.readyState === WebSocket.OPEN && term.cols && term.rows) {
        ws.send(JSON.stringify({ type: 'resize', cols: term.cols, rows: term.rows }));
      }
    };

    ws.onopen = () => {
      setConnected(true);
      fit.fit();
      sendResize();
      term.focus();
    };
    ws.onmessage = (ev) => {
      if (ev.data instanceof ArrayBuffer) {
        term.write(new Uint8Array(ev.data));
      } else {
        term.write(ev.data);
      }
    };
    ws.onclose = () => {
      term.write('\r\n\x1b[33m[session ended]\x1b[0m\r\n');
      setConnected(false);
    };
    ws.onerror = () => {
      term.write('\r\n\x1b[31m[connection error]\x1b[0m\r\n');
      setConnected(false);
    };

    term.onData((data) => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(data);
      }
    });

    term.onResize(({ cols, rows }) => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: 'resize', cols, rows }));
      }
    });

    const onWindowResize = () => {
      if (fitRef.current) fitRef.current.fit();
    };
    window.addEventListener('resize', onWindowResize);
    return () => {
      window.removeEventListener('resize', onWindowResize);
    };
  }, [selectedContainer, connected, projectName]);

  const disconnect = () => {
    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
    }
    if (termRef.current) {
      termRef.current.dispose();
      termRef.current = null;
    }
    setConnected(false);
  };

  useEffect(() => {
    return () => { disconnect(); };
  }, []);

  return (
    <div className="section-panel space-y-3">
      <h3 className="text-base font-semibold text-gray-950">Interactive Shell</h3>
      <p className="text-sm text-gray-600">Open a live shell inside a running container. Choose a container and click Connect.</p>
      <div className="flex flex-wrap items-end gap-2">
        <Field label="Container" title="Running container to exec into.">
          <select className="input w-64" value={selectedContainer} onChange={e => setSelectedContainer(e.target.value)} disabled={connected}>
            {containers.length === 0 && <option value="">No running containers</option>}
            {containers.map(c => <option key={c.name} value={c.name}>{c.name}</option>)}
          </select>
        </Field>
        {!connected ? (
          <button className="btn-primary" onClick={connect} disabled={!selectedContainer} title="Open a WebSocket to docker exec -i inside the selected container.">Connect</button>
        ) : (
          <button className="btn-danger" onClick={disconnect} title="Close the shell session.">Disconnect</button>
        )}
      </div>
      <div ref={termElRef} className="rounded-md border border-gray-700" style={{ minHeight: 300 }} />
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
  // Sticky-tail the running session's output. When the session ends the
  // hook stops trying to scroll, so users can browse the finished log
  // freely.
  const sessionPreRef = useFollowingScroll(open?.output?.length || 0, open?.status === 'running');
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
            <pre ref={sessionPreRef} className="max-h-[560px] overflow-auto rounded-md bg-gray-950 p-4 font-mono text-xs text-gray-100 whitespace-pre-wrap">{open.output || 'No output yet.'}</pre>
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
    if (!window.confirm(`Restore ${projectName} from backup ${id}?\n\nThis stops the running containers and overwrites the current project directory with the backup contents. This cannot be undone.`)) return;
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
    if (!window.confirm(`Permanently delete backup ${id}?\n\nThe archive is removed from the Stack Manager server and cannot be recovered.`)) return;
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

function formatBytes(bytes) {
  const value = Number(bytes) || 0;
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  return `${(value / 1024 / 1024).toFixed(1)} MB`;
}

function formatDate(value) {
  if (!value) return 'unknown';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return 'unknown';
  return date.toLocaleString();
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
    docs: 'Project-local README files and docs directory content.',
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
  // Sticky-tail: stays pinned to the bottom while the stream advances,
  // pauses when the user scrolls up to read history, resumes when they
  // scroll back down. The autoScroll checkbox is now an explicit off
  // switch — leave it on for the default follow-the-tail behavior.
  const preRef = useFollowingScroll(lines.length, autoScroll);
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
        <label className="ml-auto flex items-center gap-2 pb-2 text-sm text-gray-700" title="Follow the tail as new lines arrive. If you scroll up mid-stream the view pauses on its own until you scroll back to the bottom; uncheck this to disable auto-scroll entirely.">
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
