// Named places on the v4 island (Blender XY, metres). The HUD shows where the player is; landmarks also pop a
// toast the first time the player walks up to them in a visit. The list lives in server/content/places.json, which
// the Go server embeds too (and whose tests keep it valid), so the game and the server agree on every place.
import data from '../../server/content/places.json';
import { lang, tx, type Bilingual } from './i18n';

export type PlaceKind = 'landmark' | 'street' | 'area';

export interface Place {
  /** stable id (quests refer to places by it) */
  id: string;
  /** Vietnamese and English (show it with placeTitle) */
  name: Bilingual;
  /** small print under the name (year built, kind of street…); show it with placeNote */
  note: Bilingual;
  kind: PlaceKind;
  /** a point, or a polyline for streets and the coast */
  pts: ReadonlyArray<readonly [number, number]>;
  /** reach in metres from the point / line */
  r: number;
}

const KINDS: readonly PlaceKind[] = ['landmark', 'street', 'area'];

export const PLACES: readonly Place[] = data.places.map((p): Place => {
  const kind = KINDS.find((k) => k === p.kind);
  if (!kind || p.pts.some((pt) => pt.length !== 2)) throw new Error(`server/content/places.json: bad place "${p.id}"`);
  return { id: p.id, name: p.name, note: p.note, kind, pts: p.pts.map((pt) => [pt[0], pt[1]] as const), r: p.r };
});

/** The place's name in the player's language. */
export function placeTitle(p: Pick<Place, 'name'>): string {
  return tx(p.name);
}

/**
 * The small print under the name. In English it starts with the Vietnamese name, the one on the street signs:
 * "Chợ Bến Thành · built in 1914".
 */
export function placeNote(p: Pick<Place, 'name' | 'note'>): string {
  const note = tx(p.note);
  if (lang() === 'vi' || !p.name.en || p.name.en === p.name.vi) return note;
  return note ? `${p.name.vi} · ${note}` : p.name.vi;
}

/** The place with this id; undefined for ids this build does not know. */
export function placeById(id: string): Place | undefined {
  return PLACES.find((p) => p.id === id);
}

const RANK: Record<PlaceKind, number> = { landmark: 0, street: 1, area: 2 };

function segDist(px: number, py: number, ax: number, ay: number, bx: number, by: number): number {
  const vx = bx - ax;
  const vy = by - ay;
  const l2 = vx * vx + vy * vy;
  const t = l2 > 0 ? Math.max(0, Math.min(1, ((px - ax) * vx + (py - ay) * vy) / l2)) : 0;
  return Math.hypot(px - (ax + vx * t), py - (ay + vy * t));
}

/** How far the Blender point (x, y) is from a place: its point, or the nearest point of its line (m). */
export function distance(p: Place, x: number, y: number): number {
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
