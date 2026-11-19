// The map's collision: every triangle of the diorama a body can bump into, in one bounding volume hierarchy
// (three-mesh-bvh), built from the map GLB once it has loaded. What you see is what stops you: walls, kerbs,
// trunks, railings, steps and slopes are their own shapes, with nothing baked or placed by hand in between. Water
// is kept apart: it stops a body only where it is deeper than WADE, and so does the sea, drawn flat at
// MAP.seaLevel. One shape is added for collision only: where the land ends in mid-air over dry floor (the rim of
// the hill, an open shell over the flat island), a wall under that edge down to the floor keeps bodies from walking
// in under it (skirts()). Closing the shell in Blender makes the walls go away by themselves. Built things (a
// bridge deck, the Bitexco podium) get none: the space under them is open, as it looks.
//
// The player (player.ts) moves a capsule through it, the follow camera (followCamera.ts) keeps out of walls with
// it, and everything that needs a floor (spawning, parking a bike, NPCs, the shadow under someone jumping) asks
// here. The server takes the game's word for where its player stands and only checks speeds
// (docs/design/04-backend.md). In Three.js terms this is the three-mesh-bvh "character movement" example, with the
// render mesh as the hit mesh.
import * as THREE from 'three';
import { CENTER, INTERSECTED, MeshBVH, NOT_INTERSECTED, type ExtendedTriangle } from 'three-mesh-bvh';

/** a surface flatter than this (|normal.y|, about 45°) can be stood on; steeper is a wall to slide along */
export const FLOOR_NY = 0.7;
/** water deeper than this stops a body (m): wading at the edge, never swimming */
export const WADE = 0.25;
/** a ledge with no floor under it within this far (m) is the edge of the map */
export const FALL_LIMIT = 60;
/** room a body needs over a floor to stand there (m) */
const HEADROOM = 1.7;
/** floorNear() takes the highest floor up to this far over the height asked about (m): a step's worth */
const NEAR_UP = 0.6;
/** fits() lets a body touch what it stands on and what it leans on by this much (m) */
const TOUCH = 0.02;
/** nearestStanding() looks this far apart round the spot asked for (m) */
const SEARCH_STEP = 0.25;

/** what a body passes through: light glows and hanging lanterns (Blender material names without SD_) */
const PASS_THROUGH = new Set(['lampGlow', 'lantern']);
const WATER = new Set(['water', 'waterDeep', 'foam']);
/** tree crowns and the bushes painted on the hill: they stop a body, the camera looks through them (paint.ts) */
const FOLIAGE = new Set(['leaf', 'leafLight', 'leafDark']);
/**
 * tree trunks and branches, iron railings: they stop a body, but the camera looks through them. A sight line that
 * jumps in for every trunk it passes behind jerks the camera into the player's back and out again.
 */
const THIN = new Set(['bark', 'railing']);
/** per-vertex flags */
const FLAG_FOLIAGE = 1;
const FLAG_THIN = 2;
/** what the camera's sight lines and its floor leave out */
const CAMERA_SKIP = FLAG_FOLIAGE | FLAG_THIN;
/** the land itself and what grows on it: an open edge of these over dry floor gets a skirt */
const TERRAIN = new Set(['grass', 'grassDark', 'rock', 'rockDark', 'earth', 'earthDark', 'sand', 'leaf', 'leafLight', 'leafDark']);
/** the land and the roads, paths and stairs on it, welded together to find the open edges (a path up the hill is
 *  not an edge of the hill) */
const GROUND = new Set([...TERRAIN, 'asphalt', 'curb', 'roadLine', 'paving', 'pavingDark', 'stone', 'concrete', 'concreteLight', 'bridgeWhite']);
/** per-vertex flag of the ground soup: a terrain material */
const FLAG_TERRAIN = 1;
/** an open edge of the ground this high or more over dry floor gets a skirt (m): a body could get in under it */
const SKIRT_GAP = 1.0;
/** a skirt reaches this far into the floor under it (m), so nothing slips under its foot */
const SKIRT_FOOT = 0.05;
/** nearestStanding() looks from `from` to a spot at this height over both floors (m): a wall in between rules it out */
const SIGHT_H = 0.9;

/** what collide() pushed a capsule off */
export const CONTACT_FLOOR = 1;
export const CONTACT_WALL = 2;
export const CONTACT_CEILING = 4;

