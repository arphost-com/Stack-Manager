import { useCallback, useEffect, useState } from 'react';
import { audit } from '../api/client';

const emptyFilters = {
  node: '',
  action: '',
  project: '',
  actor: '',
  success: '',
  limit: 100,
  offset: 0,
};

function formatDuration(ms) {
  if (!ms || ms < 0) return '—';
  if (ms < 1000) return `${ms} ms`;
  const s = ms / 1000;
  if (s < 60) return `${s.toFixed(2)}s`;
  return `${(s / 60).toFixed(1)}m`;
}

export default function AuditLog() {
  const [filters, setFilters] = useState(emptyFilters);
  const [entries, setEntries] = useState([]);
  const [nodes, setNodes] = useState([]);
  const [actions, setActions] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

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

  useEffect(() => {
    Promise.all([audit.nodes(), audit.actions()]).then(([n, a]) => {
      setNodes(n.data || []);
      setActions(a.data || []);
    }).catch(err => setError(err.message));
    load(emptyFilters);
  }, [load]);

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

      <div className="section-panel overflow-x-auto">
        <table className="min-w-full text-sm">
          <thead className="bg-gray-50 text-left text-xs uppercase tracking-wide text-gray-500">
            <tr>
              <th className="px-3 py-2">When</th>
              <th className="px-3 py-2">Node</th>
              <th className="px-3 py-2">Actor</th>
              <th className="px-3 py-2">Action</th>
              <th className="px-3 py-2">Project</th>
              <th className="px-3 py-2">Target</th>
              <th className="px-3 py-2">Result</th>
              <th className="px-3 py-2">Duration</th>
              <th className="px-3 py-2">IP</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100">
            {entries.length === 0 && (
              <tr><td colSpan={9} className="px-3 py-6 text-center text-gray-500">No entries match the current filters.</td></tr>
            )}
            {entries.map(entry => (
              <tr key={entry.id} className="align-top hover:bg-gray-50">
                <td className="px-3 py-2 whitespace-nowrap font-mono text-xs">{new Date(entry.created_at).toLocaleString()}</td>
                <td className="px-3 py-2 whitespace-nowrap">{entry.node || '—'}</td>
                <td className="px-3 py-2 whitespace-nowrap">{entry.actor || '—'}</td>
                <td className="px-3 py-2 font-mono text-xs">{entry.action}</td>
                <td className="px-3 py-2 whitespace-nowrap">{entry.project || '—'}</td>
                <td className="px-3 py-2 whitespace-nowrap">{entry.target || '—'}</td>
                <td className="px-3 py-2 whitespace-nowrap">
                  {entry.success ? <span className="rounded bg-green-100 px-1.5 py-0.5 text-xs text-green-800">OK</span> : <span className="rounded bg-red-100 px-1.5 py-0.5 text-xs text-red-800">FAIL</span>}
                </td>
                <td className="px-3 py-2 whitespace-nowrap text-xs text-gray-600">{formatDuration(entry.duration_ms)}</td>
                <td className="px-3 py-2 whitespace-nowrap font-mono text-xs text-gray-500">{entry.remote_ip || '—'}</td>
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
    </div>
  );
}
