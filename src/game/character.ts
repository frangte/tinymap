// Low-poly Saigon pedestrian built from primitives, with a procedural idle / walk / run cycle driven by ground speed,
// and poses blended over it: in the air, riding a motorbike, and the emotes (server/content/emotes.json).
// Every optional part (long hair, skirt, áo dài panels, backpack, every hat) is built once; `setLook` only toggles
// visibility, swaps a limb between the skin and the cloth material and recolours the cloth, so nothing is rebuilt
// and the ink pass (which re-reads `mesh.material` every frame) keeps working. The model faces +Z in its local frame.
import * as THREE from 'three';
import { paintMaterial } from './paint';
import { damp, smoothstep } from './coords';
import { CRAB_YELLOW, DEFAULT_LOOK, outfitStyle, type Look } from './outfits';

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
  bucket: '#D7C7A2',
  helmet: CRAB_YELLOW,
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
/** riding: the hips sit this far below standing hip height, and this far behind the bike's middle (m) */
const SEAT_DROP = -0.04;
const BLOB_Y = 0.025;
const BLOB_OPACITY = 0.5;
const SEAT_BACK = -0.2;
/** a passenger in seat 1 sits this far behind the bike's middle, and this much higher than the rider (m) */
const PILLION_BACK = -0.6;
const PILLION_RISE = 0.06;

/** What the body does on top of walking; the state flags of a snapshot row say which. */
export interface AvatarPose {
  air: boolean;
  ride: boolean;
  /** on the bike as the passenger, behind the rider (with ride) */
  pillion: boolean;
  /** emote id (server/content/emotes.json), '' for none */
  emote: string;
  /** seconds since the emote started */
  emoteTime: number;
}

export function idlePose(): AvatarPose {
  return { air: false, ride: false, pillion: false, emote: '', emoteTime: 0 };
}

const IDLE: Readonly<AvatarPose> = idlePose();