/** The Blender material name a mesh was exported with, without SD_ and Blender's .001 suffixes (as paint.ts). */
function materialKey(mesh: THREE.Mesh): string {
  const m = Array.isArray(mesh.material) ? mesh.material[0] : mesh.material;
  return (m?.name ?? '').replace(/^SD_/, '').replace(/\.\d+$/, '');
}

interface Soup {
  position: Float32Array;
  index: Uint32Array;
  flags: Uint8Array;
}

/** Every triangle of the meshes under `root` whose material `keep` takes, in world space, as one indexed soup. */
function gather(root: THREE.Object3D, keep: (key: string) => boolean, flagOf: (key: string) => number): Soup | null {
  root.updateMatrixWorld(true);
  const meshes: { mesh: THREE.Mesh; flag: number }[] = [];
  let verts = 0;
  let indices = 0;
  root.traverse((o) => {
    if (!(o instanceof THREE.Mesh)) return;
    const key = materialKey(o);
    const g = o.geometry as THREE.BufferGeometry;
    const p = g.getAttribute('position');
    if (!keep(key) || !p) return;
    meshes.push({ mesh: o, flag: flagOf(key) });
    verts += p.count;
    indices += g.index ? g.index.count : p.count;
  });
  if (indices === 0) return null;
  const position = new Float32Array(verts * 3);
  const index = new Uint32Array(indices);
  const flags = new Uint8Array(verts);
  const v = new THREE.Vector3();
  let vo = 0;
  let io = 0;
  for (const { mesh, flag } of meshes) {
    const g = mesh.geometry as THREE.BufferGeometry;
    const p = g.getAttribute('position');
    for (let i = 0; i < p.count; i++) {
      v.fromBufferAttribute(p, i).applyMatrix4(mesh.matrixWorld);
      position[(vo + i) * 3] = v.x;
      position[(vo + i) * 3 + 1] = v.y;
      position[(vo + i) * 3 + 2] = v.z;
    }
    flags.fill(flag, vo, vo + p.count);
    const ix = g.index;
    const n = ix ? ix.count : p.count;
    for (let i = 0; i < n; i++) index[io + i] = vo + (ix ? ix.getX(i) : i);
    vo += p.count;
    io += n;
  }
  return { position, index, flags };
}

function bvhOf(soup: Soup): { bvh: MeshBVH; geometry: THREE.BufferGeometry } {
  const geometry = new THREE.BufferGeometry();
  geometry.setAttribute('position', new THREE.BufferAttribute(soup.position, 3));
  // the BVH reorders this index in place; the per-vertex flags are untouched by that
  geometry.setIndex(new THREE.BufferAttribute(soup.index, 1));
  return { bvh: new MeshBVH(geometry, { strategy: CENTER, targetLeafSize: 10 }), geometry };
}

/** The floor to stand on at (x, z) near the height nearY (CityCollider.floorNear), NaN for none. */
export type FloorAt = (x: number, z: number, nearY: number) => number;

/**
 * The edges of the ground soup that only one triangle uses, once the triangles of all its materials are welded
 * together (a seam between grass and rock is not an edge): triples of two vertex indices and that triangle's first
 * index (into the soup's index).
 */
function openEdges(ground: Soup): Int32Array {
  const pos = ground.position;
  const n = pos.length / 3;
  // weld by position to the centimetre; the key is exact in a double for ±300 m
  const Q = 60001;
  const H = 30000;
  const ids = new Map<number, number>();
  const weld = new Int32Array(n);
  for (let i = 0; i < n; i++) {
    const k = ((Math.round(pos[3 * i] * 100) + H) * Q + (Math.round(pos[3 * i + 1] * 100) + H)) * Q + (Math.round(pos[3 * i + 2] * 100) + H);
    let w = ids.get(k);
    if (w === undefined) {
      w = i;
      ids.set(k, i);
    }
    weld[i] = w;
  }
  // per edge: the triangle that uses it, -1 once a second one does
  const uses = new Map<number, number>();
  const ix = ground.index;
  for (let t = 0; t < ix.length; t += 3) {
    for (let e = 0; e < 3; e++) {
      const a = weld[ix[t + e]];
      const b = weld[ix[t + ((e + 1) % 3)]];
      if (a === b) continue;
      const k = a < b ? a * n + b : b * n + a;
      uses.set(k, uses.has(k) ? -1 : t);
    }
  }
  const out: number[] = [];
  for (const [k, t] of uses) if (t >= 0) out.push(Math.floor(k / n), k % n, t);
  return Int32Array.from(out);
}

