// Crab Bike trips on the wire (server/README.md; docs/design/08-ride-hailing.md): what the server says about
// wallets, duty, trips and offers, checked here before the game sees it, and what the game sends.
import { NAME_MAX } from '../game/profile';
import { idOf, isId, isObj, num, str, type Obj } from './check';

/** while a trip goes on: requested (looking for a driver), accepted (one is coming), on_trip; then how it ended */
export type RideState = 'requested' | 'accepted' | 'on_trip' | 'completed' | 'cancelled' | 'expired' | 'ended';
/** passenger: we booked it; rider: we drive it */
export type RideRole = 'passenger' | 'rider';

export interface RidePerson {
  id: number;
  name: string;
  /** a driver's average stars (0 before any rating) and completed trips */
  stars: number;
  trips: number;
}

export interface Ride {
  id: string;
  state: RideState;
  role: RideRole;
  /** the destination's place id (server/content/places.json) */
  dest: string;
  fare: number;
  passenger: RidePerson;
  /** null until a driver accepts */
  driver: RidePerson | null;
  /** where the passenger booked from, three.js (x, z) */
  pickup: [number, number];
  /** when the state times out (or the rating window closes), ms since the epoch; 0 for none */
  deadline: number;
  /** how it ended ('' while it goes on): completed, alighted, cancelled_by_passenger, no_driver… */
  end: string;
  /** what the driver got */
  paid: number;
}

export interface RideOffer {
  offer: string;
  dest: string;
  fare: number;
  passenger: RidePerson;
  pickup: [number, number];
  /** ms since the epoch */
  expires: number;
}

export type RideError =
  | 'no_account' | 'busy' | 'unknown_place' | 'no_bike' | 'too_far' | 'not_stopped' | 'not_there' | 'balance' | 'gone'
  | 'not_allowed' | 'tip' | 'too_soon' | 'riding' | 'here' | 'unknown';

export type RideServerMsg =
  /** our balance in xu, after every move of money */
  | { t: 'wallet'; balance: number }
  | { t: 'duty'; on: boolean }
  | ({ t: 'ride' } & Ride)
  | ({ t: 'ride_offer' } & RideOffer)
  /** why: taken, cancelled, expired, off_duty */
  | { t: 'ride_offer_gone'; offer: string; why: string }
  | { t: 'ride_rated'; id: string }
  | { t: 'ride_error'; code: RideError };

export type RideClientMsg =
  | { t: 'duty'; on: boolean }
  | { t: 'ride_request'; dest: string }
  | { t: 'ride_cancel' }
  | { t: 'ride_accept'; offer: string }
  | { t: 'ride_board' }
  | { t: 'ride_alight' }
  | { t: 'ride_complete' }
  /** stars 1–5; tip one of the tips in server/content/ridehail.json */
  | { t: 'ride_rate'; stars: number; tip: number };

const STATES: readonly RideState[] = ['requested', 'accepted', 'on_trip', 'completed', 'cancelled', 'expired', 'ended'];
const ERRORS: readonly RideError[] = [
  'no_account', 'busy', 'unknown_place', 'no_bike', 'too_far', 'not_stopped', 'not_there', 'balance', 'gone', 'not_allowed', 'tip', 'too_soon', 'riding', 'here',
];
/** trip and offer ids are ULIDs */
const ULID_RE = /^[0-9A-Z]{1,32}$/;
/** end reasons and why an offer went: lower_snake words */
const WORD_RE = /^[a-z_]{1,32}$/;

const ulid = (v: unknown): string => (typeof v === 'string' && ULID_RE.test(v) ? v : '');
const word = (v: unknown): string => (typeof v === 'string' && WORD_RE.test(v) ? v : '');
/** xu: a whole number, never negative */
const xu = (v: unknown): number | null => (Number.isInteger(v) && (v as number) >= 0 ? (v as number) : null);
const orZero = (v: unknown): number => (num(v) && v >= 0 ? v : 0);

function person(v: unknown): RidePerson | null {
  if (!isObj(v) || !isId(v.id)) return null;
  const name = str(v.name, NAME_MAX);
  if (name === null) return null;
  return { id: v.id, name, stars: Math.min(5, orZero(v.stars)), trips: Number.isInteger(v.trips) ? Math.max(0, v.trips as number) : 0 };
}

function point(v: unknown): [number, number] | null {
  return Array.isArray(v) && v.length === 2 && num(v[0]) && num(v[1]) ? [v[0], v[1]] : null;
}

function ride(m: Obj): Ride | null {
  const id = ulid(m.id);
  const state = STATES.find((s) => s === m.state);
  const role: RideRole | null = m.role === 'passenger' || m.role === 'rider' ? m.role : null;
  const dest = idOf(m.dest);
  const fare = xu(m.fare);
  const passenger = person(m.passenger);
  const pickup = point(m.pickup);
  if (!id || !state || !role || !dest || fare === null || !passenger || !pickup) return null;
  const driver = m.driver === undefined ? null : person(m.driver);
  return { id, state, role, dest, fare, passenger, driver, pickup, deadline: orZero(m.deadline), end: word(m.end), paid: xu(m.paid) ?? 0 };
}

function offer(m: Obj): RideOffer | null {
  const id = ulid(m.offer);
  const dest = idOf(m.dest);
  const fare = xu(m.fare);
  const passenger = person(m.passenger);
  const pickup = point(m.pickup);
  if (!id || !dest || fare === null || !passenger || !pickup || !num(m.expires)) return null;
  return { offer: id, dest, fare, passenger, pickup, expires: m.expires };
}

/** A ride message (its `t` is one of RideServerMsg's), or null if it is not one or is malformed. */
export function parseRideMsg(m: Obj): RideServerMsg | null {
  switch (m.t) {
    case 'wallet': {
      const balance = xu(m.balance);
      return balance === null ? null : { t: 'wallet', balance };
    }
    case 'duty':
      return { t: 'duty', on: m.on === true };
    case 'ride': {
      const r = ride(m);
      return r ? { t: 'ride', ...r } : null;
    }
    case 'ride_offer': {
      const o = offer(m);
      return o ? { t: 'ride_offer', ...o } : null;
    }
    case 'ride_offer_gone': {
      const id = ulid(m.offer);
      return id ? { t: 'ride_offer_gone', offer: id, why: word(m.why) } : null;
    }
    case 'ride_rated': {
      const id = ulid(m.id);
      return id ? { t: 'ride_rated', id } : null;
    }
    case 'ride_error':
      return { t: 'ride_error', code: ERRORS.find((c) => c === m.code) ?? 'unknown' };
    default:
      return null;
  }
}
