// Wire format between the game and the Go server; server/README.md is the spec. Everything that arrives is
// checked here before the game sees it: the server cleans what clients send, but the client trusts nothing either.
import { parseLook, type Look } from '../game/outfits';
import { NAME_MAX } from '../game/profile';

export const CHAT_MAX = 140;
/** how far a nearby chat line carries (m): world.DefaultConfig().NearbyRadius on the server */
export const NEARBY_RADIUS = 20;

/** [x, y, z, heading] in three.js metres / radians */
export type Pose = [number, number, number, number];

/** State flags (the `s` of a state and of every snapshot row): server/internal/protocol/msg.go */
export const FLAG_AIR = 1;
/** set by the server from its own record of who rides what */
export const FLAG_RIDE = 2;
export const EMOTE_SHIFT = 4;
export const EMOTE_MASK = 0xf << EMOTE_SHIFT;

/** the emote code in state flags, 0 for none */
export function emoteOf(s: number): number {
  return (s & EMOTE_MASK) >> EMOTE_SHIFT;
}

export interface PlayerInfo {
  id: number;
  name: string;
  /** archetype id, '' when the player has none (an older client) */
  arch: string;
  /** the title they carry (server/content/quests.json), '' for none */
  title: string;
  look: Look;
  /** an older server sent the pose here; a current one only sends positions in snapshots, to those nearby */
  pose: Pose | null;
}

/** A vehicle in the world. While `rider` is set it moves with them; `pose` is where it was last parked. */
export interface VehicleInfo {
  id: number;
  /** server/content/vehicles.json id */
  kind: string;
  owner: number;
  /** 0 while parked */
  rider: number;
  pose: Pose;
}

export type Channel = 'global' | 'nearby';

export interface ChatLine {
  id: number;
  name: string;
  /** the sender's archetype id ('' for none) */
  arch: string;
  /** the sender's title ('' for none) */
  title: string;
  /** an NPC said it (a call to passers-by, or a line of a public conversation); id is the NPC's then */
  npc: boolean;
  ch: Channel;
  text: string;
  /** server time, ms since the epoch */
  at: number;
}

/** A line in each language the game speaks; Vietnamese is always there. */
export interface Text {
  vi: string;
  en: string;
}

/** An NPC on the island: `npc` is its id in server/content/npcs.json (name, look and role come from there). */
export interface NpcInfo {
  id: number;
  npc: string;
  pose: Pose;
}

/** private: one player and the NPC; public: the NPC's lines are also heard nearby; group: shared, led by one */
export type TalkMode = 'private' | 'public' | 'group';

/** Where a conversation is: the NPC's line and the answers on offer (none: only goodbye is left). */
export interface Interaction {
  sid: number;
  /** the NPC's id */
  n: number;
  mode: TalkMode;
  /** counts the conversation's lines: an answer names the step of the line it answers */
  step: number;
  /** we pick the answers (always, outside a group) */
  lead: boolean;
  /** how many players share the conversation */
  size: number;
  say: Text;
  choices: Text[];
}

/** done: it reached its end; left: we closed it; far: we walked away; other: we started another one */
export type EndReason = 'done' | 'left' | 'far' | 'other' | 'unknown';

/** A started quest: done, or how far each objective got (by objective id). */
export interface QuestState {
  id: string;
  done: boolean;
  n: Record<string, number>;
}

export interface AchievementState {
  id: string;
  n: number;
  done: boolean;
}

export type NewsKind = 'quest' | 'achievement' | 'title' | 'stamp';

/** Something that just happened: a quest or achievement completed, a title or stamp earned. */
export interface News {
  kind: NewsKind;
  id: string;
}

/** A character's quests and achievements (their text is in server/content/quests.json), and what just happened. */
export interface Progress {
  quests: QuestState[];
  achievements: AchievementState[];
  /** titles and stamps earned; the title carried ('' for none) */
  titles: string[];
  stamps: string[];
  title: string;
  news: News[];
}

