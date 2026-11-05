// Low-poly Saigon pedestrian built from primitives, with a procedural idle / walk / run cycle driven by ground speed.
// Every optional part (long hair, skirt, áo dài panels, backpack, both hats) is built once; `setLook` only toggles
// visibility, swaps a limb between the skin and the cloth material and recolours the cloth, so nothing is rebuilt
// and the ink pass (which re-reads `mesh.material` every frame) keeps working. The model faces +Z in its local frame.
import * as THREE from 'three';
import { paintMaterial } from './paint';
import { damp, smoothstep } from './coords';
import { DEFAULT_LOOK, outfitStyle, type Look } from './outfits';

/** fixed colours; the cloth slots are recoloured from the outfit */
const BASE_COLORS = {
  skin: '#C68D66',
  hair: '#2A211E',
  eye: '#1E1612',
  straw: '#E0C68C',
  top: '#B5523B',
  bottom: '#38465C',
  shoe: '#2B211C',
  bag: '#4F8F88',
  cap: '#4F8F88',
} as const;
type Slot = keyof typeof BASE_COLORS;

/** hip height above the soles; the thigh + shin + sole stack adds up to it */
const HIP_Y = 0.81;
const THIGH = 0.4;
const SHIN = 0.36;
const LEG = THIGH + SHIN;

/** per-body proportions: shoulder offset, torso width and overall scale */
const FRAME = {
  man: { shoulderX: 0.225, torsoX: 1, scale: 1 },
  woman: { shoulderX: 0.2, torsoX: 0.86, scale: 0.95 },
} as const;
const SKIRT_DEPTH = 0.78;
const FLAP_REST = 0.05;

