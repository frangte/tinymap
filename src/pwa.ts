// The game as an installable app (PWA). The service worker vite-plugin-pwa builds (vite.config.ts) keeps the page,
// the code and the map on the device: an installed game opens at once, and offline too (walking needs no server;
// multiplayer reconnects by itself when the network is back).
//
// A new version must not run against an old page: the client and the server change together and the socket has
// no version check. So a downloaded update takes over in the menu straight away (nothing to lose there), and in
// the map or a game the banner asks for a reload, which also happens by itself on the next return to the menu.
import { registerSW } from 'virtual:pwa-register';
import { el } from './game/hud';

/** look for a new version this often while the app stays open (an installed app can run for days) */
const UPDATE_CHECK_MS = 60 * 60 * 1000;

function inMenu(): boolean {
  return document.body.dataset.mode === 'menu';
}

/** Register the service worker. Call it once the map has loaded, so its own download does not slow the first one. */
export function startPwa(): void {
  if (!('serviceWorker' in navigator)) return;
  const banner = el('update');
  let waiting = false;

  const updateSW = registerSW({
    onNeedRefresh() {
      waiting = true;
      if (inMenu()) void updateSW(true);
      else banner.hidden = false;
    },
    onRegisteredSW(_url, registration) {
      if (!registration) return;
      const check = (): void => {
        if (!navigator.onLine || registration.installing) return;
        registration.update().catch(() => {
          // the server is unreachable: the next check tries again
        });
      };
      window.setInterval(check, UPDATE_CHECK_MS);
      // a phone brings the app back from the background far more often than it starts it
      document.addEventListener('visibilitychange', () => {
        if (document.visibilityState === 'visible') check();
      });
    },
    onRegisterError(err: unknown) {
      console.error('service worker registration failed', err);
    },
  });

  el('update-reload').addEventListener('click', () => void updateSW(true));
  el('update-later').addEventListener('click', () => {
    banner.hidden = true;
  });
  // back in the menu with an update waiting: take it now
  new MutationObserver(() => {
    if (waiting && inMenu()) void updateSW(true);
  }).observe(document.body, { attributes: true, attributeFilter: ['data-mode'] });
}
