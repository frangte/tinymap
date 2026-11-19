// Travel tips (docs/design/10-story.md, section 10): a line that shows the first time a situation happens in play,
// then stays in the Sổ mẹo at the end of the Cẩm nang. The text and the triggers are content
// (server/content/tips.json, checked by its Go tests); which tips this player has seen is kept on this device only.
import data from '../../server/content/tips.json';
import type { Bilingual } from './i18n';
import type { Place } from './places';

/** Something that happened in play; app.ts reports each one (see TipCard.happened). */
export type TipEvent =
  | { readonly kind: 'place'; readonly place: Place }
  | { readonly kind: 'npc'; readonly npc: string }
  | { readonly kind: 'crab_booked' }
  | { readonly kind: 'fare_paid' }
  | { readonly kind: 'phone_in_street' };

interface TipTrigger {
  readonly kind: string;
  readonly place_kind?: string;
  readonly place?: string;
  readonly npc?: string;
}

export interface Tip {
  readonly id: string;
  readonly title: Bilingual;
  readonly text: Bilingual;
  readonly trigger: TipTrigger;
}

const FILE: { readonly tips: readonly Tip[] } = data;

/** every tip, in the file's order */
export const TIPS: readonly Tip[] = FILE.tips;

/** Does this happening show this tip? Kinds the game does not wire never match. */
export function triggers(tip: Tip, ev: TipEvent): boolean {
  const t = tip.trigger;
  switch (ev.kind) {
    case 'place':
      return (t.kind === 'place' && t.place === ev.place.id) || (t.kind === 'place_kind' && t.place_kind === ev.place.kind);
    case 'npc':
      return t.kind === 'npc' && t.npc === ev.npc;
    default:
      return t.kind === ev.kind;
  }
}

const SEEN_KEY = 'sg.tips.seen.v1';
const KNOWN = new Set(TIPS.map((t) => t.id));

/** The tips this player has seen, kept in this browser (storage may be blocked: then for this visit only). */
export class TipBook {
  private readonly ids: Set<string>;
  private readonly listeners = new Set<() => void>();

  constructor() {
    this.ids = new Set(TipBook.load());
  }

  private static load(): string[] {
    try {
      const raw: unknown = JSON.parse(localStorage.getItem(SEEN_KEY) ?? '[]');
      return Array.isArray(raw) ? raw.filter((v): v is string => typeof v === 'string' && KNOWN.has(v)) : [];
    } catch {
      return [];
    }
  }

  has(id: string): boolean {
    return this.ids.has(id);
  }

  get count(): number {
    return this.ids.size;
  }

  add(id: string): void {
    if (this.ids.has(id) || !KNOWN.has(id)) return;
    this.ids.add(id);
    try {
      localStorage.setItem(SEEN_KEY, JSON.stringify([...this.ids]));
    } catch {
      // storage blocked: remembered until the page is reloaded
    }
    for (const fn of this.listeners) fn();
  }

  /** Call `fn` when a tip is seen for the first time; returns the function that stops it. */
  onChange(fn: () => void): () => void {
    this.listeners.add(fn);
    return () => {
      this.listeners.delete(fn);
    };
  }
}

/** the one book of this page: the card writes it, the Sổ mẹo reads it */
export const TIP_BOOK = new TipBook();
