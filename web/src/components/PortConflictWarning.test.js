import test from 'node:test';
import assert from 'node:assert/strict';
import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import PortConflictWarning, { hasHostPortConflict } from './PortConflictWarning.js';

const conflicts = [
  'Error response from daemon: failed to bind host port 0.0.0.0:8765/tcp: address already in use',
  'Bind for 0.0.0.0:8080 failed: port is already allocated',
  'listen tcp [::]:8765: bind: address already in use',
  'listen udp 0.0.0.0:53: bind: address already in use',
  'driver failed programming external connectivity on endpoint app: address already in use',
];

for (const output of conflicts) {
  test(`startup conflict renders actionable non-mutating warning: ${output}`, () => {
    const result = { success: false, output };
    const before = JSON.stringify(result);
    const html = renderToStaticMarkup(createElement(PortConflictWarning, { result }));
    assert.equal(hasHostPortConflict(result), true);
    assert.match(html, /role="alert"/);
    assert.match(html, /Host port conflict/);
    assert.match(html, /If needed, you can choose a free host port/);
    assert.match(html, /keep the container port unchanged/);
    assert.match(html, /does not change ports or configuration/);
    assert.doesNotMatch(html, /18765|Falco|<button/);
    assert.equal(JSON.stringify(result), before);
  });
}

test('error-only API failures render the same warning', () => {
  assert.match(renderToStaticMarkup(createElement(PortConflictWarning, { result: { error: conflicts[0] } })), /Host port conflict/);
});

test('successful results, unrelated errors and empty results have no warning', () => {
  for (const result of [undefined, {}, {success: true, output: conflicts[0]}, {error: 'permission denied'}, {error: 'No such image'}, {output: 'unix socket: address already in use'}]) {
    assert.equal(renderToStaticMarkup(createElement(PortConflictWarning, { result })), '');
  }
});
