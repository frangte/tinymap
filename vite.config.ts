import { defineConfig } from 'vite';

// `vite build --mode single` bundles everything into one chunk so scripts/inline.mjs
// can produce a self-contained HTML file for sharing.
export default defineConfig(({ mode }) => ({
  server: { host: true },
  build: {
    target: 'es2020',
    chunkSizeWarningLimit: 1500,
    outDir: mode === 'single' ? 'dist-single' : 'dist',
    rollupOptions: mode === 'single' ? { output: { inlineDynamicImports: true } } : {},
  },
}));
