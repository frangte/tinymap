// Low-poly Saigon pedestrian built from primitives, dressed by a CharacterStyle (characters.ts): headwear,
// shorts or trousers, sleeves, an áo dài tunic, a backpack. One procedural idle / walk / run cycle driven by
// ground speed serves every outfit. The model faces +Z in its local frame.
import * as THREE from 'three';
import { paintMaterial } from './paint';
import { damp, smoothstep } from './coords';
import type { CharacterStyle } from './characters';

const PART_KEYS = ['skin', 'top', 'bottom', 'hat', 'hair', 'shoe', 'bag', 'eye'] as const;
type PartKey = (typeof PART_KEYS)[number];
type Materials = Record<PartKey, THREE.Material>;

const EYE = '#1E1612';

/** hip height above the soles; the thigh + shin + sole stack adds up to it */
const HIP_Y = 0.81;
const THIGH = 0.4;
const SHIN = 0.36;
const LEG = THIGH + SHIN;
/** head centre above the neck pivot */
const HEAD_Y = 0.19;

function materials(style: CharacterStyle): Materials {
  const colors: Record<PartKey, string> = {
    skin: style.skin,
    top: style.top,
    bottom: style.bottom,
    hat: style.hat,
    hair: style.hair,
    shoe: style.shoe,
    bag: style.bag,
    eye: EYE,
  };
  const out = {} as Materials;
  for (const key of PART_KEYS) {
    out[key] = paintMaterial(colors[key], `player-${key}`, { variation: 0.035, grime: 0, local: true });
  }
  return out;
}

/** A box whose top face sits on the origin, so rotating its parent swings it like a limb. */
function hanging(w: number, h: number, d: number, mat: THREE.Material): THREE.Mesh {
  const geo = new THREE.BoxGeometry(w, h, d);
  geo.translate(0, -h / 2, 0);
  return new THREE.Mesh(geo, mat);
}

function at<T extends THREE.Object3D>(o: T, x: number, y: number, z: number): T {
  o.position.set(x, y, z);
  return o;
}

function blobTexture(): THREE.Texture {
  const c = document.createElement('canvas');
  c.width = c.height = 64;
  const g = c.getContext('2d');
  if (g) {
    const gr = g.createRadialGradient(32, 32, 2, 32, 32, 31);
    gr.addColorStop(0, 'rgba(255,255,255,1)');
    gr.addColorStop(0.55, 'rgba(255,255,255,0.75)');
    gr.addColorStop(1, 'rgba(255,255,255,0)');
    g.fillStyle = gr;
    g.fillRect(0, 0, 64, 64);
  }
  return new THREE.CanvasTexture(c);
}

/** Soft contact shadow at the feet: the sun shadow map is baked once, so moving things get a blob. */
export function blobShadow(): THREE.Mesh {
  const blob = new THREE.Mesh(
    new THREE.CircleGeometry(0.42, 20),
    new THREE.MeshBasicMaterial({ map: blobTexture(), color: '#2A2338', transparent: true, opacity: 0.5, depthWrite: false, polygonOffset: true, polygonOffsetFactor: -2 }),
  );
  blob.rotation.x = -Math.PI / 2;
  blob.position.y = 0.025;
  blob.renderOrder = 1;
  return blob;
}

interface Side {
  hip: THREE.Group;
  knee: THREE.Group;
  shoulder: THREE.Group;
  elbow: THREE.Group;
}

export class Pedestrian {
  /** placed at the player's feet, rotated to the heading */
  readonly root = new THREE.Group();
  private readonly body = new THREE.Group();
  private readonly spine = new THREE.Group();
  private readonly neck = new THREE.Group();
  private readonly left: Side;
  private readonly right: Side;
  private readonly tunic: THREE.Group | null = null;
  private phase = 0;
  private walkAmt = 0;

