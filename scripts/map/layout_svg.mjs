// Draws the Sài Gòn mini plan from scripts/map/saigon_mini_layout.json, the file the Blender blockout is built from
// (scripts/blender/build_saigon_mini.py), so the drawing and the model never disagree.
//
//   node scripts/map/layout_svg.mjs [docs/img/saigon_mini_layout.svg]
//
// 1 SVG unit = 1 m, north up; colours are the game's PAL (docs/PROJECT.md).
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.resolve(HERE, '..', '..');
const OUT = path.resolve(process.argv[2] ?? path.join(ROOT, 'docs', 'img', 'saigon_mini_layout.svg'));
// a second argument draws another layout file (a proposal) instead of the current one
const SRC = process.argv[3] ? path.resolve(process.argv[3]) : path.join(HERE, 'saigon_mini_layout.json');
const L = JSON.parse(fs.readFileSync(SRC, 'utf8'));

const P = {
  ink: '#2B211C', grass: '#86A85E', grassDark: '#6F9450', sand: '#CDB98F', paving: '#BDB4A2', curb: '#D3CBB8',
  asphalt: '#6F6D69', roadLine: '#ECE6D4', water: '#6FA9B6', deep: '#3F8399', shallow: '#86CBC6',
  cream: '#EAD9AE', creamLight: '#F4EBD3', wallGrey: '#A0A7AA', wallTeal: '#5E9A96', wallOchre: '#C9A25E',
  wallBeige: '#CFC1A0', wallPink: '#D29A86', opWhite: '#EEE7D7', poYellow: '#E0B45A', brick: '#B5523B',
  stone: '#E6DCC8', concrete: '#ABA69C', roofRed: '#A9492F', slate: '#646B74', tinBlue: '#8AA3B6',
  turq: '#5BA9A1', glassBlue: '#7FA0B5', glassTeal: '#4F7A7C', bridgeWhite: '#EEF0EE', statueW: '#F2EFE8',
  leaf: '#6E9B4C', leafDark: '#4D7539', leafLight: '#93B85F',
  wallCream: '#EFE3C8', wallMint: '#A9CDBB', wallSky: '#9DBFD2',
  sigRed: '#C8553D', sigAmber: '#E3A33B', sigGreen: '#5FA77A',
};
const FONT = 'font-family="Be Vietnam Pro, system-ui, sans-serif"';
const out = [];
const add = (s) => out.push(s);
const rect = (r, fill, extra = '') => add(`<rect x="${r[0]}" y="${r[1]}" width="${r[2] - r[0]}" height="${r[3] - r[1]}" fill="${fill}" ${extra}/>`);
const stroke = (d, color, w, extra = '') =>
  add(`<path d="${d}" fill="none" stroke="${color}" stroke-width="${w}" stroke-linecap="round" stroke-linejoin="round" ${extra}/>`);
const text = (x, y, s, size, extra = '') => add(`<text x="${x}" y="${y}" font-size="${size}" ${FONT} ${extra}>${s}</text>`);
const roundedRect = (r, rad) => {
  const [x0, y0, x1, y1] = r;
  return `M ${x0 + rad},${y0} H ${x1 - rad} A ${rad} ${rad} 0 0 1 ${x1},${y0 + rad} V ${y1 - rad} A ${rad} ${rad} 0 0 1 ${x1 - rad},${y1} H ${x0 + rad} A ${rad} ${rad} 0 0 1 ${x0},${y1 - rad} V ${y0 + rad} A ${rad} ${rad} 0 0 1 ${x0 + rad},${y0} Z`;
};

