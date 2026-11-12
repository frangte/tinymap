// One command from the Blender map to the light GLB the game loads (docs/map-v5-tasks.md, T1).
//
//   npm run map:export                       # models/saigon_mini.blend -> export/saigon_mini.glb
//   npm run map:export -- <file.blend> <out.glb> [--budget]
//
// Do not use Blender's File > Export > glTF for the game: it writes float normals and positions, one primitive per
// material per object and the Blender-only preview sea (about 65 MB for Sài Gòn mini). This script runs
// scripts/blender/export_glb.py (no normals, colours only, PREVIEW_* and NAV_* left out) and then the gltf-transform
// chain: flatten, join (one mesh per material), weld, quantize positions, prune and meshopt compression (~4 MB).
// --budget also prints the triangle budget (scripts/blender/mini/budget.py).
//
// Blender: $BLENDER, else /Applications/Blender.app/Contents/MacOS/Blender, else `blender` on the PATH.
// gltf-transform: $GLTF_TRANSFORM (a binary), else `npx --yes @gltf-transform/cli@4.1.1` (downloaded once).
import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..');
const args = process.argv.slice(2);
const flags = new Set(args.filter((a) => a.startsWith('--')));
const [blendArg, outArg] = args.filter((a) => !a.startsWith('--'));
const blend = path.resolve(ROOT, blendArg ?? 'models/saigon_mini.blend');
const out = path.resolve(ROOT, outArg ?? 'export/saigon_mini.glb');
const tmp = path.join(path.dirname(out), '.tmp');
const raw = path.join(tmp, path.basename(out, '.glb') + '_raw.glb');

const MAC_BLENDER = '/Applications/Blender.app/Contents/MacOS/Blender';
const blender = process.env.BLENDER ?? (fs.existsSync(MAC_BLENDER) ? MAC_BLENDER : 'blender');
const gt = process.env.GLTF_TRANSFORM ? [process.env.GLTF_TRANSFORM] : ['npx', '--yes', '@gltf-transform/cli@4.1.1'];

const mb = (n) => (n / 1e6).toFixed(2) + ' MB';
function run(cmd, argv, label) {
  const t0 = Date.now();
  const r = spawnSync(cmd, argv, { stdio: ['ignore', 'pipe', 'pipe'], encoding: 'utf8' });
  if (r.status !== 0) {
    process.stderr.write(r.stdout ?? '');
    process.stderr.write(r.stderr ?? '');
    throw new Error(`${label} failed (${cmd} exit ${r.status ?? r.error})`);
  }
  console.log(`  ${label.padEnd(10)} ${((Date.now() - t0) / 1000).toFixed(1)} s`);
  return r.stdout;
}

if (!fs.existsSync(blend)) throw new Error(`no such file: ${blend}`);
fs.mkdirSync(tmp, { recursive: true });
console.log(`map:export ${path.relative(ROOT, blend)} -> ${path.relative(ROOT, out)}`);

run(blender, [blend, '--background', '--python', path.join(ROOT, 'scripts/blender/export_glb.py'), '--', raw], 'blender');
const steps = [
  ['flatten', []],
  ['join', []],
  ['weld', []],
  ['quantize', ['--pattern', 'POSITION']],
  ['prune', []],
  ['meshopt', []],
];
let cur = raw;
steps.forEach(([cmd, extra], i) => {
  const next = i === steps.length - 1 ? out : path.join(tmp, `${i}_${cmd}.glb`);
  run(gt[0], [...gt.slice(1), cmd, cur, next, ...extra], cmd);
  cur = next;
});

const rawBytes = fs.statSync(raw).size;
const outBytes = fs.statSync(out).size;
const gz = zlib.gzipSync(fs.readFileSync(out), { level: 9 }).length;
console.log(`\n  raw export  ${mb(rawBytes)}\n  final GLB   ${mb(outBytes)}  (gzip ${mb(gz)})`);
fs.rmSync(tmp, { recursive: true, force: true });

if (flags.has('--budget')) {
  const report = run(blender, [blend, '--background', '--python', path.join(ROOT, 'scripts/blender/mini/budget.py')], 'budget');
  console.log(report.split('\n').filter((l) => l.startsWith('[budget]')).join('\n'));
}