/** The answer to a command that carried a request id. code says why it failed (offline / timeout: no answer came). */
export interface Result {
  ok: boolean;
  code: string;
}

/** NPC lines and answers are capped like this (SayMax, ChoiceMax, ChoicesMax in server/content/cast.go) */
const SAY_MAX = 320;
const CHOICE_MAX = 90;
const CHOICES_MAX = 6;

/** archetype and app ids are short lower_snake_case words (server/content); anything else is dropped */
const ID_RE = /^[a-z0-9_]{1,32}$/;

/**
 * auth: the hello's account token is unknown to the server (forget it and get a new one); mount: getting on a
 * vehicle was refused; replaced: the same character joined from somewhere else (another tab), so this one stops
 */
export type ErrorCode = 'full' | 'bad_hello' | 'rate' | 'auth' | 'mount' | 'replaced' | 'unknown';

/** [id, x, y, z, heading, flags, age]: age = ms between the server receiving this pose and sending the snapshot */
export type SnapRow = [number, number, number, number, number, number, number];

export type ServerMsg =
  /**
   * arch: our archetype as the server keeps it ('' for none); apps: our phone apps in phone order (null: an older
   * server); pose: where the server placed us (null: an older server, which lets us stand where we like)
   */
  | { t: 'welcome'; id: number; name: string; arch: string; apps: string[] | null; max: number; pose: Pose | null; players: PlayerInfo[]; vehicles: VehicleInfo[]; npcs: NpcInfo[]; chat: ChatLine[] }
  | { t: 'join'; p: PlayerInfo }
  | { t: 'leave'; id: number }
  | { t: 'profile'; id: number; name: string; look: Look; title: string }
  /** out: players that left our view */
  | { t: 'snap'; at: number; rows: SnapRow[]; out: number[] }
  /** our last move was refused: back to pose, and echo k from now on */
  | { t: 'correct'; k: number; pose: Pose }
  | ({ t: 'vehicle' } & VehicleInfo)
  | { t: 'vehicle_gone'; id: number }
  | ({ t: 'chat' } & ChatLine)
  | ({ t: 'interaction' } & Interaction)
  | { t: 'interaction_end'; sid: number; reason: EndReason }
  | { t: 'result'; rid: number; ok: boolean; code: string }
  | ({ t: 'progress' } & Progress)
  | { t: 'error'; code: ErrorCode };

export type ClientMsg =
  /** token: the guest account (src/net/auth.ts); absent plays as a guest without one */
  /** arch: the archetype picked (the server keeps the one a character already has) */
  | { t: 'hello'; token?: string; arch?: string; name: string; look: Look }
  | { t: 'profile'; name: string; look: Look }
  /** s: state flags; k: the corrections applied so far */
  | { t: 'state'; x: number; y: number; z: number; h: number; s: number; k: number }
  /** back to the archetype's start: the one jump the server allows */
  | { t: 'respawn'; x: number; y: number; z: number; h: number; k: number }
  | { t: 'mount'; v: number }
  | { t: 'dismount' }
  /** n: the NPC to talk to; rid: the request id the result comes back with */
  | { t: 'interact'; n: number; rid: number }
  /** c: the index of the answer among the choices shown; step: that line's step */
  | { t: 'choose'; sid: number; c: number; step: number; rid: number }
  | { t: 'interact_leave'; sid: number }
  /** the title to carry, among those earned ('' for none) */
  | { t: 'title'; title: string }
  | { t: 'chat'; ch: Channel; text: string };

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

/** an archetype or app id, '' when absent or malformed */
function idOf(v: unknown): string {
  return typeof v === 'string' && ID_RE.test(v) ? v : '';
}

function vehicle(v: unknown): VehicleInfo | null {
  if (!isObj(v) || !isId(v.id) || !isId(v.owner) || !(v.rider === 0 || isId(v.rider))) return null;
  const p = pose(v.pose);
  const kind = idOf(v.kind);
  return p && kind ? { id: v.id, kind, owner: v.owner, rider: v.rider as number, pose: p } : null;
}

