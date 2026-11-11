// Crab Bike trips as the game sees them (docs/design/08-ride-hailing.md): our wallet, being on duty, the trip we
// are on (booked or driving) and the offers a driver gets. The server decides everything; this keeps its word,
// puts the next step on the action button (get on, get off, drop off) and tells the quest tracker where to go.
import * as THREE from 'three';
import data from '../../server/content/ridehail.json';
import { PLACES, distance, placeById, placeTitle, type Place } from './places';
import { lang } from './i18n';
import { L } from './i18n/rides';
import type { Player } from './player';
import type { NetClient } from '../net/client';
import type { Ride, RideError, RideOffer, RideServerMsg } from '../net/rideProtocol';

/** the tips a passenger may add, in xu */
export const TIPS: readonly number[] = data.tips;
/** the destinations the Crab app lists: the landmarks */
export const DESTINATIONS: readonly Place[] = PLACES.filter((p) => p.kind === 'landmark');
/** how long an offer lasts (ms) */
export const OFFER_MS = data.offer_seconds * 1000;
/** the part of a fare the platform keeps (%), and how far from the pickup a driver still gets the offer (m) */
export const CUT_PERCENT = data.cut_percent;
export const OFFER_RADIUS = data.offer_radius;
/** a driver mid-trip presses get-off twice within this to really get off (ms): it cancels the trip */
const CONFIRM_OFF_MS = 3000;
/** the trip states that end it */
const ENDED: ReadonlySet<string> = new Set(['completed', 'cancelled', 'expired', 'ended']);
/** errors that answer a rating (the rating may be sent again) */
const RATING_ERRORS: ReadonlySet<RideError> = new Set<RideError>(['tip', 'balance', 'gone']);
/** the button offers to get on a little past the server's reach, which has the last word (m) */
const BOARD_NEAR = data.board_reach + 0.5;

/** The fare from three.js (x, z) to a place, as the server will charge it. */
export function fareTo(place: Place, x: number, z: number): number {
  const f = data.fare;
  return Math.round((f.base + f.per_metre * Math.max(0, distance(place, x, -z))) / f.round) * f.round;
}

/** 16.000 xu (16,000 xu in English) */
export function xu(n: number): string {
  return `${n.toLocaleString(lang() === 'en' ? 'en-US' : 'vi-VN')} xu`;
}

/** A place's name in the player's language, by id (the id itself for places this build does not know). */
export function placeName(id: string): string {
  const p = placeById(id);
  return p ? placeTitle(p) : id;
}

/** A guide whose goal is written when it is read, so it follows a language switch. */
function guideTo(title: string, goal: () => string, target: () => THREE.Vector3 | null): Guide {
  return {
    title,
    get goal() {
      return goal();
    },
    target,
  };
}

/** the trip's next step on the action button */
type Step = 'alight' | 'board' | 'drop_off';

/** Where the quest tracker points instead of the quest while a trip goes on. */
export interface Guide {
  readonly title: string;
  /** a getter: read again after a language switch, it is in the new language */
  readonly goal: string;
  /** where to go now (three.js, on the ground), null for nowhere */
  target(): THREE.Vector3 | null;
}

export interface RidesDeps {
  net: NetClient;
  player: Player;
  /** where a remote player is drawn and how fast it goes, null when it is not in view */
  other(id: number): { position: THREE.Vector3; speed: number } | null;
  /** a line in the chat log */
  say(text: string): void;
  /** a notice on the phone */
  notify(title: string, text: string): void;
  /** open the phone on an app */
  openApp(id: string): void;
}

export class Rides {
  /** our balance in xu, null when unknown (a guest, offline) */
  balance: number | null = null;
  duty = false;
  /** the trip we are on, in either role; a completed one stays while it may be rated */
  ride: Ride | null = null;
  readonly offers = new Map<string, RideOffer>();
  /** a rating was sent and its answer has not come yet */
  rating = false;
  private readonly listeners = new Set<() => void>();
  private readonly to = new THREE.Vector3();
  /** when a driver mid-trip was warned that getting off cancels it (performance.now()) */
  private warnedAt = -1e9;

  constructor(private readonly d: RidesDeps) {}

  /** The server's time (ms since the epoch): offers and deadlines are on its clock. */
  now(): number {
    return this.d.net.serverNow();
  }

  onChange(fn: () => void): () => void {
    this.listeners.add(fn);
    return () => this.listeners.delete(fn);
  }

  /** A welcome or a dropped connection: the server will say again where things stand. */
  reset(): void {
    this.balance = null;
    this.duty = false;
    this.ride = null;
    this.offers.clear();
    this.rating = false;
    this.changed();
  }

