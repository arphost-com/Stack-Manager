import { useState } from 'react';
import { auth, totp } from '../api/client';
import { StackManagerDocs, DockerComposeDocs, GpuDocs } from './Documentation';

const FEATURES = [
  ['200+ one-click stacks', 'A curated catalog — AI, media, databases, dev tools, monitoring — each deploys as a normal Docker Compose project you can edit.'],
  ['One dashboard, whole fleet', 'Manage many hosts from one place: peer controllers and agents (inbound or check-in behind NAT). Commands target the server you pick.'],
  ['Operate, don’t SSH', 'Logs, live stats, in-browser config editor, scoped shell, security scans, DB checks, volumes & networks, backups to S3/SFTP/NFS — per project.'],
  ['GPU, proxy & firewall', 'One-click NVIDIA driver + passthrough, Nginx Proxy Manager, and CSF firewall port opening — wired from the dashboard.'],
];

// A lightweight CSS mock of the dashboard so the landing page has a product
// preview without shipping real screenshots.
function DashboardMock() {
  const rows = [
    ['jellyfin', 'running', 'emerald'],
    ['immich', 'running', 'emerald'],
    ['postgres', 'running', 'emerald'],
    ['radarr', 'stopped', 'gray'],
  ];
  return (
    <div className="rounded-xl border border-white/10 bg-slate-900/70 shadow-2xl ring-1 ring-white/10 backdrop-blur">
      <div className="flex items-center gap-1.5 border-b border-white/10 px-3 py-2">
        <span className="h-2.5 w-2.5 rounded-full bg-red-400/80" />
        <span className="h-2.5 w-2.5 rounded-full bg-yellow-400/80" />
        <span className="h-2.5 w-2.5 rounded-full bg-green-400/80" />
        <span className="ml-3 rounded bg-white/10 px-2 py-0.5 text-[10px] text-slate-300">https://stacks.local:8993</span>
      </div>
      <div className="space-y-2 p-3">
        {rows.map(([name, status, tone]) => (
          <div key={name} className="flex items-center justify-between rounded-lg bg-white/5 px-3 py-2">
            <div className="flex items-center gap-2">
              <span className={`h-2 w-2 rounded-full ${tone === 'emerald' ? 'bg-emerald-400' : 'bg-slate-500'}`} />
              <span className="font-mono text-xs text-slate-200">{name}</span>
              <span className={`rounded px-1.5 py-0.5 text-[9px] uppercase ${tone === 'emerald' ? 'bg-emerald-500/20 text-emerald-300' : 'bg-slate-600/40 text-slate-300'}`}>{status}</span>
            </div>
            <div className="flex gap-1">
              {['Logs', 'Update', 'Backup'].map(a => <span key={a} className="rounded bg-white/10 px-1.5 py-0.5 text-[9px] text-slate-300">{a}</span>)}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

const DOC_TABS = [
  ['stacks', 'Stack Catalog'],
  ['stackmanager', 'Stack Manager'],
  ['compose', 'Docker Compose'],
  ['gpu', 'GPU'],
];

export default function Login() {
  const [form, setForm] = useState({ username: '', password: '' });
  const [totpStep, setTotpStep] = useState(false);
  const [totpToken, setTotpToken] = useState('');
  const [totpCode, setTotpCode] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [docsTab, setDocsTab] = useState('stacks');

  const finishLogin = (data) => {
    localStorage.setItem('cm_token', data.token);
    localStorage.setItem('cm_user', JSON.stringify(data.user));
    localStorage.removeItem('cm_api_key');
    window.location.href = '/';
  };

  const submit = async (event) => {
    event.preventDefault();
    setLoading(true);
    setError('');
    try {
      const res = await auth.login(form);
      if (res.data.totp_required) {
        setTotpToken(res.data.totp_token);
        setTotpStep(true);
        setLoading(false);
        return;
      }
      finishLogin(res.data);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const submitTotp = async (event) => {
    event.preventDefault();
    setLoading(true);
    setError('');
    try {
      const res = await totp.login(totpToken, totpCode);
      finishLogin(res.data);
    } catch (err) {
      setError(err.message);
      if (err.message?.includes('expired')) {
        setTotpStep(false);
        setTotpToken('');
        setTotpCode('');
      }
    } finally {
      setLoading(false);
    }
  };

  const resetTotp = () => {
    setTotpStep(false);
    setTotpToken('');
    setTotpCode('');
    setError('');
  };

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100">
      {/* Hero */}
      <div className="relative overflow-hidden">
        <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(60%_60%_at_20%_0%,rgba(37,99,235,0.25),transparent),radial-gradient(50%_50%_at_100%_10%,rgba(139,92,246,0.2),transparent)]" />
        <div className="relative mx-auto grid max-w-6xl gap-10 px-6 py-14 lg:grid-cols-2 lg:items-center">
          <div>
            <div className="flex items-center gap-2">
              <img src="/logo.svg" alt="" className="h-9 w-9" />
              <span className="brand-wordmark text-3xl text-blue-300">Stack Manager</span>
            </div>
            <h1 className="mt-6 text-4xl font-bold leading-tight text-white sm:text-5xl">
              Run any Docker&nbsp;Compose stack across your fleet — from one dashboard.
            </h1>
            <p className="mt-4 max-w-xl text-lg text-slate-300">
              Discover, deploy, and operate Compose projects on every host you own. 200+ one-click templates, multi-server management, and the day-2 tools you actually need.
            </p>
            <div className="mt-8 grid gap-4 sm:grid-cols-2">
              {FEATURES.map(([title, body]) => (
                <div key={title} className="rounded-lg border border-white/10 bg-white/5 p-4">
                  <div className="text-sm font-semibold text-white">{title}</div>
                  <p className="mt-1 text-sm leading-6 text-slate-300">{body}</p>
                </div>
              ))}
            </div>
            <div className="mt-8">
              <DashboardMock />
            </div>
          </div>

          {/* Login card */}
          <div className="lg:pl-6">
            <div className="mx-auto w-full max-w-md rounded-2xl border border-white/10 bg-slate-900/80 p-6 shadow-2xl ring-1 ring-white/10 backdrop-blur">
              {!totpStep ? (
                <form onSubmit={submit} className="space-y-4">
                  <div>
                    <h2 className="text-xl font-semibold text-white">Sign in</h2>
                    <p className="mt-1 text-sm text-slate-400">Use your Stack Manager username and password.</p>
                  </div>
                  {error && <div className="rounded-md border border-red-500/30 bg-red-500/10 p-3 text-sm text-red-300">{error}</div>}
                  <label className="block text-sm">
                    <span className="mb-1 block font-medium text-slate-300">Username</span>
                    <input value={form.username} onChange={e => setForm({ ...form, username: e.target.value })} className="w-full rounded-md border border-white/10 bg-slate-950/60 px-3 py-2 text-slate-100 placeholder-slate-500 outline-none focus:border-blue-400" autoComplete="username" autoFocus />
                  </label>
                  <label className="block text-sm">
                    <span className="mb-1 block font-medium text-slate-300">Password</span>
                    <input type="password" value={form.password} onChange={e => setForm({ ...form, password: e.target.value })} className="w-full rounded-md border border-white/10 bg-slate-950/60 px-3 py-2 text-slate-100 placeholder-slate-500 outline-none focus:border-blue-400" autoComplete="current-password" />
                  </label>
                  <button disabled={loading} className="w-full rounded-md bg-blue-600 px-4 py-2 font-medium text-white hover:bg-blue-500 disabled:opacity-60">{loading ? 'Signing in…' : 'Sign In'}</button>
                </form>
              ) : (
                <form onSubmit={submitTotp} className="space-y-4">
                  <div>
                    <h2 className="text-xl font-semibold text-white">Two-factor authentication</h2>
                    <p className="mt-1 text-sm text-slate-400">Enter the 6-digit code from your authenticator app, or a backup code.</p>
                  </div>
                  {error && <div className="rounded-md border border-red-500/30 bg-red-500/10 p-3 text-sm text-red-300">{error}</div>}
                  <label className="block text-sm">
                    <span className="mb-1 block font-medium text-slate-300">Code</span>
                    <input
                      value={totpCode}
                      onChange={e => setTotpCode(e.target.value.replace(/[^0-9a-fA-F]/g, ''))}
                      className="w-full rounded-md border border-white/10 bg-slate-950/60 px-3 py-2 text-center text-2xl tracking-widest text-slate-100 outline-none focus:border-blue-400"
                      maxLength={8}
                      autoComplete="one-time-code"
                      inputMode="numeric"
                      autoFocus
                      placeholder="000000"
                    />
                  </label>
                  <button disabled={loading} className="w-full rounded-md bg-blue-600 px-4 py-2 font-medium text-white hover:bg-blue-500 disabled:opacity-60">{loading ? 'Verifying…' : 'Verify'}</button>
                  <button type="button" onClick={resetTotp} className="w-full rounded-md border border-white/10 px-4 py-2 text-slate-300 hover:bg-white/5">Back to login</button>
                </form>
              )}
              <p className="mt-4 text-center text-xs text-slate-500">
                Self-hosted · <a href="https://arphost.com" className="text-blue-400 hover:underline">arphost.com</a>
              </p>
            </div>
          </div>
        </div>
      </div>

      {/* Public documentation */}
      <div className="border-t border-white/10 bg-white text-gray-950">
        <div className="mx-auto max-w-6xl px-6 py-12">
          <h2 className="text-2xl font-semibold">Documentation</h2>
          <p className="mt-1 text-sm text-gray-600">What you can do with Stack Manager — no login required. Sign in above to manage your own stacks.</p>
          <div className="mt-4 flex flex-wrap gap-2" role="tablist">
            {DOC_TABS.map(([key, label]) => (
              <button
                key={key}
                type="button"
                onClick={() => setDocsTab(key)}
                className={docsTab === key ? 'btn-primary' : 'btn-secondary'}
              >
                {label}
              </button>
            ))}
          </div>
          <div className="mt-4">
            {docsTab === 'stacks' && (
              <div className="section-panel space-y-3">
                <h3 className="text-lg font-semibold">200+ one-click stacks across 19 categories</h3>
                <p className="text-sm leading-6 text-gray-700">
                  AI/ML, media servers &amp; the *arr suite, databases, dev tools, CMS, monitoring, files, proxy, security, automation, and more. Each template loads an editable <code className="rounded bg-gray-100 px-1">compose.yml</code> + <code className="rounded bg-gray-100 px-1">.env</code> into the Create form — nothing deploys until you review it.
                </p>
                <div className="flex flex-wrap gap-2 text-xs">
                  {['AI', 'Media', 'Database', 'Dev Tools', 'CMS', 'Monitoring', 'Files', 'Proxy', 'Security', 'Automation'].map(c => (
                    <span key={c} className="rounded-full bg-blue-50 px-3 py-1 font-medium text-blue-800">{c}</span>
                  ))}
                </div>
                <p className="text-sm text-gray-600">Sign in to browse and deploy the full catalog.</p>
              </div>
            )}
            {docsTab === 'stackmanager' && <StackManagerDocs />}
            {docsTab === 'compose' && <DockerComposeDocs />}
            {docsTab === 'gpu' && <GpuDocs />}
          </div>
        </div>
      </div>
    </div>
  );
}
