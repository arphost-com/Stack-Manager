import { useState } from 'react';
import { auth } from '../api/client';

export default function Login() {
  const [form, setForm] = useState({ username: 'admin', password: '' });
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  const submit = async (event) => {
    event.preventDefault();
    setLoading(true);
    setError('');
    try {
      const res = await auth.login(form);
      localStorage.setItem('cm_token', res.data.token);
      localStorage.setItem('cm_user', JSON.stringify(res.data.user));
      localStorage.removeItem('cm_api_key');
      window.location.href = '/';
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-gray-50 px-6 py-12 text-gray-950">
      <form onSubmit={submit} className="mx-auto mt-12 max-w-md section-panel space-y-4">
        <div>
          <h1 className="text-xl font-semibold">Stack Manager</h1>
          <p className="mt-1 text-sm text-gray-600">Sign in with your username and password.</p>
        </div>
        {error && <div className="rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-800">{error}</div>}
        <label className="block text-sm">
          <span className="mb-1 block font-medium text-gray-700">Username</span>
          <input value={form.username} onChange={e => setForm({ ...form, username: e.target.value })} className="input" autoComplete="username" />
        </label>
        <label className="block text-sm">
          <span className="mb-1 block font-medium text-gray-700">Password</span>
          <input type="password" value={form.password} onChange={e => setForm({ ...form, password: e.target.value })} className="input" autoComplete="current-password" autoFocus />
        </label>
        <button disabled={loading} className="btn-primary w-full">{loading ? 'Signing in...' : 'Sign In'}</button>
      </form>
    </div>
  );
}
