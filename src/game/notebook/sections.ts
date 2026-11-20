// The Sổ tay's sections that were phone apps before (docs/design/10-story.md, section 6): the character, the map,
// quests, and achievements with the stamp book. Each draws into the notebook's page when its tab opens; the map
// also gets update() a few times a second. Text only ever goes in through textContent.
import { archetypeById } from '../archetypes';
import { BODY_NAMES, HAT_NAMES, outfitStyle } from '../outfits';
import { MiniMap } from '../phone/miniMap';
import { badge } from '../badge';
import { alternatives, current, isDone, outcomeCount, type ActiveQuest, type QuestLog } from '../questLog';
import { BOOK_STAMPS, STREAK_STAMPS, countText, titleName, type StampDef } from '../quests';
import { button, h } from '../phone/dom';
import type { AppFactory } from '../phone/apps';
import { money, tx } from '../i18n';
import { L } from '../i18n/notebook';
import { L as ItemsL } from '../i18n/items';
import { L as JobsL } from '../i18n/jobs';
import { goalLine } from '../jobs';

export const profile: AppFactory = (root, ctx) => {
  const p = ctx.profile();
  const a = archetypeById(p.archetype);
  const head = h('div', 'app-head');
  head.append(h('strong', 'app-name', p.name));
  const b = badge(p.archetype);
  if (b) head.append(b);
  root.append(head);
  if (a) root.append(h('p', 'app-text', tx(a.pitch)));
  const facts = h('dl', 'app-facts');
  const fact = (k: string, v: string): void => facts.append(h('dt', '', k), h('dd', '', v));
  fact(L.role, a ? tx(a.name) : L.roleNone);
  fact(L.outfit, outfitStyle(p.look.outfit, p.look.body).name);
  fact(L.hat, HAT_NAMES[p.look.hat]);
  const balance = ctx.rides().balance;
  if (balance !== null) fact(L.wallet, money(balance));
  // what the player carries: the phone in hand (or where to buy one) and every other item
  const owned = ctx.owned();
  const phone = owned.phone();
  if (phone || owned.known) fact(ItemsL.phoneInHand, phone ? tx(phone.name) : ItemsL.noPhone);
  const others = owned.list().filter((i) => i !== phone && i.kind !== 'phone');
  const phones = owned.list().filter((i) => i.kind === 'phone' && i !== phone);
  const rest = [...phones, ...others];
  if (rest.length > 0 || owned.known) fact(ItemsL.itemsOwned, rest.length > 0 ? rest.map((i) => tx(i.name)).join(', ') : ItemsL.itemsNone);
  root.append(facts);
  if (!phone && owned.known) root.append(h('p', 'app-note', ItemsL.noPhoneWhere));
  const log = ctx.quests();
  let stop: (() => void) | undefined;
  if (log.titles.length > 0) {
    // the title shown under the name, among those earned
    const field = h('label', 'field');
    field.append(h('span', '', L.title));
    const select = document.createElement('select');
    select.className = 'title-select';
    const none = new Option(L.titleNone, '');
    select.append(none, ...log.titles.map((id) => new Option(titleName(id) ?? id, id)));
    // the server has the last word: the picker shows what it says is carried, and waits while offline
    const sync = (): void => {
      select.value = log.title;
      select.disabled = !ctx.online();
    };
    sync();
    select.addEventListener('change', () => ctx.chooseTitle(select.value));
    field.append(select);
    root.append(field);
    stop = log.onChange(sync);
  }
  const actions = h('div', 'app-actions');
  const other = BODY_NAMES[p.look.body === 'man' ? 'woman' : 'man'];
  actions.append(
    button(L.dress, () => {
      ctx.close();
      ctx.dress();
    }, true),
    button(L.switchBody(other), () => ctx.switchBody()),
  );
  const change = button(a ? L.changeRole : L.chooseRole, () => {
    ctx.close();
    ctx.changeArchetype();
  });
  change.classList.add('app-wide');
  actions.append(change);
  root.append(actions);
  return { dispose: stop };
};

export const map: AppFactory = (root, ctx) => {
  const m = new MiniMap();
  root.append(m.canvas, h('p', 'app-note', L.mapAbout));
  const draw = (): void => {
    const s = ctx.self();
    m.draw({ x: s.position.x, z: s.position.z, heading: s.heading }, ctx.others());
  };
  // after layout, so the canvas has its size
  requestAnimationFrame(draw);
  return { update: draw };
};

