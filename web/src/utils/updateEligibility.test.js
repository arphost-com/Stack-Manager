import test from 'node:test';
import assert from 'node:assert/strict';
import { canRunImageUpdate, updateBlockedReason, updateStatusLabel } from './updateEligibility.js';

test('controller is blocked even when a stale notice says available', () => {
  const project = { controller: true, update_status: { checked: true, available: true } };
  assert.equal(canRunImageUpdate(project), false);
  assert.match(updateBlockedReason(project), /Settings > Update/);
  assert.equal(updateStatusLabel(project), 'self-update only');
});

test('ordinary checked project with an available update is allowed', () => {
  assert.equal(canRunImageUpdate({ update_status: { checked: true, available: true } }), true);
});
