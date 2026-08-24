import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import path from 'path';

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    port: 8900,
    host: '127.0.0.1',
    proxy: {
      '/v1': {
        target: process.env.VITE_API_BASE_URL || 'http://127.0.0.1:8787',
        changeOrigin: true,
      },
      '/healthz': {
        target: process.env.VITE_API_BASE_URL || 'http://127.0.0.1:8787',
        changeOrigin: true,
      },
      '/readyz': {
        target: process.env.VITE_API_BASE_URL || 'http://127.0.0.1:8787',
        changeOrigin: true,
      },
    },
  },
  test: {
    globals: true,
    environment: 'jsdom',
    setupFiles: './src/test/setup.ts',
  },
});
