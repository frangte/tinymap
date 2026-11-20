// Wire format between the game and the Go server; server/README.md is the spec. Everything that arrives is
// checked here before the game sees it: the server cleans what clients send, but the client trusts nothing either.
import { parseLook, type Look } from '../game/outfits';
import { NAME_MAX } from '../game/profile';
import { ID_RE, idOf, isId, isObj, list, num, str, type Obj } from './check';
import { parseRideMsg, type RideClientMsg, type RideServerMsg } from './rideProtocol';
import { parseWayMsg, type WayClientMsg, type WayServerMsg } from './wayProtocol';

export const CHAT_MAX = 140;
/** how far a nearby chat line carries (m): world.DefaultConfig().NearbyRadius on the server */
export const NEARBY_RADIUS = 20;

/** [x, y, z, heading] in three.js metres / radians */
export type Pose = [number, number, number, number];

/** State flags (the `s` of a state and of every snapshot row): server/internal/protocol/msg.go */
export const FLAG_AIR = 1;
/** set by the server from its own record of who rides what */
export const FLAG_RIDE = 2;
/** a passenger in seat 1 behind a rider, set by the server like FLAG_RIDE */
export const FLAG_SEAT1 = 4;
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
  /** who sits in seat 1 behind the rider (a Crab Bike passenger), 0 for nobody */
  passenger: number;
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
  /** the English of an NPC's line ('' for none: players' lines are not translated, older servers send none) */
  en: string;
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
  /** the id of the shop this line opens (items.json), '' for none: the sheet then offers to look at its wares */
  shop: string;
  /** the id of the fee this line asks for (items.json), '': the sheet offers a button that pays it with `pay` */
  fee: string;
  /** the contact's id when the conversation is a phone call ('' face to face); `n` is then a voice we do not see */
  call: string;
}

/**
 * The practical of the A1 licence as the server runs it (docs/design/13-driver.md): lent (the school's bike stands
 * beside us), run (the clock is going: ride to `next`, a point in three.js x and z with a radius, within `left` seconds),
 * pass, or fail (reason: time, off, idle).
 */
export interface DriveTest {
  state: 'lent' | 'run' | 'pass' | 'fail';
  index: number;
  total: number;
  left: number;
  next: [number, number] | null;
  radius: number;
  reason: string;
}

/** A text message (docs/design/12-phone.md): `id` says which one it is, `from` is a contact id (contacts.json). */
export interface PhoneText {
  id: string;
  from: string;
  say: Text;
  /** ms since the epoch */
  at: number;
  read: boolean;
}

/** done: it reached its end; left: we closed it; far: we walked away; other: we started another one */
export type EndReason = 'done' | 'left' | 'far' | 'other' | 'unknown';

/** A started quest: done, or how far each objective got (by objective id). */
export interface QuestState {
  id: string;
  done: boolean;
  n: Record<string, number>;
  /**
   * An objective that is taken and then timed (an odd job): the seconds left of a run that is on when this arrived
   * (-1: no time limit); missing while it is not taken.
   */
  left?: Record<string, number>;
  /** the outcome a done quest ended with (the quest's `outcomes` in quests.json); missing for none */
  outcome?: string;
  /** a running quest whose objectives are done: its live outcomes, the first to complete ends the quest */
  outcomes?: OutcomeState[];
}

/** A live outcome of a running quest: how far its objective got. */
export interface OutcomeState {
  id: string;
  n: number;
}

export interface AchievementState {
  id: string;
  n: number;
  done: boolean;
}

export type NewsKind = 'quest' | 'achievement' | 'collection' | 'title' | 'stamp';

/** Something that just happened: a quest or achievement completed, a collection entry found, a title or stamp earned. */
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
  /** days in a row with a daily quest done (0 once a day went by without one) */
  streak: number;
  /** ids of the Sổ sưu tầm entries found (server/content/collection.json) */
  collection: string[];
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

/**
 * auth: the hello's account token is unknown to the server (forget it and get a new one); mount: getting on a
 * vehicle was refused; replaced: the same character joined from somewhere else (another tab), so this one stops;
 * arch_busy, arch_wait: no archetype change now (on a trip, asking the way, talking; or one a moment ago)
 */
