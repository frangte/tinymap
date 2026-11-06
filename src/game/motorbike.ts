// The xe số 110: a low-poly step-through motorbike built from primitives in the diorama style, with the yellow
// Crab Bike top box (docs/design/05-frontend.md, "Crab Bike assets"). The model faces +Z with its wheels on the
// ground at the origin; the rider sits over SEAT_Z (src/game/character.ts, ridePose). Wheels spin with the
// speed, the front turns with the steering and the whole bike leans into turns about its contact line.
import * as THREE from 'three';
import { paintMaterial } from './paint';
import { CRAB_YELLOW } from './outfits';

const WHEEL_R = 0.27;
const AXLE_Z = 0.62;
/** where the headstock is: the front end turns about a vertical axis through it */
const HEAD_Z = 0.46;
const HEAD_Y = 0.96;

const COLORS = {
  body: '#3A4A66',
  dark: '#2B211C',
  metal: '#6E6A66',
  tire: '#1F1A18',
  light: '#F3E6B0',
  tail: '#B5523B',
  box: CRAB_YELLOW,
} as const;
type Part = keyof typeof COLORS;

let shared: Record<Part, THREE.MeshToonMaterial> | null = null;
/** every bike looks the same, so they share their materials (never disposed) */
function materials(): Record<Part, THREE.MeshToonMaterial> {
  if (shared) return shared;
  const out = {} as Record<Part, THREE.MeshToonMaterial>;
  for (const key of Object.keys(COLORS) as Part[]) {
    out[key] = paintMaterial(COLORS[key], `bike-${key}`, { variation: 0.03, grime: 0, local: true });
  }
  shared = out;
  return out;
}

let shadowTex: THREE.Texture | null = null;
function shadowTexture(): THREE.Texture {
  if (shadowTex) return shadowTex;
  const c = document.createElement('canvas');
  c.width = c.height = 64;
  const g = c.getContext('2d');
  if (g) {
    const gr = g.createRadialGradient(32, 32, 2, 32, 32, 31);
    gr.addColorStop(0, 'rgba(255,255,255,1)');
    gr.addColorStop(0.6, 'rgba(255,255,255,0.7)');
    gr.addColorStop(1, 'rgba(255,255,255,0)');
    g.fillStyle = gr;
    g.fillRect(0, 0, 64, 64);
  }
  shadowTex = new THREE.CanvasTexture(c);
  return shadowTex;
}

function box(w: number, h: number, d: number, mat: THREE.Material, x: number, y: number, z: number): THREE.Mesh {
  const m = new THREE.Mesh(new THREE.BoxGeometry(w, h, d), mat);
  m.position.set(x, y, z);
  return m;
}

/** A wheel: tyre, hub and three spokes so its turning shows; spins about its local X. */
function wheel(m: Record<Part, THREE.MeshToonMaterial>): THREE.Group {
  const g = new THREE.Group();
  const tyre = new THREE.Mesh(new THREE.CylinderGeometry(WHEEL_R, WHEEL_R, 0.09, 14), m.tire);
  tyre.rotation.z = Math.PI / 2;
  g.add(tyre);
  const hub = new THREE.Mesh(new THREE.CylinderGeometry(0.07, 0.07, 0.1, 8), m.metal);
  hub.rotation.z = Math.PI / 2;
  g.add(hub);
  for (let k = 0; k < 3; k++) {
    const spoke = box(0.02, WHEEL_R * 1.8, 0.03, m.metal, 0, 0, 0);
    spoke.rotation.x = (k * Math.PI) / 3;
    g.add(spoke);
  }
  return g;
}

export class Motorbike {
  /** placed on the ground, rotated to the heading, rolled by the lean */
  readonly root = new THREE.Group();
  private readonly front = new THREE.Group();
  private readonly frontWheel: THREE.Group;
  private readonly rearWheel: THREE.Group;
  private readonly geometries: THREE.BufferGeometry[] = [];
  private readonly blobMat: THREE.MeshBasicMaterial;

