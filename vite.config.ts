import { readFileSync } from 'node:fs';
import { defineConfig, type Plugin } from 'vite';
import { VitePWA } from 'vite-plugin-pwa';
import { storyEditor } from './tools/story/vite-plugin';

/** keep the offline copy of fonts for a year; the stylesheet that names them is refreshed in the background */
const FONT_MAX_AGE = 365 * 24 * 60 * 60;
/** the map GLB is about 4 MB; Workbox skips anything over its 2 MB default */
const MAX_PRECACHE_BYTES = 8 * 1024 * 1024;

/**
 * The map file index.html loads (`#stage data-model`), for the offline copy. Read from the page so a renamed map
 * (saigon_mini.glb → the next one) is picked up, and the older maps in public/models are not shipped twice.
 */
function mapFile(): string {
  const html = readFileSync(new URL('index.html', import.meta.url), 'utf8');
  const model = /data-model="([^"]+)"/.exec(html)?.[1];
  if (!model) throw new Error('index.html: #stage needs data-model (the PWA precache lists it)');
  return model;
}

// `vite build --mode single` bundles everything into one chunk so scripts/inline.mjs
// can produce a self-contained HTML file for sharing.
// In dev, /ws is forwarded to the Go server (`npm run server`, port 8787, or SG_SERVER_PORT for a server started
// with another PORT). The Host header is kept, so the server's same-origin check passes for localhost and for
// phones on the LAN alike.
//
// The production build is an installable app (PWA): a manifest and a service worker that keeps the page, the code
// and the map on the device (src/pwa.ts registers it). Each file is precached under its content hash, so a new GLB
// under the same name still reaches every player with the next version. Off in dev and in the single-file build.
/**
 * Dev only (`apply: 'serve'`, so it is not in the build): the dev server listens on the LAN and its proxy reaches the
 * Go server from loopback, so the Go route's own loopback check could not tell a phone on the Wi-Fi from this
 * machine. This answers 403 to /api/dev/ for anyone who is not on this machine, before the proxy sees it. The Go
 * route is also off unless the server runs with SG_DEV_TOOLS=1 (server/internal/web/playtest.go).
 */
function localOnlyDevApi(): Plugin {
  const loopback = /^(127\.\d+\.\d+\.\d+|::1|::ffff:127\.\d+\.\d+\.\d+)$/;
  return {
    name: 'sg-local-only-dev-api',
    apply: 'serve',
    configureServer(server) {
      server.middlewares.use('/api/dev', (req, res, next) => {
        if (loopback.test(req.socket.remoteAddress ?? '')) return next();
        res.statusCode = 403;
        res.end('local only');
      });
    },
  };
}

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
  plugins: [
    // the Xưởng truyện's load, save and check endpoints (tools/story/), and /api/dev for this machine only; dev server only
    storyEditor(),
    localOnlyDevApi(),
    VitePWA({
      disable: mode === 'single',
      // src/pwa.ts registers the worker once the map has loaded, and decides when a new version takes over
      injectRegister: false,
      registerType: 'prompt',
      // precached next to the manifest icons (those are added by the plugin)
      includeAssets: ['icons/icon.svg', 'icons/apple-touch-icon.png'],
      manifest: {
        // the page title and the menu's heading (index.html)
        name: 'Sài Gòn mini',
        short_name: 'Sài Gòn mini',
        description: 'Sài Gòn vẽ tay, thu nhỏ giữa biển · A hand-drawn Saigon in miniature, set in the sea',
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
        globPatterns: ['**/*.{js,css,html}', mapFile()],
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
