// Player movement: camera-relative walking / running with acceleration and smooth turning, a capsule body that
// collides with the map's own shapes (collider.ts): it slides along walls at any angle, steps up kerbs and stairs,
// follows the floor down slopes and falls off what is higher than a step. Also jumping, emotes, riding a motorbike,
// and going back to where the server says when it refuses a move.
//
// Water deeper than collider.ts WADE and the edge of the map are the only things that stop a body without a shape
// of their own: a step that would end there goes along the shore instead. The server checks speeds only
// (server/content/movement.json and vehicles.json), so the game keeps to them.
import * as THREE from 'three';
import type { MoveIntent } from './input';
import { CONTACT_CEILING, CONTACT_FLOOR, FALL_LIMIT, type Body, type CityCollider } from './collider';
import { clamp, damp, smoothstep, wrapAngle } from './coords';
import { idlePose, Pedestrian, type AvatarPose } from './character';
import { emoteByCode, MOVEMENT } from './motion';
import { EMOTE_SHIFT, FLAG_AIR, FLAG_RIDE } from '../net/protocol';

export interface PlayerTuning {
  walkSpeed: number;
  runSpeed: number;
  /** how fast velocity reaches the target (1/s) */
  accel: number;
  /** how fast the body turns toward the move direction (1/s) */
  turnRate: number;
  /** the body's capsule: radius and full height (m) */
  radius: number;
  height: number;
  /** highest step a stride climbs (m): kerbs, stairs, a bench yes; walls no (jump onto those) */
  maxStep: number;
  jumpSpeed: number;
  gravity: number;
}

export const DEFAULT_TUNING: PlayerTuning = {
  walkSpeed: MOVEMENT.walkSpeed,
  runSpeed: MOVEMENT.runSpeed,
  accel: 9,
  turnRate: 12,
  radius: 0.26,
  height: 1.65,
  maxStep: 0.5,
  jumpSpeed: MOVEMENT.jumpSpeed,
  gravity: MOVEMENT.gravity,
};

/** How a vehicle handles. */
export interface RideTuning {
  /** full throttle (m/s, vehicles.json); without run (Shift, or the joystick pushed all the way) a cruise below it */
  topSpeed: number;
  cruise: number;
  /** speeding up / slowing down (m/s²) */
  accel: number;
  brake: number;
  /** fastest turn (rad/s), reached once rolling; slower on the spot */
  turnRate: number;
  /** the bike and its rider as one capsule (m) */
  radius: number;
  height: number;
  /** highest kerb it rolls up (m) */
  maxStep: number;
}

/** run (Shift, or the joystick pushed all the way) on a vehicle goes this much faster than riding along */
const RIDE_BOOST = 1.5;

export function rideTuning(topSpeed: number): RideTuning {
  return { topSpeed, cruise: 1 / RIDE_BOOST, accel: 7.2, brake: 12, turnRate: 2.3, radius: 0.42, height: 1.55, maxStep: 0.4 };
}

/** a stride is cut into steps this long at most (m), well under a body's radius, so nothing is walked through */
const SUBSTEP = 0.1;
/** on the ground the floor is followed down this far in a step (m): stairs and kerbs; further down is a fall */
const SNAP_DOWN = 0.5;
/** the floor ahead is looked for this far past the body's edge (m), to step up before bumping into the step */
const STEP_AHEAD = 0.06;
/** a body stands on the floor under its middle or under four points round it this far out (share of its radius):
 *  at the edge of a quay or a step it stays up there until most of it is past the edge */
const FOOT_RING = 0.7;
/** stepOnto() takes a floor this much higher or more as a step (m); lower, the push off the edge lifts the body */
const STEP_MIN = 0.01;
/** overTheEdge() looks this far round the body's middle when the ray under it finds deep water (m): wider than a
 *  seam between two floor pieces, narrower than anything a body could stand on */
const SEAM_PROBE = 0.08;
/** a step that leaves the body pushed down more than this under something over its head is refused (m) */
const PRESSED = 0.005;
/** a floor change bigger than this in one step eases the drawn avatar over it rather than jumping (m) */
const EASE_STEP = 0.05;
/** headings off the wish direction a step that would end in deep water or off the map tries (rad), the closest
 *  first; never 90°, so a shore that faces the move head-on stops it */
