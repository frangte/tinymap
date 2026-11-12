// Map mode: orbit / zoom / pan over the whole diorama, with camera flights to each landmark.
// Also drives the slow idle orbit behind the start menu.
import * as THREE from 'three';
import { OrbitControls } from 'three/examples/jsm/controls/OrbitControls.js';
import { fromBlender } from './coords';

export interface MapStop {
  /** the place it flies to (server/content/places.json), whose name it shows; null for the whole island */
  place: string | null;
  /** the year on its chip; null shows the place's note */
  year: string | null;
  /** look-at point, Blender XYZ */
  t: [number, number, number];
  /** camera azimuth / elevation around the look-at point (degrees, Blender convention) and distance (m) */
  az: number;
  el: number;
  d: number;
}

// Sài Gòn mini (layout v8, scripts/map/saigon_mini_layout.json): most stops look down the street that ends on the
// landmark (Lê Lợi on the Opera House, Đồng Khởi on the cathedral, Phạm Ngọc Thạch on Hồ Con Rùa)
export const MAP_STOPS: readonly MapStop[] = [
  { place: null, year: null, t: [-9, -23, 0], az: -112, el: 40, d: 500 },
  { place: 'cho_ben_thanh', year: '1914', t: [-48, 25, 5], az: -15, el: 22, d: 72 },
  { place: 'nha_hat_thanh_pho', year: '1900', t: [-11, 144, 7], az: -90, el: 16, d: 72 },
  { place: 'nha_tho_duc_ba', year: '1880', t: [-143, 127, 11], az: 0, el: 14, d: 86 },
  { place: 'buu_dien_thanh_pho', year: '1891', t: [-79, 146, 6], az: -75, el: 20, d: 60 },
  { place: 'ho_con_rua', year: '1967', t: [-111, 165, 3], az: -90, el: 30, d: 56 },
  { place: 'bitexco', year: '2010', t: [12, 57, 22], az: -15, el: 12, d: 112 },
  { place: 'bai_bien', year: null, t: [75, -152, 3], az: -112, el: 14, d: 125 },
];

/** the pose behind the start menu: a lower three-quarter view from the south-west over the city to the river */
export const MENU_STOP: MapStop = { place: null, year: null, t: [-10, 15, 0], az: -118, el: 24, d: 330 };

const BOUND = new THREE.Box3().setFromPoints([fromBlender(-250, -235, -2), fromBlender(250, 230, 80)]);

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
    c.maxDistance = 720;
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
