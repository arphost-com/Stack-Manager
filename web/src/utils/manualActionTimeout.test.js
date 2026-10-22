import test from 'node:test';
import assert from 'node:assert/strict';
import { projects, projectsForSource } from '../api/client.js';
import { DEFAULT_MANUAL_ACTION_TIMEOUT_SECONDS } from './manualActionTimeout.js';

test('manual project pull and update submit no-deadline jobs locally and through a peer', async () => {
  assert.equal(DEFAULT_MANUAL_ACTION_TIMEOUT_SECONDS, 0);
  const paths = [];
  const previousFetch = globalThis.fetch;
  const previousStorage = globalThis.localStorage;
  globalThis.localStorage = { getItem: () => null };
  globalThis.fetch = async (path) => {
    paths.push(path);
    return { ok: true, status: 202, json: async () => ({ status: 'ok', data: { id: 'job' } }) };
  };
  try {
    await projects.startJob('openbrain-voice', 'pull', DEFAULT_MANUAL_ACTION_TIMEOUT_SECONDS);
    await projectsForSource(7).startJob('openbrain-voice', 'update', DEFAULT_MANUAL_ACTION_TIMEOUT_SECONDS);
    await projects.startJob('openbrain-voice', 'pull', 900);
    assert.deepEqual(paths, [
      '/api/v1/projects/openbrain-voice/jobs/pull?timeout=0',
      '/api/v1/agent-proxy/7/projects/openbrain-voice/jobs/update?timeout=0',
      '/api/v1/projects/openbrain-voice/jobs/pull?timeout=900',
    ]);
  } finally {
    globalThis.fetch = previousFetch;
    globalThis.localStorage = previousStorage;
  }
});
