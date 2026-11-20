// Checks that the Xưởng truyện's JSON formatter (tools/story/format.ts) writes the content files back exactly as they
// are: format(parse(text)) === text. Saving content the editor did not change must give a zero git diff.
//
// The editor writes only quests.json and dialogue/*.json, so those must round-trip (a stray blank line, which the
// formatter drops, is reported but allowed: it is a one-line diff in a file that was changed anyway). The other files
// are listed for information: the editor never writes them.
// Run: node scripts/story-format-check.mjs   (Node 22.18+ runs the TypeScript directly)
import { readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import { formatJson, parseJson } from '../tools/story/format.ts';

const root = new URL('../server/content/', import.meta.url).pathname;
const files = [
  ...readdirSync(root).filter((f) => f.endsWith('.json')),
  ...readdirSync(join(root, 'dialogue')).map((f) => `dialogue/${f}`),
];
const editable = (f) => f === 'quests.json' || f.startsWith('dialogue/');
const withoutBlankLines = (t) => t.split('\n').filter((l) => l !== '').join('\n');

let failed = 0;
let exact = 0;
for (const f of files) {
  const text = readFileSync(join(root, f), 'utf8');
  let out;
  try {
    out = formatJson(parseJson(text));
  } catch (e) {
    out = `error: ${e.message}`;
  }
  if (out === text) {
    exact++;
    continue;
  }
  const blankOnly = withoutBlankLines(out) === withoutBlankLines(text);
  const a = text.split('\n');
  const b = out.split('\n');
  const at = a.findIndex((line, i) => line !== b[i]);
  const kind = blankOnly ? 'only a stray blank line' : `first difference at line ${at + 1}\n    file: ${a[at]?.slice(0, 140)}\n    made: ${b[at]?.slice(0, 140)}`;
  console.log(`${editable(f) ? (blankOnly ? 'note' : 'FAIL') : 'info'} ${f}: ${kind}`);
  if (editable(f) && !blankOnly) failed++;
}
console.log(`${exact}/${files.length} files round-trip exactly; ${failed} editable file(s) fail`);
process.exit(failed ? 1 : 0);