const SLIDE = [0.35, 0.7, 1.05, 1.4];
/** falling never gets faster than this (m/s): keeps a long fall's steps short */
const MAX_FALL = 30;
/** this far below the map's lowest point the player has fallen out of the world (m) */
const OUT_OF_WORLD = 30;
/** landing in deep water puts the player on the nearest dry spot this close (m); further, back home */
const RESCUE_REACH = 6;
/** falling but held up this long (wedged in a crevice between rocks) counts as standing there (s) */
const STUCK_AIR = 0.25;
/** spawn() looks this far round the spot asked for a place the body fits (m) */
const SPAWN_SEARCH = 8;
/** on foot, moving less than this share of the velocity is being stopped (a wall): the velocity drops to it */
const STOPPED = 0.1;
/** on a vehicle, less than this share of the way wanted is a crash: the bike stops as short as it did */
const RIDE_STOPPED = 0.5;
/** a correction closer than this eases the avatar back; further, it jumps (m) */
const EASE_LIMIT = 6;
/** a corrected pose stands on the floor there when one is within this of its height (m) */
const CORRECT_FLOOR = 1;
/** a corrected pose in a wall goes to the nearest spot within this where the body fits (m) */
const CORRECT_SEARCH = 2;
/** getting off: the rider steps this far to the left of the bike (m) */
const DISMOUNT_SIDE = 0.85;
/** a joystick nudge this small does not cancel an emote */
const EMOTE_KEEP = 0.15;

/** The higher of two heights, either of which may be NaN (none). */
function higher(a: number, b: number): number {
  return Number.isFinite(b) && !(b <= a) ? b : a;
}

export class Player {
  readonly avatar = new Pedestrian();
  /** where the body stands: the bottom of its capsule */
  readonly position = new THREE.Vector3();
  heading = 0;
  /** measured ground speed (m/s), drives the animation */
  speed = 0;
  /** what the avatar does on top of walking */
  readonly pose: AvatarPose = idlePose();
  /** the vehicle ridden (its id and handling), null on foot */
  riding: { id: number; tuning: RideTuning } | null = null;
  /** roll into a turn while riding (rad) */
  lean = 0;
  /** the vehicle we sit on behind its rider (a Crab Bike trip), 0 on our own feet or bike */
  seated = 0;
  /** fallen out of the world or into water with no shore near: the app sends the player home */
  onLost: () => void = () => {};
  private readonly vel = new THREE.Vector3();
  private readonly wish = new THREE.Vector3();
  /** where the avatar is drawn relative to `position`, easing to zero after a correction or a step */
  private readonly offset = new THREE.Vector3();
  private readonly scratch = new THREE.Vector3();
  /** where a step would take the body, before it is taken */
  private readonly cand = new THREE.Vector3();
  /** where a step would take the body up onto a kerb it bumped into (tryStep) */
  private readonly upCand = new THREE.Vector3();
  /** the sideways push in the last pushOut() */
  private readonly push = new THREE.Vector3();
  private readonly body: Body;
  /** the floor under the player, for the shadow of a jump */
  private ground = 0;
  private vy = 0;
  private airborne = false;
  /** set by a step over a ledge: the body falls once the step is taken */
  private leaving = false;
  /** how long the body has been falling without coming down (s) */
  private heldFor = 0;
  private emoteCode = 0;
  /** seconds the emote plays; 0 loops until the player moves */
  private emoteSeconds = 0;
  /** signed forward speed on a vehicle (m/s) */
  private rideSpeed = 0;
  private yawRate = 0;

  constructor(private readonly collider: CityCollider, private readonly tuning: PlayerTuning = DEFAULT_TUNING) {
    this.avatar.root.name = 'Player';
    this.body = { radius: tuning.radius, height: tuning.height };
  }

  /** State flags for the server: in the air, riding, the emote playing. */
  get flags(): number {
    // a passenger's flags are the server's to set (it ignores our states while we sit behind someone)
    if (this.seated !== 0) return 0;
    return (this.airborne ? FLAG_AIR : 0) | (this.riding ? FLAG_RIDE : 0) | (this.emoteCode << EMOTE_SHIFT);
  }

  get inAir(): boolean {
    return this.airborne;
  }