export type ErrorCode = 'full' | 'bad_hello' | 'rate' | 'auth' | 'mount' | 'replaced' | 'slow' | 'repeat' | 'new_player' | 'muted' | 'banned' | 'kicked' | 'arch_busy' | 'arch_wait' | 'unknown';

/** Why the server kept our archetype as it was. */
export type ArchRefusal = 'arch_busy' | 'arch_wait';

/** [id, x, y, z, heading, flags, age]: age = ms between the server receiving this pose and sending the snapshot */
export type SnapRow = [number, number, number, number, number, number, number];

export type ServerMsg =
  /**
   * arch: our archetype as the server keeps it ('' for none); apps: our phone apps in phone order (null: an older
   * server); pose: where the server placed us (null: an older server, which lets us stand where we like)
   */
  /**
   * wallet: our balance in xu, null for a guest without an account (or an older server); items: the ids of every
   * item we own, null for a guest or an older server
   */
  | { t: 'welcome'; id: number; name: string; arch: string; apps: string[] | null; max: number; pose: Pose | null; players: PlayerInfo[]; vehicles: VehicleInfo[]; npcs: NpcInfo[]; chat: ChatLine[]; wallet: number | null; items: string[] | null }
  | { t: 'join'; p: PlayerInfo }
  | { t: 'leave'; id: number }
  /** arch: '' from an older server, which does not say (archetypes did not change then) */
  | { t: 'profile'; id: number; name: string; arch: string; look: Look; title: string }
  /** our archetype changed: the one we have now, our phone apps and what we wear now */
  | { t: 'arch'; arch: string; apps: string[]; look: Look }
  /** out: players that left our view */
  | { t: 'snap'; at: number; rows: SnapRow[]; out: number[] }
  /** our last move was refused: back to pose, and echo k from now on */
  | { t: 'correct'; k: number; pose: Pose }
  | ({ t: 'vehicle' } & VehicleInfo)
  | { t: 'vehicle_gone'; id: number }
  /** an NPC came into the world after the welcome (an NPC passenger waiting for a ride), and its leaving */
  | { t: 'npc_join'; npc: NpcInfo }
  | { t: 'npc_leave'; id: number }
  | ({ t: 'chat' } & ChatLine)
  | ({ t: 'interaction' } & Interaction)
  | { t: 'interaction_end'; sid: number; reason: EndReason }
  | { t: 'result'; rid: number; ok: boolean; code: string }
  /** after a purchase (before its result): every item we own and the phone apps we have now */
  | { t: 'items'; items: string[]; apps: string[] }
  /** a price we bargained down in a shop: the sheet shows `percent` off its prices until it ends (0) */
  | { t: 'deal'; shop: string; percent: number }
  | ({ t: 'drive_test' } & DriveTest)
  /** every number we have (contact ids, in the order we got them) and our texts, oldest first; texts are empty without a phone */
  | { t: 'phone'; contacts: string[]; texts: PhoneText[] }
  /** a number we just got */
  | { t: 'contact'; contact: string }
  /** a text that just arrived */
  | { t: 'text'; text: PhoneText }
  | ({ t: 'progress' } & Progress)
  | { t: 'error'; code: ErrorCode }
  /** a moderation line for our chat log: a command's answer, a mute, a report received */
  | { t: 'mod_notice'; text: Text }
  /** our report of player n was stored */
  | { t: 'reported'; n: number }
  | RideServerMsg
  | WayServerMsg;

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
  /** buy an item (items.json) from the shop whose NPC we stand next to; the answer is a result (rid) */
  | { t: 'buy'; rid: number; item: string }
  /** pay a fee (items.json) to the NPC we stand next to; the answer is a result (rid): far, target, owned, locked, balance, busy */
  | { t: 'pay'; rid: number; fee: string }
  /** call a contact (contacts.json) wherever we are; the answer is a result (rid): far from nothing, but phone, target, mute, busy */
  | { t: 'call'; rid: number; contact: string }
  /** we opened the messages: all our texts are read */
  | { t: 'texts_read' }
  /** the title to carry, among those earned ('' for none) */
  | { t: 'title'; title: string }
  /** change archetype; the answer is an arch, or an error arch_busy / arch_wait */
  | { t: 'arch'; arch: string }
  | { t: 'chat'; ch: Channel; text: string }
  /** report player n to the moderators */
  | { t: 'report'; n: number }
  | RideClientMsg
  | WayClientMsg;

