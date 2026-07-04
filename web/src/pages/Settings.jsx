import { useEffect, useState } from 'react';
import { auth, users } from '../api/client';

export default function Settings() {
  const [me, setMe] = useState(null);
  const [userList, setUserList] = useState([]);
  const [message, setMessage] = useState('');
  const [error, setError] = useState('');
  const [form, setForm] = useState({ username: '', password: '', role: 'operator' });
  const [resetForm, setResetForm] = useState({ username: '', password: '' });

  const load = async () => {
    setError('');
    try {
      const meRes = await auth.me();
      setMe(meRes.data);
      if (meRes.data.role === 'admin') {
        const usersRes = await users.list();
        setUserList(usersRes.data || []);
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
        <div className="grid gap-6 lg:grid-cols-[1fr_360px]">
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
        </div>
      )}
    </div>
  );
}
