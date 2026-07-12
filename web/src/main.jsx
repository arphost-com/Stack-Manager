import React from 'react';
import ReactDOM from 'react-dom/client';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import './index.css';
import Layout from './components/Layout';
import Dashboard from './pages/Dashboard';
import ProjectDetail from './pages/ProjectDetail';
import Settings from './pages/Settings';
import Login from './pages/Login';
import { applyThemePreference } from './theme';
import StackCatalog from './pages/StackCatalog';
import AuditLog from './pages/AuditLog';
import Documentation from './pages/Documentation';

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
    </BrowserRouter>
  );
}

ReactDOM.createRoot(document.getElementById('root')).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
