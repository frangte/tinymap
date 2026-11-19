// The Sổ tay's "Sưu tầm" tab (docs/design/10-story.md, "Nhiệm vụ ngầm: Sổ sưu tầm"): the entries in three groups,
// phrases, side stories about places, dishes. A found entry shows its title and text; one not found yet is "???"
// with its hint. The server says which are found (QuestLog.collection); text only ever goes in through textContent.
import { COLLECTION_KINDS, entriesOf, type CollectionKind } from '../collection';
import { tx } from '../i18n';
import { L } from '../i18n/notebook';
import type { AppFactory } from '../phone/apps';
import { h } from '../phone/dom';

const GROUP_NAMES: Readonly<Record<CollectionKind, () => string>> = {
  phrase: () => L.collectionPhrase,
  place: () => L.collectionPlace,
  food: () => L.collectionFood,
};

export const collection: AppFactory = (root, ctx) => {
  const log = ctx.quests();
  const draw = (): void => {
    const scroll = root.scrollTop;
    root.replaceChildren();
    if (!log.known) {
      root.append(h('p', 'app-note', ctx.online() ? L.collectionNoAccount : L.collectionOffline));
      return;
    }
    const found = new Set(log.collection);
    root.append(h('p', 'app-note', L.collectionAbout));
    for (const kind of COLLECTION_KINDS) {
      const entries = entriesOf(kind);
      const have = entries.filter((e) => found.has(e.id)).length;
      root.append(h('strong', 'app-section', `${GROUP_NAMES[kind]()} · ${L.collectionCount(have, entries.length)}`));
      if (entries.length === 0) {
        root.append(h('p', 'app-note', L.collectionSoon));
        continue;
      }
      const ol = h('ol', 'app-list');
      for (const e of entries) {
        const has = found.has(e.id);
        const li = h('li', has ? 'achievement is-done' : 'achievement');
        li.append(
          h('strong', '', has ? tx(e.title) : '???'),
          h('span', '', has ? tx(e.text) : tx(e.hint)),
          h('span', 'sr-only', has ? L.collectionFound : L.collectionMissing),
        );
        ol.append(li);
      }
      root.append(ol);
    }
    root.scrollTop = scroll;
  };
  draw();
  return { dispose: log.onChange(draw) };
};
