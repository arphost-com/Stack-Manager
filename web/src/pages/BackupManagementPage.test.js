import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const main = readFileSync(new URL('../main.jsx', import.meta.url), 'utf8');
const layout = readFileSync(new URL('../components/Layout.jsx', import.meta.url), 'utf8');
const dashboard = readFileSync(new URL('./Dashboard.jsx', import.meta.url), 'utf8');

test('backups have a dedicated route and primary navigation entry', () => {
  assert.match(main, /path="\/backups"/);
  assert.match(layout, /to="\/backups"[^>]*>Backups</);
});

test('backup page preserves single, selected, and all-project actions', () => {
  assert.match(dashboard, />Backup<\/button>/);
  assert.match(dashboard, /Backup Selected/);
  assert.match(dashboard, /Backup All Active/);
});

test('backup scheduler exposes calendar and interval choices', () => {
  for (const value of ['daily', 'weekly', 'monthly', 'interval']) {
    assert.match(dashboard, new RegExp(`option value="${value}"`));
  }
});
