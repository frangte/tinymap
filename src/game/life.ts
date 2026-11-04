// Things that move on their own so the island feels alive: low-poly clouds drifting round it and a few gulls
// circling over the beach. Painted with the same toon shader and inked like the rest of the world.
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

export class Life {
  readonly root = new THREE.Group();
  private readonly clouds: Cloud[] = [];
  private readonly gulls: Gull[] = [];

  /**
   * @param center island centre (three.js) @param coast distance from the centre to the beach (m)
   */
  constructor(private readonly center: THREE.Vector3, coast: number) {
    this.root.name = 'Life';
    const rnd = mulberry32(1975);
    const puff = paintMaterial('#F7F4EC', 'cloud', { variation: 0.02, grime: 0, local: true });
    const shade = paintMaterial('#E3E4E6', 'cloudShade', { variation: 0.02, grime: 0, local: true });
    const ball = new THREE.IcosahedronGeometry(1, 1);
    for (let i = 0; i < 9; i++) {
      const g = new THREE.Group();
      const n = 4 + Math.floor(rnd() * 4);
      const len = 10 + rnd() * 14;
      for (let k = 0; k < n; k++) {
        const s = 2.6 + rnd() * 3.4 * (1 - Math.abs(k / (n - 1) - 0.5));
        const m = new THREE.Mesh(ball, k % 3 === 2 ? shade : puff);
        m.position.set((k / (n - 1) - 0.5) * len, s * 0.25 + rnd() * 1.5, (rnd() - 0.5) * 5);
        m.scale.set(s, s * 0.62, s * 0.85);
        g.add(m);
      }
      this.root.add(g);
      this.clouds.push({ group: g, radius: coast + 10 + rnd() * 80, angle: rnd() * Math.PI * 2, speed: (0.004 + rnd() * 0.004) * (rnd() < 0.2 ? -1 : 1), height: 58 + rnd() * 20, bob: rnd() * 6 });
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
  }

  update(time: number, dt: number): void {
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
