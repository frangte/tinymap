// The Cẩm nang's pages about places (docs/design/10-story.md, section 10): what the info card in the street shows
// (placeCard.ts) and the page in the Sổ tay (notebook/guideBook.ts). The text lives in server/content/guide.json,
// whose Go tests keep every page about a real place and in both languages. Which pages this player has read is
// kept on this device only: nothing depends on it yet.
import data from '../../server/content/guide.json';
import type { Bilingual } from './i18n';
import { placeById, type Place } from './places';

/** the quick facts at the top of a page; each is optional */
export interface GuideFacts {
  readonly built?: Bilingual;
  readonly hours?: Bilingual;
  readonly ticket?: Bilingual;
}

export interface GuidePage {
  readonly place: Place;
  /** the day the facts that change (hours, prices, works) were last checked, YYYY-MM-DD */
  readonly checked: string;
  readonly summary: Bilingual;
  readonly facts: GuideFacts;
  readonly history: readonly Bilingual[];
  readonly see: readonly Bilingual[];
  readonly tips: readonly Bilingual[];
  readonly gettingThere: readonly Bilingual[];
  readonly food: readonly Bilingual[];
  /** what to search for on Google Maps */
  readonly maps: string;
  readonly sources: readonly string[];
}

interface RawEntry {
  place: string;
  checked: string;
  summary: Bilingual;
  facts: GuideFacts;
  history: Bilingual[];
  see: Bilingual[];
  tips: Bilingual[];
  getting_there: Bilingual[];
  food: Bilingual[];
  maps: string;
  sources: string[];
}

const FILE: { readonly entries: readonly RawEntry[] } = data;

/** every page, in the file's order; a page about a place this build does not know is left out */
export const GUIDE: readonly GuidePage[] = FILE.entries.flatMap((e): GuidePage[] => {
  const place = placeById(e.place);
  if (!place) return [];
  return [{
    place,
    checked: e.checked,
    summary: e.summary,
    facts: e.facts,
    history: e.history,
    see: e.see,
    tips: e.tips,
    gettingThere: e.getting_there,
    food: e.food,
    maps: e.maps,
    sources: e.sources,
  }];
});

const BY_PLACE = new Map(GUIDE.map((g) => [g.place.id, g]));

/** The page about this place, if there is one. */
export function guideFor(placeId: string): GuidePage | undefined {
  return BY_PLACE.get(placeId);
}

/** Google Maps searching for the real place. */
export function mapsLink(page: GuidePage): string {
  return `https://www.google.com/maps/search/?api=1&query=${encodeURIComponent(page.maps)}`;
}

const READ_KEY = 'sg.guide.read.v1';

/** The pages this player has opened, kept in this browser (storage may be blocked: then for this visit only). */
export class ReadPages {
  private readonly ids: Set<string>;
  private readonly listeners = new Set<() => void>();

  constructor() {
    this.ids = new Set(ReadPages.load());
  }

  private static load(): string[] {
    try {
      const raw: unknown = JSON.parse(localStorage.getItem(READ_KEY) ?? '[]');
      return Array.isArray(raw) ? raw.filter((v): v is string => typeof v === 'string' && BY_PLACE.has(v)) : [];
    } catch {
      return [];
    }
  }

  has(placeId: string): boolean {
    return this.ids.has(placeId);
  }

  get count(): number {
    return this.ids.size;
  }

  add(placeId: string): void {
    if (this.ids.has(placeId) || !BY_PLACE.has(placeId)) return;
    this.ids.add(placeId);
    try {
      localStorage.setItem(READ_KEY, JSON.stringify([...this.ids]));
    } catch {
      // storage blocked: remembered until the page is reloaded
    }
    for (const fn of this.listeners) fn();
  }

  /** Call `fn` when a page is read for the first time; returns the function that stops it. */
  onChange(fn: () => void): () => void {
    this.listeners.add(fn);
    return () => {
      this.listeners.delete(fn);
    };
  }
}
