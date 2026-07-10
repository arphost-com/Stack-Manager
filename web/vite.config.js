import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { execSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

// Single source of truth for the app version is package.json "version". The
// displayed version is that base plus a short commit SHA so the footer always
// reflects the exact build and can never silently go stale. The SHA comes from
// VITE_GIT_SHA (set by CI, where the Docker build has no .git) and falls back to
// `git rev-parse` for local builds.
const pkg = JSON.parse(readFileSync(fileURLToPath(new URL('./package.json', import.meta.url)), 'utf8'))
let gitSha = process.env.VITE_GIT_SHA || ''
if (!gitSha) {
  try { gitSha = execSync('git rev-parse --short HEAD', { stdio: ['ignore', 'pipe', 'ignore'] }).toString().trim() } catch { gitSha = '' }
}
const appVersion = gitSha ? `${pkg.version}+${gitSha}` : pkg.version

export default defineConfig({
  plugins: [react()],
  define: {
    __APP_VERSION__: JSON.stringify(appVersion),
  },
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://localhost:8192',
    },
  },
})
