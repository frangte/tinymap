import { spawn } from 'node:child_process';
import { createHash } from 'node:crypto';
import type { IncomingMessage, ServerResponse } from 'node:http';
import { cpSync, existsSync, mkdirSync, mkdtempSync, readdirSync, readFileSync, renameSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import type { Plugin } from 'vite';

/**
 * Dev-only endpoints the Xưởng truyện reads and writes server/content through (`apply: 'serve'`: not in the build).
 *
 *   GET  /__story/load   every content file with a version (the hash of its text)
 *   POST /__story/save   { files: { path: { text, base } } }: writes the files whose `base` is still what is on disk
 *   POST /__story/check  { files: { path: text } }: runs the server's own content checks on a copy with these laid over
 *
 * Who may call: only this machine (the dev server listens on the LAN too), only with the page's own Origin, and
 * only with the custom header (a cross-site page cannot send it without a CORS preflight, which is refused).
 */

const PREFIX = '/__story/';
const HEADER = 'x-story-editor';
const MAX_BODY = 8 * 1024 * 1024;
/** a content file the editor may touch: server/content/<name>.json or server/content/dialogue/<id>.json */
const ALLOWED = /^(?:[a-z0-9_]+|dialogue\/[a-z0-9_]+)\.json$/;
const LOOPBACK_ADDRESS = new Set(['127.0.0.1', '::1', '::ffff:127.0.0.1']);
const LOOPBACK_HOST = new Set(['localhost', '127.0.0.1', '[::1]']);
/** how long a file we wrote ourselves is kept from reloading the pages (the watcher is slower than the write) */
const OWN_WRITE_MS = 3000;

const sha = (text: string): string => createHash('sha1').update(text).digest('hex');

class HttpError extends Error {
  readonly status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

/** Throws when the request is not from this machine's own page. */
export function assertLocal(req: Pick<IncomingMessage, 'socket' | 'headers'>): void {
  if (!LOOPBACK_ADDRESS.has(req.socket.remoteAddress ?? '')) throw new HttpError(403, 'only this machine may use the editor');
  const host = req.headers.host ?? '';
  const hostname = host.startsWith('[') ? host.slice(0, host.indexOf(']') + 1) : host.split(':')[0];
  if (!LOOPBACK_HOST.has(hostname)) throw new HttpError(403, 'open the editor as localhost');
  if (req.headers[HEADER] !== '1') throw new HttpError(403, `missing ${HEADER} header`);
  const site = req.headers['sec-fetch-site'];
  if (site !== undefined && site !== 'same-origin') throw new HttpError(403, 'cross-site request');
  const origin = req.headers.origin;
  if (origin !== undefined) {
    let originHost = '';
    try {
      originHost = new URL(origin).host;
    } catch {
      /* stays '' and fails below */
    }
    if (originHost !== host) throw new HttpError(403, 'cross-origin request');
  }
}

function listContent(dir: string): string[] {
  const top = readdirSync(dir).filter((f) => f.endsWith('.json'));
  const dialogue = existsSync(join(dir, 'dialogue')) ? readdirSync(join(dir, 'dialogue')).filter((f) => f.endsWith('.json')).map((f) => `dialogue/${f}`) : [];
  return [...top, ...dialogue].filter((f) => ALLOWED.test(f)).sort();
}

function readBody(req: IncomingMessage): Promise<unknown> {
  return new Promise((ok, fail) => {
    const chunks: Buffer[] = [];
    let size = 0;
    req.on('data', (c: Buffer) => {
      size += c.length;
      if (size > MAX_BODY) {
        fail(new HttpError(413, 'body too large'));
        req.destroy();
      } else chunks.push(c);
    });
    req.on('end', () => {
      try {
        ok(JSON.parse(Buffer.concat(chunks).toString('utf8')));
      } catch {
        fail(new HttpError(400, 'body is not JSON'));
      }
    });
    req.on('error', fail);
  });
}

function send(res: ServerResponse, status: number, body: unknown): void {
  res.statusCode = status;
  res.setHeader('content-type', 'application/json; charset=utf-8');
  res.setHeader('cache-control', 'no-store');
  res.end(JSON.stringify(body));
}

function safePath(name: unknown): string {
  if (typeof name !== 'string' || !ALLOWED.test(name)) throw new HttpError(400, `not a content file: ${String(name)}`);
  return name;
}

function parseText(name: string, text: unknown): string {
  if (typeof text !== 'string') throw new HttpError(400, `${name}: text must be a string`);
  try {
    JSON.parse(text);
  } catch {
    throw new HttpError(400, `${name}: not valid JSON`);
  }
  return text;
}

interface CheckResult {
  ok: boolean | null;
  errors: string[];
  note?: string;
}

interface Run {
  code: number | null;
  out: string;
  err: string;
}

function run(cmd: string, args: string[], cwd: string): Promise<Run> {
  return new Promise((ok) => {
    const p = spawn(cmd, args, { cwd });
    let out = '';
    let err = '';
    p.stdout.on('data', (d: Buffer) => (out += d));
    p.stderr.on('data', (d: Buffer) => (err += d));
    p.on('error', (e) => ok({ code: -1, out, err: String(e) }));
    p.on('close', (code) => ok({ code, out, err }));
  });
}

/** Builds (go caches it) and runs server/cmd/contentcheck on a content directory. */
async function runCheck(serverDir: string, contentDir: string): Promise<CheckResult> {
  const bin = join(tmpdir(), `sg-contentcheck-${process.getuid?.() ?? 0}`);
  const built = await run('go', ['build', '-o', bin, './cmd/contentcheck'], serverDir);
  if (built.code !== 0) return { ok: null, errors: [], note: `không build được công cụ kiểm tra (cần Go): ${built.err.trim().slice(0, 400)}` };
  const ran = await run(bin, ['-dir', contentDir], serverDir);
  try {
    return JSON.parse(ran.out) as CheckResult;
  } catch {
    return { ok: null, errors: [], note: `công cụ kiểm tra lỗi: ${(ran.err || ran.out).trim().slice(0, 400)}` };
  }
}

export function storyEditor(): Plugin {
  let contentDir = '';
  let serverDir = '';
  const written = new Map<string, number>();
  let queue: Promise<unknown> = Promise.resolve();

  const load = (res: ServerResponse) => {
    const files: Record<string, { text: string; version: string }> = {};
    for (const name of listContent(contentDir)) {
      const text = readFileSync(join(contentDir, name), 'utf8');
      files[name] = { text, version: sha(text) };
    }
    send(res, 200, { files });
  };

  const save = async (req: IncomingMessage, res: ServerResponse) => {
    const body = (await readBody(req)) as { files?: Record<string, { text?: unknown; base?: unknown }> };
    const incoming = Object.entries(body.files ?? {}).map(([name, f]) => ({ name: safePath(name), text: parseText(name, f?.text), base: f?.base }));
    const conflicts = incoming.filter(({ name, base }) => {
      const file = join(contentDir, name);
      if (base === null) return existsSync(file);
      return !existsSync(file) || sha(readFileSync(file, 'utf8')) !== base;
    });
    if (conflicts.length > 0) return send(res, 409, { conflicts: conflicts.map((c) => c.name) });
    const versions: Record<string, string> = {};
    for (const { name, text } of incoming) {
      const file = join(contentDir, name);
      const tmp = `${file}.story-tmp`;
      mkdirSync(dirname(file), { recursive: true });
      writeFileSync(tmp, text);
      renameSync(tmp, file);
      written.set(resolve(file), Date.now());
      versions[name] = sha(text);
    }
    send(res, 200, { versions });
  };

  const check = async (req: IncomingMessage, res: ServerResponse) => {
    const body = (await readBody(req)) as { files?: Record<string, unknown> };
    const draft = Object.entries(body.files ?? {}).map(([name, text]) => ({ name: safePath(name), text: parseText(name, text) }));
    const job = async (): Promise<CheckResult> => {
      const tmp = mkdtempSync(join(tmpdir(), 'sg-story-'));
      try {
        for (const name of listContent(contentDir)) {
          mkdirSync(dirname(join(tmp, name)), { recursive: true });
          cpSync(join(contentDir, name), join(tmp, name));
        }
        for (const { name, text } of draft) {
          mkdirSync(dirname(join(tmp, name)), { recursive: true });
          writeFileSync(join(tmp, name), text);
        }
        return await runCheck(serverDir, tmp);
      } finally {
        rmSync(tmp, { recursive: true, force: true });
      }
    };
    const result = queue.then(job, job);
    queue = result.catch(() => undefined);
    send(res, 200, await result);
  };

  return {
    name: 'story-editor',
    apply: 'serve',
    configResolved(config) {
      contentDir = join(config.root, 'server', 'content');
      serverDir = join(config.root, 'server');
    },
    configureServer(server) {
      server.middlewares.use(async (req, res, next) => {
        const url = req.url ?? '';
        if (!url.startsWith(PREFIX)) return next();
        try {
          assertLocal(req);
          const route = url.slice(PREFIX.length).split('?')[0];
          if (req.method === 'GET' && route === 'load') return load(res);
          if (req.method === 'POST' && route === 'save') return await save(req, res);
          if (req.method === 'POST' && route === 'check') return await check(req, res);
          throw new HttpError(404, 'no such endpoint');
        } catch (e) {
          const status = e instanceof HttpError ? e.status : 500;
          send(res, status, { error: e instanceof Error ? e.message : String(e) });
        }
      });
    },
    // a save is not a reason to reload the pages open on this server (the editor keeps its state)
    handleHotUpdate({ file }) {
      const at = written.get(resolve(file));
      if (at !== undefined && Date.now() - at < OWN_WRITE_MS) return [];
      return undefined;
    },
  };
}
