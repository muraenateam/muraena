import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

export default defineConfig({
  plugins: [svelte()],
  build: { outDir: 'dist', emptyOutDir: true },
  // dev only: proxy REST + WebSocket (/api/v1/ws/*) to the Go API server
  server: {
    proxy: {
      '/api': { target: 'http://127.0.0.1:8443', ws: true, changeOrigin: true },
    },
  },
});
