import type { Lang, Text } from './types';
import type { Model } from './model';

/** What the viewer has chosen: kept while the page is open, not saved. */
export interface ViewState {
  lang: Lang;
  /** "Xem như": an archetype id, or '' for everyone */
  origin: string;
  search: string;
  showSide: boolean;
  /** which kinds of arrow the overview draws */
  show: { requires: boolean; cap: boolean; flag: boolean };
}

export const state: ViewState = {
  lang: 'vi',
  origin: '',
  search: '',
  showSide: false,
  show: { requires: true, cap: true, flag: true },
};

/** A text in the chosen language, falling back to the other one. */
export function tx(text: Text | undefined, fallback = ''): string {
  if (!text) return fallback;
  return (state.lang === 'vi' ? text.vi ?? text.en : text.en ?? text.vi) ?? fallback;
}

const ORIGIN_FALLBACK = '#777';

/** The colour of an origin (archetype), the same everywhere on the page. */
export function originColor(model: Model, id: string): string {
  return model.origins.find((o) => o.id === id)?.badge?.color ?? ORIGIN_FALLBACK;
}

export function originName(model: Model, id: string): string {
  return model.origins.find((o) => o.id === id)?.name.vi ?? id;
}

export function originShort(model: Model, id: string): string {
  const o = model.origins.find((x) => x.id === id);
  return o?.badge?.short?.vi ?? o?.name.vi ?? id;
}
