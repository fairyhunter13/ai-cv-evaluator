import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src'),
    },
  },
  server: {
    port: 3001,
    proxy: {
      // Proxy API calls to the backend
      '/v1': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        secure: false,
      },
      '/admin/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        secure: false,
      },
      // Proxy auth endpoints
      '/admin/login': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        secure: false,
      },
      '/admin/logout': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        secure: false,
      },
    },
  },
  build: {
    outDir: 'dist',
    assetsDir: 'assets',
  },
})
