import { useEffect, useState } from 'react';
import { auth, users, backup } from '../api/client';

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

export default function Settings() {
  const [me, setMe] = useState(null);
  const [userList, setUserList] = useState([]);
  const [destinationList, setDestinationList] = useState([]);
  const [message, setMessage] = useState('');
  const [error, setError] = useState('');
  const [form, setForm] = useState({ username: '', password: '', role: 'operator' });
  const [resetForm, setResetForm] = useState({ username: '', password: '' });
  const [destinationForm, setDestinationForm] = useState(emptyDestinationForm);

  const load = async () => {
    setError('');
    try {
      const meRes = await auth.me();
      setMe(meRes.data);
      if (meRes.data.role === 'admin') {
        const [usersRes, destinationsRes] = await Promise.all([users.list(), backup.destinations()]);
        setUserList(usersRes.data || []);
        setDestinationList(destinationsRes.data || []);
      }
    } catch (err) {
      setError(err.message);
    }
  };

  useEffect(() => { load(); }, []);

  const logout = async () => {
    await auth.logout().catch(() => {});
    localStorage.removeItem('cm_token');
    localStorage.removeItem('cm_user');
    localStorage.removeItem('cm_api_key');
    window.location.href = '/';
  };

  const createUser = async (event) => {
    event.preventDefault();
    setError('');
    setMessage('');
    try {
      await users.create(form);
      setMessage(`Created user ${form.username}`);
      setForm({ username: '', password: '', role: 'operator' });
      load();
    } catch (err) {
      setError(err.message);
    }
  };

  const resetPassword = async (event) => {
    event.preventDefault();
    setError('');
    setMessage('');
    try {
      await users.setPassword(resetForm.username, resetForm.password);
      setMessage(`Updated password for ${resetForm.username}`);
      setResetForm({ username: '', password: '' });
    } catch (err) {
      setError(err.message);
    }
  };

  const deleteUser = async (username) => {
    setError('');
    setMessage('');
    try {
      await users.delete(username);
      setMessage(`Deleted user ${username}`);
      load();
    } catch (err) {
      setError(err.message);
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
    setError('');
    setMessage('');
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
      setMessage(`Saved backup destination ${res.data.name}`);
      setDestinationForm(emptyDestinationForm);
      load();
    } catch (err) {
      setError(err.message);
    }
  };

  const testDestination = async (destination) => {
    setError('');
    setMessage('');
    try {
      const res = await backup.testDestination(destination.id);
      setMessage(`Backup destination ${destination.name} tested: ${res.data.success ? 'success' : res.data.error || 'failed'}`);
    } catch (err) {
      setError(err.message);
    }
  };

  const deleteDestination = async (destination) => {
    setError('');
    setMessage('');
    try {
      await backup.deleteDestination(destination.id);
      setMessage(`Deleted backup destination ${destination.name}`);
      load();
    } catch (err) {
      setError(err.message);
    }
  };

  return (
    <div className="mx-auto max-w-5xl space-y-6">
      <div className="section-panel">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h1 className="text-xl font-semibold text-gray-950">Account</h1>
            <p className="mt-1 text-sm text-gray-600">{me ? `${me.username} (${me.role})` : 'Loading account...'}</p>
          </div>
          <button onClick={logout} className="btn-secondary">Sign Out</button>
        </div>
      </div>

      {error && <div className="rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-800">{error}</div>}
      {message && <div className="rounded-md border border-green-200 bg-green-50 p-3 text-sm text-green-800">{message}</div>}

      {me?.role === 'admin' && (
        <div className="grid gap-6 lg:grid-cols-[1fr_380px]">
          <div className="section-panel">
            <h2 className="mb-3 text-lg font-semibold text-gray-950">Users</h2>
            <div className="overflow-x-auto">
              <table className="w-full min-w-[620px] text-left text-sm">
                <thead>
                  <tr className="border-b border-gray-200 text-xs uppercase text-gray-500">
                    <th className="py-2">Username</th>
                    <th>Role</th>
                    <th>Created</th>
                    <th className="text-right">Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {userList.map(user => (
                    <tr key={user.username} className="border-b border-gray-100">
                      <td className="py-2 font-medium">{user.username}</td>
                      <td>{user.role}</td>
                      <td className="text-gray-500">{new Date(user.created_at).toLocaleString()}</td>
                      <td className="text-right">
                        <button onClick={() => deleteUser(user.username)} className="mini-danger" title="Delete this user. The last admin cannot be deleted.">Delete</button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>

          <div className="space-y-4">
            <form onSubmit={createUser} className="section-panel space-y-3">
              <h2 className="text-lg font-semibold text-gray-950">Add User</h2>
              <input className="input" placeholder="username" value={form.username} onChange={e => setForm({ ...form, username: e.target.value })} required />
              <input className="input" type="password" placeholder="password, 12+ chars" value={form.password} onChange={e => setForm({ ...form, password: e.target.value })} required />
              <select className="input" value={form.role} onChange={e => setForm({ ...form, role: e.target.value })}>
                <option value="operator">operator</option>
                <option value="admin">admin</option>
              </select>
              <button className="btn-primary w-full">Create User</button>
            </form>

            <form onSubmit={resetPassword} className="section-panel space-y-3">
              <h2 className="text-lg font-semibold text-gray-950">Reset Password</h2>
              <input className="input" placeholder="username" value={resetForm.username} onChange={e => setResetForm({ ...resetForm, username: e.target.value })} required />
              <input className="input" type="password" placeholder="new password, 12+ chars" value={resetForm.password} onChange={e => setResetForm({ ...resetForm, password: e.target.value })} required />
              <button className="btn-secondary w-full">Update Password</button>
            </form>
          </div>

          <div className="section-panel lg:col-span-2">
            <div className="mb-4 flex flex-col gap-1">
              <h2 className="text-lg font-semibold text-gray-950">Backup Endpoints</h2>
              <p className="text-sm text-gray-600">Configure local, mounted, FTP, SFTP, and S3 destinations. CIFS and NFS should be mounted on the host and exposed to the server container as paths.</p>
            </div>
            <div className="grid gap-6 xl:grid-cols-[1fr_420px]">
              <div className="overflow-x-auto">
                <table className="w-full min-w-[760px] text-left text-sm">
                  <thead>
                    <tr className="border-b border-gray-200 text-xs uppercase text-gray-500">
                      <th className="py-2">Name</th>
                      <th>Type</th>
                      <th>Path / Target</th>
                      <th>Status</th>
                      <th className="text-right">Actions</th>
                    </tr>
                  </thead>
                  <tbody>
                    {destinationList.map(destination => (
                      <tr key={destination.id} className="border-b border-gray-100 align-top">
                        <td className="py-2 font-medium">{destination.name}</td>
                        <td>{destination.type}</td>
                        <td className="font-mono text-xs text-gray-600">{destinationSummary(destination)}</td>
                        <td>{destination.enabled ? 'enabled' : 'disabled'}{destination.has_secret ? ' · secret saved' : ''}</td>
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
        </div>
      )}
    </div>
  );
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
