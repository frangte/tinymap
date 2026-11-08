// Checks for what arrives from the server: everything is unknown until one of these says what it is.

export type Obj = Record<string, unknown>;

/** archetype, app and place ids are short lower_snake_case words (server/content); anything else is dropped */
export const ID_RE = /^[a-z0-9_]{1,32}$/;

export function isObj(v: unknown): v is Obj {
  return typeof v === 'object' && v !== null && !Array.isArray(v);
}

export function num(v: unknown): v is number {
  return typeof v === 'number' && Number.isFinite(v);
}

/** a positive whole number: a player, vehicle or NPC id */
export function isId(v: unknown): v is number {
  return Number.isInteger(v) && (v as number) > 0;
}

/** a string capped to `max` characters, or null */
export function str(v: unknown, max: number): string | null {
  return typeof v === 'string' ? Array.from(v).slice(0, max).join('') : null;
}

/** an archetype, app or place id, '' when absent or malformed */
export function idOf(v: unknown): string {
  return typeof v === 'string' && ID_RE.test(v) ? v : '';
}

/** the valid items of an array (bad ones are skipped), or null if it is not an array */
export function list<T>(v: unknown, item: (x: unknown) => T | null): T[] | null {
  if (!Array.isArray(v)) return null;
  const out: T[] = [];
  for (const x of v) {
    const ok = item(x);
    if (ok) out.push(ok);
  }
  return out;
}
