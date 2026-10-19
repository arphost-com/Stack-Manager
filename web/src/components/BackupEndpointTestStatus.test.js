import test from 'node:test';
import assert from 'node:assert/strict';
import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import BackupEndpointTestStatus, { backupEndpointTestState } from './BackupEndpointTestStatus.js';

test('backup endpoint success remains visible in the endpoint row', () => {
  const state = backupEndpointTestState({ success: true });
  const html = renderToStaticMarkup(createElement(BackupEndpointTestStatus, { result: state }));
  assert.equal(state.status, 'success');
  assert.match(html, /role="status"/);
  assert.match(html, /Connection test succeeded/);
});

test('backup endpoint failure preserves the returned reason', () => {
  const state = backupEndpointTestState({ success: false, error: 'authentication rejected' });
  const html = renderToStaticMarkup(createElement(BackupEndpointTestStatus, { result: state }));
  assert.equal(state.status, 'error');
  assert.match(html, /authentication rejected/);
});

test('transport failures are represented as failed tests', () => {
  assert.deepEqual(backupEndpointTestState(null, new Error('network unavailable')), {
    status: 'error', message: 'network unavailable',
  });
});
