// Player movement on the nav grid: camera-relative walking / running with acceleration, smooth turning,
// sliding along walls at any angle, and following the floor height over kerbs, plazas and the bridge. Also
// jumping (a jump can land on something up to its height plus a step higher), emotes, riding a motorbike, and
// going back to where the server says when it refuses a move.
//
// The server checks every move against the same nav grid and the same speeds (server/content/movement.json and
// vehicles.json), so what this allows, it allows.
import * as THREE from 'three';
import type { MoveIntent } from './input';
import type { NavGrid } from './nav';
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
  /** body radius on the nav grid (m) */
  radius: number;
  /** highest floor change one stride can climb (m): kerbs yes, walls no */
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
  maxStep: 0.45,
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
  /** footprint on the nav grid (m) */
  radius: number;
}

/** run (Shift, or the joystick pushed all the way) on a vehicle goes this much faster than riding along */
const RIDE_BOOST = 1.5;

export function rideTuning(topSpeed: number): RideTuning {
  return { topSpeed, cruise: 1 / RIDE_BOOST, accel: 7.2, brake: 12, turnRate: 2.3, radius: 0.42 };
}

/** headings to try, in order, when the straight move is blocked (radians off the wish direction) */
const SLIDE = [0.35, -0.35, 0.7, -0.7, 1.05, -1.05, 1.35, -1.35];
const SUBSTEP = 0.12;
/** a correction closer than this eases the avatar back; further, it jumps (m) */
const EASE_LIMIT = 6;
/** getting off: the rider steps this far to the left of the bike (m) */
const DISMOUNT_SIDE = 0.85;
/** a joystick nudge this small does not cancel an emote */
const EMOTE_KEEP = 0.15;

export class Player {
  readonly avatar = new Pedestrian();
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
  private readonly vel = new THREE.Vector3();
  private readonly wish = new THREE.Vector3();
  /** where the avatar is drawn relative to `position`, easing to zero after a correction */
  private readonly offset = new THREE.Vector3();
  private readonly scratch = new THREE.Vector3();
  private floor = 0;
  /** the blended floor under the player, for the shadow */
  private ground = 0;
  private vy = 0;
  private airborne = false;
  private emoteCode = 0;
  /** seconds the emote plays; 0 loops until the player moves */
  private emoteSeconds = 0;
  /** signed forward speed on a vehicle (m/s) */
  private rideSpeed = 0;
  private yawRate = 0;
  /** the floor heights the next step may reach, from reach() */
  private lo = 0;
  private hi = 0;