// each strip is one house frontage, in the house-kit colours (docs/map-v5.md)
function stripes(id, w, colors, h = 120) {
  const r = colors.map((c, i) => `<rect x="${i * w}" y="0" width="${w}" height="${h}" fill="${c}"/>` +
    `<rect x="${i * w}" y="0" width="0.35" height="${h}" fill="${P.ink}" opacity="0.35"/>`).join('');
  return `<pattern id="${id}" width="${w * colors.length}" height="${h}" patternUnits="userSpaceOnUse">${r}</pattern>`;
}
const defs = [
  stripes('np', 4.5, [P.wallPink, P.cream, P.wallMint, P.opWhite, P.wallSky, P.poYellow, P.wallTeal, P.creamLight]),
  stripes('hem', 3.5, [P.wallBeige, P.cream, P.wallMint, P.wallGrey, P.wallPink, P.creamLight, P.wallSky]),
  stripes('bar', 4.5, [P.poYellow, P.wallTeal, P.roofRed, P.wallPink, P.wallMint, P.brick]),
  stripes('cl', 4.5, [P.wallOchre, P.poYellow, P.brick, P.cream, P.wallOchre, P.roofRed]),
  stripes('ks', 6, [P.opWhite, P.wallSky, P.creamLight, P.wallMint, P.opWhite, P.cream]),
  `<clipPath id="land"><path d="${L.coast}"/></clipPath>`,
  `<clipPath id="inland"><path d="${L.inland ?? L.coast}"/></clipPath>`,
].join('');

// sea, shallows, land, sea wall, beaches, rocks
rect([-20, -20, 520, 495], P.deep);
add(`<path d="${L.coast}" fill="none" stroke="${P.shallow}" stroke-width="22" stroke-linejoin="round"/>`);
add(`<path d="${L.coast}" fill="${P.grass}" stroke="${P.ink}" stroke-width="0.8"/>`);
add('<g clip-path="url(#land)">');
stroke(L.coast, P.curb, 10);
for (const b of L.beaches) stroke(b.path, P.sand, b.width);
add('</g>');
for (const [x, y, r] of L.rocks) add(`<circle cx="${x}" cy="${y}" r="${r}" fill="${P.concrete}" stroke="${P.ink}" stroke-width="0.5"/>`);

// river (both ends in the sea) and promenades
stroke(L.river.path, P.shallow, L.river.shallows);
for (const c of L.canals ?? []) stroke(c.path, P.shallow, c.shallows ?? c.width + 6);
stroke(L.river.path, P.water, L.river.width);
for (const c of L.canals ?? []) stroke(c.path, P.water, c.width);
for (const p of L.promenades) stroke(p.path, p.kind === 'cycle' ? P.sigGreen : P.paving, p.w);

// mountains
const greens = [P.leafDark, P.leaf, P.leafLight];
for (const m of L.mountains) {
  add(`<path d="${m.outline}" fill="${P.leafDark}" stroke="${P.ink}" stroke-width="0.6"${m.optional ? ' stroke-dasharray="3 2"' : ''}/>`);
  for (const [px, py, , pr] of m.peaks) {
    for (let k = 1; k < 3; k++) add(`<circle cx="${px}" cy="${py}" r="${(pr * (3 - k)) / 2.4}" fill="${greens[k]}" opacity="0.75"/>`);
    add(`<circle cx="${px}" cy="${py}" r="6" fill="${P.concrete}" stroke="${P.ink}" stroke-width="0.5"/>`);
  }
  if (m.stairs) stroke(m.stairs, P.stone, 2.2, 'stroke-dasharray="2.4 1.4"');
}

// house blocks: grid cells and extra lots (only on the land inside the coastal road)
add('<g clip-path="url(#inland)">');
const inset = L.grid.inset;
for (const cell of L.grid.cells) {
  for (let i = 0; i < cell.x.length - 1; i++) {
    for (let j = 0; j < cell.y.length - 1; j++) {
      rect([cell.x[i] + inset, cell.y[j] + inset, cell.x[i + 1] - inset, cell.y[j + 1] - inset], `url(#${cell.kind})`, `stroke="${P.ink}" stroke-width="0.5"`);
    }
  }
}
for (const lot of L.lots) rect(lot.rect, `url(#${lot.kind})`, `stroke="${P.ink}" stroke-width="0.5"`);
add('</g>');

