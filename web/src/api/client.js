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
    throw new Error(body.error || `HTTP ${res.status}`);
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

export const auth = {
  login: (body) => request('/auth/login', { method: 'POST', body: JSON.stringify(body) }),
  logout: () => request('/auth/logout', { method: 'POST' }),
  me: () => request('/auth/me'),
};

export const users = {
  list: () => request('/users'),
  create: (body) => request('/users', { method: 'POST', body: JSON.stringify(body) }),
  setPassword: (username, password) => request(`/users/${encodeURIComponent(username)}/password`, { method: 'PUT', body: JSON.stringify({ password }) }),
  delete: (username) => request(`/users/${encodeURIComponent(username)}`, { method: 'DELETE' }),
};

export const projects = {
  list: (params = {}) => {
    const qs = new URLSearchParams(params).toString();
    return request(`/projects${qs ? '?' + qs : ''}`);
  },
  create: (body) => request('/projects', { method: 'POST', body: JSON.stringify(body) }),
  delete: (name, body) => request(`/projects/${name}`, { method: 'DELETE', body: JSON.stringify(body) }),
  get: (name) => request(`/projects/${name}`),
  images: (name) => request(`/projects/${name}/images`),
  status: (name) => request(`/projects/${name}/status`),
  pull: (name, timeout) => request(`/projects/${name}/pull${timeout ? '?timeout=' + timeout : ''}`, { method: 'POST' }),
  up: (name) => request(`/projects/${name}/up`, { method: 'POST' }),
  down: (name) => request(`/projects/${name}/down`, { method: 'POST' }),
  update: (name, timeout) => request(`/projects/${name}/update${timeout ? '?timeout=' + timeout : ''}`, { method: 'POST' }),
  restart: (name) => request(`/projects/${name}/restart`, { method: 'POST' }),
  startJob: (name, action, timeout) => request(`/projects/${name}/jobs/${action}${timeout ? '?timeout=' + timeout : ''}`, { method: 'POST' }),
  updatePolicy: (name) => request(`/projects/${name}/update-policy`),
  setUpdatePolicy: (name, body) => request(`/projects/${name}/update-policy`, { method: 'PUT', body: JSON.stringify(body) }),
  setInactive: (name, inactive) => request(`/projects/${name}/inactive`, { method: 'PUT', body: JSON.stringify({ inactive }) }),
  bulk: (action, body) => request(`/projects/bulk/${action}`, { method: 'POST', body: JSON.stringify(body) }),
};

export const jobs = {
  list: () => request('/jobs'),
  get: (id) => request(`/jobs/${id}`),
};

export const stackTemplates = {
  list: () => request('/stack-templates'),
  get: (id) => request(`/stack-templates/${encodeURIComponent(id)}`),
};

export const agents = {
  list: () => request('/agents'),
  save: (body) => request('/agents', { method: 'POST', body: JSON.stringify(body) }),
  projects: (id) => request(`/agents/${id}/projects`),
  delete: (id) => request(`/agents/${id}`, { method: 'DELETE' }),
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
  scan: (name) => request(`/skills/security/scan/${name}`),
  audit: (name) => request(`/skills/security/audit/${name}`),
  report: () => request('/skills/security/report'),
};

export const debug = {
  logs: (name, tail = 100, container) => {
    const params = new URLSearchParams({ tail: String(tail) });
    if (container) params.set('container', container);
    return request(`/skills/debug/logs/${name}?${params}`);
  },
  shell: (name, body) => request(`/skills/debug/shell/${name}`, { method: 'POST', body: JSON.stringify(body) }),
  inspect: (name) => request(`/skills/debug/inspect/${name}`),
  stats: (name) => request(`/skills/debug/stats/${name}`),
  events: (since = '1h', limit = 50) => request(`/skills/debug/events?since=${since}&limit=${limit}`),
  top: (name) => request(`/skills/debug/top/${name}`),
};

export const backup = {
  create: (name, body = {}) => request(`/skills/backup/create/${name}`, { method: 'POST', body: JSON.stringify(body) }),
  list: () => request('/skills/backup/list'),
  listProject: (name) => request(`/skills/backup/list/${name}`),
  download: (id) => download(`/skills/backup/download/${encodeURIComponent(id)}`, id),
  restore: (name, backupId) => request(`/skills/backup/restore/${name}/${backupId}`, { method: 'POST' }),
  delete: (id) => request(`/skills/backup/${id}`, { method: 'DELETE' }),
  destinations: () => request('/skills/backup/destinations'),
  saveDestination: (body) => request('/skills/backup/destinations', { method: 'POST', body: JSON.stringify(body) }),
  deleteDestination: (id) => request(`/skills/backup/destinations/${id}`, { method: 'DELETE' }),
  testDestination: (id) => request(`/skills/backup/destinations/${id}/test`, { method: 'POST' }),
  schedules: () => request('/skills/backup/schedules'),
  saveSchedule: (body) => request('/skills/backup/schedules', { method: 'POST', body: JSON.stringify(body) }),
  deleteSchedule: (id) => request(`/skills/backup/schedules/${id}`, { method: 'DELETE' }),
  runSchedule: (id) => request(`/skills/backup/schedules/${id}/run`, { method: 'POST' }),
};

export const dbadmin = {
  discover: () => request('/skills/dbadmin/discover'),
  discoverProject: (name) => request(`/skills/dbadmin/discover/${name}`),
  health: (name) => request(`/skills/dbadmin/health/${name}`),
  dump: (name) => request(`/skills/dbadmin/dump/${name}`, { method: 'POST' }),
  dumps: () => request('/skills/dbadmin/dumps'),
  projectDumps: (name) => request(`/skills/dbadmin/dumps/${name}`),
};

export const system = {
  prune: (mode = 'safe') => request('/prune', { method: 'POST', body: JSON.stringify({ mode }) }),
};

export const registries = {
  login: (body) => request('/registries/login', { method: 'POST', body: JSON.stringify(body) }),
};

export const dockerSettings = {
  daemon: () => request('/docker/daemon'),
  saveDaemon: (body) => request('/docker/daemon', { method: 'PUT', body: JSON.stringify(body) }),
};
