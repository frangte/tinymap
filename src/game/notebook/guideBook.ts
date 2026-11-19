// The Cẩm nang in the Sổ tay (docs/design/10-story.md, section 10): every landmark, those with a page first, and the
// page itself: history, what to see, tips, how to get there for real, with the day it was checked and its sources.
// The pages are content (server/content/guide.json, through guide.ts). Text only ever goes in through textContent;
// links are only the page's own https sources and a Google Maps search.
import { PLACES, placeNote, placeTitle, type Place } from '../places';
import { GUIDE, guideFor, mapsLink, type GuidePage, type ReadPages } from '../guide';
import { button, h } from '../phone/dom';
import { tx, type Bilingual } from '../i18n';
import { L } from '../i18n/guide';

/** What the Cẩm nang needs from the notebook and the game. */
export interface GuideNav {
  openPage(placeId: string): void;
  readonly read: ReadPages;
  /** the landmarks walked up to this visit */
  seen(): ReadonlySet<Place>;
  /** the button that takes the player there ("Dẫn đường tới đây" in play, "Bay tới đây" over the map), or none */
  goLabel(place: Place): string | null;
  goTo(place: Place): void;
}

const LANDMARKS = PLACES.filter((p) => p.kind === 'landmark');
/** the pages in the file's order, then the landmarks that have none yet */
const LISTED: readonly Place[] = [...GUIDE.map((g) => g.place), ...LANDMARKS.filter((l) => !guideFor(l.id))];

function marks(place: Place, nav: GuideNav, hasPage: boolean): HTMLElement {
  const box = h('span', 'guide-marks');
  if (nav.seen().has(place)) box.append(h('span', 'guide-mark is-seen', L.visited));
  if (hasPage && nav.read.has(place.id)) box.append(h('span', 'guide-mark is-read', L.read));
  return box;
}

function listItem(place: Place, nav: GuideNav): HTMLLIElement {
  const page = guideFor(place.id);
  const li = h('li', page ? 'guide-item' : 'guide-item is-soon');
  if (!page) {
    li.append(h('strong', '', placeTitle(place)), h('span', 'guide-note', placeNote(place)), h('small', 'guide-soon', L.soon), marks(place, nav, false));
    return li;
  }
  const open = h('button', 'guide-open');
  open.type = 'button';
  open.append(h('strong', '', placeTitle(place)), h('span', 'guide-note', placeNote(place)), h('span', 'guide-summary', tx(page.summary)), marks(place, nav, true));
  open.addEventListener('click', () => nav.openPage(place.id));
  li.append(open);
  return li;
}

/** The list of landmarks; it redraws when a page is read, and when a landmark is walked up to while it is open. */
export function guideList(root: HTMLElement, nav: GuideNav): { update(): void; dispose(): void } {
  let seenCount = -1;
  const draw = (): void => {
    const seen = nav.seen();
    seenCount = seen.size;
    const visited = LANDMARKS.filter((l) => seen.has(l)).length;
    const ol = h('ol', 'guide-list');
    for (const p of LISTED) ol.append(listItem(p, nav));
    root.replaceChildren(h('p', 'app-text', L.intro), h('p', 'app-note', L.progress(visited, LANDMARKS.length, nav.read.count, GUIDE.length)), ol);
  };
  draw();
  return {
    update: () => {
      if (nav.seen().size !== seenCount) draw();
    },
    dispose: nav.read.onChange(draw),
  };
}

function textSection(title: string, paragraphs: readonly Bilingual[]): HTMLElement | null {
  if (paragraphs.length === 0) return null;
  const box = h('section', 'guide-section');
  box.append(h('h4', '', title), ...paragraphs.map((t) => h('p', '', tx(t))));
  return box;
}

function listSection(title: string, items: readonly Bilingual[]): HTMLElement | null {
  if (items.length === 0) return null;
  const box = h('section', 'guide-section');
  const ul = h('ul', '');
  ul.append(...items.map((t) => h('li', '', tx(t))));
  box.append(h('h4', '', title), ul);
  return box;
}

function link(href: string, text: string, cls = ''): HTMLAnchorElement {
  const a = h('a', cls, text);
  a.href = href;
  a.target = '_blank';
  a.rel = 'noopener noreferrer';
  return a;
}

/** "19 tháng 11, 2026" / "19 November 2026" for a YYYY-MM-DD day */
function dayText(day: string): string {
  const d = new Date(`${day}T00:00:00`);
  return Number.isNaN(d.getTime()) ? day : d.toLocaleDateString(L.dateLocale, { day: 'numeric', month: 'long', year: 'numeric' });
}

function hostOf(href: string): string {
  try {
    return new URL(href).hostname.replace(/^www\./, '');
  } catch {
    return href;
  }
}

/** One place's page; opening it counts as reading it. */
export function placePage(root: HTMLElement, page: GuidePage, nav: GuideNav): void {
  nav.read.add(page.place.id);
  const head = h('header', 'guide-head');
  head.append(h('h3', 'guide-title', placeTitle(page.place)), h('p', 'guide-note', placeNote(page.place)), h('p', 'guide-lede', tx(page.summary)));
  const facts = h('dl', 'guide-facts');
  const fact = (label: string, t: Bilingual | undefined): void => {
    if (t) facts.append(h('dt', '', label), h('dd', '', tx(t)));
  };
  fact(L.built, page.facts.built);
  fact(L.hours, page.facts.hours);
  fact(L.ticket, page.facts.ticket);
  const actions = h('div', 'guide-actions');
  const go = nav.goLabel(page.place);
  if (go) actions.append(button(go, () => nav.goTo(page.place), true));
  actions.append(link(mapsLink(page), L.maps, 'guide-maps'));
  const foot = h('footer', 'guide-foot');
  const sources = h('p', '');
  sources.append(`${L.sources}: `);
  page.sources.forEach((s, i) => {
    if (i > 0) sources.append(' · ');
    sources.append(link(s, hostOf(s)));
  });
  foot.append(h('p', '', L.checked(dayText(page.checked))), sources);
  const sections = [
    textSection(L.history, page.history),
    listSection(L.see, page.see),
    listSection(L.tips, page.tips),
    listSection(L.gettingThere, page.gettingThere),
    listSection(L.food, page.food),
  ].filter((s): s is HTMLElement => s !== null);
  root.append(head, facts, actions, ...sections, foot);
}
