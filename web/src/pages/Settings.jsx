import { useEffect, useMemo, useState } from 'react';
import { auth, users, backup, projects, agents, schedules, registries, dockerSettings } from '../api/client';
import { getThemePreference, setThemePreference } from '../theme';

const ACTIONS = [
  { key: 'update', label: 'Update' },
  { key: 'pull', label: 'Pull' },
  { key: 'up', label: 'Start' },
  { key: 'restart', label: 'Restart' },
  { key: 'down', label: 'Stop' },
  { key: 'status', label: 'Status' },
];

const DESTINATION_TYPES = [
  { value: 'local', label: 'Linux path' },
  { value: 'mount', label: 'Mounted path' },
  { value: 'cifs', label: 'CIFS mount' },
  { value: 'nfs', label: 'NFS mount' },
  { value: 'sftp', label: 'SFTP' },
  { value: 'ftp', label: 'FTP' },
  { value: 's3', label: 'S3' },
];

const emptyDestinationForm = {
  id: 0,
  name: '',
  type: 'local',
  enabled: true,
  config: {
    path: '',
    host: '',
    port: '',
    username: '',
    remote_path: '',
    key_file: '',
    bucket: '',
    prefix: '',
    endpoint: '',
    region: '',
    provider: 'Other',
  },
  secrets: {
    password: '',
    access_key_id: '',
    secret_access_key: '',
  },
};

const emptyScheduleForm = {
  id: 0,
  agent_id: '',
  project: '',
  action: 'update',
  enabled: true,
  interval_minutes: 1440,
  timeout_seconds: 300,
};

const emptyAgentForm = { name: '', base_url: '', token: '', enabled: true };

const emptyDockerForm = {
  live_restore: false,
  log_driver: 'json-file',
  log_max_size: '10m',
  log_max_file: '3',
  dns: '',
  registry_mirrors: '',
  insecure_registries: '',
  default_address_pools: '',
  bip: '',
  ipv6: false,
  fixed_cidr_v6: '',
  expose_tcp: false,
  tcp_bind: '127.0.0.1',
  tcp_port: '2376',
};