// parks, lakes, plazas (on land only)
add('<g clip-path="url(#land)">');
for (const pk of L.parks) {
  const fill = pk.kind === 'flowers' ? P.sigAmber : P.grassDark;
  if (pk.r) add(`<path d="${roundedRect(pk.rect, pk.r)}" fill="${fill}"/>`);
  else rect(pk.rect, fill, pk.kind === 'flowers' ? 'opacity="0.8"' : '');
  if (pk.kind === 'trees') for (let x = pk.rect[0] + 3; x < pk.rect[2]; x += 6) for (let y = pk.rect[1] + 3; y < pk.rect[3]; y += 6) add(`<circle cx="${x}" cy="${y}" r="2.2" fill="${P.leaf}"/>`);
}
for (const lk of L.lakes) {
  const [cx, cy, rx, ry] = lk.ellipse;
  add(`<ellipse cx="${cx}" cy="${cy}" rx="${rx}" ry="${ry}" fill="${P.water}" stroke="${P.ink}" stroke-width="0.5"/>`);
  if (lk.fountain) add(`<circle cx="${cx}" cy="${cy}" r="3" fill="${P.stone}" stroke="${P.ink}" stroke-width="0.4"/><circle cx="${cx}" cy="${cy}" r="1.2" fill="${P.shallow}"/>`);
}
for (const pz of L.plazas ?? []) rect(pz, P.paving);
add('</g>');

// roads (ink edge, asphalt, lane dashes), loops, walking streets
const roads = L.roads;
for (const r of roads) stroke(r.path, P.ink, r.w + 1.2);
for (const lp of L.loops) add(`<path d="${roundedRect(lp.rect, lp.r)}" fill="none" stroke="${P.ink}" stroke-width="${lp.w + 1.2}"/>`);
for (const r of roads) stroke(r.path, P.asphalt, r.w);
for (const lp of L.loops) add(`<path d="${roundedRect(lp.rect, lp.r)}" fill="none" stroke="${P.asphalt}" stroke-width="${lp.w}"/>`);
for (const r of roads) stroke(r.path, P.roadLine, 0.5, 'stroke-dasharray="3 3"');
for (const w of L.walks) {
  stroke(w.path, P.ink, w.w + 1.2);
  stroke(w.path, P.paving, w.w);
}
const nh = L.walks.find((w) => w.planters);
if (nh) {
  const [x0, y] = nh.path.match(/-?\d+(?:\.\d+)?/g).map(Number);
  for (let x = x0 + 4; x < 282; x += 14) add(`<rect x="${x}" y="${y - 6}" width="6" height="2.4" fill="${P.leaf}"/><rect x="${x}" y="${y + 3.6}" width="6" height="2.4" fill="${P.leaf}"/>`);
}

// bridges
for (const b of L.bridges) {
  if (b.optional) {
    stroke(b.path, P.bridgeWhite, b.w - 2, 'stroke-dasharray="4 3" opacity="0.85"');
    continue;
  }
  stroke(b.path, P.ink, b.w);
  stroke(b.path, P.bridgeWhite, b.w - 1.4);
  if (b.pylon) {
    const [px, py] = b.pylon;
    stroke(`M ${px},${py + 6} Q ${px + 6},${py - 14} ${px - 4},${py - 20}`, P.ink, 2.6);
    stroke(`M ${px},${py + 6} Q ${px + 6},${py - 14} ${px - 4},${py - 20}`, P.bridgeWhite, 1.6);
    for (const dx of [-26, -20, -14, -8]) stroke(`M ${px - 3},${py - 18} L ${px + dx},${py}`, P.ink, 0.35);
  }
}

// piers (layout v8)
for (const pr of L.piers ?? []) {
  stroke(pr.path, P.ink, pr.w + 0.8);
  stroke(pr.path, '#8A5E3C', pr.w);
}

// roundabouts
for (const ra of L.roundabouts) {
  const [x, y] = ra.c;
  add(`<circle cx="${x}" cy="${y}" r="${ra.r + 5}" fill="${P.asphalt}" stroke="${P.ink}" stroke-width="0.6"/>`);
  add(`<circle cx="${x}" cy="${y}" r="${ra.r}" fill="${P[ra.inner] ?? P.paving}" stroke="${P.ink}" stroke-width="0.6"/>`);
}