function player(v: unknown): PlayerInfo | null {
  if (!isObj(v) || !isId(v.id)) return null;
  const name = str(v.name, NAME_MAX);
  const look = parseLook(v.look);
  if (name === null || !look) return null;
  return { id: v.id, name, arch: idOf(v.arch), title: idOf(v.title), look, pose: v.pose === undefined ? null : pose(v.pose) };
}

function chatLine(v: unknown): ChatLine | null {
  if (!isObj(v) || !isId(v.id) || !num(v.at)) return null;
  const name = str(v.name, NAME_MAX);
  const body = str(v.text, CHAT_MAX);
  if (name === null || !body) return null;
  // older servers send no channel: everything was global then
  return { id: v.id, name, arch: idOf(v.arch), title: idOf(v.title), npc: v.npc === true, ch: v.ch === 'nearby' ? 'nearby' : 'global', text: body, at: v.at };
}

function text(v: unknown, max: number): Text | null {
  if (!isObj(v)) return null;
  const vi = str(v.vi, max);
  if (!vi) return null;
  return { vi, en: str(v.en, max) ?? '' };
}

function npcInfo(v: unknown): NpcInfo | null {
  if (!isObj(v) || !isId(v.id)) return null;
  const p = pose(v.pose);
  const key = idOf(v.npc);
  return p && key ? { id: v.id, npc: key, pose: p } : null;
}

const NEWS: readonly NewsKind[] = ['quest', 'achievement', 'title', 'stamp'];
/** quest ids are dotted ids (traveler.chao_sai_gon) */
const QUEST_RE = /^[a-z0-9_]+(\.[a-z0-9_]+)*$/;

function questId(v: unknown): string {
  return typeof v === 'string' && v.length <= 64 && QUEST_RE.test(v) ? v : '';
}

/** more than the game data could ever have: anything past it is dropped */
const QUESTS_MAX = 64;
const NEWS_MAX = 16;
const OBJECTIVES_MAX = 8;

function counts(v: unknown): Record<string, number> {
  // no prototype: an objective id can never find an inherited property
  const out: Record<string, number> = Object.create(null) as Record<string, number>;
  if (!isObj(v)) return out;
  for (const [k, n] of Object.entries(v).slice(0, OBJECTIVES_MAX)) {
    if ((k === '' || ID_RE.test(k)) && Number.isInteger(n) && (n as number) >= 0) out[k] = n as number;
  }
  return out;
}

/** the first of each id, at most max */
function distinct<T extends { id: string }>(items: T[], max: number): T[] {
  const seen = new Set<string>();
  return items.filter((x) => !seen.has(x.id) && seen.add(x.id) !== undefined).slice(0, max);
}

function progress(m: Obj): Progress | null {
  const quests = list(m.quests, (q) => (isObj(q) && questId(q.id) ? { id: questId(q.id), done: q.done === true, n: counts(q.n) } : null));
  const achievements = list(m.achievements, (a) =>
    isObj(a) && idOf(a.id) && Number.isInteger(a.n) && (a.n as number) >= 0 ? { id: idOf(a.id), n: a.n as number, done: a.done === true } : null,
  );
  const ids = (v: unknown): string[] => [...new Set(list(v, (x) => idOf(x) || null) ?? [])].slice(0, QUESTS_MAX);
  if (!quests || !achievements) return null;
  const news = list(m.news, (n) => {
    if (!isObj(n)) return null;
    const kind = NEWS.find((k) => k === n.kind);
    const id = kind === 'quest' ? questId(n.id) : idOf(n.id);
    return kind && id ? { kind, id } : null;
  }) ?? [];
  return {
    quests: distinct(quests, QUESTS_MAX),
    achievements: distinct(achievements, QUESTS_MAX),
    titles: ids(m.titles),
    stamps: ids(m.stamps),
    title: idOf(m.title),
    news: news.slice(0, NEWS_MAX),
  };
}

const MODES: readonly TalkMode[] = ['private', 'public', 'group'];
const REASONS: readonly EndReason[] = ['done', 'left', 'far', 'other'];

