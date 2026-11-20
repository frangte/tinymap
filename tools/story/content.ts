import { parseJson } from './format';
import type { DialogueDef, QuestDef, RawContent } from './types';

/** A content file as the editor holds it: what is on disk, and the draft being edited. */
export interface ContentFile {
  path: string;
  /** hash of the text on disk when it was loaded; a save is refused if the disk has moved on */
  version: string;
  /** the text on disk when it was loaded */
  original: string;
  /** the draft's data as plain JSON when it was loaded or last saved, to tell whether it has changed */
  baseline: string;
  data: unknown;
}

interface LoadReply {
  files: Record<string, { text: string; version: string }>;
}

const HEADERS = { 'x-story-editor': '1' };

/** Reads every content file through the dev server's /__story/load (not import: a save must not reload the page). */
export async function fetchFiles(): Promise<Map<string, ContentFile>> {
  const res = await fetch('/__story/load', { headers: HEADERS, cache: 'no-store' });
  if (!res.ok) throw new Error(`/__story/load: ${res.status} ${await res.text()}`);
  const reply = (await res.json()) as LoadReply;
  const files = new Map<string, ContentFile>();
  for (const [path, f] of Object.entries(reply.files)) {
    const data = parseJson(f.text);
    files.set(path, { path, version: f.version, original: f.text, baseline: JSON.stringify(data), data });
  }
  return files;
}

export interface SaveReply {
  versions?: Record<string, string>;
  conflicts?: string[];
  error?: string;
}

/** Writes the given files; `base` is the version each was loaded at. */
export async function postSave(files: { path: string; text: string; base: string }[]): Promise<{ status: number; body: SaveReply }> {
  const body = { files: Object.fromEntries(files.map((f) => [f.path, { text: f.text, base: f.base }])) };
  const res = await fetch('/__story/save', { method: 'POST', headers: { ...HEADERS, 'content-type': 'application/json' }, body: JSON.stringify(body) });
  return { status: res.status, body: (await res.json()) as SaveReply };
}

export interface CheckReply {
  /** null: the check could not run (no Go on this machine) */
  ok: boolean | null;
  errors: string[];
  note?: string;
}

/** Runs the server's own content checks on the draft files laid over the content on disk. */
export async function postCheck(files: { path: string; text: string }[]): Promise<CheckReply> {
  const body = { files: Object.fromEntries(files.map((f) => [f.path, f.text])) };
  const res = await fetch('/__story/check', { method: 'POST', headers: { ...HEADERS, 'content-type': 'application/json' }, body: JSON.stringify(body) });
  if (!res.ok) return { ok: null, errors: [], note: `${res.status} ${await res.text()}` };
  return (await res.json()) as CheckReply;
}

function data<T>(files: Map<string, ContentFile>, name: string): T {
  const f = files.get(`${name}.json`);
  if (!f) throw new Error(`server/content/${name}.json not found`);
  return f.data as T;
}

/** What `?fixture` adds on top of the real content: extra quests and replaced or added dialogues. */
export interface Fixture {
  quests?: QuestDef[];
  dialogues?: DialogueDef[];
}

/** The content the views read, made of the files' own objects (an edit to them is an edit to the draft). */
export function contentFrom(files: Map<string, ContentFile>, fixture?: Fixture): RawContent {
  const dialogues = [...files.values()]
    .filter((f) => f.path.startsWith('dialogue/'))
    .map((f) => f.data as DialogueDef)
    .sort((a, b) => a.id.localeCompare(b.id));
  const content: RawContent = {
    quests: data(files, 'quests'),
    archetypes: data(files, 'archetypes'),
    npcs: data<{ npcs: RawContent['npcs'] }>(files, 'npcs').npcs,
    contacts: data<{ contacts: RawContent['contacts'] }>(files, 'contacts').contacts,
    items: data(files, 'items'),
    collection: data<{ entries: RawContent['collection'] }>(files, 'collection').entries,
    places: data<{ places: RawContent['places'] }>(files, 'places').places,
    dialogues,
  };
  if (!fixture) return content;
  content.quests = { ...content.quests, quests: [...content.quests.quests, ...(fixture.quests ?? [])] };
  for (const d of fixture.dialogues ?? []) {
    const at = content.dialogues.findIndex((x) => x.id === d.id);
    if (at >= 0) content.dialogues[at] = d;
    else content.dialogues.push(d);
  }
  return content;
}
