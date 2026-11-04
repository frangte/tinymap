// Named places on the v4 island (Blender XY, metres). The HUD shows where the player is; landmarks also pop a
// toast the first time the player walks up to them in a visit.

export type PlaceKind = 'landmark' | 'street' | 'area';

export interface Place {
  name: string;
  /** small print under the name (year built, kind of street…) */
  note: string;
  kind: PlaceKind;
  /** a point, or a polyline for streets */
  pts: ReadonlyArray<readonly [number, number]>;
  /** reach in metres from the point / line */
  r: number;
}

/** the grass edge of the island (simplified), for the beach label */
const COAST: ReadonlyArray<readonly [number, number]> = [
  [-65.1, -7.6], [-56.2, -4.7], [-45.2, 7.1], [-43.5, 37.0], [-27.8, 69.4], [-13.1, 80.5], [13.7, 87.1], [21.5, 105.6],
  [32.9, 115.1], [46.2, 117.3], [64.5, 110.1], [90.6, 109.4], [96.0, 103.1], [99.3, 82.7], [115.7, 72.4], [116.6, 37.2],
  [125.6, 9.2], [125.5, -17.3], [101.6, -39.6], [100.6, -64.7], [90.9, -75.8], [68.7, -73.5], [56.7, -83.6], [47.3, -85.6],
  [4.1, -71.4], [-31.9, -43.1], [-63.4, -39.5], [-71.4, -19.4], [-65.1, -7.6],
];

export const PLACES: readonly Place[] = [
  { name: 'Chợ Bến Thành', note: 'xây năm 1914', kind: 'landmark', pts: [[8, 8]], r: 25 },
  { name: 'Nhà hát Thành phố', note: 'xây năm 1900', kind: 'landmark', pts: [[106, -4]], r: 16 },
  { name: 'Quảng trường Lam Sơn', note: 'trước Nhà hát', kind: 'landmark', pts: [[89, -4]], r: 8 },
  { name: 'Nhà thờ Đức Bà', note: 'xây năm 1880', kind: 'landmark', pts: [[78, 82]], r: 20 },
  { name: 'Bưu điện Thành phố', note: 'xây năm 1891', kind: 'landmark', pts: [[100, 54]], r: 15 },
  { name: 'Công xã Paris', note: 'tượng Đức Mẹ', kind: 'landmark', pts: [[78, 50]], r: 10 },
  { name: 'Hồ Con Rùa', note: 'công trường Quốc tế', kind: 'landmark', pts: [[44, 90]], r: 16 },
  { name: 'Bitexco', note: 'tháp tài chính, 2010', kind: 'landmark', pts: [[44, -62]], r: 18 },
  { name: 'Dãy nhà phố', note: 'đường Lê Lai', kind: 'landmark', pts: [[-32, -12]], r: 13 },
  { name: 'Phố đi bộ Nguyễn Huệ', note: 'đầu phố để dành cho UBND', kind: 'street', pts: [[60, 22], [66, -58]], r: 8.5 },
  { name: 'Đường Lê Lợi', note: 'chợ ra Nhà hát', kind: 'street', pts: [[8, -24], [44, -15], [81, -6]], r: 7.5 },
  { name: 'Đường Hàm Nghi', note: 'chợ ra biển', kind: 'street', pts: [[8, -24], [36, -37], [62, -50]], r: 7.5 },
  { name: 'Đường Lê Lai', note: 'về phía Bùi Viện', kind: 'street', pts: [[8, -24], [-40, -24.5], [-70, -25]], r: 7.5 },
  { name: 'Đường Lê Thánh Tôn', note: 'sau chợ', kind: 'street', pts: [[-14, 34], [30, 35], [80, 34.5]], r: 6.5 },
  { name: 'Đường Đồng Khởi', note: 'Nhà thờ ra biển', kind: 'street', pts: [[78, 50], [80, 10], [84, -60]], r: 6.5 },
  { name: 'Đường Phạm Ngọc Thạch', note: 'Nhà thờ lên Hồ Con Rùa', kind: 'street', pts: [[78, 50], [62, 60], [52, 74], [44, 90]], r: 6 },
  { name: 'Công viên 23/9', note: 'dọc Lê Lai', kind: 'area', pts: [[-28, -40]], r: 14 },
  { name: 'Bùng binh Quách Thị Trang', note: 'trước chợ Bến Thành', kind: 'street', pts: [[8, -24]], r: 13 },
  { name: 'Bãi biển', note: 'quanh đảo', kind: 'area', pts: COAST, r: 12 },
];

const RANK: Record<PlaceKind, number> = { landmark: 0, street: 1, area: 2 };

function segDist(px: number, py: number, ax: number, ay: number, bx: number, by: number): number {
  const vx = bx - ax;
  const vy = by - ay;
  const l2 = vx * vx + vy * vy;
  const t = l2 > 0 ? Math.max(0, Math.min(1, ((px - ax) * vx + (py - ay) * vy) / l2)) : 0;
  return Math.hypot(px - (ax + vx * t), py - (ay + vy * t));
}

function distance(p: Place, x: number, y: number): number {
  const pts = p.pts;
  if (pts.length === 1) return Math.hypot(x - pts[0][0], y - pts[0][1]);
  let d = Infinity;
  for (let i = 1; i < pts.length; i++) d = Math.min(d, segDist(x, y, pts[i - 1][0], pts[i - 1][1], pts[i][0], pts[i][1]));
  return d;
}

/** The place the Blender point (x, y) is in: landmarks first, then streets, then areas; null if none. */
export function placeAt(x: number, y: number, current: Place | null): Place | null {
  let best: Place | null = null;
  let bestScore = Infinity;
  for (const p of PLACES) {
    // a little hysteresis so the label does not flicker on a boundary
    const reach = p === current ? p.r + 3 : p.r;
    const d = distance(p, x, y);
    if (d > reach) continue;
    const score = RANK[p.kind] * 10 + d / reach;
    if (score < bestScore) {
      bestScore = score;
      best = p;
    }
  }
  return best;
}
