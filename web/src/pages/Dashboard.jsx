import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { projects, jobs, skills as skillsApi, system, registries, agents as agentsApi, schedules as schedulesApi, metrics as metricsApi, backup as backupApi } from '../api/client';

// Client-side snapshot cache so the Dashboard paints the last-known state
// immediately on mount, then refreshes in the background. Keyed by the
// filter combo so include-inactive / running-only don't leak into each other.
const SNAPSHOT_VERSION = 1;
const snapshotKey = (filters) => `cm_dashboard_v${SNAPSHOT_VERSION}_${filters.includeInactive ? 1 : 0}_${filters.runningOnly ? 1 : 0}`;
const readSnapshot = (filters) => {
  try {
    const raw = localStorage.getItem(snapshotKey(filters));
    return raw ? JSON.parse(raw) : null;
  } catch { return null; }
};
const writeSnapshot = (filters, data) => {
  try { localStorage.setItem(snapshotKey(filters), JSON.stringify({ ...data, cached_at: Date.now() })); } catch {}
};

const ACTIONS = [
  { key: 'update', label: 'Update', title: 'Pull newer images, then recreate containers. Projects with update hooks run only their hook.' },
  { key: 'pull', label: 'Pull', title: 'Download newer images without recreating containers.' },
  { key: 'up', label: 'Start', title: 'Run docker compose up -d for selected projects.' },
  { key: 'restart', label: 'Restart', title: 'Restart currently created containers.' },
  { key: 'down', label: 'Stop', title: 'Run docker compose down and remove project containers.' },
];

const PRUNE_MODES = [
  { key: 'safe', label: 'Safe: images, networks, volumes', title: 'Prune unused images, networks, and volumes using the safest Stack Manager prune mode.' },
  { key: 'system', label: 'System prune', title: 'Run docker system prune for unused containers, networks, images, and build cache.' },
  { key: 'system-all', label: 'System prune --all --volumes', title: 'Run docker system prune --all --volumes. This is the broadest cleanup option.' },
  { key: 'images', label: 'Images --all', title: 'Remove unused Docker images, including unreferenced tagged images.' },
  { key: 'volumes', label: 'Volumes', title: 'Remove unused Docker volumes.' },
  { key: 'networks', label: 'Networks', title: 'Remove unused Docker networks.' },
  { key: 'builder', label: 'Builder cache --all', title: 'Remove Docker build cache.' },
];