  constructor(readonly style: CharacterStyle) {
    const m = materials(style);
    this.root.name = `Player:${style.id}`;
    this.root.add(this.body);

    this.body.add(at(new THREE.Mesh(new THREE.BoxGeometry(0.31, 0.15, 0.2), m.bottom), 0, HIP_Y - 0.02, 0));
    this.left = this.side(m, 1);
    this.right = this.side(m, -1);

    at(this.spine, 0, HIP_Y + 0.03, 0);
    this.body.add(this.spine);
    const torsoGeo = new THREE.CylinderGeometry(0.28, 0.22, 0.5, 4, 1);
    torsoGeo.rotateY(Math.PI / 4);
    torsoGeo.scale(1, 1, 0.62);
    torsoGeo.translate(0, 0.25, 0);
    this.spine.add(new THREE.Mesh(torsoGeo, m.top));
    if (style.backpack) this.spine.add(at(new THREE.Mesh(new THREE.BoxGeometry(0.27, 0.3, 0.11), m.bag), 0, 0.27, -0.16));
    if (style.tunic) this.tunic = this.tunicPanels(m);

    at(this.neck, 0, 0.5, 0);
    this.spine.add(this.neck);
    this.neck.add(at(new THREE.Mesh(new THREE.CylinderGeometry(0.055, 0.065, 0.09, 6), m.skin), 0, 0.03, 0));
    this.neck.add(at(new THREE.Mesh(new THREE.IcosahedronGeometry(0.15, 1), m.skin), 0, HEAD_Y, 0.01));
    for (const sx of [1, -1]) this.neck.add(at(new THREE.Mesh(new THREE.BoxGeometry(0.028, 0.034, 0.02), m.eye), sx * 0.052, 0.2, 0.148));
    this.headwear(m);
  }

  /** One leg + one arm. `sx` = +1 for the character's left (+X), -1 for its right. */
  private side(m: Materials, sx: number): Side {
    const { trousers, longSleeves } = this.style;
    const hip = at(new THREE.Group(), sx * 0.085, HIP_Y, 0);
    hip.add(hanging(0.15, THIGH, 0.16, m.bottom));
    const knee = at(new THREE.Group(), 0, -THIGH, 0);
    hip.add(knee);
    knee.add(hanging(trousers ? 0.125 : 0.105, SHIN, trousers ? 0.13 : 0.11, trousers ? m.bottom : m.skin));
    knee.add(at(new THREE.Mesh(new THREE.BoxGeometry(0.11, 0.05, 0.23), m.shoe), 0, -SHIN - 0.025, 0.045));
    this.body.add(hip);

    const shoulder = at(new THREE.Group(), sx * 0.225, 0.455, 0);
    shoulder.add(at(hanging(0.13, 0.15, 0.13, m.top), 0, 0.03, 0));
    shoulder.add(hanging(longSleeves ? 0.1 : 0.082, 0.27, longSleeves ? 0.1 : 0.085, longSleeves ? m.top : m.skin));
    const elbow = at(new THREE.Group(), 0, -0.27, 0);
    shoulder.add(elbow);
    elbow.add(hanging(longSleeves ? 0.095 : 0.078, 0.23, longSleeves ? 0.095 : 0.08, longSleeves ? m.top : m.skin));
    elbow.add(at(new THREE.Mesh(new THREE.BoxGeometry(0.085, 0.085, 0.085), m.skin), 0, -0.27, 0));
    this.spine.add(shoulder);
    return { hip, knee, shoulder, elbow };
  }

  /** Áo dài: two slim panels hanging from the waist, front and back, that trail a little when walking. */
  private tunicPanels(m: Materials): THREE.Group {
    const g = at(new THREE.Group(), 0, 0.02, 0);
    g.add(at(hanging(0.27, 0.56, 0.035, m.top), 0, 0, 0.1));
    g.add(at(hanging(0.27, 0.56, 0.035, m.top), 0, 0, -0.1));
    this.spine.add(g);
    return g;
  }

