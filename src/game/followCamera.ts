// Third-person follow camera: orbits a pivot above the player's head (drag / wheel / pinch), drifts back
// behind the player while walking forward, and pulls in instead of entering a building.
import * as THREE from 'three';
import type { LookDelta } from './input';
import type { NavGrid } from './nav';
import { clamp, damp, wrapAngle } from './coords';

const PIVOT_H = 1.45;
const MIN_DIST = 2.6;
const MAX_DIST = 48;
const MIN_PITCH = 0.06;
const MAX_PITCH = 1.3;
/** keep this far in front of an occluding wall */
const WALL_GAP = 0.55;

export class FollowCamera {
  /** horizontal angle of the camera around the player; the camera looks along (-sin yaw, -cos yaw) */
  yaw = 0;
  pitch = 0.46;
  private goalDist = 10.5;
  private dist = 10.5;
  private readonly pivot = new THREE.Vector3();
  private readonly dir = new THREE.Vector3();
  private readonly probe = new THREE.Vector3();

  constructor(private readonly camera: THREE.PerspectiveCamera, private readonly nav: NavGrid) {}

  /** Jump straight behind a player at `target` facing `heading`. */
  snapBehind(target: THREE.Vector3, heading: number): void {
    this.yaw = heading + Math.PI;
    this.pivot.set(target.x, target.y + PIVOT_H, target.z);
    this.dist = this.goalDist;
    this.place();
  }

  /**
   * @param follow 0…1, how strongly to swing back behind the player this frame (0 while the user steers the camera)
   */
  update(dt: number, target: THREE.Vector3, heading: number, look: LookDelta, follow: number): void {
    this.yaw = wrapAngle(this.yaw + look.yaw);
    this.pitch = clamp(this.pitch + look.pitch, MIN_PITCH, MAX_PITCH);
    this.goalDist = clamp(this.goalDist * Math.exp(look.zoom), MIN_DIST, MAX_DIST);
    if (follow > 0) {
      const behind = wrapAngle(heading + Math.PI - this.yaw);
      // never whip round when the player turns to face the camera
      if (Math.abs(behind) < 2.3) this.yaw = wrapAngle(this.yaw + behind * damp(1.7 * follow, dt));
    }

    // the pivot trails the player slightly, which reads as weight
    this.pivot.x += (target.x - this.pivot.x) * damp(14, dt);
    this.pivot.z += (target.z - this.pivot.z) * damp(14, dt);
    this.pivot.y += (target.y + PIVOT_H - this.pivot.y) * damp(9, dt);

    const free = this.clearDistance(this.goalDist);
    // pull in at once, ease back out
    this.dist += (free - this.dist) * damp(free < this.dist ? 30 : 3.5, dt);
    this.place();
  }

  private setDir(): void {
    const cp = Math.cos(this.pitch);
    this.dir.set(Math.sin(this.yaw) * cp, Math.sin(this.pitch), Math.cos(this.yaw) * cp);
  }

  /** How far the camera can back away from the pivot before it would enter a building. */
  private clearDistance(want: number): number {
    this.setDir();
    const step = 0.3;
    for (let t = 0.5; t <= want; t += step) {
      this.probe.copy(this.pivot).addScaledVector(this.dir, t);
      const roof = this.nav.roofAt(this.probe.x, this.probe.z);
      if (roof > 0 && this.probe.y < roof + 0.3) return Math.max(0.8, t - WALL_GAP);
    }
    return want;
  }

  private place(): void {
    this.setDir();
    this.camera.position.copy(this.pivot).addScaledVector(this.dir, this.dist);
    // stay above the ground behind the player (ramps, the bridge)
    const floor = this.nav.floorAt(this.camera.position.x, this.camera.position.z);
    if (Number.isFinite(floor) && this.nav.isWalkable(this.camera.position.x, this.camera.position.z)) {
      this.camera.position.y = Math.max(this.camera.position.y, floor + 0.5);
    }
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
