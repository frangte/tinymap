// What each archetype is told on arrival (storyCard.ts): who they are, what to do first, and the one tip that
// gets them going. Game-side text only, so it is not in server/content/archetypes.json (the server never reads it);
// the quests it points at are server/content/quests.json and the design in docs/design/01-gameplay.md. The text, in
// Vietnamese and English, is src/game/i18n/stories.ts.
import { lang } from './i18n';
import { STORIES, type Story } from './i18n/stories';

export type { Story };

/** The story for an archetype id, in the current language; a generic one for ids this build does not know. */
export function storyFor(archetype: string): Story {
  const s = STORIES[lang()];
  return s.byArchetype[archetype] ?? s.fallback;
}
