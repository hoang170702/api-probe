import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// The backend address used by the dev proxy; override with CASERUNNER_API_URL.
const apiTarget = process.env.CASERUNNER_API_URL ?? 'http://127.0.0.1:8080'

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      // SSE streams also go through this proxy; Vite does not buffer them.
      '/api': { target: apiTarget, changeOrigin: true },
    },
  },
})
