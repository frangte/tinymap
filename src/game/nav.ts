// Walkable heightfield baked from the Blender diorama (public/models/saigon_mini_nav.*):
// a 0.35 m grid over the island with the floor height per cell, a walkable flag, and the building top
// of every non-walkable cell (used to keep the camera out of buildings).
import { clearanceField } from './clearance';
import { clamp } from './coords';

export interface NavMeta {
  /** Blender X of the grid's west edge */
  x0: number;
  /** Blender Y of the grid's south edge */
  y0: number;
  /** cell size in metres */
  cell: number;
  nx: number;
  ny: number;
  /** spawn point, Blender XY */
  spawn: [number, number];
  /** sea level (Blender z); the sea is drawn at this height */
  sea_level?: number;
  /** centre of the island, Blender XY */
  island_center?: [number, number];
}

interface Pixels {
  data: Uint8ClampedArray;
  width: number;
  height: number;
}

async function readPixels(url: string): Promise<Pixels> {
  const res = await fetch(url);
  if (!res.ok) throw new Error(`${url}: HTTP ${res.status}`);
  // no colour management or premultiplication: the channels hold raw numbers, not colours
  const bmp = await createImageBitmap(await res.blob(), { colorSpaceConversion: 'none', premultiplyAlpha: 'none' });
  const canvas = document.createElement('canvas');
  canvas.width = bmp.width;
  canvas.height = bmp.height;
  const g = canvas.getContext('2d', { willReadFrequently: true });
  if (!g) throw new Error('2D canvas unavailable');
  g.drawImage(bmp, 0, 0);
  bmp.close();
  return { data: g.getImageData(0, 0, canvas.width, canvas.height).data, width: canvas.width, height: canvas.height };
}

function isNavMeta(v: unknown): v is NavMeta {
  if (typeof v !== 'object' || v === null) return false;
  const m = v as Record<string, unknown>;
  return ['x0', 'y0', 'cell', 'nx', 'ny'].every((k) => typeof m[k] === 'number') && Array.isArray(m.spawn) && m.spawn.length === 2;
}

export class NavGrid {
  private constructor(
    readonly meta: NavMeta,
    private readonly floor: Float32Array,
    private readonly walk: Uint8Array,
    private readonly roof: Float32Array,
    /** per cell, metres from its centre to the nearest blocked cell (clearance.ts) */
    private readonly clear: Float32Array,
  ) {}

  /** Loads `<base>_nav.json`, `<base>_nav.png` and `<base>_roof.png`. */
  static async load(base: string): Promise<NavGrid> {
    const res = await fetch(`${base}_nav.json`);
    if (!res.ok) throw new Error(`${base}_nav.json: HTTP ${res.status}`);
    const meta: unknown = await res.json();
    if (!isNavMeta(meta)) throw new Error(`${base}_nav.json: unexpected format`);
    const [nav, top] = await Promise.all([readPixels(`${base}_nav.png`), readPixels(`${base}_roof.png`)]);
    const n = meta.nx * meta.ny;
    if (nav.width !== meta.nx || nav.height !== meta.ny || top.width !== meta.nx || top.height !== meta.ny) {
      throw new Error('nav map size does not match its metadata');
    }
    const floor = new Float32Array(n);
    const walk = new Uint8Array(n);
    const roof = new Float32Array(n);
    for (let i = 0; i < n; i++) {
      const p = i * 4;
      floor[i] = (nav.data[p] * 256 + nav.data[p + 1]) / 100 - 10;
      walk[i] = nav.data[p + 2] > 127 ? 1 : 0;
      roof[i] = (top.data[p] * 256 + top.data[p + 1]) / 100;
    }
    return new NavGrid(meta, floor, walk, roof, clearanceField(walk, meta.nx, meta.ny, meta.cell));
  }

  /** Cell index for a three.js position (x, z), or -1 outside the grid. Row 0 is the south edge. */
  index(x: number, z: number): number {
    const i = Math.floor((x - this.meta.x0) / this.meta.cell);
    const j = Math.floor((-z - this.meta.y0) / this.meta.cell);
    if (i < 0 || j < 0 || i >= this.meta.nx || j >= this.meta.ny) return -1;
    return j * this.meta.nx + i;
  }

  isWalkable(x: number, z: number): boolean {
    const k = this.index(x, z);
    return k >= 0 && this.walk[k] === 1;
  }

  /** Floor height of the cell under (x, z); NaN outside the grid. */
  floorAt(x: number, z: number): number {
    const k = this.index(x, z);
    return k < 0 ? NaN : this.floor[k];
  }

