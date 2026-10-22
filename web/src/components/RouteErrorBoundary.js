import React from 'react';

// React otherwise unmounts the app when a route component throws (including a
// lazy route whose JavaScript chunk cannot be loaded after a deployment).
export default class RouteErrorBoundary extends React.Component {
  constructor(props) {
    super(props);
    this.state = { failed: false };
  }

  static getDerivedStateFromError() {
    return { failed: true };
  }

  reload = () => (this.props.onReload || (() => window.location.reload()))();

  componentDidCatch(error) {
    // A page kept open across a deployment may request a chunk that no longer
    // exists. Refresh once to load the current asset manifest, then leave a
    // visible retry control if the fresh page still fails.
    if (!/Failed to fetch dynamically imported module|Importing a module script failed|ChunkLoadError|Loading chunk .* failed/i.test(error?.message || '')) return;
    try {
      const storage = this.props.recoveryStorage || window.sessionStorage;
      const key = `stack-manager:route-reload:${this.props.routeKey}`;
      const lastReload = Number(storage.getItem(key) || 0);
      if (Date.now() - lastReload < 5 * 60 * 1000) return;
      storage.setItem(key, String(Date.now()));
      this.reload();
    } catch {
      // Storage can be disabled; the visible Reload page button still works.
    }
  }

  componentDidUpdate(previousProps) {
    if (previousProps.routeKey !== this.props.routeKey && this.state.failed) {
      this.setState({ failed: false });
    }
  }

  render() {
    if (!this.state.failed) return this.props.children;
    return React.createElement('main', { className: 'mx-auto max-w-xl px-6 py-16 text-gray-950' },
      React.createElement('h1', { className: 'text-xl font-semibold' }, 'This page could not load'),
      React.createElement('p', { className: 'mt-3 text-sm text-gray-700' }, 'Reload the page to try again. If it keeps failing, report the project name and the action you took.'),
      React.createElement('button', {
        type: 'button',
        className: 'btn-primary mt-5',
        title: 'Reload this page and request the current app files.',
        onClick: this.reload,
      }, 'Reload page'));
  }
}