function pose(v: unknown): Pose | null {
  return Array.isArray(v) && v.length === 4 && v.every(num) ? [v[0], v[1], v[2], v[3]] : null;
}

function vehicle(v: unknown): VehicleInfo | null {
  if (!isObj(v) || !isId(v.id) || !isId(v.owner) || !(v.rider === 0 || isId(v.rider))) return null;
  const p = pose(v.pose);
  const kind = idOf(v.kind);
  // nobody behind the rider is left out of the message
  const passenger = isId(v.passenger) ? v.passenger : 0;
  return p && kind ? { id: v.id, kind, owner: v.owner, rider: v.rider as number, passenger, pose: p } : null;
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
  const npc = v.npc === true;
  const en = npc ? (str(v.en, CHAT_MAX) ?? '') : '';
  return { id: v.id, name, arch: idOf(v.arch), title: idOf(v.title), npc, ch: v.ch === 'nearby' ? 'nearby' : 'global', text: body, en, at: v.at };
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

const NEWS: readonly NewsKind[] = ['quest', 'achievement', 'collection', 'title', 'stamp'];
/** quest ids are dotted ids (traveler.chao_sai_gon) */
const QUEST_RE = /^[a-z0-9_]+(\.[a-z0-9_]+)*$/;

function questId(v: unknown): string {
  return typeof v === 'string' && v.length <= 64 && QUEST_RE.test(v) ? v : '';
}

/** more than the game data could ever have: anything past it is dropped */
const QUESTS_MAX = 64;
const NEWS_MAX = 16;
/** a streak longer than this is shown as this (years of dailies) */
const STREAK_MAX = 9999;
const OBJECTIVES_MAX = 8;

/** the seconds left of each running errand objective: -1 for no time limit, otherwise 0 or more; none when nothing runs */
function lefts(v: unknown): Record<string, number> | undefined {
  if (!isObj(v)) return undefined;
  const out: Record<string, number> = Object.create(null) as Record<string, number>;
  for (const [k, n] of Object.entries(v).slice(0, OBJECTIVES_MAX)) {
    if ((k === '' || ID_RE.test(k)) && Number.isInteger(n) && (n as number) >= -1) out[k] = n as number;
  }
  return Object.keys(out).length > 0 ? out : undefined;
}

const OUTCOMES_MAX = 6;

/** the live outcomes of a quest, none when it has none */
function outcomeStates(v: unknown): OutcomeState[] | undefined {
  const out = (list(v, (o) => (isObj(o) && ID_RE.test(String(o.id)) && Number.isInteger(o.n) && (o.n as number) >= 0 ? { id: String(o.id), n: o.n as number } : null)) ?? []).slice(0, OUTCOMES_MAX);
  return out.length > 0 ? out : undefined;
}

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
  const quests = list(m.quests, (q) =>
    isObj(q) && questId(q.id)
      ? { id: questId(q.id), done: q.done === true, n: counts(q.n), left: lefts(q.left), outcome: idOf(q.outcome) || undefined, outcomes: outcomeStates(q.outcomes) }
      : null,
  );
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
    collection: ids(m.collection),
    title: idOf(m.title),
    streak: Number.isInteger(m.streak) && (m.streak as number) >= 0 ? Math.min(m.streak as number, STREAK_MAX) : 0,
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
  return { sid: m.sid, n: m.n, mode, step: m.step as number, lead: m.lead === true, size: Math.max(1, m.size as number), say, choices, shop: idOf(m.shop), fee: idOf(m.fee), call: idOf(m.call) };
}

/** a text's id is the quest, its cycle and the moment that sent it (citizen.ca_phe_sang//start/0) */
const TEXT_ID_MAX = 96;
const TEXT_MAX = 200;
/** more numbers and texts than the game data could ever have: anything past it is dropped */
const CONTACTS_MAX = 64;
const TEXTS_MAX = 256;

function phoneText(v: unknown): PhoneText | null {
  if (!isObj(v) || typeof v.id !== 'string' || v.id === '' || v.id.length > TEXT_ID_MAX || !num(v.at)) return null;
  const from = idOf(v.from);
  const say = text(v.say, TEXT_MAX);
  return from && say ? { id: v.id, from, say, at: v.at, read: v.read === true } : null;
}

