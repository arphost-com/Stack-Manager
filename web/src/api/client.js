const BASE_URL = '/api/v1';

function getApiKey() {
  return localStorage.getItem('cm_api_key') || '';
}

function getToken() {
  return localStorage.getItem('cm_token') || '';
}

// Clears the local auth footprint (mirrors what Settings > Sign Out does) and
// bounces to the root URL, which the top-level App component will render as
// <Login /> because cm_token is gone. Guarded so it only fires once even if
// several parallel requests all get 401 at the same time.
let sessionExpiredHandled = false;
function handleSessionExpired() {
  if (sessionExpiredHandled) return;
  sessionExpiredHandled = true;
  try {
    localStorage.removeItem('cm_token');
    localStorage.removeItem('cm_user');
    localStorage.removeItem('cm_api_key');
    // Snapshot caches would show stale data next login as a different user.
    Object.keys(localStorage).filter(k => k.startsWith('cm_dashboard_v')).forEach(k => localStorage.removeItem(k));
  } catch {}
  // Hard redirect so every in-flight React state gets torn down cleanly.
  window.location.href = '/';
}

async function request(path, options = {}) {
  const token = getToken();
  const apiKey = getApiKey();
  const authHeaders = token ? { Authorization: `Bearer ${token}` } : apiKey ? { 'X-API-Key': apiKey } : {};
  const res = await fetch(`${BASE_URL}${path}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...authHeaders,
      ...options.headers,
    },
  });

  if (res.status === 401 && path !== '/auth/login') {
    handleSessionExpired();
    // Still throw so the calling code knows the request didn't succeed. The
    // page will unmount as soon as location.href fires, so the error will
    // usually never render.
    throw new Error('Session expired. Please sign in again.');
  }

  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    // Attach the full response envelope to the thrown Error so callers can
    // display richer detail (compose output, exit code, etc.) instead of
    // just the top-level message. Handlers that only touch err.message keep
    // working; ones that render an output pane can read err.data.
    const err = new Error(body.error || `HTTP ${res.status}`);
    err.data = body?.data;
    err.status = res.status;
    err.envelope = body;
    throw err;
  }

  return res.json();
}

async function download(path, filename) {
  const token = getToken();
  const apiKey = getApiKey();
  const authHeaders = token ? { Authorization: `Bearer ${token}` } : apiKey ? { 'X-API-Key': apiKey } : {};
  const res = await fetch(`${BASE_URL}${path}`, { headers: authHeaders });
  if (res.status === 401) {
    handleSessionExpired();
    throw new Error('Session expired. Please sign in again.');
  }
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `HTTP ${res.status}`);
  }
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}

export const totp = {
  enroll: () => request('/auth/totp/enroll', { method: 'POST' }),
  verify: (code) => request('/auth/totp/verify', { method: 'POST', body: JSON.stringify({ code }) }),
  disable: (password) => request('/auth/totp/disable', { method: 'POST', body: JSON.stringify({ password }) }),
  login: (totpToken, code) => request('/auth/totp/login', { method: 'POST', body: JSON.stringify({ totp_token: totpToken, code }) }),
  resetUser: (username) => request(`/users/${username}/totp`, { method: 'DELETE' }),
};

export const auth = {
  login: (body) => request('/auth/login', { method: 'POST', body: JSON.stringify(body) }),
  logout: () => request('/auth/logout', { method: 'POST' }),
  me: () => request('/auth/me'),
  changePassword: (current, next) => request('/auth/change-password', {
    method: 'POST',
    body: JSON.stringify({ current_password: current, new_password: next }),
  }),
};

export const users = {
  list: () => request('/users'),
  create: (body) => request('/users', { method: 'POST', body: JSON.stringify(body) }),
  setPassword: (username, password) => request(`/users/${encodeURIComponent(username)}/password`, { method: 'PUT', body: JSON.stringify({ password }) }),
  delete: (username) => request(`/users/${encodeURIComponent(username)}`, { method: 'DELETE' }),
};

// makeProjects builds the project API against a path prefix. The default
// prefix ('') targets the local controller. A peer-scoped prefix
// ('/agent-proxy/<agentId>') routes every call through the controller's
// agent-proxy, which forwards to the peer's /api/v1 with the peer's API key —
// so a peer's projects are viewable and manageable from this dashboard.
function makeProjects(prefix = '') {
  const p = (path) => `${prefix}${path}`;
  return {
    list: (params = {}) => {
      const qs = new URLSearchParams(params).toString();
      return request(p(`/projects${qs ? '?' + qs : ''}`));
    },
    create: (body) => request(p('/projects'), { method: 'POST', body: JSON.stringify(body) }),
    delete: (name, body) => request(p(`/projects/${encodeURIComponent(name)}`), { method: 'DELETE', body: JSON.stringify(body) }),
    get: (name) => request(p(`/projects/${encodeURIComponent(name)}`)),
    docs: (name) => request(p(`/projects/${encodeURIComponent(name)}/docs`)),
    docContent: (name, path) => request(p(`/projects/${encodeURIComponent(name)}/docs/content?path=${encodeURIComponent(path)}`)),
    images: (name) => request(p(`/projects/${encodeURIComponent(name)}/images`)),
    volumes: (name) => request(p(`/projects/${encodeURIComponent(name)}/volumes`)),
    deleteVolume: (name, vol) => request(p(`/projects/${encodeURIComponent(name)}/volumes/${encodeURIComponent(vol)}`), { method: 'DELETE' }),
    networks: (name) => request(p(`/projects/${encodeURIComponent(name)}/networks`)),
    deleteNetwork: (name, net) => request(p(`/projects/${encodeURIComponent(name)}/networks/${encodeURIComponent(net)}`), { method: 'DELETE' }),
    status: (name) => request(p(`/projects/${encodeURIComponent(name)}/status`)),
    pull: (name, timeout) => request(p(`/projects/${encodeURIComponent(name)}/pull${timeout ? '?timeout=' + timeout : ''}`), { method: 'POST' }),
    up: (name) => request(p(`/projects/${encodeURIComponent(name)}/up`), { method: 'POST' }),
    down: (name) => request(p(`/projects/${encodeURIComponent(name)}/down`), { method: 'POST' }),
    update: (name, timeout) => request(p(`/projects/${encodeURIComponent(name)}/update${timeout ? '?timeout=' + timeout : ''}`), { method: 'POST' }),
    restart: (name) => request(p(`/projects/${encodeURIComponent(name)}/restart`), { method: 'POST' }),
    startJob: (name, action, timeout) => request(p(`/projects/${encodeURIComponent(name)}/jobs/${encodeURIComponent(action)}${timeout ? '?timeout=' + timeout : ''}`), { method: 'POST' }),
    updatePolicy: (name) => request(p(`/projects/${encodeURIComponent(name)}/update-policy`)),
    setUpdatePolicy: (name, body) => request(p(`/projects/${encodeURIComponent(name)}/update-policy`), { method: 'PUT', body: JSON.stringify(body) }),
    setInactive: (name, inactive) => request(p(`/projects/${encodeURIComponent(name)}/inactive`), { method: 'PUT', body: JSON.stringify({ inactive }) }),
    bulk: (action, body) => request(p(`/projects/bulk/${action}`), { method: 'POST', body: JSON.stringify(body) }),
    files: (name) => request(p(`/projects/${encodeURIComponent(name)}/files`)),
    fileContent: (name, path) => request(p(`/projects/${encodeURIComponent(name)}/files/content?path=${encodeURIComponent(path)}`)),
    saveFile: (name, path, content) => request(p(`/projects/${encodeURIComponent(name)}/files/content`), { method: 'PUT', body: JSON.stringify({ path, content }) }),
  };
}

export const projects = makeProjects('');

// projectsForSource returns a project API scoped to a peer/agent by numeric id.
// Pass a falsy id (or 'local') to get the local controller API unchanged.
export function projectsForSource(agentId) {
  if (!agentId) return projects;
  return makeProjects(`/agent-proxy/${agentId}`);
}

export const updates = {
  check: () => request('/updates/check', { method: 'POST' }),
};

export const jobs = {
  list: () => request('/jobs'),
  get: (id) => request(`/jobs/${id}`),
};

// jobsForSource scopes job polling to a peer/agent so actions started on a peer
// (which return that peer's job id) can be tracked to completion.
export function jobsForSource(agentId) {
  if (!agentId) return jobs;
  const p = (path) => `/agent-proxy/${agentId}${path}`;
  return {
    list: () => request(p('/jobs')),
    get: (id) => request(p(`/jobs/${id}`)),
  };
}

export const stackTemplates = {
  list: () => request('/stack-templates'),
  get: (id) => request(`/stack-templates/${encodeURIComponent(id)}`),
};

export const agents = {
  list: () => request('/agents'),
  save: (body) => request('/agents', { method: 'POST', body: JSON.stringify(body) }),
  projects: (id) => request(`/agents/${id}/projects`),
  delete: (id) => request(`/agents/${id}`, { method: 'DELETE' }),
  // Callback (outbound-only) agents can't be called directly; queue a command
  // for them to run on their next check-in, and read the queue's status.
  enqueueCommand: (id, body) => request(`/agents/${id}/commands`, { method: 'POST', body: JSON.stringify(body) }),
  commands: (id, project) => request(`/agents/${id}/commands${project ? '?project=' + encodeURIComponent(project) : ''}`),
};

export const schedules = {
  list: () => request('/schedules'),
  save: (body) => request('/schedules', { method: 'POST', body: JSON.stringify(body) }),
  delete: (id) => request(`/schedules/${id}`, { method: 'DELETE' }),
  run: (id) => request(`/schedules/${id}/run`, { method: 'POST' }),
};

export const metrics = {
  summary: () => request('/metrics/summary'),
  history: (hours = 24, project = '') => {
    const params = new URLSearchParams({ hours: String(hours) });
    if (project) params.set('project', project);
    return request(`/metrics/history?${params}`);
  },
  backupActivity: (hours = 24) => request(`/metrics/backup-activity?hours=${hours}`),
  refresh: () => request('/metrics/refresh', { method: 'POST' }),
};

export const skills = {
  list: () => request('/skills'),
  get: (name) => request(`/skills/${name}`),
};

export const security = {
  scan: (name) => request(`/skills/security/scan/${encodeURIComponent(name)}`),
  audit: (name) => request(`/skills/security/audit/${encodeURIComponent(name)}`),
  report: () => request('/skills/security/report'),
};

export const debug = {
  logs: (name, tail = 100, container) => {
    const params = new URLSearchParams({ tail: String(tail) });
    if (container) params.set('container', container);
    return request(`/skills/debug/logs/${encodeURIComponent(name)}?${params}`);
  },
  shell: (name, body) => request(`/skills/debug/shell/${encodeURIComponent(name)}`, { method: 'POST', body: JSON.stringify(body) }),
  inspect: (name) => request(`/skills/debug/inspect/${encodeURIComponent(name)}`),
  stats: (name) => request(`/skills/debug/stats/${encodeURIComponent(name)}`),
  events: (since = '1h', limit = 50) => request(`/skills/debug/events?since=${since}&limit=${limit}`),
  top: (name) => request(`/skills/debug/top/${encodeURIComponent(name)}`),
};

export const backup = {
  create: (name, body = {}) => request(`/skills/backup/create/${encodeURIComponent(name)}`, { method: 'POST', body: JSON.stringify(body) }),
  list: () => request('/skills/backup/list'),
  listProject: (name) => request(`/skills/backup/list/${encodeURIComponent(name)}`),
  download: (id) => download(`/skills/backup/download/${encodeURIComponent(id)}`, id),
  restore: (name, backupId) => request(`/skills/backup/restore/${encodeURIComponent(name)}/${encodeURIComponent(backupId)}`, { method: 'POST' }),
  delete: (id) => request(`/skills/backup/${encodeURIComponent(id)}`, { method: 'DELETE' }),
  destinations: () => request('/skills/backup/destinations'),
  saveDestination: (body) => request('/skills/backup/destinations', { method: 'POST', body: JSON.stringify(body) }),
  deleteDestination: (id) => request(`/skills/backup/destinations/${id}`, { method: 'DELETE' }),
  testDestination: (id) => request(`/skills/backup/destinations/${id}/test`, { method: 'POST' }),
  destinationPublicKey: (id) => request(`/skills/backup/destinations/${id}/public-key`),
  generateSSHKey: () => request('/skills/backup/keys/generate', { method: 'POST' }),
  schedules: () => request('/skills/backup/schedules'),
  saveSchedule: (body) => request('/skills/backup/schedules', { method: 'POST', body: JSON.stringify(body) }),
  deleteSchedule: (id) => request(`/skills/backup/schedules/${id}`, { method: 'DELETE' }),
  runSchedule: (id) => request(`/skills/backup/schedules/${id}/run`, { method: 'POST' }),
};

export const dbadmin = {
  discover: () => request('/skills/dbadmin/discover'),
  discoverProject: (name) => request(`/skills/dbadmin/discover/${encodeURIComponent(name)}`),
  health: (name) => request(`/skills/dbadmin/health/${encodeURIComponent(name)}`),
  dump: (name) => request(`/skills/dbadmin/dump/${encodeURIComponent(name)}`, { method: 'POST' }),
  dumps: () => request('/skills/dbadmin/dumps'),
  projectDumps: (name) => request(`/skills/dbadmin/dumps/${encodeURIComponent(name)}`),
};

export const registries = {
  login: (body) => request('/registries/login', { method: 'POST', body: JSON.stringify(body) }),
  list: () => request('/registries'),
  delete: (registry) => request(`/registries/${encodeURIComponent(registry)}`, { method: 'DELETE' }),
};

export const dockerSettings = {
  daemon: () => request('/docker/daemon'),
  saveDaemon: (body) => request('/docker/daemon', { method: 'PUT', body: JSON.stringify(body) }),
  restartDocker: () => request('/docker/restart', { method: 'POST' }),
};

export const proxy = {
  status: () => request('/proxy/status'),
  deploy: () => request('/proxy/deploy', { method: 'POST' }),
  configure: (url, email, password) => request('/proxy/configure', { method: 'POST', body: JSON.stringify({ url, email, password }) }),
  listHosts: () => request('/proxy/hosts'),
  createHost: (body) => request('/proxy/hosts', { method: 'POST', body: JSON.stringify(body) }),
  deleteHost: (id) => request(`/proxy/hosts?id=${id}`, { method: 'DELETE' }),
  suggestions: () => request('/proxy/suggestions'),
};

// systemForSource scopes host-level system commands (prune, gpu) to a peer/agent
// by id, routing through the controller's agent-proxy. Pass a falsy id for the
// local controller.
export function systemForSource(agentId) {
  if (!agentId) return system;
  const p = (path) => `/agent-proxy/${agentId}${path}`;
  return {
    gpu: () => request(p('/system/gpu')),
    prune: (mode = 'safe') => request(p('/prune'), { method: 'POST', body: JSON.stringify({ mode }) }),
  };
}

export const system = {
  gpu: () => request('/system/gpu'),
  gpuTest: () => request('/system/gpu/test', { method: 'POST' }),
  gpuSetupStatus: () => request('/system/gpu/setup'),
  gpuSetupInstall: () => request('/system/gpu/setup/install', { method: 'POST' }),
  gpuSetupUninstall: () => request('/system/gpu/setup/uninstall', { method: 'POST' }),
  gpuSetupReboot: () => request('/system/gpu/setup/reboot', { method: 'POST' }),
  osStatus: () => request('/system/os/status'),
  osUpgrade: () => request('/system/os/upgrade', { method: 'POST' }),
  osAutoremove: () => request('/system/os/autoremove', { method: 'POST' }),
  osSearch: (q) => request(`/system/os/search?q=${encodeURIComponent(q)}`),
  osInstall: (pkg) => request('/system/os/install', { method: 'POST', body: JSON.stringify({ package: pkg }) }),
  updateStatus: () => request('/system/update/status'),
  selfUpdate: () => request('/system/update', { method: 'POST' }),
  info: () => request('/system/info'),
  setName: (name) => request('/system/info', { method: 'PUT', body: JSON.stringify({ server_name: name }) }),
  tzStatus: () => request('/system/tz'),
  setTz: (tz) => request('/system/tz', { method: 'POST', body: JSON.stringify({ tz }) }),
  prune: (mode = 'safe') => request('/prune', { method: 'POST', body: JSON.stringify({ mode }) }),
};

export const envSettings = {
  get: () => request('/settings/env'),
  save: (body) => request('/settings/env', { method: 'PUT', body: JSON.stringify(body) }),
  rollAPIKey: () => request('/settings/env/roll-api-key', { method: 'POST' }),
};

export const ssl = {
  get: () => request('/settings/ssl'),
  regenerateSelfSigned: (body) => request('/settings/ssl/self-signed', { method: 'POST', body: JSON.stringify(body) }),
  enableLetsEncrypt: (body) => request('/settings/ssl/letsencrypt', { method: 'POST', body: JSON.stringify(body) }),
  renewLetsEncrypt: () => request('/settings/ssl/letsencrypt/renew', { method: 'POST' }),
};

export const firewall = {
  status: () => request('/skills/firewall/status'),
  version: () => request('/skills/firewall/version'),
  install: () => request('/skills/firewall/install', { method: 'POST' }),
  uninstall: () => request('/skills/firewall/uninstall', { method: 'POST', body: JSON.stringify({ confirm: 'UNINSTALL' }) }),
  restart: () => request('/skills/firewall/restart', { method: 'POST' }),
  reloadLFD: () => request('/skills/firewall/reload-lfd', { method: 'POST' }),
  listAllow: () => request('/skills/firewall/allow'),
  listDeny: () => request('/skills/firewall/deny'),
  listTempbans: () => request('/skills/firewall/tempbans'),
  allowIP: (ip, comment) => request('/skills/firewall/ips/allow', { method: 'POST', body: JSON.stringify({ ip, comment }) }),
  denyIP: (ip, comment) => request('/skills/firewall/ips/deny', { method: 'POST', body: JSON.stringify({ ip, comment }) }),
  allowPorts: (ports, proto = 'tcp') => request('/skills/firewall/ports/allow', { method: 'POST', body: JSON.stringify({ ports, proto }) }),
  removeIP: (ip) => request(`/skills/firewall/ips/${encodeURIComponent(ip)}`, { method: 'DELETE' }),
  readConfig: (name) => request(`/skills/firewall/config/${encodeURIComponent(name)}`),
  writeConfig: (name, content) => request(`/skills/firewall/config/${encodeURIComponent(name)}`, { method: 'PUT', body: JSON.stringify({ content }) }),
  tailLog: (lines = 200) => request(`/skills/firewall/log?lines=${encodeURIComponent(lines)}`),
  confSettings: () => request('/skills/firewall/conf-settings'),
  saveConfSettings: (body) => request('/skills/firewall/conf-settings', { method: 'PUT', body: JSON.stringify(body) }),
  clientIP: () => request('/skills/firewall/client-ip'),
  allowMyIP: () => request('/skills/firewall/allow-my-ip', { method: 'POST' }),
};

// Watch = Up + persistent live-tail startup log. Sessions are stored on disk
// so a browser refresh mid-stream replays from where it left off.
export const watch = {
  start: (name) => request(`/projects/${encodeURIComponent(name)}/watch`, { method: 'POST' }),
  list: (name) => request(`/projects/${encodeURIComponent(name)}/watch`),
  get: (name, sessionId) => request(`/projects/${encodeURIComponent(name)}/watch/${encodeURIComponent(sessionId)}`),
  stop: (name, sessionId) => request(`/projects/${encodeURIComponent(name)}/watch/${encodeURIComponent(sessionId)}`, { method: 'DELETE' }),
  // The stream URL is opened directly via EventSource, which cannot set
  // custom headers. Auth flows through the session cookie set at login;
  // if the caller is using the legacy X-API-Key, pass it as ?api_key=.
  streamUrl: (name, sessionId, apiKey) => {
    const key = apiKey || localStorage.getItem('cm_api_key') || '';
    const qs = key ? `?api_key=${encodeURIComponent(key)}` : '';
    return `/api/v1/projects/${encodeURIComponent(name)}/watch/${encodeURIComponent(sessionId)}/stream${qs}`;
  },
};

export const audit = {
  list: (params = {}) => {
    const qs = new URLSearchParams(Object.entries(params).filter(([, v]) => v !== '' && v !== undefined && v !== null)).toString();
    return request(`/audit${qs ? '?' + qs : ''}`);
  },
  nodes: () => request('/audit/nodes'),
  actions: () => request('/audit/actions'),
};
