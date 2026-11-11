// The game in Vietnamese or English. The language belongs to the player, not the archetype
// (docs/design/01-gameplay.md, Language), and is kept on this device: ?lang= in the URL wins for one visit, then
// the choice saved by the switch, then the browser's languages (the first of vi* / en* it lists; English when
// neither is there, since the first look at Saigon is for people who do not know it yet).
//
// Two kinds of text:
//  - interface text lives in catalogs next to this file: `vi` is the source and `en` is typed `typeof vi`, so tsc
//    fails on a missing key or an entry that takes other arguments. Read one with catalog({ vi, en }).key.
//  - content text (quests, dialogue, places, apps…) is a { vi, en } pair in server/content; tx() picks one.
//
// Switching is live: setLang() tells every listener, and each view that keeps text on screen redraws it.

export type Lang = 'vi' | 'en';

export const LANGS: readonly Lang[] = ['vi', 'en'];

/** a content text: Vietnamese always, English when there is one */
export interface Bilingual {
  readonly vi: string;
  readonly en?: string;
}

const STORE_KEY = 'sg.lang.v1';

function isLang(v: unknown): v is Lang {
  return v === 'vi' || v === 'en';
}

function fromUrl(): Lang | null {
  const v = new URLSearchParams(window.location.search).get('lang');
  return isLang(v) ? v : null;
}

function saved(): Lang | null {
  try {
    const v = localStorage.getItem(STORE_KEY);
    return isLang(v) ? v : null;
  } catch {
    // storage blocked (private mode, sandboxed frame): the browser's languages decide
    return null;
  }
}

function fromBrowser(): Lang {
  const langs = navigator.languages?.length ? navigator.languages : [navigator.language];
  for (const l of langs) {
    const code = l.toLowerCase();
    if (code.startsWith('vi')) return 'vi';
    if (code.startsWith('en')) return 'en';
  }
  return 'en';
}

let current: Lang = fromUrl() ?? saved() ?? fromBrowser();
document.documentElement.lang = current;
document.documentElement.dataset.lang = current;

const listeners = new Set<(lang: Lang) => void>();

/** The language the game is shown in now. */
export function lang(): Lang {
  return current;
}

/** Switch language, remember it on this device and redraw every view that listens. */
export function setLang(next: Lang): void {
  if (next === current) return;
  current = next;
  try {
    localStorage.setItem(STORE_KEY, next);
  } catch {
    // storage blocked: the choice lasts until the page is reloaded
  }
  document.documentElement.lang = next;
  document.documentElement.dataset.lang = next;
  for (const fn of listeners) fn(next);
}

/** Call `fn` after every switch; returns the function that stops it. */
export function onLangChange(fn: (lang: Lang) => void): () => void {
  listeners.add(fn);
  return () => {
    listeners.delete(fn);
  };
}

/** A content text in the current language (Vietnamese when it has no English). */
export function tx(t: Bilingual): string {
  return current === 'en' && t.en ? t.en : t.vi;
}

type Entry = string | ((...args: never[]) => string);

/**
 * One catalog of interface text in both languages: reading a key gives the entry in the current language.
 * Write it as `const vi = {…}; const en: typeof vi = {…}; export const L = catalog({ vi, en });`.
 */
export function catalog<T extends Record<string, Entry>>(texts: { readonly vi: T; readonly en: T }): T {
  return new Proxy(texts.vi, {
    get: (_target, key) => (texts[current] as Record<string | symbol, unknown>)[key],
  });
}
