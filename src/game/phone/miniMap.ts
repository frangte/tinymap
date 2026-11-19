// The phone's map: the island drawn flat (north up), its roads, named streets and landmarks, the NPCs to talk to as
// diamonds, everyone out right now as a dot in their archetype's colour, and an arrow for the player. Redrawn a few
// times a second while the app is open.
import { PLACES, placeTitle } from '../places';
import { NPC_DEFS } from '../npcs';
import { archetypeById } from '../archetypes';
import type { RemotePlayer } from '../remotePlayers';
import { L } from '../i18n/phone';
import mapLand from './mapLand.json';

const SEA = '#9FCFCB';
const LAND_FILL = '#E7DBC4';
const STREET = '#BDB4A2';
/** every other road and walk, under the named streets */
const ROAD = '#CFC6B3';
const INK = '#2B211C';
const SELF = '#B5523B';
/** dot colour of a player without an archetype */
const NONE = '#6A5A4C';
/** the NPCs' diamonds */
const NPC_MARK = '#E0B45A';
/** margin around the coast (m) */
const PAD = 6;
/** a landmark label that would come closer than this (CSS px) to one already drawn is left out */
const LABEL_GAP = 3;

type Line = ReadonlyArray<readonly [number, number]>;
const asLines = (ls: number[][][]): Line[] => ls.map((l) => l.map((q) => [q[0], q[1]] as const));
/** the land (the coast cut by the river and the canal) and all the roads and walks, from the layout (mapLand.json) */
const LAND = asLines(mapLand.land);
const ROADS = asLines([...mapLand.roads, ...mapLand.walks]);
const STREETS = PLACES.filter((p) => p.kind === 'street' && p.pts.length > 1);
const LANDMARKS = PLACES.filter((p) => p.kind === 'landmark');

const xs = LAND.flatMap((l) => l.map((q) => q[0]));
const ys = LAND.flatMap((l) => l.map((q) => q[1]));
const BOUNDS = { minX: Math.min(...xs) - PAD, maxX: Math.max(...xs) + PAD, minY: Math.min(...ys) - PAD, maxY: Math.max(...ys) + PAD };

export class MiniMap {
  readonly canvas = document.createElement('canvas');
  private scale = 1;
  private ox = 0;
  private oy = 0;

  constructor() {
    this.canvas.className = 'mini-map';
    this.canvas.setAttribute('role', 'img');
    this.canvas.setAttribute('aria-label', L.mapLabel);
  }

