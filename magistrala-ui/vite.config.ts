import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import vueJsx from '@vitejs/plugin-vue-jsx'
import vueDevTools from 'vite-plugin-vue-devtools'
import { sessionPlugin } from './server/session-server.mjs'
import { cameraPlugin } from './server/camera-gateway.mjs'
import { createAgentProxy } from './server/agent-proxy.mjs'

// https://vite.dev/config/
export default defineConfig({
  // Keep the running app's optimized dependencies separate from test servers.
  cacheDir: 'node_modules/.vite-dev',
  plugins: [sessionPlugin(), cameraPlugin(), vue(), vueJsx(), vueDevTools()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    host: '0.0.0.0',
    port: 5173,
    strictPort: true,
    proxy: {
      '/api/agent': createAgentProxy(),
      ...Object.fromEntries(
        Object.entries({
          rules: 'http://127.0.0.1:9008',
          alarms: 'http://127.0.0.1:8050',
          reports: 'http://127.0.0.1:9017',
          bootstrap: 'http://127.0.0.1:9010',
          readers: 'http://127.0.0.1:9011',
          ui: 'http://127.0.0.1:9097',
          http: 'http://127.0.0.1:80',
          certs: 'http://127.0.0.1:80',
          notifiers: process.env.AGRI_NOTIFIER_URL || 'http://127.0.0.1:9014',
        }).map(([service, target]) => [
          `/api/platform/${service}`,
          {
            target,
            changeOrigin: true,
            rewrite: (path: string) => path.replace(`/api/platform/${service}`, ''),
          },
        ]),
      ),
      '/auth': {
        target: 'http://localhost:80',
        changeOrigin: true,
      },
      '/graphql': {
        target: 'http://localhost:80',
        changeOrigin: true,
      },
      '/Users': {
        target: 'http://localhost:80',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/Users/, '/users'),
      },
      '/Domains': {
        target: 'http://localhost:9003',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/Domains/, '/domains'),
      },
      '/Channels': {
        target: 'http://localhost:9005',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/Channels/, ''),
      },
      '/Clients': {
        target: 'http://localhost:9006',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/Clients/, ''),
      },
      '/Messages': {
        target: 'http://localhost:9011',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/Messages/, ''),
      },
      '/image-upload': {
        target: 'http://localhost:18080',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/image-upload/, ''),
      },
      '/agri-api': {
        target: 'http://localhost:8889',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/agri-api/, '/api'),
      },
    },
  },
})
