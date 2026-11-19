// Things that move on their own so the island feels alive: low-poly clouds drifting round it and a few gulls
// circling over the beach. Painted with the same toon shader and inked like the rest of the world. The weather
// (setWeather) brings more clouds round the island, greys them in the rain, and sends the gulls to shelter.
import * as THREE from 'three';
import { paintMaterial } from './paint';

interface Cloud {
  group: THREE.Group;
  radius: number;
  angle: number;
  speed: number;
  height: number;
  bob: number;
}

interface Gull {
  group: THREE.Group;
  left: THREE.Object3D;
  right: THREE.Object3D;
  cx: number;
  cz: number;
  radius: number;
  angle: number;
  speed: number;
  height: number;
  flap: number;
}

/** small deterministic random so the sky looks the same on every visit */
function mulberry32(seed: number): () => number {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

/** the clouds' paint in fair weather and under rain */
const CLOUD_FAIR = new THREE.Color('#F7F4EC');
const CLOUD_RAIN = new THREE.Color('#A3A8AE');
const SHADE_FAIR = new THREE.Color('#E3E4E6');
const SHADE_RAIN = new THREE.Color('#838A92');
/** clouds that only come with a cloudy sky */
const STORM_CLOUDS = 10;
/** gulls shelter from rain heavier than this */
const GULLS_SHELTER = 0.45;

export class Life {
  readonly root = new THREE.Group();
  private readonly clouds: Cloud[] = [];
  /** the clouds of a cloudy sky, shown one by one as it clouds over */
  private readonly extra: Cloud[] = [];
  private readonly gulls: Gull[] = [];
  private readonly puff: THREE.MeshToonMaterial;
  private readonly shade: THREE.MeshToonMaterial;
  private shownCloud = -1;
  private shownRain = -1;

  /**
   * @param center island centre (three.js) @param coast distance from the centre to the beach (m)
   */
  constructor(private readonly center: THREE.Vector3, coast: number) {
    this.root.name = 'Life';
    const rnd = mulberry32(1975);
    const puff = paintMaterial(CLOUD_FAIR, 'cloud', { variation: 0.02, grime: 0, local: true });
    const shade = paintMaterial(SHADE_FAIR, 'cloudShade', { variation: 0.02, grime: 0, local: true });
    this.puff = puff;
    this.shade = shade;
    const ball = new THREE.IcosahedronGeometry(1, 1);
    const cloud = (r: () => number, scale: number): THREE.Group => {
      const g = new THREE.Group();
      const n = 4 + Math.floor(r() * 4);
      const len = (10 + r() * 14) * scale;
      for (let k = 0; k < n; k++) {
        const s = (2.6 + r() * 3.4 * (1 - Math.abs(k / (n - 1) - 0.5))) * scale;
        const m = new THREE.Mesh(ball, k % 3 === 2 ? shade : puff);
        m.position.set((k / (n - 1) - 0.5) * len, s * 0.25 + r() * 1.5, (r() - 0.5) * 5 * scale);
        m.scale.set(s, s * 0.62, s * 0.85);
        g.add(m);
      }
      this.root.add(g);
      return g;
    };
    for (let i = 0; i < 9; i++) {
      this.clouds.push({ group: cloud(rnd, 1), radius: coast + 10 + rnd() * 80, angle: rnd() * Math.PI * 2, speed: (0.004 + rnd() * 0.004) * (rnd() < 0.2 ? -1 : 1), height: 58 + rnd() * 20, bob: rnd() * 6 });
    }

    const white = paintMaterial('#F4F1EA', 'gull', { variation: 0.02, grime: 0, local: true });
    const tip = paintMaterial('#3B3330', 'gullTip', { variation: 0.02, grime: 0, local: true });
    const bodyGeo = new THREE.BoxGeometry(0.22, 0.2, 0.75);
    const wingGeo = new THREE.BoxGeometry(0.9, 0.04, 0.32);
    wingGeo.translate(0.45, 0, 0);
    const tipGeo = new THREE.BoxGeometry(0.3, 0.045, 0.22);
    tipGeo.translate(1.05, 0, -0.04);
    for (let i = 0; i < 6; i++) {
      const g = new THREE.Group();
      g.add(new THREE.Mesh(bodyGeo, white));
      const mk = (side: number): THREE.Object3D => {
        const w = new THREE.Group();
        w.add(new THREE.Mesh(wingGeo, white), new THREE.Mesh(tipGeo, tip));
        w.scale.x = side;
        g.add(w);
        return w;
      };
      const a = rnd() * Math.PI * 2;
      const r = coast - 6 + rnd() * 18;
      this.gulls.push({
        group: g,
        left: mk(1),
        right: mk(-1),
        cx: center.x + Math.cos(a) * r,
        cz: center.z + Math.sin(a) * r,
        radius: 10 + rnd() * 18,
        angle: rnd() * Math.PI * 2,
        speed: (0.22 + rnd() * 0.12) * (rnd() < 0.5 ? -1 : 1),
        height: 11 + rnd() * 12,
        flap: rnd() * 10,
      });
      this.root.add(g);
    }
    // a stream of its own, so the fair-weather sky stays as it always was
    const wet = mulberry32(2611);
    for (let i = 0; i < STORM_CLOUDS; i++) {
      const g = cloud(wet, 1.35);
      g.visible = false;
      // round the island like the fair-weather clouds, never over the streets: from above they would hide the city
      this.extra.push({ group: g, radius: coast + 5 + wet() * 110, angle: wet() * Math.PI * 2, speed: (0.003 + wet() * 0.004) * (wet() < 0.3 ? -1 : 1), height: 60 + wet() * 18, bob: wet() * 6 });
    }
  }

  /** The sky's cloud cover and the rain's strength (0–1), from daylight/weather.ts. */
  setWeather(cloud: number, rain: number): void {
    if (Math.abs(cloud - this.shownCloud) < 0.01 && Math.abs(rain - this.shownRain) < 0.01) return;
    this.shownCloud = cloud;
    this.shownRain = rain;
    this.extra.forEach((c, i) => {
      c.group.visible = cloud > 0.35 + (i / STORM_CLOUDS) * 0.6;
    });
    const dark = Math.min(1, rain * 0.85 + Math.max(0, cloud - 0.5) * 0.4);
    this.puff.color.lerpColors(CLOUD_FAIR, CLOUD_RAIN, dark);
    this.shade.color.lerpColors(SHADE_FAIR, SHADE_RAIN, dark);
    for (const g of this.gulls) g.group.visible = rain < GULLS_SHELTER;
  }

  update(time: number, dt: number): void {
    for (const c of this.extra) {
      if (!c.group.visible) continue;
      c.angle += c.speed * dt;
      c.group.position.set(this.center.x + Math.cos(c.angle) * c.radius, c.height + Math.sin(time * 0.05 + c.bob) * 1.5, this.center.z + Math.sin(c.angle) * c.radius);
      c.group.rotation.y = -c.angle;
    }
    for (const c of this.clouds) {
      c.angle += c.speed * dt;
      c.group.position.set(this.center.x + Math.cos(c.angle) * c.radius, c.height + Math.sin(time * 0.05 + c.bob) * 1.5, this.center.z + Math.sin(c.angle) * c.radius);
      c.group.rotation.y = -c.angle;
    }
    for (const g of this.gulls) {
      g.angle += g.speed * dt;
      const x = g.cx + Math.cos(g.angle) * g.radius;
      const z = g.cz + Math.sin(g.angle) * g.radius;
      g.group.position.set(x, g.height + Math.sin(time * 0.4 + g.flap) * 1.2, z);
      // face along the circle and bank into it
      const dir = Math.sign(g.speed);
      g.group.rotation.set(0, -g.angle - (dir > 0 ? 0 : Math.PI), 0.35 * dir);
      // flap in bursts, glide in between
      const burst = Math.max(0, Math.sin(time * 0.5 + g.flap));
      const f = Math.sin(time * 9 + g.flap) * 0.55 * burst + 0.12;
      g.left.rotation.z = f;
      g.right.rotation.z = -f;
    }
  }
}
