import { fileURLToPath, URL } from 'node:url'
import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  const port = Number(env.PORT || 5747)
  if (!env.VITE_API_URL) {
    throw new Error('VITE_API_URL is not set: copy frontend/.env.example to frontend/.env (http://localhost:8747/api/v1 locally, /api/v1 behind nginx)')
  }

  return {
    plugins: [react()],
    resolve: {
      alias: {
        '@': fileURLToPath(new URL('./src', import.meta.url)),
      },
    },
    build: {
      rollupOptions: {
        output: {
          // React and the libraries every page needs change far less often than app code,
          // so in their own files they stay cached across deploys. Everything else is left
          // to Rollup on purpose: it keeps heavy code (such as the table) in
          // the chunk of the pages that use it, instead of in the first download.
          manualChunks(id) {
            if (!id.includes('node_modules')) return undefined
            if (/node_modules\/(react|react-dom|react-router|react-router-dom|scheduler)\//.test(id)) return 'vendor-react'
            if (/node_modules\/(@tanstack|zustand|i18next|react-i18next|i18next-browser-languagedetector)\//.test(id)) return 'vendor'
            return undefined
          },
        },
      },
      // The Ant Design parts the shell needs are above Vite's default 500 kB warning
      chunkSizeWarningLimit: 900,
    },
    server: {
      port,
      strictPort: true,
    },
    preview: {
      port,
      strictPort: true,
    },
  }
})