  /** Floor height blended over the walkable cells around (x, z), so ramps and kerbs read smoothly. */
  smoothFloor(x: number, z: number, fallback: number): number {
    const { x0, y0, cell, nx, ny } = this.meta;
    const fx = (x - x0) / cell - 0.5;
    const fy = (-z - y0) / cell - 0.5;
    const i0 = Math.floor(fx);
    const j0 = Math.floor(fy);
    const tx = fx - i0;
    const ty = fy - j0;
    let sum = 0;
    let wsum = 0;
    for (let dj = 0; dj <= 1; dj++) {
      for (let di = 0; di <= 1; di++) {
        const i = clamp(i0 + di, 0, nx - 1);
        const j = clamp(j0 + dj, 0, ny - 1);
        const k = j * nx + i;
        if (this.walk[k] !== 1) continue;
        const w = (di ? tx : 1 - tx) * (dj ? ty : 1 - ty) + 1e-4;
        sum += this.floor[k] * w;
        wsum += w;
      }
    }
    return wsum > 0 ? sum / wsum : fallback;
  }

  /**
   * How far (x, z) is from the nearest cell nobody can walk on (m), blended between cell centres so obstacles read
   * round: a trunk is a disc to slide around, not a square to catch on. Negative inside an obstacle and outside the grid.
   */
  clearance(x: number, z: number): number {
    const { x0, y0, cell, nx, ny } = this.meta;
    const fx = (x - x0) / cell - 0.5;
    const fy = (-z - y0) / cell - 0.5;
    const i0 = Math.floor(fx);
    const j0 = Math.floor(fy);
    const tx = fx - i0;
    const ty = fy - j0;
    const south = this.clearAt(i0, j0, nx, ny) * (1 - tx) + this.clearAt(i0 + 1, j0, nx, ny) * tx;
    const north = this.clearAt(i0, j0 + 1, nx, ny) * (1 - tx) + this.clearAt(i0 + 1, j0 + 1, nx, ny) * tx;
    return south * (1 - ty) + north * ty;
  }

  private clearAt(i: number, j: number, nx: number, ny: number): number {
    return i < 0 || j < 0 || i >= nx || j >= ny ? -1 : this.clear[j * nx + i];
  }

  /** The way out of the nearest obstacle at (x, z), a unit vector on the ground written into `out`; false where there is none. */
  awayFromObstacle(x: number, z: number, out: { x: number; z: number }): boolean {
    const h = this.meta.cell / 2;
    const gx = this.clearance(x + h, z) - this.clearance(x - h, z);
    const gz = this.clearance(x, z + h) - this.clearance(x, z - h);
    const len = Math.hypot(gx, gz);
    if (len < 1e-9) return false;
    out.x = gx / len;
    out.z = gz / len;
    return true;
  }

  /** Building top above the cell (0 when open sky / walkable). */
  roofAt(x: number, z: number): number {
    const k = this.index(x, z);
    return k < 0 ? 0 : this.roof[k];
  }

  /** The closest spot to (x, z) where a body of `radius` fits, searching outward up to `maxDist` metres. */
  nearestStanding(x: number, z: number, radius: number, maxDist = 8): [number, number] | null {
    const c = this.meta.cell;
    for (let r = 0; r <= maxDist; r += c) {
      const n = Math.max(1, Math.round((2 * Math.PI * r) / c));
      for (let k = 0; k < n; k++) {
        const a = (k / n) * Math.PI * 2;
        const px = x + Math.cos(a) * r;
        const pz = z + Math.sin(a) * r;
        if (this.canStand(px, pz, radius, this.floorAt(px, pz), 0)) return [px, pz];
      }
    }
    return null;
  }

  /**
   * Whether a body of `radius` fits at (x, z): nothing blocked within `radius` of it (its clearance) and the floor
   * under its centre within `maxStep` of `fromFloor` (kerbs yes, walls and drops no).
   */
  canStand(x: number, z: number, radius: number, fromFloor: number, maxStep: number): boolean {
    return this.canStandBetween(x, z, radius, fromFloor - maxStep, fromFloor + maxStep);
  }

  /** Like canStand, with the floor under the centre anywhere in [lo, hi] (a jump reaches higher than a step). */
  canStandBetween(x: number, z: number, radius: number, lo: number, hi: number): boolean {
    const k = this.index(x, z);
    if (k < 0 || this.walk[k] !== 1) return false;
    if (this.floor[k] < lo || this.floor[k] > hi) return false;
    return radius <= 0 || this.clearance(x, z) >= radius;
  }
}
