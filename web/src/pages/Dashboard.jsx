import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { projects, jobs, skills as skillsApi, system, registries, agents as agentsApi, schedules as schedulesApi, metrics as metricsApi } from '../api/client';

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
  const [agentList, setAgentList] = useState([]);
  const [scheduleList, setScheduleList] = useState([]);
  const [metricsSummary, setMetricsSummary] = useState(null);
  const [metricsHistory, setMetricsHistory] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [actionResult, setActionResult] = useState(null);
  const [selected, setSelected] = useState([]);
  const [filters, setFilters] = useState({ includeInactive: true, runningOnly: false, query: '' });
  const [quickFilter, setQuickFilter] = useState('all');
  const [timeout, setTimeoutValue] = useState(300);
  const [pruneMode, setPruneMode] = useState('safe');
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

  const fetchData = async () => {
    try {
      setLoading(true);
      const [projRes, skillRes, agentRes, scheduleRes, metricsRes, historyRes] = await Promise.all([
        projects.list({ include_inactive: filters.includeInactive ? 'true' : 'false', running_only: filters.runningOnly ? 'true' : 'false' }),
        skillsApi.list(),
        agentsApi.list(),
        schedulesApi.list(),
        metricsApi.summary(),
        metricsApi.history(24),
      ]);
      setProjectList(projRes.data || []);
      setSkillList(skillRes.data || []);
      setAgentList(agentRes.data || []);
      setScheduleList(scheduleRes.data || []);
      setMetricsSummary(metricsRes.data || null);
      setMetricsHistory(historyRes.data || []);
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
      setActionResult({ label: `delete ${project.name}`, status: 'error', error: err.message });
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
          <select value={pruneMode} onChange={e => setPruneMode(e.target.value)} className="input w-56" title="Choose which Docker prune command to run.">
            <option value="safe">Safe: images, networks, volumes</option>
            <option value="system">System prune</option>
            <option value="system-all">System prune --all --volumes</option>
            <option value="images">Images --all</option>
            <option value="volumes">Volumes</option>
            <option value="networks">Networks</option>
            <option value="builder">Builder cache --all</option>
          </select>
          <button title="Run the selected Docker prune command on this host." onClick={async () => {
            if (pruneMode === 'system-all' && !window.confirm('Run docker system prune --all --volumes? This removes unused images, build cache, networks, and volumes.')) return;
            setActionResult({ label: 'system prune', status: 'running' });
            try {
              const res = await system.prune(pruneMode);
              setActionResult({ label: 'system prune', status: 'done', result: res.data });
            } catch (err) {
              setActionResult({ label: 'system prune', status: 'error', error: err.message });
            }
          }} className="btn-danger">Prune</button>
        </div>
      </div>

      <div className="grid grid-cols-2 gap-3 lg:grid-cols-6">
        <StatCard label="Projects" value={projectList.length} active={quickFilter === 'all'} title="Show all discovered projects." onClick={() => setSummaryFilter('all')} />
        <StatCard label="Running" value={running} active={quickFilter === 'running'} title="Show projects with running containers." onClick={() => setSummaryFilter('running')} />
        <StatCard label="Inactive" value={inactive} active={quickFilter === 'inactive'} title="Show projects marked inactive." onClick={() => setSummaryFilter('inactive')} />
        <StatCard label="No updates" value={noUpdates} active={quickFilter === 'no_updates'} title="Show projects whose update policy skips updates." onClick={() => setSummaryFilter('no_updates')} />
        <StatCard label="Registry services" value={registryServices} active={quickFilter === 'registry'} title="Show projects with registry-backed services." onClick={() => setSummaryFilter('registry')} />
        <StatCard label="Custom builds" value={customServices} active={quickFilter === 'custom'} title="Show projects with custom build services." onClick={() => setSummaryFilter('custom')} />
      </div>

      <MetricsPanel summary={metricsSummary} history={metricsHistory} onRefresh={async () => {
        setActionResult({ label: 'metrics refresh', status: 'running' });
        try {
          const res = await metricsApi.refresh();
          const hist = await metricsApi.history(24);
          setMetricsSummary(res.data || null);
          setMetricsHistory(hist.data || []);
          setActionResult({ label: 'metrics refresh', status: 'done' });
        } catch (err) {
          setActionResult({ label: 'metrics refresh', status: 'error', error: err.message });
        }
      }} />

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
              {quickFilter !== 'all' && (
                <button title="Clear the summary-card filter." onClick={() => setSummaryFilter('all')} className="mini-button" type="button">
                  Clear {quickFilter.replace('_', ' ')}
                </button>
              )}
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
                        <button key={action.key} title={action.key === 'update' && p.update_policy?.effective_policy === 'no_updates' ? 'Updates are disabled for this project; this records a skipped session.' : action.title} onClick={() => runAction(p.name, action.key)} className={action.key === 'down' ? 'mini-danger' : 'mini-button'}>
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

      <div className="grid gap-4 xl:grid-cols-[1fr_420px]">
        <div className="section-panel">
          <div className="mb-3 flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
            <div>
              <h2 className="text-lg font-semibold text-gray-950">Scheduled Updates</h2>
              <p className="text-sm text-gray-600">Run update, pull, restart, start, stop, or status on a fixed interval.</p>
            </div>
            <Badge tone="blue">{scheduleList.length} schedules</Badge>
          </div>
          <form onSubmit={saveSchedule} className="mb-4 grid gap-3 lg:grid-cols-6">
            <Field label="Agent" title="Leave local for this server, or choose a registered agent.">
              <select value={scheduleForm.agent_id} onChange={e => setScheduleForm({ ...scheduleForm, agent_id: e.target.value })} className="input">
                <option value="">Local</option>
                {agentList.map(agent => <option key={agent.id} value={agent.id}>{agent.name}</option>)}
              </select>
            </Field>
            <Field label="Project" title="Project folder name on the selected server or agent.">
              <input required list="local-projects" value={scheduleForm.project} onChange={e => setScheduleForm({ ...scheduleForm, project: e.target.value })} className="input" placeholder="project-name" />
              <datalist id="local-projects">
                {projectList.map(p => <option key={p.name} value={p.name} />)}
              </datalist>
            </Field>
            <Field label="Action" title="Scheduled compose action. Update respects no-update project policy.">
              <select value={scheduleForm.action} onChange={e => setScheduleForm({ ...scheduleForm, action: e.target.value })} className="input">
                {ACTIONS.map(action => <option key={action.key} value={action.key}>{action.label}</option>)}
                <option value="status">Status</option>
              </select>
            </Field>
            <Field label="Every minutes" title="Minimum 5 minutes. Use 1440 for daily.">
              <input type="number" min="5" value={scheduleForm.interval_minutes} onChange={e => setScheduleForm({ ...scheduleForm, interval_minutes: Number(e.target.value) })} className="input" />
            </Field>
            <Field label="Timeout" title="Seconds before pull/update commands time out.">
              <input type="number" min="0" value={scheduleForm.timeout_seconds} onChange={e => setScheduleForm({ ...scheduleForm, timeout_seconds: Number(e.target.value) })} className="input" />
            </Field>
            <div className="flex items-end gap-2">
              <label className="mb-2 flex items-center gap-2 text-sm text-gray-700" title="Disable to keep the schedule saved without running it.">
                <input type="checkbox" checked={scheduleForm.enabled} onChange={e => setScheduleForm({ ...scheduleForm, enabled: e.target.checked })} />
                Enabled
              </label>
              <button type="submit" title="Save this schedule to MariaDB." className="btn-primary">{scheduleForm.id ? 'Update' : 'Add'}</button>
            </div>
          </form>
          <div className="overflow-x-auto">
            <table className="w-full min-w-[840px] text-left text-sm">
              <thead><tr className="border-b border-gray-200 text-xs uppercase text-gray-500"><th className="py-2">Target</th><th>Action</th><th>Interval</th><th>Next</th><th>Last</th><th className="text-right">Tools</th></tr></thead>
              <tbody>
                {scheduleList.map(schedule => (
                  <tr key={schedule.id} className="border-b border-gray-100">
                    <td className="py-2">
                      <div className="font-medium">{schedule.project}</div>
                      <div className="text-xs text-gray-500">{schedule.agent_name || 'Local'}</div>
                    </td>
                    <td><Badge tone={schedule.enabled ? 'green' : 'gray'}>{schedule.action}</Badge></td>
                    <td>{schedule.interval_minutes}m</td>
                    <td className="text-xs text-gray-500">{formatDate(schedule.next_run_at)}</td>
                    <td className="text-xs text-gray-500">{schedule.last_status || 'never'} {schedule.last_job_id ? `· ${schedule.last_job_id}` : ''}</td>
                    <td>
                      <div className="flex justify-end gap-1">
                        <button title="Run this schedule now and update its next-run time." onClick={() => runSchedule(schedule)} className="mini-button">Run</button>
                        <button title="Load this schedule into the edit form." onClick={() => editSchedule(schedule)} className="mini-button">Edit</button>
                        <button title="Delete this schedule." onClick={() => deleteSchedule(schedule)} className="mini-danger">Delete</button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
            {scheduleList.length === 0 && <div className="py-6 text-gray-500">No schedules configured.</div>}
          </div>
        </div>

        <form onSubmit={saveAgent} className="section-panel space-y-3">
          <h2 className="text-lg font-semibold text-gray-950">Agents</h2>
          <Field label="Name" title="Friendly name for the remote compose host.">
            <input value={agentForm.name} onChange={e => setAgentForm({ ...agentForm, name: e.target.value })} className="input" placeholder="docker03" />
          </Field>
          <Field label="Agent URL" title="Base URL for the remote Compose Manager agent, for example https://docker03.example.com.">
            <input value={agentForm.base_url} onChange={e => setAgentForm({ ...agentForm, base_url: e.target.value })} className="input" placeholder="https://host:8192" />
          </Field>
          <Field label="Token" title="Bearer token used by the controller to call this agent.">
            <input type="password" value={agentForm.token} onChange={e => setAgentForm({ ...agentForm, token: e.target.value })} className="input" />
          </Field>
          <label className="flex items-center gap-2 text-sm text-gray-700" title="Disabled agents remain saved but scheduled actions will not run.">
            <input type="checkbox" checked={agentForm.enabled} onChange={e => setAgentForm({ ...agentForm, enabled: e.target.checked })} />
            Enabled
          </label>
          <button type="submit" title="Save this agent connection." className="btn-secondary w-full">Save Agent</button>
          <div className="space-y-2 pt-2">
            {agentList.map(agent => (
              <div key={agent.id} className="flex items-center justify-between gap-2 rounded-md border border-gray-200 p-2 text-sm">
                <div>
                  <div className="font-medium">{agent.name}</div>
                  <div className="max-w-[260px] truncate text-xs text-gray-500" title={agent.base_url}>{agent.base_url}</div>
                </div>
                <div className="flex items-center gap-2">
                  <Badge tone={agent.enabled ? 'green' : 'gray'}>{agent.enabled ? 'on' : 'off'}</Badge>
                  <button title="Delete this agent and its schedules." type="button" onClick={() => deleteAgent(agent)} className="mini-danger">Delete</button>
                </div>
              </div>
            ))}
            {agentList.length === 0 && <div className="py-3 text-sm text-gray-500">No agents registered.</div>}
          </div>
        </form>
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
