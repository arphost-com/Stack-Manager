import test from 'node:test';
import assert from 'node:assert/strict';
import React, { Suspense } from 'react';
import TestRenderer, { act } from 'react-test-renderer';
import RouteErrorBoundary from './RouteErrorBoundary.js';

test('a failed lazy project route offers a visible reload instead of a blank page', async () => {
  const originalError = console.error;
  console.error = () => {}; // React logs the deliberately rejected route import.
  let reloads = 0;
  const saved = new Map();
  const recoveryStorage = { getItem: key => saved.get(key), setItem: (key, value) => saved.set(key, value) };
  let renderer;
  try {
    const FailedRoute = React.lazy(() => Promise.reject(new Error('Failed to fetch dynamically imported module')));
    await act(async () => {
      renderer = TestRenderer.create(
        React.createElement(RouteErrorBoundary, { routeKey: '/projects/openbrain-voice', recoveryStorage, onReload: () => { reloads += 1; } },
          React.createElement(Suspense, { fallback: React.createElement('p', null, 'Loading') },
            React.createElement(FailedRoute))));
    });
    const view = renderer.toJSON();
    assert.match(JSON.stringify(view), /This page could not load/);
    assert.equal(reloads, 1, 'a missing lazy chunk reloads automatically once');
    const button = renderer.root.findByType('button');
    assert.equal(button.props.children, 'Reload page');
    act(() => button.props.onClick());
    assert.equal(reloads, 2);
  } finally {
    renderer?.unmount();
    console.error = originalError;
  }
});
