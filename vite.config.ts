import { defineConfig } from 'vite';

// `vite build --mode single` bundles everything into one chunk so scripts/inline.mjs
// can produce a self-contained HTML file for sharing.
// In dev, /ws is forwarded to the Go server (`npm run server`, port 8787, or SG_SERVER_PORT for a server started
// with another PORT). The Host header is kept, so the server's same-origin check passes for localhost and for
// phones on the LAN alike.
// (node's process, without @types/node for this one read)
declare const process: { env: Record<string, string | undefined> };
const serverPort = process.env.SG_SERVER_PORT ?? '8787';

export default defineConfig(({ mode }) => ({
  server: {
    host: true,
    proxy: {
      '/ws': { target: `ws://localhost:${serverPort}`, ws: true },
      // the account API (guest tokens), on the same server
      '/api': { target: `http://localhost:${serverPort}` },
    },
  },
  build: {
    target: 'es2020',
    chunkSizeWarningLimit: 1500,
    outDir: mode === 'single' ? 'dist-single' : 'dist',
    rollupOptions: mode === 'single' ? { output: { inlineDynamicImports: true } } : {},
  },
}));
