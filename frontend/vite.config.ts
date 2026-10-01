import { defineConfig, type Plugin, type UserConfig } from 'vite'
import react from '@vitejs/plugin-react'

// En producción /embed/config.js lo genera nginx a partir de CRM_ORIGIN
// (docker-entrypoint.d/18-crm-origin.envsh). En `npm run dev` lo sirve este
// plugin desde la MISMA variable, para no tener una segunda configuración.
const EMBED_ORIGIN = /^(https:\/\/[A-Za-z0-9.-]+|http:\/\/(localhost|127\.0\.0\.1))(:\d+)?$/
function embedConfigDev(): Plugin {
  return {
    name: 'obertrack-embed-config',
    apply: 'serve',
    configureServer(server) {
      server.middlewares.use('/embed/config.js', (_req, res) => {
        const origin = process.env.CRM_ORIGIN ?? ''
        const crmOrigin = EMBED_ORIGIN.test(origin) ? origin : ''
        res.setHeader('Content-Type', 'application/javascript')
        res.setHeader('Cache-Control', 'no-store')
        res.end(`window.__OBERTRACK_EMBED__ = ${JSON.stringify({ crmOrigin })};`)
      })
    },
  }
}

export default defineConfig({
  plugins: [react(), embedConfigDev()],
  test: {
    globals: true,
    environment: 'jsdom',
    setupFiles: './src/test/setup.ts',
    css: false,
  },
  // Strip console.* and debugger from production bundles. Logs stay in dev.
  esbuild: {
    drop: ['debugger'],
    pure: ['console.log', 'console.debug', 'console.info'],
  },
  build: {
    target: 'esnext',
    chunkSizeWarningLimit: 1200,
    rollupOptions: {
      output: {
        manualChunks: {
          'vendor-react': ['react', 'react-dom', 'react-router-dom'],
          'vendor-charts': ['recharts'],
          'vendor-geo': ['country-state-city', 'leaflet', 'react-leaflet'],
          'vendor-icons': ['lucide-react'],
          'vendor-query': ['@tanstack/react-query', 'axios'],
        },
      },
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/ws': {
        target: 'http://localhost:8080',
        ws: true,
      },
    },
  },
} as UserConfig & { test?: Record<string, any> })