function interaction(m: Obj): Interaction | null {
  const mode = MODES.find((x) => x === m.mode);
  const say = text(m.say, SAY_MAX);
  if (!isId(m.sid) || !isId(m.n) || !mode || !say || !Number.isInteger(m.size) || !Number.isInteger(m.step) || !Array.isArray(m.choices)) return null;
  // answers are picked by index: dropping a bad one would shift the rest, so a bad one drops the whole line
  if (m.choices.length > CHOICES_MAX) return null;
  const choices: Text[] = [];
  for (const c of m.choices) {
    const t = text(c, CHOICE_MAX);
    if (!t) return null;
    choices.push(t);
  }
  return { sid: m.sid, n: m.n, mode, step: m.step as number, lead: m.lead === true, size: Math.max(1, m.size as number), say, choices };
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

const ERROR_CODES: readonly ErrorCode[] = ['full', 'bad_hello', 'rate', 'auth', 'mount', 'replaced'];

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
      // an older server sends no apps: null lets the game work them out from the archetype itself
      const apps = Array.isArray(m.apps) ? m.apps.map(idOf).filter((a) => a !== '') : null;
      const vehicles = list(m.vehicles, vehicle) ?? [];
      const npcs = list(m.npcs, npcInfo) ?? [];
      return { t: 'welcome', id: m.id, name, arch: idOf(m.arch), apps, max: m.max, pose: pose(m.pose), players, vehicles, npcs, chat };
    }
    case 'join': {
      const p = player(m.p);
      return p ? { t: 'join', p } : null;
    }
    case 'leave':
      return isId(m.id) ? { t: 'leave', id: m.id } : null;
    case 'profile': {
      const p = player(m);
      return p ? { t: 'profile', id: p.id, name: p.name, look: p.look, title: p.title } : null;
    }
    case 'progress': {
      const p = progress(m);
      return p ? { t: 'progress', ...p } : null;
    }
    case 'snap': {
      if (!num(m.at) || !Array.isArray(m.p)) return null;
      const rows: SnapRow[] = [];
      for (const r of m.p) {
        if (!Array.isArray(r) || !isId(r[0]) || !r.every(num)) continue;
        // an older server sends [id, x, y, z, h, age], without flags
        if (r.length === 7 && r[6] >= 0 && Number.isInteger(r[5])) rows.push([r[0], r[1], r[2], r[3], r[4], r[5], r[6]]);
        else if (r.length === 6 && r[5] >= 0) rows.push([r[0], r[1], r[2], r[3], r[4], 0, r[5]]);
      }
      const out = Array.isArray(m.out) ? m.out.filter(isId) : [];
      return { t: 'snap', at: m.at, rows, out };
    }
    case 'correct': {
      const p = pose(m.pose);
      return p && Number.isInteger(m.k) && (m.k as number) >= 0 ? { t: 'correct', k: m.k as number, pose: p } : null;
    }
    case 'vehicle': {
      const v = vehicle(m);
      return v ? { t: 'vehicle', ...v } : null;
    }
    case 'vehicle_gone':
      return isId(m.id) ? { t: 'vehicle_gone', id: m.id } : null;
    case 'chat': {
      const line = chatLine(m);
      return line ? { t: 'chat', ...line } : null;
    }
    case 'interaction': {
      const i = interaction(m);
      return i ? { t: 'interaction', ...i } : null;
    }
    case 'interaction_end':
      return isId(m.sid) ? { t: 'interaction_end', sid: m.sid, reason: REASONS.find((r) => r === m.reason) ?? 'unknown' } : null;
    case 'result':
      return isId(m.rid) ? { t: 'result', rid: m.rid, ok: m.ok === true, code: typeof m.code === 'string' ? m.code.slice(0, 16) : '' } : null;
    case 'error': {
      const code = ERROR_CODES.find((c) => c === m.code) ?? 'unknown';
      return { t: 'error', code };
    }
    default:
      return null;
  }
}
