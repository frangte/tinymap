// Third-person follow camera: orbits a pivot above the player's head (drag / wheel / pinch), drifts back
// behind the player while walking forward, and cranes up over a building behind the player (pulling in only as far
// as it still has to) instead of entering it. Walls are the map's own shapes (collider.ts); tree crowns, trunks and
// iron railings do not count (paint.ts fades the crowns in the way), and neither does a pole, which only one of three
// side by side sight lines runs into.
import * as THREE from 'three';
import type { LookDelta } from './input';
import type { CityCollider } from './collider';
import { clamp, damp, wrapAngle } from './coords';

const PIVOT_H = 1.45;
const MIN_DIST = 2.6;
const MAX_DIST = 48;
const MIN_PITCH = 0.06;
const MAX_PITCH = 1.3;
/** keep this far in front of an occluding wall */
const WALL_GAP = 0.55;
/** with a building behind the player, look down from higher up until the camera has this much room (m) */
const ROOM = 5;
/** how much higher each try looks from (rad) */
const LIFT_STEP = 0.12;
/** the sight line from the pivot is also tried this far to either side (m): what blocks one of the three only is
 *  thin (a lamp post, a trunk), and the camera does not jump in for it */
const SIDE_SIGHT = 0.45;
/** the camera stays this far over the floor below it (m) */
const ABOVE_FLOOR = 0.5;
/** ...easing up to it over a kerb, a parapet or a roof edge passing under it, but never closer than this (m) */
const MIN_ABOVE_FLOOR = 0.15;
/** after pulling in for a wall the camera backs out again no faster than this (m/s): an exponential way back out
 *  starts at 30 m/s or more, which reads as the camera jerking away from the player */
const OUT_SPEED = 4;

/** Close framing of the whole body for the dressing panel. */
export interface Framing {
  /** look-at height above the feet (m); lower puts the player higher on screen */
  height: number;
  dist: number;
  pitch: number;
}

export class FollowCamera {
  /** horizontal angle of the camera around the player; the camera looks along (-sin yaw, -cos yaw) */
  yaw = 0;
  pitch = 0.46;
  /** set while dressing: ease into this framing; null = normal follow, easing the pitch back to where it was */
  portrait: Framing | null = null;
  private wasPortrait = false;
  private pitchBefore = 0.46;
  private pitchReturn: number | null = null;
  private goalDist = 10.5;
  private dist = 10.5;
  private readonly pivot = new THREE.Vector3();
  private readonly dir = new THREE.Vector3();
  private readonly probe = new THREE.Vector3();
  private readonly side = new THREE.Vector3();
  /** extra pitch while a building behind the player is in the way (eased, on top of the user's pitch) */
  private lift = 0;
  /** how far the camera is raised over what passes under it (eased) */
  private floorLift = 0;
  /** set while the camera makes its way back out after a wall pulled it in */
  private backingOut = false;

  constructor(private readonly camera: THREE.PerspectiveCamera, private readonly collider: CityCollider) {}

  /** Jump straight behind a player at `target` facing `heading`. */
  snapBehind(target: THREE.Vector3, heading: number): void {
    this.yaw = heading + Math.PI;
    this.pivot.set(target.x, target.y + PIVOT_H, target.z);
    this.dist = this.goalDist;
    this.place(0);
  }

