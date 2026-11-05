import { defineConfig } from 'vite';

// `vite build --mode single` bundles everything into one chunk so scripts/inline.mjs
// can produce a self-contained HTML file for sharing.
// In dev, /ws is forwarded to the Go server (`npm run server`, port 8787). The Host header is kept, so the
// server's same-origin check passes for localhost and for phones on the LAN alike.
export default defineConfig(({ mode }) => ({
  server: {
    host: true,
    proxy: { '/ws': { target: 'ws://localhost:8787', ws: true } },
  },
  build: {
    target: 'es2020',
    chunkSizeWarningLimit: 1500,
    outDir: mode === 'single' ? 'dist-single' : 'dist',
    rollupOptions: mode === 'single' ? { output: { inlineDynamicImports: true } } : {},
  },
}));
