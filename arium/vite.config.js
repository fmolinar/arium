import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// The app calls the API at relative /api URLs; both `vite` and `vite preview`
// forward them to the backend, so there's no API URL baked into the build and
// no CORS. In Docker Compose, API_PROXY_TARGET points at the backend service.
const apiTarget = process.env.API_PROXY_TARGET ?? 'http://localhost:8080'

// Vite answers only localhost requests unless told otherwise. ALLOWED_HOSTS is a
// comma-separated list of extra hostnames, e.g. the public domain a Cloudflare
// Tunnel forwards to `vite preview` in Compose.
const allowedHosts = (process.env.ALLOWED_HOSTS ?? '')
  .split(',')
  .map((h) => h.trim())
  .filter(Boolean)

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    proxy: { '/api': apiTarget },
  },
  preview: {
    proxy: { '/api': apiTarget },
    allowedHosts,
  },
})
