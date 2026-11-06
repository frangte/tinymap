// Players this one chose not to hear (personal mute, docs/design/04-backend.md, Moderation): their chat lines and
// speech bubbles are hidden here, nowhere else. Kept by name in the browser (the server gives no lasting id for a
// player), and by player id for this visit, so a renamed player stays muted until they leave.
const STORE_KEY = 'sg.mutes.v1';
/** a muted list longer than this drops its oldest names */
const KEEP = 200;

function load(): string[] {
  try {
    const v: unknown = JSON.parse(localStorage.getItem(STORE_KEY) ?? '[]');
    return Array.isArray(v) ? v.filter((x): x is string => typeof x === 'string').slice(-KEEP) : [];
  } catch {
    return [];
  }
}

export class Mutes {
  private readonly names = new Set<string>(load());
  private readonly ids = new Set<number>();
  private readonly listeners = new Set<() => void>();

  has(id: number, name: string): boolean {
    return this.ids.has(id) || this.names.has(name);
  }

  add(id: number, name: string): void {
    this.ids.add(id);
    this.names.add(name);
    this.changed();
  }

  remove(name: string): void {
    this.names.delete(name);
    // the ids muted under that name go with it (the next line shows again)
    this.ids.clear();
    this.changed();
  }

  /** the names muted, oldest first */
  list(): string[] {
    return [...this.names];
  }

  onChange(fn: () => void): () => void {
    this.listeners.add(fn);
    return () => this.listeners.delete(fn);
  }

  private changed(): void {
    try {
      localStorage.setItem(STORE_KEY, JSON.stringify([...this.names].slice(-KEEP)));
    } catch {
      // storage blocked: muted for this visit only
    }
    for (const fn of this.listeners) fn();
  }
}