// landmarks: footprints to scale, special symbols for the ones without one
const SPECIAL = {
  ho_con_rua: ([x, y]) => add(`<circle cx="${x}" cy="${y}" r="8" fill="${P.water}" stroke="${P.turq}" stroke-width="1.6"/><circle cx="${x}" cy="${y}" r="2" fill="${P.stone}"/>`),
  cong_xa_paris: ([x, y]) => add(`<circle cx="${x}" cy="${y}" r="2.2" fill="${P.statueW}" stroke="${P.ink}" stroke-width="0.5"/>`),
  bung_binh_quach_thi_trang: ([x, y]) => add(`<circle cx="${x}" cy="${y}" r="2.2" fill="${P.statueW}" stroke="${P.ink}" stroke-width="0.5"/>`),
  tuong_chua: ([x, y]) => add(`<g transform="translate(${x},${y})"><rect x="-1" y="-4" width="2" height="8" fill="${P.statueW}" stroke="${P.ink}" stroke-width="0.4"/><rect x="-5" y="-2.6" width="10" height="1.6" fill="${P.statueW}" stroke="${P.ink}" stroke-width="0.4"/></g>`),
  hai_dang: ([x, y]) => add(`<circle cx="${x}" cy="${y}" r="2.6" fill="${P.opWhite}" stroke="${P.ink}" stroke-width="0.5"/><circle cx="${x}" cy="${y}" r="1" fill="${P.roofRed}"/>`),
  hon_ba: ([x, y]) => {
    add(`<circle cx="${x}" cy="${y}" r="7" fill="${P.concrete}" stroke="${P.ink}" stroke-width="0.6"/>`);
    rect([x - 2.5, y - 2.5, x + 2.5, y + 2.5], P.roofRed, `stroke="${P.ink}" stroke-width="0.3"`);
    // the low-tide path to the shore (layout tideTo), north by default as in v2
    const lm = L.landmarks.find((m) => m.id === 'hon_ba');
    const [tx, ty] = lm.tideTo ?? [x, y - 24];
    const dl = Math.hypot(tx - x, ty - y);
    stroke(`M ${tx},${ty} L ${x + ((tx - x) / dl) * 8},${y + ((ty - y) / dl) * 8}`, P.sand, 2.4, 'stroke-dasharray="2 1.6"');
  },
  // Bùi Viện (v4 street block, scripts/blender/mini/bui_vien.py): two rows of bars either side of the walk, the gate
  bui_vien: ([x, y]) => {
    const lm = L.landmarks.find((m) => m.id === 'bui_vien');
    const [w, d] = lm.size ?? [52, 34];
    const cols = [P.wallOchre, P.wallTeal, P.sigRed, P.cream, P.wallMint, P.wallPink, P.poYellow, P.wallSky];
    for (const [y0, y1] of [[y - d / 2, y - 7.6], [y + 7.6, y + d / 2]]) {
      let k = 0;
      for (let x0 = x - w / 2; x0 < x + w / 2 - 0.5; x0 += 4.8, k++) {
        rect([x0, y0, Math.min(x0 + 4.8, x + w / 2), y1], cols[k % cols.length], `stroke="${P.ink}" stroke-width="0.25"`);
      }
    }
    for (let x0 = x - w / 2 + 3; x0 < x + w / 2; x0 += 6) stroke(`M ${x0},${y - 7.6} L ${x0 + 3},${y + 7.6}`, P.poYellow, 0.3, 'opacity="0.8"');
    rect([x + w / 2 - 2.6, y - 7.8, x + w / 2 - 1.4, y + 7.8], P.sigRed, `stroke="${P.ink}" stroke-width="0.4"`);
  },
};
const MODEL_FILL = { me_linh: [P.paving, P.statueW], ben_bach_dang: [P.stone, null], bitexco: [P.glassBlue, P.glassTeal] };
for (const lm of L.landmarks) {
  if (SPECIAL[lm.id] && lm.at) { SPECIAL[lm.id](lm.at); continue; }
  if (!lm.at || !lm.size) continue;
  const [cx, cy] = lm.at;
  const [w, d] = lm.size;
  const r = [cx - w / 2, cy - d / 2, cx + w / 2, cy + d / 2];
  const [fill, roof] = MODEL_FILL[lm.id] ?? [P[lm.color] ?? P.concrete, P[lm.roof]];
  if (lm.id === 'bitexco') {
    add(`<rect x="${r[0]}" y="${r[1]}" width="${w}" height="${d}" rx="8" fill="${P.paving}" stroke="${P.ink}" stroke-width="0.6"/>`);
    add(`<ellipse cx="${cx}" cy="${cy}" rx="8" ry="6" fill="${fill}" stroke="${P.ink}" stroke-width="0.7"/>`);
    continue;
  }
  rect(r, fill, `stroke="${P.ink}" stroke-width="0.7"${lm.status === 'optional' ? ' stroke-dasharray="2 1.5"' : ''}`);
  if (roof) rect([r[0] + 2, r[1] + 2, r[2] - 2, r[3] - 2], roof, 'opacity="0.55"');
  if (lm.id === 'me_linh') add(`<circle cx="${cx}" cy="${cy}" r="2.4" fill="${P.statueW}" stroke="${P.ink}" stroke-width="0.5"/>`);
  // the facade side, so the drawing says which way a building faces
  const f = { E: [r[2] - 1.2, cy - d * 0.3, r[2], cy + d * 0.3], W: [r[0], cy - d * 0.3, r[0] + 1.2, cy + d * 0.3], N: [cx - w * 0.3, r[1], cx + w * 0.3, r[1] + 1.2], S: [cx - w * 0.3, r[3] - 1.2, cx + w * 0.3, r[3]] }[lm.face ?? 'S'];
  if (lm.id !== 'ben_bach_dang') rect(f, P.ink, 'opacity="0.75"');
}
for (const [x0, y0, x1, y1] of L.towers) rect([x0, y0, x1, y1], P.glassTeal, `stroke="${P.ink}" stroke-width="0.7"`), rect([x0 + 2, y0 + 2, x1 - 2, y1 - 2], P.glassBlue, 'opacity="0.55"');