  /**
   * Place the player at three.js (x, z), or the nearest spot round it where the body fits, on the floor nearest the
   * height `nearY` (the ground for 0, not a roof or a bridge over it), facing `heading` (0 = +Z).
   */
  spawn(x: number, z: number, heading: number, nearY = 0): void {
    if (this.collider.nearestStanding(x, z, this.body, nearY, SPAWN_SEARCH, this.cand)) this.position.copy(this.cand);
    else this.position.set(x, nearY, z);
    this.heading = heading;
    this.settle();
    this.ground = this.position.y;
    this.offset.set(0, 0, 0);
    this.syncAvatar();
  }

  /** Whether the body fits on dry ground within a step of (x, z), on the floor near the height `y` (none: false). */
  fitsNear(x: number, z: number, y: number | undefined): boolean {
    return y !== undefined && this.collider.nearestStanding(x, z, this.body, y, 1, this.cand);
  }

  /** Jump, if standing on the ground on foot. */
  jump(): boolean {
    if (this.airborne || this.riding || this.seated !== 0) return false;
    this.stopEmote();
    this.vy = this.tuning.jumpSpeed;
    this.airborne = true;
    return true;
  }

  /** Play an emote (server/content/emotes.json code), on the ground and on foot only. */
  playEmote(code: number): boolean {
    const e = emoteByCode(code);
    if (!e || this.riding || this.airborne || this.seated !== 0) return false;
    this.emoteCode = e.code;
    this.emoteSeconds = e.seconds;
    this.pose.emote = e.id;
    this.pose.emoteTime = 0;
    return true;
  }

  stopEmote(): void {
    this.emoteCode = 0;
    this.pose.emote = '';
  }

  /** Get on a vehicle parked at (x, y, z) facing `heading`: the rider hops over onto it. */
  mount(id: number, tuning: RideTuning, x: number, y: number, z: number, heading: number): void {
    this.stopEmote();
    const floor = this.collider.floorNear(x, z, y);
    this.moveTo(x, Number.isFinite(floor) ? floor : y, z, true);
    this.heading = heading;
    this.riding = { id, tuning };
    this.pose.ride = true;
    this.rideSpeed = 0;
  }

  /** Get off to the left of the vehicle, which stays where it is. Returns where it is parked: [x, y, z, h]. */
  dismount(): [number, number, number, number] {
    const p = this.position;
    const parked: [number, number, number, number] = [p.x, p.y, p.z, this.heading];
    const h = this.heading;
    const fit = this.collider.nearestStanding(p.x + Math.cos(h) * DISMOUNT_SIDE, p.z - Math.sin(h) * DISMOUNT_SIDE, this.body, p.y, 3, this.cand, p);
    this.riding = null;
    this.pose.ride = false;
    if (fit) this.moveTo(this.cand.x, this.cand.y, this.cand.z, true);
    else this.moveTo(p.x, p.y, p.z, true);
    return parked;
  }

  /** Sit behind the rider of a vehicle (the server put us in seat 1): from now on carry() places us. */
  sit(vehicle: number): void {
    this.settle();
    this.seated = vehicle;
    this.pose.ride = true;
    this.pose.pillion = true;
  }

  /** Off the seat: on our feet where we are; the server's correction puts us beside the bike. */
  stand(): void {
    if (this.seated === 0) return;
    this.seated = 0;
    this.pose.ride = false;
    this.pose.pillion = false;
    this.lean = 0;
    this.settle();
  }

  /** While seated, every frame: where the rider is drawn (the joystick does nothing meanwhile). */
  carry(at: THREE.Vector3, heading: number, lean: number, speed: number): void {
    this.offset.set(0, 0, 0);
    this.position.copy(at);
    this.ground = at.y;
    this.heading = heading;
    this.lean = lean;
    this.speed = speed;
    this.syncAvatar();
  }

  /**
   * The server says where we are (a refused move, a welcome, getting off someone's bike): go there, easing the
   * avatar over unless it is far. The server knows nothing of the map's shapes, so its pose may lack the height
   * (a home, a spot beside a bike) or be in a wall: the body stands on the floor there, or on the nearest spot it fits.
   */
  correct(x: number, y: number, z: number): void {
    // the server never corrects a seated passenger's moves (it ignores them): this is it putting us down
    this.stand();
    const col = this.collider;
    const body = this.riding ? this.riding.tuning : this.body;
    const floor = col.floorNear(x, z, y);
    let ty = Number.isFinite(floor) && Math.abs(floor - y) < CORRECT_FLOOR ? floor : y;
    if (!col.fits(x, ty, z, body) && col.nearestStanding(x, z, body, ty, CORRECT_SEARCH, this.cand, this.position)) {
      ({ x, y: ty, z } = this.cand);
    }
    this.moveTo(x, ty, z, Math.hypot(x - this.position.x, z - this.position.z) < EASE_LIMIT);
  }

