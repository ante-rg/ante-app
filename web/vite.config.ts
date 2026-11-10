import react from '@vitejs/plugin-react';
import { defineConfig } from 'vitest/config';

const api = 'http://localhost:8080';

export default defineConfig({
  plugins: [react()],
  server: { proxy: { '/api': api, '/ingest': api, '/healthz': api } },
  test: { environment: 'jsdom', globals: true, setupFiles: ['src/test/setup.ts'] },
});
