import test from 'node:test';
import assert from 'node:assert/strict';
import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import ProjectActionButton from '../components/ProjectActionButton.js';
import { canRunProjectAction } from './updateEligibility.js';

const cases = [
  { name: 'not checked', project: { name: 'gitlab' } },
  { name: 'current images', project: { name: 'PowerDNS-Admin', update_status: { checked: true, available: false } } },
  { name: 'check warning', project: { update_status: { checked: true, error: 'registry unavailable' } } },
  { name: 'automatic updates disabled', project: { update_policy: { effective_policy: 'no_updates' } } },
];

for (const { name, project } of cases) {
  test(`manual Pull control is enabled with ${name}`, () => {
    const button = renderToStaticMarkup(createElement(ProjectActionButton, { project, action: 'pull', title: 'Pull images' }, 'Pull'));
    assert.match(button, />Pull<\/button>/);
    assert.doesNotMatch(button, /disabled=/);
    assert.equal(canRunProjectAction(project, 'pull'), true);
  });
}

test('bulk Pull includes unchecked and current projects but excludes controller', () => {
  const projects = [...cases.map(({ project }) => project), { controller: true }];
  assert.equal(projects.filter(project => canRunProjectAction(project, 'pull')).length, cases.length);
  const button = renderToStaticMarkup(createElement(ProjectActionButton, { projects, action: 'pull' }, 'Pull selected'));
  assert.doesNotMatch(button, /disabled=/);
});

test('controller-only Pull remains disabled and uses self-update guidance', () => {
  const project = { controller: true, update_status: { checked: true, available: true } };
  const button = renderToStaticMarkup(createElement(ProjectActionButton, { project, action: 'pull' }, 'Pull'));
  assert.match(button, /disabled=/);
  assert.match(button, /Settings &gt; Update/);
  assert.equal(canRunProjectAction(project, 'pull'), false);
  const bulk = renderToStaticMarkup(createElement(ProjectActionButton, { projects: [project], action: 'pull' }, 'Pull selected'));
  assert.match(bulk, /disabled=/);
});

test('pending actions and All Servers read-only mode remain disabled', () => {
  const button = renderToStaticMarkup(createElement(ProjectActionButton, { project: {}, action: 'pull', disabled: true, disabledReason: 'Select one server' }, 'Pull'));
  assert.match(button, /disabled=/);
  assert.match(button, /Select one server/);
});

test('manual Update is enabled independently of check results', () => {
  for (const { project } of cases.slice(0, 3)) {
    assert.equal(canRunProjectAction(project, 'update'), true);
    const button = renderToStaticMarkup(createElement(ProjectActionButton, { project, action: 'update' }, 'Update'));
    assert.doesNotMatch(button, /disabled=/);
  }
  assert.equal(canRunProjectAction({ update_status: { checked: true, available: true } }, 'update'), true);
});

test('manual and bulk Update preserve No updates and controller protection', () => {
  for (const project of [cases[3].project, { controller: true }]) {
    assert.equal(canRunProjectAction(project, 'update'), false);
    const button = renderToStaticMarkup(createElement(ProjectActionButton, { project, action: 'update' }, 'Update'));
    assert.match(button, /disabled=/);
  }
  const projects = [...cases.map(({ project }) => project), { controller: true }];
  assert.equal(projects.filter(project => canRunProjectAction(project, 'update')).length, 3);
  const button = renderToStaticMarkup(createElement(ProjectActionButton, { projects, action: 'update' }, 'Update selected'));
  assert.doesNotMatch(button, /disabled=/);
});
