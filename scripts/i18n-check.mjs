#!/usr/bin/env node
// npm run i18n:check — keeps every piece of interface text in the catalogs (src/game/i18n/), since there is no test
// runner to catch a string typed straight into the code. It fails when:
//  - a string or template literal with Vietnamese marks sits in src/ outside src/game/i18n/ (comments are fine);
//  - index.html has Vietnamese text or a Vietnamese title / aria-label / placeholder outside the few names that
//    stay as written (the game's name, "Tiếng Việt" on the switch);
//  - index.html uses a data-i18n key that src/game/i18n/page.ts does not have.
// Missing English in a catalog is tsc's job (`en: typeof vi`); missing English in server/content is `go test`'s.
import { readFileSync, readdirSync, statSync } from 'node:fs';
import { join, relative } from 'node:path';
import ts from 'typescript';

const ROOT = new URL('..', import.meta.url).pathname;
const CATALOGS = join(ROOT, 'src/game/i18n');
const VIETNAMESE = /[àáảãạăằắẳẵặâầấẩẫậèéẻẽẹêềếểễệìíỉĩịòóỏõọôồốổỗộơờớởỡợùúủũụưừứửữựỳýỷỹỵđ]/i;
/** text in index.html that is a name, the same in both languages */
const AS_WRITTEN = new Set(['Sài Gòn mini', 'Tiếng Việt']);
const LITERALS = new Set([
  ts.SyntaxKind.StringLiteral,
  ts.SyntaxKind.NoSubstitutionTemplateLiteral,
  ts.SyntaxKind.TemplateHead,
  ts.SyntaxKind.TemplateMiddle,
  ts.SyntaxKind.TemplateTail,
]);

const problems = [];

function sourceFiles(dir) {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) return path === CATALOGS ? [] : sourceFiles(path);
    return name.endsWith('.ts') ? [path] : [];
  });
}

function checkSource(path) {
  const file = ts.createSourceFile(path, readFileSync(path, 'utf8'), ts.ScriptTarget.Latest, true);
  const visit = (node) => {
    if (LITERALS.has(node.kind) && VIETNAMESE.test(node.text)) {
      const { line } = file.getLineAndCharacterOfPosition(node.getStart(file));
      problems.push(`${relative(ROOT, path)}:${line + 1}: Vietnamese text in code, move it to a catalog in src/game/i18n/: ${JSON.stringify(node.text)}`);
    }
    ts.forEachChild(node, visit);
  };
  visit(file);
}

/** the keys of `const vi = { … }` in src/game/i18n/page.ts */
function pageKeys() {
  const path = join(CATALOGS, 'page.ts');
  const file = ts.createSourceFile(path, readFileSync(path, 'utf8'), ts.ScriptTarget.Latest, true);
  const keys = new Set();
  const visit = (node) => {
    if (ts.isVariableDeclaration(node) && node.name.getText(file) === 'vi' && node.initializer && ts.isObjectLiteralExpression(node.initializer)) {
      for (const p of node.initializer.properties) if (p.name) keys.add(p.name.getText(file));
    }
    ts.forEachChild(node, visit);
  };
  visit(file);
  if (keys.size === 0) problems.push('src/game/i18n/page.ts: found no `const vi = { … }` catalog');
  return keys;
}

function lineOf(text, index) {
  return text.slice(0, index).split('\n').length;
}

function checkPage() {
  const raw = readFileSync(join(ROOT, 'index.html'), 'utf8');
  // comments keep their length (as spaces) so line numbers stay right
  const html = raw.replace(/<!--[\s\S]*?-->/g, (c) => c.replace(/[^\n]/g, ' '));
  for (const m of html.matchAll(/>([^<]+)</g)) {
    const text = m[1].trim();
    if (text && VIETNAMESE.test(text) && !AS_WRITTEN.has(text)) {
      problems.push(`index.html:${lineOf(html, m.index)}: Vietnamese text, give the element a data-i18n key (src/game/i18n/page.ts): ${JSON.stringify(text)}`);
    }
  }
  for (const m of html.matchAll(/\s(title|aria-label|placeholder)="([^"]*)"/g)) {
    if (VIETNAMESE.test(m[2])) {
      problems.push(`index.html:${lineOf(html, m.index)}: Vietnamese ${m[1]}, use data-i18n-${m[1] === 'aria-label' ? 'aria' : m[1]}: ${JSON.stringify(m[2])}`);
    }
  }
  const keys = pageKeys();
  for (const m of html.matchAll(/\sdata-i18n(?:-[a-z]+)?="([^"]*)"/g)) {
    if (!keys.has(m[1])) problems.push(`index.html:${lineOf(html, m.index)}: data-i18n key "${m[1]}" is not in src/game/i18n/page.ts`);
  }
}

for (const path of sourceFiles(join(ROOT, 'src'))) checkSource(path);
checkPage();

if (problems.length > 0) {
  console.error(problems.join('\n'));
  console.error(`\ni18n:check: ${problems.length} problem${problems.length === 1 ? '' : 's'}`);
  process.exit(1);
}
console.log('i18n:check: all interface text is in the catalogs');