  /**
   * @param forward camera forward on the ground plane (unit vector)
   */
  update(dt: number, move: MoveIntent, forward: THREE.Vector3, time: number): void {
    // camera-relative wish direction: right = (-fz, 0, fx)
    this.wish.set(forward.x * move.y - forward.z * move.x, 0, forward.z * move.y + forward.x * move.x);
    if (this.seated !== 0) {
      this.avatar.update(dt, 0, time, this.pose);
      return;
    }
    const mag = Math.min(1, this.wish.length());
    if (mag > 1e-3) this.wish.multiplyScalar(1 / this.wish.length());
    if (this.riding) this.ride(dt, mag, move.run, this.riding.tuning);
    else this.walk(dt, mag, move.run);
    this.fall(dt);

    if (this.emoteCode !== 0) {
      this.pose.emoteTime += dt;
      const over = this.emoteSeconds > 0 && this.pose.emoteTime >= this.emoteSeconds;
      if (over || mag > EMOTE_KEEP) this.stopEmote();
    }
    this.pose.air = this.airborne;
    this.offset.multiplyScalar(1 - damp(14, dt));
    this.syncAvatar();
    this.avatar.update(dt, this.speed, time, this.pose);
  }

  private walk(dt: number, mag: number, run: boolean): void {
    const t = this.tuning;
    const top = (run ? t.runSpeed : t.walkSpeed) * mag;
    // faster to stop than to start, so the player never skates; little control in the air
    const rate = (mag > 1e-3 ? t.accel : t.accel * 1.6) * (this.airborne ? 0.35 : 1);
    this.vel.x += (this.wish.x * top - this.vel.x) * damp(rate, dt);
    this.vel.z += (this.wish.z * top - this.vel.z) * damp(rate, dt);

    if (mag > 0.05) {
      const goal = Math.atan2(this.wish.x, this.wish.z);
      this.heading = wrapAngle(this.heading + wrapAngle(goal - this.heading) * damp(t.turnRate, dt));
    }

    const moved = this.travel(this.vel.x * dt, this.vel.z * dt, this.body, t.maxStep);
    const measured = dt > 0 ? moved / dt : 0;
    // stopped by a wall: drop the velocity to what actually happened; sliding along one keeps the pace
    const vlen = Math.hypot(this.vel.x, this.vel.z);
    if (vlen > 1e-3 && measured < vlen * STOPPED) this.vel.multiplyScalar(Math.max(0, measured) / vlen);
    this.speed += (measured - this.speed) * damp(14, dt);
    this.lean += (0 - this.lean) * damp(8, dt);
  }

  /** On a vehicle: it only goes where it points, turns toward the wish direction and leans into the turn. */
  private ride(dt: number, mag: number, run: boolean, t: RideTuning): void {
    let goalSpeed = 0;
    let turn = 0;
    if (mag > 0.05) {
      const diff = wrapAngle(Math.atan2(this.wish.x, this.wish.z) - this.heading);
      const rate = t.turnRate * (0.35 + 0.65 * smoothstep(0, 4, Math.abs(this.rideSpeed))) * dt;
      turn = clamp(diff, -rate, rate);
      this.heading = wrapAngle(this.heading + turn);
      // pointing the other way: slow right down and come round in a tight circle
      goalSpeed = t.topSpeed * (run ? 1 : t.cruise) * mag * clamp(Math.cos(diff), 0.2, 1);
    }
    const rate = (goalSpeed > this.rideSpeed ? t.accel : t.brake) * dt;
    this.rideSpeed += clamp(goalSpeed - this.rideSpeed, -rate, rate);
    const want = this.rideSpeed * dt;
    const moved = this.travel(Math.sin(this.heading) * want, Math.cos(this.heading) * want, t, t.maxStep);
    // into a wall: the bike stops as short as it did; a glancing slide keeps its speed
    if (want > 1e-4 && moved < want * RIDE_STOPPED) this.rideSpeed = dt > 0 ? moved / dt : 0;
    this.speed += ((dt > 0 ? moved / dt : 0) - this.speed) * damp(14, dt);
    this.yawRate += ((dt > 0 ? turn / dt : 0) - this.yawRate) * damp(10, dt);
    const lean = clamp(-this.yawRate * this.speed * 0.07, -0.42, 0.42);
    this.lean += (lean - this.lean) * damp(6, dt);
  }