export default function Settings() {
  const [me, setMe] = useState(null);
  const [activeTab, setActiveTab] = useState('account');
  const [userList, setUserList] = useState([]);
  const [destinationList, setDestinationList] = useState([]);
  const [projectList, setProjectList] = useState([]);
  const [agentList, setAgentList] = useState([]);
  const [agentProjects, setAgentProjects] = useState({});
  const [scheduleList, setScheduleList] = useState([]);
  const [message, setMessage] = useState('');
  const [error, setError] = useState('');
  const [form, setForm] = useState({ username: '', password: '', role: 'operator' });
  const [resetForm, setResetForm] = useState({ username: '', password: '' });
  const [destinationForm, setDestinationForm] = useState(emptyDestinationForm);
  const [scheduleForm, setScheduleForm] = useState(emptyScheduleForm);
  const [agentForm, setAgentForm] = useState(emptyAgentForm);
  const [registryForm, setRegistryForm] = useState({ registry: '', username: '', password: '' });
  const [dockerHubForm, setDockerHubForm] = useState({ username: '', password: '' });
  const [dockerForm, setDockerForm] = useState(emptyDockerForm);
  const [dockerRaw, setDockerRaw] = useState('{}\n');
  const [dockerStatus, setDockerStatus] = useState(null);
  const [theme, setTheme] = useState(getThemePreference());

  const admin = me?.role === 'admin';
  const tabs = useMemo(() => {
    const base = [{ key: 'account', label: 'Account', title: 'Theme, current user, and sign out.' }];
    if (!admin) return base;
    return [
      ...base,
      { key: 'users', label: 'Users', title: 'Manage dashboard users and passwords.' },
      { key: 'agents', label: 'Agents', title: 'Register and edit remote Compose Manager agents.' },
      { key: 'schedules', label: 'Scheduled Updates', title: 'Schedule local or agent project updates.' },
      { key: 'registries', label: 'Registry Logins', title: 'Docker Hub account and private registry logins.' },
      { key: 'docker', label: 'Docker Settings', title: 'Edit Docker daemon.json settings for this host.' },
      { key: 'backups', label: 'Backup Endpoints', title: 'Configure local and remote backup destinations.' },
    ];
  }, [admin]);

  const load = async () => {
    setError('');
    try {
      const meRes = await auth.me();
      setMe(meRes.data);
      if (meRes.data.role !== 'admin') return;
      const [usersRes, destinationsRes, projectsRes, agentsRes, schedulesRes] = await Promise.all([
        users.list(),
        backup.destinations(),
        projects.list({ include_inactive: 'true', running_only: 'false' }),
        agents.list(),
        schedules.list(),
      ]);
      setUserList(usersRes.data || []);
      setDestinationList(destinationsRes.data || []);
      setProjectList(projectsRes.data || []);
      setAgentList(agentsRes.data || []);
      setScheduleList(schedulesRes.data || []);
    } catch (err) {
      setError(err.message);
    }
  };

  useEffect(() => { load(); }, []);

  useEffect(() => {
    if (me && !admin && activeTab !== 'account') setActiveTab('account');
  }, [me, admin, activeTab]);

  useEffect(() => {
    if (admin && activeTab === 'docker') loadDockerSettings();
  }, [admin, activeTab]);

  const showMessage = (text) => {
    setError('');
    setMessage(text);
  };

  const showError = (err) => {
    setMessage('');
    setError(err.message || String(err));
  };

  const logout = async () => {
    await auth.logout().catch(() => {});
    localStorage.removeItem('cm_token');
    localStorage.removeItem('cm_user');
    localStorage.removeItem('cm_api_key');
    window.location.href = '/';
  };

  const createUser = async (event) => {
    event.preventDefault();
    try {
      await users.create(form);
      showMessage(`Created user ${form.username}`);
      setForm({ username: '', password: '', role: 'operator' });
      load();
    } catch (err) {
      showError(err);
    }
  };

  const resetPassword = async (event) => {
    event.preventDefault();
    try {
      await users.setPassword(resetForm.username, resetForm.password);
      showMessage(`Updated password for ${resetForm.username}`);
      setResetForm({ username: '', password: '' });
    } catch (err) {
      showError(err);
    }
  };

  const deleteUser = async (username) => {
    if (!window.confirm(`Delete user ${username}?`)) return;
    try {
      await users.delete(username);
      showMessage(`Deleted user ${username}`);
      load();
    } catch (err) {
      showError(err);
    }
  };

  const editAgent = (agent) => {
    setAgentForm({ name: agent.name, base_url: agent.base_url, token: '', enabled: Boolean(agent.enabled) });
  };

  const saveAgent = async (event) => {
    event.preventDefault();
    try {
      const res = await agents.save(agentForm);
      showMessage(`Saved agent ${res.data.name}`);
      setAgentForm(emptyAgentForm);
      load();
    } catch (err) {
      showError(err);
    }
  };

  const deleteAgent = async (agent) => {
    if (!window.confirm(`Delete agent ${agent.name}? Schedules for this agent will also be removed.`)) return;
    try {
      await agents.delete(agent.id);
      showMessage(`Deleted agent ${agent.name}`);
      load();
    } catch (err) {
      showError(err);
    }
  };

  const loadAgentProjects = async (agentID) => {
    if (!agentID || agentProjects[agentID]) return;
    try {
      const res = await agents.projects(agentID);
      setAgentProjects(current => ({ ...current, [agentID]: res.data || [] }));
    } catch (err) {
      showError(err);
    }
  };

  const selectScheduleAgent = (agentID) => {
    setScheduleForm({ ...scheduleForm, agent_id: agentID, project: '' });
    loadAgentProjects(agentID);
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
      const res = await schedules.save(body);
      showMessage(`Saved schedule for ${res.data.project}`);
      setScheduleForm(emptyScheduleForm);
      load();
    } catch (err) {
      showError(err);
    }
  };

  const editSchedule = (schedule) => {
    const agentID = schedule.agent_id ? String(schedule.agent_id) : '';
    setScheduleForm({
      id: schedule.id,
      agent_id: agentID,
      project: schedule.project,
      action: schedule.action || 'update',
      enabled: Boolean(schedule.enabled),
      interval_minutes: schedule.interval_minutes || 1440,
      timeout_seconds: schedule.timeout_seconds || 300,
    });
    loadAgentProjects(agentID);
  };

  const runSchedule = async (schedule) => {
    try {
      const res = await schedules.run(schedule.id);
      showMessage(`Started schedule for ${res.data.project}`);
      load();
    } catch (err) {
      showError(err);
    }
  };

  const deleteSchedule = async (schedule) => {
    if (!window.confirm(`Delete schedule for ${schedule.project}?`)) return;
    try {
      await schedules.delete(schedule.id);
      showMessage(`Deleted schedule for ${schedule.project}`);
      load();
    } catch (err) {
      showError(err);
    }
  };

  const loginRegistry = async (event) => {
    event.preventDefault();
    try {
      await registries.login(registryForm);
      showMessage(`Logged in to ${registryForm.registry || 'Docker Hub'}`);
      setRegistryForm({ registry: registryForm.registry, username: registryForm.username, password: '' });
    } catch (err) {
      showError(err);
    }
  };

  const loginDockerHub = async (event) => {
    event.preventDefault();
    try {
      await registries.login({ registry: '', username: dockerHubForm.username, password: dockerHubForm.password });
      showMessage(`Logged in to Docker Hub as ${dockerHubForm.username}`);
      setDockerHubForm({ username: dockerHubForm.username, password: '' });
    } catch (err) {
      showError(err);
    }
  };

  const loadDockerSettings = async () => {
    try {
      const res = await dockerSettings.daemon();
      const data = res.data || {};
      setDockerStatus(data);
      setDockerRaw(data.raw || '{}\n');
      setDockerForm(formFromDockerConfig(data.config || {}));
    } catch (err) {
      showError(err);
    }
  };

  const saveDockerSettings = async (event) => {
    event.preventDefault();
    try {
      const config = buildDockerConfig(dockerRaw, dockerForm);
      const res = await dockerSettings.saveDaemon({ config });
      const data = res.data || {};
      setDockerStatus(data);
      setDockerRaw(data.raw || JSON.stringify(config, null, 2) + '\n');
      setDockerForm(formFromDockerConfig(data.config || config));
      showMessage(`Saved Docker daemon settings${data.backup ? ` with backup ${data.backup}` : ''}. Restart Docker for changes to apply.`);
    } catch (err) {
      showError(err);
    }
  };

  const setDestinationConfig = (key, value) => {
    setDestinationForm({ ...destinationForm, config: { ...destinationForm.config, [key]: value } });
  };

  const setDestinationSecret = (key, value) => {
    setDestinationForm({ ...destinationForm, secrets: { ...destinationForm.secrets, [key]: value } });
  };

  const editDestination = (destination) => {
    setDestinationForm({
      ...emptyDestinationForm,
      id: destination.id,
      name: destination.name,
      type: destination.type,
      enabled: Boolean(destination.enabled),
      config: { ...emptyDestinationForm.config, ...(destination.config || {}) },
      secrets: { ...emptyDestinationForm.secrets },
    });
  };

  const saveDestination = async (event) => {
    event.preventDefault();
    const body = {
      id: destinationForm.id || undefined,
      name: destinationForm.name,
      type: destinationForm.type,
      enabled: destinationForm.enabled,
      config: pruneMap(destinationForm.config),
      secrets: pruneMap(destinationForm.secrets),
    };
    try {
      const res = await backup.saveDestination(body);
      showMessage(`Saved backup endpoint ${res.data.name}`);
      setDestinationForm(emptyDestinationForm);
      load();
    } catch (err) {
      showError(err);
    }
  };

  const testDestination = async (destination) => {
    try {
      const res = await backup.testDestination(destination.id);
      showMessage(`Backup endpoint ${destination.name}: ${res.data.success ? 'success' : res.data.error || 'failed'}`);
    } catch (err) {
      showError(err);
    }
  };

  const deleteDestination = async (destination) => {
    if (!window.confirm(`Delete backup endpoint ${destination.name}? Existing backup files are not removed.`)) return;
    try {
      await backup.deleteDestination(destination.id);
      showMessage(`Deleted backup endpoint ${destination.name}`);
      load();
    } catch (err) {
      showError(err);
    }
  };

  const updateTheme = (value) => {
    setTheme(value);
    setThemePreference(value);
  };

  const availableProjects = scheduleForm.agent_id ? (agentProjects[scheduleForm.agent_id] || []) : projectList;

  return (
    <div className="mx-auto max-w-6xl space-y-6">
      <div className="section-panel">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <h1 className="text-xl font-semibold text-gray-950">Settings</h1>
            <p className="mt-1 text-sm text-gray-600">{me ? `${me.username} (${me.role})` : 'Loading account...'}</p>
          </div>
          <div className="flex flex-wrap gap-2">
            {tabs.map(tab => (
              <button key={tab.key} type="button" onClick={() => setActiveTab(tab.key)} className={activeTab === tab.key ? 'btn-primary' : 'btn-secondary'} title={tab.title}>
                {tab.label}
              </button>
            ))}
          </div>
        </div>
      </div>

      {error && <div className="rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-800">{error}</div>}
      {message && <div className="rounded-md border border-green-200 bg-green-50 p-3 text-sm text-green-800">{message}</div>}

      {activeTab === 'account' && (
        <div className="section-panel">
          <div className="grid gap-4 md:grid-cols-[1fr_300px] md:items-end">
            <div>
              <h2 className="text-lg font-semibold text-gray-950">Account</h2>
              <p className="mt-1 text-sm text-gray-600">Theme preference is saved in this browser. Signing out removes the current browser session.</p>
            </div>
            <div className="flex flex-wrap items-center gap-3">
              <label className="flex items-center gap-2 text-sm text-gray-700" title="Choose light, dark, or follow the operating system theme.">
                Theme
                <select value={theme} onChange={e => updateTheme(e.target.value)} className="input w-36">
                  <option value="system">System</option>
                  <option value="light">Light</option>
                  <option value="dark">Dark</option>
                </select>
              </label>
              <button onClick={logout} className="btn-secondary">Sign Out</button>
            </div>
          </div>
        </div>
      )}

      {admin && activeTab === 'users' && (
        <div className="grid gap-6 lg:grid-cols-[1fr_360px]">
          <div className="section-panel">
            <h2 className="mb-3 text-lg font-semibold text-gray-950">Users</h2>
            <div className="overflow-x-auto">
              <table className="w-full min-w-[620px] text-left text-sm">
                <thead><tr className="border-b border-gray-200 text-xs uppercase text-gray-500"><th className="py-2">Username</th><th>Role</th><th>Created</th><th className="text-right">Actions</th></tr></thead>
                <tbody>
                  {userList.map(user => (
                    <tr key={user.username} className="border-b border-gray-100">
                      <td className="py-2 font-medium">{user.username}</td>
                      <td>{user.role}</td>
                      <td className="text-gray-500">{new Date(user.created_at).toLocaleString()}</td>
                      <td className="text-right"><button onClick={() => deleteUser(user.username)} className="mini-danger" title="Delete this user. The last admin cannot be deleted.">Delete</button></td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
          <div className="space-y-4">
            <form onSubmit={createUser} className="section-panel space-y-3">
              <h2 className="text-lg font-semibold text-gray-950">Add User</h2>
              <input className="input" placeholder="username" value={form.username} onChange={e => setForm({ ...form, username: e.target.value })} required title="New dashboard username." />
              <input className="input" type="password" placeholder="password, 12+ chars" value={form.password} onChange={e => setForm({ ...form, password: e.target.value })} required title="Use a strong password. Minimum length is enforced by the server." />
              <select className="input" value={form.role} onChange={e => setForm({ ...form, role: e.target.value })} title="Admins can manage users and settings. Operators can manage projects.">
                <option value="operator">operator</option>
                <option value="admin">admin</option>
              </select>
              <button className="btn-primary w-full">Create User</button>
            </form>
            <form onSubmit={resetPassword} className="section-panel space-y-3">
              <h2 className="text-lg font-semibold text-gray-950">Reset Password</h2>
              <input className="input" placeholder="username" value={resetForm.username} onChange={e => setResetForm({ ...resetForm, username: e.target.value })} required title="Existing dashboard username to reset." />
              <input className="input" type="password" placeholder="new password, 12+ chars" value={resetForm.password} onChange={e => setResetForm({ ...resetForm, password: e.target.value })} required title="New password. Minimum length is enforced by the server." />
              <button className="btn-secondary w-full">Update Password</button>
            </form>
          </div>
        </div>
      )}

      {admin && activeTab === 'agents' && (
        <div className="grid gap-6 xl:grid-cols-[1fr_420px]">
          <div className="space-y-4">
            <div className="section-panel">
              <h2 className="text-lg font-semibold text-gray-950">Agent Setup</h2>
              <div className="mt-3 grid gap-3 text-sm text-gray-700 xl:grid-cols-3">
                <div className="rounded-md border border-gray-200 p-3">
                  <div className="font-medium text-gray-950">1. Prepare agent state</div>
                  <pre className="mt-2 overflow-auto rounded bg-gray-950 p-3 font-mono text-xs text-gray-100">git clone https://github.com/arphost-com/Compose-Manager.git{"\n"}cd Compose-Manager{"\n"}./scripts/prepare-state.sh --agent .env</pre>
                </div>
                <div className="rounded-md border border-gray-200 p-3">
                  <div className="font-medium text-gray-950">2. Start the agent only</div>
                  <pre className="mt-2 overflow-auto rounded bg-gray-950 p-3 font-mono text-xs text-gray-100">docker compose --env-file .env \{"\n"}  -f docker-compose.agent.yml \{"\n"}  up -d --build</pre>
                </div>
                <div className="rounded-md border border-gray-200 p-3">
                  <div className="font-medium text-gray-950">3. Register below</div>
                  <pre className="mt-2 overflow-auto rounded bg-gray-950 p-3 font-mono text-xs text-gray-100">grep -E '^(HOST_URL|AGENT_NAME|AGENT_TOKEN|DOCKER_ROOT)=' .env{"\n"}# Agent URL: HOST_URL{"\n"}# Token: AGENT_TOKEN</pre>
                </div>
              </div>
              <p className="mt-3 text-sm text-gray-600">Agent mode uses `DOCKER_ROOT` on the remote host and mounts it into the agent as `/docker`; the setup script writes `APP_MODE=agent`, `AGENT_NAME`, `AGENT_TOKEN`, and `HOST_URL` into `.env`. Register `HOST_URL` as the Agent URL and `AGENT_TOKEN` as the token.</p>
            </div>
            <div className="section-panel">
              <h2 className="mb-3 text-lg font-semibold text-gray-950">Registered Agents</h2>
              <div className="overflow-x-auto">
                <table className="w-full min-w-[760px] text-left text-sm">
                  <thead><tr className="border-b border-gray-200 text-xs uppercase text-gray-500"><th className="py-2">Name</th><th>URL</th><th>Status</th><th>Last Seen</th><th className="text-right">Actions</th></tr></thead>
                  <tbody>
                    {agentList.map(agent => (
                      <tr key={agent.id} className="border-b border-gray-100">
                        <td className="py-2 font-medium">{agent.name}</td>
                        <td className="max-w-[280px] truncate font-mono text-xs text-gray-600" title={agent.base_url}>{agent.base_url}</td>
                        <td><Badge tone={agent.enabled ? 'green' : 'gray'}>{agent.enabled ? 'enabled' : 'disabled'}</Badge></td>
                        <td className="text-xs text-gray-500">{formatDate(agent.last_seen)}</td>
                        <td className="space-x-2 text-right">
                          <button onClick={() => editAgent(agent)} className="mini-button" title="Load this agent into the edit form. Leave token blank to keep the saved token.">Edit</button>
                          <button onClick={() => deleteAgent(agent)} className="mini-danger" title="Delete this agent and its schedules.">Delete</button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
                {agentList.length === 0 && <div className="py-6 text-sm text-gray-500">No agents registered.</div>}
              </div>
            </div>
          </div>
          <form onSubmit={saveAgent} className="section-panel space-y-3">
            <h2 className="text-lg font-semibold text-gray-950">{agentForm.name ? 'Edit Agent' : 'Add Agent'}</h2>
            <Field label="Name" title="Friendly unique name for the remote compose host. Saving an existing name updates it.">
              <input value={agentForm.name} onChange={e => setAgentForm({ ...agentForm, name: e.target.value })} className="input" placeholder="docker03" required />
            </Field>
            <Field label="Agent URL" title="Base URL for the remote Compose Manager agent, for example https://docker03.example.com:8192.">
              <input value={agentForm.base_url} onChange={e => setAgentForm({ ...agentForm, base_url: e.target.value })} className="input" placeholder="https://host:8192" required />
            </Field>
            <Field label="Token" title="Bearer token used by the controller to call this agent. Leave blank when editing to keep the saved token.">
              <input type="password" value={agentForm.token} onChange={e => setAgentForm({ ...agentForm, token: e.target.value })} className="input" placeholder={agentForm.name ? 'leave blank to keep saved token' : 'agent token'} />
            </Field>
            <label className="flex items-center gap-2 text-sm text-gray-700" title="Disabled agents remain saved but scheduled actions will not run.">
              <input type="checkbox" checked={agentForm.enabled} onChange={e => setAgentForm({ ...agentForm, enabled: e.target.checked })} />
              Enabled
            </label>
            <div className="flex gap-2">
              <button type="submit" className="btn-primary">Save Agent</button>
              {agentForm.name && <button type="button" onClick={() => setAgentForm(emptyAgentForm)} className="btn-secondary">Clear</button>}
            </div>
          </form>
        </div>
      )}

      {admin && activeTab === 'schedules' && (
        <div className="section-panel">
          <div className="mb-4 flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
            <div>
              <h2 className="text-lg font-semibold text-gray-950">Scheduled Updates</h2>
              <p className="text-sm text-gray-600">Choose a server or agent, then pick one of its discovered projects from the dropdown.</p>
            </div>
            <Badge tone="blue">{scheduleList.length} schedules</Badge>
          </div>
          <form onSubmit={saveSchedule} className="mb-5 grid gap-3 lg:grid-cols-6">
            <Field label="Target host" title="Leave as this server for local projects, or select a registered agent.">
              <select value={scheduleForm.agent_id} onChange={e => selectScheduleAgent(e.target.value)} className="input">
                <option value="">This server</option>
                {agentList.map(agent => <option key={agent.id} value={agent.id}>{agent.name}</option>)}
              </select>
            </Field>
            <Field label="Project" title="Available projects from the selected server or agent.">
              <select required value={scheduleForm.project} onChange={e => setScheduleForm({ ...scheduleForm, project: e.target.value })} className="input">
                <option value="">Choose project</option>
                {availableProjects.map(project => <option key={project.name} value={project.name}>{project.name}</option>)}
              </select>
            </Field>
            <Field label="Action" title="Scheduled compose action. Update respects no-update project policy.">
              <select value={scheduleForm.action} onChange={e => setScheduleForm({ ...scheduleForm, action: e.target.value })} className="input">
                {ACTIONS.map(action => <option key={action.key} value={action.key}>{action.label}</option>)}
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
              <button type="submit" className="btn-primary">{scheduleForm.id ? 'Update' : 'Add'}</button>
            </div>
          </form>
          <div className="overflow-x-auto">
            <table className="w-full min-w-[840px] text-left text-sm">
              <thead><tr className="border-b border-gray-200 text-xs uppercase text-gray-500"><th className="py-2">Target</th><th>Action</th><th>Interval</th><th>Next</th><th>Last</th><th className="text-right">Tools</th></tr></thead>
              <tbody>
                {scheduleList.map(schedule => (
                  <tr key={schedule.id} className="border-b border-gray-100">
                    <td className="py-2"><div className="font-medium">{schedule.project}</div><div className="text-xs text-gray-500">{schedule.agent_name || 'This server'}</div></td>
                    <td><Badge tone={schedule.enabled ? 'green' : 'gray'}>{schedule.action}</Badge></td>
                    <td>{schedule.interval_minutes}m</td>
                    <td className="text-xs text-gray-500">{formatDate(schedule.next_run_at)}</td>
                    <td className="text-xs text-gray-500">{schedule.last_status || 'never'} {schedule.last_job_id ? `- ${schedule.last_job_id}` : ''}</td>
                    <td>
                      <div className="flex justify-end gap-1">
                        <button title="Run this schedule now." onClick={() => runSchedule(schedule)} className="mini-button">Run</button>
                        <button title="Load this schedule into the edit form." onClick={() => editSchedule(schedule)} className="mini-button">Edit</button>
                        <button title="Delete this schedule." onClick={() => deleteSchedule(schedule)} className="mini-danger">Delete</button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
            {scheduleList.length === 0 && <div className="py-6 text-sm text-gray-500">No schedules configured.</div>}
          </div>
        </div>
      )}

      {admin && activeTab === 'registries' && (
        <div className="space-y-4">
          <form onSubmit={loginDockerHub} className="section-panel space-y-3">
            <h2 className="text-lg font-semibold text-gray-950">Docker Hub Account</h2>
            <p className="text-sm text-gray-600">Authenticated Docker Hub pulls raise this host's rate limit from 100 to 200 pulls per 6 hours and are required for private Docker Hub repos. Use a Docker Hub personal access token when possible.</p>
            <div className="grid gap-3 md:grid-cols-2">
              <Field label="Docker Hub username" title="Docker Hub account username.">
                <input required autoComplete="username" value={dockerHubForm.username} onChange={e => setDockerHubForm({ ...dockerHubForm, username: e.target.value })} className="input" placeholder="dockerhub-username" />
              </Field>
              <Field label="Password or access token" title="Sent to docker login via password-stdin. Not stored by the web app after submission. Prefer a Docker Hub PAT over an account password.">
                <input required type="password" autoComplete="new-password" value={dockerHubForm.password} onChange={e => setDockerHubForm({ ...dockerHubForm, password: e.target.value })} className="input" />
              </Field>
            </div>
            <button type="submit" className="btn-primary">Login to Docker Hub</button>
          </form>

          <form onSubmit={loginRegistry} className="section-panel space-y-3">
            <h2 className="text-lg font-semibold text-gray-950">Private Registry Login</h2>
            <p className="text-sm text-gray-600">Login Docker to a non-Docker-Hub registry such as GitLab, GHCR, or a self-hosted registry.</p>
            <div className="grid gap-3 md:grid-cols-3">
              <Field label="Registry" title="Docker registry host only, such as registry.gitlab.com. Leave blank for Docker Hub — but prefer the Docker Hub Account panel above for Docker Hub logins.">
                <input required value={registryForm.registry} onChange={e => setRegistryForm({ ...registryForm, registry: e.target.value })} className="input" placeholder="registry.gitlab.com" />
              </Field>
              <Field label="Username" title="Registry username or deploy token username.">
                <input required value={registryForm.username} onChange={e => setRegistryForm({ ...registryForm, username: e.target.value })} className="input" />
              </Field>
              <Field label="Token or password" title="Sent to docker login via password-stdin. It is not stored by the web app after submission.">
                <input required type="password" value={registryForm.password} onChange={e => setRegistryForm({ ...registryForm, password: e.target.value })} className="input" />
              </Field>
            </div>
            <button type="submit" className="btn-primary">Login Registry</button>
          </form>
        </div>
      )}

      {admin && activeTab === 'docker' && (
        <form onSubmit={saveDockerSettings} className="space-y-4">
          <div className="section-panel">
            <div className="mb-4 flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
              <div>
                <h2 className="text-lg font-semibold text-gray-950">Docker Settings</h2>
                <p className="text-sm text-gray-600">Edit the host Docker daemon configuration. Saves write `daemon.json`; Docker must be restarted on the host before settings apply.</p>
              </div>
              <button type="button" onClick={loadDockerSettings} className="btn-secondary" title="Reload /etc/docker/daemon.json from the Docker host.">Reload</button>
            </div>

            {dockerStatus?.warnings?.length > 0 && (
              <div className="mb-4 rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900">
                {dockerStatus.warnings.map(warning => <div key={warning}>{warning}</div>)}
              </div>
            )}

            <div className="grid gap-4 lg:grid-cols-2">
              <div className="space-y-3">
                <h3 className="text-sm font-semibold uppercase text-gray-500">Runtime</h3>
                <label className="block text-sm text-gray-700" title="Zero-downtime daemon restart: containers keep running while dockerd is restarted (e.g. Docker upgrade or daemon.json edit). Off means every restart stops all containers until they come back up. Does not affect host reboots.">
                  <span className="flex items-center gap-2">
                    <input type="checkbox" checked={dockerForm.live_restore} onChange={e => setDockerForm({ ...dockerForm, live_restore: e.target.checked })} />
                    Live restore
                  </span>
                  <span className="mt-1 block text-xs text-gray-500">Keeps containers running when the Docker daemon restarts (e.g. after upgrade or daemon.json edit). Doesn't apply to host reboots.</span>
                </label>
                <label className="block text-sm text-gray-700" title="Give containers IPv6 addresses in addition to IPv4. You almost always also need a routed IPv6 subnet in Fixed CIDR v6 for this to be useful.">
                  <span className="flex items-center gap-2">
                    <input type="checkbox" checked={dockerForm.ipv6} onChange={e => setDockerForm({ ...dockerForm, ipv6: e.target.checked })} />
                    Enable IPv6
                  </span>
                  <span className="mt-1 block text-xs text-gray-500">Gives containers IPv6 addresses. Also needs a routed IPv6 subnet in Fixed CIDR v6 below.</span>
                </label>
                <Field label="Fixed CIDR v6" title="IPv6 subnet Docker carves container addresses from, e.g. fd00:dead:beef::/48. Leave blank unless your host has real IPv6 connectivity.">
                  <input className="input" value={dockerForm.fixed_cidr_v6} onChange={e => setDockerForm({ ...dockerForm, fixed_cidr_v6: e.target.value })} placeholder="fd00:dead:beef::/48" />
                </Field>
                <Field label="Docker0 bridge IP (bip)" title="Explicitly assigns the default docker0 bridge an IP and subnet, taking docker0 out of default-address-pools reconciliation. Prevents 'all predefined address pools have been fully subnetted' when using tight pool slicing. Pick something outside the default-address-pools range below." hint="Example: 172.30.0.1/24 or 192.168.100.1/24. Leave blank to let Docker pick.">
                  <input className="input" value={dockerForm.bip} onChange={e => setDockerForm({ ...dockerForm, bip: e.target.value })} placeholder="172.30.0.1/24" />
                </Field>
                <Field label="Default address pools" title="Which IPv4 ranges Docker uses when it auto-creates bridge networks (each compose stack usually gets one). Set this to steer Docker away from ranges your VPN or LAN already uses. Simple form: one CIDR per line = one network of that size. Advanced form: base,size = carve many subnets of /size out of the /base pool." hint="Simple: 172.31.0.0/24 = one /24 network. Advanced: 172.31.0.0/18,26 = up to 256 /26 subnets from within the /18. Use whichever is easier. Set the Docker0 bridge IP (bip) above to a different subnet or dockerd may refuse to start.">
                  <textarea className="textarea h-24 font-mono" value={dockerForm.default_address_pools} onChange={e => setDockerForm({ ...dockerForm, default_address_pools: e.target.value })} placeholder="172.31.0.0/24&#10;172.30.0.0/16,24" />
                </Field>
              </div>

              <div className="space-y-3">
                <h3 className="text-sm font-semibold uppercase text-gray-500">Logging</h3>
                <Field label="Log driver" title="How Docker stores stdout/stderr from new containers. json-file (default) writes JSON files on disk — safest and most compatible. local writes smaller binary files with built-in rotation. journald ships logs to systemd's journal.">
                  <select className="input" value={dockerForm.log_driver} onChange={e => setDockerForm({ ...dockerForm, log_driver: e.target.value })}>
                    <option value="json-file">json-file</option>
                    <option value="local">local</option>
                    <option value="journald">journald</option>
                  </select>
                </Field>
                <div className="grid gap-3 sm:grid-cols-2">
                  <Field label="Max log size" title="Maximum size per log file for json-file/local logging, such as 10m, 50m, or 1g.">
                    <input className="input" value={dockerForm.log_max_size} onChange={e => setDockerForm({ ...dockerForm, log_max_size: e.target.value })} />
                  </Field>
                  <Field label="Max log files" title="How many rotated log files Docker keeps per container.">
                    <input className="input" value={dockerForm.log_max_file} onChange={e => setDockerForm({ ...dockerForm, log_max_file: e.target.value })} />
                  </Field>
                </div>
                <Field label="DNS servers" title="One DNS server per line. These become Docker daemon defaults for new containers." hint="One IP per line. Applied to new containers only; existing containers keep their current resolvers.">
                  <textarea className="textarea h-20 font-mono" value={dockerForm.dns} onChange={e => setDockerForm({ ...dockerForm, dns: e.target.value })} placeholder="1.1.1.1&#10;8.8.8.8" />
                </Field>
              </div>

              <div className="space-y-3">
                <h3 className="text-sm font-semibold uppercase text-gray-500">Registries</h3>
                <Field label="Registry mirrors" title="One mirror URL per line. Mirrors can reduce Docker Hub rate limits and speed up pulls. Use trusted HTTPS mirrors." hint="One HTTPS URL per line. Useful for Docker Hub caching mirrors when rate limits are a concern.">
                  <textarea className="textarea h-24 font-mono" value={dockerForm.registry_mirrors} onChange={e => setDockerForm({ ...dockerForm, registry_mirrors: e.target.value })} placeholder="https://mirror.example.com" />
                </Field>
                <Field label="Insecure registries" title="One host[:port] or CIDR per line. Docker skips TLS verification for these registries. Use only on trusted private networks." hint="One host[:port] or CIDR per line. Only for trusted internal registries.">
                  <textarea className="textarea h-24 font-mono" value={dockerForm.insecure_registries} onChange={e => setDockerForm({ ...dockerForm, insecure_registries: e.target.value })} placeholder="registry.local:5000" />
                </Field>
              </div>

              <div className="space-y-3">
                <h3 className="text-sm font-semibold uppercase text-gray-500">Remote API</h3>
                <label className="flex items-center gap-2 text-sm text-gray-700" title="Adds a tcp:// host entry to Docker daemon.json. This can expose root-equivalent Docker API access; bind to VPN/private IP and require firewall/TLS controls.">
                  <input type="checkbox" checked={dockerForm.expose_tcp} onChange={e => setDockerForm({ ...dockerForm, expose_tcp: e.target.checked })} />
                  Expose Docker TCP API
                </label>
                <div className="grid gap-3 sm:grid-cols-2">
                  <Field label="Bind address" title="IP address Docker should listen on for the TCP API. 127.0.0.1 is safest; 0.0.0.0 exposes it on every interface.">
                    <input className="input" value={dockerForm.tcp_bind} onChange={e => setDockerForm({ ...dockerForm, tcp_bind: e.target.value })} />
                  </Field>
                  <Field label="TCP port" title="2376 is normally used with TLS. Avoid unauthenticated 2375 except on isolated networks.">
                    <input className="input" value={dockerForm.tcp_port} onChange={e => setDockerForm({ ...dockerForm, tcp_port: e.target.value })} />
                  </Field>
                </div>
                <div className="rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-900" title="Anyone who can reach an unauthenticated Docker TCP API can control containers, mount host files, and effectively root the host.">
                  Remote Docker socket access is root-equivalent. Prefer SSH access, VPN-only binding, firewall allowlists, and TLS on port 2376.
                </div>
              </div>
            </div>
          </div>

          <div className="section-panel space-y-3">
            <div>
              <h3 className="text-sm font-semibold uppercase text-gray-500">Advanced daemon.json</h3>
              <p className="text-sm text-gray-600">This JSON is merged with the structured fields above on save. Keep valid Docker daemon JSON.</p>
            </div>
            <textarea className="textarea h-80 font-mono" value={dockerRaw} onChange={e => setDockerRaw(e.target.value)} title="Raw daemon.json. Invalid JSON is rejected before saving." />
            <div className="flex flex-wrap items-center gap-3">
              <button type="submit" className="btn-primary" title="Validate and save daemon.json on the Docker host.">Save Docker Settings</button>
              {dockerStatus?.restart_required && <Badge tone="red">restart Docker to apply</Badge>}
              {dockerStatus?.backup && <span className="text-xs text-gray-500">Backup: {dockerStatus.backup}</span>}
            </div>
          </div>
        </form>
      )}

      {admin && activeTab === 'backups' && (
        <div className="section-panel">
          <div className="mb-4 flex flex-col gap-1">
            <h2 className="text-lg font-semibold text-gray-950">Backup Endpoints</h2>
            <p className="text-sm text-gray-600">Configure local, mounted, FTP, SFTP, and S3 destinations. CIFS and NFS should be mounted on the host and exposed to the server container as paths.</p>
          </div>
          <div className="grid gap-6 xl:grid-cols-[1fr_420px]">
            <div className="overflow-x-auto">
              <table className="w-full min-w-[760px] text-left text-sm">
                <thead><tr className="border-b border-gray-200 text-xs uppercase text-gray-500"><th className="py-2">Name</th><th>Type</th><th>Path / Target</th><th>Status</th><th className="text-right">Actions</th></tr></thead>
                <tbody>
                  {destinationList.map(destination => (
                    <tr key={destination.id} className="border-b border-gray-100 align-top">
                      <td className="py-2 font-medium">{destination.name}</td>
                      <td>{destination.type}</td>
                      <td className="font-mono text-xs text-gray-600">{destinationSummary(destination)}</td>
                      <td>{destination.enabled ? 'enabled' : 'disabled'}{destination.has_secret ? ' - secret saved' : ''}</td>
                      <td className="space-x-2 text-right">
                        <button onClick={() => testDestination(destination)} className="mini-button" title="Create and remove a small test file or run a remote upload test with this endpoint.">Test</button>
                        <button onClick={() => editDestination(destination)} className="mini-button" title="Load this endpoint into the edit form. Saved secrets are kept unless replaced.">Edit</button>
                        <button onClick={() => deleteDestination(destination)} className="mini-danger" title="Delete this backup endpoint configuration. Existing backup files are not removed.">Delete</button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
              {destinationList.length === 0 && <div className="py-6 text-sm text-gray-500">No backup endpoints configured.</div>}
            </div>

            <form onSubmit={saveDestination} className="space-y-3">
              <h3 className="text-sm font-semibold uppercase text-gray-500">{destinationForm.id ? 'Edit Endpoint' : 'Add Endpoint'}</h3>
              <input className="input" placeholder="endpoint name" value={destinationForm.name} onChange={e => setDestinationForm({ ...destinationForm, name: e.target.value })} required title="Friendly name shown in backup dropdowns." />
              <select className="input" value={destinationForm.type} onChange={e => setDestinationForm({ ...destinationForm, type: e.target.value })} title="Choose the backup endpoint type.">
                {DESTINATION_TYPES.map(type => <option key={type.value} value={type.value}>{type.label}</option>)}
              </select>
              <label className="flex items-center gap-2 text-sm text-gray-700" title="Disabled endpoints stay saved but cannot be selected for new backups.">
                <input type="checkbox" checked={destinationForm.enabled} onChange={e => setDestinationForm({ ...destinationForm, enabled: e.target.checked })} />
                Enabled
              </label>

              {['local', 'mount', 'cifs', 'nfs'].includes(destinationForm.type) && (
                <input className="input" placeholder="/mnt/backups/compose-manager" value={destinationForm.config.path} onChange={e => setDestinationConfig('path', e.target.value)} required title="Absolute path inside the server container. Bind mount host CIFS/NFS paths into the server container before using them here." />
              )}

              {['ftp', 'sftp'].includes(destinationForm.type) && (
                <div className="grid gap-2">
                  <input className="input" placeholder="host" value={destinationForm.config.host} onChange={e => setDestinationConfig('host', e.target.value)} required title="FTP/SFTP host name or IP." />
                  <div className="grid gap-2 sm:grid-cols-2">
                    <input className="input" placeholder="port" value={destinationForm.config.port} onChange={e => setDestinationConfig('port', e.target.value)} title="Optional port. Defaults are handled by rclone." />
                    <input className="input" placeholder="username" value={destinationForm.config.username} onChange={e => setDestinationConfig('username', e.target.value)} required title="FTP/SFTP username." />
                  </div>
                  <input className="input" type="password" placeholder={destinationForm.id ? 'password, leave blank to keep saved' : 'password'} value={destinationForm.secrets.password} onChange={e => setDestinationSecret('password', e.target.value)} title="FTP/SFTP password. Leave blank during edits to keep the saved secret." />
                  {destinationForm.type === 'sftp' && <input className="input" placeholder="/state/keys/backup_id_ed25519" value={destinationForm.config.key_file} onChange={e => setDestinationConfig('key_file', e.target.value)} title="Optional SFTP private key path inside the server container." />}
                  <input className="input" placeholder="remote/path" value={destinationForm.config.remote_path} onChange={e => setDestinationConfig('remote_path', e.target.value)} title="Remote directory for uploaded backup archives." />
                </div>
              )}

              {destinationForm.type === 's3' && (
                <div className="grid gap-2">
                  <input className="input" placeholder="bucket" value={destinationForm.config.bucket} onChange={e => setDestinationConfig('bucket', e.target.value)} required title="S3 bucket name." />
                  <input className="input" placeholder="prefix, e.g. compose-manager/docker01" value={destinationForm.config.prefix} onChange={e => setDestinationConfig('prefix', e.target.value)} title="Optional object prefix inside the bucket." />
                  <input className="input" placeholder="endpoint, e.g. https://s3.amazonaws.com" value={destinationForm.config.endpoint} onChange={e => setDestinationConfig('endpoint', e.target.value)} title="Optional endpoint for S3-compatible storage." />
                  <div className="grid gap-2 sm:grid-cols-2">
                    <input className="input" placeholder="region" value={destinationForm.config.region} onChange={e => setDestinationConfig('region', e.target.value)} title="Optional region." />
                    <input className="input" placeholder="provider" value={destinationForm.config.provider} onChange={e => setDestinationConfig('provider', e.target.value)} title="rclone S3 provider. Use Other for generic S3-compatible endpoints." />
                  </div>
                  <input className="input" type="password" placeholder={destinationForm.id ? 'access key, leave blank to keep saved' : 'access key'} value={destinationForm.secrets.access_key_id} onChange={e => setDestinationSecret('access_key_id', e.target.value)} title="S3 access key ID. Leave blank during edits to keep the saved value." />
                  <input className="input" type="password" placeholder={destinationForm.id ? 'secret key, leave blank to keep saved' : 'secret key'} value={destinationForm.secrets.secret_access_key} onChange={e => setDestinationSecret('secret_access_key', e.target.value)} title="S3 secret access key. Leave blank during edits to keep the saved value." />
                </div>
              )}

              <div className="flex gap-2">
                <button className="btn-primary" title="Save this backup endpoint in MariaDB.">Save Endpoint</button>
                {destinationForm.id > 0 && <button type="button" onClick={() => setDestinationForm(emptyDestinationForm)} className="btn-secondary" title="Clear the edit form.">Cancel</button>}
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}

function Field({ label, title, hint, children }) {
  return (
    <label className="block text-sm" title={title}>
      <span className="mb-1 block font-medium text-gray-700">{label}</span>
      {children}
      {hint && <span className="mt-1 block text-xs text-gray-500">{hint}</span>}
    </label>
  );
}

function Badge({ tone = 'gray', children }) {
  const tones = {
    gray: 'bg-gray-100 text-gray-700',
    green: 'bg-green-100 text-green-800',
    red: 'bg-red-100 text-red-800',
    blue: 'bg-blue-100 text-blue-800',
  };
  return <span className={`inline-flex rounded px-2 py-0.5 text-xs font-medium ${tones[tone] || tones.gray}`}>{children}</span>;
}

function formatDate(value) {
  if (!value) return 'none';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return 'none';
  return date.toLocaleString();
}

function formFromDockerConfig(config) {
  const logOpts = config['log-opts'] || {};
  const hosts = Array.isArray(config.hosts) ? config.hosts : [];
  const tcpHost = hosts.find(host => typeof host === 'string' && host.startsWith('tcp://')) || '';
  const tcpMatch = tcpHost.match(/^tcp:\/\/([^:]+):(\d+)$/);
  return {
    live_restore: Boolean(config['live-restore']),
    log_driver: String(config['log-driver'] || 'json-file'),
    log_max_size: String(logOpts['max-size'] || '10m'),
    log_max_file: String(logOpts['max-file'] || '3'),
    dns: arrayToLines(config.dns),
    registry_mirrors: arrayToLines(config['registry-mirrors']),
    insecure_registries: arrayToLines(config['insecure-registries']),
    default_address_pools: poolsToLines(config['default-address-pools']),
    bip: String(config.bip || ''),
    ipv6: Boolean(config.ipv6),
    fixed_cidr_v6: String(config['fixed-cidr-v6'] || ''),
    expose_tcp: Boolean(tcpHost),
    tcp_bind: tcpMatch ? tcpMatch[1] : '127.0.0.1',
    tcp_port: tcpMatch ? tcpMatch[2] : '2376',
  };
}

function buildDockerConfig(raw, form) {
  let config = {};
  if (String(raw || '').trim()) {
    config = JSON.parse(raw);
  }
  if (!config || Array.isArray(config) || typeof config !== 'object') {
    throw new Error('daemon.json must be a JSON object');
  }
  config['live-restore'] = Boolean(form.live_restore);
  config.ipv6 = Boolean(form.ipv6);
  setOrDelete(config, 'fixed-cidr-v6', form.fixed_cidr_v6);
  setOrDelete(config, 'dns', linesToArray(form.dns));
  setOrDelete(config, 'registry-mirrors', linesToArray(form.registry_mirrors));
  setOrDelete(config, 'insecure-registries', linesToArray(form.insecure_registries));
  setOrDelete(config, 'default-address-pools', linesToPools(form.default_address_pools));
  setOrDelete(config, 'bip', String(form.bip || '').trim());

  if (form.log_driver) {
    config['log-driver'] = form.log_driver;
    config['log-opts'] = {
      ...(config['log-opts'] || {}),
      'max-size': form.log_max_size || '10m',
      'max-file': form.log_max_file || '3',
    };
  }

  const hosts = Array.isArray(config.hosts) ? config.hosts.filter(host => typeof host !== 'string' || !host.startsWith('tcp://')) : [];
  if (form.expose_tcp) {
    if (!hosts.some(host => typeof host === 'string' && (host.startsWith('unix://') || host === 'fd://'))) {
      hosts.unshift('unix:///var/run/docker.sock');
    }
    hosts.push(`tcp://${form.tcp_bind || '127.0.0.1'}:${form.tcp_port || '2376'}`);
  }
  setOrDelete(config, 'hosts', hosts);
  return config;
}

function setOrDelete(object, key, value) {
  if (Array.isArray(value) && value.length === 0) {
    delete object[key];
    return;
  }
  if (typeof value === 'string' && value.trim() === '') {
    delete object[key];
    return;
  }
  object[key] = value;
}

function arrayToLines(value) {
  return Array.isArray(value) ? value.join('\n') : '';
}

function linesToArray(value) {
  return String(value || '').split(/\r?\n/).map(item => item.trim()).filter(Boolean);
}

function poolsToLines(value) {
  if (!Array.isArray(value)) return '';
  return value.map(pool => `${pool.base || ''},${pool.size || ''}`).filter(line => line !== ',').join('\n');
}

function linesToPools(value) {
  const lines = linesToArray(value);
  return lines.map((line, index) => {
    const parts = line.split(',').map(part => part.trim());
    const base = parts[0] || '';
    let sizeText = parts[1] || '';
    if (!base) throw new Error(`Default address pool line ${index + 1} is empty.`);
    if (!/^\d+\.\d+\.\d+\.\d+\/\d+$/.test(base)) {
      throw new Error(`Default address pool line ${index + 1}: enter a CIDR like 172.30.0.0/16 (got "${base}").`);
    }
    const baseMask = Number(base.split('/')[1]);
    // Single-CIDR form (no comma): treat the CIDR itself as a one-subnet pool.
    // Advanced form (base,size): base is the outer pool and size is the subnet
    // size Docker slices out. 172.30.0.0/16,24 gives 256 /24 subnets from a /16.
    if (!sizeText) {
      sizeText = String(baseMask);
    }
    const size = Number(sizeText);
    if (!Number.isInteger(size) || size < 1 || size > 32) {
      throw new Error(`Default address pool line ${index + 1}: size must be an integer 1-32 (got "${sizeText}").`);
    }
    if (size < baseMask) {
      throw new Error(`Default address pool line ${index + 1}: subnet size /${size} must be greater than or equal to the base mask /${baseMask}.`);
    }
    return { base, size };
  });
}

function pruneMap(value) {
  return Object.fromEntries(Object.entries(value || {}).filter(([, v]) => String(v || '').trim() !== ''));
}

function destinationSummary(destination) {
  const config = destination.config || {};
  if (['local', 'mount', 'cifs', 'nfs'].includes(destination.type)) return config.path || '';
  if (['ftp', 'sftp'].includes(destination.type)) {
    const port = config.port ? `:${config.port}` : '';
    const path = config.remote_path ? `/${config.remote_path}` : '';
    return `${config.host || ''}${port}${path}`;
  }
  if (destination.type === 's3') {
    const prefix = config.prefix ? `/${config.prefix}` : '';
    return `${config.bucket || ''}${prefix}`;
  }
  return '';
}
