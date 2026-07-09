import { useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { auth, users, backup, projects, agents, schedules, registries, dockerSettings, ssl as sslApi, firewall as firewallApi, totp as totpApi, envSettings, proxy as proxyApi } from '../api/client';
import { getThemePreference, setThemePreference } from '../theme';
import { buildDockerConfig, formFromDockerConfig, pruneMap } from '../utils/dockerSettings';

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
    private_key: '',
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
  cadence: 'daily',
  time_of_day: '03:00',
  day_of_week: 6,
  day_of_month: 1,
  interval_minutes: 1440,
  timeout_seconds: 300,
};

const WEEKDAYS = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'];

const emptyAgentForm = { id: 0, name: '', base_url: '', mode: 'callback', token: '', enabled: true };

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
  const [changeMyPasswordForm, setChangeMyPasswordForm] = useState({ current: '', next: '', confirm: '' });
  const [destinationForm, setDestinationForm] = useState(emptyDestinationForm);
  const [destinationFormOpen, setDestinationFormOpen] = useState(false);
  const [destinationPublicKey, setDestinationPublicKey] = useState('');
  const [scheduleForm, setScheduleForm] = useState(emptyScheduleForm);
  const [agentForm, setAgentForm] = useState(emptyAgentForm);
  const [agentFormOpen, setAgentFormOpen] = useState(false);
  const [registryForm, setRegistryForm] = useState({ registry: '', username: '', password: '' });
  const [dockerHubForm, setDockerHubForm] = useState({ username: '', password: '' });
  const [savedRegistries, setSavedRegistries] = useState([]);
  const [dockerForm, setDockerForm] = useState(emptyDockerForm);
  const [dockerRaw, setDockerRaw] = useState('{}\n');
  const [dockerStatus, setDockerStatus] = useState(null);
  const [sslInfo, setSslInfo] = useState(null);
  const [sslLoading, setSslLoading] = useState(false);
  const [sslSelfSignedForm, setSslSelfSignedForm] = useState({ cn: '', extra_sans: '', days: 3650 });
  const [sslLeForm, setSslLeForm] = useState({ domain: '', email: '', staging: false });
  const [theme, setTheme] = useState(getThemePreference());
  const [controllerUrl, setControllerUrl] = useState(() => {
    try {
      const saved = localStorage.getItem('cm_controller_url');
      if (saved) return saved;
    } catch {}
    return typeof window !== 'undefined' ? window.location.origin : '';
  });
  const [copiedKey, setCopiedKey] = useState('');
  const [setupMode, setSetupMode] = useState(() => {
    try { return localStorage.getItem('cm_agent_setup_mode') || 'callback'; } catch { return 'callback'; }
  });
  const [firewallStatus, setFirewallStatus] = useState(null);
  const [firewallAllow, setFirewallAllow] = useState([]);
  const [firewallDeny, setFirewallDeny] = useState([]);
  const [firewallLog, setFirewallLog] = useState('');
  const [firewallLogLines, setFirewallLogLines] = useState(200);
  const [firewallConfigName, setFirewallConfigName] = useState('csf.conf');
  const [firewallConfigContent, setFirewallConfigContent] = useState('');
  const [firewallConfigDirty, setFirewallConfigDirty] = useState(false);
  const [firewallIPForm, setFirewallIPForm] = useState({ ip: '', comment: '' });
  const [firewallMyIP, setFirewallMyIP] = useState('');
  const [firewallBusy, setFirewallBusy] = useState(false);
  const [firewallConf, setFirewallConf] = useState(null);
  const [firewallConfForm, setFirewallConfForm] = useState({});
  const [generalSettings, setGeneralSettings] = useState(null);
  const [generalForm, setGeneralForm] = useState({});
  const [rolledAPIKey, setRolledAPIKey] = useState('');
  const [npmStatus, setNpmStatus] = useState(null);
  const [npmForm, setNpmForm] = useState({ url: '', email: 'admin@example.com', password: '' });
  const [npmHosts, setNpmHosts] = useState([]);
  const [npmSuggestions, setNpmSuggestions] = useState([]);
  const [npmHostForm, setNpmHostForm] = useState({ domain: '', forward_host: '', forward_port: '', forward_scheme: 'http' });
  const [totpEnrolling, setTotpEnrolling] = useState(false);
  const [totpEnrollData, setTotpEnrollData] = useState(null);
  const [totpVerifyCode, setTotpVerifyCode] = useState('');
  const [totpDisablePassword, setTotpDisablePassword] = useState('');

  const admin = me?.role === 'admin';
  const tabs = useMemo(() => {
    const base = [{ key: 'account', label: 'Account', title: 'Theme, current user, and sign out.' }];
    if (!admin) return base;
    return [
      ...base,
      { key: 'general', label: 'General', title: 'Ports, cache, API key, and host settings.' },
      { key: 'users', label: 'Users', title: 'Manage dashboard users and passwords.' },
      { key: 'agents', label: 'Agents', title: 'Register and edit remote Stack Manager agents.' },
      { key: 'schedules', label: 'Scheduled Updates', title: 'Schedule local or agent project updates.' },
      { key: 'registries', label: 'Registry Logins', title: 'Docker Hub account and private registry logins.' },
      { key: 'docker', label: 'Docker Settings', title: 'Edit Docker daemon.json settings for this host.' },
      { key: 'ssl', label: 'SSL / TLS', title: 'View, regenerate, or switch the TLS cert (self-signed or Let’s Encrypt).' },
      { key: 'backups', label: 'Backup Endpoints', title: 'Configure local and remote backup destinations.' },
      { key: 'firewall', label: 'Firewall', title: 'ConfigServer Firewall (csf/lfd) install, monitor, and IP management.' },
      { key: 'proxy', label: 'Reverse Proxy', title: 'Set up Nginx Proxy Manager for domain-based HTTPS and proxying.' },
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

  useEffect(() => {
    if (admin && activeTab === 'ssl') loadSslInfo();
  }, [admin, activeTab]);

  useEffect(() => {
    if (admin && activeTab === 'registries') loadSavedRegistries();
  }, [admin, activeTab]);

  useEffect(() => {
    if (admin && activeTab === 'proxy') loadProxyStatus();
  }, [admin, activeTab]);

  const loadProxyStatus = async () => {
    try {
      const res = await proxyApi.status();
      setNpmStatus(res.data);
      if (res.data?.connected) {
        const [hostsRes, sugRes] = await Promise.all([proxyApi.listHosts().catch(() => ({ data: [] })), proxyApi.suggestions().catch(() => ({ data: [] }))]);
        setNpmHosts(Array.isArray(hostsRes.data) ? hostsRes.data : hostsRes.data ? [hostsRes.data] : []);
        setNpmSuggestions(sugRes.data || []);
      }
    } catch (err) { showError(err); }
  };

  const connectNpm = async () => {
    try {
      await proxyApi.configure(npmForm.url, npmForm.email, npmForm.password);
      showMessage('Connected to Nginx Proxy Manager.');
      setNpmForm({ ...npmForm, password: '' });
      loadProxyStatus();
    } catch (err) { showError(err); }
  };

  const createProxyHost = async () => {
    const domain = npmHostForm.domain.trim();
    if (!domain || !npmHostForm.forward_host || !npmHostForm.forward_port) {
      showError(new Error('Domain, forward host, and forward port are required.')); return;
    }
    try {
      await proxyApi.createHost({
        domain_names: [domain],
        forward_scheme: npmHostForm.forward_scheme || 'http',
        forward_host: npmHostForm.forward_host.trim(),
        forward_port: Number(npmHostForm.forward_port),
        block_exploits: true,
        allow_websocket_upgrade: true,
        access_list_id: 0,
        certificate_id: 0,
        meta: { letsencrypt_agree: false, dns_challenge: false },
        advanced_config: '',
        locations: [],
        caching_enabled: false,
        ssl_forced: false,
        http2_support: false,
        hsts_enabled: false,
        hsts_subdomains: false,
      });
      showMessage(`Proxy host ${domain} created.`);
      setNpmHostForm({ domain: '', forward_host: '', forward_port: '', forward_scheme: 'http' });
      loadProxyStatus();
    } catch (err) { showError(err); }
  };

  const deleteProxyHost = async (id, domains) => {
    if (!window.confirm(`Delete proxy host ${domains}?`)) return;
    try {
      await proxyApi.deleteHost(id);
      showMessage(`Deleted proxy host ${domains}.`);
      loadProxyStatus();
    } catch (err) { showError(err); }
  };

  useEffect(() => {
    if (admin && activeTab === 'general') loadGeneralSettings();
  }, [admin, activeTab]);

  const loadGeneralSettings = async () => {
    try {
      const res = await envSettings.get();
      setGeneralSettings(res.data);
      setGeneralForm(res.data);
      setRolledAPIKey('');
    } catch (err) { showError(err); }
  };

  const saveGeneralSettings = async () => {
    try {
      const res = await envSettings.save(generalForm);
      showMessage('Settings saved. ' + (res.data?.warnings?.join(' ') || ''));
      loadGeneralSettings();
    } catch (err) { showError(err); }
  };

  const rollAPIKey = async () => {
    if (!window.confirm('Generate a new API key? Existing API-key scripts will need the new value. Session-based logins are not affected.')) return;
    try {
      const res = await envSettings.rollAPIKey();
      setRolledAPIKey(res.data?.api_key || '');
      showMessage(res.data?.warning || 'API key rolled.');
    } catch (err) { showError(err); }
  };

  useEffect(() => {
    if (admin && activeTab === 'firewall') loadFirewall();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [admin, activeTab]);

  useEffect(() => {
    if (admin && activeTab === 'firewall' && firewallStatus?.installed) {
      loadFirewallConfig(firewallConfigName);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [firewallConfigName, firewallStatus?.installed]);

  const loadFirewall = async () => {
    try {
      const [statusRes, myIpRes] = await Promise.all([firewallApi.status(), firewallApi.clientIP().catch(() => ({ data: {} }))]);
      setFirewallStatus(statusRes.data);
      setFirewallMyIP(myIpRes.data?.ip || '');
      if (statusRes.data?.installed) {
        await Promise.all([loadFirewallLists(), loadFirewallLog(firewallLogLines), loadFirewallConf()]);
      }
    } catch (err) {
      showError(err);
    }
  };

  const loadFirewallLists = async () => {
    try {
      const [allowRes, denyRes] = await Promise.all([firewallApi.listAllow(), firewallApi.listDeny()]);
      setFirewallAllow(allowRes.data?.entries || []);
      setFirewallDeny(denyRes.data?.entries || []);
    } catch (err) {
      showError(err);
    }
  };

  const loadFirewallLog = async (lines) => {
    try {
      const res = await firewallApi.tailLog(lines);
      setFirewallLog(res.data?.content || '');
    } catch (err) {
      showError(err);
    }
  };

  const loadFirewallConfig = async (name) => {
    try {
      const res = await firewallApi.readConfig(name);
      setFirewallConfigContent(res.data?.content || '');
      setFirewallConfigDirty(false);
    } catch (err) {
      showError(err);
    }
  };

  const installFirewall = async () => {
    if (!window.confirm('Install ConfigServer Firewall from Black-HOST/csf on this host? This changes host firewall state and may briefly interrupt connectivity.')) return;
    setFirewallBusy(true);
    try {
      const res = await firewallApi.install();
      showMessage(`Firewall install: ${res.data?.output?.trim() || 'ok'}`);
      await loadFirewall();
    } catch (err) { showError(err); } finally { setFirewallBusy(false); }
  };

  const uninstallFirewall = async () => {
    const typed = window.prompt('Type UNINSTALL to remove ConfigServer Firewall from this host.');
    if ((typed || '').toUpperCase() !== 'UNINSTALL') return;
    setFirewallBusy(true);
    try {
      const res = await firewallApi.uninstall();
      showMessage(`Firewall uninstall: ${res.data?.output?.trim() || 'ok'}`);
      await loadFirewall();
    } catch (err) { showError(err); } finally { setFirewallBusy(false); }
  };

  const restartFirewall = async () => {
    setFirewallBusy(true);
    try {
      const res = await firewallApi.restart();
      showMessage(`csf -r: ${res.data?.output?.trim() || 'ok'}`);
      await loadFirewall();
    } catch (err) { showError(err); } finally { setFirewallBusy(false); }
  };

  const reloadLFD = async () => {
    setFirewallBusy(true);
    try {
      const res = await firewallApi.reloadLFD();
      showMessage(`lfd reload: ${res.data?.output?.trim() || 'ok'}`);
    } catch (err) { showError(err); } finally { setFirewallBusy(false); }
  };

  const submitFirewallIP = async (action) => {
    const ip = firewallIPForm.ip.trim();
    const comment = firewallIPForm.comment.trim();
    if (!ip || !comment) { showError(new Error('IP and comment are required')); return; }
    setFirewallBusy(true);
    try {
      if (action === 'allow') await firewallApi.allowIP(ip, comment);
      else await firewallApi.denyIP(ip, comment);
      showMessage(`${action} ${ip}: ok`);
      setFirewallIPForm({ ip: '', comment: '' });
      await loadFirewallLists();
    } catch (err) { showError(err); } finally { setFirewallBusy(false); }
  };

  const removeFirewallIP = async (ip) => {
    if (!window.confirm(`Remove ${ip} from allow and deny lists?`)) return;
    setFirewallBusy(true);
    try {
      await firewallApi.removeIP(ip);
      showMessage(`removed ${ip}`);
      await loadFirewallLists();
    } catch (err) { showError(err); } finally { setFirewallBusy(false); }
  };

  const allowMyIP = async () => {
    setFirewallBusy(true);
    try {
      const res = await firewallApi.allowMyIP();
      showMessage(`allowed ${res.data?.ip}: ${res.data?.output?.trim() || 'ok'}`);
      await loadFirewallLists();
    } catch (err) { showError(err); } finally { setFirewallBusy(false); }
  };

  const loadFirewallConf = async () => {
    try {
      const res = await firewallApi.confSettings();
      setFirewallConf(res.data);
      setFirewallConfForm(res.data);
    } catch {}
  };

  const saveFirewallConf = async () => {
    setFirewallBusy(true);
    try {
      const res = await firewallApi.saveConfSettings(firewallConfForm);
      showMessage(res.data?.hint || 'Saved.');
      setFirewallConf(firewallConfForm);
    } catch (err) { showError(err); } finally { setFirewallBusy(false); }
  };

  const saveFirewallConfig = async () => {
    if (!window.confirm(`Overwrite /etc/csf/${firewallConfigName}? A timestamped backup is kept under /var/backups/stack-manager-csf/.`)) return;
    setFirewallBusy(true);
    try {
      const res = await firewallApi.writeConfig(firewallConfigName, firewallConfigContent);
      showMessage(res.data?.output?.trim() || 'saved');
      setFirewallConfigDirty(false);
    } catch (err) { showError(err); } finally { setFirewallBusy(false); }
  };

  const loadSavedRegistries = async () => {
    try {
      const res = await registries.list();
      setSavedRegistries(res.data || []);
    } catch (err) {
      showError(err);
    }
  };

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

  const changeMyPassword = async (event) => {
    event.preventDefault();
    if (changeMyPasswordForm.next !== changeMyPasswordForm.confirm) {
      showError(new Error('New password and confirmation do not match.'));
      return;
    }
    if (changeMyPasswordForm.next.length < 12) {
      showError(new Error('New password must be at least 12 characters.'));
      return;
    }
    try {
      await auth.changePassword(changeMyPasswordForm.current, changeMyPasswordForm.next);
      showMessage('Password changed. Use the new password next time you sign in.');
      setChangeMyPasswordForm({ current: '', next: '', confirm: '' });
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
    setAgentForm({ id: agent.id, name: agent.name, base_url: agent.base_url || '', mode: agent.mode || (agent.base_url ? 'inbound' : 'callback'), token: '', enabled: Boolean(agent.enabled) });
    setAgentFormOpen(true);
  };

  const closeAgentForm = () => {
    setAgentFormOpen(false);
    setAgentForm(emptyAgentForm);
  };

  const saveAgent = async (event) => {
    event.preventDefault();
    try {
      const res = await agents.save(agentForm);
      showMessage(`Saved agent ${res.data.name}`);
      closeAgentForm();
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
      cadence: scheduleForm.cadence,
      time_of_day: scheduleForm.time_of_day,
      day_of_week: Number(scheduleForm.day_of_week),
      day_of_month: Number(scheduleForm.day_of_month),
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
      cadence: schedule.cadence || 'interval',
      time_of_day: schedule.time_of_day || '03:00',
      day_of_week: schedule.day_of_week ?? 6,
      day_of_month: schedule.day_of_month ?? 1,
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
      loadSavedRegistries();
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
      loadSavedRegistries();
    } catch (err) {
      showError(err);
    }
  };

  const editSavedRegistry = (login) => {
    if (login.registry === 'https://index.docker.io/v1/') {
      setDockerHubForm({ username: login.username, password: '' });
    } else {
      setRegistryForm({ registry: login.registry, username: login.username, password: '' });
    }
    showMessage(`Loaded ${login.display} into the form. Enter a new password/token and click Login to update.`);
  };

  const deleteSavedRegistry = async (login) => {
    if (!window.confirm(`Delete saved login for ${login.display}?`)) return;
    try {
      await registries.delete(login.registry);
      showMessage(`Removed saved login for ${login.display}`);
      loadSavedRegistries();
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

  const loadSslInfo = async () => {
    setSslLoading(true);
    try {
      const res = await sslApi.get();
      setSslInfo(res.data || null);
      if (res.data?.cn && !sslSelfSignedForm.cn) {
        setSslSelfSignedForm(f => ({ ...f, cn: res.data.cn }));
      }
      if (res.data?.domain && !sslLeForm.domain) {
        setSslLeForm(f => ({ ...f, domain: res.data.domain }));
      }
    } catch (err) {
      showError(err);
    } finally {
      setSslLoading(false);
    }
  };

  const regenerateSelfSigned = async (event) => {
    event.preventDefault();
    const cn = sslSelfSignedForm.cn.trim();
    if (!cn) { showError(new Error('Common name is required')); return; }
    if (!window.confirm(`Regenerate self-signed cert for ${cn}? nginx will reload in place.`)) return;
    try {
      const res = await sslApi.regenerateSelfSigned({
        cn,
        extra_sans: sslSelfSignedForm.extra_sans.split(',').map(s => s.trim()).filter(Boolean),
        days: Number(sslSelfSignedForm.days) || 3650,
      });
      const data = res.data || {};
      if (data.cert) setSslInfo(data.cert);
      const warn = data.reload_warning ? ` (${data.reload_warning})` : '';
      showMessage(`Regenerated self-signed cert${warn}.`);
    } catch (err) {
      showError(err);
    }
  };

  const enableLetsEncrypt = async (event) => {
    event.preventDefault();
    const domain = sslLeForm.domain.trim();
    const email = sslLeForm.email.trim();
    if (!domain || !email) { showError(new Error('Domain and email are required')); return; }
    if (!window.confirm(`Request a ${sslLeForm.staging ? 'staging' : 'production'} Let's Encrypt cert for ${domain}? This requires WEB_HTTP_PORT=80 and the box reachable on that port from the internet.`)) return;
    try {
      const res = await sslApi.enableLetsEncrypt({ domain, email, staging: sslLeForm.staging });
      const data = res.data || {};
      if (data.cert) setSslInfo(data.cert);
      const warn = data.reload_warning ? ` (${data.reload_warning})` : '';
      showMessage(`Let's Encrypt cert issued for ${domain}${warn}.`);
    } catch (err) {
      showError(err);
    }
  };

  const renewLetsEncrypt = async () => {
    if (!window.confirm('Attempt to renew the Let’s Encrypt cert now?')) return;
    try {
      const res = await sslApi.renewLetsEncrypt();
      const data = res.data || {};
      if (data.cert) setSslInfo(data.cert);
      showMessage('Renewal request completed. Check cert expiry above.');
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

  const editDestination = async (destination) => {
    setDestinationForm({
      ...emptyDestinationForm,
      id: destination.id,
      name: destination.name,
      type: destination.type,
      enabled: Boolean(destination.enabled),
      config: { ...emptyDestinationForm.config, ...(destination.config || {}) },
      secrets: { ...emptyDestinationForm.secrets },
    });
    setDestinationPublicKey('');
    setDestinationFormOpen(true);
    // Try to fetch the saved public key (SFTP only) so an edit can show
    // the same key the operator has to authorize on the remote side.
    if (destination.type === 'sftp') {
      try {
        const res = await backup.destinationPublicKey(destination.id);
        setDestinationPublicKey(res.data?.public_key || '');
      } catch { /* no key saved is fine */ }
    }
  };

  const closeDestinationForm = () => {
    setDestinationFormOpen(false);
    setDestinationForm(emptyDestinationForm);
    setDestinationPublicKey('');
  };

  const generateSSHKey = async () => {
    try {
      const res = await backup.generateSSHKey();
      const priv = res.data?.private_key || '';
      const pub = res.data?.public_key || '';
      setDestinationForm(current => ({
        ...current,
        secrets: { ...current.secrets, private_key: priv },
      }));
      setDestinationPublicKey(pub);
      showMessage('Ed25519 key generated. Save the endpoint, then copy the public key to the SFTP server.');
    } catch (err) {
      showError(err);
    }
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
      closeDestinationForm();
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

  const updateControllerUrl = (value) => {
    setControllerUrl(value);
    try { localStorage.setItem('cm_controller_url', value); } catch {}
  };

  const updateSetupMode = (value) => {
    setSetupMode(value);
    try { localStorage.setItem('cm_agent_setup_mode', value); } catch {}
  };

  const copyToClipboard = async (key, text) => {
    try {
      if (navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(text);
      } else {
        // Fallback for insecure contexts (http://) where clipboard API is blocked.
        const ta = document.createElement('textarea');
        ta.value = text;
        ta.style.position = 'fixed';
        ta.style.left = '-1000px';
        document.body.appendChild(ta);
        ta.focus();
        ta.select();
        document.execCommand('copy');
        document.body.removeChild(ta);
      }
      setCopiedKey(key);
      setTimeout(() => setCopiedKey(current => (current === key ? '' : current)), 1500);
    } catch (err) {
      showError(err);
    }
  };

  const agentControllerHost = (controllerUrl || '').trim() || (typeof window !== 'undefined' ? window.location.origin : 'https://your-controller:8993');

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
        <div className="space-y-4">
          <div className="section-panel">
            <div className="grid gap-4 md:grid-cols-[1fr_300px] md:items-end">
              <div>
                <h2 className="text-lg font-semibold text-gray-950">Account</h2>
                <p className="mt-1 text-sm text-gray-600">
                  Signed in as <span className="font-mono">{me?.username || '…'}</span>{me?.role && <> · role <span className="font-mono">{me.role}</span></>}. Theme preference is saved in this browser.
                </p>
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

          <form onSubmit={changeMyPassword} className="section-panel space-y-3">
            <div>
              <h2 className="text-lg font-semibold text-gray-950">Change my password</h2>
              <p className="mt-1 text-sm text-gray-600">
                Rotates your login credential. Requires your current password so a stolen browser session can not change it without you.
              </p>
            </div>
            <div className="grid gap-3 md:grid-cols-3">
              <label className="text-sm">
                <span className="text-gray-700">Current password</span>
                <input
                  type="password"
                  className="input mt-1 w-full"
                  autoComplete="current-password"
                  value={changeMyPasswordForm.current}
                  onChange={e => setChangeMyPasswordForm({ ...changeMyPasswordForm, current: e.target.value })}
                  required
                />
              </label>
              <label className="text-sm">
                <span className="text-gray-700">New password (12+ chars)</span>
                <input
                  type="password"
                  className="input mt-1 w-full"
                  autoComplete="new-password"
                  value={changeMyPasswordForm.next}
                  onChange={e => setChangeMyPasswordForm({ ...changeMyPasswordForm, next: e.target.value })}
                  required
                  minLength={12}
                />
              </label>
              <label className="text-sm">
                <span className="text-gray-700">Confirm new password</span>
                <input
                  type="password"
                  className="input mt-1 w-full"
                  autoComplete="new-password"
                  value={changeMyPasswordForm.confirm}
                  onChange={e => setChangeMyPasswordForm({ ...changeMyPasswordForm, confirm: e.target.value })}
                  required
                  minLength={12}
                />
              </label>
            </div>
            <div>
              <button type="submit" className="btn-primary" title="Update your login password. Existing browser sessions stay valid until they expire; new sign-ins must use the new password.">
                Change password
              </button>
            </div>
          </form>
          <div className="section-panel">
            <h2 className="text-lg font-semibold text-gray-950">Two-Factor Authentication (TOTP)</h2>
            <p className="mt-1 text-sm text-gray-600">Protect your account with Google Authenticator, Authy, or any TOTP-compatible app.</p>
            {me?.totp_enabled ? (
              <div className="mt-3 space-y-3">
                <div className="flex items-center gap-2 text-sm"><Badge tone="green">enabled</Badge> TOTP is active on your account.</div>
                <div className="grid gap-2 sm:grid-cols-[1fr_auto]">
                  <input type="password" className="input" placeholder="Enter your current password to disable" value={totpDisablePassword} onChange={e => setTotpDisablePassword(e.target.value)} />
                  <button className="mini-danger" disabled={!totpDisablePassword} onClick={async () => {
                    try {
                      await totpApi.disable(totpDisablePassword);
                      showMessage('TOTP disabled.');
                      setTotpDisablePassword('');
                      load();
                    } catch (err) { showError(err); }
                  }}>Disable 2FA</button>
                </div>
              </div>
            ) : !totpEnrollData ? (
              <div className="mt-3">
                <button className="btn-primary" disabled={totpEnrolling} onClick={async () => {
                  setTotpEnrolling(true);
                  try {
                    const res = await totpApi.enroll();
                    setTotpEnrollData(res.data);
                  } catch (err) { showError(err); }
                  setTotpEnrolling(false);
                }}>Set up 2FA</button>
              </div>
            ) : (
              <div className="mt-3 space-y-3">
                <div className="rounded-md border border-blue-200 bg-blue-50 p-3 text-sm text-blue-900">
                  <div className="font-medium">Scan this QR code with your authenticator app, or enter the secret manually.</div>
                  <div className="mt-2">
                    <img src={`https://api.qrserver.com/v1/create-qr-code/?size=200x200&data=${encodeURIComponent(totpEnrollData.otp_url)}`} alt="TOTP QR code" className="rounded" width={200} height={200} />
                  </div>
                  <div className="mt-2 break-all font-mono text-xs">{totpEnrollData.secret}</div>
                </div>
                <div className="rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900">
                  <div className="font-medium">Backup codes — save these somewhere safe</div>
                  <div className="mt-2 grid grid-cols-4 gap-2 font-mono text-xs">
                    {totpEnrollData.backup_codes?.map(code => <div key={code} className="rounded bg-white px-2 py-1 text-center">{code}</div>)}
                  </div>
                  <div className="mt-2 text-xs">Each backup code can be used once if you lose access to your authenticator app.</div>
                </div>
                <div className="grid gap-2 sm:grid-cols-[1fr_auto]">
                  <input className="input text-center text-lg tracking-widest" maxLength={6} inputMode="numeric" placeholder="Enter 6-digit code to verify" value={totpVerifyCode} onChange={e => setTotpVerifyCode(e.target.value.replace(/\D/g, ''))} />
                  <button className="btn-primary" disabled={totpVerifyCode.length !== 6} onClick={async () => {
                    try {
                      await totpApi.verify(totpVerifyCode);
                      showMessage('TOTP enabled! Your account now requires a code at login.');
                      setTotpEnrollData(null);
                      setTotpVerifyCode('');
                      load();
                    } catch (err) { showError(err); }
                  }}>Verify and Enable</button>
                </div>
                <button type="button" className="btn-secondary" onClick={() => { setTotpEnrollData(null); setTotpVerifyCode(''); }}>Cancel</button>
              </div>
            )}
          </div>
        </div>
      )}

      {admin && activeTab === 'general' && generalSettings && (
        <div className="space-y-4">
          <div className="section-panel space-y-3">
            <h2 className="text-lg font-semibold text-gray-950">Ports</h2>
            <p className="text-sm text-gray-600">Change the host ports Stack Manager listens on. Port changes require a full <code className="rounded bg-gray-100 px-1">docker compose --env-file .env up -d</code> restart — the browser URL will change.</p>
            <div className="grid gap-3 sm:grid-cols-2">
              <Field label="HTTPS port (WEB_SSL_PORT)" title="Host port for HTTPS. Default 8993. Set to 443 for standard HTTPS." hint="Default: 8993">
                <input className="input" type="number" min="1" max="65535" value={generalForm.web_ssl_port || ''} onChange={e => setGeneralForm({ ...generalForm, web_ssl_port: e.target.value })} />
              </Field>
              <Field label="HTTP port (WEB_HTTP_PORT)" title="Host port for HTTP redirect and ACME challenges. Set to 80 for Let's Encrypt, 0 to disable." hint="Default: 8193. Set 0 to disable.">
                <input className="input" type="number" min="0" max="65535" value={generalForm.web_http_port || ''} onChange={e => setGeneralForm({ ...generalForm, web_http_port: e.target.value })} />
              </Field>
            </div>
          </div>

          <div className="section-panel space-y-3">
            <h2 className="text-lg font-semibold text-gray-950">Cache and Refresh</h2>
            <p className="text-sm text-gray-600">Control how frequently the server re-discovers projects and how long cached data lives. Lower values show changes faster; higher values reduce Docker API load. Changes take effect on next server restart.</p>
            <div className="grid gap-3 sm:grid-cols-3">
              <Field label="Cache TTL (seconds)" title="How long project/image/job reads stay in Redis before refresh." hint="Default: 15">
                <input className="input" type="number" min="1" value={generalForm.cache_ttl_seconds || ''} onChange={e => setGeneralForm({ ...generalForm, cache_ttl_seconds: e.target.value })} />
              </Field>
              <Field label="Metrics refresh (minutes)" title="Background interval for project cache warmup, stats snapshots, and metrics history." hint="Default: 15. Minimum: 15.">
                <input className="input" type="number" min="15" value={generalForm.metrics_refresh_minutes || ''} onChange={e => setGeneralForm({ ...generalForm, metrics_refresh_minutes: e.target.value })} />
              </Field>
              <Field label="Warm cache TTL (minutes)" title="Redis TTL for background-warmed project and image-source caches." hint="Default: 30">
                <input className="input" type="number" min="1" value={generalForm.warm_cache_ttl_minutes || ''} onChange={e => setGeneralForm({ ...generalForm, warm_cache_ttl_minutes: e.target.value })} />
              </Field>
            </div>
          </div>

          <div className="section-panel space-y-3">
            <h2 className="text-lg font-semibold text-gray-950">Host URL</h2>
            <Field label="HOST_URL" title="The public URL printed in setup output and used in agent setup commands." hint="e.g. https://docker02:8993">
              <input className="input" value={generalForm.host_url || ''} onChange={e => setGeneralForm({ ...generalForm, host_url: e.target.value })} placeholder="https://your-host:8993" />
            </Field>
          </div>

          <div className="section-panel space-y-3">
            <h2 className="text-lg font-semibold text-gray-950">Extra Docker Roots</h2>
            <p className="text-sm text-gray-600">Discover projects in additional directories beyond the primary <code className="rounded bg-gray-100 px-1">DOCKER_ROOT</code>. Comma-separated absolute paths. Each extra root must be bind-mounted into the server container via a <code className="rounded bg-gray-100 px-1">compose.override.yml</code> for the server to access it. New projects are always created in the primary root.</p>
            <Field label="EXTRA_DOCKER_ROOTS" title="Comma-separated list of additional directories to scan for compose projects." hint="e.g. /opt/stacks,/home/user/compose-projects">
              <input className="input" value={generalForm.extra_docker_roots || ''} onChange={e => setGeneralForm({ ...generalForm, extra_docker_roots: e.target.value })} placeholder="/opt/stacks,/home/user/compose-projects" />
            </Field>
          </div>

          <div className="flex gap-2">
            <button className="btn-primary" onClick={saveGeneralSettings}>Save to .env</button>
            <button className="btn-secondary" onClick={loadGeneralSettings}>Reset</button>
          </div>

          <div className="section-panel space-y-3 border-amber-200 bg-amber-50">
            <h2 className="text-lg font-semibold text-amber-900">API Key</h2>
            <p className="text-sm text-amber-900">Generate a new API key. The old key stops working on the next server restart. Session-based browser logins are not affected.</p>
            <div className="flex items-center gap-2">
              <button className="mini-danger" onClick={rollAPIKey}>Roll API Key</button>
              {rolledAPIKey && (
                <div className="flex items-center gap-2">
                  <code className="rounded bg-white px-2 py-1 font-mono text-xs text-gray-800 break-all">{rolledAPIKey}</code>
                  <button className="mini-button" onClick={() => { navigator.clipboard?.writeText(rolledAPIKey); showMessage('Copied'); }}>Copy</button>
                </div>
              )}
            </div>
            {rolledAPIKey && <p className="text-xs text-amber-800">Save this key — it will not be shown again. Restart the server to activate it.</p>}
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
        <div className="space-y-4">
          <div className="space-y-4">
            <div className="section-panel">
              <h2 className="text-lg font-semibold text-gray-950">Agent Setup</h2>
              <div className="mt-3 flex flex-col gap-1">
                <label className="text-sm font-medium text-gray-700" htmlFor="controller-url">Controller URL <span className="font-normal text-gray-500">(IP or FQDN this controller is reachable at, used to fill the sample commands)</span></label>
                <input id="controller-url" className="input" value={controllerUrl} onChange={e => updateControllerUrl(e.target.value)} placeholder="https://10.10.10.93:8993" title="Base URL agents will call back to. Defaults to this browser tab's origin and is saved per browser." />
              </div>
              {(() => {
                const modes = [
                  { value: 'callback', label: 'Outbound check-in', appMode: 'agent-callback', hint: 'Agent phones home to this controller. No inbound port required on the agent host. Best behind NAT.' },
                  { value: 'inbound', label: 'Inbound listener', appMode: 'agent', hint: 'Agent opens a port and this controller calls it. Requires the agent to be reachable from this controller.' },
                  { value: 'both', label: 'Both (combined)', appMode: 'agent-both', hint: 'Runs the inbound listener and the outbound check-in in the same container. Good for hosts that are sometimes reachable.' },
                ];
                const active = modes.find(m => m.value === setupMode) || modes[0];
                const cmdPrepare = `git clone https://github.com/arphost-com/Stack-Manager.git stack-manager\ncd stack-manager\n./scripts/prepare-state.sh --agent .env\n# Edit .env: set DOCKER_ROOT and AGENT_CONTROLLER_URL=${agentControllerHost}`;
                const cmdStart = `APP_MODE=${active.appMode} \\\ndocker compose --env-file .env -f docker-compose.agent.yml up -d --build`;
                const cmdRegister = `grep -E '^(AGENT_NAME|AGENT_TOKEN|AGENT_PORT|DOCKER_ROOT|AGENT_CONTROLLER_URL)=' .env\n# Then in Settings > Agents, click Add Agent:\n#   Name  : value of AGENT_NAME\n#   Mode  : ${active.value === 'inbound' ? 'Inbound URL' : active.value === 'both' ? 'Inbound URL (or Outbound check-in)' : 'Outbound check-in'}\n#   URL   : ${active.value === 'callback' ? '(leave blank)' : 'https://<agent-host>:${AGENT_PORT}'}\n#   Token : value of AGENT_TOKEN`;
                const cards = [
                  { key: 'cmd-prepare', title: '1. Prepare agent state', body: cmdPrepare },
                  { key: 'cmd-start-' + active.value, title: `2. Start the agent container (${active.label})`, body: cmdStart },
                  { key: 'cmd-register-' + active.value, title: '3. Register below', body: cmdRegister },
                ];
                return (
                  <>
                    <div className="mt-3 rounded-md border border-gray-200 bg-gray-50 p-3">
                      <div className="text-sm font-medium text-gray-950">Agent mode</div>
                      <div className="mt-2 flex flex-wrap gap-2">
                        {modes.map(m => (
                          <button
                            key={m.value}
                            type="button"
                            onClick={() => updateSetupMode(m.value)}
                            className={setupMode === m.value ? 'btn-primary' : 'btn-secondary'}
                            title={m.hint}
                          >
                            {m.label}
                          </button>
                        ))}
                      </div>
                      <p className="mt-2 text-xs text-gray-600">{active.hint}</p>
                    </div>
                    <div className="mt-3 grid gap-3 text-sm text-gray-700 xl:grid-cols-3">
                      {cards.map(card => (
                        <div key={card.key} className="rounded-md border border-gray-200 p-3">
                          <div className="flex items-center justify-between gap-2">
                            <div className="font-medium text-gray-950">{card.title}</div>
                            <button type="button" onClick={() => copyToClipboard(card.key, card.body)} className="mini-button" title="Copy this snippet to the clipboard.">{copiedKey === card.key ? 'Copied' : 'Copy'}</button>
                          </div>
                          <pre className="mt-2 whitespace-pre-wrap break-all rounded bg-gray-950 p-3 font-mono text-xs text-gray-100">{card.body}</pre>
                        </div>
                      ))}
                    </div>
                  </>
                );
              })()}
              <p className="mt-3 text-sm text-gray-600">The agent runs as a container from <code className="rounded bg-gray-100 px-1">docker-compose.agent.yml</code>. Pick a mode above and the sample commands update to use the matching <code className="rounded bg-gray-100 px-1">APP_MODE</code>. The compose file mounts <code className="rounded bg-gray-100 px-1">DOCKER_ROOT</code> from the host so the agent sees the same paths projects use.</p>
            </div>
            <div className="section-panel">
              <div className="mb-3 flex items-center justify-between gap-2">
                <h2 className="text-lg font-semibold text-gray-950">Registered Agents</h2>
                <button type="button" className="btn-primary" onClick={() => { setAgentForm(emptyAgentForm); setAgentFormOpen(true); }} title="Open the agent editor.">Add Agent</button>
              </div>
              <div>
                <table className="w-full table-fixed text-left text-sm">
                  <thead><tr className="border-b border-gray-200 text-xs uppercase text-gray-500"><th className="w-[18%] py-2">Name</th><th className="w-[12%]">Mode</th><th className="w-[26%]">URL</th><th className="w-[12%]">Status</th><th className="w-[16%]">Last Seen</th><th className="w-[16%] text-right">Actions</th></tr></thead>
                  <tbody>
                    {agentList.map(agent => (
                      <tr key={agent.id} className="border-b border-gray-100 align-top">
                        <td className="py-2 pr-2 font-medium break-all">{agent.name}</td>
                        <td className="pr-2"><Badge tone={(agent.mode || (agent.base_url ? 'inbound' : 'callback')) === 'callback' ? 'blue' : 'gray'}>{(agent.mode || (agent.base_url ? 'inbound' : 'callback')) === 'callback' ? 'check-in' : 'inbound'}</Badge></td>
                        <td className="pr-2 font-mono text-xs text-gray-600 break-all" title={agent.base_url}>{agent.base_url}</td>
                        <td className="pr-2"><Badge tone={agent.enabled ? 'green' : 'gray'}>{agent.enabled ? 'enabled' : 'disabled'}</Badge></td>
                        <td className="pr-2 text-xs text-gray-500 break-words">{formatDate(agent.last_seen)}</td>
                        <td className="space-x-1 text-right whitespace-nowrap">
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
          <Modal open={agentFormOpen} onClose={closeAgentForm} title={agentForm.id ? 'Edit Agent' : 'Add Agent'}>
            <form onSubmit={saveAgent} className="space-y-3">
              <Field label="Name" title="Friendly unique name for the remote compose host. Saving an existing name updates it.">
                <input value={agentForm.name} onChange={e => setAgentForm({ ...agentForm, name: e.target.value })} className="input" placeholder="docker03" required />
              </Field>
              <Field label="Mode" title="Outbound check-in is for clients that cannot accept inbound connections. Inbound URL is for directly reachable agents.">
                <select value={agentForm.mode} onChange={e => setAgentForm({ ...agentForm, mode: e.target.value })} className="input">
                  <option value="callback">Outbound check-in</option>
                  <option value="inbound">Inbound URL</option>
                </select>
              </Field>
              <Field label="Agent URL" title="Base URL for inbound agents only, for example https://docker03.example.com:8192. Leave blank for outbound check-in agents.">
                <input value={agentForm.base_url} onChange={e => setAgentForm({ ...agentForm, base_url: e.target.value })} className="input" placeholder="https://host:8192" required={agentForm.mode === 'inbound'} />
              </Field>
              <Field label="Token" title="Bearer token used by the agent and controller. Leave blank when editing to keep the saved token.">
                <input type="password" value={agentForm.token} onChange={e => setAgentForm({ ...agentForm, token: e.target.value })} className="input" placeholder={agentForm.id ? 'leave blank to keep saved token' : 'agent token'} />
              </Field>
              <label className="flex items-center gap-2 text-sm text-gray-700" title="Disabled agents remain saved but scheduled actions will not run.">
                <input type="checkbox" checked={agentForm.enabled} onChange={e => setAgentForm({ ...agentForm, enabled: e.target.checked })} />
                Enabled
              </label>
              <div className="flex gap-2 pt-2">
                <button type="submit" className="btn-primary">Save Agent</button>
                <button type="button" onClick={closeAgentForm} className="btn-secondary">Cancel</button>
              </div>
            </form>
          </Modal>
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
            <Field label="Cadence" title="How often the action runs. All times are UTC.">
              <select value={scheduleForm.cadence} onChange={e => setScheduleForm({ ...scheduleForm, cadence: e.target.value })} className="input">
                <option value="daily">Daily</option>
                <option value="weekly">Weekly</option>
                <option value="monthly">Monthly</option>
                <option value="interval">Every N minutes</option>
              </select>
            </Field>
            {scheduleForm.cadence !== 'interval' && (
              <Field label="Time (UTC)" title="Hour and minute in UTC when the action fires.">
                <input type="time" value={scheduleForm.time_of_day} onChange={e => setScheduleForm({ ...scheduleForm, time_of_day: e.target.value })} className="input" />
              </Field>
            )}
            {scheduleForm.cadence === 'weekly' && (
              <Field label="Day of week" title="Which weekday.">
                <select value={scheduleForm.day_of_week} onChange={e => setScheduleForm({ ...scheduleForm, day_of_week: Number(e.target.value) })} className="input">
                  {WEEKDAYS.map((day, idx) => <option key={idx} value={idx}>{day}</option>)}
                </select>
              </Field>
            )}
            {scheduleForm.cadence === 'monthly' && (
              <Field label="Day of month" title="1–31. Months with fewer days run on the last day (e.g. Feb 28).">
                <input type="number" min="1" max="31" value={scheduleForm.day_of_month} onChange={e => setScheduleForm({ ...scheduleForm, day_of_month: Number(e.target.value) })} className="input" />
              </Field>
            )}
            {scheduleForm.cadence === 'interval' && (
              <Field label="Every minutes" title="Minimum 5 minutes.">
                <input type="number" min="5" value={scheduleForm.interval_minutes} onChange={e => setScheduleForm({ ...scheduleForm, interval_minutes: Number(e.target.value) })} className="input" />
              </Field>
            )}
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
              <thead><tr className="border-b border-gray-200 text-xs uppercase text-gray-500"><th className="py-2">Target</th><th>Action</th><th>Cadence</th><th>Next</th><th>Last</th><th className="text-right">Tools</th></tr></thead>
              <tbody>
                {scheduleList.map(schedule => (
                  <tr key={schedule.id} className="border-b border-gray-100">
                    <td className="py-2"><div className="font-medium">{schedule.project}</div><div className="text-xs text-gray-500">{schedule.agent_name || 'This server'}</div></td>
                    <td><Badge tone={schedule.enabled ? 'green' : 'gray'}>{schedule.action}</Badge></td>
                    <td>{formatCadence(schedule)}</td>
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
          <div className="section-panel space-y-3">
            <div className="flex items-center justify-between">
              <h2 className="text-lg font-semibold text-gray-950">Saved logins</h2>
              <button type="button" onClick={loadSavedRegistries} className="btn-secondary" title="Reload saved logins from the Docker config on disk.">Reload</button>
            </div>
            <p className="text-sm text-gray-600">Passwords never leave the host. Docker Hub requires your <strong>username</strong>, not your email — email logins silently fail even though Docker Hub says "login succeeded".</p>
            {savedRegistries.length === 0 ? (
              <div className="rounded-md border border-gray-100 bg-gray-50 p-3 text-sm text-gray-500">No saved Docker logins. Use the forms below to log in.</div>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full min-w-[560px] text-left text-sm">
                  <thead><tr className="border-b border-gray-200 text-xs uppercase text-gray-500"><th className="py-2">Registry</th><th>Username</th><th>Password</th><th>Config</th><th className="text-right">Actions</th></tr></thead>
                  <tbody>
                    {savedRegistries.map(login => {
                      const looksLikeEmail = login.username.includes('@');
                      const isDockerHub = login.registry === 'https://index.docker.io/v1/';
                      return (
                        <tr key={login.registry} className="border-b border-gray-100">
                          <td className="py-2 font-medium">{login.display}</td>
                          <td className="font-mono text-xs" title={looksLikeEmail && isDockerHub ? 'Docker Hub does not accept email logins — this login silently fails on pulls. Delete it and re-login with your username.' : login.username}>
                            {login.masked_user || '(none)'}
                            {looksLikeEmail && isDockerHub && <Badge tone="red">email won't work</Badge>}
                          </td>
                          <td className="font-mono text-xs text-gray-400">{'*'.repeat(12)}</td>
                          <td className="font-mono text-[11px] text-gray-500" title={login.stored_in}>{(login.stored_in || '').split('/').slice(-2).join('/')}</td>
                          <td className="space-x-2 text-right">
                            <button title={isDockerHub ? 'Load into the Docker Hub form to update the password/token.' : 'Load this login into the private registry form.'} onClick={() => editSavedRegistry(login)} className="mini-button">Edit</button>
                            <button title={`Remove this saved login. Runs \`docker logout ${login.display}\`.`} onClick={() => deleteSavedRegistry(login)} className="mini-danger">Delete</button>
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            )}
          </div>

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
              <div className="flex gap-2">
                <button type="button" onClick={loadDockerSettings} className="btn-secondary" title="Reload /etc/docker/daemon.json from the Docker host.">Reload</button>
                <button type="button" className="mini-danger" title="Restart the Docker daemon on the host via nsenter. All containers will briefly stop and restart with their restart policies." onClick={async () => {
                  if (!window.confirm('Restart Docker on the host? All containers will briefly stop. Containers with restart policies come back automatically.')) return;
                  try {
                    const res = await dockerSettings.restartDocker();
                    showMessage(res.data?.status || 'Docker restarted.');
                  } catch (err) { showError(err); }
                }}>Restart Docker</button>
              </div>
            </div>

            {dockerStatus?.warnings?.length > 0 && (
              <div className="mb-4 rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900">
                {dockerStatus.warnings.map(warning => <div key={warning}>{warning}</div>)}
              </div>
            )}

            {dockerStatus?.network_change && (
              <div className="mb-4 rounded-md border-2 border-red-300 bg-red-50 p-4 text-sm text-red-900">
                <div className="mb-2 flex items-center gap-2 font-semibold">
                  <span>⚠</span>
                  <span>Network fields changed — full teardown required</span>
                </div>
                <p className="mb-3">
                  You changed <code className="rounded bg-red-100 px-1 py-0.5 font-mono text-xs">{(dockerStatus.network_fields || []).join(', ')}</code>. A plain <code className="rounded bg-red-100 px-1 py-0.5 font-mono text-xs">systemctl restart docker</code> reloads these values into the daemon but existing bridges and iptables rules keep the OLD settings until every container and every user network is torn down. Run these commands as root on the host in order:
                </p>
                <pre className="mb-3 max-h-64 overflow-auto rounded bg-gray-950 p-3 font-mono text-xs text-gray-100">
                  {(dockerStatus.teardown_guide || []).join('\n')}
                </pre>
                <p className="text-xs text-red-800">
                  Volumes and images are preserved. Only containers, custom bridge networks, and iptables rules get rebuilt. Data-backed containers will re-attach their volumes on the next <code className="rounded bg-red-100 px-1 py-0.5">docker compose up -d</code>.
                </p>
                <div className="mt-3">
                  <button
                    type="button"
                    className="btn-secondary"
                    onClick={() => navigator.clipboard?.writeText((dockerStatus.teardown_guide || []).join('\n'))}
                    title="Copy the full teardown sequence to the clipboard."
                  >
                    Copy commands
                  </button>
                </div>
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

      {admin && activeTab === 'ssl' && (
        <div className="space-y-4">
          <div className="section-panel">
            <div className="mb-4 flex items-center justify-between">
              <div>
                <h2 className="text-lg font-semibold text-gray-950">Current Certificate</h2>
                <p className="text-sm text-gray-600">nginx terminates TLS with the cert files under <code className="rounded bg-gray-100 px-1 py-0.5 text-xs">STATE_DIR/ssl/</code>. Reads directly from the running container.</p>
              </div>
              <button type="button" className="btn-secondary" onClick={loadSslInfo} disabled={sslLoading} title="Re-read the certificate from disk.">
                {sslLoading ? 'Refreshing…' : 'Refresh'}
              </button>
            </div>
            {!sslInfo && <p className="text-sm text-gray-500">Loading cert info…</p>}
            {sslInfo && (
              <dl className="grid gap-3 text-sm md:grid-cols-2">
                <div>
                  <dt className="text-xs uppercase tracking-wide text-gray-500">Mode</dt>
                  <dd className="font-medium text-gray-950">
                    {sslInfo.mode === 'letsencrypt' ? "Let's Encrypt" : sslInfo.mode === 'self-signed' ? 'Self-signed' : sslInfo.mode || 'unknown'}
                    {sslInfo.self_signed && sslInfo.mode !== 'self-signed' && <span className="ml-2 text-xs text-gray-500">(cert is self-signed)</span>}
                  </dd>
                </div>
                <div>
                  <dt className="text-xs uppercase tracking-wide text-gray-500">Common Name</dt>
                  <dd className="font-medium text-gray-950">{sslInfo.cn || '—'}</dd>
                </div>
                <div>
                  <dt className="text-xs uppercase tracking-wide text-gray-500">Issuer</dt>
                  <dd className="text-gray-800">{sslInfo.issuer || '—'}</dd>
                </div>
                <div>
                  <dt className="text-xs uppercase tracking-wide text-gray-500">Expires</dt>
                  <dd className={sslInfo.days_left < 15 ? 'text-red-700' : sslInfo.days_left < 45 ? 'text-yellow-700' : 'text-gray-800'}>
                    {sslInfo.not_after ? new Date(sslInfo.not_after).toLocaleString() : '—'}
                    {typeof sslInfo.days_left === 'number' && <span className="ml-2 text-xs text-gray-500">({sslInfo.days_left} days)</span>}
                  </dd>
                </div>
                <div className="md:col-span-2">
                  <dt className="text-xs uppercase tracking-wide text-gray-500">Subject Alternative Names</dt>
                  <dd className="text-gray-800">
                    {sslInfo.sans && sslInfo.sans.length > 0 ? (
                      <div className="flex flex-wrap gap-1.5">
                        {sslInfo.sans.map(san => (
                          <span key={san} className="rounded bg-gray-100 px-1.5 py-0.5 font-mono text-xs">{san}</span>
                        ))}
                      </div>
                    ) : '—'}
                  </dd>
                </div>
                <div>
                  <dt className="text-xs uppercase tracking-wide text-gray-500">HTTP port (redirect)</dt>
                  <dd className="text-gray-800">{sslInfo.http_port}</dd>
                </div>
                <div>
                  <dt className="text-xs uppercase tracking-wide text-gray-500">HTTPS port</dt>
                  <dd className="text-gray-800">{sslInfo.ssl_port}</dd>
                </div>
                <div className="md:col-span-2">
                  <dt className="text-xs uppercase tracking-wide text-gray-500">Let's Encrypt ready</dt>
                  <dd className={sslInfo.letsencrypt_ready ? 'text-green-700' : 'text-yellow-700'}>
                    {sslInfo.letsencrypt_ready ? 'Yes — ports match HTTP-01 requirements (80/443).' : (sslInfo.letsencrypt_reason || 'No.')}
                  </dd>
                </div>
              </dl>
            )}
          </div>

          <div className="section-panel">
            <form onSubmit={regenerateSelfSigned} className="space-y-3">
              <div>
                <h2 className="text-lg font-semibold text-gray-950">Regenerate self-signed</h2>
                <p className="text-sm text-gray-600">Writes a fresh cert to <code className="rounded bg-gray-100 px-1 py-0.5 text-xs">STATE_DIR/ssl/</code> and sends SIGHUP to nginx. Browsers will still show an untrusted-issuer warning until the CA is imported.</p>
              </div>
              <div className="grid gap-3 md:grid-cols-3">
                <label className="text-sm">
                  <span className="text-gray-700">Common name</span>
                  <input type="text" className="input mt-1 w-full" value={sslSelfSignedForm.cn} onChange={e => setSslSelfSignedForm({ ...sslSelfSignedForm, cn: e.target.value })} placeholder="stack.example.com" required />
                </label>
                <label className="text-sm">
                  <span className="text-gray-700">Extra SANs (comma-separated)</span>
                  <input type="text" className="input mt-1 w-full" value={sslSelfSignedForm.extra_sans} onChange={e => setSslSelfSignedForm({ ...sslSelfSignedForm, extra_sans: e.target.value })} placeholder="10.0.0.5, api.example.com" />
                </label>
                <label className="text-sm">
                  <span className="text-gray-700">Days valid</span>
                  <input type="number" min="1" max="3650" className="input mt-1 w-full" value={sslSelfSignedForm.days} onChange={e => setSslSelfSignedForm({ ...sslSelfSignedForm, days: e.target.value })} />
                </label>
              </div>
              <div>
                <button type="submit" className="btn-primary" title="Regenerate self-signed cert and reload nginx.">Regenerate</button>
              </div>
            </form>
          </div>

          <div className="section-panel">
            <form onSubmit={enableLetsEncrypt} className="space-y-3">
              <div>
                <h2 className="text-lg font-semibold text-gray-950">Switch to Let's Encrypt</h2>
                <p className="text-sm text-gray-600">
                  HTTP-01 challenge via certbot. Requires <code className="rounded bg-gray-100 px-1 py-0.5 text-xs">WEB_HTTP_PORT=80</code>, <code className="rounded bg-gray-100 px-1 py-0.5 text-xs">WEB_SSL_PORT=443</code>, and the domain’s DNS pointed at this host. Use <em>staging</em> first to avoid the production rate limit.
                </p>
              </div>
              <div className="grid gap-3 md:grid-cols-3">
                <label className="text-sm">
                  <span className="text-gray-700">Domain</span>
                  <input type="text" className="input mt-1 w-full" value={sslLeForm.domain} onChange={e => setSslLeForm({ ...sslLeForm, domain: e.target.value })} placeholder="stack.example.com" required />
                </label>
                <label className="text-sm">
                  <span className="text-gray-700">Contact email</span>
                  <input type="email" className="input mt-1 w-full" value={sslLeForm.email} onChange={e => setSslLeForm({ ...sslLeForm, email: e.target.value })} placeholder="ops@example.com" required />
                </label>
                <label className="mt-6 flex items-center gap-2 text-sm">
                  <input type="checkbox" checked={sslLeForm.staging} onChange={e => setSslLeForm({ ...sslLeForm, staging: e.target.checked })} />
                  <span>Use Let's Encrypt staging</span>
                </label>
              </div>
              <div className="flex flex-wrap items-center gap-3">
                <button type="submit" className="btn-primary" disabled={!sslInfo?.letsencrypt_ready} title={sslInfo?.letsencrypt_ready ? 'Request cert now' : 'Change WEB_HTTP_PORT=80 and WEB_SSL_PORT=443 in .env first, then redeploy.'}>
                  Issue certificate
                </button>
                <button type="button" className="btn-secondary" onClick={renewLetsEncrypt} disabled={sslInfo?.mode !== 'letsencrypt'} title="Trigger certbot renew now.">
                  Renew now
                </button>
                {!sslInfo?.letsencrypt_ready && (
                  <span className="text-xs text-yellow-700">Ports are not set to 80/443 — update <code className="rounded bg-gray-100 px-1 py-0.5">.env</code> and redeploy before requesting.</span>
                )}
              </div>
            </form>
          </div>
        </div>
      )}

      {admin && activeTab === 'proxy' && (
        <div className="space-y-4">
          <div className="section-panel">
            <h2 className="text-lg font-semibold text-gray-950">Reverse Proxy (Nginx Proxy Manager)</h2>
            <p className="mt-1 text-sm text-gray-600">{npmStatus?.connected ? 'Connected to NPM. Manage proxy hosts below.' : 'Set up a reverse proxy when you have real domain names and want per-domain HTTPS via Let\'s Encrypt.'}</p>
          </div>

          {!npmStatus?.connected && (
            <>
              <div className="section-panel space-y-3">
                <h3 className="text-base font-semibold text-gray-950">Setup Guide</h3>
                <ol className="list-decimal space-y-3 pl-5 text-sm text-gray-700">
                  <li>
                    <span className="font-medium">Deploy NPM from the Stack Catalog.</span> Go to <Link to="/catalog" className="underline text-blue-700">Stack Catalog</Link>, search for <code className="rounded bg-gray-100 px-1">nginx-proxy-manager</code>, and click <span className="font-medium">Spin it Up</span>. It binds ports 80 (HTTP), 443 (HTTPS), and 81 (admin).
                  </li>
                  <li>
                    <span className="font-medium">Resolve port conflicts.</span> If Stack Manager's web container also binds port 80, change <code className="rounded bg-gray-100 px-1">WEB_HTTP_PORT</code> in <Link to="/settings" onClick={() => {}} className="underline text-blue-700">Settings &gt; General</Link> to a different port or set it to <code className="rounded bg-gray-100 px-1">0</code> to disable. Restart the stack afterward.
                  </li>
                  <li>
                    <span className="font-medium">Log into NPM admin.</span> Open <code className="rounded bg-gray-100 px-1">http://&lt;host-ip&gt;:81</code>. Default credentials: <code className="rounded bg-gray-100 px-1">admin@example.com</code> / <code className="rounded bg-gray-100 px-1">changeme</code>. <strong>Change the password immediately.</strong>
                  </li>
                  <li>
                    <span className="font-medium">Connect below.</span> Enter the NPM admin URL and your updated credentials, then click Connect. Once connected, you can manage proxy hosts from this panel.
                  </li>
                </ol>
              </div>

              <div className="section-panel border-amber-200 bg-amber-50">
                <h3 className="text-base font-semibold text-amber-900">When NOT to use a reverse proxy</h3>
                <ul className="mt-2 list-disc space-y-1 pl-5 text-sm text-amber-900">
                  <li>Private networks with no public DNS — Let's Encrypt can't issue certificates for IPs or unreachable domains.</li>
                  <li>Public IPs with no domain — same reason. Use the self-signed cert and accept the browser warning.</li>
                  <li>Single-service hosts — a reverse proxy adds complexity. The built-in nginx with a self-signed cert is simpler.</li>
                </ul>
              </div>
            </>
          )}

          <div className="section-panel space-y-3">
            <h3 className="text-base font-semibold text-gray-950">NPM Connection</h3>
            <div className="grid gap-2 sm:grid-cols-3">
              <Field label="NPM Admin URL" hint="e.g. http://78.109.20.111:81">
                <input className="input" value={npmForm.url} onChange={e => setNpmForm({ ...npmForm, url: e.target.value })} placeholder="http://localhost:81" />
              </Field>
              <Field label="Admin Email" hint="default: admin@example.com">
                <input className="input" value={npmForm.email} onChange={e => setNpmForm({ ...npmForm, email: e.target.value })} />
              </Field>
              <Field label="Password">
                <input className="input" type="password" value={npmForm.password} onChange={e => setNpmForm({ ...npmForm, password: e.target.value })} placeholder={npmStatus?.connected ? 'connected' : 'changeme'} />
              </Field>
            </div>
            <div className="flex items-center gap-3">
              <button className="btn-primary" onClick={connectNpm}>Connect</button>
              {npmStatus?.connected && <Badge tone="green">connected</Badge>}
              {npmStatus && !npmStatus.connected && npmStatus.configured && <Badge tone="red">disconnected</Badge>}
            </div>
          </div>

          {npmStatus?.connected && (
            <>
              <div className="section-panel space-y-3">
                <div className="flex items-center justify-between gap-2">
                  <h3 className="text-base font-semibold text-gray-950">Proxy Hosts ({npmHosts.length})</h3>
                  <a href={npmStatus?.url || '#'} target="_blank" rel="noreferrer" className="text-xs text-blue-700 underline" title="Open the full NPM admin panel in a new tab.">Open NPM Admin</a>
                </div>
                {npmHosts.length > 0 && (
                  <div className="overflow-x-auto">
                    <table className="w-full text-left text-sm">
                      <thead><tr className="border-b border-gray-200 text-xs uppercase text-gray-500"><th className="py-2">Domain(s)</th><th>Forward</th><th>SSL</th><th className="text-right">Actions</th></tr></thead>
                      <tbody>
                        {npmHosts.map(host => (
                          <tr key={host.id} className="border-b border-gray-100">
                            <td className="py-2 font-mono text-xs">{(host.domain_names || []).join(', ')}</td>
                            <td className="text-xs text-gray-600">{host.forward_scheme}://{host.forward_host}:{host.forward_port}</td>
                            <td>{host.certificate_id > 0 ? <Badge tone="green">SSL</Badge> : <Badge tone="gray">none</Badge>}</td>
                            <td className="text-right"><button className="mini-danger" onClick={() => deleteProxyHost(host.id, (host.domain_names || []).join(', '))}>Delete</button></td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}
                {npmHosts.length === 0 && <p className="text-sm text-gray-500">No proxy hosts configured yet. Add one below or use the <a href={npmStatus?.url || '#'} target="_blank" rel="noreferrer" className="underline text-blue-700">NPM admin panel</a> for full control.</p>}
              </div>

              <div className="section-panel space-y-3">
                <h3 className="text-base font-semibold text-gray-950">Add Proxy Host</h3>
                {npmSuggestions.length > 0 && (
                  <div className="text-sm text-gray-600">
                    Running projects with exposed ports:
                    <div className="mt-1 flex flex-wrap gap-1">
                      {npmSuggestions.map(s => (
                        <button key={s.name} className="rounded bg-blue-50 px-2 py-0.5 text-xs text-blue-800 hover:bg-blue-100" onClick={() => {
                          const portMatch = s.ports.match(/(\d+)->(\d+)/);
                          setNpmHostForm({ ...npmHostForm, forward_host: window.location.hostname, forward_port: portMatch ? portMatch[1] : '', domain: s.name + '.example.com' });
                        }} title={s.ports}>{s.name}</button>
                      ))}
                    </div>
                  </div>
                )}
                <div className="grid gap-2 sm:grid-cols-4">
                  <Field label="Domain">
                    <input className="input" value={npmHostForm.domain} onChange={e => setNpmHostForm({ ...npmHostForm, domain: e.target.value })} placeholder="app.example.com" />
                  </Field>
                  <Field label="Forward Host">
                    <input className="input" value={npmHostForm.forward_host} onChange={e => setNpmHostForm({ ...npmHostForm, forward_host: e.target.value })} placeholder={window.location.hostname} />
                  </Field>
                  <Field label="Forward Port">
                    <input className="input" type="number" value={npmHostForm.forward_port} onChange={e => setNpmHostForm({ ...npmHostForm, forward_port: e.target.value })} placeholder="8080" />
                  </Field>
                  <Field label="Scheme">
                    <select className="input" value={npmHostForm.forward_scheme} onChange={e => setNpmHostForm({ ...npmHostForm, forward_scheme: e.target.value })}>
                      <option value="http">http</option>
                      <option value="https">https</option>
                    </select>
                  </Field>
                </div>
                <button className="btn-primary" onClick={createProxyHost}>Create Proxy Host</button>
                <p className="text-xs text-gray-500">After creating, open <a href={npmStatus?.url || '#'} target="_blank" rel="noreferrer" className="underline text-blue-700">NPM admin</a> to enable SSL and request a Let's Encrypt certificate for this host.</p>
              </div>

              <div className="section-panel space-y-3">
                <h3 className="text-base font-semibold text-gray-950">NPM Documentation</h3>
                <p className="text-sm text-gray-600">For full proxy host configuration, SSL certificates, redirections, streams, access lists, and advanced settings, use the NPM admin panel directly.</p>
                <div className="flex flex-wrap gap-2">
                  <a href={npmStatus?.url || '#'} target="_blank" rel="noreferrer" className="btn-secondary inline-flex items-center gap-1">Open NPM Admin</a>
                  <a href="https://nginxproxymanager.com/guide/" target="_blank" rel="noreferrer" className="btn-secondary inline-flex items-center gap-1">NPM Official Docs</a>
                  <Link to="/documentation" className="btn-secondary inline-flex items-center gap-1">Stack Manager Docs</Link>
                </div>
              </div>
            </>
          )}
        </div>
      )}

      {admin && activeTab === 'backups' && (
        <div className="section-panel">
          <div className="mb-4 flex flex-col gap-1">
            <h2 className="text-lg font-semibold text-gray-950">Backup Endpoints</h2>
            <p className="text-sm text-gray-600">Configure local, mounted, FTP, SFTP, and S3 destinations. CIFS and NFS should be mounted on the host and exposed to the server container as paths.</p>
          </div>
          <div className="flex items-center justify-between gap-2">
            <div className="text-sm text-gray-600">{destinationList.length} endpoint{destinationList.length === 1 ? '' : 's'} saved.</div>
            <button type="button" className="btn-primary" onClick={() => { setDestinationForm(emptyDestinationForm); setDestinationFormOpen(true); }} title="Open the endpoint editor.">Add Endpoint</button>
          </div>
          <div className="mt-3">
            <div>
              <table className="w-full table-fixed text-left text-sm">
                <thead><tr className="border-b border-gray-200 text-xs uppercase text-gray-500"><th className="w-[20%] py-2">Name</th><th className="w-[12%]">Type</th><th className="w-[36%]">Path / Target</th><th className="w-[14%]">Status</th><th className="w-[18%] text-right">Actions</th></tr></thead>
                <tbody>
                  {destinationList.map(destination => (
                    <tr key={destination.id} className="border-b border-gray-100 align-top">
                      <td className="py-2 pr-2 font-medium break-all">{destination.name}</td>
                      <td className="pr-2">{destination.type}</td>
                      <td className="pr-2 font-mono text-xs text-gray-600 break-all">{destinationSummary(destination)}</td>
                      <td className="pr-2 break-words">{destination.enabled ? 'enabled' : 'disabled'}{destination.has_secret ? ' - secret saved' : ''}</td>
                      <td className="space-x-1 text-right whitespace-nowrap">
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
          </div>

          <Modal open={destinationFormOpen} onClose={closeDestinationForm} title={destinationForm.id ? 'Edit Backup Endpoint' : 'Add Backup Endpoint'}>
            <form onSubmit={saveDestination} className="space-y-3">
              <input className="input" placeholder="endpoint name" value={destinationForm.name} onChange={e => setDestinationForm({ ...destinationForm, name: e.target.value })} required title="Friendly name shown in backup dropdowns." />
              <select className="input" value={destinationForm.type} onChange={e => setDestinationForm({ ...destinationForm, type: e.target.value })} title="Choose the backup endpoint type.">
                {DESTINATION_TYPES.map(type => <option key={type.value} value={type.value}>{type.label}</option>)}
              </select>
              <label className="flex items-center gap-2 text-sm text-gray-700" title="Disabled endpoints stay saved but cannot be selected for new backups.">
                <input type="checkbox" checked={destinationForm.enabled} onChange={e => setDestinationForm({ ...destinationForm, enabled: e.target.checked })} />
                Enabled
              </label>

              {['local', 'mount', 'cifs', 'nfs'].includes(destinationForm.type) && (
                <input className="input" placeholder="/mnt/backups/stack-manager" value={destinationForm.config.path} onChange={e => setDestinationConfig('path', e.target.value)} required title="Absolute path inside the server container. Bind mount host CIFS/NFS paths into the server container before using them here." />
              )}

              {['ftp', 'sftp'].includes(destinationForm.type) && (
                <div className="grid gap-2">
                  <input className="input" placeholder="host" value={destinationForm.config.host} onChange={e => setDestinationConfig('host', e.target.value)} required title="FTP/SFTP host name or IP." />
                  <div className="grid gap-2 sm:grid-cols-2">
                    <input className="input" placeholder="port" value={destinationForm.config.port} onChange={e => setDestinationConfig('port', e.target.value)} title="Optional port. Defaults are handled by rclone." />
                    <input className="input" placeholder="username" value={destinationForm.config.username} onChange={e => setDestinationConfig('username', e.target.value)} required title="FTP/SFTP username." />
                  </div>
                  <input className="input" type="password" placeholder={destinationForm.id ? 'password, leave blank to keep saved' : 'password'} value={destinationForm.secrets.password} onChange={e => setDestinationSecret('password', e.target.value)} title="FTP/SFTP password. Leave blank during edits to keep the saved secret." />
                  {destinationForm.type === 'sftp' && (
                    <div className="rounded-md border border-gray-200 bg-gray-50 p-3">
                      <div className="flex items-center justify-between gap-2">
                        <div className="text-sm font-medium text-gray-800">SSH private key</div>
                        <button type="button" className="mini-button" onClick={generateSSHKey} title="Generate a fresh Ed25519 key on the server. Paste the returned public key into the SFTP server's ~/.ssh/authorized_keys.">Generate Ed25519 key</button>
                      </div>
                      <textarea
                        className="input mt-2 font-mono text-xs"
                        rows={4}
                        placeholder={'Paste private key (OpenSSH PEM, -----BEGIN OPENSSH PRIVATE KEY----- ...) or click Generate.\nLeave blank on edit to keep the saved key.'}
                        value={destinationForm.secrets.private_key}
                        onChange={e => setDestinationSecret('private_key', e.target.value)}
                        title="Private key content. Stored encrypted-at-rest in MariaDB alongside the password."
                      />
                      <input className="input mt-2" placeholder="Advanced: key_file inside container (leave blank if pasting a key above)" value={destinationForm.config.key_file} onChange={e => setDestinationConfig('key_file', e.target.value)} title="Optional path to a private key file already inside the server container. Only needed if you don't want to paste the key above." />
                      {destinationPublicKey && (
                        <div className="mt-3">
                          <div className="flex items-center justify-between gap-2">
                            <div className="text-xs font-medium uppercase text-gray-500">Public key — paste into the SFTP server</div>
                            <button type="button" className="mini-button" onClick={() => copyToClipboard('sftp-pub', destinationPublicKey)}>{copiedKey === 'sftp-pub' ? 'Copied' : 'Copy'}</button>
                          </div>
                          <pre className="mt-1 whitespace-pre-wrap break-all rounded bg-gray-950 p-2 font-mono text-xs text-gray-100">{destinationPublicKey}</pre>
                          <div className="mt-1 text-xs text-gray-600">On the SFTP host, append this line to <code className="rounded bg-gray-100 px-1">~/.ssh/authorized_keys</code> for user <code className="rounded bg-gray-100 px-1">{destinationForm.config.username || '(username)'}</code>.</div>
                        </div>
                      )}
                    </div>
                  )}
                  <input className="input" placeholder="remote/path" value={destinationForm.config.remote_path} onChange={e => setDestinationConfig('remote_path', e.target.value)} title="Remote directory for uploaded backup archives." />
                </div>
              )}

              {destinationForm.type === 's3' && (
                <div className="grid gap-2">
                  <input className="input" placeholder="bucket" value={destinationForm.config.bucket} onChange={e => setDestinationConfig('bucket', e.target.value)} required title="S3 bucket name." />
                  <input className="input" placeholder="prefix, e.g. stack-manager/docker01" value={destinationForm.config.prefix} onChange={e => setDestinationConfig('prefix', e.target.value)} title="Optional object prefix inside the bucket." />
                  <input className="input" placeholder="endpoint, e.g. https://s3.amazonaws.com" value={destinationForm.config.endpoint} onChange={e => setDestinationConfig('endpoint', e.target.value)} title="Optional endpoint for S3-compatible storage." />
                  <div className="grid gap-2 sm:grid-cols-2">
                    <input className="input" placeholder="region" value={destinationForm.config.region} onChange={e => setDestinationConfig('region', e.target.value)} title="Optional region." />
                    <input className="input" placeholder="provider" value={destinationForm.config.provider} onChange={e => setDestinationConfig('provider', e.target.value)} title="rclone S3 provider. Use Other for generic S3-compatible endpoints." />
                  </div>
                  <input className="input" type="password" placeholder={destinationForm.id ? 'access key, leave blank to keep saved' : 'access key'} value={destinationForm.secrets.access_key_id} onChange={e => setDestinationSecret('access_key_id', e.target.value)} title="S3 access key ID. Leave blank during edits to keep the saved value." />
                  <input className="input" type="password" placeholder={destinationForm.id ? 'secret key, leave blank to keep saved' : 'secret key'} value={destinationForm.secrets.secret_access_key} onChange={e => setDestinationSecret('secret_access_key', e.target.value)} title="S3 secret access key. Leave blank during edits to keep the saved value." />
                </div>
              )}

              <div className="flex gap-2 pt-2">
                <button className="btn-primary" title="Save this backup endpoint in MariaDB.">Save Endpoint</button>
                <button type="button" onClick={closeDestinationForm} className="btn-secondary" title="Close without saving.">Cancel</button>
              </div>
            </form>
          </Modal>
        </div>
      )}

      {admin && activeTab === 'firewall' && (
        <div className="space-y-4">
          <div className="section-panel">
            <div className="flex flex-col gap-1">
              <h2 className="text-lg font-semibold text-gray-950">Firewall (ConfigServer csf/lfd)</h2>
              <p className="text-sm text-gray-600">
                Drives csf on the host through a scoped root helper at <code className="rounded bg-gray-100 px-1">/usr/local/sbin/stack-manager-csf</code>. Successful logins allow-list the caller&apos;s IP automatically. Upstream: <a className="underline" href="https://github.com/Black-HOST/csf" target="_blank" rel="noreferrer">Black-HOST/csf</a>.
              </p>
            </div>
            <div className="mt-3 grid gap-3 md:grid-cols-4">
              <div className="rounded-md border border-gray-200 bg-gray-50 p-3">
                <div className="text-xs uppercase text-gray-500">Installed</div>
                <div className="mt-1 text-sm font-medium">{firewallStatus?.installed ? 'yes' : 'no'}</div>
              </div>
              <div className="rounded-md border border-gray-200 bg-gray-50 p-3">
                <div className="text-xs uppercase text-gray-500">LFD active</div>
                <div className="mt-1 text-sm font-medium">{firewallStatus?.lfd_active ? 'yes' : 'no'}</div>
              </div>
              <div className="rounded-md border border-gray-200 bg-gray-50 p-3" title="TESTING=1 in csf.conf means csf writes rules but flushes on lfd restart.">
                <div className="text-xs uppercase text-gray-500">Testing mode</div>
                <div className="mt-1 text-sm font-medium">{firewallStatus?.testing_mode || '-'}</div>
              </div>
              <div className="rounded-md border border-gray-200 bg-gray-50 p-3">
                <div className="text-xs uppercase text-gray-500">iptables rules</div>
                <div className="mt-1 text-sm font-medium">{firewallStatus?.iptables_rules ?? '-'}</div>
              </div>
            </div>
            {firewallStatus?.version && <div className="mt-2 text-xs font-mono text-gray-600 break-all">{firewallStatus.version}</div>}
            {firewallStatus && firewallStatus.helper_installed === false && (
              <div className="mt-3 rounded-md border border-amber-200 bg-amber-50 p-3">
                <div className="text-sm font-medium text-amber-900">Host helper script not installed</div>
                <p className="mt-1 text-xs text-amber-900">The server container drives csf through <code className="rounded bg-white/60 px-1">/usr/local/sbin/stack-manager-csf</code> on the host. Install it once on this host before using this panel — no sudoers changes needed.</p>
                <div className="mt-2 flex items-center justify-between gap-2">
                  <pre className="flex-1 whitespace-pre-wrap break-all rounded bg-gray-950 p-2 font-mono text-xs text-gray-100">{firewallStatus.helper_install_hint || 'sudo install -m 750 scripts/stack-manager-csf.sh /usr/local/sbin/stack-manager-csf'}</pre>
                  <button type="button" className="mini-button" onClick={() => copyToClipboard('firewall-helper-hint', firewallStatus.helper_install_hint || 'sudo install -m 750 scripts/stack-manager-csf.sh /usr/local/sbin/stack-manager-csf')} title="Copy the install command.">{copiedKey === 'firewall-helper-hint' ? 'Copied' : 'Copy'}</button>
                </div>
                <p className="mt-2 text-xs text-amber-900">After running that on the host, click <span className="font-medium">Refresh</span> below.</p>
              </div>
            )}
            <div className="mt-3 flex flex-wrap gap-2">
              {!firewallStatus?.installed && firewallStatus?.helper_installed !== false && <button className="btn-primary" disabled={firewallBusy} onClick={installFirewall} title="Clone Black-HOST/csf and run its installer via the root helper.">Install csf</button>}
              {firewallStatus?.installed && <>
                <button className="btn-secondary" disabled={firewallBusy} onClick={restartFirewall} title="Run csf -r on the host.">Restart csf</button>
                <button className="btn-secondary" disabled={firewallBusy} onClick={reloadLFD} title="systemctl restart lfd.">Reload lfd</button>
                <button className="mini-danger" disabled={firewallBusy} onClick={uninstallFirewall} title="Run /etc/csf/uninstall.sh. Removes csf and lfd from the host.">Uninstall csf</button>
              </>}
              <button className="btn-secondary" disabled={firewallBusy} onClick={loadFirewall} title="Re-poll status, allow/deny lists, and log tail.">Refresh</button>
            </div>
          </div>

          <div className="section-panel">
            <div className="flex flex-col gap-1">
              <h3 className="text-base font-semibold text-gray-950">Your IP</h3>
              <p className="text-sm text-gray-600">Detected from the request. When csf is installed, every successful login already runs an allow for this IP; use the button to force one if the login-time attempt failed.</p>
            </div>
            <div className="mt-2 flex flex-wrap items-center gap-2">
              <span className="font-mono text-sm text-gray-800">{firewallMyIP || '(unknown)'}</span>
              <button className="btn-primary" disabled={!firewallStatus?.installed || firewallBusy || !firewallMyIP} onClick={allowMyIP} title="Run csf -a for your detected IP with a Stack Manager comment.">Add my IP</button>
            </div>
          </div>

          {firewallStatus?.installed && (
            <>
              <div className="section-panel">
                <h3 className="text-base font-semibold text-gray-950">Manual allow / deny</h3>
                <div className="mt-3 grid gap-2 md:grid-cols-[1fr_2fr_auto_auto]">
                  <input className="input" placeholder="IP or CIDR (v4 or v6)" value={firewallIPForm.ip} onChange={e => setFirewallIPForm({ ...firewallIPForm, ip: e.target.value })} />
                  <input className="input" placeholder="Comment (letters, digits, space, . _ @ : / -)" value={firewallIPForm.comment} onChange={e => setFirewallIPForm({ ...firewallIPForm, comment: e.target.value })} maxLength={120} />
                  <button className="btn-primary" disabled={firewallBusy} onClick={() => submitFirewallIP('allow')} title="csf -a for IPv4 or csf6 -a for IPv6.">Allow</button>
                  <button className="mini-danger" disabled={firewallBusy} onClick={() => submitFirewallIP('deny')} title="csf -d for IPv4 or csf6 -d for IPv6.">Deny</button>
                </div>
              </div>

              <div className="section-panel">
                <div className="grid gap-6 lg:grid-cols-2">
                  <div>
                    <h3 className="text-base font-semibold text-gray-950">csf.allow ({firewallAllow.length})</h3>
                    <div className="mt-2 max-h-72 overflow-auto rounded border border-gray-200">
                      <table className="w-full text-left text-xs">
                        <thead className="bg-gray-50 text-gray-500"><tr><th className="p-2">IP</th><th className="p-2">Comment</th><th className="p-2 text-right">Action</th></tr></thead>
                        <tbody>
                          {firewallAllow.map(entry => (
                            <tr key={'allow-' + entry.raw} className="border-t border-gray-100">
                              <td className="p-2 font-mono whitespace-nowrap">{entry.ip}</td>
                              <td className="p-2">{entry.comment}</td>
                              <td className="p-2 text-right whitespace-nowrap"><button className="mini-danger" disabled={firewallBusy} onClick={() => removeFirewallIP(entry.ip)} title="Remove from both allow and deny.">Remove</button></td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                      {firewallAllow.length === 0 && <div className="p-3 text-xs text-gray-500">No allow entries.</div>}
                    </div>
                  </div>
                  <div>
                    <h3 className="text-base font-semibold text-gray-950">csf.deny ({firewallDeny.length})</h3>
                    <div className="mt-2 max-h-72 overflow-auto rounded border border-gray-200">
                      <table className="w-full text-left text-xs">
                        <thead className="bg-gray-50 text-gray-500"><tr><th className="p-2">IP</th><th className="p-2">Comment</th><th className="p-2 text-right">Action</th></tr></thead>
                        <tbody>
                          {firewallDeny.map(entry => (
                            <tr key={'deny-' + entry.raw} className="border-t border-gray-100">
                              <td className="p-2 font-mono whitespace-nowrap">{entry.ip}</td>
                              <td className="p-2">{entry.comment}</td>
                              <td className="p-2 text-right whitespace-nowrap"><button className="mini-danger" disabled={firewallBusy} onClick={() => removeFirewallIP(entry.ip)} title="Remove from both allow and deny.">Remove</button></td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                      {firewallDeny.length === 0 && <div className="p-3 text-xs text-gray-500">No deny entries.</div>}
                    </div>
                  </div>
                </div>
              </div>

              {firewallConf && (
                <div className="section-panel space-y-4">
                  <h3 className="text-base font-semibold text-gray-950">Firewall Settings</h3>
                  <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
                    <label className="flex items-center gap-2 text-sm" title="TESTING=1 means csf writes rules but flushes on lfd restart. Disable testing mode once your ports are correct so rules persist.">
                      <input type="checkbox" checked={firewallConfForm.testing === '1'} onChange={e => setFirewallConfForm({ ...firewallConfForm, testing: e.target.checked ? '1' : '0' })} />
                      <span>Testing mode <span className="text-xs text-gray-500">(TESTING)</span></span>
                    </label>
                    <label className="flex items-center gap-2 text-sm" title="Enable Docker compatibility so CSF accommodates Docker's iptables chains.">
                      <input type="checkbox" checked={firewallConfForm.docker === '1'} onChange={e => setFirewallConfForm({ ...firewallConfForm, docker: e.target.checked ? '1' : '0' })} />
                      <span>Docker mode <span className="text-xs text-gray-500">(DOCKER)</span></span>
                    </label>
                    <label className="flex items-center gap-2 text-sm" title="Enable SYN flood protection.">
                      <input type="checkbox" checked={firewallConfForm.synflood === '1'} onChange={e => setFirewallConfForm({ ...firewallConfForm, synflood: e.target.checked ? '1' : '0' })} />
                      <span>SYN flood protection</span>
                    </label>
                    <label className="flex items-center gap-2 text-sm" title="Restrict syslog/kernel logging to prevent log injection attacks. 0=off, 3=most restrictive.">
                      <span className="whitespace-nowrap">Syslog restrict</span>
                      <select className="input w-16" value={firewallConfForm.restrict_syslog || '0'} onChange={e => setFirewallConfForm({ ...firewallConfForm, restrict_syslog: e.target.value })}>
                        <option value="0">0</option><option value="1">1</option><option value="2">2</option><option value="3">3</option>
                      </select>
                    </label>
                  </div>

                  <div className="grid gap-3 sm:grid-cols-2">
                    <Field label="TCP IN (incoming)" hint="Ports people connect TO (SSH, HTTP, HTTPS, etc.)" title="Comma-separated list of allowed incoming TCP ports.">
                      <input className="input font-mono text-xs" value={firewallConfForm.tcp_in || ''} onChange={e => setFirewallConfForm({ ...firewallConfForm, tcp_in: e.target.value })} placeholder="22,80,443" />
                    </Field>
                    <Field label="TCP OUT (outgoing)" hint="Ports your server connects OUT on (DNS, HTTP, SMTP, etc.)" title="Comma-separated list of allowed outgoing TCP ports.">
                      <input className="input font-mono text-xs" value={firewallConfForm.tcp_out || ''} onChange={e => setFirewallConfForm({ ...firewallConfForm, tcp_out: e.target.value })} placeholder="20,21,22,25,53,80,110,113,443" />
                    </Field>
                    <Field label="UDP IN" title="Comma-separated list of allowed incoming UDP ports.">
                      <input className="input font-mono text-xs" value={firewallConfForm.udp_in || ''} onChange={e => setFirewallConfForm({ ...firewallConfForm, udp_in: e.target.value })} placeholder="20,21,53" />
                    </Field>
                    <Field label="UDP OUT" title="Comma-separated list of allowed outgoing UDP ports.">
                      <input className="input font-mono text-xs" value={firewallConfForm.udp_out || ''} onChange={e => setFirewallConfForm({ ...firewallConfForm, udp_out: e.target.value })} placeholder="20,21,53,113,123" />
                    </Field>
                  </div>

                  <div className="flex gap-2">
                    <button className="btn-primary" disabled={firewallBusy} onClick={saveFirewallConf} title="Save changes to csf.conf. Click Restart csf to apply.">Save Settings</button>
                    <button className="btn-secondary" disabled={firewallBusy} onClick={loadFirewallConf}>Reset</button>
                    <button className="btn-secondary" disabled={firewallBusy} onClick={restartFirewall} title="Apply saved settings by running csf -r.">Restart csf</button>
                  </div>
                </div>
              )}

              <div className="section-panel">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <h3 className="text-base font-semibold text-gray-950">Advanced: raw config file</h3>
                  <div className="flex flex-wrap gap-2">
                    <select className="input" value={firewallConfigName} onChange={e => setFirewallConfigName(e.target.value)}>
                      {['csf.conf', 'csf.allow', 'csf.deny', 'csf.ignore', 'csf.pignore'].map(name => <option key={name} value={name}>{name}</option>)}
                    </select>
                    <button className="btn-secondary" disabled={firewallBusy} onClick={() => loadFirewallConfig(firewallConfigName)}>Reload</button>
                    <button className="btn-primary" disabled={firewallBusy || !firewallConfigDirty} onClick={saveFirewallConfig} title="A timestamped backup is written under /var/backups/stack-manager-csf/ before overwriting.">Save</button>
                  </div>
                </div>
                <textarea
                  className="mt-2 w-full rounded border border-gray-200 bg-gray-950 p-3 font-mono text-xs text-gray-100"
                  rows={16}
                  spellCheck={false}
                  value={firewallConfigContent}
                  onChange={e => { setFirewallConfigContent(e.target.value); setFirewallConfigDirty(true); }}
                />
                {firewallConfigDirty && <div className="mt-1 text-xs text-amber-700">Unsaved changes.</div>}
              </div>

              <div className="section-panel">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <h3 className="text-base font-semibold text-gray-950">/var/log/lfd.log</h3>
                  <div className="flex flex-wrap items-center gap-2">
                    <label className="text-xs text-gray-600" title="Number of trailing lines to show. Cap 5000.">Lines
                      <input className="input ml-1 w-24" type="number" min={10} max={5000} value={firewallLogLines} onChange={e => setFirewallLogLines(Number(e.target.value) || 200)} />
                    </label>
                    <button className="btn-secondary" disabled={firewallBusy} onClick={() => loadFirewallLog(firewallLogLines)}>Refresh</button>
                  </div>
                </div>
                <pre className="mt-2 max-h-96 overflow-auto rounded bg-gray-950 p-3 font-mono text-xs text-gray-100 whitespace-pre-wrap break-all">{firewallLog || '(empty)'}</pre>
              </div>

              <div className="section-panel space-y-3">
                <h3 className="text-base font-semibold text-gray-950">CSF Documentation</h3>

                <div className="space-y-4 text-sm text-gray-700">
                  <div>
                    <h4 className="font-semibold text-gray-950">What is CSF?</h4>
                    <p className="mt-1">ConfigServer Security &amp; Firewall (CSF) is an iptables/nftables-based firewall for Linux. It manages INPUT, OUTPUT, and FORWARD chains to control which network traffic is allowed to and from your server. LFD (Login Failure Daemon) monitors log files for repeated failed login attempts and temporarily blocks offending IPs.</p>
                  </div>

                  <div>
                    <h4 className="font-semibold text-gray-950">Testing Mode vs Production</h4>
                    <p className="mt-1"><strong>Testing mode (TESTING=1)</strong> loads your firewall rules but automatically flushes them after 5 minutes. This is a safety net: if you lock yourself out with a bad rule, wait 5 minutes and your connection will be restored. Once your port rules are verified, uncheck Testing mode and click Restart csf to switch to production mode where rules persist permanently.</p>
                  </div>

                  <div>
                    <h4 className="font-semibold text-gray-950">Port Configuration</h4>
                    <ul className="mt-1 list-disc space-y-1 pl-5">
                      <li><strong>TCP IN</strong> — ports other machines connect TO on your server: SSH (22), HTTP (80), HTTPS (443), and any Stack Manager or project ports (8993, 3000, etc.).</li>
                      <li><strong>TCP OUT</strong> — ports your server connects OUT on: DNS (53), HTTP/HTTPS (80, 443), SMTP (25, 587), and Docker registry pulls.</li>
                      <li><strong>UDP IN/OUT</strong> — same concept for UDP. Typically DNS (53), NTP (123), and WireGuard (51820) if used.</li>
                    </ul>
                    <p className="mt-2">If a port is not listed in the appropriate direction, CSF blocks it. Always include your SSH port (usually 22) in TCP IN or you will lose remote access.</p>
                  </div>

                  <div>
                    <h4 className="font-semibold text-gray-950">CSF + Docker</h4>
                    <p className="mt-1">Docker and CSF both manage iptables rules. Without configuration, <code className="rounded bg-gray-100 px-1">csf -r</code> (restart) flushes Docker's NAT and FORWARD chains, breaking container networking. Stack Manager handles this automatically:</p>
                    <ul className="mt-1 list-disc space-y-1 pl-5">
                      <li><strong>DOCKER=1</strong> in csf.conf tells CSF to accommodate Docker's chains (auto-set during install).</li>
                      <li><strong>csfpost.sh</strong> runs <code className="rounded bg-gray-100 px-1">systemctl restart docker</code> after every <code className="rounded bg-gray-100 px-1">csf -r</code> so Docker cleanly re-creates its chains.</li>
                      <li><strong>csfpre.sh</strong> saves the iptables state before CSF flushes, as a safety backup.</li>
                    </ul>
                    <p className="mt-2">If container networking ever breaks after a CSF restart, run <code className="rounded bg-gray-100 px-1">systemctl restart docker</code> on the host (or use the Restart Docker button in Docker Settings).</p>
                  </div>

                  <div>
                    <h4 className="font-semibold text-gray-950">Allow / Deny Lists</h4>
                    <ul className="mt-1 list-disc space-y-1 pl-5">
                      <li><strong>csf.allow</strong> — IPs that are always permitted, regardless of port rules. Use for your admin IP, office range, or VPN exit.</li>
                      <li><strong>csf.deny</strong> — IPs that are always blocked. LFD adds entries here when it detects brute-force attacks.</li>
                      <li><strong>csf.ignore</strong> — IPs that LFD should never block (even if they trigger login failures). Useful for monitoring systems that generate auth errors.</li>
                    </ul>
                    <p className="mt-2">Stack Manager auto-allows the caller's IP on every successful dashboard login. This means you can always reach the dashboard from your current IP, even after tightening firewall rules.</p>
                  </div>

                  <div>
                    <h4 className="font-semibold text-gray-950">Common Scenarios</h4>
                    <div className="mt-1 overflow-x-auto">
                      <table className="w-full text-left text-xs">
                        <thead><tr className="border-b border-gray-200 text-gray-500"><th className="py-1 pr-3">Scenario</th><th className="py-1">Ports to add to TCP IN</th></tr></thead>
                        <tbody>
                          <tr className="border-b border-gray-100"><td className="py-1 pr-3">Web server (HTTP + HTTPS)</td><td className="py-1 font-mono">80, 443</td></tr>
                          <tr className="border-b border-gray-100"><td className="py-1 pr-3">Stack Manager default</td><td className="py-1 font-mono">8993 (or 443 if using standard ports)</td></tr>
                          <tr className="border-b border-gray-100"><td className="py-1 pr-3">Nginx Proxy Manager admin</td><td className="py-1 font-mono">81</td></tr>
                          <tr className="border-b border-gray-100"><td className="py-1 pr-3">Mail server</td><td className="py-1 font-mono">25, 110, 143, 465, 587, 993, 995</td></tr>
                          <tr className="border-b border-gray-100"><td className="py-1 pr-3">WireGuard VPN</td><td className="py-1 font-mono">51820 (UDP IN)</td></tr>
                          <tr className="border-b border-gray-100"><td className="py-1 pr-3">Docker project on custom port</td><td className="py-1 font-mono">the host-mapped port from docker compose</td></tr>
                        </tbody>
                      </table>
                    </div>
                  </div>

                  <div>
                    <h4 className="font-semibold text-gray-950">Troubleshooting</h4>
                    <ul className="mt-1 list-disc space-y-1 pl-5">
                      <li><strong>Locked out?</strong> If testing mode was on, wait 5 minutes. If not, access the server via console/IPMI and run <code className="rounded bg-gray-100 px-1">csf -x</code> to disable the firewall temporarily.</li>
                      <li><strong>Docker containers can't reach the internet?</strong> Run <code className="rounded bg-gray-100 px-1">systemctl restart docker</code> on the host. CSF may have flushed Docker's iptables chains.</li>
                      <li><strong>LFD keeps blocking legitimate IPs?</strong> Add them to csf.ignore (not csf.allow — ignore prevents the block; allow overrides it after the fact).</li>
                      <li><strong>High CPU from LFD?</strong> Set <code className="rounded bg-gray-100 px-1">PT_USERMEM</code> and <code className="rounded bg-gray-100 px-1">PT_USERTIME</code> to 0 in csf.conf to disable process tracking if you don't need it.</li>
                    </ul>
                  </div>

                  <div>
                    <h4 className="font-semibold text-gray-950">Host Setup</h4>
                    <p className="mt-1">CSF is managed from the dashboard but the host needs a one-time helper script install:</p>
                    <pre className="mt-2 rounded bg-gray-950 p-2 font-mono text-xs text-gray-100">sudo install -m 750 scripts/stack-manager-csf.sh /usr/local/sbin/stack-manager-csf</pre>
                    <p className="mt-2">After that, use the Install csf button above. The installer auto-configures Docker mode and creates the csfpre/csfpost scripts.</p>
                  </div>

                  <div>
                    <h4 className="font-semibold text-gray-950">Upstream</h4>
                    <p className="mt-1">Stack Manager uses the community fork at <a className="underline" href="https://github.com/Black-HOST/csf" target="_blank" rel="noreferrer">Black-HOST/csf</a>. The original ConfigServer development was discontinued in August 2025; the community fork continues active maintenance.</p>
                  </div>
                </div>
              </div>
            </>
          )}
        </div>
      )}
    </div>
  );
}

function Modal({ open, onClose, title, children, widthClass = 'max-w-xl' }) {
  if (!open) return null;
  return (
    <div className="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto bg-gray-950/60 p-4" onClick={onClose}>
      <div className={`mt-16 w-full ${widthClass} rounded-lg bg-white p-5 shadow-xl`} onClick={e => e.stopPropagation()}>
        <div className="mb-3 flex items-start justify-between gap-4">
          <h3 className="text-base font-semibold text-gray-950">{title}</h3>
          <button type="button" className="mini-button" onClick={onClose} aria-label="Close">Close</button>
        </div>
        {children}
      </div>
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

const WEEKDAY_LABELS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];

function formatCadence(schedule) {
  const tod = schedule.time_of_day || '00:00';
  switch (schedule.cadence) {
    case 'daily': return `Daily ${tod} UTC`;
    case 'weekly': return `${WEEKDAY_LABELS[schedule.day_of_week] || '?'} ${tod} UTC`;
    case 'monthly': return `${ordinal(schedule.day_of_month)} ${tod} UTC`;
    default: return `${schedule.interval_minutes}m`;
  }
}

function ordinal(n) {
  const s = ['th', 'st', 'nd', 'rd'];
  const v = n % 100;
  return n + (s[(v - 20) % 10] || s[v] || s[0]);
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