/** A body's shape: a capsule standing on its lowest point. */
export interface Body {
  radius: number;
  height: number;
}

export class CityCollider {
  /** the lowest point of the map (three.js y): a body below it has fallen out of the world */
  readonly bottom: number;
  private readonly top: number;
  private readonly index: ArrayLike<number>;

  // ---- the ray cast in flight (castRay), and its answer
  private readonly ray = new THREE.Ray();
  private rayFar = 0;
  private raySkip = 0;
  private rayBest = 0;
  /** the normal's y of the last castRay() hit, for telling floors from walls */
  private hitNormalY = 0;
  // ---- the capsule in flight (collide, fits)
  private readonly seg = new THREE.Line3();
  private readonly segBox = new THREE.Box3();
  private capRadius = 0;
  private contacts = 0;
  // ---- scratch
  private readonly v0 = new THREE.Vector3();
  private readonly v1 = new THREE.Vector3();
  private readonly v2 = new THREE.Vector3();
  private readonly down = new THREE.Vector3(0, -1, 0);
  private readonly sightFrom = new THREE.Vector3();
  private readonly sightTo = new THREE.Vector3();
  private readonly wetRay = new THREE.Ray();
  /** the walls under the open edges of the ground (skirts()), for bodies only: the camera and floors ignore them */
  private skirt: MeshBVH | null = null;
  /** how many skirt triangles there are (the ?debug view) */
  skirtTriangles = 0;
  /**
   * The sideways part of the last collide()'s push (y unused). It points away from what the body bumped into, a
   * wall or the edge of a kerb, so the player can look for a step on the other side (player.ts). Standing on a
   * slope adds nothing: the player stands a body on the highest floor round it, clear of the slope.
   */
  readonly sidePush = new THREE.Vector3();

  private constructor(
    private readonly bvh: MeshBVH,
    readonly geometry: THREE.BufferGeometry,
    private readonly flags: Uint8Array,
    private readonly water: MeshBVH | null,
    readonly seaLevel: number,
  ) {
    const box = new THREE.Box3();
    bvh.getBoundingBox(box);
    this.top = box.max.y + 1;
    this.bottom = box.min.y;
    const index = geometry.index;
    if (!index) throw new Error('collider: no index');
    this.index = index.array;
  }

  /**
   * The collision of the map as loaded: every mesh but the water and the lights. Run it before paintWorld(), which
   * swaps the materials for ones without the Blender names. About half a second for the whole island.
   */
  static fromWorld(world: THREE.Object3D, seaLevel: number): CityCollider {
    const solid = gather(
      world,
      (k) => !PASS_THROUGH.has(k) && !WATER.has(k),
      (k) => (FOLIAGE.has(k) ? FLAG_FOLIAGE : 0) | (THIN.has(k) ? FLAG_THIN : 0),
    );
    if (!solid) throw new Error('the map has nothing to stand on');
    const wet = gather(world, (k) => WATER.has(k), () => 0);
    const { bvh, geometry } = bvhOf(solid);
    const collider = new CityCollider(bvh, geometry, solid.flags, wet ? bvhOf(wet).bvh : null, seaLevel);
    const ground = gather(world, (k) => GROUND.has(k), (k) => (TERRAIN.has(k) ? FLAG_TERRAIN : 0));
    if (ground) collider.skirts(ground);
    return collider;
  }

