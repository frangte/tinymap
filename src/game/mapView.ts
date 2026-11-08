// Map mode: orbit / zoom / pan over the whole diorama, with camera flights to each landmark.
// Also drives the slow idle orbit behind the start menu.
import * as THREE from 'three';
import { OrbitControls } from 'three/examples/jsm/controls/OrbitControls.js';
import { fromBlender } from './coords';

export interface MapStop {
  name: string;
  note: string;
  /** look-at point, Blender XYZ */
  t: [number, number, number];
  /** camera azimuth / elevation around the look-at point (degrees, Blender convention) and distance (m) */
  az: number;
  el: number;
  d: number;
}

export const MAP_STOPS: readonly MapStop[] = [
  { name: 'Toàn cảnh', note: 'Quận 1', t: [38, 10, 0], az: -112, el: 40, d: 270 },
  { name: 'Chợ Bến Thành', note: '1914', t: [8, 4, 6], az: -100, el: 26, d: 72 },
  { name: 'Nhà hát Thành phố', note: '1900', t: [104, -4, 6], az: 195, el: 18, d: 62 },
  { name: 'Nhà thờ Đức Bà', note: '1880', t: [78, 76, 10], az: -105, el: 16, d: 82 },
  { name: 'Bưu điện Thành phố', note: '1891', t: [98, 54, 6], az: 200, el: 20, d: 58 },
  { name: 'Hồ Con Rùa', note: '1967', t: [44, 90, 6], az: -120, el: 38, d: 54 },
  { name: 'Bitexco', note: '2010', t: [44, -62, 20], az: -130, el: 14, d: 100 },
  { name: 'Bãi biển', note: 'quanh đảo', t: [70, -70, 1], az: -40, el: 12, d: 70 },
];

/** the pose behind the start menu: a lower three-quarter view from the south-west, the market in front */
export const MENU_STOP: MapStop = { name: 'Sài Gòn Q1', note: 'Quận 1', t: [40, 12, 0], az: -118, el: 24, d: 250 };

const BOUND = new THREE.Box3().setFromPoints([fromBlender(-100, -120, -2), fromBlender(160, 150, 40)]);

interface Flight {
  t0: number;
  dur: number;
  fromT: THREE.Vector3;
  fromP: THREE.Vector3;
  toT: THREE.Vector3;
  toP: THREE.Vector3;
  done?: () => void;
}

const ease = (k: number): number => (k < 0.5 ? 4 * k * k * k : 1 - Math.pow(-2 * k + 2, 3) / 2);
const reducedMotion = (): boolean => window.matchMedia('(prefers-reduced-motion: reduce)').matches;

export function stopPose(s: MapStop): { target: THREE.Vector3; pos: THREE.Vector3 } {
  const target = fromBlender(s.t[0], s.t[1], s.t[2]);
  const az = THREE.MathUtils.degToRad(s.az);
  const el = THREE.MathUtils.degToRad(s.el);
  const dir = new THREE.Vector3(Math.cos(el) * Math.cos(az), Math.sin(el), -Math.cos(el) * Math.sin(az));
  return { target, pos: target.clone().addScaledVector(dir, s.d) };
}

export class MapView {
  readonly controls: OrbitControls;
  private fly: Flight | null = null;
  /** called when the user grabs the camera (so the HUD can clear the selected stop) */
  onUserMove: (() => void) | null = null;

  constructor(private readonly camera: THREE.PerspectiveCamera, dom: HTMLElement) {
    const c = new OrbitControls(camera, dom);
    c.enableDamping = true;
    c.dampingFactor = 0.08;
    c.minDistance = 10;
    c.maxDistance = 460;
    c.maxPolarAngle = 1.46;
    c.screenSpacePanning = false;
    c.zoomSpeed = 0.9;
    c.autoRotateSpeed = 0.35;
    c.enabled = false;
    c.addEventListener('start', () => {
      this.fly = null;
      this.onUserMove?.();
    });
    this.controls = c;
  }

  /** Let the user orbit / zoom / pan (map mode), or only animate (menu, transitions). */
  set interactive(on: boolean) {
    this.controls.enabled = on;
  }

  /** Slow orbit behind the start menu. */
  set idle(on: boolean) {
    this.controls.autoRotate = on;
  }

  /** Take over a camera that something else was driving, looking at `target`. */
  adopt(target: THREE.Vector3): void {
    this.controls.target.copy(target);
    this.controls.update();
  }

  flyTo(s: MapStop, instant = false, done?: () => void): void {
    const p = stopPose(s);
    if (instant || reducedMotion()) {
      this.fly = null;
      this.controls.target.copy(p.target);
      this.camera.position.copy(p.pos);
      this.controls.update();
      done?.();
      return;
    }
    this.fly = {
      t0: -1,
      dur: 1400,
      fromT: this.controls.target.clone(),
      fromP: this.camera.position.clone(),
      toT: p.target,
      toP: p.pos,
      done,
    };
  }

  get flying(): boolean {
    return this.fly !== null;
  }

  update(now: number): void {
    const f = this.fly;
    if (f) {
      if (f.t0 < 0) f.t0 = now;
      const k = Math.min(1, (now - f.t0) / f.dur);
      const e = ease(k);
      this.controls.target.lerpVectors(f.fromT, f.toT, e);
      this.camera.position.lerpVectors(f.fromP, f.toP, e);
      if (k >= 1) {
        this.fly = null;
        f.done?.();
      }
    }
    this.controls.update();
    this.controls.target.clamp(BOUND.min, BOUND.max);
  }
}

/**
 * A one-off camera flight between two arbitrary poses (map/menu <-> the follow camera).
 * Position and look-at point are blended separately, so the camera turns while it travels.
 */
export class CameraFlight {
  private f: Flight | null = null;
  private readonly look = new THREE.Vector3();

  constructor(private readonly camera: THREE.PerspectiveCamera) {}

  start(fromTarget: THREE.Vector3, toPos: THREE.Vector3, toTarget: THREE.Vector3, done: () => void, dur = 1500): void {
    if (reducedMotion()) {
      this.camera.position.copy(toPos);
      this.camera.lookAt(toTarget);
      done();
      return;
    }
    this.f = { t0: -1, dur, fromT: fromTarget.clone(), fromP: this.camera.position.clone(), toT: toTarget.clone(), toP: toPos.clone(), done };
  }

  get active(): boolean {
    return this.f !== null;
  }

  /** The point the camera is looking at right now (valid while active). */
  get lookAt(): THREE.Vector3 {
    return this.look;
  }

  cancel(): void {
    this.f = null;
  }

  update(now: number): void {
    const f = this.f;
    if (!f) return;
    if (f.t0 < 0) f.t0 = now;
    const k = Math.min(1, (now - f.t0) / f.dur);
    const e = ease(k);
    this.camera.position.lerpVectors(f.fromP, f.toP, e);
    // arc a little higher on the way, like a camera crane
    this.camera.position.y += Math.sin(Math.PI * e) * f.fromP.distanceTo(f.toP) * 0.08;
    this.look.lerpVectors(f.fromT, f.toT, e);
    this.camera.lookAt(this.look);
    if (k >= 1) {
      this.f = null;
      f.done?.();
    }
  }
}