  /** Move by (dx, dz) in short steps through the map. Returns the distance covered on the ground plane. */
  private travel(dx: number, dz: number, body: Body, maxStep: number): number {
    const len = Math.hypot(dx, dz);
    if (len < 1e-6) return 0;
    const n = Math.ceil(len / SUBSTEP);
    const sx = dx / n;
    const sz = dz / n;
    let covered = 0;
    for (let i = 0; i < n; i++) {
      const d = this.stride(sx, sz, body, maxStep);
      if (d <= 0) break;
      covered += d;
    }
    return covered;
  }

  /** One step, or where it would end in deep water or off the map, the nearest heading along the shore. */
  private stride(sx: number, sz: number, body: Body, maxStep: number): number {
    const moved = this.tryStep(sx, sz, body, maxStep);
    if (moved >= 0) return moved;
    const a0 = Math.atan2(sx, sz);
    const len = Math.hypot(sx, sz);
    for (const off of SLIDE) {
      const k = len * Math.cos(off);
      for (let side = 1; side >= -1; side -= 2) {
        const a = a0 + off * side;
        const m = this.tryStep(Math.sin(a) * k, Math.cos(a) * k, body, maxStep);
        if (m >= 0) return m;
      }
    }
    return 0;
  }

  /**
   * Take a step of (sx, sz) if it can be taken: onto the floor (up a step, down a kerb), pushed out of walls, or
   * over a ledge onto dry ground below (the body then falls). Returns the distance moved on the ground plane, or -1
   * where the step would end in deep water or over the edge of the map.
   */
  private tryStep(sx: number, sz: number, body: Body, maxStep: number): number {
    const p = this.position;
    const c = this.cand.set(p.x + sx, p.y, p.z + sz);
    this.leaving = false;
    let floor = NaN;
    if (!this.airborne) {
      floor = this.footing(c.x, c.z, p.y + maxStep + 0.01, maxStep + 0.01 + SNAP_DOWN, body.radius, sx, sz);
      if (Number.isFinite(floor)) c.y = floor;
      // else over a ledge: the body falls, onto dry ground somewhere below (checked once pushed out of walls)
      else this.leaving = true;
    }
    const wantX = c.x;
    const wantZ = c.z;
    this.pushOut(c, body);
    // bumped into a kerb or a platform's edge at a slant: the probes ahead (footing) look along the move and miss
    // it, the more so the shorter the step (a 120 Hz screen). Up onto it, as when walking at it straight on
    if (Number.isFinite(floor)) {
      const up = this.stepOnto(c, floor, body.radius, maxStep);
      if (Number.isFinite(up)) {
        const u = this.upCand.set(wantX, up, wantZ);
        this.pushOut(u, body);
        // only with room up there (no branch over the head, not out over the water): else the step as it was, along
        // what it bumped into
        if (up - u.y <= PRESSED && !this.overTheEdge(u, body)) {
          c.copy(u);
          floor = up;
        }
      }
    }
    // never with the body's middle over deep water or the edge of the map, even standing on a railing there: it
    // would come down there
    if (this.overTheEdge(c, body)) return -1;
    // pressed down under something lower than the body (a bridge's end over the bank): no room there. Even a little
    // each step would let it creep in under and wedge; but where it is pressed as much already (put there by a
    // correction), a step that frees it is fine
    if (Number.isFinite(floor) && floor - c.y > PRESSED && floor - c.y >= this.pressedHere(body, maxStep) - 1e-4) return 0;
    const moved = Math.hypot(c.x - p.x, c.z - p.z);
    const dy = c.y - p.y;
    if (!this.airborne && Math.abs(dy) > EASE_STEP) this.offset.y -= dy;
    p.copy(c);
    if (this.leaving) {
      this.leaving = false;
      this.airborne = true;
      this.vy = 0;
    }
    return moved;
  }

