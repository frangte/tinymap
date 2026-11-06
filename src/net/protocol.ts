// Wire format between the game and the Go server; server/README.md is the spec. Everything that arrives is
// checked here before the game sees it: the server cleans what clients send, but the client trusts nothing either.
import { parseLook, type Look } from '../game/outfits';
import { NAME_MAX } from '../game/profile';

export const CHAT_MAX = 140;

/** [x, y, z, heading] in three.js metres / radians */
export type Pose = [number, number, number, number];

export interface PlayerInfo {
  id: number;
  name: string;
  look: Look;
  pose: Pose | null;
}

export interface ChatLine {
  id: number;
  name: string;
  text: string;
  /** server time, ms since the epoch */
  at: number;
}

/** auth: the hello's account token is unknown to the server (forget it and get a new one) */
export type ErrorCode = 'full' | 'bad_hello' | 'rate' | 'auth' | 'unknown';

/** [id, x, y, z, heading, age]: age = ms between the server receiving this pose and sending the snapshot */
export type SnapRow = [number, number, number, number, number, number];

export type ServerMsg =
  | { t: 'welcome'; id: number; name: string; max: number; players: PlayerInfo[]; chat: ChatLine[] }
  | { t: 'join'; p: PlayerInfo }
  | { t: 'leave'; id: number }
  | { t: 'profile'; id: number; name: string; look: Look }
  | { t: 'snap'; at: number; rows: SnapRow[] }
  | ({ t: 'chat' } & ChatLine)
  | { t: 'error'; code: ErrorCode };

export type ClientMsg =
  /** token: the guest account (src/net/auth.ts); absent plays as a guest without one */
  | { t: 'hello'; token?: string; name: string; look: Look }
  | { t: 'profile'; name: string; look: Look }
  | { t: 'state'; x: number; y: number; z: number; h: number }
  | { t: 'chat'; text: string };

type Obj = Record<string, unknown>;

function isObj(v: unknown): v is Obj {
  return typeof v === 'object' && v !== null && !Array.isArray(v);
}
function num(v: unknown): v is number {
  return typeof v === 'number' && Number.isFinite(v);
}
function isId(v: unknown): v is number {
  return Number.isInteger(v) && (v as number) > 0;
}
/** a string capped to `max` characters, or null */
function str(v: unknown, max: number): string | null {
  return typeof v === 'string' ? Array.from(v).slice(0, max).join('') : null;
}
function pose(v: unknown): Pose | null {
  return Array.isArray(v) && v.length === 4 && v.every(num) ? [v[0], v[1], v[2], v[3]] : null;
}

function player(v: unknown): PlayerInfo | null {
  if (!isObj(v) || !isId(v.id)) return null;
  const name = str(v.name, NAME_MAX);
  const look = parseLook(v.look);
  if (name === null || !look) return null;
  return { id: v.id, name, look, pose: v.pose === undefined ? null : pose(v.pose) };
}

function chatLine(v: unknown): ChatLine | null {
  if (!isObj(v) || !isId(v.id) || !num(v.at)) return null;
  const name = str(v.name, NAME_MAX);
  const body = str(v.text, CHAT_MAX);
  return name === null || !body ? null : { id: v.id, name, text: body, at: v.at };
}

/** the valid items of an array (bad ones are skipped), or null if it is not an array */
function list<T>(v: unknown, item: (x: unknown) => T | null): T[] | null {
  if (!Array.isArray(v)) return null;
  const out: T[] = [];
  for (const x of v) {
    const ok = item(x);
    if (ok) out.push(ok);
  }
  return out;
}

const ERROR_CODES: readonly ErrorCode[] = ['full', 'bad_hello', 'rate', 'auth'];

/** One server frame, or null if it is not a message this client understands. */
export function parseServerMsg(data: unknown): ServerMsg | null {
  if (typeof data !== 'string') return null;
  let m: unknown;
  try {
    m = JSON.parse(data);
  } catch {
    return null;
  }
  if (!isObj(m)) return null;
  switch (m.t) {
    case 'welcome': {
      const players = list(m.players, player);
      const chat = list(m.chat, chatLine);
      const name = str(m.name, NAME_MAX);
      if (!isId(m.id) || !num(m.max) || !players || !chat || name === null) return null;
      return { t: 'welcome', id: m.id, name, max: m.max, players, chat };
    }
    case 'join': {
      const p = player(m.p);
      return p ? { t: 'join', p } : null;
    }
    case 'leave':
      return isId(m.id) ? { t: 'leave', id: m.id } : null;
    case 'profile': {
      const p = player(m);
      return p ? { t: 'profile', id: p.id, name: p.name, look: p.look } : null;
    }
    case 'snap': {
      if (!num(m.at) || !Array.isArray(m.p)) return null;
      const rows: SnapRow[] = [];
      for (const r of m.p) {
        if (Array.isArray(r) && r.length === 6 && isId(r[0]) && r.every(num) && r[5] >= 0) rows.push([r[0], r[1], r[2], r[3], r[4], r[5]]);
      }
      return { t: 'snap', at: m.at, rows };
    }
    case 'chat': {
      const line = chatLine(m);
      return line ? { t: 'chat', ...line } : null;
    }
    case 'error': {
      const code = ERROR_CODES.find((c) => c === m.code) ?? 'unknown';
      return { t: 'error', code };
    }
    default:
      return null;
  }
}