  /**
   * @param self three.js position and heading of the player
   * @param others the players around, in three.js metres
   */
  draw(self: { x: number; z: number; heading: number }, others: Iterable<RemotePlayer>): void {
    const dpr = Math.min(2, window.devicePixelRatio || 1);
    const w = Math.max(1, Math.round(this.canvas.clientWidth * dpr));
    const h = Math.max(1, Math.round(this.canvas.clientHeight * dpr));
    if (this.canvas.width !== w || this.canvas.height !== h) {
      this.canvas.width = w;
      this.canvas.height = h;
    }
    const g = this.canvas.getContext('2d');
    if (!g) return;
    const bw = BOUNDS.maxX - BOUNDS.minX;
    const bh = BOUNDS.maxY - BOUNDS.minY;
    this.scale = Math.min(w / bw, h / bh);
    this.ox = (w - bw * this.scale) / 2;
    this.oy = (h - bh * this.scale) / 2;

    g.fillStyle = SEA;
    g.fillRect(0, 0, w, h);
    g.lineJoin = 'round';
    g.lineCap = 'round';
    for (const c of LAND) {
      this.path(g, c, true);
      g.fillStyle = LAND_FILL;
      g.fill();
      g.strokeStyle = INK;
      g.lineWidth = 1.2 * dpr;
      g.stroke();
    }
    g.strokeStyle = ROAD;
    g.lineWidth = 2 * dpr;
    for (const r of ROADS) {
      this.path(g, r, false);
      g.stroke();
    }
    g.strokeStyle = STREET;
    g.lineWidth = 3.5 * dpr;
    for (const s of STREETS) {
      this.path(g, s.pts, false);
      g.stroke();
    }
    g.font = `600 ${9 * dpr}px 'Be Vietnam Pro', system-ui, sans-serif`;
    g.textAlign = 'center';
    /** the boxes of the labels drawn so far: x0, y0, x1, y1 */
    const labelled: Array<[number, number, number, number]> = [];
    for (const l of LANDMARKS) {
      const [x, y] = this.px(l.pts[0][0], l.pts[0][1]);
      g.fillStyle = INK;
      g.beginPath();
      g.arc(x, y, 2.5 * dpr, 0, Math.PI * 2);
      g.fill();
      // landmarks listed first win: a label goes above its dot, else below, right or left, else is left out
      const title = placeTitle(l);
      const w = g.measureText(title).width;
      const gap = LABEL_GAP * dpr;
      const th = 10 * dpr;
      const spots: Array<[number, number]> = [
        [x - w / 2, y - 5 * dpr - th],
        [x - w / 2, y + 5 * dpr],
        [x + 5 * dpr, y - th / 2],
        [x - 5 * dpr - w, y - th / 2],
      ];
      const spot = spots.find(([sx, sy]) => !labelled.some(([a0, b0, a1, b1]) => sx - gap < a1 && a0 < sx + w + gap && sy - gap < b1 && b0 < sy + th + gap));
      if (!spot) continue;
      labelled.push([spot[0], spot[1], spot[0] + w, spot[1] + th]);
      g.textAlign = 'left';
      g.textBaseline = 'top';
      g.fillText(title, spot[0], spot[1]);
    }
    g.textBaseline = 'alphabetic';
    // NPCs stand at their spots (they only turn round), so the shared list says where they are
    g.fillStyle = NPC_MARK;
    g.strokeStyle = INK;
    g.lineWidth = dpr;
    for (const n of NPC_DEFS) {
      const [x, y] = this.px(n.at[0], n.at[1]);
      const r = 4 * dpr;
      g.beginPath();
      g.moveTo(x, y - r);
      g.lineTo(x + r, y);
      g.lineTo(x, y + r);
      g.lineTo(x - r, y);
      g.closePath();
      g.fill();
      g.stroke();
    }
    for (const r of others) {
      // the server only says where the players near us are
      if (!r.inView) continue;
      // three.js (x, z) is Blender (x, -z)
      const [x, y] = this.px(r.position.x, -r.position.z);
      g.fillStyle = archetypeById(r.arch)?.badge.color ?? NONE;
      g.strokeStyle = INK;
      g.lineWidth = dpr;
      g.beginPath();
      g.arc(x, y, 3.5 * dpr, 0, Math.PI * 2);
      g.fill();
      g.stroke();
    }
    this.arrow(g, self, dpr);
  }

  private px(bx: number, by: number): [number, number] {
    return [this.ox + (bx - BOUNDS.minX) * this.scale, this.oy + (BOUNDS.maxY - by) * this.scale];
  }

  private path(g: CanvasRenderingContext2D, pts: ReadonlyArray<readonly [number, number]>, close: boolean): void {
    g.beginPath();
    pts.forEach(([bx, by], i) => {
      const [x, y] = this.px(bx, by);
      if (i === 0) g.moveTo(x, y);
      else g.lineTo(x, y);
    });
    if (close) g.closePath();
  }

  /** the player: a triangle pointing where they face (heading 0 faces three +Z, which is Blender -Y: south) */
  private arrow(g: CanvasRenderingContext2D, self: { x: number; z: number; heading: number }, dpr: number): void {
    const [x, y] = this.px(self.x, -self.z);
    const angle = Math.atan2(Math.cos(self.heading), Math.sin(self.heading));
    g.save();
    g.translate(x, y);
    g.rotate(angle);
    g.beginPath();
    g.moveTo(8 * dpr, 0);
    g.lineTo(-5 * dpr, 5 * dpr);
    g.lineTo(-2 * dpr, 0);
    g.lineTo(-5 * dpr, -5 * dpr);
    g.closePath();
    g.fillStyle = SELF;
    g.strokeStyle = INK;
    g.lineWidth = 1.2 * dpr;
    g.fill();
    g.stroke();
    g.restore();
  }
}