export default function Dashboard() {
  // Initial state hydrates from localStorage snapshot when one exists so
  // the page paints instantly on a return visit.
  const initialFilters = { includeInactive: true, runningOnly: false, query: '' };
  const initialSnapshot = readSnapshot(initialFilters);
  const [projectList, setProjectList] = useState(initialSnapshot?.projectList || []);
  const [skillList, setSkillList] = useState(initialSnapshot?.skillList || []);
  const [agentList, setAgentList] = useState(initialSnapshot?.agentList || []);
  const [scheduleList, setScheduleList] = useState(initialSnapshot?.scheduleList || []);
  const [metricsSummary, setMetricsSummary] = useState(initialSnapshot?.metricsSummary || null);
  const [metricsHistory, setMetricsHistory] = useState(initialSnapshot?.metricsHistory || []);
  const [backupList, setBackupList] = useState(initialSnapshot?.backupList || []);
  const [backupDestinations, setBackupDestinations] = useState(initialSnapshot?.backupDestinations || []);
  const [backupSchedules, setBackupSchedules] = useState(initialSnapshot?.backupSchedules || []);
  const [mainTab, setMainTab] = useState('projects');
  const [loading, setLoading] = useState(!initialSnapshot);
  const [refreshing, setRefreshing] = useState(false);
  // Tracks in-flight actions so the specific button that was clicked can show
  // a spinner and disabled state instead of leaving the user guessing.
  // Keys: `${project}:${action}` for per-row; `bulk:${action}` for bulk bar.
  const [pendingActions, setPendingActions] = useState(new Set());
  const markPending = (key, on) => setPendingActions(prev => {
    const next = new Set(prev);
    if (on) next.add(key); else next.delete(key);
    return next;
  });
  const isPending = (key) => pendingActions.has(key);
  const [error, setError] = useState(null);
  const [actionResult, setActionResult] = useState(null);
  const [selected, setSelected] = useState([]);
  const [filters, setFilters] = useState(initialFilters);
  const [quickFilter, setQuickFilter] = useState('all');
  const [updatePageSize, setUpdatePageSize] = useState(10);
  const [updatePage, setUpdatePage] = useState(1);
  const [timeout, setTimeoutValue] = useState(300);
  const [pruneMode, setPruneMode] = useState('safe');
  const [showPruneMenu, setShowPruneMenu] = useState(false);
  const [showCreate, setShowCreate] = useState(false);
  const [createForm, setCreateForm] = useState({
    name: '',
    compose_content: 'services:\n  app:\n    image: nginx:stable\n    restart: unless-stopped\n',
    env_content: '',
    inactive: false,
    overwrite: false,
  });
  const [registryForm, setRegistryForm] = useState({ registry: '', username: '', password: '' });
  const [agentForm, setAgentForm] = useState({ name: '', base_url: '', token: '', enabled: true });
  const [scheduleForm, setScheduleForm] = useState({
    id: 0,
    agent_id: '',
    project: '',
    action: 'update',
    enabled: true,
    interval_minutes: 1440,
    timeout_seconds: 300,
  });
  const [selectedBackupProjects, setSelectedBackupProjects] = useState([]);
  const [selectedBackupDestinations, setSelectedBackupDestinations] = useState([]);
  const [backupScheduleForm, setBackupScheduleForm] = useState({
    id: 0,
    name: '',
    projects: [],
    all_projects: true,
    destination_ids: [],
    enabled: true,
    interval_minutes: 1440,
  });

  // Force the server to re-discover projects + refresh the Redis cache, then
  // re-fetch the dashboard. Use when container state changed on the host
  // (e.g. compose up/down/restart outside the UI) and the dashboard is
  // still showing the stale cached snapshot.
  const forceRefresh = async () => {
    setRefreshing(true);
    try {
      await metricsApi.refresh();
    } catch (err) {
      // metrics/refresh failure isn't fatal — still try to re-fetch below.
    }
    await fetchData({ background: true });
  };

  const fetchData = async ({ background = false } = {}) => {
    try {
      if (background) setRefreshing(true); else setLoading(true);
      const [projRes, skillRes, agentRes, scheduleRes, metricsRes, historyRes, backupRes, backupDestRes, backupScheduleRes] = await Promise.all([
        projects.list({ include_inactive: filters.includeInactive ? 'true' : 'false', running_only: filters.runningOnly ? 'true' : 'false' }),
        skillsApi.list(),
        agentsApi.list(),
        schedulesApi.list(),
        metricsApi.summary(),
        metricsApi.history(24),
        backupApi.list(),
        backupApi.destinations(),
        backupApi.schedules(),
      ]);
      const fresh = {
        projectList: projRes.data || [],
        skillList: skillRes.data || [],
        agentList: agentRes.data || [],
        scheduleList: scheduleRes.data || [],
        metricsSummary: metricsRes.data || null,
        metricsHistory: historyRes.data || [],
        backupList: backupRes.data || [],
        backupDestinations: backupDestRes.data || [],
        backupSchedules: backupScheduleRes.data || [],
      };
      setProjectList(fresh.projectList);
      setSkillList(fresh.skillList);
      setAgentList(fresh.agentList);
      setScheduleList(fresh.scheduleList);
      setMetricsSummary(fresh.metricsSummary);
      setMetricsHistory(fresh.metricsHistory);
      setBackupList(fresh.backupList);
      setBackupDestinations(fresh.backupDestinations);
      setBackupSchedules(fresh.backupSchedules);
      writeSnapshot(filters, fresh);
      setError(null);
    } catch (err) {
      // Preserve cached data on background failures so a blip doesn't blank the page.
      if (!background) setError(err.message);
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  };

  // On mount (or filter change) do a background refresh if we already have
  // cached data, otherwise show the loading state.
  useEffect(() => {
    const snapshot = readSnapshot(filters);
    if (snapshot) {
      setProjectList(snapshot.projectList || []);
      setSkillList(snapshot.skillList || []);
      setAgentList(snapshot.agentList || []);
      setScheduleList(snapshot.scheduleList || []);
      setMetricsSummary(snapshot.metricsSummary || null);
      setMetricsHistory(snapshot.metricsHistory || []);
      setBackupList(snapshot.backupList || []);
      setBackupDestinations(snapshot.backupDestinations || []);
      setBackupSchedules(snapshot.backupSchedules || []);
      fetchData({ background: true });
    } else {
      fetchData();
    }
  }, [filters.includeInactive, filters.runningOnly]);

  const filteredProjects = projectList.filter((p) => {
    const q = filters.query.trim().toLowerCase();
    if (q && !p.name.toLowerCase().includes(q) && !p.dir.toLowerCase().includes(q)) return false;
    if (quickFilter === 'running') return p.running;
    if (quickFilter === 'inactive') return p.inactive;
    if (quickFilter === 'no_updates') return p.update_policy?.effective_policy === 'no_updates';
    if (quickFilter === 'registry') return (p.image_sources || []).some(s => s.source_type === 'registry');
    if (quickFilter === 'custom') return (p.image_sources || []).some(s => s.source_type === 'custom');
    return true;
  });

  const running = projectList.filter(p => p.running).length;
  const inactive = projectList.filter(p => p.inactive).length;
  const noUpdates = projectList.filter(p => p.update_policy?.effective_policy === 'no_updates').length;
  const customServices = projectList.reduce((sum, p) => sum + (p.image_sources || []).filter(s => s.source_type === 'custom').length, 0);
  const registryServices = projectList.reduce((sum, p) => sum + (p.image_sources || []).filter(s => s.source_type === 'registry').length, 0);
  const updatePageCount = Math.max(1, Math.ceil(projectList.length / updatePageSize));
  const updatePageSafe = Math.min(updatePage, updatePageCount);
  const pagedUpdateProjects = projectList.slice((updatePageSafe - 1) * updatePageSize, updatePageSafe * updatePageSize);

  const setSummaryFilter = (filter) => {
    setQuickFilter(filter);
    if (filter === 'running') {
      setFilters({ ...filters, includeInactive: true, runningOnly: false });
    } else if (filter === 'inactive') {
      setFilters({ ...filters, includeInactive: true, runningOnly: false });
    }
    setSelected([]);
  };

  const runAction = async (name, action) => {
    const key = `${name}:${action}`;
    markPending(key, true);
    try {
      const res = await projects.startJob(name, action, timeout);
      setActionResult({ label: `${action} ${name}`, status: 'running', job: res.data });
      pollJob(res.data.id, `${action} ${name}`, key);
    } catch (err) {
      setActionResult({ label: `${action} ${name}`, status: 'error', error: err.message });
      markPending(key, false);
    }
  };

  const pollJob = (jobId, label, pendingKey) => {
    const tick = async () => {
      try {
        const res = await jobs.get(jobId);
        const job = res.data;
        setActionResult({ label, status: job.status === 'running' ? 'running' : job.success ? 'done' : 'error', job });
        if (job.status === 'running') {
          window.setTimeout(tick, 1500);
        } else {
          if (pendingKey) markPending(pendingKey, false);
          fetchData();
        }
      } catch (err) {
        setActionResult({ label, status: 'error', error: err.message });
        if (pendingKey) markPending(pendingKey, false);
      }
    };
    window.setTimeout(tick, 750);
  };

  const runBulk = async (action) => {
    const key = `bulk:${action}`;
    const targets = selected.length > 0 ? selected : filteredProjects.map(p => p.name);
    markPending(key, true);
    try {
      setActionResult({ label: `${action} ${targets.length} project${targets.length === 1 ? '' : 's'}`, status: 'running' });
      const res = await projects.bulk(action, { projects: targets, timeout });
      setActionResult({ label: `bulk ${action}`, status: 'done', result: res.data });
      setSelected([]);
      fetchData();
    } catch (err) {
      setActionResult({ label: `bulk ${action}`, status: 'error', error: err.message });
    } finally {
      markPending(key, false);
    }
  };

  const deleteProject = async (project) => {
    const confirmName = window.prompt(`Type ${project.name} to delete the whole project directory.`);
    if (confirmName !== project.name) {
      setActionResult({ label: `delete ${project.name}`, status: 'error', error: 'Project name confirmation did not match.' });
      return;
    }
    try {
      setActionResult({ label: `delete ${project.name}`, status: 'running' });
      const res = await projects.delete(project.name, { confirm_name: confirmName, stop_first: true });
      setActionResult({ label: `delete ${project.name}`, status: 'done', result: res.data });
      setSelected(selected.filter(name => name !== project.name));
      fetchData();
    } catch (err) {
      setActionResult({ label: `delete ${project.name}`, status: 'error', error: err.message, result: err.data });
      // Refresh the list too - if the delete partially succeeded (compose down
      // fine but RemoveAll failed halfway, or the underlying directory is
      // already gone as when compose-manager was cleaned up out-of-band) the
      // cached list would still show the stale row.
      fetchData();
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

  const saveSchedule = async (event) => {
    event.preventDefault();
    const body = {
      id: Number(scheduleForm.id) || undefined,
      agent_id: scheduleForm.agent_id ? Number(scheduleForm.agent_id) : null,
      project: scheduleForm.project,
      action: scheduleForm.action,
      enabled: scheduleForm.enabled,
      interval_minutes: Number(scheduleForm.interval_minutes),
      timeout_seconds: Number(scheduleForm.timeout_seconds),
    };
    try {
      setActionResult({ label: 'save schedule', status: 'running' });
      const res = await schedulesApi.save(body);
      setActionResult({ label: 'save schedule', status: 'done', result: res.data });
      setScheduleForm({ ...scheduleForm, id: 0, project: '' });
      fetchData();
    } catch (err) {
      setActionResult({ label: 'save schedule', status: 'error', error: err.message });
    }
  };

  const editSchedule = (schedule) => {
    setScheduleForm({
      id: schedule.id,
      agent_id: schedule.agent_id || '',
      project: schedule.project,
      action: schedule.action || 'update',
      enabled: Boolean(schedule.enabled),
      interval_minutes: schedule.interval_minutes || 1440,
      timeout_seconds: schedule.timeout_seconds || 300,
    });
  };

  const runSchedule = async (schedule) => {
    try {
      setActionResult({ label: `run schedule ${schedule.project}`, status: 'running' });
      const res = await schedulesApi.run(schedule.id);
      setActionResult({ label: `run schedule ${schedule.project}`, status: 'done', result: res.data });
      fetchData();
    } catch (err) {
      setActionResult({ label: `run schedule ${schedule.project}`, status: 'error', error: err.message });
    }
  };

  const deleteSchedule = async (schedule) => {
    try {
      setActionResult({ label: `delete schedule ${schedule.project}`, status: 'running' });
      await schedulesApi.delete(schedule.id);
      setActionResult({ label: `delete schedule ${schedule.project}`, status: 'done' });
      fetchData();
    } catch (err) {
      setActionResult({ label: `delete schedule ${schedule.project}`, status: 'error', error: err.message });
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

  const saveAgent = async (event) => {
    event.preventDefault();
    try {
      setActionResult({ label: `save agent ${agentForm.name}`, status: 'running' });
      const res = await agentsApi.save(agentForm);
      setActionResult({ label: `save agent ${agentForm.name}`, status: 'done', result: res.data });
      setAgentForm({ name: '', base_url: '', token: '', enabled: true });
      fetchData();
    } catch (err) {
      setActionResult({ label: `save agent ${agentForm.name}`, status: 'error', error: err.message });
    }
  };

  const deleteAgent = async (agent) => {
    if (!window.confirm(`Delete agent ${agent.name}? Schedules for this agent will also be removed.`)) return;
    try {
      setActionResult({ label: `delete agent ${agent.name}`, status: 'running' });
      await agentsApi.delete(agent.id);
      setActionResult({ label: `delete agent ${agent.name}`, status: 'done' });
      fetchData();
    } catch (err) {
      setActionResult({ label: `delete agent ${agent.name}`, status: 'error', error: err.message });
    }
  };

  const runPrune = async () => {
    if (pruneMode === 'system-all' && !window.confirm('Run docker system prune --all --volumes? This removes unused images, build cache, networks, and volumes.')) return;
    setShowPruneMenu(false);
    setActionResult({ label: 'system prune', status: 'running' });
    try {
      const res = await system.prune(pruneMode);
      setActionResult({ label: 'system prune', status: 'done', result: res.data });
    } catch (err) {
      setActionResult({ label: 'system prune', status: 'error', error: err.message });
    }
  };

  const toggleBackupProject = (name) => {
    setSelectedBackupProjects(current => current.includes(name) ? current.filter(item => item !== name) : [...current, name]);
  };

  const toggleBackupDestination = (id) => {
    setSelectedBackupDestinations(current => current.includes(id) ? current.filter(item => item !== id) : [...current, id]);
  };

  const runBackupProject = async (name) => {
    try {
      setActionResult({ label: `backup ${name}`, status: 'running' });
      const body = selectedBackupDestinations.length > 0 ? { destination_ids: selectedBackupDestinations } : {};
      const res = await backupApi.create(name, body);
      setActionResult({ label: `backup ${name}`, status: 'done', result: res.data });
      fetchData();
    } catch (err) {
      setActionResult({ label: `backup ${name}`, status: 'error', error: err.message });
    }
  };

  const runBackupBatch = async (mode) => {
    const targets = mode === 'all' ? projectList.filter(p => !p.inactive).map(p => p.name) : selectedBackupProjects;
    if (targets.length === 0) {
      setActionResult({ label: 'backup batch', status: 'error', error: 'No projects selected.' });
      return;
    }
    const results = [];
    setActionResult({ label: `backup ${targets.length} project${targets.length === 1 ? '' : 's'}`, status: 'running' });
    for (const name of targets) {
      try {
        const body = selectedBackupDestinations.length > 0 ? { destination_ids: selectedBackupDestinations } : {};
        const res = await backupApi.create(name, body);
        results.push({ project: name, success: true, backup: res.data?.id });
        setActionResult({ label: `backup ${targets.length} projects`, status: 'running', result: { output: results.map(r => `${r.project}: ${r.success ? r.backup : r.error}`).join('\n') } });
      } catch (err) {
        results.push({ project: name, success: false, error: err.message });
      }
    }
    const failed = results.filter(r => !r.success);
    setActionResult({
      label: `backup ${targets.length} projects`,
      status: failed.length ? 'error' : 'done',
      result: { output: results.map(r => `${r.project}: ${r.success ? r.backup : r.error}`).join('\n') },
      error: failed.length ? `${failed.length} project backup${failed.length === 1 ? '' : 's'} failed.` : undefined,
    });
    setSelectedBackupProjects([]);
    fetchData();
  };

  const toggleBackupScheduleProject = (name) => {
    setBackupScheduleForm(current => ({
      ...current,
      projects: current.projects.includes(name) ? current.projects.filter(item => item !== name) : [...current.projects, name],
      all_projects: false,
    }));
  };

  const toggleBackupScheduleDestination = (id) => {
    setBackupScheduleForm(current => ({
      ...current,
      destination_ids: current.destination_ids.includes(id) ? current.destination_ids.filter(item => item !== id) : [...current.destination_ids, id],
    }));
  };

  const saveBackupSchedule = async (event) => {
    event.preventDefault();
    const body = {
      id: Number(backupScheduleForm.id) || undefined,
      name: backupScheduleForm.name,
      enabled: backupScheduleForm.enabled,
      projects: backupScheduleForm.all_projects ? [] : backupScheduleForm.projects,
      destination_ids: backupScheduleForm.destination_ids,
      interval_minutes: Number(backupScheduleForm.interval_minutes),
    };
    try {
      setActionResult({ label: 'save backup schedule', status: 'running' });
      const res = await backupApi.saveSchedule(body);
      setActionResult({ label: 'save backup schedule', status: 'done', result: res.data });
      setBackupScheduleForm({ id: 0, name: '', projects: [], all_projects: true, destination_ids: [], enabled: true, interval_minutes: 1440 });
      fetchData();
    } catch (err) {
      setActionResult({ label: 'save backup schedule', status: 'error', error: err.message });
    }
  };

  const editBackupSchedule = (schedule) => {
    setBackupScheduleForm({
      id: schedule.id,
      name: schedule.name,
      projects: schedule.projects || [],
      all_projects: !schedule.projects || schedule.projects.length === 0,
      destination_ids: schedule.destination_ids || [],
      enabled: Boolean(schedule.enabled),
      interval_minutes: schedule.interval_minutes || 1440,
    });
  };

  const runBackupSchedule = async (schedule) => {
    try {
      setActionResult({ label: `run backup schedule ${schedule.name}`, status: 'running' });
      const res = await backupApi.runSchedule(schedule.id);
      setActionResult({ label: `run backup schedule ${schedule.name}`, status: 'done', result: res.data });
      fetchData();
    } catch (err) {
      setActionResult({ label: `run backup schedule ${schedule.name}`, status: 'error', error: err.message });
    }
  };

  const deleteBackupSchedule = async (schedule) => {
    if (!window.confirm(`Delete backup schedule ${schedule.name}?`)) return;
    try {
      setActionResult({ label: `delete backup schedule ${schedule.name}`, status: 'running' });
      await backupApi.deleteSchedule(schedule.id);
      setActionResult({ label: `delete backup schedule ${schedule.name}`, status: 'done' });
      fetchData();
    } catch (err) {
      setActionResult({ label: `delete backup schedule ${schedule.name}`, status: 'error', error: err.message });
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
          <h1 className="text-2xl font-semibold text-gray-950">Stack Manager</h1>
          <p className="text-sm text-gray-600">Manage compose projects from the configured Docker root.</p>
        </div>
        <div className="flex flex-wrap gap-2">
          <button disabled={refreshing} title="Force a live re-discovery of every project (compose ps, image sources, running state) and clear the server-side cache. Use when container state changed on the host outside the UI (e.g. compose up/down/restart from a shell)." onClick={forceRefresh} className="btn-secondary inline-flex items-center gap-2">
            {refreshing && <span className="spinner" aria-hidden="true"></span>}
            Refresh
          </button>
          <button title="Create a new project directory with a compose.yml file." onClick={() => setShowCreate(!showCreate)} className="btn-primary">Create Project</button>
          <div className="relative inline-flex">
            <button title={`Run ${PRUNE_MODES.find(mode => mode.key === pruneMode)?.label || 'selected prune command'} on this host.`} onClick={runPrune} className="rounded-l-md bg-red-700 px-3 py-2 text-sm font-medium text-white hover:bg-red-600">
              Prune
            </button>
            <button type="button" title="Choose a Docker prune command." onClick={() => setShowPruneMenu(!showPruneMenu)} className="rounded-r-md border-l border-red-500 bg-red-700 px-2 py-2 text-sm font-medium text-white hover:bg-red-600" aria-haspopup="menu" aria-expanded={showPruneMenu}>
              ▾
            </button>
            {showPruneMenu && (
              <div className="absolute right-0 top-full z-20 mt-1 w-64 overflow-hidden rounded-md border border-gray-200 bg-white py-1 text-sm shadow-lg" role="menu">
                {PRUNE_MODES.map(mode => (
                  <button
                    key={mode.key}
                    type="button"
                    title={mode.title}
                    onClick={() => {
                      setPruneMode(mode.key);
                      setShowPruneMenu(false);
                    }}
                    className={`flex w-full items-center gap-2 px-3 py-2 text-left hover:bg-gray-100 ${pruneMode === mode.key ? 'bg-gray-100 text-blue-700' : 'text-gray-800'}`}
                    role="menuitem"
                  >
                    <span className="w-4">{pruneMode === mode.key ? '✓' : ''}</span>
                    <span>{mode.label}</span>
                  </button>
                ))}
              </div>
            )}
          </div>
        </div>
      </div>

      <div className="section-panel">
        <div className="flex flex-wrap gap-2">
          <button type="button" onClick={() => setMainTab('projects')} className={mainTab === 'projects' ? 'btn-primary' : 'btn-secondary'} title="Show project discovery, filters, and compose actions.">Projects</button>
          <button type="button" onClick={() => setMainTab('updates')} className={mainTab === 'updates' ? 'btn-primary' : 'btn-secondary'} title="Show every project and its update policy.">Updates</button>
          <button type="button" onClick={() => setMainTab('backups')} className={mainTab === 'backups' ? 'btn-primary' : 'btn-secondary'} title="Run and schedule backups across projects.">Backups</button>
          <button type="button" onClick={() => setMainTab('system')} className={mainTab === 'system' ? 'btn-primary' : 'btn-secondary'} title="Show system health, enabled modules, and historical host statistics.">System Status</button>
        </div>
      </div>

      {actionResult && <ActionResult result={actionResult} onDismiss={() => setActionResult(null)} />}

      {mainTab === 'projects' && (
        <>
      <div className="grid grid-cols-2 gap-3 lg:grid-cols-6">
        <StatCard label="Projects" value={projectList.length} active={quickFilter === 'all'} title="Show all discovered projects." onClick={() => setSummaryFilter('all')} />
        <StatCard label="Running" value={running} active={quickFilter === 'running'} title="Show projects with running containers." onClick={() => setSummaryFilter('running')} />
        <StatCard label="Inactive" value={inactive} active={quickFilter === 'inactive'} title="Show projects marked inactive." onClick={() => setSummaryFilter('inactive')} />
        <StatCard label="No updates" value={noUpdates} active={quickFilter === 'no_updates'} title="Show projects whose update policy skips updates." onClick={() => setSummaryFilter('no_updates')} />
        <StatCard label="Registry services" value={registryServices} active={quickFilter === 'registry'} title="Show projects with registry-backed services." onClick={() => setSummaryFilter('registry')} />
        <StatCard label="Custom builds" value={customServices} active={quickFilter === 'custom'} title="Show projects with custom build services." onClick={() => setSummaryFilter('custom')} />
      </div>

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
              {quickFilter !== 'all' && (
                <button title="Clear the summary-card filter." onClick={() => setSummaryFilter('all')} className="mini-button" type="button">
                  Clear {quickFilter.replace('_', ' ')}
                </button>
              )}
            </div>
          </div>
          <div className="flex flex-wrap gap-2">
            {ACTIONS.map(action => (
              <button key={action.key} disabled={isPending(`bulk:${action.key}`)} title={`${action.title} Applies to selected rows, or the current filtered list if none are selected.`} onClick={() => runBulk(action.key)} className={`${action.key === 'down' ? 'btn-danger' : 'btn-secondary'} inline-flex items-center gap-2`}>
                {isPending(`bulk:${action.key}`) && <span className="spinner" aria-hidden="true"></span>}
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
                      {p.update_policy?.effective_policy === 'no_updates' && <Badge tone="amber">no updates</Badge>}
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
                        <button key={action.key} disabled={isPending(`${p.name}:${action.key}`)} title={action.key === 'update' && p.update_policy?.effective_policy === 'no_updates' ? 'Updates are disabled for this project; this records a skipped session.' : action.title} onClick={() => runAction(p.name, action.key)} className={`${action.key === 'down' ? 'mini-danger' : 'mini-button'} inline-flex items-center gap-1`}>
                          {isPending(`${p.name}:${action.key}`) && <span className="spinner" aria-hidden="true"></span>}
                          {action.label}
                        </button>
                      ))}
                      <button disabled={!p.inactive} title={p.inactive ? 'Delete the whole project directory after exact-name confirmation.' : 'Mark the project inactive before deleting its directory.'} onClick={() => deleteProject(p)} className={p.inactive ? 'mini-danger' : 'mini-button opacity-50'}>
                        Delete
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          {filteredProjects.length === 0 && <div className="py-8 text-center text-gray-500">No projects match the current filters.</div>}
        </div>
      </div>

        </>
      )}

      {mainTab === 'updates' && (
        <UpdatesPanel
          projects={projectList}
          page={updatePageSafe}
          pageCount={updatePageCount}
          pageSize={updatePageSize}
          pagedProjects={pagedUpdateProjects}
          setPage={setUpdatePage}
          setPageSize={(value) => {
            setUpdatePageSize(value);
            setUpdatePage(1);
          }}
          runAction={runAction}
        />
      )}

      {mainTab === 'backups' && (
        <BackupsPanel
          projects={projectList}
          backups={backupList}
          destinations={backupDestinations}
          schedules={backupSchedules}
          selectedProjects={selectedBackupProjects}
          selectedDestinations={selectedBackupDestinations}
          scheduleForm={backupScheduleForm}
          setScheduleForm={setBackupScheduleForm}
          toggleProject={toggleBackupProject}
          toggleDestination={toggleBackupDestination}
          runProject={runBackupProject}
          runBatch={runBackupBatch}
          toggleScheduleProject={toggleBackupScheduleProject}
          toggleScheduleDestination={toggleBackupScheduleDestination}
          saveSchedule={saveBackupSchedule}
          editSchedule={editBackupSchedule}
          runSchedule={runBackupSchedule}
          deleteSchedule={deleteBackupSchedule}
        />
      )}

      {mainTab === 'system' && (
        <SystemStatus
          skills={skillList}
          summary={metricsSummary}
          history={metricsHistory}
          onRefresh={async () => {
            setActionResult({ label: 'metrics refresh', status: 'running' });
            try {
              const res = await metricsApi.refresh();
              const [skillsRes, hist] = await Promise.all([skillsApi.list(), metricsApi.history(24)]);
              setSkillList(skillsRes.data || []);
              setMetricsSummary(res.data || null);
              setMetricsHistory(hist.data || []);
              setActionResult({ label: 'metrics refresh', status: 'done' });
            } catch (err) {
              setActionResult({ label: 'metrics refresh', status: 'error', error: err.message });
            }
          }}
        />
      )}
    </div>
  );
}

function SystemStatus({ skills, summary, history, onRefresh }) {
  return (
    <div className="space-y-4">
      <div className="section-panel">
        <div className="mb-3 flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
          <div>
            <h2 className="text-lg font-semibold text-gray-950">System Status</h2>
            <p className="text-sm text-gray-600">Host modules, cache health, and historical system statistics.</p>
          </div>
          <Badge tone="blue">{skills.length} modules</Badge>
        </div>
        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
          {skills.map(s => (
            <div key={s.name} className="rounded-md border border-gray-200 p-3">
              <div className="flex items-start justify-between gap-3">
                <div>
                  <div className="font-medium capitalize text-gray-900">{s.name === 'frontend' ? 'web dashboard' : s.name}</div>
                  <div className="mt-1 text-xs text-gray-500">{s.description}</div>
                  {s.version && <div className="mt-2 text-xs text-gray-400">v{s.version}</div>}
                </div>
                <Badge tone={s.healthy ? 'green' : 'red'}>{s.healthy ? 'healthy' : 'down'}</Badge>
              </div>
            </div>
          ))}
          {skills.length === 0 && <div className="py-6 text-sm text-gray-500">No system modules reported yet.</div>}
        </div>
      </div>
      <MetricsPanel summary={summary} history={history} onRefresh={onRefresh} />
    </div>
  );
}

function UpdatesPanel({ projects, pagedProjects, page, pageCount, pageSize, setPage, setPageSize, runAction }) {
  return (
    <div className="section-panel">
      <div className="mb-4 flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
        <div>
          <h2 className="text-lg font-semibold text-gray-950">Update Policies</h2>
          <p className="text-sm text-gray-600">Review every discovered project before running updates.</p>
        </div>
        <div className="flex flex-wrap items-center gap-2 text-sm">
          <span className="text-gray-500">{projects.length} projects</span>
          <select className="input max-w-[120px]" value={pageSize} onChange={e => setPageSize(Number(e.target.value))} title="Rows per page.">
            {[10, 25, 50, 100].map(size => <option key={size} value={size}>{size} rows</option>)}
          </select>
          <button type="button" className="mini-button" disabled={page <= 1} onClick={() => setPage(page - 1)} title="Previous page.">Prev</button>
          <span className="text-xs text-gray-500">Page {page} of {pageCount}</span>
          <button type="button" className="mini-button" disabled={page >= pageCount} onClick={() => setPage(page + 1)} title="Next page.">Next</button>
        </div>
      </div>
      <div className="overflow-x-auto">
        <table className="w-full min-w-[920px] text-left text-sm">
          <thead>
            <tr className="border-b border-gray-200 text-xs uppercase text-gray-500">
              <th className="py-2">Project</th>
              <th>Policy</th>
              <th>Reason</th>
              <th>Sources</th>
              <th className="text-right">Actions</th>
            </tr>
          </thead>
          <tbody>
            {pagedProjects.map(project => {
              const policy = project.update_policy?.effective_policy || project.update_policy?.policy || 'auto';
              const reason = project.update_policy?.reason || project.update_policy?.effective_reason || '';
              return (
                <tr key={project.name} className="border-b border-gray-100 align-top">
                  <td className="py-3">
                    <Link to={`/projects/${project.name}`} className="font-medium text-blue-700 hover:underline">{project.name}</Link>
                    <div className="mt-1 flex flex-wrap gap-1">
                      {project.inactive && <Badge tone="amber">inactive</Badge>}
                      <Badge tone={project.running ? 'green' : 'gray'}>{project.running ? 'running' : 'stopped'}</Badge>
                    </div>
                  </td>
                  <td className="py-3"><Badge tone={policy === 'no_updates' ? 'amber' : 'blue'}>{policy.replace('_', ' ')}</Badge></td>
                  <td className="max-w-[360px] py-3 text-xs text-gray-600">{reason || 'Default update behavior.'}</td>
                  <td className="py-3"><SourceSummary sources={project.image_sources || []} /></td>
                  <td className="py-3">
                    <div className="flex justify-end gap-1">
                      <button type="button" onClick={() => runAction(project.name, 'pull')} className="mini-button" title="Pull images for this project.">Pull</button>
                      <button type="button" onClick={() => runAction(project.name, 'update')} className="mini-button" title="Run the update workflow for this project.">Update</button>
                    </div>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
        {projects.length === 0 && <div className="py-8 text-center text-sm text-gray-500">No projects discovered.</div>}
      </div>
    </div>
  );
}

function BackupsPanel({
  projects,
  backups,
  destinations,
  schedules,
  selectedProjects,
  selectedDestinations,
  scheduleForm,
  setScheduleForm,
  toggleProject,
  toggleDestination,
  runProject,
  runBatch,
  toggleScheduleProject,
  toggleScheduleDestination,
  saveSchedule,
  editSchedule,
  runSchedule,
  deleteSchedule,
}) {
  const activeProjects = projects.filter(project => !project.inactive);
  const enabledDestinations = destinations.filter(destination => destination.enabled);
  const totalBytes = backups.reduce((sum, backup) => sum + Number(backup.size_bytes || 0), 0);
  return (
    <div className="space-y-4">
      <div className="grid gap-3 md:grid-cols-4">
        <MetricCard label="Local backups" value={backups.length} sub={formatBytes(totalBytes)} />
        <MetricCard label="Backup endpoints" value={enabledDestinations.length} sub={`${destinations.length} configured`} />
        <MetricCard label="Backup schedules" value={schedules.length} sub={`${schedules.filter(s => s.enabled).length} enabled`} />
        <MetricCard label="Active projects" value={activeProjects.length} />
      </div>

      <div className="section-panel">
        <div className="mb-4 flex flex-col gap-2 xl:flex-row xl:items-start xl:justify-between">
          <div>
            <h2 className="text-lg font-semibold text-gray-950">Run Backups</h2>
            <p className="text-sm text-gray-600">Create local archives and optionally copy them to configured endpoints.</p>
          </div>
          <div className="flex flex-wrap gap-2">
            <button type="button" className="btn-secondary" onClick={() => runBatch('selected')} title="Back up checked projects.">Backup Selected ({selectedProjects.length})</button>
            <button type="button" className="btn-primary" onClick={() => runBatch('all')} title="Back up every active project.">Backup All Active</button>
          </div>
        </div>

        <div className="mb-4 rounded-md border border-gray-200 p-3">
          <div className="mb-2 text-sm font-medium text-gray-800">Copy to endpoints</div>
          <div className="flex flex-wrap gap-3">
            {enabledDestinations.map(destination => (
              <label key={destination.id} className="flex items-center gap-2 text-sm text-gray-700" title={`Copy new backups to ${destination.name}.`}>
                <input type="checkbox" checked={selectedDestinations.includes(destination.id)} onChange={() => toggleDestination(destination.id)} />
                {destination.name} <span className="text-xs text-gray-500">({destination.type})</span>
              </label>
            ))}
            {enabledDestinations.length === 0 && <span className="text-sm text-gray-500">No enabled endpoints. Configure them in Settings &gt; Backup Endpoints.</span>}
          </div>
        </div>

        <div className="overflow-x-auto">
          <table className="w-full min-w-[860px] text-left text-sm">
            <thead>
              <tr className="border-b border-gray-200 text-xs uppercase text-gray-500">
                <th className="w-10 py-2">Pick</th>
                <th>Project</th>
                <th>State</th>
                <th>Last local backup</th>
                <th className="text-right">Action</th>
              </tr>
            </thead>
            <tbody>
              {projects.map(project => {
                const lastBackup = backups.find(backup => backup.project === project.name);
                return (
                  <tr key={project.name} className="border-b border-gray-100">
                    <td className="py-3"><input type="checkbox" checked={selectedProjects.includes(project.name)} onChange={() => toggleProject(project.name)} title={`Select ${project.name} for backup.`} /></td>
                    <td className="py-3"><Link to={`/projects/${project.name}`} className="font-medium text-blue-700 hover:underline">{project.name}</Link></td>
                    <td><Badge tone={project.inactive ? 'amber' : project.running ? 'green' : 'gray'}>{project.inactive ? 'inactive' : project.running ? 'running' : 'stopped'}</Badge></td>
                    <td className="text-xs text-gray-500">{lastBackup ? `${formatDate(lastBackup.created_at)} · ${formatBytes(lastBackup.size_bytes)}` : 'none'}</td>
                    <td className="text-right"><button type="button" className="mini-button" onClick={() => runProject(project.name)} title={`Create a backup for ${project.name}.`}>Backup</button></td>
                  </tr>
                );
              })}
            </tbody>
          </table>
          {projects.length === 0 && <div className="py-8 text-center text-sm text-gray-500">No projects discovered.</div>}
        </div>
      </div>

      <div className="grid gap-4 xl:grid-cols-[420px_1fr]">
        <form onSubmit={saveSchedule} className="section-panel space-y-3">
          <h2 className="text-lg font-semibold text-gray-950">{scheduleForm.id ? 'Edit Backup Schedule' : 'New Backup Schedule'}</h2>
          <Field label="Name" title="Friendly schedule name.">
            <input className="input" value={scheduleForm.name} onChange={e => setScheduleForm({ ...scheduleForm, name: e.target.value })} placeholder="nightly backups" required />
          </Field>
          <Field label="Interval minutes" title="Minimum 5 minutes. Use 1440 for daily.">
            <input className="input" type="number" min="5" value={scheduleForm.interval_minutes} onChange={e => setScheduleForm({ ...scheduleForm, interval_minutes: Number(e.target.value) })} required />
          </Field>
          <label className="flex items-center gap-2 text-sm text-gray-700" title="Run this schedule automatically.">
            <input type="checkbox" checked={scheduleForm.enabled} onChange={e => setScheduleForm({ ...scheduleForm, enabled: e.target.checked })} />
            Enabled
          </label>
          <label className="flex items-center gap-2 text-sm text-gray-700" title="Back up every active project when the schedule runs.">
            <input type="checkbox" checked={scheduleForm.all_projects} onChange={e => setScheduleForm({ ...scheduleForm, all_projects: e.target.checked, projects: e.target.checked ? [] : scheduleForm.projects })} />
            All active projects
          </label>
          {!scheduleForm.all_projects && (
            <div className="max-h-40 overflow-auto rounded-md border border-gray-200 p-2">
              {projects.map(project => (
                <label key={project.name} className="flex items-center gap-2 py-1 text-sm text-gray-700">
                  <input type="checkbox" checked={scheduleForm.projects.includes(project.name)} onChange={() => toggleScheduleProject(project.name)} />
                  {project.name}
                </label>
              ))}
            </div>
          )}
          <div className="rounded-md border border-gray-200 p-2">
            <div className="mb-1 text-sm font-medium text-gray-800">Endpoints</div>
            {enabledDestinations.map(destination => (
              <label key={destination.id} className="flex items-center gap-2 py-1 text-sm text-gray-700">
                <input type="checkbox" checked={scheduleForm.destination_ids.includes(destination.id)} onChange={() => toggleScheduleDestination(destination.id)} />
                {destination.name} <span className="text-xs text-gray-500">({destination.type})</span>
              </label>
            ))}
            {enabledDestinations.length === 0 && <div className="text-sm text-gray-500">No enabled endpoints selected; schedules will create local backups only.</div>}
          </div>
          <div className="flex gap-2">
            <button className="btn-primary flex-1" title="Save this backup schedule.">Save Schedule</button>
            {scheduleForm.id > 0 && (
              <button type="button" className="btn-secondary" onClick={() => setScheduleForm({ id: 0, name: '', projects: [], all_projects: true, destination_ids: [], enabled: true, interval_minutes: 1440 })} title="Clear the edit form.">New</button>
            )}
          </div>
        </form>

        <div className="section-panel">
          <h2 className="mb-3 text-lg font-semibold text-gray-950">Backup Schedules</h2>
          <div className="overflow-x-auto">
            <table className="w-full min-w-[820px] text-left text-sm">
              <thead>
                <tr className="border-b border-gray-200 text-xs uppercase text-gray-500">
                  <th className="py-2">Name</th>
                  <th>Scope</th>
                  <th>Interval</th>
                  <th>Next Run</th>
                  <th>Last Run</th>
                  <th>Status</th>
                  <th className="text-right">Actions</th>
                </tr>
              </thead>
              <tbody>
                {schedules.map(schedule => (
                  <tr key={schedule.id} className="border-b border-gray-100 align-top">
                    <td className="py-3 font-medium">{schedule.name}</td>
                    <td className="py-3 text-xs text-gray-600">{schedule.projects?.length ? schedule.projects.join(', ') : 'all active projects'}</td>
                    <td>{schedule.interval_minutes}m</td>
                    <td className="text-xs text-gray-500">{formatDate(schedule.next_run_at)}</td>
                    <td className="text-xs text-gray-500">{formatDate(schedule.last_run_at)}</td>
                    <td>
                      <Badge tone={!schedule.enabled ? 'gray' : schedule.last_status === 'failed' ? 'red' : schedule.last_status === 'partial' ? 'amber' : 'green'}>
                        {!schedule.enabled ? 'disabled' : schedule.last_status || 'ready'}
                      </Badge>
                      {schedule.last_error && <div className="mt-1 max-w-[260px] whitespace-pre-wrap text-xs text-red-700">{schedule.last_error}</div>}
                    </td>
                    <td className="py-3">
                      <div className="flex justify-end gap-1">
                        <button type="button" className="mini-button" onClick={() => editSchedule(schedule)} title="Edit this schedule.">Edit</button>
                        <button type="button" className="mini-button" onClick={() => runSchedule(schedule)} title="Run this schedule now.">Run</button>
                        <button type="button" className="mini-danger" onClick={() => deleteSchedule(schedule)} title="Delete this schedule.">Delete</button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
            {schedules.length === 0 && <div className="py-8 text-center text-sm text-gray-500">No backup schedules configured.</div>}
          </div>
        </div>
      </div>
    </div>
  );
}

function MetricsPanel({ summary, history, onRefresh }) {
  const lastSample = summary?.last_sampled_at ? new Date(summary.last_sampled_at).toLocaleString() : 'No samples yet';
  const hostHistory = aggregateHistory(history);
  return (
    <div className="section-panel">
      <div className="mb-4 flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
        <div>
          <h2 className="text-lg font-semibold text-gray-950">Operations History</h2>
          <p className="text-sm text-gray-600">Background sampled every 15-30 minutes. Last sample: {lastSample}</p>
        </div>
        <button title="Run project cache warmup and metrics collection now." onClick={onRefresh} className="btn-secondary">Refresh Metrics</button>
      </div>
      <div className="grid gap-3 md:grid-cols-3 xl:grid-cols-6">
        <MetricCard label="Containers" value={summary?.container_count || 0} />
        <MetricCard label="CPU avg" value={`${(summary?.cpu_percent_avg || 0).toFixed(1)}%`} />
        <MetricCard label="Memory avg" value={`${(summary?.memory_percent_avg || 0).toFixed(1)}%`} />
        <MetricCard label="Inbound" value={formatBytes(summary?.net_rx_bytes || 0)} />
        <MetricCard label="Outbound" value={formatBytes(summary?.net_tx_bytes || 0)} />
        <MetricCard label="Backups 24h" value={summary?.backup_count_24h || 0} sub={formatBytes(summary?.backup_bytes_24h || 0)} />
      </div>
      <div className="mt-4 grid gap-4 xl:grid-cols-2">
        <LineChart
          title="CPU and memory trend"
          points={hostHistory}
          series={[
            { key: 'cpu_percent_avg', label: 'CPU %', color: '#2563eb' },
            { key: 'memory_percent_avg', label: 'Memory %', color: '#16a34a' },
          ]}
          valueSuffix="%"
        />
        <LineChart
          title="Network traffic counters"
          points={hostHistory}
          series={[
            { key: 'net_rx_bytes', label: 'Inbound', color: '#0891b2' },
            { key: 'net_tx_bytes', label: 'Outbound', color: '#dc2626' },
          ]}
          formatter={formatBytes}
        />
      </div>
      <BackupActivity activity={summary?.backup_activity_24h || []} />
    </div>
  );
}

function aggregateHistory(points) {
  const grouped = new Map();
  for (const point of points || []) {
    const key = point.sampled_at;
    const current = grouped.get(key) || {
      sampled_at: key,
      container_count: 0,
      cpu_weighted: 0,
      memory_weighted: 0,
      memory_usage_bytes: 0,
      net_rx_bytes: 0,
      net_tx_bytes: 0,
    };
    const count = Number(point.container_count || 0);
    current.container_count += count;
    current.cpu_weighted += Number(point.cpu_percent_avg || 0) * count;
    current.memory_weighted += Number(point.memory_percent_avg || 0) * count;
    current.memory_usage_bytes += Number(point.memory_usage_bytes || 0);
    current.net_rx_bytes += Number(point.net_rx_bytes || 0);
    current.net_tx_bytes += Number(point.net_tx_bytes || 0);
    grouped.set(key, current);
  }
  return Array.from(grouped.values()).map(point => ({
    sampled_at: point.sampled_at,
    container_count: point.container_count,
    cpu_percent_avg: point.container_count ? point.cpu_weighted / point.container_count : 0,
    memory_percent_avg: point.container_count ? point.memory_weighted / point.container_count : 0,
    memory_usage_bytes: point.memory_usage_bytes,
    net_rx_bytes: point.net_rx_bytes,
    net_tx_bytes: point.net_tx_bytes,
  }));
}

function MetricCard({ label, value, sub }) {
  return (
    <div className="rounded-md border border-gray-200 p-3">
      <div className="text-lg font-semibold text-gray-950">{value}</div>
      <div className="text-xs text-gray-500">{label}</div>
      {sub && <div className="mt-1 text-xs text-gray-500">{sub}</div>}
    </div>
  );
}

function LineChart({ title, points, series, formatter, valueSuffix = '' }) {
  const width = 720;
  const height = 190;
  const pad = 28;
  const values = points.flatMap(p => series.map(s => Number(p[s.key] || 0)));
  const max = Math.max(1, ...values);
  const x = (i) => points.length <= 1 ? pad : pad + (i * (width - pad * 2)) / (points.length - 1);
  const y = (value) => height - pad - (Number(value || 0) / max) * (height - pad * 2);
  const label = formatter ? formatter(max) : `${max.toFixed(1)}${valueSuffix}`;
  return (
    <div className="rounded-md border border-gray-200 p-3">
      <div className="mb-2 flex items-center justify-between gap-3">
        <h3 className="text-sm font-semibold text-gray-950">{title}</h3>
        <div className="flex flex-wrap gap-2 text-xs text-gray-500">
          {series.map(s => <span key={s.key}><span style={{ backgroundColor: s.color }} className="mr-1 inline-block h-2 w-2 rounded-full" />{s.label}</span>)}
        </div>
      </div>
      {points.length === 0 ? (
        <div className="py-12 text-center text-sm text-gray-500">No history collected yet.</div>
      ) : (
        <svg viewBox={`0 0 ${width} ${height}`} role="img" aria-label={title} className="h-52 w-full">
          <line x1={pad} y1={height - pad} x2={width - pad} y2={height - pad} stroke="#94a3b8" strokeWidth="1" />
          <line x1={pad} y1={pad} x2={pad} y2={height - pad} stroke="#94a3b8" strokeWidth="1" />
          <text x={pad + 4} y={pad - 8} fill="#64748b" fontSize="12">{label}</text>
          {series.map(s => (
            <polyline
              key={s.key}
              fill="none"
              stroke={s.color}
              strokeWidth="2.5"
              points={points.map((p, i) => `${x(i)},${y(p[s.key])}`).join(' ')}
            />
          ))}
        </svg>
      )}
    </div>
  );
}

function BackupActivity({ activity }) {
  const max = Math.max(1, ...activity.map(p => (p.backups || 0) + (p.restores || 0) + (p.uploads || 0)));
  return (
    <div className="mt-4 rounded-md border border-gray-200 p-3">
      <div className="mb-2 flex items-center justify-between">
        <h3 className="text-sm font-semibold text-gray-950">Backup and restore activity</h3>
        <span className="text-xs text-gray-500">24h</span>
      </div>
      {activity.length === 0 ? (
        <div className="py-8 text-center text-sm text-gray-500">No backup activity recorded yet.</div>
      ) : (
        <div className="flex h-28 items-end gap-1">
          {activity.map(point => {
            const total = (point.backups || 0) + (point.restores || 0) + (point.uploads || 0);
            return (
              <div key={point.bucket_start} title={`${new Date(point.bucket_start).toLocaleString()}: ${total} events`} className="flex flex-1 items-end">
                <div className="w-full rounded-t bg-blue-600" style={{ height: `${Math.max(8, (total / max) * 100)}%` }} />
              </div>
            );
          })}
        </div>
      )}
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

function formatBytes(bytes) {
  const value = Number(bytes || 0);
  if (value < 1024) return `${value} B`;
  const units = ['KB', 'MB', 'GB', 'TB'];
  let size = value / 1024;
  let unit = 0;
  while (size >= 1024 && unit < units.length - 1) {
    size /= 1024;
    unit += 1;
  }
  return `${size.toFixed(size >= 10 ? 0 : 1)} ${units[unit]}`;
}

function StatCard({ label, value, active, title, onClick }) {
  return (
    <button type="button" title={title} onClick={onClick} className={`section-panel py-4 text-left transition hover:border-blue-300 hover:bg-blue-50 ${active ? 'border-blue-500 bg-blue-50 ring-2 ring-blue-100' : ''}`}>
      <div className="text-2xl font-semibold text-gray-950">{value}</div>
      <div className="text-sm text-gray-500">{label}</div>
    </button>
  );
}

function formatDate(value) {
  if (!value) return 'none';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return 'none';
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
  const tone = result.status === 'running' ? 'border-blue-300 bg-blue-50 text-blue-900' :
    result.status === 'error' ? 'border-red-300 bg-red-50 text-red-900' :
    'border-green-300 bg-green-50 text-green-900';
  const icon = result.status === 'running' ? <span className="spinner" aria-hidden="true"></span>
    : result.status === 'error' ? <span aria-hidden="true">✖</span>
    : <span aria-hidden="true">✓</span>;
  // Bulk actions return { results:[{project, action, success, output, exit_code}], total, success, failed }
  const bulk = result.result && Array.isArray(result.result.results) ? result.result : null;
  const failedProjects = bulk ? bulk.results.filter(r => !r.success) : [];
  const succeededProjects = bulk ? bulk.results.filter(r => r.success) : [];
  const [showFailed, setShowFailed] = useState(true);
  const [showSucceeded, setShowSucceeded] = useState(false);
  return (
    <div className={`sticky top-2 z-30 rounded-md border-2 px-4 py-3 text-sm shadow-lg ${tone}`}>
      <div className="flex items-start justify-between gap-4">
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2 text-base font-semibold">
            {icon}
            <span>
              {result.status === 'running' ? `Running ${result.label}…`
                : result.status === 'error' ? `Error during ${result.label}`
                : bulk ? `${result.label}: ${bulk.success} succeeded, ${bulk.failed} failed (of ${bulk.total})`
                : `${result.label} completed`}
            </span>
          </div>
          {result.error && <div className="mt-1 font-mono text-xs">{result.error}</div>}
          {result.job && <div className="mt-1 text-xs">Session: <span className="font-mono">{result.job.id}</span> · {result.job.status}{typeof result.job.exit_code === 'number' && ` · exit ${result.job.exit_code}`}</div>}
          {bulk && failedProjects.length > 0 && (
            <div className="mt-2">
              <button type="button" onClick={() => setShowFailed(!showFailed)} className="text-xs font-medium underline" title="Toggle failed project details.">
                {showFailed ? 'Hide' : 'Show'} {failedProjects.length} failure{failedProjects.length === 1 ? '' : 's'}
              </button>
              {showFailed && (
                <ul className="mt-1 space-y-2">
                  {failedProjects.map((r, i) => (
                    <li key={`f${i}`} className="rounded bg-red-100/60 p-2">
                      <div className="font-mono text-xs font-semibold">{r.project} — {r.action} (exit {r.exit_code ?? '?'})</div>
                      {r.output && <pre className="mt-1 max-h-40 overflow-auto whitespace-pre-wrap rounded bg-gray-950 p-2 font-mono text-[11px] text-gray-100">{r.output}</pre>}
                    </li>
                  ))}
                </ul>
              )}
            </div>
          )}
          {bulk && succeededProjects.length > 0 && (
            <div className="mt-2">
              <button type="button" onClick={() => setShowSucceeded(!showSucceeded)} className="text-xs font-medium underline" title="Toggle succeeded project details.">
                {showSucceeded ? 'Hide' : 'Show'} {succeededProjects.length} success{succeededProjects.length === 1 ? '' : 'es'}
              </button>
              {showSucceeded && (
                <ul className="mt-1 flex flex-wrap gap-1">
                  {succeededProjects.map((r, i) => (
                    <li key={`s${i}`} className="rounded bg-green-100/60 px-2 py-0.5 font-mono text-xs">{r.project}</li>
                  ))}
                </ul>
              )}
            </div>
          )}
          {!bulk && (result.job?.output || result.result?.output) && (
            <pre className="mt-2 max-h-80 overflow-auto whitespace-pre-wrap rounded bg-gray-950 p-3 font-mono text-xs text-gray-100">
              {result.job?.output || result.result?.output}
            </pre>
          )}
        </div>
        <button title="Dismiss this message." onClick={onDismiss} className="mini-button">Dismiss</button>
      </div>
    </div>
  );
}