/** A row of stamps, earned ones inked. */
function stampRow(stamps: readonly StampDef[], earned: ReadonlySet<string>, cls = 'stamp-book'): HTMLOListElement {
  const book = h('ol', cls);
  book.setAttribute('role', 'list');
  for (const s of stamps) {
    const li = h('li', earned.has(s.id) ? 'stamp is-earned' : 'stamp');
    li.append(h('span', '', tx(s.name)), h('span', 'sr-only', earned.has(s.id) ? L.stampEarned : L.stampMissing));
    book.append(li);
  }
  return book;
}

const DAY_MS = 24 * 60 * 60 * 1000;
/** Saigon is UTC+7 all year: the dailies start over at its midnight */
const SAIGON_OFFSET_MS = 7 * 60 * 60 * 1000;

/** "4 giờ 12 phút" ("4 h 12 min") until the dailies start over. */
function untilMidnight(now: number): string {
  const t = now + SAIGON_OFFSET_MS;
  const left = Math.max(0, (Math.floor(t / DAY_MS) + 1) * DAY_MS - t);
  const hours = Math.floor(left / 3_600_000);
  const minutes = Math.ceil((left % 3_600_000) / 60_000);
  return hours > 0 ? L.hoursMinutes(hours, minutes) : L.minutes(minutes);
}

/** One quest's card: its objectives, and how to track it. */
function questCard(q: ActiveQuest, log: QuestLog, tracked: ActiveQuest | null): HTMLElement {
  const now = current(q);
  const card = h('section', q.def === tracked?.def ? 'quest-card is-tracked' : 'quest-card');
  card.append(h('strong', 'quest-title', tx(q.def.title)), h('p', 'app-note', tx(q.def.about)));
  const goals = h('ul', 'quest-goals');
  for (const o of q.def.objectives) {
    const li = h('li', isDone(q, o) ? 'is-done' : now.includes(o) ? 'is-now' : '');
    const line = goalLine(log, q, o);
    li.append(h('span', '', line.text));
    if (line.count) li.append(h('small', '', line.count));
    goals.append(li);
  }
  card.append(goals);
  const ways = alternatives(q);
  if (ways.length > 0) {
    card.append(h('small', 'quest-or', L.chooseOne));
    const list = h('ul', 'quest-goals');
    ways.forEach((o, i) => {
      const li = h('li', 'is-now');
      li.append(h('span', '', (i > 0 ? `${L.or} ` : '') + tx(o.text)));
      const count = countText(o, outcomeCount(q, o));
      if (count) li.append(h('small', '', count));
      list.append(li);
    });
    card.append(list);
  }
  if (q.def.xu > 0) card.append(h('small', 'quest-reward', L.reward(money(q.def.xu))));
  card.append(q.def === tracked?.def ? h('small', 'quest-tracking', L.tracking) : button(L.track, () => log.track(q.def.id)));
  return card;
}

/** Today's dailies, the streak and its stamps. */
function dailySection(log: QuestLog, tracked: ActiveQuest | null): HTMLElement | null {
  const dailies = log.dailies();
  if (dailies.length === 0) return null;
  const box = h('section', 'daily-section');
  const streak = log.streak;
  box.append(
    h('strong', 'app-section', L.daily),
    h('p', 'app-note', `${streak > 0 ? L.streak(streak) : L.noStreak} · ${L.resetsIn(untilMidnight(Date.now()))}`),
  );
  for (const q of dailies) {
    if (!q.state.done) {
      box.append(questCard(q, log, tracked));
      continue;
    }
    const card = h('section', 'quest-card is-done');
    card.append(h('strong', 'quest-title', tx(q.def.title)), h('small', 'quest-tracking', q.def.xu > 0 ? L.doneTodayPaid(money(q.def.xu)) : L.doneToday));
    box.append(card);
  }
  if (STREAK_STAMPS.length > 0) box.append(stampRow(STREAK_STAMPS, new Set(log.stamps), 'stamp-book is-small'));
  return box;
}