const ERROR_CODES: readonly ErrorCode[] = ['full', 'bad_hello', 'rate', 'auth', 'mount', 'replaced', 'slow', 'repeat', 'new_player', 'muted', 'banned', 'kicked', 'arch_busy', 'arch_wait'];
/** a moderation notice is a line or two for the chat log */
const NOTICE_MAX = 400;

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
      const wallet = Number.isInteger(m.wallet) && (m.wallet as number) >= 0 ? (m.wallet as number) : null;
      const items = Array.isArray(m.items) ? m.items.map(idOf).filter((i) => i !== '') : null;
      return { t: 'welcome', id: m.id, name, arch: idOf(m.arch), apps, max: m.max, pose: pose(m.pose), players, vehicles, npcs, chat, wallet, items };
    }
    case 'join': {
      const p = player(m.p);
      return p ? { t: 'join', p } : null;
    }
    case 'leave':
      return isId(m.id) ? { t: 'leave', id: m.id } : null;
    case 'profile': {
      const p = player(m);
      return p ? { t: 'profile', id: p.id, name: p.name, arch: p.arch, look: p.look, title: p.title } : null;
    }
    case 'arch': {
      const arch = idOf(m.arch);
      const look = parseLook(m.look);
      const apps = Array.isArray(m.apps) ? m.apps.map(idOf).filter((a) => a !== '') : null;
      return arch && look && apps ? { t: 'arch', arch, apps, look } : null;
    }
    case 'progress': {
      const p = progress(m);
      return p ? { t: 'progress', ...p } : null;
    }
    case 'mod_notice': {
      // a notice for players carries its English too; replies to moderators are Vietnamese only
      const vi = str(m.text, NOTICE_MAX);
      return vi ? { t: 'mod_notice', text: { vi, en: str(m.en, NOTICE_MAX) ?? '' } } : null;
    }
    case 'reported':
      return isId(m.n) ? { t: 'reported', n: m.n } : null;
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
    case 'npc_join': {
      // the fields of an NPC of the welcome sit beside the type
      const n = npcInfo(m);
      return n ? { t: 'npc_join', npc: n } : null;
    }
    case 'npc_leave':
      return isId(m.id) ? { t: 'npc_leave', id: m.id } : null;
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
    case 'items': {
      if (!Array.isArray(m.items) || !Array.isArray(m.apps)) return null;
      return { t: 'items', items: m.items.map(idOf).filter((i) => i !== ''), apps: m.apps.map(idOf).filter((a) => a !== '') };
    }
    case 'deal':
      return idOf(m.shop) && Number.isInteger(m.percent) ? { t: 'deal', shop: idOf(m.shop), percent: Math.max(0, Math.min(100, m.percent as number)) } : null;
    case 'drive_test': {
      const state = (['lent', 'run', 'pass', 'fail'] as const).find((s) => s === m.state);
      if (!state) return null;
      const next = Array.isArray(m.next) && m.next.length === 2 && m.next.every(num) ? ([m.next[0], m.next[1]] as [number, number]) : null;
      const n = (v: unknown): number => (num(v) ? v : 0);
      return { t: 'drive_test', state, index: n(m.index), total: n(m.total), left: n(m.left), next, radius: n(m.radius), reason: idOf(m.reason) };
    }
    case 'phone': {
      const contacts = list(m.contacts, (c) => idOf(c) || null);
      const texts = list(m.texts, phoneText);
      return contacts && texts ? { t: 'phone', contacts: [...new Set(contacts)].slice(0, CONTACTS_MAX), texts: texts.slice(-TEXTS_MAX) } : null;
    }
    case 'contact':
      return idOf(m.contact) ? { t: 'contact', contact: idOf(m.contact) } : null;
    case 'text': {
      const t = phoneText(m.text);
      return t ? { t: 'text', text: t } : null;
    }
    case 'result':
      return isId(m.rid) ? { t: 'result', rid: m.rid, ok: m.ok === true, code: typeof m.code === 'string' ? m.code.slice(0, 16) : '' } : null;
    case 'error': {
      const code = ERROR_CODES.find((c) => c === m.code) ?? 'unknown';
      return { t: 'error', code };
    }
    default:
      return parseRideMsg(m) ?? parseWayMsg(m);
  }
}