  /**
   * A wall under every open edge of the land (TERRAIN) that hangs SKIRT_GAP or more over dry floor, down into that
   * floor: the rim of the hollow hill. Over water the water stops a body already. The land is welded to the roads,
   * paths and stairs on it first, so the side of a path up the hill is not an edge; a road's or a deck's own open
   * edges get no skirt (the space under a bridge's end on the bank, under the Bitexco podium's rim, is open).
   */
  private skirts(ground: Soup): void {
    const edges = openEdges(ground);
    const p = ground.position;
    const out: number[] = [];
    const floorUnder = (x: number, y: number, z: number): number => this.floorBelow(x, y - 0.02, z, FALL_LIMIT);
    for (let e = 0; e < edges.length; e += 3) {
      // only the land itself: a deck, a podium or a kerb has open space under it, as it looks
      if ((ground.flags[ground.index[edges[e + 2]]] & FLAG_TERRAIN) === 0) continue;
      const a = edges[e] * 3;
      const b = edges[e + 1] * 3;
      const mx = (p[a] + p[b]) / 2;
      const my = (p[a + 1] + p[b + 1]) / 2;
      const mz = (p[a + 2] + p[b + 2]) / 2;
      const mid = floorUnder(mx, my, mz);
      if (!Number.isFinite(mid) || my - mid < SKIRT_GAP || this.isDeep(mx, mz, mid)) continue;
      const fa = floorUnder(p[a], p[a + 1], p[a + 2]);
      const fb = floorUnder(p[b], p[b + 1], p[b + 2]);
      const ya = (Number.isFinite(fa) ? Math.min(fa, mid) : mid) - SKIRT_FOOT;
      const yb = (Number.isFinite(fb) ? Math.min(fb, mid) : mid) - SKIRT_FOOT;
      // a, b, b under, a under
      out.push(p[a], p[a + 1], p[a + 2], p[b], p[b + 1], p[b + 2], p[b], yb, p[b + 2], p[a], ya, p[a + 2]);
    }
    const quads = out.length / 12;
    if (quads === 0) return;
    const index = new Uint32Array(quads * 6);
    for (let q = 0; q < quads; q++) index.set([4 * q, 4 * q + 1, 4 * q + 2, 4 * q, 4 * q + 2, 4 * q + 3], 6 * q);
    this.skirt = bvhOf({ position: Float32Array.from(out), index, flags: new Uint8Array(quads * 4) }).bvh;
    this.skirtTriangles = quads * 2;
  }

  // ---------------------------------------------------------------- rays

  /** shapecast callbacks for castRay(): boxes nearest first, stop looking past the nearest hit so far */
  private readonly rayCast = {
    // Ray.intersectBox gives the way out of a box the ray starts in: that one is at distance 0
    boundsTraverseOrder: (box: THREE.Box3): number =>
      box.containsPoint(this.ray.origin) ? 0 : this.ray.intersectBox(box, this.v0) ? this.v0.distanceTo(this.ray.origin) : Infinity,
    intersectsBounds: (_box: THREE.Box3, _leaf: boolean, score: number | undefined): number =>
      score !== undefined && score < this.rayBest ? INTERSECTED : NOT_INTERSECTED,
    intersectsTriangle: (tri: ExtendedTriangle, i: number): boolean => {
      if (this.raySkip !== 0 && (this.flags[this.index[3 * i]] & this.raySkip) !== 0) return false;
      if (!this.ray.intersectTriangle(tri.a, tri.b, tri.c, false, this.v1)) return false;
      const d = this.v1.distanceTo(this.ray.origin);
      if (d < this.rayBest) {
        this.rayBest = d;
        tri.getNormal(this.v2);
        this.hitNormalY = this.v2.y;
      }
      return false;
    },
  };

  /** Distance along the ray to the nearest triangle within `far` (Infinity for none); `skip` leaves out flagged ones. */
  private castRay(origin: THREE.Vector3, dir: THREE.Vector3, far: number, skip: number): number {
    this.ray.origin.copy(origin);
    this.ray.direction.copy(dir);
    this.rayFar = far;
    this.raySkip = skip;
    this.rayBest = far;
    this.hitNormalY = 0;
    this.bvh.shapecast(this.rayCast);
    return this.rayBest < this.rayFar ? this.rayBest : Infinity;
  }

  /**
   * The first surface below (x, fromY, z) within `reach` metres: its height, or NaN for none. `flat` asks for one
   * that can be stood on; a steeper one gives NaN too.
   */
  floorBelow(x: number, fromY: number, z: number, reach: number, flat = false): number {
    const d = this.castRay(this.v2.set(x, fromY, z), this.down, reach, 0);
    if (!Number.isFinite(d) || (flat && Math.abs(this.hitNormalY) < FLOOR_NY)) return NaN;
    return fromY - d;
  }

  /**
   * The first surface below (x, fromY, z) for the camera to keep above: like floorBelow(), but tree crowns, trunks
   * and railings do not count (the camera looks through them), or it would hop up over every crown it passes.
   */
  cameraFloor(x: number, fromY: number, z: number, reach: number): number {
    const d = this.castRay(this.v2.set(x, fromY, z), this.down, reach, CAMERA_SKIP);
    return Number.isFinite(d) ? fromY - d : NaN;
  }

  /** The normal's y of the surface the last floorBelow() found. */
  get lastNormalY(): number {
    return this.hitNormalY;
  }

