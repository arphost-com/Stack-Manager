const BASE_URL = '/api/v1';

function getApiKey() {
  return localStorage.getItem('cm_api_key') || '';
}

function getToken() {
  return localStorage.getItem('cm_token') || '';
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

  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `HTTP ${res.status}`);
  }

  return res.json();
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
  get: (name) => request(`/projects/${name}`),
  images: (name) => request(`/projects/${name}/images`),
  status: (name) => request(`/projects/${name}/status`),
  pull: (name, timeout) => request(`/projects/${name}/pull${timeout ? '?timeout=' + timeout : ''}`, { method: 'POST' }),
  up: (name) => request(`/projects/${name}/up`, { method: 'POST' }),
  down: (name) => request(`/projects/${name}/down`, { method: 'POST' }),
  update: (name, timeout) => request(`/projects/${name}/update${timeout ? '?timeout=' + timeout : ''}`, { method: 'POST' }),
  restart: (name) => request(`/projects/${name}/restart`, { method: 'POST' }),
  startJob: (name, action, timeout) => request(`/projects/${name}/jobs/${action}${timeout ? '?timeout=' + timeout : ''}`, { method: 'POST' }),
  setInactive: (name, inactive) => request(`/projects/${name}/inactive`, { method: 'PUT', body: JSON.stringify({ inactive }) }),
  bulk: (action, body) => request(`/projects/bulk/${action}`, { method: 'POST', body: JSON.stringify(body) }),
};

export const jobs = {
  list: () => request('/jobs'),
  get: (id) => request(`/jobs/${id}`),
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
  inspect: (name) => request(`/skills/debug/inspect/${name}`),
  stats: (name) => request(`/skills/debug/stats/${name}`),
  events: (since = '1h', limit = 50) => request(`/skills/debug/events?since=${since}&limit=${limit}`),
  top: (name) => request(`/skills/debug/top/${name}`),
};

export const backup = {
  create: (name) => request(`/skills/backup/create/${name}`, { method: 'POST' }),
  list: () => request('/skills/backup/list'),
  listProject: (name) => request(`/skills/backup/list/${name}`),
  restore: (name, backupId) => request(`/skills/backup/restore/${name}/${backupId}`, { method: 'POST' }),
  delete: (id) => request(`/skills/backup/${id}`, { method: 'DELETE' }),
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
  prune: () => request('/prune', { method: 'POST' }),
};

export const registries = {
  login: (body) => request('/registries/login', { method: 'POST', body: JSON.stringify(body) }),
};
