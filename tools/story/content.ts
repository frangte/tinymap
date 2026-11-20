import type { DialogueDef, QuestDef, RawContent } from './types';

const files = import.meta.glob('/server/content/**/*.json', { eager: true, import: 'default' }) as Record<string, unknown>;
const fixtureFile = import.meta.glob('./fixture.json', { eager: true, import: 'default' }) as Record<string, unknown>;

/** What `?fixture` adds on top of the real content: extra quests and replaced or added dialogues. */
interface Fixture {
  quests?: QuestDef[];
  dialogues?: DialogueDef[];
}

function file<T>(name: string): T {
  const data = files[`/server/content/${name}.json`];
  if (!data) throw new Error(`server/content/${name}.json not found`);
  return data as T;
}

/**
 * Reads the content JSON through Vite, so editing a file reloads the page.
 * With `?fixture`, `tools/story/fixture.json` is laid over it (a sample of outcomes and flags no real content uses yet).
 */
export function loadContent(useFixture: boolean): RawContent {
  const dialogues = Object.entries(files)
    .filter(([path]) => path.startsWith('/server/content/dialogue/'))
    .map(([, data]) => structuredClone(data as DialogueDef))
    .sort((a, b) => a.id.localeCompare(b.id));
  const quests = structuredClone(file<RawContent['quests']>('quests'));
  const content: RawContent = {
    quests,
    archetypes: file('archetypes'),
    npcs: file<{ npcs: RawContent['npcs'] }>('npcs').npcs,
    contacts: file<{ contacts: RawContent['contacts'] }>('contacts').contacts,
    items: file('items'),
    collection: file<{ entries: RawContent['collection'] }>('collection').entries,
    places: file<{ places: RawContent['places'] }>('places').places,
    dialogues,
  };
  if (!useFixture) return content;
  const fixture = Object.values(fixtureFile)[0] as Fixture | undefined;
  if (!fixture) throw new Error('tools/story/fixture.json not found');
  quests.quests.push(...(fixture.quests ?? []));
  for (const d of fixture.dialogues ?? []) {
    const at = content.dialogues.findIndex((x) => x.id === d.id);
    if (at >= 0) content.dialogues[at] = d;
    else content.dialogues.push(d);
  }
  return content;
}