  /**
   * The highest floor flat enough to stand on, under the body standing at (x, z): under its middle, under four points
   * round it (FOOT_RING), and, moving along (dx, dz), just past its front edge (so it steps up a kerb before bumping
   * into it). Looked for from `from` down `reach` metres; NaN where there is none (a ledge).
   */
  private footing(x: number, z: number, from: number, reach: number, radius: number, dx: number, dz: number, flat = true): number {
    const col = this.collider;
    const k = radius * FOOT_RING;
    let best = col.floorBelow(x, from, z, reach, flat);
    best = higher(best, col.floorBelow(x + k, from, z, reach, flat));
    best = higher(best, col.floorBelow(x - k, from, z, reach, flat));
    best = higher(best, col.floorBelow(x, from, z + k, reach, flat));
    best = higher(best, col.floorBelow(x, from, z - k, reach, flat));
    const len = Math.hypot(dx, dz);
    if (len > 1e-6) {
      const a = (radius + STEP_AHEAD) / len;
      best = higher(best, col.floorBelow(x + dx * a, from, z + dz * a, reach, flat));
    }
    return best;
  }

  /**
   * Push a body at `c` out of what it overlaps, twice (the first pass can push it from one triangle into another in a
   * corner). Leaves the total sideways push in `this.push`.
   */
  private pushOut(c: THREE.Vector3, body: Body): void {
    const col = this.collider;
    this.push.set(0, 0, 0);
    if (col.collide(c, body) === 0) return;
    this.push.add(col.sidePush);
    if (col.collide(c, body) !== 0) this.push.add(col.sidePush);
  }

  /**
   * How far down whatever is over the body pushes it where it stands now, measured from the floor there as a step
   * is (m): 0 with room over its head.
   */
  private pressedHere(body: Body, maxStep: number): number {
    const p = this.position;
    const floor = this.footing(p.x, p.z, p.y + maxStep + 0.01, maxStep + 0.01 + SNAP_DOWN, body.radius, 0, 0);
    if (!Number.isFinite(floor)) return 0;
    const q = this.scratch.set(p.x, floor, p.z);
    if (this.collider.collide(q, body) !== 0) this.collider.collide(q, body);
    return Math.max(0, floor - q.y);
  }

  /**
   * The top of a step the body at `c` (on `floor`) was just pushed back from (pushOut): the floor just past its
   * edge the other way from the push, if it is higher than `floor` by no more than a step. NaN for none: a wall, a
   * post or a fence has no floor there within a step's height.
   */
  private stepOnto(c: THREE.Vector3, floor: number, radius: number, maxStep: number): number {
    const len = Math.hypot(this.push.x, this.push.z);
    if (len < 1e-6) return NaN;
    const k = -(radius + STEP_AHEAD) / len;
    const top = this.collider.floorBelow(c.x + this.push.x * k, floor + maxStep + 0.01, c.z + this.push.z * k, maxStep, true);
    return top > floor + STEP_MIN ? top : NaN;
  }

  /**
   * Whether a body at `c` would come down in deep water or off the edge of the map: what is under its middle. A ray
   * that slips through a seam in the floor down to the seabed is not water: a few round it decide.
   */
  private overTheEdge(c: THREE.Vector3, body: Body): boolean {
    const col = this.collider;
    const from = c.y + body.radius;
    const below = col.floorBelow(c.x, from, c.z, FALL_LIMIT);
    if (Number.isFinite(below) && !col.isDeep(c.x, c.z, below)) return false;
    for (let i = 0; i < 4; i++) {
      const x = c.x + SEAM_PROBE * (i === 0 ? 1 : i === 1 ? -1 : 0);
      const z = c.z + SEAM_PROBE * (i === 2 ? 1 : i === 3 ? -1 : 0);
      const f = col.floorBelow(x, from, z, FALL_LIMIT);
      if (Number.isFinite(f) && !col.isDeep(x, z, f)) return false;
    }
    return true;
  }

