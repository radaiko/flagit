import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import { resolve } from 'node:path';

// Two entry points, one build: the in-app overlay and the admin dashboard.
// Both are plain static bundles — the Go binary embeds dist/ and serves them.
export default defineConfig(({ mode }) => ({
  plugins: [svelte()],

  // Under Vitest, Svelte would otherwise resolve to its server build and
  // mount() would be unavailable. jsdom is a browser, so say so.
  resolve: mode === 'test' ? { conditions: ['browser'] } : {},

  build: {
    outDir: 'dist',
    emptyOutDir: true,
    rollupOptions: {
      input: {
        overlay: resolve(import.meta.dirname, 'overlay.html'),
        dashboard: resolve(import.meta.dirname, 'dashboard.html'),
      },
    },
  },

  server: { port: 5173 },

  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./tests/setup.js'],
    include: ['tests/**/*.test.js'],
    coverage: {
      provider: 'v8',
      include: ['src/**/*.{js,svelte}'],
      // Bootstrap files only mount a component into the DOM; the component
      // tests already cover everything they do.
      exclude: ['src/**/main.js'],
      reporter: ['text', 'html'],
      thresholds: { lines: 90, functions: 90, statements: 90 },
    },
  },
}));