  /**
   * @param follow 0…1, how strongly to swing back behind the player this frame (0 while the user steers the camera)
   */
  update(dt: number, target: THREE.Vector3, heading: number, look: LookDelta, follow: number): void {
    this.yaw = wrapAngle(this.yaw + look.yaw);
    this.pitch = clamp(this.pitch + look.pitch, MIN_PITCH, MAX_PITCH);
    this.goalDist = clamp(this.goalDist * Math.exp(look.zoom), MIN_DIST, MAX_DIST);
    const frame = this.portrait;
    if (frame && !this.wasPortrait) this.pitchBefore = this.pitch;
    if (!frame && this.wasPortrait) this.pitchReturn = this.pitchBefore;
    this.wasPortrait = frame !== null;
    if (frame) {
      this.pitch += (frame.pitch - this.pitch) * damp(5, dt);
    } else if (this.pitchReturn !== null) {
      // a drag takes over from the easing
      if (look.pitch !== 0 || Math.abs(this.pitchReturn - this.pitch) < 1e-3) this.pitchReturn = null;
      else this.pitch += (this.pitchReturn - this.pitch) * damp(4, dt);
    }
    if (follow > 0) {
      const behind = wrapAngle(heading + Math.PI - this.yaw);
      // never whip round when the player turns to face the camera
      if (Math.abs(behind) < 2.3) this.yaw = wrapAngle(this.yaw + behind * damp(1.7 * follow, dt));
    }

    // the pivot trails the player slightly, which reads as weight
    this.pivot.x += (target.x - this.pivot.x) * damp(14, dt);
    this.pivot.z += (target.z - this.pivot.z) * damp(14, dt);
    this.pivot.y += (target.y + (frame ? frame.height : PIVOT_H) - this.pivot.y) * damp(frame ? 5 : 9, dt);

    const want = frame ? frame.dist : this.goalDist;
    // a wall behind the player: crane up over it rather than pull into the player's back
    let goal = 0;
    const room = Math.min(want, ROOM);
    const level = frame ? want : this.clearDistance(want, this.pitch);
    if (level < room) {
      // the lowest pitch that gives the camera its room, else the one that gives it the most (if that helps at all)
      let best = level + 1;
      for (let p = this.pitch + LIFT_STEP; p <= MAX_PITCH + 1e-6; p += LIFT_STEP) {
        const d = this.clearDistance(want, p);
        if (d >= room) {
          goal = p - this.pitch;
          break;
        }
        if (d > best) {
          best = d;
          goal = p - this.pitch;
        }
      }
    }
    this.lift += (goal - this.lift) * damp(goal > this.lift ? 8 : 2, dt);
    const free = this.clearDistance(want, this.pitch + this.lift);
    // pull in at once for a wall, glide in for the dressing framing, ease back out (slowly after a wall: OUT_SPEED)
    const inRate = frame && free >= want ? 5 : 30;
    if (free < this.dist) {
      this.dist += (free - this.dist) * damp(inRate, dt);
      if (free < want) this.backingOut = true;
    } else {
      const step = (free - this.dist) * damp(3.5, dt);
      this.dist += this.backingOut ? Math.min(step, OUT_SPEED * dt) : step;
      if (free - this.dist < 0.05) this.backingOut = false;
    }
    this.place(dt);
  }

  private setDir(pitch = this.pitch + this.lift): void {
    const p = Math.min(pitch, MAX_PITCH);
    const cp = Math.cos(p);
    this.dir.set(Math.sin(this.yaw) * cp, Math.sin(p), Math.cos(this.yaw) * cp);
  }

  /** How far the camera can back away from the pivot, looking down at `pitch`, before it would enter a building. */
  private clearDistance(want: number, pitch: number): number {
    this.setDir(pitch);
    const col = this.collider;
    const mid = col.sightDistance(this.pivot, this.dir, want);
    if (mid >= want) return want;
    // the horizontal at right angles to the sight line
    this.side.set(Math.cos(this.yaw), 0, -Math.sin(this.yaw));
    const left = col.sightDistance(this.probe.copy(this.pivot).addScaledVector(this.side, SIDE_SIGHT), this.dir, want);
    const right = col.sightDistance(this.probe.copy(this.pivot).addScaledVector(this.side, -SIDE_SIGHT), this.dir, want);
    // the middle one of the three: a wall blocks two or three of them, a pole one
    const d = Math.max(Math.min(mid, left), Math.min(Math.max(mid, left), right));
    return d >= want ? want : Math.max(0.8, d - WALL_GAP);
  }

  /** @param dt 0 snaps (a jump behind the player) */
  private place(dt: number): void {
    this.setDir();
    const cam = this.camera.position;
    cam.copy(this.pivot).addScaledVector(this.dir, this.dist);
    // stay above the ground behind the player (slopes, the hill, a bridge); tree crowns do not count
    const from = Math.max(this.pivot.y, cam.y);
    const floor = this.collider.cameraFloor(cam.x, from, cam.z, from - cam.y + ABOVE_FLOOR + 3);
    const need = Number.isFinite(floor) ? Math.max(0, floor + ABOVE_FLOOR - cam.y) : 0;
    // ease up over what passes under the camera instead of hopping, and back down slowly
    this.floorLift = dt > 0 ? this.floorLift + (need - this.floorLift) * damp(need > this.floorLift ? 10 : 3, dt) : need;
    cam.y += Math.max(this.floorLift, need - (ABOVE_FLOOR - MIN_ABOVE_FLOOR));
    this.camera.lookAt(this.pivot);
  }

  /** The point the camera looks at (above the player's head). */
  get target(): THREE.Vector3 {
    return this.pivot;
  }

  /** The direction the player walks when pushing "forward", on the ground plane. */
  forward(out: THREE.Vector3): THREE.Vector3 {
    return out.set(-Math.sin(this.yaw), 0, -Math.cos(this.yaw));
  }
}
