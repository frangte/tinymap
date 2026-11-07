// The playable cast: each entry is a paint job + a few build switches for the primitive pedestrian rig in
// character.ts. The pick is remembered in localStorage so the next visit starts as the same person.

export type Headwear = 'nonla' | 'bun' | 'helmet' | 'bucket';

export interface CharacterStyle {
  readonly id: string;
  readonly name: string;
  readonly note: string;
  readonly skin: string;
  /** shirt / tunic */
  readonly top: string;
  /** shorts or trousers */
  readonly bottom: string;
  readonly hat: string;
  readonly hair: string;
  readonly shoe: string;
  readonly bag: string;
  readonly headwear: Headwear;
  /** trousers reach the ankle; otherwise shorts stop at the knee */
  readonly trousers: boolean;
  /** sleeves cover the whole arm */
  readonly longSleeves: boolean;
  /** áo dài: tunic panels hanging front and back */
  readonly tunic: boolean;
  readonly backpack: boolean;
}

export const CHARACTERS: readonly CharacterStyle[] = [
  {
    id: 'cho',
    name: 'Người đi chợ',
    note: 'nón lá, ba lô',
    skin: '#C68D66',
    top: '#B5523B',
    bottom: '#38465C',
    hat: '#E0C68C',
    hair: '#35261E',
    shoe: '#2B211C',
    bag: '#4F8F88',
    headwear: 'nonla',
    trousers: false,
    longSleeves: false,
    tunic: false,
    backpack: true,
  },
  {
    id: 'aodai',
    name: 'Nữ sinh áo dài',
    note: 'tóc búi, quần trắng',
    skin: '#D9A882',
    top: '#D8798D',
    bottom: '#EFE8DA',
    hat: '#35261E',
    hair: '#35261E',
    shoe: '#2B211C',
    bag: '#4F8F88',
    headwear: 'bun',
    trousers: true,
    longSleeves: true,
    tunic: true,
    backpack: false,
  },
  {
    id: 'xeom',
    name: 'Bác xe ôm',
    note: 'mũ bảo hiểm, áo xanh',
    skin: '#B8795A',
    top: '#6F7B4B',
    bottom: '#2F3140',
    hat: '#3C6F92',
    hair: '#35261E',
    shoe: '#2B211C',
    bag: '#4F8F88',
    headwear: 'helmet',
    trousers: true,
    longSleeves: true,
    tunic: false,
    backpack: false,
  },
  {
    id: 'khach',
    name: 'Khách du lịch',
    note: 'mũ tai bèo, áo hoa',
    skin: '#E2B59A',
    top: '#E0A93A',
    bottom: '#C9B48F',
    hat: '#D7C7A2',
    hair: '#6B4A2E',
    shoe: '#EFE8DA',
    bag: '#B5523B',
    headwear: 'bucket',
    trousers: false,
    longSleeves: false,
    tunic: false,
    backpack: true,
  },
];

const STORAGE_KEY = 'sg-q1-character';

export function characterIndex(id: string | null | undefined): number {
  const i = CHARACTERS.findIndex((c) => c.id === id);
  return i < 0 ? 0 : i;
}

/** The remembered pick, or the first of the cast. localStorage may be unavailable (private mode). */
export function loadCharacterIndex(): number {
  try {
    return characterIndex(localStorage.getItem(STORAGE_KEY));
  } catch {
    return 0;
  }
}

export function saveCharacterIndex(i: number): void {
  try {
    localStorage.setItem(STORAGE_KEY, CHARACTERS[i]?.id ?? CHARACTERS[0].id);
  } catch {
    // storage blocked: the pick just lasts for this visit
  }
}
