import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import { resolve } from 'path'

export default defineConfig({
  plugins: [
    vue(),
    tailwindcss(),
  ],
  resolve: {
    alias: {
      '@': resolve(import.meta.dirname, 'src'),
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'https://crm-test-production-6d2d.up.railway.app',
        changeOrigin: true,
      },
      '/webhook': {
        target: 'https://crm-test-production-6d2d.up.railway.app',
        changeOrigin: true,
      },
    },
  },
})
