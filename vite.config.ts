import { readFileSync } from 'node:fs';
import { defineConfig } from 'vite';
import { VitePWA } from 'vite-plugin-pwa';

/** keep the offline copy of fonts for a year; the stylesheet that names them is refreshed in the background */
const FONT_MAX_AGE = 365 * 24 * 60 * 60;
/** the map GLB is about 3 MB; Workbox skips anything over its 2 MB default */
const MAX_PRECACHE_BYTES = 8 * 1024 * 1024;

/**
 * The map files index.html loads (`#stage data-model` and `data-nav`), for the offline copy. Read from the page
 * so a renamed map (saigon_q1_v5.glb → v6) is picked up, and the older maps in public/models are not shipped twice.
 */
function mapFiles(): string[] {
  const html = readFileSync(new URL('index.html', import.meta.url), 'utf8');
  const model = /data-model="([^"]+)"/.exec(html)?.[1];
  const nav = /data-nav="([^"]+)"/.exec(html)?.[1];
  if (!model || !nav) throw new Error('index.html: #stage needs data-model and data-nav (the PWA precache lists them)');
  return [model, `${nav}_nav.json`, `${nav}_nav.png`, `${nav}_roof.png`];
}

// `vite build --mode single` bundles everything into one chunk so scripts/inline.mjs
// can produce a self-contained HTML file for sharing.
// In dev, /ws is forwarded to the Go server (`npm run server`, port 8787). The Host header is kept, so the
// server's same-origin check passes for localhost and for phones on the LAN alike.
//
// The production build is an installable app (PWA): a manifest and a service worker that keeps the page, the code
// and the map on the device (src/pwa.ts registers it). Each file is precached under its content hash, so a re-baked
// nav map or a new GLB under the same name still reaches every player with the next version. Off in dev and in
// the single-file build.
export default defineConfig(({ mode }) => ({
  server: {
    host: true,
    proxy: {
      '/ws': { target: 'ws://localhost:8787', ws: true },
      // the account API (guest tokens), on the same server
      '/api': { target: 'http://localhost:8787' },
    },
  },
  build: {
    target: 'es2020',
    chunkSizeWarningLimit: 1500,
    outDir: mode === 'single' ? 'dist-single' : 'dist',
    rollupOptions: mode === 'single' ? { output: { inlineDynamicImports: true } } : {},
  },
  plugins: [
    VitePWA({
      disable: mode === 'single',
      // src/pwa.ts registers the worker once the map has loaded, and decides when a new version takes over
      injectRegister: false,
      registerType: 'prompt',
      // precached next to the manifest icons (those are added by the plugin)
      includeAssets: ['icons/icon.svg', 'icons/apple-touch-icon.png'],
      manifest: {
        name: 'Sài Gòn Q1',
        short_name: 'Sài Gòn Q1',
        description: 'Quận 1 thu nhỏ: đi bộ, nhận việc, gặp người chơi khác · A walkable miniature of District 1, Saigon',
        lang: 'vi',
        dir: 'ltr',
        categories: ['games'],
        display: 'fullscreen',
        orientation: 'any',
        // --paper in src/style.css: the splash and the system bars match the menu
        background_color: '#f3ebdd',
        theme_color: '#f3ebdd',
        icons: [
          { src: 'icons/icon-192.png', sizes: '192x192', type: 'image/png', purpose: 'any' },
          { src: 'icons/icon-512.png', sizes: '512x512', type: 'image/png', purpose: 'any' },
          { src: 'icons/maskable-512.png', sizes: '512x512', type: 'image/png', purpose: 'maskable' },
        ],
      },
      workbox: {
        globPatterns: ['**/*.{js,css,html}', ...mapFiles()],
        maximumFileSizeToCacheInBytes: MAX_PRECACHE_BYTES,
        cleanupOutdatedCaches: true,
        // any page URL (?lang=en, ?debug, #choi) opens the cached page; the game server's paths stay on the network
        navigateFallback: 'index.html',
        navigateFallbackDenylist: [/^\/(api|ws|healthz)(\/|$)/],
        runtimeCaching: [
          {
            urlPattern: /^https:\/\/fonts\.googleapis\.com\//,
            handler: 'StaleWhileRevalidate',
            options: { cacheName: 'google-fonts-css', cacheableResponse: { statuses: [0, 200] } },
          },
          {
            urlPattern: /^https:\/\/fonts\.gstatic\.com\//,
            handler: 'CacheFirst',
            options: {
              cacheName: 'google-fonts',
              expiration: { maxEntries: 20, maxAgeSeconds: FONT_MAX_AGE },
              cacheableResponse: { statuses: [0, 200] },
            },
          },
        ],
      },
    }),
  ],
}));
