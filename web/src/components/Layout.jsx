import { Link, Outlet, useLocation } from 'react-router-dom';

export default function Layout() {
  const location = useLocation();
  const isActive = (path) => location.pathname === path ? 'bg-gray-100 text-gray-950' : 'text-gray-600';

  return (
    <div className="min-h-screen bg-gray-50 text-gray-950">
      <nav className="border-b border-gray-200 bg-white px-6 py-3">
        <div className="flex items-center gap-6">
          <Link to="/" className="brand-wordmark text-2xl text-blue-800" title="Open the Stack Manager dashboard.">ARPHost Stack Manager</Link>
          <Link to="/" className={`rounded-md px-3 py-1.5 text-sm hover:bg-gray-100 ${isActive('/')}`}>Dashboard</Link>
          <Link to="/catalog" className={`rounded-md px-3 py-1.5 text-sm hover:bg-gray-100 ${isActive('/catalog')}`}>Stack Catalog</Link>
          <Link to="/audit" className={`rounded-md px-3 py-1.5 text-sm hover:bg-gray-100 ${isActive('/audit')}`}>Audit Log</Link>
          <Link to="/settings" className={`rounded-md px-3 py-1.5 text-sm hover:bg-gray-100 ${isActive('/settings')}`}>Settings</Link>
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
