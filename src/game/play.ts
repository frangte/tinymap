// Play mode: input -> player -> follow camera -> place labels. Owns the player for the whole session,
// so leaving for the map and coming back continues where the player stood.
import * as THREE from 'three';
import type { NavGrid } from './nav';
import { Player } from './player';
import { FollowCamera, type Framing } from './followCamera';
import { InputController, type LookDelta, type MoveIntent } from './input';
import { placeAt, type Place } from './places';
import { FOLIAGE } from './paint';
import { damp, wrapAngle } from './coords';
import type { Hud } from './hud';
import type { Spawn } from './archetypes';

/** auto-follow resumes this long after the last camera drag (ms) */
const FOLLOW_DELAY = 1400;
/**
 * dressing: whole body beside the side panel (wide screens) or above the bottom sheet (phones). The phone framing
 * looks down from about 2.4 m at a point below the feet, which lifts the player clear of the sheet and keeps
 * low props the camera does not collide with (the roundabout decoration) under the frame.
 */
const FRAME_WIDE: Framing = { height: 0.9, dist: 4.4, pitch: 0.16 };
const FRAME_TALL: Framing = { height: -0.5, dist: 5.4, pitch: 0.55 };
/** the panel is a bottom sheet below this width (keep in sync with style.css) */
const SHEET_MAX_WIDTH = 560;
/** how fast the player turns round to face the camera while dressing (1/s) */
const FACE_RATE = 6;
/** spawn up to this far (m) from the market spawn point on each axis */
const SPAWN_SPREAD = 1.5;

export class PlayMode {
  readonly player: Player;
  readonly follow: FollowCamera;
  readonly input: InputController;
  private place: Place | null = null;
  private readonly seen = new Set<Place>();
  private readonly move: MoveIntent = { x: 0, y: 0, run: false };
  private readonly look: LookDelta = { yaw: 0, pitch: 0, zoom: 0 };
  private readonly forward = new THREE.Vector3();
  private readonly savePos = new THREE.Vector3();
  private readonly saveQuat = new THREE.Quaternion();
  private hintHidden = false;
  /** where spawn() puts the player: the archetype's start, or null for the market gate */
  private home: Spawn | null = null;
  /** a landmark seen for the first time this visit (the phone notes it) */
  onLandmark: (p: Place) => void = () => {};
  /** between enter() and exit() */
  private active = false;
  private dressing = false;

  constructor(private readonly nav: NavGrid, private readonly camera: THREE.PerspectiveCamera, private readonly hud: Hud, surface: HTMLElement) {
    this.player = new Player(nav);
    this.follow = new FollowCamera(camera, nav);
    this.input = new InputController(surface, hud.joystick);
    this.input.enabled = false;
    this.spawnAtHome();
  }

  /** Where spawnAtHome() goes from now on: the archetype's start, or null for the market gate. */
  setHome(home: Spawn | null): void {
    this.home = home;
  }

  /** Landmarks first seen this visit, in the order they were found. */
  get landmarksSeen(): ReadonlySet<Place> {
    return this.seen;
  }

  /**
   * Back to the start: the archetype's spawn point facing its `face` point, or (without one) the square in front of
   * the market's south gate, facing the market (Blender +Y = three -Z). A small random offset keeps players who
   * arrive together from standing inside each other; spawn() snaps it to a walkable cell.
   */
  spawnAtHome(): void {
    const jitter = (): number => (Math.random() - 0.5) * 2 * SPAWN_SPREAD;
    if (!this.home) {
      const [bx, by] = this.nav.meta.spawn;
      this.player.spawn(bx + jitter(), -by + jitter(), Math.PI);
      return;
    }
    const [bx, by] = this.home.at;
    const [fx, fy] = this.home.face;
    // three.js heading 0 faces +Z; Blender (x, y) is three (x, -y)
    this.player.spawn(bx + jitter(), -by + jitter(), Math.atan2(fx - bx, -(fy - by)));
  }

  /**
   * Where the follow camera will be once play starts: camera position and look-at point.
   * Computed by placing the camera and putting it back, so the follow camera's state is primed.
   */
  entryPose(outPos: THREE.Vector3, outTarget: THREE.Vector3): void {
    this.savePos.copy(this.camera.position);
    this.saveQuat.copy(this.camera.quaternion);
    this.follow.snapBehind(this.player.position, this.player.heading);
    outPos.copy(this.camera.position);
    outTarget.copy(this.follow.target);
    this.camera.position.copy(this.savePos);
    this.camera.quaternion.copy(this.saveQuat);
  }

  /** Start taking input; the camera should already sit at entryPose(). */
  enter(): void {
    this.follow.snapBehind(this.player.position, this.player.heading);
    this.active = true;
    this.input.enabled = !this.dressing;
    FOLIAGE.uSee.value = 1;
    this.place = null;
    this.hud.setPlace(null);
  }

  exit(): void {
    this.active = false;
    this.input.enabled = false;
  }

  /** While dressing the player stands still, turns to face the camera and the camera frames the whole body. */
  setDressing(on: boolean): void {
    this.dressing = on;
    this.input.enabled = this.active && !on;
    if (!on) this.follow.portrait = null;
  }

  /** Current look-at point of the follow camera. */
  get target(): THREE.Vector3 {
    return this.follow.target;
  }

  /**
   * @param driveCamera false while another controller (a transition) owns the camera: the player still idles
   */
  update(dt: number, now: number, clock: number, driveCamera: boolean): void {
    this.input.readMove(this.move);
    this.input.consumeLook(this.look);
    if (this.dressing) {
      // the model faces +Z rotated by heading; the camera sits along (sin yaw, cos yaw), so heading = yaw faces it
      const h = this.player.heading;
      this.player.heading = wrapAngle(h + wrapAngle(this.follow.yaw - h) * damp(FACE_RATE, dt));
      this.follow.portrait = window.innerWidth <= SHEET_MAX_WIDTH ? FRAME_TALL : FRAME_WIDE;
    }
    this.follow.forward(this.forward);
    this.player.update(dt, this.move, this.forward, clock);
    if (!driveCamera) return;

    const steering = now - this.input.lastLook < FOLLOW_DELAY;
    const followAmt = steering ? 0 : Math.max(0, this.move.y) * Math.min(1, this.player.speed / 2.5);
    this.follow.update(dt, this.player.position, this.player.heading, this.look, followAmt);
    FOLIAGE.uPivot.value.copy(this.follow.target);

    const p = placeAt(this.player.position.x, -this.player.position.z, this.place);
    if (p !== this.place) {
      this.place = p;
      this.hud.setPlace(p);
      // not while picking an archetype or dressing: the player has not walked anywhere yet
      if (p && p.kind === 'landmark' && !this.seen.has(p) && !this.dressing) {
        this.seen.add(p);
        this.hud.toast(p);
        this.onLandmark(p);
      }
    }
    if (!this.hintHidden && this.input.firstMove !== null && now - this.input.firstMove > 6000) {
      this.hintHidden = true;
      this.hud.hideHint();
    }
  }
}
