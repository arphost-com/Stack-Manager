import React from 'react';

export function backupEndpointTestState(data, error) {
  if (error) return { status: 'error', message: error.message || 'Connection test failed.' };
  if (data?.success) return { status: 'success', message: 'Connection test succeeded.' };
  return { status: 'error', message: data?.error || 'Connection test failed.' };
}

export default function BackupEndpointTestStatus({ result }) {
  if (!result) return null;
  const color = result.status === 'success' ? 'text-green-700' : result.status === 'error' ? 'text-red-700' : 'text-blue-700';
  return React.createElement('div', { className: `mt-1 text-xs font-medium ${color}`, role: 'status' }, result.message);
}