// boats, umbrellas, the hydrofoil route
const BOATS = L.boats ?? [[118, 402, -20], [134, 406, 10], [152, 410, -5], [126, 414, 25], [162, 418, -15], [312, 172, 90]];
for (const [x, y, a] of BOATS) {
  add(`<g transform="translate(${x},${y}) rotate(${a})"><path d="M -4,0 Q 0,-2.2 4,0 Q 0,2.2 -4,0 Z" fill="${P.wallTeal}" stroke="${P.ink}" stroke-width="0.4"/><rect x="-1" y="-0.6" width="2" height="1.2" fill="${P.roofRed}"/></g>`);
}
const UMBRELLAS = L.umbrellas ?? Array.from({ length: 17 }, (_, k) => [270 + 9 * k, 411 + (((270 + 9 * k) * 7) % 5)]);
UMBRELLAS.forEach(([x, y], k) => {
  const c = [P.sigRed, P.poYellow, P.wallSky, P.wallMint][k % 4];
  add(`<circle cx="${x}" cy="${y}" r="1.6" fill="${c}" stroke="${P.ink}" stroke-width="0.3"/>`);
});
stroke(L.hydrofoil, P.creamLight, 0.9, 'stroke-dasharray="1.5 2.5" opacity="0.9"');
// the sightseeing loop (layout v8 'tour')
if (L.tour) {
  stroke(L.tour, P.creamLight, 3.6, 'stroke-linejoin="round" opacity="0.9"');
  stroke(L.tour, '#E3703B', 2.0, 'stroke-dasharray="4 2.4" stroke-linejoin="round"');
}