  constructor() {
    const m = materials();
    const frame = new THREE.Group();
    this.root.add(frame);

    this.rearWheel = wheel(m);
    this.rearWheel.position.set(0, WHEEL_R, -AXLE_Z);
    frame.add(this.rearWheel);

    // body: the cowl under the seat, the seat, the leg shield and the spine tube between them
    frame.add(box(0.3, 0.26, 0.72, m.body, 0, 0.58, -0.3));
    frame.add(box(0.29, 0.07, 0.62, m.dark, 0, 0.745, -0.26));
    const shield = box(0.36, 0.44, 0.06, m.body, 0, 0.64, 0.32);
    shield.rotation.x = -0.28;
    frame.add(shield);
    frame.add(box(0.09, 0.09, 0.62, m.metal, 0, 0.4, 0.02));
    for (const sx of [1, -1]) frame.add(box(0.12, 0.03, 0.06, m.dark, sx * 0.18, 0.3, -0.05));
    // rear: fender, tail light, exhaust on the right, and the yellow top box with the Crab Bike mark
    frame.add(box(0.15, 0.04, 0.42, m.body, 0, 0.6, -0.68));
    frame.add(box(0.12, 0.05, 0.03, m.tail, 0, 0.66, -0.74));
    const pipe = new THREE.Mesh(new THREE.CylinderGeometry(0.04, 0.045, 0.52, 8), m.metal);
    pipe.rotation.x = Math.PI / 2;
    pipe.position.set(-0.17, 0.3, -0.42);
    frame.add(pipe);
    frame.add(box(0.38, 0.3, 0.36, m.box, 0, 0.93, -0.6));
    frame.add(box(0.385, 0.03, 0.365, m.dark, 0, 1.02, -0.6));
    frame.add(box(0.14, 0.09, 0.01, m.dark, 0, 0.9, -0.785));

    // the front turns: fork, front wheel, headlight cowl and handlebar
    this.front.position.set(0, HEAD_Y, HEAD_Z);
    frame.add(this.front);
    for (const sx of [1, -1]) {
      const fork = box(0.035, 0.74, 0.035, m.metal, sx * 0.065, -0.35, 0.08);
      fork.rotation.x = -0.28;
      this.front.add(fork);
    }
    this.frontWheel = wheel(m);
    this.frontWheel.position.set(0, WHEEL_R - HEAD_Y, AXLE_Z - HEAD_Z);
    this.front.add(this.frontWheel);
    this.front.add(box(0.15, 0.04, 0.34, m.body, 0, WHEEL_R - HEAD_Y + 0.31, AXLE_Z - HEAD_Z));
    this.front.add(box(0.25, 0.15, 0.17, m.body, 0, 0.05, 0.03));
    this.front.add(box(0.13, 0.08, 0.02, m.light, 0, 0.05, 0.125));
    this.front.add(box(0.64, 0.03, 0.03, m.dark, 0, 0.09, -0.05));
    for (const sx of [1, -1]) this.front.add(box(0.03, 0.18, 0.02, m.metal, sx * 0.22, 0.2, -0.04));

    // blob shadow: the sun shadow map is baked once, so moving things bring their own
    this.blobMat = new THREE.MeshBasicMaterial({
      map: shadowTexture(), color: '#2A2338', transparent: true, opacity: 0.5, depthWrite: false, polygonOffset: true, polygonOffsetFactor: -2,
    });
    const blob = new THREE.Mesh(new THREE.CircleGeometry(0.5, 20), this.blobMat);
    blob.rotation.x = -Math.PI / 2;
    blob.scale.set(0.8, 1.7, 1);
    blob.position.y = 0.025;
    blob.renderOrder = 1;
    this.root.add(blob);

    this.root.traverse((o) => {
      if (o instanceof THREE.Mesh) this.geometries.push(o.geometry);
    });
  }

  /**
   * @param speed forward ground speed (m/s), spins the wheels
   * @param steer how far the front is turned (rad, + to the left)
   * @param lean roll into the turn (rad, + to the right)
   */
  update(dt: number, speed: number, steer: number, lean: number): void {
    const spin = (speed * dt) / WHEEL_R;
    this.frontWheel.rotation.x += spin;
    this.rearWheel.rotation.x += spin;
    this.front.rotation.y = steer;
    this.root.rotation.z = lean;
  }

  /** Free what this bike owns (the materials and textures are shared). */
  dispose(): void {
    for (const g of this.geometries) g.dispose();
    this.blobMat.dispose();
  }
}