  constructor(private readonly nav: NavGrid, private readonly tuning: PlayerTuning = DEFAULT_TUNING) {
    this.avatar.root.name = 'Player';
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

  /** Place the player at three.js (x, z), or the nearest spot that fits, facing `heading` (0 = +Z). */
  spawn(x: number, z: number, heading: number): void {
    const fit = this.nav.nearestStanding(x, z, this.tuning.radius);
    if (fit) [x, z] = fit;
    this.floor = this.nav.floorAt(x, z);
    if (!Number.isFinite(this.floor)) this.floor = 0;
    this.position.set(x, this.floor, z);
    this.heading = heading;
    this.settle();
    this.offset.set(0, 0, 0);
    this.syncAvatar();
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

  /** Get on a vehicle parked at (x, z) facing `heading`: the rider hops over onto it. */
  mount(id: number, tuning: RideTuning, x: number, z: number, heading: number): void {
    this.stopEmote();
    this.moveTo(x, this.nav.floorAt(x, z), z, true);
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
    const fit = this.nav.nearestStanding(p.x + Math.cos(h) * DISMOUNT_SIDE, p.z - Math.sin(h) * DISMOUNT_SIDE, this.tuning.radius, 3);
    const [x, z] = fit ?? [p.x, p.z];
    this.riding = null;
    this.pose.ride = false;
    this.moveTo(x, this.nav.floorAt(x, z), z, true);
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
    this.floor = this.ground = at.y;
    this.heading = heading;
    this.lean = lean;
    this.speed = speed;
    this.syncAvatar();
  }

  /** The server refused a move: go back to its pose, easing the avatar there unless it is far. */
  correct(x: number, y: number, z: number): void {
    // the server never corrects a seated passenger's moves (it ignores them): this is it putting us down
    this.stand();
    this.moveTo(x, y, z, Math.hypot(x - this.position.x, z - this.position.z) < EASE_LIMIT);
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

    const moved = this.travel(this.vel.x * dt, this.vel.z * dt, t.radius);
    const measured = dt > 0 ? moved / dt : 0;
    // walking into a wall: drop the velocity to what actually happened
    const vlen = Math.hypot(this.vel.x, this.vel.z);
    if (vlen > 1e-3 && measured < vlen * 0.98) this.vel.multiplyScalar(Math.max(0, measured) / vlen);
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
    const moved = this.travel(Math.sin(this.heading) * want, Math.cos(this.heading) * want, t.radius);
    // into a wall: the bike stops as short as it did
    if (want > 1e-4 && moved < want * 0.98) this.rideSpeed = dt > 0 ? moved / dt : 0;
    this.speed += ((dt > 0 ? moved / dt : 0) - this.speed) * damp(14, dt);
    this.yawRate += ((dt > 0 ? turn / dt : 0) - this.yawRate) * damp(10, dt);
    const lean = clamp(-this.yawRate * this.speed * 0.07, -0.42, 0.42);
    this.lean += (lean - this.lean) * damp(6, dt);
  }

  /** Gravity in a jump; on the ground, ease up and down kerbs. */
  private fall(dt: number): void {
    const p = this.position;
    const ground = this.nav.smoothFloor(p.x, p.z, this.floor);
    this.ground = ground;
    if (this.airborne) {
      this.vy -= this.tuning.gravity * dt;
      p.y += this.vy * dt;
      if (p.y <= ground) {
        // coming down, or rising into something it jumped onto
        p.y = ground;
        if (this.vy <= 0) {
          this.airborne = false;
          this.vy = 0;
        }
      }
      return;
    }
    p.y += (ground - p.y) * damp(ground > p.y ? 22 : 16, dt);
  }

  /** The floor heights a step may reach (into lo, hi): one step either way, up to a step above the feet in a jump. */
  private reach(): void {
    const step = this.tuning.maxStep;
    this.lo = this.floor - step;
    this.hi = (this.airborne ? Math.max(this.floor, this.position.y) : this.floor) + step;
  }

  /** Move by (dx, dz) in small substeps, sliding along obstacles. Returns the distance covered. */
  private travel(dx: number, dz: number, bodyRadius: number): number {
    const len = Math.hypot(dx, dz);
    if (len < 1e-6) return 0;
    const n = Math.ceil(len / SUBSTEP);
    const sx = dx / n;
    const sz = dz / n;
    const step = len / n;
    // if the body already overlaps a wall (a respawn, a teleport), only the centre has to stay walkable
    const p = this.position;
    this.reach();
    const radius = this.nav.canStandBetween(p.x, p.z, bodyRadius, this.lo, this.hi) ? bodyRadius : 0;
    let covered = 0;
    for (let i = 0; i < n; i++) {
      const d = this.tryStep(sx, sz, step, radius);
      if (d === 0) break;
      covered += d;
    }
    return covered;
  }

  private tryStep(sx: number, sz: number, step: number, radius: number): number {
    const p = this.position;
    this.reach();
    const { lo, hi } = this;
    if (this.nav.canStandBetween(p.x + sx, p.z + sz, radius, lo, hi)) {
      this.commit(p.x + sx, p.z + sz);
      return step;
    }
    const a0 = Math.atan2(sx, sz);
    for (const off of SLIDE) {
      const k = Math.cos(off);
      const a = a0 + off;
      const nx = p.x + Math.sin(a) * step * k;
      const nz = p.z + Math.cos(a) * step * k;
      if (this.nav.canStandBetween(nx, nz, radius, lo, hi)) {
        this.commit(nx, nz);
        return step * k;
      }
    }
    return 0;
  }

  private commit(x: number, z: number): void {
    this.position.x = x;
    this.position.z = z;
    this.floor = this.nav.floorAt(x, z);
  }

  /** Put the player somewhere else at once; `ease` keeps the avatar where it was and lets it catch up. */
  private moveTo(x: number, y: number, z: number, ease: boolean): void {
    if (ease) this.offset.add(this.position).sub(this.scratch.set(x, Number.isFinite(y) ? y : this.position.y, z));
    else this.offset.set(0, 0, 0);
    this.floor = this.nav.floorAt(x, z);
    if (!Number.isFinite(this.floor)) this.floor = this.position.y;
    this.position.set(x, Number.isFinite(y) ? y : this.floor, z);
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
