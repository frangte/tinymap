// What a pedestrian wears: body (nam / nữ), outfit preset and hat. The same ids travel over the network,
// so `parseLook` is the one gate for anything that did not come from this module.

export const BODIES = ['man', 'woman'] as const;
export const OUTFITS = ['thun', 'aodai', 'baba', 'somi', 'xeom'] as const;
export const HATS = ['nonla', 'cap', 'none'] as const;

export type Body = (typeof BODIES)[number];
export type OutfitId = (typeof OUTFITS)[number];
export type HatId = (typeof HATS)[number];

export interface Look {
  body: Body;
  outfit: OutfitId;
  hat: HatId;
}

export type Sleeves = 'short' | 'long';
export type Legs = 'shorts' | 'pants' | 'skirt';

/** Colours and cut of one outfit; `woman` / `man` override fields for that body. */
export interface OutfitStyle {
  /** Vietnamese label for the HUD */
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
}

interface OutfitDef extends OutfitStyle {
  woman?: Partial<OutfitStyle>;
  man?: Partial<OutfitStyle>;
}

/** Colours from the PROJECT.md palette (brick, wallTeal, turq, clockBlue, poGreen, lotusTop, ...). */
const OUTFIT_DEFS: Record<OutfitId, OutfitDef> = {
  thun: {
    name: 'Áo thun', top: '#B5523B', bottom: '#38465C', shoe: '#2B211C', accent: '#4F8F88', bag: '#4F8F88',
    sleeves: 'short', legs: 'shorts', flaps: false,
    woman: { top: '#D29A86', bottom: '#4E7CA6', legs: 'skirt', bag: '#E0B45A' },
  },
  aodai: {
    name: 'Áo dài', top: '#5BA9A1', bottom: '#F3EEE2', shoe: '#2B211C', accent: '#C9A24A', bag: null,
    sleeves: 'long', legs: 'pants', flaps: true,
    man: { top: '#4E7CA6', bottom: '#EEE7D7' },
  },
  baba: {
    name: 'Áo bà ba', top: '#E6D29C', bottom: '#2E2A2B', shoe: '#6B4C36', accent: '#7D3326', bag: null,
    sleeves: 'long', legs: 'pants', flaps: false,
    woman: { top: '#C0613F' },
  },
  somi: {
    name: 'Sơ mi', top: '#EEF0EE', bottom: '#4A4F57', shoe: '#2B211C', accent: '#4E7CA6', bag: '#3B3330',
    sleeves: 'short', legs: 'pants', flaps: false,
    woman: { top: '#F4EBD3', bottom: '#646B74', legs: 'skirt' },
  },
  xeom: {
    name: 'Áo khoác xe ôm', top: '#3E7F5F', bottom: '#4E6A8A', shoe: '#2B211C', accent: '#E0B45A', bag: null,
    sleeves: 'long', legs: 'pants', flaps: false,
  },
};

export const BODY_NAMES: Record<Body, string> = { man: 'Nam', woman: 'Nữ' };
export const HAT_NAMES: Record<HatId, string> = { nonla: 'Nón lá', cap: 'Nón kết', none: 'Không nón' };

export const DEFAULT_LOOK: Readonly<Look> = { body: 'man', outfit: 'thun', hat: 'nonla' };
/** what a body starts in when the player has not picked an outfit yet */
export const DEFAULT_OUTFIT: Record<Body, OutfitId> = { man: 'thun', woman: 'aodai' };

/** The resolved style of `outfit` on `body`. */
export function outfitStyle(outfit: OutfitId, body: Body): OutfitStyle {
  const { woman, man, ...base } = OUTFIT_DEFS[outfit];
  return { ...base, ...(body === 'woman' ? woman : man) };
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
