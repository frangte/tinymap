// Named places on the map (Blender XY, metres). The HUD shows where the player is; landmarks also pop a
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

export const PLACES: readonly Place[] = [
  { name: 'Chợ Bến Thành', note: 'xây năm 1914', kind: 'landmark', pts: [[8, 8]], r: 25 },
  { name: 'Nhà hát Thành phố', note: 'xây năm 1900', kind: 'landmark', pts: [[65.4, -68.5]], r: 19 },
  { name: 'Nhà thờ Đức Bà', note: 'xây năm 1880', kind: 'landmark', pts: [[98, -5]], r: 23 },
  { name: 'Bưu điện Thành phố', note: 'xây năm 1891', kind: 'landmark', pts: [[94.9, -41.8]], r: 17 },
  { name: 'Hồ Con Rùa', note: 'công trường Quốc tế', kind: 'landmark', pts: [[91.5, 22.4]], r: 15 },
  { name: 'Bitexco', note: 'tháp tài chính, 2010', kind: 'landmark', pts: [[-15.5, -53.5]], r: 21 },
  { name: 'Cầu Ba Son', note: 'cầu dây văng, 2022', kind: 'landmark', pts: [[95, -88]], r: 15 },
  { name: 'Công xã Paris', note: 'tượng Đức Mẹ', kind: 'landmark', pts: [[79.7, -25.8]], r: 9 },
  { name: 'Phố đi bộ Nguyễn Huệ', note: 'phố đi bộ', kind: 'street', pts: [[-5.8, -85.1], [30.5, -43.8], [46.4, -25.8]], r: 8.5 },
  { name: 'Đường Lê Lợi', note: 'Quận 1', kind: 'street', pts: [[8, -24], [49.3, -54.3]], r: 8 },
  { name: 'Đường Đồng Khởi', note: 'Quận 1', kind: 'street', pts: [[17.7, -90.2], [49.3, -54.3], [79.7, -25.8]], r: 7 },
  { name: 'Đường Hàm Nghi', note: 'Quận 1', kind: 'street', pts: [[8, -24], [8, -69.4]], r: 7.5 },
  { name: 'Đường Lê Lai', note: 'Quận 1', kind: 'street', pts: [[8, -24], [-70, -24]], r: 7.5 },
  { name: 'Đường Lê Thánh Tôn', note: 'Quận 1', kind: 'street', pts: [[-13.5, 31], [30, 31.5], [52, 30], [68, 26.5], [91.5, 22.4]], r: 6.5 },
  { name: 'Đường Tôn Đức Thắng', note: 'ven sông Sài Gòn', kind: 'street', pts: [[-92.6, -65.7], [-44.6, -74.9], [5.6, -87.6], [47.9, -92.3], [85.1, -76.1], [125.6, -41.6], [169.3, -15.4]], r: 7 },
  { name: 'Bùng binh Quách Thị Trang', note: 'trước chợ Bến Thành', kind: 'street', pts: [[8, -24]], r: 13 },
  { name: 'Thủ Thiêm', note: 'bờ đông sông Sài Gòn', kind: 'area', pts: [[112, -112]], r: 40 },
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
