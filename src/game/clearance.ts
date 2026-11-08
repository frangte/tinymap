// How far each nav cell is from the nearest cell nobody can walk on: an exact Euclidean distance transform
// (Felzenszwalb & Huttenlocher, "Distance Transforms of Sampled Functions"), two passes of a 1D transform of
// squared distances. Sampled bilinearly, it makes the grid's square obstacles read round, so a body slides off
// a tree trunk instead of catching on its corners (src/game/nav.ts, NavGrid.clearance).

const FAR = 1e20;

/** The 1D squared-distance transform of f[0..n) into d (v and z are scratch). */
function transform1d(f: Float64Array, n: number, d: Float64Array, v: Int32Array, z: Float64Array): void {
  let k = 0;
  v[0] = 0;
  z[0] = -FAR;
  z[1] = FAR;
  for (let q = 1; q < n; q++) {
    let s = (f[q] + q * q - (f[v[k]] + v[k] * v[k])) / (2 * q - 2 * v[k]);
    while (s <= z[k]) {
      k--;
      s = (f[q] + q * q - (f[v[k]] + v[k] * v[k])) / (2 * q - 2 * v[k]);
    }
    k++;
    v[k] = q;
    z[k] = s;
    z[k + 1] = FAR;
  }
  k = 0;
  for (let q = 0; q < n; q++) {
    while (z[k + 1] < q) k++;
    d[q] = (q - v[k]) * (q - v[k]) + f[v[k]];
  }
}

/**
 * Per cell (row-major, nx × ny), the distance in metres from its centre to the edge of the nearest blocked cell:
 * centre to centre, less half a cell. Negative on blocked cells.
 */
export function clearanceField(walk: Uint8Array, nx: number, ny: number, cell: number): Float32Array {
  const g = new Float64Array(nx * ny);
  for (let i = 0; i < nx * ny; i++) g[i] = walk[i] === 1 ? FAR : 0;
  const m = Math.max(nx, ny);
  const f = new Float64Array(m);
  const d = new Float64Array(m);
  const v = new Int32Array(m);
  const z = new Float64Array(m + 1);
  for (let i = 0; i < nx; i++) {
    for (let j = 0; j < ny; j++) f[j] = g[j * nx + i];
    transform1d(f, ny, d, v, z);
    for (let j = 0; j < ny; j++) g[j * nx + i] = d[j];
  }
  for (let j = 0; j < ny; j++) {
    const row = j * nx;
    for (let i = 0; i < nx; i++) f[i] = g[row + i];
    transform1d(f, nx, d, v, z);
    for (let i = 0; i < nx; i++) g[row + i] = d[i];
  }
  const out = new Float32Array(nx * ny);
  for (let i = 0; i < nx * ny; i++) out[i] = Math.sqrt(g[i]) * cell - cell / 2;
  return out;
}
