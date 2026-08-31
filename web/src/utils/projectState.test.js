import test from 'node:test';
import assert from 'node:assert/strict';

import { projectState, projectStateTone, stateTone } from './projectState.js';

test('projectState prefers the API state over the legacy running boolean', () => {
  assert.equal(projectState({ state: 'restarting', running: true }), 'restarting');
  assert.equal(projectState({ state: 'exited', running: false }), 'exited');
});

test('projectState remains compatible with older peers and agents', () => {
  assert.equal(projectState({ running: true }), 'running');
  assert.equal(projectState({ running: false }), 'stopped');
});

test('restart loops use an error tone everywhere', () => {
  assert.equal(projectStateTone({ state: 'restarting' }), 'red');
  assert.equal(stateTone('running'), 'green');
  assert.equal(stateTone('paused'), 'amber');
});