  /**
   * How far the camera can go from `origin` along `dir` (unit) before a wall, up to `far`. Foliage does not count
   * (paint.ts fades the crowns between the camera and the player instead), nor do trunks and railings (THIN).
   */
  sightDistance(origin: THREE.Vector3, dir: THREE.Vector3, far: number): number {
    return Math.min(far, this.castRay(origin, dir, far, CAMERA_SKIP));
  }

  /**
   * Every height a body could stand at above (x, z), lowest first: surfaces flat enough, with HEADROOM over them.
   * A slab gives its top only; the ground under a bridge deck or a roof is in the list too.
   */
  floorsAt(x: number, z: number): number[] {
    this.ray.origin.set(x, this.top, z);
    this.ray.direction.copy(this.down);
    const hits = this.bvh.raycast(this.ray, THREE.DoubleSide);
    hits.sort((a, b) => b.point.y - a.point.y);
    const out: number[] = [];
    let above = Infinity;
    for (const h of hits) {
      const y = h.point.y;
      if (Math.abs(h.face?.normal.y ?? 0) >= FLOOR_NY && above - y >= HEADROOM) out.push(y);
      above = y;
    }
    return out.reverse();
  }

  /**
   * The floor to stand on at (x, z) for something at about height `nearY`: the highest one at most a step above
   * it, else the lowest there is; NaN where nothing can be stood on. With nearY 0 that is the ground, not a roof
   * or a bridge deck above it.
   */
  floorNear(x: number, z: number, nearY: number): number {
    const floors = this.floorsAt(x, z);
    let best = NaN;
    for (const f of floors) if (f <= nearY + NEAR_UP) best = f;
    return Number.isFinite(best) ? best : floors.length > 0 ? floors[0] : NaN;
  }

  /** How deep the water is over a floor at height `floorY` (m; 0 or less on dry land): the sea, a lake, a fountain. */
  waterDepth(x: number, z: number, floorY: number): number {
    let depth = this.seaLevel - floorY;
    if (this.water) {
      this.wetRay.origin.set(x, floorY + 4, z);
      this.wetRay.direction.copy(this.down);
      const hit = this.water.raycastFirst(this.wetRay, THREE.DoubleSide, 0, 4.05);
      if (hit) depth = Math.max(depth, hit.point.y - floorY);
    }
    return depth;
  }

  /** Whether a floor at height `floorY` is under water too deep to wade. */
  isDeep(x: number, z: number, floorY: number): boolean {
    return this.waterDepth(x, z, floorY) > WADE;
  }

  // ---------------------------------------------------------------- capsules

  private setCapsule(x: number, y: number, z: number, radius: number, height: number, lift: number): void {
    this.seg.start.set(x, y + radius + lift, z);
    this.seg.end.set(x, y + Math.max(radius + lift, height - radius), z);
    this.segBox.makeEmpty();
    this.segBox.expandByPoint(this.seg.start);
    this.segBox.expandByPoint(this.seg.end);
    this.segBox.min.addScalar(-radius);
    this.segBox.max.addScalar(radius);
    this.capRadius = radius;
  }

  /** shapecast callbacks for collide(): push the capsule's axis out of every triangle closer than its radius */
  private readonly pushOut = {
    intersectsBounds: (box: THREE.Box3): number => (box.intersectsBox(this.segBox) ? INTERSECTED : NOT_INTERSECTED),
    intersectsTriangle: (tri: ExtendedTriangle): boolean => {
      const onTri = this.v0;
      const onAxis = this.v1;
      const d = tri.closestPointToSegment(this.seg, onTri, onAxis);
      const r = this.capRadius;
      if (d >= r) return false;
      const dir = onAxis.sub(onTri);
      if (d > 1e-6) {
        dir.multiplyScalar(1 / d);
      } else {
        // the axis runs through the triangle: out along its normal, on the side the capsule's middle is
        tri.getNormal(dir);
        this.v2.addVectors(this.seg.start, this.seg.end).multiplyScalar(0.5).sub(onTri);
        if (dir.dot(this.v2) < 0) dir.negate();
      }
      const depth = r - d;
      this.seg.start.addScaledVector(dir, depth);
      this.seg.end.addScaledVector(dir, depth);
      this.contacts |= dir.y > FLOOR_NY ? CONTACT_FLOOR : dir.y < -FLOOR_NY ? CONTACT_CEILING : CONTACT_WALL;
      this.sidePush.x += dir.x * depth;
      this.sidePush.z += dir.z * depth;
      return false;
    },
  };

