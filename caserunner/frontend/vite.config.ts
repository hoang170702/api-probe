import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// The backend address used by the dev proxy; override with CASERUNNER_API_URL.
const apiTarget = process.env.CASERUNNER_API_URL ?? 'http://127.0.0.1:8080'

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      // SSE streams also go through this proxy; Vite does not buffer them.
      // The backend refuses calls whose Origin is another site (BRD-001 §13),
      // so the proxy presents them as coming from the backend's own origin.
      '/api': { target: apiTarget, changeOrigin: true, headers: { origin: apiTarget } },
    },
  },
})
