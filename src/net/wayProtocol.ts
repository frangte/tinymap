// Asking the way on the wire (server/README.md, Asking the way; server/internal/directions): what the server says
// to a Traveler who asks and to the Citizens it calls, checked here before the game sees it, and what the game sends.
import { NAME_MAX } from '../game/profile';
import { idOf, isId, isObj, num, str, type Obj } from './check';

/** where our ask stands: asking (Citizens called), then answered, unanswered (nobody came) or cancelled */
export type WayState = 'asking' | 'answered' | 'unanswered' | 'cancelled';

export type WayError = 'no_account' | 'not_allowed' | 'busy' | 'unknown_place' | 'too_soon' | 'gone' | 'too_far' | 'already_helped';

export interface WayPerson {
  id: number;
  name: string;
}

/** Our own ask, as a Traveler. */
export interface WayAsk {
  id: string;
  state: WayState;
  /** the landmark asked for (server/content/places.json) */
  dest: string;
  /** when the ask stops waiting (ms since the epoch, server clock), 0 when not asking */
  until: number;
  helper: WayPerson | null;
  /** why it went unanswered: nobody (no Citizen online), no_answer (time ran out) */
  why: string;
}

/** A Traveler's ask, as a Citizen gets it. */
export interface WayCall {
  id: string;
  from: WayPerson;
  dest: string;
  until: number;
}

export type WayServerMsg =
  | ({ t: 'way' } & WayAsk)
  | ({ t: 'way_call' } & WayCall)
  | { t: 'way_gone'; id: string; why: string }
  | { t: 'way_done'; id: string; to: WayPerson; dest: string }
  | { t: 'way_error'; code: WayError };

export type WayClientMsg = { t: 'way_ask'; dest: string } | { t: 'way_cancel' } | { t: 'way_answer'; ask: string };

const STATES: readonly WayState[] = ['asking', 'answered', 'unanswered', 'cancelled'];
const ERRORS: readonly WayError[] = ['no_account', 'not_allowed', 'busy', 'unknown_place', 'too_soon', 'gone', 'too_far', 'already_helped'];
/** ask ids are ULIDs */
const ULID_RE = /^[0-9A-Z]{1,32}$/;
const WORD_RE = /^[a-z_]{1,32}$/;

const ulid = (v: unknown): string => (typeof v === 'string' && ULID_RE.test(v) ? v : '');
const word = (v: unknown): string => (typeof v === 'string' && WORD_RE.test(v) ? v : '');
const time = (v: unknown): number => (num(v) && v >= 0 ? v : 0);

function person(v: unknown): WayPerson | null {
  if (!isObj(v) || !isId(v.id)) return null;
  const name = str(v.name, NAME_MAX);
  return name === null ? null : { id: v.id, name };
}

/** An asking-the-way message (its `t` is one of WayServerMsg's), or null if it is not one or is malformed. */
export function parseWayMsg(m: Obj): WayServerMsg | null {
  switch (m.t) {
    case 'way': {
      const state = STATES.find((s) => s === m.state);
      const dest = idOf(m.dest);
      if (!state || !dest) return null;
      const helper = m.helper === undefined ? null : person(m.helper);
      return { t: 'way', id: ulid(m.id), state, dest, until: time(m.until), helper, why: word(m.why) };
    }
    case 'way_call': {
      const id = ulid(m.id);
      const from = person(m.from);
      const dest = idOf(m.dest);
      return id && from && dest ? { t: 'way_call', id, from, dest, until: time(m.until) } : null;
    }
    case 'way_gone': {
      const id = ulid(m.id);
      return id ? { t: 'way_gone', id, why: word(m.why) } : null;
    }
    case 'way_done': {
      const id = ulid(m.id);
      const to = person(m.to);
      const dest = idOf(m.dest);
      return id && to && dest ? { t: 'way_done', id, to, dest } : null;
    }
    case 'way_error': {
      const code = ERRORS.find((c) => c === m.code);
      return code ? { t: 'way_error', code } : null;
    }
    default:
      return null;
  }
}