// traffic lights
for (const [x, y] of L.signals) {
  add(`<g transform="translate(${x + 3.5},${y - 6.5})"><rect x="-1.6" y="-3.6" width="3.2" height="7.2" rx="0.8" fill="${P.ink}"/>` +
    `<circle cx="0" cy="-2.1" r="0.85" fill="${P.sigRed}"/><circle cx="0" cy="0" r="0.85" fill="${P.sigAmber}"/><circle cx="0" cy="2.1" r="0.85" fill="${P.sigGreen}"/></g>`);
}

// names
const halo = `fill="${P.ink}" style="paint-order:stroke" stroke="${P.roadLine}" stroke-width="0.8"`;
for (const r of [...roads, ...L.walks]) {
  if (!r.name || !r.label) continue;
  const [x, y] = r.label;
  if (r.vertical) text(x, y, r.name, 3.6, `${halo} transform="rotate(-90 ${x} ${y})"`);
  else text(x, y, r.name, 3.6, halo);
}
const area = `fill="${P.ink}" font-weight="700" letter-spacing="1.2" fill-opacity="0.75"`;
// district names: the layout's 'areas' (v7) or the v2 places
const AREAS = L.areas ?? [['NHÀ PHỐ &amp; HẺM', 22, 232, 6], ['CHỢ LỚN', 22, 328, 6], ['THỦ THIÊM', L.inland ? 372 : 392, L.inland ? 124 : 96, 7],
  ['VŨNG TÀU', 210, 348, 7], ['TRUNG TÂM', 200, 60, 6]];
for (const [s, x, y, size] of AREAS) text(x, y, s, size, area);
text(318, 76, 'SÔNG SÀI GÒN', 6, `fill="${P.creamLight}" font-weight="700" letter-spacing="1.5" transform="rotate(90 318 76)"`);
const [sx, sy] = L.seaLabel ?? [330, 486];
text(sx, sy, 'BIỂN VŨNG TÀU', 8, `fill="${P.creamLight}" font-weight="700" letter-spacing="2" fill-opacity="0.85"`);
text(468, 4, 'BIỂN', 5, `fill="${P.creamLight}" font-weight="700" letter-spacing="1"`);

// numbered markers (the key is in docs/map-v5.md and the review page)
for (const lm of L.landmarks) {
  const [x, y] = lm.mark;
  add(`<g transform="translate(${x},${y})"><circle r="4.4" fill="${P.ink}" stroke="${P.creamLight}" stroke-width="0.7"/>` +
    `<text y="1.75" text-anchor="middle" font-size="${lm.n > 9 ? 4.2 : 4.8}" font-weight="700" fill="${P.creamLight}" ${FONT}>${lm.n}</text></g>`);
}

// a proposal can carry the coast it replaces, drawn as a dashed line for comparison
if (L.compareCoast) add(`<path d="${L.compareCoast}" fill="none" stroke="${P.sigRed}" stroke-width="1.4" stroke-dasharray="5 3"/>`);

// scale bar and north arrow
add(`<g transform="translate(14,470)"><rect width="50" height="3" fill="${P.creamLight}"/><rect x="50" width="50" height="3" fill="${P.ink}"/>` +
  `<text y="11" font-size="5" fill="${P.creamLight}" ${FONT}>0</text><text x="44" y="11" font-size="5" fill="${P.creamLight}" ${FONT}>50</text>` +
  `<text x="90" y="11" font-size="5" fill="${P.creamLight}" ${FONT}>100 m</text></g>`);
add(`<g transform="translate(500,470)"><path d="M 0,-12 L 5,4 L 0,1 L -5,4 Z" fill="${P.creamLight}" stroke="${P.ink}" stroke-width="0.5"/>` +
  `<text y="12" text-anchor="middle" font-size="6" font-weight="700" fill="${P.creamLight}" ${FONT}>B</text></g>`);

const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="-20 -20 540 515" role="img" aria-label="Sơ đồ map Sài Gòn mini, bắc ở trên, 1 đơn vị = 1 m">` +
  `<defs>${defs}</defs>${out.join('\n')}</svg>\n`;
fs.writeFileSync(OUT, svg);
console.log('wrote', path.relative(ROOT, OUT), svg.length, 'bytes');
