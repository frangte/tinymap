/**
 * Reads and writes the content JSON the way the files are written by hand: some objects on one line
 * (`{ "vi": "…", "en": "…" }`), some spread over lines, 2-space indent, a newline at the end.
 *
 * The text alone does not say which is which, so the parser remembers it for every object and array it reads
 * (`LAYOUT`: true = spread over lines) and `formatJson` writes each one the way it was read. A value with no
 * remembered layout (something the editor just made) copies the layout of its neighbour in the same list, or of
 * the same field of one, and only when there is none fits on one line when short enough.
 *
 * Self-contained and written in erasable TypeScript, so `scripts/story-format-check.mjs` can run it under Node.
 */

/** Containers (objects and arrays) by identity: true when they were written over several lines. */
const LAYOUT = new WeakMap<object, boolean>();

const INDENT = '  ';
/** the longest line a container without a layout may take when written on one line */
const LINE_WIDTH = 120;

type Json = null | boolean | number | string | Json[] | { [key: string]: Json };

class Reader {
  private at = 0;
  private readonly text: string;
  constructor(text: string) {
    this.text = text;
  }

  read(): Json {
    const value = this.value();
    this.space();
    if (this.at !== this.text.length) this.fail('unexpected text after the value');
    return value;
  }

  private fail(why: string): never {
    throw new SyntaxError(`${why} at ${this.at}`);
  }

  private space(): void {
    while (this.at < this.text.length && ' \t\r\n'.includes(this.text[this.at])) this.at++;
  }

  private expect(ch: string): void {
    this.space();
    if (this.text[this.at] !== ch) this.fail(`expected ${ch}`);
    this.at++;
  }

  /** true when a line break comes before the next token */
  private breaks(): boolean {
    const from = this.at;
    this.space();
    return this.text.slice(from, this.at).includes('\n');
  }

  private string(): string {
    const start = this.at;
    this.at++;
    while (this.at < this.text.length && this.text[this.at] !== '"') this.at += this.text[this.at] === '\\' ? 2 : 1;
    if (this.at >= this.text.length) this.fail('unterminated string');
    this.at++;
    return JSON.parse(this.text.slice(start, this.at)) as string;
  }

  private value(): Json {
    this.space();
    const ch = this.text[this.at];
    if (ch === '{') return this.object();
    if (ch === '[') return this.array();
    if (ch === '"') return this.string();
    const m = /^(?:true|false|null|-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?)/.exec(this.text.slice(this.at, this.at + 40));
    if (!m) this.fail('unexpected character');
    this.at += m[0].length;
    return JSON.parse(m[0]) as Json;
  }

  private object(): Json {
    const out: { [key: string]: Json } = {};
    this.at++;
    const spread = this.breaks();
    if (this.text[this.at] === '}') {
      this.at++;
      LAYOUT.set(out, false);
      return out;
    }
    for (;;) {
      this.space();
      const key = this.string();
      this.expect(':');
      out[key] = this.value();
      this.space();
      if (this.text[this.at] === ',') {
        this.at++;
        continue;
      }
      this.expect('}');
      break;
    }
    LAYOUT.set(out, spread);
    return out;
  }

  private array(): Json {
    const out: Json[] = [];
    this.at++;
    const spread = this.breaks();
    if (this.text[this.at] === ']') {
      this.at++;
      LAYOUT.set(out, false);
      return out;
    }
    for (;;) {
      out.push(this.value());
      this.space();
      if (this.text[this.at] === ',') {
        this.at++;
        continue;
      }
      this.expect(']');
      break;
    }
    LAYOUT.set(out, spread);
    return out;
  }
}

/** Parses JSON text, remembering how each object and array was laid out. */
export function parseJson(text: string): unknown {
  return new Reader(text).read();
}

function isContainer(v: unknown): v is object {
  return typeof v === 'object' && v !== null;
}

/** A deep copy that keeps the layouts (structuredClone would drop them). */
export function cloneJson<T>(value: T): T {
  if (!isContainer(value)) return value;
  const copy: any = Array.isArray(value) ? value.map((x) => cloneJson(x)) : Object.fromEntries(Object.entries(value).map(([k, v]) => [k, cloneJson(v)]));
  const layout = LAYOUT.get(value);
  if (layout !== undefined) LAYOUT.set(copy, layout);
  return copy;
}

/** Gives `target` (and what it holds) the layout of `like`, field by field, where `like` has one. */
export function copyLayout(target: unknown, like: unknown): void {
  if (!isContainer(target) || !isContainer(like) || Array.isArray(target) !== Array.isArray(like)) return;
  const layout = LAYOUT.get(like);
  if (layout !== undefined) LAYOUT.set(target, layout);
  for (const [key, child] of Object.entries(target)) copyLayout(child, (like as Record<string, unknown>)[key]);
}

/** The layout a value without one takes: that of the closest neighbour of the same kind in its list. */
function neighbour(list: unknown[], at: number): unknown {
  const same = (x: unknown) => isContainer(x) && LAYOUT.has(x) && Array.isArray(x) === Array.isArray(list[at]);
  for (let d = 1; d < list.length; d++) {
    if (at - d >= 0 && same(list[at - d])) return list[at - d];
    if (at + d < list.length && same(list[at + d])) return list[at + d];
  }
  return undefined;
}

function spread(value: object, like: unknown, pad: string, entries: string[]): boolean {
  const own = LAYOUT.get(value);
  if (own !== undefined) return own;
  const inherited = isContainer(like) ? LAYOUT.get(like) : undefined;
  if (inherited !== undefined) return inherited;
  const flat = entries.join(', ');
  return flat.includes('\n') || pad.length + flat.length + 4 > LINE_WIDTH;
}

function write(value: unknown, pad: string, like: unknown): string {
  if (!isContainer(value)) return JSON.stringify(value) ?? 'null';
  const inner = pad + INDENT;
  const model = LAYOUT.has(value) ? value : like;
  const sub = (key: string | number) => (isContainer(model) ? (model as Record<string, unknown>)[key] : undefined);
  let items: string[];
  let open: string;
  let close: string;
  if (Array.isArray(value)) {
    if (value.length === 0) return '[]';
    items = value.map((x, i) => write(x, inner, LAYOUT.has(x as object) ? x : isContainer(x) ? (Array.isArray(model) && model !== value ? model[i] ?? neighbour(value, i) : neighbour(value, i)) : undefined));
    [open, close] = ['[', ']'];
  } else {
    const keys = Object.keys(value);
    if (keys.length === 0) return '{}';
    items = keys.map((k) => `${JSON.stringify(k)}: ${write((value as Record<string, unknown>)[k], inner, sub(k))}`);
    [open, close] = ['{', '}'];
  }
  if (spread(value, like, pad, items)) return `${open}\n${items.map((x) => inner + x).join(',\n')}\n${pad}${close}`;
  return Array.isArray(value) ? `[${items.join(', ')}]` : `{ ${items.join(', ')} }`;
}

/** The file text for a value: the layouts it was read with, a newline at the end. */
export function formatJson(value: unknown): string {
  return `${write(value, '', undefined)}\n`;
}
