export const THEME_KEY = 'cm_theme';

export function getThemePreference() {
  return localStorage.getItem(THEME_KEY) || 'system';
}

export function applyThemePreference(preference = getThemePreference()) {
  const prefersDark = window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches;
  const dark = preference === 'dark' || (preference === 'system' && prefersDark);
  document.documentElement.classList.toggle('dark', dark);
  document.documentElement.dataset.theme = preference;
}

export function setThemePreference(preference) {
  localStorage.setItem(THEME_KEY, preference);
  applyThemePreference(preference);
  window.dispatchEvent(new CustomEvent('cm-theme-change', { detail: { preference } }));
}
