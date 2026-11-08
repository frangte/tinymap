// Crab Bike trips as the game sees them (docs/design/08-ride-hailing.md): our wallet, being on duty, the trip we
// are on (booked or driving) and the offers a driver gets. The server decides everything; this keeps its word,
// puts the next step on the action button (get on, get off, drop off) and tells the quest tracker where to go.
import * as THREE from 'three';
import data from '../../server/content/ridehail.json';
import { PLACES, distance, type Place } from './places';
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

/** 16.000 xu */
export function xu(n: number): string {
  return `${n.toLocaleString('vi-VN')} xu`;
}

export function placeName(id: string): string {
  return PLACES.find((p) => p.id === id)?.name ?? id;
}

/** Where the quest tracker points instead of the quest while a trip goes on. */
export interface Guide {
  title: string;
  goal: string;
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

const ERRORS: Record<RideError, string> = {
  no_account: 'Đặt xe cần tài khoản: tải lại trang để thử lại.',
  busy: 'Bạn đang có một chuyến rồi.',
  unknown_place: 'Không tìm thấy nơi này.',
  no_bike: 'Tài xế phải đang ngồi trên xe của mình.',
  too_far: 'Lại gần xe hơn chút nữa nhé.',
  not_stopped: 'Đợi xe dừng hẳn đã.',
  not_there: 'Chưa tới nơi: chở khách vào gần hơn.',
  balance: 'Ví không đủ tiền.',
  gone: 'Cuốc này không còn nữa.',
  not_allowed: 'Chỉ tài xế Crab mới nhận cuốc được.',
  tip: 'Tiền tip không hợp lệ.',
  too_soon: 'Vừa đặt xong, đợi vài giây rồi đặt lại nhé.',
  riding: 'Xuống xe của bạn trước đã.',
  here: 'Bạn đang ở đó rồi.',
  unknown: 'Có lỗi, thử lại nhé.',
};

/** how a trip ended, for each side (the passenger's line, the driver's line) */
const ENDS: Record<string, [string, string]> = {
  alighted: ['Bạn đã xuống xe giữa đường.', 'Khách đã xuống xe giữa đường.'],
  cancelled_by_passenger: ['Đã huỷ chuyến, tiền đã hoàn lại.', 'Khách đã huỷ chuyến.'],
  cancelled_by_driver: ['Tài xế đã huỷ cuốc.', 'Bạn đã huỷ cuốc.'],
  no_driver: ['Chưa có tài xế nào nhận. Tiền đã hoàn lại, thử lại sau nhé.', ''],
  pickup_timeout: ['Tài xế không tới kịp. Tiền đã hoàn lại.', 'Quá giờ chưa đón khách: cuốc đã huỷ.'],
  passenger_left: ['', 'Khách đã rời phố.'],
  driver_left: ['Tài xế đã rời phố. Tiền đã hoàn lại.', ''],
  driver_got_off: ['Tài xế xuống xe giữa đường. Tiền đã hoàn lại.', 'Bạn xuống xe giữa đường: khách được hoàn tiền.'],
  passenger_got_off: ['Bạn đã rời xe giữa đường.', 'Khách đã rời xe giữa đường.'],
  hold_failed: ['Chưa đặt được xe, thử lại nhé.', ''],
  trip_timeout: ['Chuyến đi quá lâu nên đã huỷ, tiền đã hoàn lại.', 'Chở khách quá lâu: cuốc đã huỷ, khách được hoàn tiền.'],
};

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
        this.d.say('Cảm ơn bạn đã đánh giá tài xế!');
        break;
      case 'ride_error':
        if (RATING_ERRORS.has(m.code)) this.rating = false;
        // seated with no trip the server knows of (we lost track of it): back on our feet
        if (m.code === 'gone' && this.d.player.seated !== 0 && this.ride?.state !== 'on_trip') this.d.player.stand();
        this.d.say(ERRORS[m.code]);
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
    if (!this.d.net.sendRide({ t: 'ride_request', dest })) this.d.say('Đang mất kết nối, chưa đặt xe được.');
  }

  cancel(): void {
    this.d.net.sendRide({ t: 'ride_cancel' });
  }

  setDuty(on: boolean): void {
    if (!this.d.net.sendRide({ t: 'duty', on })) this.d.say('Đang mất kết nối.');
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
    // seated behind someone, the button always gets off (even if we lost track of the trip)
    if (this.d.player.seated !== 0) return 'Xuống xe';
    const r = this.ride;
    if (!r) return null;
    if (r.role === 'passenger') {
      if (r.state === 'on_trip' && this.d.player.seated !== 0) return 'Xuống xe';
      if (r.state === 'accepted' && this.driverNear()) return 'Lên xe Crab';
    } else if (r.state === 'on_trip' && this.d.player.riding && this.arrived()) {
      return 'Trả khách';
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
      this.d.say('Xuống xe giữa đường là huỷ cuốc, khách được hoàn tiền. Bấm lần nữa để xuống xe.');
      return true;
    }
    switch (this.label()) {
      case 'Xuống xe':
        this.d.net.sendRide({ t: 'ride_alight' });
        return true;
      case 'Lên xe Crab':
        this.d.net.sendRide({ t: 'ride_board' });
        return true;
      case 'Trả khách':
        if (this.d.player.speed > data.stopped_speed) this.d.say('Dừng xe hẳn rồi trả khách.');
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
    const dest = PLACES.find((p) => p.id === r.dest);
    const toDest = (): THREE.Vector3 | null => (dest ? this.to.set(dest.pts[0][0], 0, -dest.pts[0][1]) : null);
    const toPerson = (id: number, fallback: readonly [number, number]): THREE.Vector3 =>
      this.d.other(id)?.position ?? this.to.set(fallback[0], 0, fallback[1]);
    const name = dest?.name ?? r.dest;
    if (r.role === 'rider') {
      if (r.state === 'accepted') return { title: 'Crab Driver', goal: `Đón ${r.passenger.name}`, target: () => toPerson(r.passenger.id, r.pickup) };
      return { title: 'Crab Driver', goal: `Chở khách tới ${name}`, target: toDest };
    }
    switch (r.state) {
      case 'requested':
        return { title: 'Crab', goal: 'Đang tìm tài xế…', target: () => null };
      case 'accepted': {
        const driver = r.driver;
        return { title: 'Crab', goal: `${driver?.name ?? 'Tài xế'} đang tới đón bạn`, target: () => (driver ? toPerson(driver.id, r.pickup) : null) };
      }
      default:
        return { title: 'Crab', goal: `Đang tới ${name}`, target: toDest };
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
      this.d.notify('Tới nơi', r.end === 'completed' ? `${name}. Đã trả ${xu(r.fare)}.` : `Xuống xe giữa đường, đã trả ${xu(r.fare)}.`);
      if (r.end !== 'completed') this.d.say(ENDS.alighted[0]);
      this.d.openApp('crab');
      return;
    }
    this.ride = null;
    if (!passenger && r.end === 'completed') {
      this.d.notify('Hoàn thành cuốc xe', `Chở ${r.passenger.name} tới ${name}. Nhận ${xu(r.paid)}.`);
      return;
    }
    const line = ENDS[r.end]?.[passenger ? 0 : 1] ?? (passenger ? 'Chuyến đi đã kết thúc.' : 'Cuốc xe đã kết thúc.');
    if (line) this.d.say(r.paid > 0 && !passenger ? `${line} Nhận ${xu(r.paid)}.` : line);
  }

  private announce(r: Ride, before: Ride | null): void {
    if (r.role === 'rider') {
      if (r.state === 'accepted') this.d.say(`Đã nhận cuốc: tới đón ${r.passenger.name}.`);
      else if (r.state === 'on_trip') this.d.say(`${r.passenger.name} đã lên xe. Chở tới ${placeName(r.dest)}, tới nơi thì dừng xe và bấm Trả khách.`);
      return;
    }
    if (r.state === 'requested' && before?.state === 'accepted') this.d.say('Tài xế đã huỷ, đang tìm tài xế khác…');
    else if (r.state === 'requested') this.d.say(`Đã đặt xe tới ${placeName(r.dest)}, đang tìm tài xế…`);
    else if (r.state === 'accepted' && r.driver) this.d.notify('Đã có tài xế', `${r.driver.name} đang tới đón bạn. Xe tới thì bấm Lên xe Crab.`);
    else if (r.state === 'on_trip') this.d.say(`Lên xe rồi! Đang tới ${placeName(r.dest)}.`);
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
    const dest = PLACES.find((p) => p.id === this.ride?.dest);
    const p = this.d.player.position;
    return !!dest && distance(dest, p.x, -p.z) <= dest.r;
  }

  private changed(): void {
    for (const fn of this.listeners) fn();
  }
}
