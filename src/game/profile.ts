// The player's display name and look, remembered in localStorage so the next visit walks straight in.
// Storage can be missing (private windows, blocked site data): the game then just asks again next time.
import { DEFAULT_LOOK, parseLook, type Look } from './outfits';
import { archetypeById } from './archetypes';

export interface Profile {
  name: string;
  look: Look;
  /** archetype id (server/content/archetypes.json); '' until the player has picked one */
  archetype: string;
}

const STORAGE_KEY = 'sg.profile.v1';
export const NAME_MAX = 16;

/** Trim, drop control / invisible format characters, collapse spaces and cap the length (in characters). */
export function cleanName(raw: string): string {
  const flat = raw.replace(/[\p{Cc}\p{Cf}]/gu, '').replace(/\s+/g, ' ').trim();
  return Array.from(flat).slice(0, NAME_MAX).join('').trim();
}

export function randomName(): string {
  return `Khách ${100 + Math.floor(Math.random() * 900)}`;
}

export function defaultProfile(): Profile {
  return { name: randomName(), look: { ...DEFAULT_LOOK }, archetype: '' };
}

/** The saved profile, or null when there is none, storage is blocked, or the entry no longer validates. */
export function loadProfile(): Profile | null {
  let raw: string | null;
  try {
    raw = localStorage.getItem(STORAGE_KEY);
  } catch {
    return null; // storage blocked: ask again
  }
  if (!raw) return null;
  try {
    const data = JSON.parse(raw) as { name?: unknown; look?: unknown; archetype?: unknown };
    const look = parseLook(data.look);
    const name = typeof data.name === 'string' ? cleanName(data.name) : '';
    // saved before archetypes existed, or an id this build does not know: the picker asks again
    const archetype = typeof data.archetype === 'string' && archetypeById(data.archetype) ? data.archetype : '';
    return look && name ? { name, look, archetype } : null;
  } catch {
    return null; // an older or hand-edited entry: start over
  }
}

export function saveProfile(p: Profile): void {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(p));
  } catch {
    // storage blocked: the choice just lasts for this visit
  }
}