  handle(m: RideServerMsg): void {
    switch (m.t) {
      case 'wallet':
        this.balance = m.balance;
        break;
      case 'duty':
        this.duty = m.on;
        if (!m.on) this.offers.clear();
        break;
      case 'ride': {
        const { t: _t, ...r } = m;
        this.trip(r);
        break;
      }
      case 'ride_offer': {
        const { t: _t, ...o } = m;
        if (this.duty) this.offers.set(o.offer, o);
        break;
      }
      case 'ride_offer_gone':
        this.offers.delete(m.offer);
        break;
      case 'ride_rated':
        this.rating = false;
        if (this.ride?.id === m.id) this.ride = null;
        this.d.say(L.rated);
        break;
      case 'ride_error':
        if (RATING_ERRORS.has(m.code)) this.rating = false;
        // seated with no trip the server knows of (we lost track of it): back on our feet
        if (m.code === 'gone' && this.d.player.seated !== 0 && this.ride?.state !== 'on_trip') this.d.player.stand();
        this.d.say(L.error(m.code));
        break;
    }
    this.changed();
  }

  /** A few times a second: offers and the rating window run out on their own. */
  update(): void {
    const now = this.now();
    let changed = false;
    for (const [id, o] of this.offers) {
      if (o.expires < now) {
        this.offers.delete(id);
        changed = true;
      }
    }
    const r = this.ride;
    if (r && r.state === 'completed' && r.deadline > 0 && r.deadline < now && !this.rating) {
      this.ride = null;
      changed = true;
    }
    if (changed) this.changed();
  }

  // ---- commands (the answers come back through handle)

  book(dest: string): void {
    if (!this.d.net.sendRide({ t: 'ride_request', dest })) this.d.say(L.offlineBook);
  }

  cancel(): void {
    this.d.net.sendRide({ t: 'ride_cancel' });
  }

  setDuty(on: boolean): void {
    if (!this.d.net.sendRide({ t: 'duty', on })) this.d.say(L.offline);
  }

  accept(offer: string): void {
    this.d.net.sendRide({ t: 'ride_accept', offer });
  }

  /** Let an offer go by (the others may still take it). */
  decline(offer: string): void {
    if (this.offers.delete(offer)) this.changed();
  }

  rate(stars: number, tip: number): void {
    if (this.rating || !this.d.net.sendRide({ t: 'ride_rate', stars, tip })) return;
    this.rating = true;
    this.changed();
  }

  /** Close a completed trip without rating it. */
  skipRating(): void {
    if (this.ride?.state !== 'completed') return;
    this.ride = null;
    this.changed();
  }

  /** The offer to show on the banner: the one running out first, while free to take one. */
  nextOffer(): RideOffer | null {
    if (!this.duty || this.ride) return null;
    let best: RideOffer | null = null;
    for (const o of this.offers.values()) if (!best || o.expires < best.expires) best = o;
    return best;
  }

  /** How far a remote player is from us (m), null when not in view. */
  howFar(id: number): number | null {
    const o = this.d.other(id);
    const p = this.d.player.position;
    return o ? Math.hypot(o.position.x - p.x, o.position.z - p.z) : null;
  }

  /** How far a three.js point (x, z) is from us (m). */
  howFarTo(x: number, z: number): number {
    const p = this.d.player.position;
    return Math.hypot(x - p.x, z - p.z);
  }

  // ---- the action button

  /** The trip's next step for the action button (E), null for none. */
  label(): string | null {
    switch (this.step()) {
      case 'alight':
        return L.stepAlight;
      case 'board':
        return L.stepBoard;
      case 'drop_off':
        return L.stepDropOff;
      default:
        return null;
    }
  }

  private step(): Step | null {
    // seated behind someone, the button always gets off (even if we lost track of the trip)
    if (this.d.player.seated !== 0) return 'alight';
    const r = this.ride;
    if (!r) return null;
    if (r.role === 'passenger') {
      if (r.state === 'accepted' && this.driverNear()) return 'board';
    } else if (r.state === 'on_trip' && this.d.player.riding && this.arrived()) {
      return 'drop_off';
    }
    return null;
  }

  /** Take that step; false when there is none (the button does something else). */
  use(): boolean {
    const r = this.ride;
    if (r?.role === 'rider' && r.state === 'on_trip' && this.d.player.riding && !this.arrived()) {
      // getting off the bike mid-trip cancels it: the second press within a few seconds does it
      const now = performance.now();
      if (now - this.warnedAt < CONFIRM_OFF_MS) return false;
      this.warnedAt = now;
      this.d.say(L.confirmGetOff);
      return true;
    }
    switch (this.step()) {
      case 'alight':
        this.d.net.sendRide({ t: 'ride_alight' });
        return true;
      case 'board':
        this.d.net.sendRide({ t: 'ride_board' });
        return true;
      case 'drop_off':
        if (this.d.player.speed > data.stopped_speed) this.d.say(L.stopFirst);
        else this.d.net.sendRide({ t: 'ride_complete' });
        return true;
      default:
        return false;
    }
  }

