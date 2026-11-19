// The start menu's "install the app" button, for phones and tablets (README, "Installing as an app").
// Chrome on Android (and the other Chromium browsers) hands the page a `beforeinstallprompt` event, so there the
// button opens the browser's own install dialog. Safari on iPhone and iPad has no such event, nor do the other
// Android browsers or the browsers inside Zalo, Facebook and the like, so there it unfolds the steps to do by hand.
// Hidden when the game already runs as an app, on computers, and in builds without a manifest (npm run dev, the
// single-file build), which cannot be installed.
import { el } from './game/hud';
import { onLangChange } from './game/i18n';
import { L } from './game/i18n/install';
import { icon } from './game/phone/icons';

/** Chromium's install offer (not in lib.dom): prompt() once, then userChoice says what the player did */
interface InstallPromptEvent extends Event {
  prompt(): Promise<void>;
  readonly userChoice: Promise<{ outcome: 'accepted' | 'dismissed' }>;
}

type Platform = 'ios' | 'android' | 'inApp';

interface Step {
  /** the button the step points at, by its name in game/phone/icons.ts */
  icon: string;
  text: () => string;
}

const STEPS: Record<Platform, readonly Step[]> = {
  ios: [
    { icon: 'share', text: () => L.iosShare },
    { icon: 'addHome', text: () => L.iosAdd },
    { icon: 'home', text: () => L.iosDone },
  ],
  android: [
    { icon: 'menuDots', text: () => L.androidMenu },
    { icon: 'addHome', text: () => L.androidAdd },
    { icon: 'home', text: () => L.androidDone },
  ],
  inApp: [
    { icon: 'menuDots', text: () => L.inAppMenu },
    { icon: 'browser', text: () => L.inAppOpen },
    { icon: 'addHome', text: () => L.inAppAgain(L.button) },
  ],
};

/** the browsers inside other apps (and Android's bare web views): they cannot install a page */
const IN_APP = /FBAN|FBAV|FB_IAB|Instagram|Messenger|Zalo|Line\/|TikTok|musical_ly|BytedanceWebview|; wv\)/i;
/** the ways an installed copy can run; the manifest asks for fullscreen, a browser may fall back to the others */
const APP_DISPLAY = ['fullscreen', 'standalone', 'minimal-ui'];

function platform(): Platform | null {
  const ua = navigator.userAgent;
  // iPadOS asks for the desktop site and says Macintosh; only its touch screen gives it away
  const ios = /iPhone|iPad|iPod/.test(ua) || (/Macintosh/.test(ua) && navigator.maxTouchPoints > 1);
  if (!ios && !/Android/.test(ua)) return null;
  if (IN_APP.test(ua)) return 'inApp';
  return ios ? 'ios' : 'android';
}

function runningAsApp(): boolean {
  const iosHomeScreen = (navigator as Navigator & { standalone?: boolean }).standalone === true;
  return iosHomeScreen || APP_DISPLAY.some((m) => window.matchMedia(`(display-mode: ${m})`).matches);
}

function stepItem(step: Step): HTMLLIElement {
  const li = document.createElement('li');
  const text = document.createElement('span');
  text.textContent = step.text();
  li.append(icon(step.icon), text);
  return li;
}

/** Show the button where it can help. Call it before the map loads: Chrome may offer the install early. */
export function startInstallButton(): void {
  const where = platform();
  if (!where || runningAsApp() || !document.querySelector('link[rel="manifest"]')) return;
  const root = el('install');
  const open = el<HTMLButtonElement>('install-open');
  const steps = el('install-steps');
  const lead = el('install-lead');
  const list = el('install-list');
  let offer: InstallPromptEvent | null = null;

  window.addEventListener('beforeinstallprompt', (e) => {
    // our button instead of Chrome's own bar at the bottom of the screen
    e.preventDefault();
    offer = e as InstallPromptEvent;
  });
  window.addEventListener('appinstalled', () => {
    root.hidden = true;
  });

  const ask = async (prompt: InstallPromptEvent): Promise<void> => {
    try {
      await prompt.prompt();
      const { outcome } = await prompt.userChoice;
      if (outcome === 'accepted') root.hidden = true;
    } catch (err: unknown) {
      console.error('install prompt failed', err);
    }
  };

  open.addEventListener('click', () => {
    // an offer can be shown once; after a "no" the steps by hand are what is left
    if (offer) {
      void ask(offer);
      offer = null;
      return;
    }
    const show = steps.hidden;
    steps.hidden = !show;
    open.setAttribute('aria-expanded', String(show));
    if (show) steps.scrollIntoView({ block: 'nearest' });
  });

  const fill = (): void => {
    el('install-label').textContent = L.button;
    el('install-note').textContent = L.note;
    steps.setAttribute('aria-label', L.stepsLabel);
    lead.hidden = where !== 'inApp';
    lead.textContent = where === 'inApp' ? L.inAppLead : '';
    list.replaceChildren(...STEPS[where].map(stepItem));
  };
  fill();
  onLangChange(fill);
  root.hidden = false;
}
