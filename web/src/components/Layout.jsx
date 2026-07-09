import { useEffect, useState } from 'react';
import { Link, Outlet, useLocation } from 'react-router-dom';
import { auth } from '../api/client';

export default function Layout() {
  const location = useLocation();
  const isActive = (path) => location.pathname === path ? 'bg-gray-100 text-gray-950' : 'text-gray-600';
  const [me, setMe] = useState(() => {
    try { return JSON.parse(localStorage.getItem('cm_user') || 'null'); } catch { return null; }
  });

  useEffect(() => {
    // Refresh cached identity so the header shows the right name after login.
    auth.me().then(res => {
      if (res.data) {
        setMe(res.data);
        localStorage.setItem('cm_user', JSON.stringify(res.data));
      }
    }).catch(() => {});
  }, []);

  const logout = async () => {
    if (!window.confirm('Sign out of Stack Manager?')) return;
    try { await auth.logout(); } catch {}
    localStorage.removeItem('cm_token');
    localStorage.removeItem('cm_user');
    localStorage.removeItem('cm_api_key');
    Object.keys(localStorage).filter(k => k.startsWith('cm_dashboard_v')).forEach(k => localStorage.removeItem(k));
    window.location.href = '/';
  };

  return (
    <div className="min-h-screen bg-gray-50 text-gray-950">
      <nav className="border-b border-gray-200 bg-white px-6 py-3">
        <div className="flex items-center gap-6">
          <Link to="/" className="flex items-center gap-2 brand-wordmark text-2xl text-blue-800" title="Open the Stack Manager dashboard.">
            <img src="/logo.svg" alt="" className="h-8 w-8" />
            ARPHost Stack Manager
          </Link>
          <Link to="/" className={`rounded-md px-3 py-1.5 text-sm hover:bg-gray-100 ${isActive('/')}`}>Dashboard</Link>
          <Link to="/catalog" className={`rounded-md px-3 py-1.5 text-sm hover:bg-gray-100 ${isActive('/catalog')}`}>Stack Catalog</Link>
          <Link to="/audit" className={`rounded-md px-3 py-1.5 text-sm hover:bg-gray-100 ${isActive('/audit')}`}>Audit Log</Link>
          <Link to="/docs" className={`rounded-md px-3 py-1.5 text-sm hover:bg-gray-100 ${isActive('/docs')}`}>Documentation</Link>
          <Link to="/settings" className={`rounded-md px-3 py-1.5 text-sm hover:bg-gray-100 ${isActive('/settings')}`}>Settings</Link>
          <div className="ml-auto flex items-center gap-3 text-sm">
            {me && (
              <span className="text-gray-600" title={`Signed in as ${me.username} (${me.role})`}>
                {me.username} <span className="text-xs text-gray-400">({me.role})</span>
              </span>
            )}
            <button
              type="button"
              onClick={logout}
              className="btn-secondary text-sm"
              title="Sign out of Stack Manager. Ends this browser session."
            >
              Sign Out
            </button>
          </div>
        </div>
      </nav>
      <main className="max-w-7xl mx-auto px-6 py-6">
        <Outlet />
      </main>
      <footer className="mx-auto flex max-w-7xl items-center justify-end gap-3 px-6 pb-6 text-right text-sm">
        <span className="brand-wordmark text-lg text-blue-800">ARPHost Stack Manager</span>
        <a href="https://arphost.com" className="text-blue-700 hover:underline" title="Open ARPHost website.">arphost.com</a>
      </footer>
    </div>
  );
}
