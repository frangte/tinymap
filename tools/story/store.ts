import { contentFrom, fetchFiles, postCheck, postSave, type CheckReply, type ContentFile, type Fixture } from './content';
import { cloneJson, formatJson } from './format';
import type { RawContent } from './types';

/** The files the editor changes; the rest of server/content is read only. */
export const isEditable = (path: string): boolean => path === 'quests.json' || path.startsWith('dialogue/');

type Snapshot = Map<string, unknown>;

const UNDO_LIMIT = 200;
const CHECK_DELAY_MS = 700;

export type CheckState =
  | { phase: 'idle' }
  | { phase: 'running' }
  | { phase: 'done'; reply: CheckReply; at: number };

export type SaveResult = { kind: 'saved'; files: string[] } | { kind: 'conflict'; files: string[] } | { kind: 'error'; message: string } | { kind: 'nothing' };

/**
 * The draft: every content file, the session's edits to the editable ones, undo and redo, what is dirty, and the
 * server's verdict on the draft. Views read `content` (made of the files' own objects) and call `edit`.
 */
export class Store {
  files = new Map<string, ContentFile>();
  content!: RawContent;
  check: CheckState = { phase: 'idle' };
  readonly readOnly: boolean;
  private fixture?: Fixture;
  private undoStack: Snapshot[] = [];
  private redoStack: Snapshot[] = [];
  private listeners = new Set<(silent: boolean) => void>();
  private checkTimer = 0;
  private checkSeq = 0;
  private dirtyCache: string[] | null = [];

  constructor(readOnly: boolean, fixture?: Fixture) {
    this.readOnly = readOnly;
    this.fixture = fixture;
  }

  async load(): Promise<void> {
    this.files = await fetchFiles();
    this.undoStack = [];
    this.redoStack = [];
    this.rebuild();
  }

  /** Called after every change; `silent` means the view on screen already shows it (a typed text). */
  subscribe(fn: (silent: boolean) => void): void {
    this.listeners.add(fn);
  }

  private rebuild(): void {
    this.content = contentFrom(this.files, this.fixture);
    this.dirtyCache = null;
  }

  private notify(silent: boolean): void {
    this.listeners.forEach((fn) => fn(silent));
  }

  private snapshot(): Snapshot {
    return new Map([...this.files].filter(([p]) => isEditable(p)).map(([p, f]) => [p, cloneJson(f.data)]));
  }

  private restore(s: Snapshot): void {
    for (const [p, data] of s) this.files.get(p)!.data = cloneJson(data);
    for (const p of [...this.files.keys()]) if (!s.has(p) && isEditable(p) && this.files.get(p)!.version === '') this.files.delete(p);
    this.rebuild();
  }

  /** Applies one change to the draft as one undo step. */
  edit(change: (content: RawContent) => void, silent = false): void {
    if (this.readOnly) return;
    this.undoStack.push(this.snapshot());
    if (this.undoStack.length > UNDO_LIMIT) this.undoStack.shift();
    this.redoStack = [];
    change(this.content);
    this.rebuild();
    this.notify(silent);
    this.scheduleCheck();
  }

  get canUndo(): boolean {
    return this.undoStack.length > 0;
  }

  get canRedo(): boolean {
    return this.redoStack.length > 0;
  }

  undo(): void {
    const s = this.undoStack.pop();
    if (!s) return;
    this.redoStack.push(this.snapshot());
    this.restore(s);
    this.notify(false);
    this.scheduleCheck();
  }

  redo(): void {
    const s = this.redoStack.pop();
    if (!s) return;
    this.undoStack.push(this.snapshot());
    this.restore(s);
    this.notify(false);
    this.scheduleCheck();
  }

  /** Paths of the files whose draft differs from what was loaded. */
  dirtyFiles(): string[] {
    if (this.dirtyCache) return this.dirtyCache;
    this.dirtyCache = [...this.files.values()].filter((f) => isEditable(f.path) && JSON.stringify(f.data) !== f.baseline).map((f) => f.path);
    return this.dirtyCache;
  }

  /** Drafts as text, for the dirty files. */
  drafts(): { path: string; text: string; base: string }[] {
    return this.dirtyFiles().map((p) => ({ path: p, text: formatJson(this.files.get(p)!.data), base: this.files.get(p)!.version }));
  }

  private scheduleCheck(): void {
    window.clearTimeout(this.checkTimer);
    if (this.readOnly) return;
    this.checkTimer = window.setTimeout(() => void this.runCheck(), CHECK_DELAY_MS);
  }

  /** Asks the server's own checks about the draft (all dirty files laid over the content on disk). */
  async runCheck(): Promise<CheckReply> {
    const seq = ++this.checkSeq;
    this.check = { phase: 'running' };
    this.notify(true);
    let reply: CheckReply;
    try {
      reply = await postCheck(this.drafts().map((d) => ({ path: d.path, text: d.text })));
    } catch (e) {
      reply = { ok: null, errors: [], note: e instanceof Error ? e.message : String(e) };
    }
    if (seq === this.checkSeq) {
      this.check = { phase: 'done', reply, at: Date.now() };
      this.notify(true);
    }
    return reply;
  }

  async save(): Promise<SaveResult> {
    const drafts = this.drafts();
    if (drafts.length === 0) return { kind: 'nothing' };
    try {
      const { status, body } = await postSave(drafts);
      if (status === 409) return { kind: 'conflict', files: body.conflicts ?? [] };
      if (status !== 200 || !body.versions) return { kind: 'error', message: body.error ?? `HTTP ${status}` };
      for (const d of drafts) {
        const f = this.files.get(d.path)!;
        f.original = d.text;
        f.baseline = JSON.stringify(f.data);
        f.version = body.versions[d.path];
      }
      this.dirtyCache = null;
      this.notify(true);
      return { kind: 'saved', files: drafts.map((d) => d.path) };
    } catch (e) {
      return { kind: 'error', message: e instanceof Error ? e.message : String(e) };
    }
  }

  /** Drops the draft and reads the disk again. */
  async reload(): Promise<void> {
    await this.load();
    this.notify(false);
    this.scheduleCheck();
  }
}
