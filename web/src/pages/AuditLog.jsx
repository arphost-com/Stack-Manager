import { useCallback, useEffect, useRef, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { audit, system } from '../api/client';

const emptyFilters = {
  node: '',
  action: '',
  q: '',
  project: '',
  actor: '',
  success: '',
  limit: 100,
  offset: 0,
};

// Quick presets: substring-match the action name so every project's update /
// backup collapses into one view.
const PRESETS = [
  { key: 'updates', label: 'Updates run', q: 'update' },
  { key: 'backups', label: 'Backups run', q: 'backup/create' },
];

function formatDuration(ms) {
  if (!ms || ms < 0) return '—';
  if (ms < 1000) return `${ms} ms`;
  const s = ms / 1000;
  if (s < 60) return `${s.toFixed(2)}s`;
  return `${(s / 60).toFixed(1)}m`;
}

export default function AuditLog() {
  const [searchParams] = useSearchParams();
  const [filters, setFilters] = useState(emptyFilters);
  const [entries, setEntries] = useState([]);
  const [nodes, setNodes] = useState([]);
  const [actions, setActions] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [highlightId, setHighlightId] = useState('');
  const rowRefs = useRef({});

  // Second tab: Stack Manager's OWN server container log.
  const [view, setView] = useState('audit'); // 'audit' | 'applog'
  const [appLog, setAppLog] = useState('');
  const [appLogTail, setAppLogTail] = useState(500);
  const [appLogContainer, setAppLogContainer] = useState('');
  const [appLogLoading, setAppLogLoading] = useState(false);
  const [appLogErr, setAppLogErr] = useState('');

  const loadAppLog = useCallback(async (tail) => {
    setAppLogLoading(true);
    setAppLogErr('');
    try {
      const res = await system.appLog(tail);
      setAppLog(res.data?.log || '');
      setAppLogContainer(res.data?.container || '');
    } catch (err) {
      setAppLogErr(err.message);
    } finally {
      setAppLogLoading(false);
    }
  }, []);

  const load = useCallback(async (nextFilters) => {
    setLoading(true);
    setError('');
    try {
      const res = await audit.list(nextFilters);
      setEntries(res.data?.entries || []);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, []);

  // Seed filters from the URL on mount so deep links work, e.g.
  // /audit?project=lidarr&q=update&highlight=42 (Updates for a project, jumping
  // to a specific entry).
  useEffect(() => {
    Promise.all([audit.nodes(), audit.actions()]).then(([n, a]) => {
      setNodes(n.data || []);
      setActions(a.data || []);
    }).catch(err => setError(err.message));
    const fromUrl = {
      ...emptyFilters,
      node: searchParams.get('node') || '',
      action: searchParams.get('action') || '',
      q: searchParams.get('q') || '',
      project: searchParams.get('project') || '',
      actor: searchParams.get('actor') || '',
      success: searchParams.get('success') || '',
    };
    setHighlightId(searchParams.get('highlight') || '');
    setFilters(fromUrl);
    load(fromUrl);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [load]);

  // Scroll to the deep-linked entry once the rows render.
  useEffect(() => {
    if (!highlightId) return;
    const el = rowRefs.current[String(highlightId)];
    if (el) el.scrollIntoView({ behavior: 'smooth', block: 'center' });
  }, [highlightId, entries]);

  const applyPreset = (q) => {
    const next = { ...emptyFilters, q };
    setFilters(next);
    setHighlightId('');
    load(next);
  };

  const submit = (event) => {
    event.preventDefault();
    load({ ...filters, offset: 0 });
  };

  const clear = () => {
    setFilters(emptyFilters);
    load(emptyFilters);
  };

  const paginate = (delta) => {
    const next = { ...filters, offset: Math.max(0, (Number(filters.offset) || 0) + delta) };
    setFilters(next);
    load(next);
  };

  return (
    <div className="space-y-4">
      <div className="flex gap-1 border-b border-gray-200">
        <button
          type="button"
          onClick={() => setView('audit')}
          className={`-mb-px border-b-2 px-4 py-2 text-sm font-medium ${view === 'audit' ? 'border-blue-600 text-blue-700' : 'border-transparent text-gray-500 hover:text-gray-700'}`}
        >Command Audit</button>
        <button
          type="button"
          onClick={() => { setView('applog'); if (!appLog && !appLogLoading) loadAppLog(appLogTail); }}
          className={`-mb-px border-b-2 px-4 py-2 text-sm font-medium ${view === 'applog' ? 'border-blue-600 text-blue-700' : 'border-transparent text-gray-500 hover:text-gray-700'}`}
        >Server Log</button>
      </div>

      {view === 'audit' && (<>
      <div className="section-panel">
        <div className="mb-4 flex items-center justify-between">
          <div>
            <h1 className="text-xl font-semibold text-gray-950">Command Audit Log</h1>
            <p className="text-sm text-gray-600">Every mutating API call is recorded with actor, node, action, and success/failure. Filter by node to see per-agent activity.</p>
          </div>
          <button type="button" className="btn-secondary" onClick={() => load(filters)} disabled={loading} title="Re-fetch the current filter results.">
            {loading ? 'Loading…' : 'Refresh'}
          </button>
        </div>
        <div className="mb-3 flex flex-wrap items-center gap-2">
          <span className="text-xs font-medium uppercase tracking-wide text-gray-500">Quick view:</span>
          {PRESETS.map(p => (
            <button key={p.key} type="button" onClick={() => applyPreset(p.q)} className={`btn-secondary text-xs ${filters.q === p.q ? 'ring-2 ring-blue-500' : ''}`}>{p.label}</button>
          ))}
          <button type="button" onClick={clear} className="btn-secondary text-xs">All activity</button>
        </div>
        <form onSubmit={submit} className="grid gap-3 md:grid-cols-6">
          <label className="text-sm">
            <span className="text-gray-700">Node</span>
            <select className="input mt-1 w-full" value={filters.node} onChange={e => setFilters({ ...filters, node: e.target.value })}>
              <option value="">All nodes</option>
              {nodes.map(n => <option key={n} value={n}>{n}</option>)}
            </select>
          </label>
          <label className="text-sm">
            <span className="text-gray-700">Action</span>
            <select className="input mt-1 w-full" value={filters.action} onChange={e => setFilters({ ...filters, action: e.target.value })}>
              <option value="">All actions</option>
              {actions.map(a => <option key={a} value={a}>{a}</option>)}
            </select>
          </label>
          <label className="text-sm">
            <span className="text-gray-700">Project</span>
            <input className="input mt-1 w-full" value={filters.project} onChange={e => setFilters({ ...filters, project: e.target.value })} placeholder="project name" />
          </label>
          <label className="text-sm">
            <span className="text-gray-700">Actor</span>
            <input className="input mt-1 w-full" value={filters.actor} onChange={e => setFilters({ ...filters, actor: e.target.value })} placeholder="username" />
          </label>
          <label className="text-sm">
            <span className="text-gray-700">Result</span>
            <select className="input mt-1 w-full" value={filters.success} onChange={e => setFilters({ ...filters, success: e.target.value })}>
              <option value="">Any</option>
              <option value="true">Success only</option>
              <option value="false">Failure only</option>
            </select>
          </label>
          <label className="text-sm">
            <span className="text-gray-700">Rows per page</span>
            <select className="input mt-1 w-full" value={filters.limit} onChange={e => setFilters({ ...filters, limit: Number(e.target.value) })}>
              <option value={50}>50</option>
              <option value={100}>100</option>
              <option value={250}>250</option>
              <option value={500}>500</option>
            </select>
          </label>
          <div className="md:col-span-6 flex gap-2">
            <button type="submit" className="btn-primary">Apply filters</button>
            <button type="button" className="btn-secondary" onClick={clear}>Clear</button>
          </div>
        </form>
      </div>

      {error && <div className="rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-800">{error}</div>}

      <div className="section-panel">
        <table className="w-full table-fixed text-sm">
          <colgroup>
            <col className="w-[10%]" />
            <col className="w-[12%]" />
            <col className="w-[9%]" />
            <col className="w-[27%]" />
            <col className="w-[12%]" />
            <col className="w-[8%]" />
            <col className="w-[5%]" />
            <col className="w-[7%]" />
            <col className="w-[10%]" />
          </colgroup>
          <thead className="bg-gray-50 text-left text-xs uppercase tracking-wide text-gray-500">
            <tr>
              <th className="px-2 py-2">When</th>
              <th className="px-2 py-2">Node</th>
              <th className="px-2 py-2">Actor</th>
              <th className="px-2 py-2">Action</th>
              <th className="px-2 py-2">Project</th>
              <th className="px-2 py-2">Target</th>
              <th className="px-2 py-2">Result</th>
              <th className="px-2 py-2">Duration</th>
              <th className="px-2 py-2">IP</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100">
            {entries.length === 0 && (
              <tr><td colSpan={9} className="px-2 py-6 text-center text-gray-500">No entries match the current filters.</td></tr>
            )}
            {entries.map(entry => (
              <tr
                key={entry.id}
                ref={el => { if (el) rowRefs.current[String(entry.id)] = el; }}
                className={`align-top ${String(entry.id) === String(highlightId) ? 'bg-yellow-100 ring-2 ring-inset ring-yellow-400' : 'hover:bg-gray-100'}`}
              >
                <td className="truncate px-2 py-2 font-mono text-[11px]" title={new Date(entry.created_at).toISOString()}>{new Date(entry.created_at).toLocaleString(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit', second: '2-digit' })}</td>
                <td className="truncate px-2 py-2 text-xs" title={entry.node}>{entry.node || '—'}</td>
                <td className="truncate px-2 py-2 text-xs" title={entry.actor}>{entry.actor || '—'}</td>
                <td className="truncate px-2 py-2 font-mono text-[11px]" title={entry.action}>{entry.action}</td>
                <td className="truncate px-2 py-2 text-xs" title={entry.project}>{entry.project || '—'}</td>
                <td className="truncate px-2 py-2 text-xs" title={entry.target}>{entry.target || '—'}</td>
                <td className="px-2 py-2">
                  {entry.success ? <span className="rounded bg-green-100 px-1.5 py-0.5 text-[11px] text-green-800">OK</span> : <span className="rounded bg-red-100 px-1.5 py-0.5 text-[11px] text-red-800">FAIL</span>}
                </td>
                <td className="px-2 py-2 text-[11px] text-gray-600">{formatDuration(entry.duration_ms)}</td>
                <td className="truncate px-2 py-2 font-mono text-[11px] text-gray-500" title={entry.remote_ip}>{entry.remote_ip || '—'}</td>
              </tr>
            ))}
          </tbody>
        </table>
        <div className="mt-3 flex items-center justify-between text-sm text-gray-600">
          <span>Showing {entries.length} row{entries.length === 1 ? '' : 's'} (offset {filters.offset})</span>
          <div className="flex gap-2">
            <button type="button" className="btn-secondary" onClick={() => paginate(-filters.limit)} disabled={filters.offset === 0 || loading}>Previous</button>
            <button type="button" className="btn-secondary" onClick={() => paginate(filters.limit)} disabled={entries.length < filters.limit || loading}>Next</button>
          </div>
        </div>
      </div>
      </>)}

      {view === 'applog' && (
        <div className="section-panel">
          <div className="mb-4 flex items-center justify-between gap-3">
            <div>
              <h1 className="text-xl font-semibold text-gray-950">Server Log</h1>
              <p className="text-sm text-gray-600">
                Stack Manager's own server container log (startup, errors, scheduler, background metrics).
                {appLogContainer && <span className="ml-1 font-mono text-xs text-gray-500">[{appLogContainer.slice(0, 12)}]</span>}
              </p>
            </div>
            <div className="flex shrink-0 items-center gap-2">
              <label className="text-sm text-gray-700">
                Lines
                <select className="input ml-1" value={appLogTail} onChange={e => { const t = Number(e.target.value); setAppLogTail(t); loadAppLog(t); }}>
                  <option value={200}>200</option>
                  <option value={500}>500</option>
                  <option value={1000}>1000</option>
                  <option value={2000}>2000</option>
                </select>
              </label>
              <button type="button" className="btn-secondary" onClick={() => loadAppLog(appLogTail)} disabled={appLogLoading}>
                {appLogLoading ? 'Loading…' : 'Refresh'}
              </button>
            </div>
          </div>
          {appLogErr && <div className="mb-3 rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-800">{appLogErr}</div>}
          <pre className="max-h-[70vh] overflow-auto rounded bg-gray-900 p-3 font-mono text-[11px] leading-relaxed text-gray-100 whitespace-pre-wrap break-words">
            {appLog || (appLogLoading ? 'Loading…' : 'No log output.')}
          </pre>
        </div>
      )}
    </div>
  );
}