const mix = (a: number, b: number, w: number): number => a + (b - a) * w;

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
  /** the cap of the sleeve over the shoulder joint */
  pad: THREE.Mesh;
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
  /** bikini top: a band around the chest */
  private readonly bra: THREE.Mesh;
  private readonly bag: THREE.Mesh;
  private readonly skirt: THREE.Mesh;
  private readonly flapFront = new THREE.Group();
  private readonly flapBack = new THREE.Group();
  private readonly longHair: THREE.Mesh;
  private readonly nonLa: THREE.Mesh;
  private readonly cap = new THREE.Group();
  private readonly bucket = new THREE.Group();
  private readonly helmet = new THREE.Group();
  private readonly bun: THREE.Mesh;
  /** a phone in the right hand, for the photo emote */
  private readonly phone: THREE.Mesh;
  /** the contact shadow, kept on the ground in a jump */
  private readonly blob: THREE.Mesh;
  private readonly blobMat: THREE.MeshBasicMaterial;
  private current: Look = { ...DEFAULT_LOOK };
  private phase = 0;
  private walkAmt = 0;
  /** how far each overlay pose is blended in (0…1) */
  private airW = 0;
  private rideW = 0;
  private emoteW = 0;
  /** the emote being shown, kept while it fades out */
  private emote = '';

  constructor(look: Look = DEFAULT_LOOK) {
    const m = this.mat;
    this.root.add(this.body);

    this.body.add(at(new THREE.Mesh(new THREE.BoxGeometry(0.31, 0.15, 0.2), m.bottom), 0, HIP_Y - 0.02, 0));
    this.left = this.side(1);
    this.right = this.side(-1);
    this.phone = at(new THREE.Mesh(new THREE.BoxGeometry(0.065, 0.12, 0.014), m.eye), 0, -0.27, 0.05);
    this.phone.visible = false;
    this.right.elbow.add(this.phone);

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
    // bikini top: a slice of the torso shape, a hair wider, across the chest (a child, so it narrows with the torso)
    const braGeo = new THREE.CylinderGeometry(0.283, 0.268, 0.12, 4, 1);
    braGeo.rotateY(Math.PI / 4);
    braGeo.scale(1, 1, 0.62);
    this.bra = at(new THREE.Mesh(braGeo, m.top), 0, 0.36, 0);
    this.torso.add(this.bra);
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
    // mũ tai bèo: a soft crown and a wide drooping brim
    this.bucket.add(at(new THREE.Mesh(new THREE.CylinderGeometry(0.145, 0.155, 0.13, 10), m.bucket), 0, 0.3, 0.005));
    this.bucket.add(at(new THREE.Mesh(new THREE.CylinderGeometry(0.25, 0.22, 0.03, 12), m.bucket), 0, 0.245, 0.005));
    this.neck.add(this.bucket);
    // tóc búi: the hair gathered in a knot at the back of the head (the long hair is put up)
    this.bun = at(new THREE.Mesh(new THREE.IcosahedronGeometry(0.075, 1), m.hair), 0, 0.27, -0.14);
    this.neck.add(this.bun);
    // mũ bảo hiểm (the Crab Bike helmet): a yellow half shell over the hair and a dark visor brim
    const shell = new THREE.SphereGeometry(0.178, 12, 6, 0, Math.PI * 2, 0, Math.PI / 2);
    shell.scale(1, 0.92, 1.05);
    this.helmet.add(at(new THREE.Mesh(shell, m.helmet), 0, 0.225, -0.005));
    this.helmet.add(at(new THREE.Mesh(new THREE.BoxGeometry(0.22, 0.03, 0.09), m.eye), 0, 0.235, 0.175));
    this.neck.add(this.helmet);

    // blob shadow: the sun shadow map is baked once, so pedestrians get a soft contact shadow instead
    this.blobMat = new THREE.MeshBasicMaterial({ map: blobTexture(), color: '#2A2338', transparent: true, opacity: BLOB_OPACITY, depthWrite: false, polygonOffset: true, polygonOffsetFactor: -2 });
    const blob = new THREE.Mesh(new THREE.CircleGeometry(0.42, 20), this.blobMat);
    blob.rotation.x = -Math.PI / 2;
    blob.position.y = BLOB_Y;
    blob.renderOrder = 1;
    this.blob = blob;
    this.root.add(blob);

    this.setLook(look);
  }

  /** The feet are `h` metres above the ground (a jump): the shadow stays down there, smaller and fainter. */
  setHeight(h: number): void {
    const k = Math.max(0, h);
    this.blob.position.y = BLOB_Y - k;
    this.blob.scale.setScalar(1 / (1 + 0.8 * k));
    this.blobMat.opacity = BLOB_OPACITY / (1 + k);
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

    const legSkin = s.legs === 'skirt' || s.legs === 'bare';
    const shinSkin = s.legs !== 'pants';
    const armSkin = s.sleeves === 'short';
    this.torso.material = s.torso === 'shirt' ? m.top : m.skin;
    this.bra.visible = s.torso === 'bikini';
    const frame = FRAME[look.body];
    for (const side of [this.left, this.right]) {
      side.thigh.material = legSkin ? m.skin : m.bottom;
      side.shin.material = shinSkin ? m.skin : m.bottom;
      side.upperArm.material = armSkin ? m.skin : m.top;
      side.forearm.material = armSkin ? m.skin : m.top;
      // the sleeve cap over the shoulder joint: on bare shoulders it would read as a shoulder pad
      side.pad.visible = s.torso === 'shirt';
    }
    this.left.shoulder.position.x = frame.shoulderX;
    this.right.shoulder.position.x = -frame.shoulderX;
    this.torso.scale.x = frame.torsoX;
    this.body.scale.setScalar(frame.scale);

    this.bag.visible = s.bag !== null;
    this.skirt.visible = s.legs === 'skirt';
    this.flapFront.visible = this.flapBack.visible = s.flaps;
    this.longHair.visible = look.body === 'woman' && look.hat !== 'bun';
    this.nonLa.visible = look.hat === 'nonla';
    this.cap.visible = look.hat === 'cap';
    this.bucket.visible = look.hat === 'bucket';
    this.helmet.visible = look.hat === 'helmet';
    this.bun.visible = look.hat === 'bun';
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
    const pad = at(hanging(0.13, 0.15, 0.13, m.top), 0, 0.03, 0);
    shoulder.add(pad);
    const upperArm = hanging(0.082, 0.27, 0.085, m.skin);
    shoulder.add(upperArm);
    const elbow = at(new THREE.Group(), 0, -0.27, 0);
    shoulder.add(elbow);
    const forearm = hanging(0.078, 0.23, 0.08, m.skin);
    elbow.add(forearm);
    elbow.add(at(new THREE.Mesh(new THREE.BoxGeometry(0.085, 0.085, 0.085), m.skin), 0, -0.27, 0));
    this.spine.add(shoulder);
    return { hip, knee, shoulder, elbow, thigh, shin, upperArm, forearm, pad };
  }

  /**
   * Advance the gait. `speed` is the actual ground speed in m/s, `time` seconds since start; `pose` says what to
   * blend in on top.
   */
  update(dt: number, speed: number, time: number, pose: Readonly<AvatarPose> = IDLE): void {
    this.gait(dt, pose.ride ? 0 : speed, time);
    const playing = pose.emote !== '' && !pose.ride;
    if (playing) this.emote = pose.emote;
    this.emoteW += ((playing ? 1 : 0) - this.emoteW) * damp(8, dt);
    this.airW += ((pose.air && !pose.ride ? 1 : 0) - this.airW) * damp(14, dt);
    this.rideW += ((pose.ride ? 1 : 0) - this.rideW) * damp(12, dt);
    if (this.emoteW > 0.002) this.emotePose(this.emote, pose.emoteTime, this.emoteW);
    this.phone.visible = this.emote === 'photo' && this.emoteW > 0.4;
    if (this.airW > 0.002) this.airPose(this.airW);
    this.ridePose(this.rideW, pose.pillion);
  }

  /** The walk / run / idle cycle. */
  private gait(dt: number, speed: number, time: number): void {
    // only the overlay poses turn these; walking keeps them straight
    this.left.hip.rotation.z = this.right.hip.rotation.z = 0;
    this.left.elbow.rotation.z = this.right.elbow.rotation.z = 0;
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

  /** Emotes, by id (server/content/emotes.json); `t` is seconds since it started, `w` how far it is blended in. */
  private emotePose(id: string, t: number, w: number): void {
    const { left: l, right: r } = this;
    switch (id) {
      case 'wave': {
        // right hand up by the head, waving from the elbow
        const s = Math.sin(t * 10);
        r.shoulder.rotation.z = mix(r.shoulder.rotation.z, -2.5 + 0.12 * s, w);
        r.shoulder.rotation.x = mix(r.shoulder.rotation.x, -0.25, w);
        r.elbow.rotation.x = mix(r.elbow.rotation.x, -0.2, w);
        r.elbow.rotation.z = mix(r.elbow.rotation.z, -0.45 * s, w);
        this.neck.rotation.y = mix(this.neck.rotation.y, 0.12, w);
        break;
      }
      case 'bow': {
        // down and up again, arms hanging straight; the spine carries the arms, so they turn back by as much
        const b = smoothstep(0, 0.45, t) * (1 - smoothstep(1.15, 1.75, t));
        this.spine.rotation.x = mix(this.spine.rotation.x, 0.8 * b, w);
        for (let i = 0; i < 2; i++) {
          const side = i === 0 ? l : r;
          side.shoulder.rotation.x = mix(side.shoulder.rotation.x, -0.7 * b, w);
          side.elbow.rotation.x = mix(side.elbow.rotation.x, -0.1, w);
        }
        this.neck.rotation.x = mix(this.neck.rotation.x, 0.25 * b, w);
        this.neck.rotation.y = mix(this.neck.rotation.y, 0, w);
        break;
      }
      case 'photo':
        // both hands hold the phone up in front of the face
        l.shoulder.rotation.x = mix(l.shoulder.rotation.x, -1.35, w);
        r.shoulder.rotation.x = mix(r.shoulder.rotation.x, -1.35, w);
        l.shoulder.rotation.z = mix(l.shoulder.rotation.z, -0.32, w);
        r.shoulder.rotation.z = mix(r.shoulder.rotation.z, 0.32, w);
        l.elbow.rotation.x = mix(l.elbow.rotation.x, -1.3, w);
        r.elbow.rotation.x = mix(r.elbow.rotation.x, -1.3, w);
        this.neck.rotation.y = mix(this.neck.rotation.y, 0, w);
        this.neck.rotation.x = mix(this.neck.rotation.x, 0.08, w);
        break;
      case 'exercise': {
        // morning exercise in the park: arms swing up overhead and down while the knees bend
        const c = Math.sin(t * 3.2);
        const up = 0.25 + 1.25 * (1 + c);
        const squat = 0.5 * (1 - c);
        l.shoulder.rotation.z = mix(l.shoulder.rotation.z, up, w);
        r.shoulder.rotation.z = mix(r.shoulder.rotation.z, -up, w);
        for (let i = 0; i < 2; i++) {
          const side = i === 0 ? l : r;
          side.shoulder.rotation.x = mix(side.shoulder.rotation.x, 0, w);
          side.elbow.rotation.x = mix(side.elbow.rotation.x, -0.1, w);
          side.hip.rotation.x = mix(side.hip.rotation.x, -0.45 * squat, w);
          side.knee.rotation.x = mix(side.knee.rotation.x, 0.9 * squat, w);
        }
        this.body.position.y = mix(this.body.position.y, -0.1 * squat * this.body.scale.y, w);
        this.spine.rotation.x = mix(this.spine.rotation.x, 0.12 * squat, w);
        this.neck.rotation.y = mix(this.neck.rotation.y, 0, w);
        break;
      }
    }
  }

  /** In the air: knees up, arms out for balance. */
  private airPose(w: number): void {
    const { left: l, right: r } = this;
    l.hip.rotation.x = mix(l.hip.rotation.x, -0.75, w);
    r.hip.rotation.x = mix(r.hip.rotation.x, -0.3, w);
    l.knee.rotation.x = mix(l.knee.rotation.x, 1.15, w);
    r.knee.rotation.x = mix(r.knee.rotation.x, 0.7, w);
    l.shoulder.rotation.z = mix(l.shoulder.rotation.z, 0.55, w);
    r.shoulder.rotation.z = mix(r.shoulder.rotation.z, -0.55, w);
    l.shoulder.rotation.x = mix(l.shoulder.rotation.x, -0.45, w);
    r.shoulder.rotation.x = mix(r.shoulder.rotation.x, -0.45, w);
    l.elbow.rotation.x = mix(l.elbow.rotation.x, -0.7, w);
    r.elbow.rotation.x = mix(r.elbow.rotation.x, -0.7, w);
    // the gait's walking bob would push the body down into the jump
    this.body.position.y = mix(this.body.position.y, 0, w);
  }

  /**
   * Sitting on a motorbike (src/game/motorbike.ts): feet on the footrests, hands on the handlebar; as the
   * passenger, further back and higher, hands on the knees.
   */
  private ridePose(w: number, pillion: boolean): void {
    this.body.position.z = (pillion ? PILLION_BACK : SEAT_BACK) * w;
    if (w <= 0.002) return;
    for (let i = 0; i < 2; i++) {
      const side = i === 0 ? this.left : this.right;
      const sx = i === 0 ? 1 : -1;
      side.hip.rotation.x = mix(side.hip.rotation.x, pillion ? -1.3 : -1.4, w);
      side.hip.rotation.z = mix(side.hip.rotation.z, (pillion ? 0.2 : 0.13) * sx, w);
      side.knee.rotation.x = mix(side.knee.rotation.x, pillion ? 1.6 : 1.5, w);
      side.shoulder.rotation.x = mix(side.shoulder.rotation.x, pillion ? -0.55 : -1.0, w);
      side.shoulder.rotation.z = mix(side.shoulder.rotation.z, (pillion ? 0.2 : 0.12) * sx, w);
      side.elbow.rotation.x = mix(side.elbow.rotation.x, pillion ? -0.9 : -0.55, w);
    }
    const rise = pillion ? PILLION_RISE : 0;
    this.body.position.y = mix(this.body.position.y, (SEAT_DROP + rise) * this.body.scale.y, w);
    this.spine.rotation.x = mix(this.spine.rotation.x, pillion ? 0.08 : 0.24, w);
    this.spine.rotation.y = mix(this.spine.rotation.y, 0, w);
    this.neck.rotation.x = mix(this.neck.rotation.x, -0.18, w);
    this.neck.rotation.y = mix(this.neck.rotation.y, 0, w);
    // the áo dài front panel lies over the thighs, the skirt spreads over the seat
    this.flapFront.rotation.x = mix(this.flapFront.rotation.x, -1.35, w);
    this.flapBack.rotation.x = mix(this.flapBack.rotation.x, 0.35, w);
    this.skirt.scale.z = mix(this.skirt.scale.z, 1.45, w);
  }
}

