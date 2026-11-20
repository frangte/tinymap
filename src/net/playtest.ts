// "Chơi thử từ đây" (docs/design/14-story-tools.md): /?playtest=<token> opens the game as a test character that the
// dev server made in a chosen story state (POST /api/dev/playtest, from the Xưởng truyện). This module is the first
// import of main.ts, so it runs before anything reads the browser's storage: in a test tab `localStorage` is
// replaced by a copy in memory that starts with the test token and profile. The player's own saved character, token,
// pose and settings in this browser are neither read nor written, and the tab forgets the test when it closes.
// It imports nothing on purpose: nothing may touch storage before it has run.

const TOKEN_RE = /^[A-Za-z0-9_-]{20,64}$/;
const ID_RE = /^[a-z0-9_]{1,32}$/;
/** a quest id: words joined by dots (quests.json) */
const QUEST_RE = /^[a-z0-9_]+(\.[a-z0-9_]+)*$/;
/** the profile's name limit (src/game/profile.ts NAME_MAX) */
const NAME_MAX = 16;

/** the storage keys the game uses for the token and the profile (src/net/auth.ts, src/game/profile.ts) */
const TOKEN_KEY = 'sg.token.v1';
const PROFILE_KEY = 'sg.profile.v1';
/** the quest the tracker follows (src/game/questLog.ts) */
const TRACK_KEY = 'sg.track.v1';

/** A Storage that lives in memory. */
class MemoryStorage implements Storage {
  private readonly items = new Map<string, string>();
  get length(): number {
    return this.items.size;
  }
  clear(): void {
    this.items.clear();
  }
  getItem(key: string): string | null {
    return this.items.get(key) ?? null;
  }
  key(index: number): string | null {
    return [...this.items.keys()][index] ?? null;
  }
  removeItem(key: string): void {
    this.items.delete(key);
  }
  setItem(key: string, value: string): void {
    this.items.set(key, String(value));
  }
  [name: string]: unknown;
}

/** What the tab was opened with; null for a normal visit. */
export interface PlaytestSession {
  token: string;
  name: string;
}

function readSession(): { session: PlaytestSession; profile: string; track: string } | null {
  const q = new URLSearchParams(window.location.search);
  const token = q.get('playtest') ?? '';
  if (!TOKEN_RE.test(token)) return null;
  const arch = q.get('ptarch') ?? '';
  const [body, outfit, hat] = (q.get('ptlook') ?? '').split('.');
  const name = Array.from(q.get('ptname') ?? '').slice(0, NAME_MAX).join('').trim();
  if (!ID_RE.test(arch) || ![body, outfit, hat].every((s) => s !== undefined && ID_RE.test(s)) || !name) return null;
  const quest = q.get('ptquest') ?? '';
  return { session: { token, name }, track: QUEST_RE.test(quest) ? quest : '', profile: JSON.stringify({ name, look: { body, outfit, hat }, archetype: arch }) };
}

function install(): PlaytestSession | null {
  const found = readSession();
  if (!found) return null;
  const memory = new MemoryStorage();
  memory.setItem(TOKEN_KEY, found.session.token);
  memory.setItem(PROFILE_KEY, found.profile);
  if (found.track) memory.setItem(TRACK_KEY, found.track);
  try {
    Object.defineProperty(window, 'localStorage', { value: memory, configurable: true });
  } catch {
    // the browser will not let go of its storage: better no test tab than one that overwrites the player's save
    throw new Error('playtest: cannot replace localStorage in this browser');
  }
  return found.session;
}

/** Set when this tab is a test character's (null otherwise). */
export const playtest: PlaytestSession | null = install();