  // ---- guidance

  /** What the tracker shows while a trip goes on, null for the quest. */
  guide(): Guide | null {
    const r = this.ride;
    if (!r || r.state === 'completed') return null;
    const dest = placeById(r.dest);
    const toDest = (): THREE.Vector3 | null => (dest ? this.to.set(dest.pts[0][0], 0, -dest.pts[0][1]) : null);
    const toPerson = (id: number, fallback: readonly [number, number]): THREE.Vector3 =>
      this.d.other(id)?.position ?? this.to.set(fallback[0], 0, fallback[1]);
    const name = (): string => (dest ? placeTitle(dest) : r.dest);
    if (r.role === 'rider') {
      if (r.state === 'accepted') return guideTo('Crab Driver', () => L.goalPickUp(r.passenger.name), () => toPerson(r.passenger.id, r.pickup));
      return guideTo('Crab Driver', () => L.goalCarryTo(name()), toDest);
    }
    switch (r.state) {
      case 'requested':
        return guideTo('Crab', () => L.goalFinding, () => null);
      case 'accepted': {
        const driver = r.driver;
        return guideTo('Crab', () => L.goalDriverComing(driver?.name ?? null), () => (driver ? toPerson(driver.id, r.pickup) : null));
      }
      default:
        return guideTo('Crab', () => L.goalGoingTo(name()), toDest);
    }
  }

  // ----

  private trip(r: Ride): void {
    const before = this.ride;
    const passenger = r.role === 'passenger';
    const ended = ENDED.has(r.state);
    // late word about another trip than the one going on changes nothing; nor does the same news twice
    if (before && before.id !== r.id && !ENDED.has(before.state)) return;
    if (before?.id === r.id && before.state === r.state && before.end === r.end && ended) return;
    // a driver with a trip takes no other offer (the server drops them too)
    if (!passenger) this.offers.clear();
    if (!ended) {
      this.ride = r;
      if (before?.id !== r.id || before.state !== r.state) this.announce(r, before);
      return;
    }
    const name = placeName(r.dest);
    if (passenger && r.state === 'completed') {
      // the trip stays for the rating window
      this.ride = r;
      this.rating = false;
      this.d.notify(L.arrivedTitle, r.end === 'completed' ? L.arrivedPaid(name, xu(r.fare)) : L.alightedPaid(xu(r.fare)));
      if (r.end !== 'completed') this.d.say(L.end('alighted', true));
      this.d.openApp('crab');
      return;
    }
    this.ride = null;
    if (!passenger && r.end === 'completed') {
      this.d.notify(L.tripDoneTitle, L.tripDone(r.passenger.name, name, xu(r.paid)));
      return;
    }
    const line = L.end(r.end, passenger);
    if (line) this.d.say(r.paid > 0 && !passenger ? L.earned(line, xu(r.paid)) : line);
  }

  private announce(r: Ride, before: Ride | null): void {
    if (r.role === 'rider') {
      if (r.state === 'accepted') this.d.say(L.acceptedRider(r.passenger.name));
      else if (r.state === 'on_trip') this.d.say(L.boardedRider(r.passenger.name, placeName(r.dest)));
      return;
    }
    if (r.state === 'requested' && before?.state === 'accepted') this.d.say(L.driverCancelled);
    else if (r.state === 'requested') this.d.say(L.requested(placeName(r.dest)));
    else if (r.state === 'accepted' && r.driver) this.d.notify(L.driverFoundTitle, L.driverFound(r.driver.name));
    else if (r.state === 'on_trip') this.d.say(L.onTrip(placeName(r.dest)));
  }

  /** The driver's bike is within reach of us, stopped. */
  private driverNear(): boolean {
    const id = this.ride?.driver?.id;
    const o = id ? this.d.other(id) : null;
    if (!o) return false;
    const p = this.d.player.position;
    return Math.hypot(o.position.x - p.x, o.position.z - p.z) <= BOARD_NEAR;
  }

  /** Our bike is within the destination's reach. */
  private arrived(): boolean {
    const dest = this.ride ? placeById(this.ride.dest) : undefined;
    const p = this.d.player.position;
    return !!dest && distance(dest, p.x, -p.z) <= dest.r;
  }

  private changed(): void {
    for (const fn of this.listeners) fn();
  }
}