  private headwear(m: Materials): void {
    const n = this.neck;
    switch (this.style.headwear) {
      case 'nonla':
        // a shallow cone sitting low on the head
        n.add(at(new THREE.Mesh(new THREE.ConeGeometry(0.34, 0.17, 12), m.hat), 0, 0.355, 0.01));
        break;
      case 'bun':
        // hair cap stops above the brow, a bun at the back
        n.add(at(new THREE.Mesh(new THREE.SphereGeometry(0.162, 8, 5, 0, Math.PI * 2, 0, Math.PI * 0.42), m.hair), 0, HEAD_Y + 0.02, -0.015));
        n.add(at(new THREE.Mesh(new THREE.IcosahedronGeometry(0.075, 1), m.hair), 0, HEAD_Y + 0.08, -0.14));
        break;
      case 'helmet':
        n.add(at(new THREE.Mesh(new THREE.SphereGeometry(0.175, 10, 6, 0, Math.PI * 2, 0, Math.PI * 0.5), m.hat), 0, HEAD_Y + 0.005, 0.01));
        n.add(at(new THREE.Mesh(new THREE.BoxGeometry(0.22, 0.03, 0.1), m.hat), 0, HEAD_Y + 0.01, 0.17));
        break;
      case 'bucket':
        n.add(at(new THREE.Mesh(new THREE.CylinderGeometry(0.145, 0.155, 0.13, 10), m.hat), 0, HEAD_Y + 0.11, 0.005));
        n.add(at(new THREE.Mesh(new THREE.CylinderGeometry(0.25, 0.22, 0.03, 12), m.hat), 0, HEAD_Y + 0.055, 0.005));
        break;
    }
  }

  /** Carry the gait over from the pedestrian this one replaces, so a mid-stride swap doesn't snap the pose. */
  copyGait(from: Pedestrian): void {
    this.phase = from.phase;
    this.walkAmt = from.walkAmt;
  }

  /** Free the GPU resources of this outfit; the materials are shared between its meshes, so each goes once. */
  dispose(): void {
    const done = new Set<THREE.Material>();
    this.root.traverse((o) => {
      if (!(o instanceof THREE.Mesh)) return;
      o.geometry.dispose();
      const mats = Array.isArray(o.material) ? o.material : [o.material];
      for (const mat of mats) {
        if (done.has(mat)) continue;
        done.add(mat);
        mat.dispose();
      }
    });
  }

  /** Advance the gait. `speed` is the actual ground speed in m/s, `time` seconds since start. */
  update(dt: number, speed: number, time: number): void {
    this.walkAmt += (smoothstep(0.12, 1.3, speed) - this.walkAmt) * damp(10, dt);
    const walk = this.walkAmt;
    const run = smoothstep(3.6, 6.2, speed);
    const stride = 1.35 + 0.95 * run; // metres per full cycle (two steps)
    this.phase = (this.phase + ((speed * dt) / stride) * Math.PI * 2) % (Math.PI * 2);
    const s = Math.sin(this.phase);
    const c = Math.cos(this.phase);
    const swing = s * (0.42 + 0.36 * run) * walk;
    const idle = 1 - walk;
    const breathe = Math.sin(time * 2.1);

    // legs: negative x-rotation swings a hanging limb forward (+Z); the knee folds while the leg recovers
    this.left.hip.rotation.x = -swing;
    this.right.hip.rotation.x = swing;
    this.left.knee.rotation.x = walk * (0.05 + (0.55 + 0.55 * run) * Math.max(0, -c));
    this.right.knee.rotation.x = walk * (0.05 + (0.55 + 0.55 * run) * Math.max(0, c));
    // arms counter-swing; elbows bend more when running
    const arm = swing * (0.8 - 0.15 * run);
    this.left.shoulder.rotation.x = arm;
    this.right.shoulder.rotation.x = -arm;
    const out = 0.07 + 0.1 * run + idle * 0.015 * breathe;
    this.left.shoulder.rotation.z = out;
    this.right.shoulder.rotation.z = -out;
    const elbow = -(0.12 + 0.2 * walk + 1.05 * run);
    this.left.elbow.rotation.x = elbow;
    this.right.elbow.rotation.x = elbow;
    // the tunic trails behind with speed and flutters a little
    if (this.tunic) this.tunic.rotation.x = 0.25 * walk + 0.45 * run + 0.04 * walk * Math.sin(this.phase * 2);

    // keep the planted foot on the ground; a little flight phase when running
    this.body.position.y = -LEG * (1 - Math.cos(swing)) + run * 0.045 * Math.abs(c);
    this.spine.rotation.x = 0.04 * walk + 0.2 * run;
    this.spine.rotation.y = s * 0.1 * walk;
    this.spine.position.y = HIP_Y + 0.03 + idle * 0.006 * breathe;
    // idle: look around a little
    this.neck.rotation.y = idle * 0.32 * Math.sin(time * 0.47) * Math.sin(time * 0.19 + 1.3);
    this.neck.rotation.x = -0.05 * run;
  }
}
