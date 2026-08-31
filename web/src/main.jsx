import React, { Suspense, lazy } from 'react';
import ReactDOM from 'react-dom/client';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import './index.css';
import Layout from './components/Layout';
import { applyThemePreference } from './theme';

const Dashboard = lazy(() => import('./pages/Dashboard'));
const ProjectDetail = lazy(() => import('./pages/ProjectDetail'));
const Settings = lazy(() => import('./pages/Settings'));
const Login = lazy(() => import('./pages/Login'));
const StackCatalog = lazy(() => import('./pages/StackCatalog'));
const AuditLog = lazy(() => import('./pages/AuditLog'));
const Documentation = lazy(() => import('./pages/Documentation'));

function PageFallback() {
  return <div className="py-12 text-center text-sm text-gray-500">Loading…</div>;
}

applyThemePreference();
if (window.matchMedia) {
  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => applyThemePreference());
}

function App() {
  const credential = localStorage.getItem('cm_token');

  // BrowserRouter must wrap BOTH branches: the login/landing page reuses doc
  // components that render <Link>, which throws without a Router context.
  return (
    <BrowserRouter>
      <Suspense fallback={<PageFallback />}>
        {!credential ? (
          <Login />
        ) : (
          <Routes>
            <Route element={<Layout />}>
              <Route path="/" element={<Dashboard />} />
              <Route path="/catalog" element={<StackCatalog />} />
              <Route path="/audit" element={<AuditLog />} />
              <Route path="/docs" element={<Documentation />} />
              <Route path="/projects/:name" element={<ProjectDetail />} />
              <Route path="/settings" element={<Settings />} />
              <Route path="*" element={<Navigate to="/" />} />
            </Route>
          </Routes>
        )}
      </Suspense>
    </BrowserRouter>
  );
}

ReactDOM.createRoot(document.getElementById('root')).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