function materials(): Record<Slot, THREE.MeshToonMaterial> {
  const out = {} as Record<Slot, THREE.MeshToonMaterial>;
  for (const key of Object.keys(BASE_COLORS) as Slot[]) {
    out[key] = paintMaterial(BASE_COLORS[key], `player-${key}`, { variation: 0.035, grime: 0, local: true });
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

let blobTex: THREE.Texture | null = null;
/** soft round contact shadow, shared by every pedestrian */
function blobTexture(): THREE.Texture {
  if (blobTex) return blobTex;
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
  blobTex = new THREE.CanvasTexture(c);
  return blobTex;
}

interface Side {
  hip: THREE.Group;
  knee: THREE.Group;
  shoulder: THREE.Group;
  elbow: THREE.Group;
  thigh: THREE.Mesh;
  shin: THREE.Mesh;
  upperArm: THREE.Mesh;
  forearm: THREE.Mesh;
}

export class Pedestrian {
  /** placed at the feet, rotated to the heading */
  readonly root = new THREE.Group();
  private readonly body = new THREE.Group();
  private readonly spine = new THREE.Group();
  private readonly neck = new THREE.Group();
  private readonly left: Side;
  private readonly right: Side;
  private readonly mat = materials();
  private readonly torso: THREE.Mesh;
  private readonly bag: THREE.Mesh;
  private readonly skirt: THREE.Mesh;
  private readonly flapFront = new THREE.Group();
  private readonly flapBack = new THREE.Group();
  private readonly longHair: THREE.Mesh;
  private readonly nonLa: THREE.Mesh;
  private readonly cap = new THREE.Group();
  private current: Look = { ...DEFAULT_LOOK };
  private phase = 0;
  private walkAmt = 0;

  constructor(look: Look = DEFAULT_LOOK) {
    const m = this.mat;
    this.root.add(this.body);

    this.body.add(at(new THREE.Mesh(new THREE.BoxGeometry(0.31, 0.15, 0.2), m.bottom), 0, HIP_Y - 0.02, 0));
    this.left = this.side(1);
    this.right = this.side(-1);

    // skirt: a flared 8-sided tube from the waist to just above the knee
    const skirtGeo = new THREE.CylinderGeometry(0.19, 0.27, 0.36, 8, 1, true);
    skirtGeo.translate(0, -0.18, 0);
    this.skirt = at(new THREE.Mesh(skirtGeo, m.bottom), 0, HIP_Y + 0.05, 0);
    this.body.add(this.skirt);
    // áo dài: front and back panels hinged at the waist, pushed out by the thighs while walking
    // (hinged inside the torso and in front of the pelvis, so no band of trousers shows at the waist)
    this.flapFront.add(hanging(0.25, 0.7, 0.022, m.top));
    this.flapBack.add(hanging(0.26, 0.68, 0.022, m.top));
    this.body.add(at(this.flapFront, 0, HIP_Y + 0.1, 0.115), at(this.flapBack, 0, HIP_Y + 0.1, -0.115));

    at(this.spine, 0, HIP_Y + 0.03, 0);
    this.body.add(this.spine);
    const torsoGeo = new THREE.CylinderGeometry(0.28, 0.22, 0.5, 4, 1);
    torsoGeo.rotateY(Math.PI / 4);
    torsoGeo.scale(1, 1, 0.62);
    torsoGeo.translate(0, 0.25, 0);
    this.torso = new THREE.Mesh(torsoGeo, m.top);
    this.spine.add(this.torso);
    this.bag = at(new THREE.Mesh(new THREE.BoxGeometry(0.27, 0.3, 0.11), m.bag), 0, 0.27, -0.16);
    this.spine.add(this.bag);

    at(this.neck, 0, 0.5, 0);
    this.spine.add(this.neck);
    this.neck.add(at(new THREE.Mesh(new THREE.CylinderGeometry(0.055, 0.065, 0.09, 6), m.skin), 0, 0.03, 0));
    this.neck.add(at(new THREE.Mesh(new THREE.IcosahedronGeometry(0.15, 1), m.skin), 0, 0.19, 0.01));
    for (const sx of [1, -1]) this.neck.add(at(new THREE.Mesh(new THREE.BoxGeometry(0.028, 0.034, 0.02), m.eye), sx * 0.052, 0.2, 0.148));
    // hair: a squashed cap over the crown and the back of the head; women add a long panel down the back
    const hairGeo = new THREE.IcosahedronGeometry(0.158, 1);
    hairGeo.scale(1.02, 0.78, 1.02);
    this.neck.add(at(new THREE.Mesh(hairGeo, m.hair), 0, 0.235, -0.018));
    this.longHair = at(hanging(0.25, 0.34, 0.07, m.hair), 0, 0.27, -0.112);
    this.longHair.rotation.x = 0.1;
    this.neck.add(this.longHair);
    // nón lá: a shallow cone sitting low on the head
    this.nonLa = at(new THREE.Mesh(new THREE.ConeGeometry(0.34, 0.17, 12), m.straw), 0, 0.355, 0.01);
    this.neck.add(this.nonLa);
    // nón kết: rounded crown and a forward brim
    this.cap.add(at(new THREE.Mesh(new THREE.CylinderGeometry(0.15, 0.168, 0.1, 10), m.cap), 0, 0.315, -0.005));
    this.cap.add(at(new THREE.Mesh(new THREE.BoxGeometry(0.2, 0.018, 0.13), m.cap), 0, 0.272, 0.16));
    this.neck.add(this.cap);

    // blob shadow: the sun shadow map is baked once, so pedestrians get a soft contact shadow instead
    const blob = new THREE.Mesh(
      new THREE.CircleGeometry(0.42, 20),
      new THREE.MeshBasicMaterial({ map: blobTexture(), color: '#2A2338', transparent: true, opacity: 0.5, depthWrite: false, polygonOffset: true, polygonOffsetFactor: -2 }),
    );
    blob.rotation.x = -Math.PI / 2;
    blob.position.y = 0.025;
    blob.renderOrder = 1;
    this.root.add(blob);

    this.setLook(look);
  }

  get look(): Readonly<Look> {
    return this.current;
  }

  /** Dress the pedestrian: body proportions, outfit colours and cut, hat. */
  setLook(look: Look): void {
    this.current = { ...look };
    const s = outfitStyle(look.outfit, look.body);
    const m = this.mat;
    m.top.color.set(s.top);
    m.bottom.color.set(s.bottom);
    m.shoe.color.set(s.shoe);
    m.cap.color.set(s.accent);
    if (s.bag) m.bag.color.set(s.bag);

    const legSkin = s.legs === 'skirt';
    const shinSkin = s.legs !== 'pants';
    const armSkin = s.sleeves === 'short';
    const frame = FRAME[look.body];
    for (const side of [this.left, this.right]) {
      side.thigh.material = legSkin ? m.skin : m.bottom;
      side.shin.material = shinSkin ? m.skin : m.bottom;
      side.upperArm.material = armSkin ? m.skin : m.top;
      side.forearm.material = armSkin ? m.skin : m.top;
    }
    this.left.shoulder.position.x = frame.shoulderX;
    this.right.shoulder.position.x = -frame.shoulderX;
    this.torso.scale.x = frame.torsoX;
    this.body.scale.setScalar(frame.scale);

    this.bag.visible = s.bag !== null;
    this.skirt.visible = s.legs === 'skirt';
    this.flapFront.visible = this.flapBack.visible = s.flaps;
    this.longHair.visible = look.body === 'woman';
    this.nonLa.visible = look.hat === 'nonla';
    this.cap.visible = look.hat === 'cap';
  }

  /** Free the GPU resources of a pedestrian that leaves the scene (the shared blob texture stays). */
  dispose(): void {
    this.root.traverse((o) => {
      if (!(o instanceof THREE.Mesh)) return;
      o.geometry.dispose();
      if (o.material instanceof THREE.MeshBasicMaterial) o.material.dispose();
    });
    for (const mat of Object.values(this.mat)) mat.dispose();
  }

  /** One leg + one arm. `sx` = +1 for the character's left (+X), -1 for its right. */
  private side(sx: number): Side {
    const m = this.mat;
    const hip = at(new THREE.Group(), sx * 0.085, HIP_Y, 0);
    const thigh = hanging(0.15, THIGH, 0.16, m.bottom);
    hip.add(thigh);
    const knee = at(new THREE.Group(), 0, -THIGH, 0);
    hip.add(knee);
    const shin = hanging(0.105, SHIN, 0.11, m.skin);
    knee.add(shin);
    knee.add(at(new THREE.Mesh(new THREE.BoxGeometry(0.11, 0.05, 0.23), m.shoe), 0, -SHIN - 0.025, 0.045));
    this.body.add(hip);

    const shoulder = at(new THREE.Group(), sx * FRAME.man.shoulderX, 0.455, 0);
    shoulder.add(at(hanging(0.13, 0.15, 0.13, m.top), 0, 0.03, 0));
    const upperArm = hanging(0.082, 0.27, 0.085, m.skin);
    shoulder.add(upperArm);
    const elbow = at(new THREE.Group(), 0, -0.27, 0);
    shoulder.add(elbow);
    const forearm = hanging(0.078, 0.23, 0.08, m.skin);
    elbow.add(forearm);
    elbow.add(at(new THREE.Mesh(new THREE.BoxGeometry(0.085, 0.085, 0.085), m.skin), 0, -0.27, 0));
    this.spine.add(shoulder);
    return { hip, knee, shoulder, elbow, thigh, shin, upperArm, forearm };
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

    // cloth: the leading thigh pushes the front panel out and the skirt hem forward, the trailing one the back
    const spread = Math.abs(swing);
    const flutter = 0.03 * walk * Math.sin(time * 9.0);
    this.flapFront.rotation.x = -(FLAP_REST + spread * 1.05 + 0.25 * run) + flutter;
    this.flapBack.rotation.x = FLAP_REST + spread * 0.8 + 0.35 * run - flutter;
    this.skirt.scale.z = SKIRT_DEPTH + spread * 0.55;

    // keep the planted foot on the ground; a little flight phase when running
    this.body.position.y = (-LEG * (1 - Math.cos(swing)) + run * 0.045 * Math.abs(c)) * this.body.scale.y;
    this.spine.rotation.x = 0.04 * walk + 0.2 * run;
    this.spine.rotation.y = s * 0.1 * walk;
    this.spine.position.y = HIP_Y + 0.03 + idle * 0.006 * breathe;
    // idle: look around a little
    this.neck.rotation.y = idle * 0.32 * Math.sin(time * 0.47) * Math.sin(time * 0.19 + 1.3);
    this.neck.rotation.x = -0.05 * run;
  }
}
