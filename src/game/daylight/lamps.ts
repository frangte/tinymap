// The street lamps at night. The map has no list of them: every lamp globe, the string lights of Bùi Viện and the
// lights of Ba Son bridge are painted lampGlow and merged into one mesh, so the lamps are found again by grouping its
// vertices. A string of bulbs would make one long band of light, so pools are kept a few metres apart. The paint
// shader lights a pool under the few nearest to what the camera looks at (paint.ts NIGHT, MAX_LAMPS).
import * as THREE from 'three';
import { MAX_LAMPS, NIGHT } from '../paint';

/** vertices closer than this (metres) belong to the same lamp */
const SAME_LAMP = 1.5;
/** pools closer than this (metres) are one: a string of bulbs lights the street every few metres, not all along */
const POOL_SPACING = 7;
/** the pool's centre sits this far below the globe, so the ground under it gets the bright band */
const POOL_DROP = 2.6;
/** pick the nearest lamps again when the focus has moved this far (metres) */
const REFOCUS = 3;
/** lamps further than this from the focus are left out even when there are fewer than MAX_LAMPS */
const REACH = 70;

interface Cluster {
  sum: THREE.Vector3;
  n: number;
}

export class StreetLamps {
  /** pool centres, three.js world */
  readonly pools: THREE.Vector3[] = [];
  private readonly focusAt = new THREE.Vector3(Infinity, 0, 0);
  private readonly order: number[] = [];
  private readonly dist: number[] = [];

  constructor(world: THREE.Object3D) {
    world.updateMatrixWorld(true);
    const clusters: Cluster[] = [];
    const grid = new Map<string, number[]>();
    const v = new THREE.Vector3();
    const cellOf = (x: number, z: number): [number, number] => [Math.floor(x / SAME_LAMP), Math.floor(z / SAME_LAMP)];
    world.traverse((o) => {
      if (!(o instanceof THREE.Mesh) || o.userData.paintKey !== 'lampGlow') return;
      const pos = (o.geometry as THREE.BufferGeometry).getAttribute('position');
      for (let i = 0; i < pos.count; i++) {
        v.fromBufferAttribute(pos, i).applyMatrix4(o.matrixWorld);
        const [cx, cz] = cellOf(v.x, v.z);
        let found = -1;
        for (let dx = -1; dx <= 1 && found < 0; dx++) {
          for (let dz = -1; dz <= 1 && found < 0; dz++) {
            for (const c of grid.get(`${cx + dx},${cz + dz}`) ?? []) {
              const k = clusters[c];
              if (Math.hypot(k.sum.x / k.n - v.x, k.sum.z / k.n - v.z) < SAME_LAMP) {
                found = c;
                break;
              }
            }
          }
        }
        if (found < 0) {
          found = clusters.length;
          clusters.push({ sum: new THREE.Vector3(), n: 0 });
          const key = `${cx},${cz}`;
          const list = grid.get(key);
          if (list) list.push(found);
          else grid.set(key, [found]);
        }
        clusters[found].sum.add(v);
        clusters[found].n++;
      }
    });
    for (const c of clusters) {
      const p = c.sum.divideScalar(c.n);
      p.y -= POOL_DROP;
      if (this.pools.some((q) => Math.hypot(q.x - p.x, q.z - p.z) < POOL_SPACING)) continue;
      this.pools.push(p);
      this.order.push(this.order.length);
      this.dist.push(0);
    }
  }

  /** Light the lamps nearest to `focus` (three.js world; what the camera looks at). */
  focus(focus: THREE.Vector3): void {
    if (Math.hypot(focus.x - this.focusAt.x, focus.z - this.focusAt.z) < REFOCUS) return;
    this.focusAt.copy(focus);
    for (let i = 0; i < this.pools.length; i++) this.dist[i] = Math.hypot(this.pools[i].x - focus.x, this.pools[i].z - focus.z);
    this.order.sort((a, b) => this.dist[a] - this.dist[b]);
    const out = NIGHT.uLamps.value;
    let n = 0;
    for (const i of this.order) {
      if (n === MAX_LAMPS || this.dist[i] > REACH) break;
      out[n++].copy(this.pools[i]);
    }
    NIGHT.uLampCount.value = n;
  }
}
