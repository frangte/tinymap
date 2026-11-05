// Player movement on the nav grid: camera-relative walking / running with acceleration, smooth turning,
// sliding along walls at any angle, and following the floor height over kerbs, plazas and the bridge.
import * as THREE from 'three';
import type { MoveIntent } from './input';
import type { NavGrid } from './nav';
import { damp, wrapAngle } from './coords';
import { Pedestrian } from './character';

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
}

export const DEFAULT_TUNING: PlayerTuning = {
  walkSpeed: 3.1,
  runSpeed: 6.8,
  accel: 9,
  turnRate: 12,
  radius: 0.26,
  maxStep: 0.45,
};

/** headings to try, in order, when the straight move is blocked (radians off the wish direction) */
const SLIDE = [0.35, -0.35, 0.7, -0.7, 1.05, -1.05, 1.35, -1.35];
const SUBSTEP = 0.12;

export class Player {
  readonly avatar = new Pedestrian();
  readonly position = new THREE.Vector3();
  heading = 0;
  /** measured ground speed (m/s), drives the animation */
  speed = 0;
  private readonly vel = new THREE.Vector3();
  private readonly wish = new THREE.Vector3();
  private floor = 0;

  constructor(private readonly nav: NavGrid, private readonly tuning: PlayerTuning = DEFAULT_TUNING) {
    this.avatar.root.name = 'Player';
  }

  /** Place the player at three.js (x, z), or the nearest spot that fits, facing `heading` (0 = +Z). */
  spawn(x: number, z: number, heading: number): void {
    const fit = this.nav.nearestStanding(x, z, this.tuning.radius);
    if (fit) [x, z] = fit;
    this.floor = this.nav.floorAt(x, z);
    if (!Number.isFinite(this.floor)) this.floor = 0;
    this.position.set(x, this.floor, z);
    this.heading = heading;
    this.vel.set(0, 0, 0);
    this.speed = 0;
    this.syncAvatar();
  }

  /**
   * @param forward camera forward on the ground plane (unit vector)
   */
  update(dt: number, move: MoveIntent, forward: THREE.Vector3, time: number): void {
    const t = this.tuning;
    // camera-relative wish direction: right = (-fz, 0, fx)
    this.wish.set(forward.x * move.y - forward.z * move.x, 0, forward.z * move.y + forward.x * move.x);
    const mag = Math.min(1, this.wish.length());
    if (mag > 1e-3) this.wish.multiplyScalar(1 / this.wish.length());
    const top = (move.run ? t.runSpeed : t.walkSpeed) * mag;
    // faster to stop than to start, so the player never skates
    const rate = mag > 1e-3 ? t.accel : t.accel * 1.6;
    this.vel.x += (this.wish.x * top - this.vel.x) * damp(rate, dt);
    this.vel.z += (this.wish.z * top - this.vel.z) * damp(rate, dt);

    if (mag > 0.05) {
      const goal = Math.atan2(this.wish.x, this.wish.z);
      this.heading = wrapAngle(this.heading + wrapAngle(goal - this.heading) * damp(t.turnRate, dt));
    }

    const moved = this.travel(this.vel.x * dt, this.vel.z * dt);
    const measured = dt > 0 ? moved / dt : 0;
    // walking into a wall: drop the velocity to what actually happened
    const vlen = Math.hypot(this.vel.x, this.vel.z);
    if (vlen > 1e-3 && measured < vlen * 0.98) this.vel.multiplyScalar(Math.max(0, measured) / vlen);
    this.speed += (measured - this.speed) * damp(14, dt);

    const goalY = this.nav.smoothFloor(this.position.x, this.position.z, this.floor);
    this.position.y += (goalY - this.position.y) * damp(goalY > this.position.y ? 22 : 16, dt);
    this.syncAvatar();
    this.avatar.update(dt, this.speed, time);
  }

  /** Move by (dx, dz) in small substeps, sliding along obstacles. Returns the distance covered. */
  private travel(dx: number, dz: number): number {
    const len = Math.hypot(dx, dz);
    if (len < 1e-6) return 0;
    const n = Math.ceil(len / SUBSTEP);
    const sx = dx / n;
    const sz = dz / n;
    const step = len / n;
    // if the body already overlaps a wall (a respawn, a teleport), only the centre has to stay walkable
    const p = this.position;
    const radius = this.nav.canStand(p.x, p.z, this.tuning.radius, this.floor, this.tuning.maxStep) ? this.tuning.radius : 0;
    let covered = 0;
    for (let i = 0; i < n; i++) {
      const d = this.tryStep(sx, sz, step, radius);
      if (d === 0) break;
      covered += d;
    }
    return covered;
  }

  private tryStep(sx: number, sz: number, step: number, radius: number): number {
    const { maxStep } = this.tuning;
    const p = this.position;
    if (this.nav.canStand(p.x + sx, p.z + sz, radius, this.floor, maxStep)) {
      this.commit(p.x + sx, p.z + sz);
      return step;
    }
    const a0 = Math.atan2(sx, sz);
    for (const off of SLIDE) {
      const k = Math.cos(off);
      const a = a0 + off;
      const nx = p.x + Math.sin(a) * step * k;
      const nz = p.z + Math.cos(a) * step * k;
      if (this.nav.canStand(nx, nz, radius, this.floor, maxStep)) {
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

  private syncAvatar(): void {
    this.avatar.root.position.copy(this.position);
    this.avatar.root.rotation.y = this.heading;
  }
}
