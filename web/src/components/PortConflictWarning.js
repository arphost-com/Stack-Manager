import { createElement } from 'react';

export function hasHostPortConflict(result = {}) {
  if (result?.success === true) return false;
  const text = [result?.output, result?.error].filter(value => typeof value === 'string').join('\n');
  return /port is already allocated|(?:failed to bind host port|bind for|listen (?:tcp|udp)[46]?|driver failed programming external connectivity)[^\n]*(?:address already in use|port is already allocated)/i.test(text);
}

// Advice only: never choose a port, edit configuration, or stop another service.
export default function PortConflictWarning({ result }) {
  if (!hasHostPortConflict(result)) return null;
  return createElement('div', {
    role: 'alert',
    className: 'my-2 rounded border border-amber-300 bg-amber-50 p-3 text-sm text-amber-950',
  },
  createElement('strong', null, 'Host port conflict. '),
  'A published host port is already in use. Check the failed port in the output and identify its owner on the selected server. ',
  'If needed, you can choose a free host port in this stack’s Compose or documented port setting; keep the container port unchanged and follow the stack’s documentation. ',
  'Do not stop an unrelated service just to free the port. This warning does not change ports or configuration.');
}