  /**
   * Push a body standing at `feet` out of everything it overlaps, moving `feet`. Returns what it was pushed off
   * (CONTACT_FLOOR, CONTACT_WALL, CONTACT_CEILING bits), 0 for nothing.
   */
  collide(feet: THREE.Vector3, body: Body): number {
    this.setCapsule(feet.x, feet.y, feet.z, body.radius, body.height, 0);
    const sx = this.seg.start.x;
    const sy = this.seg.start.y;
    const sz = this.seg.start.z;
    this.contacts = 0;
    this.sidePush.set(0, 0, 0);
    this.bvh.shapecast(this.pushOut);
    this.skirt?.shapecast(this.pushOut);
    feet.x += this.seg.start.x - sx;
    feet.y += this.seg.start.y - sy;
    feet.z += this.seg.start.z - sz;
    return this.contacts;
  }

  /** shapecast callbacks for fits(): stop at the first triangle inside the capsule */
  private readonly overlap = {
    intersectsBounds: (box: THREE.Box3): number => (box.intersectsBox(this.segBox) ? INTERSECTED : NOT_INTERSECTED),
    intersectsTriangle: (tri: ExtendedTriangle): boolean =>
      tri.closestPointToSegment(this.seg, this.v0, this.v1) < this.capRadius - TOUCH,
  };

  /** Whether a body standing on (x, y, z) overlaps nothing (it may touch the floor and a wall). */
  fits(x: number, y: number, z: number, body: Body): boolean {
    this.setCapsule(x, y, z, body.radius, body.height, TOUCH);
    return !this.bvh.shapecast(this.overlap) && !this.skirt?.shapecast(this.overlap);
  }

  /** Whether nothing a body would bump into stands between two spots at SIGHT_H over their floors. */
  private clearBetween(a: THREE.Vector3, bx: number, by: number, bz: number): boolean {
    const from = this.sightFrom.set(a.x, a.y + SIGHT_H, a.z);
    const to = this.sightTo.set(bx, by + SIGHT_H, bz).sub(from);
    const d = to.length();
    if (d < 1e-6) return true;
    to.multiplyScalar(1 / d);
    if (Number.isFinite(this.castRay(from, to, d, 0))) return false;
    if (!this.skirt) return true;
    this.wetRay.origin.copy(from);
    this.wetRay.direction.copy(to);
    return this.skirt.raycastFirst(this.wetRay, THREE.DoubleSide, 0, d) === null;
  }

  /**
   * The nearest spot to (x, z), within `maxDist` metres, where a body stands on dry ground and overlaps nothing,
   * on the floor nearest the height `nearY` (see floorNear): written into `out`. False where there is none. With
   * `from` (where the body is now), only a spot it could get to from there: nothing in between, so getting off a
   * bike next to a house never puts the rider inside it.
   */
  nearestStanding(x: number, z: number, body: Body, nearY: number, maxDist: number, out: THREE.Vector3, from?: THREE.Vector3): boolean {
    for (let r = 0; r <= maxDist + 1e-6; r += SEARCH_STEP) {
      const n = r === 0 ? 1 : Math.max(6, Math.round((2 * Math.PI * r) / SEARCH_STEP));
      for (let k = 0; k < n; k++) {
        const a = (k / n) * Math.PI * 2;
        const px = x + Math.cos(a) * r;
        const pz = z + Math.sin(a) * r;
        const y = this.floorNear(px, pz, nearY);
        if (!Number.isFinite(y) || this.isDeep(px, pz, y) || !this.fits(px, y, pz, body)) continue;
        if (from && !this.clearBetween(from, px, y, pz)) continue;
        out.set(px, y, pz);
        return true;
      }
    }
    return false;
  }

  /** A wireframe of what bodies collide with, the skirts solid (the ?debug view, sgGame.showCollider()). */
  debugMesh(): THREE.Object3D {
    const root = new THREE.Group();
    root.name = 'Collider (debug)';
    const mesh = new THREE.Mesh(this.geometry, new THREE.MeshBasicMaterial({ color: 0xff2a6d, wireframe: true, transparent: true, opacity: 0.35 }));
    mesh.renderOrder = 10;
    root.add(mesh);
    const skirt = this.skirt?.geometry;
    if (skirt) {
      const walls = new THREE.Mesh(skirt, new THREE.MeshBasicMaterial({ color: 0xffb000, side: THREE.DoubleSide, transparent: true, opacity: 0.6 }));
      walls.renderOrder = 11;
      root.add(walls);
    }
    return root;
  }
}
