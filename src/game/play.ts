// Play mode: input -> player -> follow camera -> place labels. Owns the player for the whole session,
// so leaving for the map and coming back continues where the player stood.
import * as THREE from 'three';
import type { NavGrid } from './nav';
import { Player } from './player';
import { FollowCamera } from './followCamera';
import { InputController, type LookDelta, type MoveIntent } from './input';
import { placeAt, type Place } from './places';
import { SEE_THROUGH } from './paint';
import type { Hud } from './hud';

/** auto-follow resumes this long after the last camera drag (ms) */
const FOLLOW_DELAY = 1400;

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

  constructor(private readonly nav: NavGrid, private readonly camera: THREE.PerspectiveCamera, private readonly hud: Hud, surface: HTMLElement) {
    this.player = new Player(nav);
    this.follow = new FollowCamera(camera, nav);
    this.input = new InputController(surface, hud.joystick);
    this.input.enabled = false;
    this.spawnAtMarket();
  }

  /** Back to the square in front of the south gate, facing the market (Blender +Y = three -Z). */
  spawnAtMarket(): void {
    const [bx, by] = this.nav.meta.spawn;
    this.player.spawn(bx, -by, Math.PI);
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
    this.input.enabled = true;
    SEE_THROUGH.uSee.value = 1;
    this.place = null;
    this.hud.setPlace(null);
  }

  exit(): void {
    this.input.enabled = false;
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
    this.follow.forward(this.forward);
    this.player.update(dt, this.move, this.forward, clock);
    if (!driveCamera) return;

    const steering = now - this.input.lastLook < FOLLOW_DELAY;
    const followAmt = steering ? 0 : Math.max(0, this.move.y) * Math.min(1, this.player.speed / 2.5);
    this.follow.update(dt, this.player.position, this.player.heading, this.look, followAmt);
    SEE_THROUGH.uPivot.value.copy(this.follow.target);

    const p = placeAt(this.player.position.x, -this.player.position.z, this.place);
    if (p !== this.place) {
      this.place = p;
      this.hud.setPlace(p);
      if (p && p.kind === 'landmark' && !this.seen.has(p)) {
        this.seen.add(p);
        this.hud.toast(p);
      }
    }
    if (!this.hintHidden && this.input.firstMove !== null && now - this.input.firstMove > 6000) {
      this.hintHidden = true;
      this.hud.hideHint();
    }
  }
}