  /** Gravity in a jump or a fall, landing on whatever is below; on the ground, notice a floor that went away. */
  private fall(dt: number): void {
    const p = this.position;
    const col = this.collider;
    const body = this.riding ? this.riding.tuning : this.body;
    if (!this.airborne) {
      // standing still where the floor went away (a correction to mid-air): fall. Any surface holds a body that
      // stands still, a steep one too (wedged between rocks); walking off it is a fall (tryStep).
      if (!Number.isFinite(this.footing(p.x, p.z, p.y + 0.05, SNAP_DOWN + 0.05, body.radius, 0, 0, false))) {
        this.airborne = true;
        this.vy = 0;
      } else {
        this.ground = p.y;
        return;
      }
    }
    this.vy = Math.max(-MAX_FALL, this.vy - this.tuning.gravity * dt);
    const dy = this.vy * dt;
    const fromY = p.y;
    const n = Math.max(1, Math.ceil(Math.abs(dy) / (body.radius * 0.5)));
    for (let i = 0; i < n && this.airborne; i++) {
      const c = this.cand.copy(p);
      c.y += dy / n;
      const hit = col.collide(c, body);
      // pushed sideways off an edge: never out over deep water or the edge of the map
      if ((c.x !== p.x || c.z !== p.z) && this.overTheEdge(c, body)) {
        c.x = p.x;
        c.z = p.z;
      }
      p.copy(c);
      if ((hit & CONTACT_FLOOR) !== 0 && this.vy <= 0) this.land();
      else if ((hit & CONTACT_CEILING) !== 0 && this.vy > 0) this.vy = 0;
    }
    // falling but held where it is (wedged in a crevice, its sides too steep to stand on): it stands there, so it
    // can jump out, and in water too deep, it is helped out
    this.heldFor = this.airborne && dy < 0 && fromY - p.y < 0.002 ? this.heldFor + dt : 0;
    if (this.heldFor > STUCK_AIR) {
      this.heldFor = 0;
      this.land();
    }
    if (this.airborne && this.vy <= 0) {
      // touching down on a floor, or on an edge under part of the body (a touch is not an overlap, so collide()
      // does not report it, and an edge pushes the body sideways rather than up)
      const f = this.footing(p.x, p.z, p.y + 0.05, 0.08, body.radius, 0, 0);
      if (Number.isFinite(f)) {
        p.y = Math.max(p.y, f);
        this.land();
      }
    }
    if (this.airborne) {
      const g = col.floorBelow(p.x, p.y + 0.05, p.z, FALL_LIMIT);
      this.ground = Number.isFinite(g) ? g : p.y;
      if (p.y < col.bottom - OUT_OF_WORLD) this.onLost();
    } else {
      this.ground = p.y;
    }
  }

  /** On the ground again; in water too deep to stand in, out onto the nearest dry spot. */
  private land(): void {
    this.airborne = false;
    this.vy = 0;
    this.heldFor = 0;
    const p = this.position;
    if (!this.collider.isDeep(p.x, p.z, p.y)) return;
    const body = this.riding ? this.riding.tuning : this.body;
    // the nearest dry spot in sight, on a floor near the water's surface (not the bottom); wedged between rocks,
    // where nothing is in sight, the nearest one at all
    const col = this.collider;
    const surface = p.y + col.waterDepth(p.x, p.z, p.y);
    if (col.nearestStanding(p.x, p.z, body, surface, RESCUE_REACH, this.scratch, p) || col.nearestStanding(p.x, p.z, body, surface, RESCUE_REACH, this.scratch)) {
      this.moveTo(this.scratch.x, this.scratch.y, this.scratch.z, true);
    } else {
      this.onLost();
    }
  }

  /** Put the player somewhere else at once; `ease` keeps the avatar where it was and lets it catch up. */
  private moveTo(x: number, y: number, z: number, ease: boolean): void {
    const ny = Number.isFinite(y) ? y : this.position.y;
    if (ease) this.offset.add(this.position).sub(this.scratch.set(x, ny, z));
    else this.offset.set(0, 0, 0);
    this.position.set(x, ny, z);
    this.ground = ny;
    this.settle();
  }

  /** Stop: no velocity, no jump, no emote. */
  private settle(): void {
    this.vel.set(0, 0, 0);
    this.speed = 0;
    this.rideSpeed = 0;
    this.vy = 0;
    this.airborne = false;
    this.stopEmote();
  }

  private syncAvatar(): void {
    this.avatar.root.position.copy(this.position).add(this.offset);
    this.avatar.root.rotation.y = this.heading;
    this.avatar.root.rotation.z = this.lean;
    this.avatar.setHeight(this.airborne ? this.position.y - this.ground : 0);
  }

  /** Where the avatar is drawn (the position plus any correction still easing out). */
  get drawn(): THREE.Vector3 {
    return this.avatar.root.position;
  }
}
