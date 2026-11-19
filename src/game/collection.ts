// The Sổ sưu tầm's entries (docs/design/10-story.md, "Nhiệm vụ ngầm: Sổ sưu tầm"): server/content/collection.json,
// the file the Go server runs the hidden quests from (its tests keep it valid). Which ones a character found comes
// from the server (the `progress` message, QuestLog.collection); the text is here. Place and food entries come
// later from the designer, so those groups may be empty.
import data from '../../server/content/collection.json';
import type { Bilingual } from './i18n';

export type CollectionKind = 'phrase' | 'place' | 'food';

/** the groups, in the order the Sổ tay shows them */
export const COLLECTION_KINDS: readonly CollectionKind[] = ['phrase', 'place', 'food'];

export interface CollectionEntry {
  readonly id: string;
  readonly kind: CollectionKind;
  /** the phrase itself for a phrase */
  readonly title: Bilingual;
  /** what the player reads once it is found */
  readonly text: Bilingual;
  /** shown while the entry is still "???" */
  readonly hint: Bilingual;
}

interface RawEntry {
  id: string;
  kind: string;
  title: Bilingual;
  text: Bilingual;
  hint: Bilingual;
}

const FILE: { readonly entries: readonly RawEntry[] } = data;

const isKind = (k: string): k is CollectionKind => (COLLECTION_KINDS as readonly string[]).includes(k);

/** every entry, in the file's order */
export const COLLECTION: readonly CollectionEntry[] = FILE.entries.flatMap((e): CollectionEntry[] =>
  isKind(e.kind) ? [{ id: e.id, kind: e.kind, title: e.title, text: e.text, hint: e.hint }] : [],
);

export function collectionEntry(id: string): CollectionEntry | undefined {
  return COLLECTION.find((e) => e.id === id);
}

export function entriesOf(kind: CollectionKind): readonly CollectionEntry[] {
  return COLLECTION.filter((e) => e.kind === kind);
}