/** Today's odd jobs (docs/design/13-jobs.md): once a day each, apart from the streak of dailies. */
function jobSection(log: QuestLog, tracked: ActiveQuest | null): HTMLElement | null {
  const jobs = log.jobs();
  if (jobs.length === 0) return null;
  const box = h('section', 'daily-section');
  box.append(h('strong', 'app-section', JobsL.title), h('p', 'app-note', `${JobsL.intro} ${L.resetsIn(untilMidnight(Date.now()))}`));
  for (const q of jobs) {
    if (!q.state.done) {
      box.append(questCard(q, log, tracked));
      continue;
    }
    const card = h('section', 'quest-card is-done');
    card.append(h('strong', 'quest-title', tx(q.def.title)), h('small', 'quest-tracking', q.def.xu > 0 ? L.doneTodayPaid(money(q.def.xu)) : JobsL.doneToday));
    box.append(card);
  }
  return box;
}

export const quests: AppFactory = (root, ctx) => {
  const log = ctx.quests();
  let doneOpen = false;
  const draw = (): void => {
    // a redraw (progress ticking while the section is open) keeps the place and what was unfolded
    const scroll = root.scrollTop;
    root.replaceChildren();
    if (!log.known) {
      root.append(h('p', 'app-note', ctx.online() ? L.questsNoAccount : L.questsOffline));
      return;
    }
    const tracked = log.tracked();
    const daily = dailySection(log, tracked);
    if (daily) root.append(daily);
    const jobs = jobSection(log, tracked);
    if (jobs) root.append(jobs);
    const active = log.active().filter((q) => !q.def.daily);
    if (active.length > 0 && (daily || jobs)) root.append(h('strong', 'app-section', L.quests));
    if (active.length === 0 && !daily && !jobs) root.append(h('p', 'app-note', L.questsAllDone));
    for (const q of active) root.append(questCard(q, log, tracked));
    const done = log.done();
    if (done.length > 0) {
      const box = document.createElement('details');
      box.className = 'quest-done';
      box.open = doneOpen;
      box.addEventListener('toggle', () => {
        doneOpen = box.open;
      });
      box.append(h('summary', '', L.questsDone(done.length)));
      const ol = h('ol', 'app-list');
      for (const d of done) {
        const li = h('li', 'landmark is-seen', tx(d.title));
        const ending = log.outcomeOf(d.id);
        if (ending) li.append(h('small', 'quest-ending', ` · ${L.endedWith(tx(ending.title))}`));
        ol.append(li);
      }
      box.append(ol);
      root.append(box);
    }
    root.scrollTop = scroll;
  };
  draw();
  return { dispose: log.onChange(draw) };
};

/** The stamp book, when quests.json has stamps for it (none since the Traveler's questline left, 2026-11-19). */
function stampBook(log: QuestLog): HTMLElement | null {
  if (BOOK_STAMPS.length === 0) return null;
  const earned = new Set(log.stamps);
  const have = BOOK_STAMPS.filter((s) => earned.has(s.id)).length;
  const box = h('section', 'stamp-section');
  box.append(h('strong', 'app-section', L.stampBook(have, BOOK_STAMPS.length)), stampRow(BOOK_STAMPS, earned));
  return box;
}

export const achievements: AppFactory = (root, ctx) => {
  const log = ctx.quests();
  const draw = (): void => {
    root.replaceChildren();
    if (!log.known) {
      root.append(h('p', 'app-note', ctx.online() ? L.achievementsNoAccount : L.achievementsOffline));
      return;
    }
    const book = stampBook(log);
    if (book) root.append(book);
    root.append(h('strong', 'app-section', L.achievements));
    const ol = h('ol', 'app-list');
    for (const { def, state } of log.achievements()) {
      const done = state?.done === true;
      const secret = def.hidden && !done;
      const li = h('li', done ? 'achievement is-done' : 'achievement');
      li.append(h('strong', '', secret ? '???' : tx(def.title)), h('span', '', secret ? L.achievementSecret : tx(def.about)));
      if (!done && def.count > 1) {
        const n = Math.min(state?.n ?? 0, def.count);
        const bar = h('span', 'meter');
        bar.setAttribute('role', 'progressbar');
        bar.setAttribute('aria-valuemin', '0');
        bar.setAttribute('aria-valuemax', String(def.count));
        bar.setAttribute('aria-valuenow', String(n));
        bar.setAttribute('aria-label', tx(def.title));
        const fill = h('i', '');
        fill.style.width = `${Math.round((100 * n) / def.count)}%`;
        bar.append(fill);
        li.append(bar, h('small', '', countText(def, state?.n ?? 0)));
      }
      ol.append(li);
    }
    root.append(ol);
  };
  draw();
  return { dispose: log.onChange(draw) };
};
