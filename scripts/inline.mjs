// Inlines the single-chunk build (dist-single) into one self-contained HTML file.
import { readFileSync, writeFileSync, readdirSync } from 'node:fs';
import { join } from 'node:path';

const dir = 'dist-single';
let html = readFileSync(join(dir, 'index.html'), 'utf8');
const assets = readdirSync(join(dir, 'assets'));
for (const file of assets) {
  const body = readFileSync(join(dir, 'assets', file), 'utf8');
  if (file.endsWith('.js')) {
    html = html.replace(new RegExp(`<script type="module" crossorigin src="/assets/${file}"></script>`), () => `<script type="module">${body.replace(/<\/script/g, '<\\/script')}</script>`);
  } else if (file.endsWith('.css')) {
    html = html.replace(new RegExp(`<link rel="stylesheet" crossorigin href="/assets/${file}">`), () => `<style>${body}</style>`);
  }
}
writeFileSync(join(dir, 'saigon-noir.html'), html);
console.log(`wrote ${join(dir, 'saigon-noir.html')} (${(html.length / 1024).toFixed(0)} kB)`);
