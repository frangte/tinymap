// What a pedestrian wears: body (nam / nữ), outfit preset and hat. The same ids travel over the network,
// so `parseLook` is the one gate for anything that did not come from this module.

import { BODY_NAMES as BODY_NAMES_TEXT, HAT_NAMES as HAT_NAMES_TEXT, OUTFIT_NAMES } from './i18n/outfits';

export const BODIES = ['man', 'woman'] as const;
// crab and helmet are the Crab Bike uniform: who may wear them is in server/content/archetypes.json
export const OUTFITS = ['thun', 'aodai', 'baba', 'somi', 'xeom', 'hoa', 'bikini', 'crab'] as const;
// bun is a hairstyle rather than a hat, but it takes the same place on the head
export const HATS = ['nonla', 'cap', 'bucket', 'helmet', 'bun', 'none'] as const;

export type Body = (typeof BODIES)[number];
export type OutfitId = (typeof OUTFITS)[number];
export type HatId = (typeof HATS)[number];

export interface Look {
  body: Body;
  outfit: OutfitId;
  hat: HatId;
}

export type Sleeves = 'short' | 'long';
/** shorts: bare shins; bare: bare thighs too (swimwear) */
export type Legs = 'shorts' | 'pants' | 'skirt' | 'bare';
/** shirt: the torso in the top colour; bikini: bare with a top band; bare: no top (swim trunks) */
export type Torso = 'shirt' | 'bikini' | 'bare';

/** Colours and cut of one outfit; `woman` / `man` override fields for that body. */
export interface OutfitStyle {
  /** label for the HUD, in the current language */
  name: string;
  top: string;
  bottom: string;
  shoe: string;
  /** cap colour (nón kết) */
  accent: string;
  /** backpack colour, or null for no bag */
  bag: string | null;
  sleeves: Sleeves;
  legs: Legs;
  /** áo dài: long front and back panels from the waist to the shins */
  flaps: boolean;
  torso: Torso;
}

type Cut = Omit<OutfitStyle, 'name'>;

interface OutfitDef extends Cut {
  woman?: Partial<Cut>;
  man?: Partial<Cut>;
}

/** the Crab Bike brand yellow: the jacket, the helmet, the badge (server/content/archetypes.json) */
export const CRAB_YELLOW = '#D9A21B';

/** Colours from the PROJECT.md palette (brick, wallTeal, turq, clockBlue, poGreen, lotusTop, ...). */
const OUTFIT_DEFS: Record<OutfitId, OutfitDef> = {
  thun: {
    top: '#B5523B', bottom: '#38465C', shoe: '#2B211C', accent: '#4F8F88', bag: '#4F8F88',
    sleeves: 'short', legs: 'shorts', flaps: false, torso: 'shirt',
    woman: { top: '#D29A86', bottom: '#4E7CA6', legs: 'skirt', bag: '#E0B45A' },
  },
  aodai: {
    top: '#5BA9A1', bottom: '#F3EEE2', shoe: '#2B211C', accent: '#C9A24A', bag: null,
    sleeves: 'long', legs: 'pants', flaps: true, torso: 'shirt',
    man: { top: '#4E7CA6', bottom: '#EEE7D7' },
  },
  baba: {
    top: '#E6D29C', bottom: '#2E2A2B', shoe: '#6B4C36', accent: '#7D3326', bag: null,
    sleeves: 'long', legs: 'pants', flaps: false, torso: 'shirt',
    woman: { top: '#C0613F' },
  },
  somi: {
    top: '#EEF0EE', bottom: '#4A4F57', shoe: '#2B211C', accent: '#4E7CA6', bag: '#3B3330',
    sleeves: 'short', legs: 'pants', flaps: false, torso: 'shirt',
    woman: { top: '#F4EBD3', bottom: '#646B74', legs: 'skirt' },
  },
  xeom: {
    top: '#3E7F5F', bottom: '#4E6A8A', shoe: '#2B211C', accent: '#E0B45A', bag: null,
    sleeves: 'long', legs: 'pants', flaps: false, torso: 'shirt',
  },
  // the tourist: a loud short-sleeved shirt, khaki shorts, light sneakers and a backpack
  hoa: {
    top: '#E0A93A', bottom: '#C9B48F', shoe: '#EFE8DA', accent: '#B5523B', bag: '#B5523B',
    sleeves: 'short', legs: 'shorts', flaps: false, torso: 'shirt',
  },
  // for the beach: a bikini, or swim trunks and no shirt; dép tông on the feet. `top` is the swatch colour,
  // so the men's swatch shows bare skin over the trunks
  bikini: {
    top: '#C68D66', bottom: '#4E7CA6', shoe: '#B5523B', accent: '#E0B45A', bag: null,
    sleeves: 'short', legs: 'shorts', flaps: false, torso: 'bare',
    woman: { top: '#5BA9A1', bottom: '#5BA9A1', shoe: '#E0B45A', accent: '#B5523B', legs: 'bare', torso: 'bikini' },
  },
  // the Crab Bike driver's yellow jacket (the helmet is the same yellow, character.ts)
  crab: {
    top: CRAB_YELLOW, bottom: '#3B3F4A', shoe: '#2B211C', accent: CRAB_YELLOW, bag: null,
    sleeves: 'long', legs: 'pants', flaps: false, torso: 'shirt',
  },
};

/** names in the current language (src/game/i18n/outfits.ts) */
export const BODY_NAMES: Readonly<Record<Body, string>> = BODY_NAMES_TEXT;
export const HAT_NAMES: Readonly<Record<HatId, string>> = HAT_NAMES_TEXT;

export const DEFAULT_LOOK: Readonly<Look> = { body: 'man', outfit: 'thun', hat: 'nonla' };
/** what a body starts in when the player has not picked an outfit yet */
export const DEFAULT_OUTFIT: Record<Body, OutfitId> = { man: 'thun', woman: 'aodai' };

/** The resolved style of `outfit` on `body`, its name in the current language. */
export function outfitStyle(outfit: OutfitId, body: Body): OutfitStyle {
  const { woman, man, ...base } = OUTFIT_DEFS[outfit];
  const name = outfit === 'bikini' && body === 'woman' ? OUTFIT_NAMES.bikiniWoman : OUTFIT_NAMES[outfit];
  return { ...base, ...(body === 'woman' ? woman : man), name };
}

function oneOf<T extends string>(list: readonly T[], v: unknown): v is T {
  return typeof v === 'string' && (list as readonly string[]).includes(v);
}

/** A Look from untrusted data (network, localStorage), or null if any field is unknown. */
export function parseLook(v: unknown): Look | null {
  if (typeof v !== 'object' || v === null) return null;
  const o = v as Record<string, unknown>;
  if (!oneOf(BODIES, o.body) || !oneOf(OUTFITS, o.outfit) || !oneOf(HATS, o.hat)) return null;
  return { body: o.body, outfit: o.outfit, hat: o.hat };
}

export function sameLook(a: Look, b: Look): boolean {
  return a.body === b.body && a.outfit === b.outfit && a.hat === b.hat;
}
