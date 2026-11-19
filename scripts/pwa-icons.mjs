#!/usr/bin/env node
// node scripts/pwa-icons.mjs — draws the app icon (Chợ Bến Thành's clock tower, paper and ink, the palette of
// docs/PROJECT.md) and writes every size the install needs into public/icons/. Run it again after changing the
// drawing; the PNGs are committed, so a build never needs it. Rasterizes with rsvg-convert (`brew install librsvg`).
//   icon.svg              favicon, and the source of the "any" icons
//   icon-192.png / -512   "any": a rounded paper card, transparent corners (desktop, browser UI)
//   maskable-512.png      "maskable": full bleed, the drawing inside the 80 % safe circle (Android cuts its own shape)
//   apple-touch-icon.png  180 px, full bleed and opaque (iOS rounds it itself)
import { mkdirSync, writeFileSync } from 'node:fs';
import { spawnSync } from 'node:child_process';
import { join } from 'node:path';

const OUT = new URL('../public/icons/', import.meta.url).pathname;

const C = {
  paper: '#F3EBDD',
  sun: '#FFE6A8',
  ink: '#2B211C',
  roof: '#A9492F',
  roofLight: '#C0613F',
  wall: '#A0A7AA',
  cream: '#EAD9AE',
  arch: '#3B3330',
  clockBlue: '#4E7CA6',
  clockWhite: '#F3EEE2',
  gold: '#C9A24A',
};

/** the market front, about x 64–448 and y 40–456 on a 512 canvas */
const TOWER = `
  <circle cx="256" cy="196" r="150" fill="${C.sun}"/>
  <g stroke="${C.ink}" stroke-width="10" stroke-linejoin="round" stroke-linecap="round">
    <path d="M76 352 H180 V456 H76 Z" fill="${C.wall}"/>
    <path d="M332 352 H436 V456 H332 Z" fill="${C.wall}"/>
    <path d="M64 356 L180 292 V356 Z" fill="${C.roof}"/>
    <path d="M448 356 L332 292 V356 Z" fill="${C.roof}"/>
    <path d="M196 166 H316 V316 H196 Z" fill="${C.wall}"/>
    <path d="M156 316 H356 V456 H156 Z" fill="${C.cream}"/>
    <path d="M218 456 V392 A38 38 0 0 1 294 392 V456 Z" fill="${C.arch}"/>
    <path d="M176 172 L256 88 L336 172 Z" fill="${C.roof}"/>
    <path d="M256 88 L176 172 H256 Z" fill="${C.roofLight}" stroke="none"/>
    <path d="M176 172 L256 88 L336 172 Z" fill="none"/>
    <path d="M256 88 V54"/>
    <circle cx="256" cy="48" r="8" fill="${C.gold}" stroke-width="6"/>
    <circle cx="256" cy="240" r="46" fill="${C.clockBlue}" stroke-width="8"/>
    <circle cx="256" cy="240" r="32" fill="${C.clockWhite}" stroke="none"/>
    <path d="M256 240 V216 M256 240 L274 250" stroke-width="7"/>
    <path d="M40 456 H472"/>
  </g>`;

const svg = (body) => `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512">${body}\n</svg>\n`;

/** "any": a paper card with an ink edge, the drawing at full size */
const card = svg(`
  <rect x="16" y="16" width="480" height="480" rx="96" fill="${C.paper}" stroke="${C.ink}" stroke-width="12"/>
  <clipPath id="card"><rect x="22" y="22" width="468" height="468" rx="90"/></clipPath>
  <g clip-path="url(#card)">${TOWER}</g>`);

/** full bleed, the drawing scaled into the safe circle (radius 40 % of the size) */
const bleed = (scale) => svg(`
  <rect width="512" height="512" fill="${C.paper}"/>
  <g transform="translate(256 256) scale(${scale}) translate(-256 -252)">${TOWER}</g>`);

function rasterize(name, source, size) {
  const r = spawnSync('rsvg-convert', ['-w', String(size), '-h', String(size), '-o', join(OUT, name)], { input: source });
  if (r.error) throw new Error(`rsvg-convert: ${r.error.message} (brew install librsvg)`);
  if (r.status !== 0) throw new Error(`rsvg-convert ${name}: ${r.stderr}`);
  console.log(`wrote public/icons/${name}`);
}

mkdirSync(OUT, { recursive: true });
writeFileSync(join(OUT, 'icon.svg'), card);
console.log('wrote public/icons/icon.svg');
rasterize('icon-192.png', card, 192);
rasterize('icon-512.png', card, 512);
rasterize('maskable-512.png', bleed(0.72), 512);
rasterize('apple-touch-icon.png', bleed(0.8), 180);
