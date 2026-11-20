/**
 * "Chơi thử từ đây": pick a quest, and the page makes a test character in the state just before it (every quest it
 * needs done, the flags and capabilities it reads) through the dev route of the Go server (POST /api/dev/playtest,
 * server/internal/playtest, on only with SG_DEV_TOOLS=1), then opens the game as that character in a new tab.
 */
import './playtest.css';
import { h, type Child } from './dom';
import { allObjectives, questTitle, type Model, type QuestNode } from './model';
import { tx } from './state';

const API = '/api/dev/playtest';
/** the longest label the game's name takes after "Thử: " (src/game/profile.ts NAME_MAX is 16) */
const LABEL_MAX = 11;
const DEFAULT_BODY = 'man';

export interface DoneQuest {
  quest: string;
  outcome?: string;
}

/** What the server is asked to make; the same shape as playtest.Request in Go. */
export interface PlaytestRequest {
  archetype: string;
  done: DoneQuest[];
  flags: string[];
  items: string[];
  xu: number;
  stand: string;
  label: string;
}

/** What the page works out from the chosen quest, before the author changes anything. */
export interface Plan {
  /** done quests with the outcome each ends with ('' for a quest without outcomes), in the order found */
  done: Map<string, string>;
  items: Set<string>;
  warnings: string[];
}

/** The set a quest needs done, and the items that give the capabilities it and they read. */
export function planFor(model: Model, questId: string, origin: string): Plan {
  const plan: Plan = { done: new Map(), items: new Set(), warnings: [] };
  const target = model.quests.get(questId);
  if (!target) return plan;
  const forced = new Map<string, string>();
  const origins = model.origins.find((o) => o.id === origin);
  const caps = new Set<string>(origins?.capabilities ?? []);
  const scope = (): QuestNode[] => [...plan.done.keys(), questId].flatMap((id) => model.quests.get(id) ?? []);

  const addQuest = (id: string): void => {
    const q = model.quests.get(id);
    if (!q || q.def.repeat || plan.done.has(id) || id === questId) return;
    plan.done.set(id, '');
    needsOf(q);
  };
  const addFlag = (flag: string): void => {
    const setters = [...model.quests.values()].filter((q) => !q.def.repeat && q.flagsSet.includes(flag));
    const setter = setters.find((q) => q.effective.has(origin)) ?? setters[0];
    if (!setter) {
      plan.warnings.push(`cờ ${flag}: không quest nào đặt`);
      return;
    }
    const outcome = (setter.def.outcomes ?? []).find((o) => flag === `${setter.id}:${o.id ?? ''}` || (o.flags ?? []).includes(flag));
    if (outcome?.id) forced.set(setter.id, outcome.id);
    addQuest(setter.id);
  };
  const needsOf = (q: QuestNode): void => {
    for (const r of q.def.requires ?? []) addQuest(r);
    for (const f of q.def.requires_flags ?? []) addFlag(f);
  };
  needsOf(target);

  // capabilities: an item that gives it, else a quest that does; repeat until nothing new is needed
  for (let again = true; again; ) {
    again = false;
    for (const q of scope()) {
      if (q.def.rewards?.capability) caps.add(q.def.rewards.capability);
    }
    for (const item of plan.items) for (const g of itemGrants(model, item)) caps.add(g);
    for (const q of scope()) {
      for (const cap of q.def.requires_caps ?? []) {
        if (caps.has(cap)) continue;
        const item = (model.content.items.items ?? []).find((i) => i.grants?.includes(cap));
        const quest = (model.capSources.get(cap) ?? []).find((s) => s.kind === 'quest');
        if (item) plan.items.add(item.id);
        else if (quest?.kind === 'quest') addQuest(quest.quest);
        else {
          plan.warnings.push(`quyền ${cap}: không item hay quest nào cấp`);
          caps.add(cap);
        }
        again = true;
      }
    }
  }

  const unwanted = new Set(scope().flatMap((q) => q.def.unless_flags ?? []));
  for (const id of plan.done.keys()) {
    const outcomes = model.quests.get(id)?.def.outcomes ?? [];
    if (!outcomes.length) continue;
    const ok = outcomes.find((o) => ![`${id}:${o.id ?? ''}`, ...(o.flags ?? [])].some((f) => unwanted.has(f)));
    plan.done.set(id, forced.get(id) ?? ok?.id ?? outcomes[0]?.id ?? '');
  }
  if (origin && !target.effective.has(origin)) plan.warnings.push('xuất phát này không bao giờ được mời quest này');
  return plan;
}

function itemGrants(model: Model, id: string): string[] {
  return (model.content.items.items ?? []).find((i) => i.id === id)?.grants ?? [];
}

/** Where to stand by default: the first place or NPC the quest's objectives point at. */
export function defaultStand(q: QuestNode): string {
  return allObjectives(q.def).find((o) => o.at)?.at ?? '';
}

/** The label after "Thử: " in the character's name. */
export function labelFor(questId: string): string {
  return Array.from(questId.split('.').pop() ?? questId).slice(0, LABEL_MAX).join('');
}

/** The game's address for a test character the server made. */
export function gameUrl(quest: string, res: { token: string; archetype: string; name: string; look: { body?: string; outfit: string; hat: string } }): string {
  const q = new URLSearchParams({
    playtest: res.token,
    ptarch: res.archetype,
    ptname: res.name,
    ptquest: quest,
    ptlook: [res.look.body || DEFAULT_BODY, res.look.outfit, res.look.hat].join('.'),
    lang: 'vi',
  });
  return `/?${q.toString()}`;
}

interface PanelState {
  origin: string;
  outcomes: Map<string, string>;
  items: Set<string>;
  flags: Set<string>;
  xu: number;
  stand: string;
}

const row = (label: string, control: Child, hint?: string): HTMLElement =>
  h('label', { class: 'pt-row' }, h('span', {}, label), control, hint ? h('small', {}, hint) : null);

/** The "Chơi thử" tab: `questId` is the quest to start from ('' to pick one). */
export function renderPlaytest(model: Model, questId: string): HTMLElement {
  const root = h('div', { class: 'panel playtest' }, h('h2', {}, 'Chơi thử từ đây'));
  root.append(h('p', { class: 'hint' }, 'Chọn quest muốn bắt đầu chơi. Trang tự đánh dấu xong mọi quest nó cần, rồi tạo một nhân vật thử ở đúng trạng thái đó và mở game. Cần ', h('code', {}, 'SG_DEV_TOOLS=1 npm run server'), '.'));
  root.append(questPicker(model, questId));
  const q = model.quests.get(questId);
  if (q) root.append(planPanel(model, q));
  return root;
}

function questPicker(model: Model, current: string): HTMLElement {
  const select = h('select', {});
  select.append(h('option', { value: '' }, '— chọn quest —'));
  for (const q of model.quests.values()) {
    if (q.kind !== 'story') continue;
    select.append(h('option', { value: q.id }, `${questTitle(q.def)} (${q.id})`));
  }
  select.value = current;
  select.addEventListener('change', () => {
    location.hash = `#/playtest/${encodeURIComponent(select.value)}`;
  });
  return row('Bắt đầu từ quest', select);
}

function planPanel(model: Model, q: QuestNode): HTMLElement {
  const st: PanelState = { origin: [...q.effective][0] ?? model.origins[0]?.id ?? '', outcomes: new Map(), items: new Set(), flags: new Set(), xu: 0, stand: defaultStand(q) };
  const box = h('div', { class: 'pt-plan' });
  const status = h('p', { class: 'pt-status' });
  const draw = (reset: boolean): void => {
    if (reset) {
      const plan = planFor(model, q.id, st.origin);
      st.outcomes = new Map(plan.done);
      st.items = new Set(plan.items);
      box.dataset.warnings = JSON.stringify(plan.warnings);
    }
    box.replaceChildren(...sections(model, q, st, () => draw(false), (r) => draw(r)), status);
  };
  draw(true);
  return box;

  function sections(m: Model, quest: QuestNode, s: PanelState, redraw: () => void, redo: (reset: boolean) => void): HTMLElement[] {
    const warnings = JSON.parse(box.dataset.warnings ?? '[]') as string[];
    const origin = h('select', {});
    for (const o of m.origins) origin.append(h('option', { value: o.id }, tx(o.name, o.id)));
    origin.value = s.origin;
    origin.addEventListener('change', () => {
      s.origin = origin.value;
      redo(true);
    });
    const button = h('button', { class: 'pt-go' }, 'Chơi thử');
    button.addEventListener('click', () => void launch(quest.id, request(m, quest, s), status, button));
    return [
      row('Xuất phát', origin),
      ...(warnings.length ? [h('ul', { class: 'pt-warn' }, warnings.map((w) => h('li', {}, w)))] : []),
      doneList(m, s, redraw),
      itemList(m, s),
      flagList(m, s),
      row('Thêm tiền (đồng)', moneyInput(s)),
      row('Đứng ở', standSelect(m, s), 'trống: chỗ xuất phát của nhân vật'),
      h('div', { class: 'pt-actions' }, button),
    ];
  }
}

function request(model: Model, q: QuestNode, s: PanelState): PlaytestRequest {
  const done = [...s.outcomes].map(([quest, outcome]) => ({ quest, ...(outcome ? { outcome } : {}) }));
  void model;
  return { archetype: s.origin, done, flags: [...s.flags], items: [...s.items], xu: s.xu, stand: s.stand, label: labelFor(q.id) };
}

function doneList(model: Model, s: PanelState, redraw: () => void): HTMLElement {
  const list = h('ol', { class: 'pt-done' });
  for (const [id, outcome] of s.outcomes) {
    const def = model.quests.get(id)?.def;
    const item = h('li', {}, h('b', {}, questTitle(def ?? { id })), h('small', {}, ` ${id}`));
    if (def?.outcomes?.length) {
      const pick = h('select', {});
      for (const o of def.outcomes) pick.append(h('option', { value: o.id ?? '' }, tx(o.title, o.id)));
      pick.value = outcome;
      pick.addEventListener('change', () => {
        s.outcomes.set(id, pick.value);
        redraw();
      });
      item.append(' kết cục: ', pick);
    }
    list.append(item);
  }
  return h('section', { class: 'sect' }, h('h3', {}, `Đã xong (${s.outcomes.size})`), s.outcomes.size ? list : h('p', { class: 'hint' }, 'Quest này không cần quest nào xong trước.'));
}

function checkbox(label: string, checked: boolean, on: (v: boolean) => void): HTMLElement {
  const box = h('input', { type: 'checkbox' });
  box.checked = checked;
  box.addEventListener('change', () => on(box.checked));
  return h('label', { class: 'pt-check' }, box, label);
}

function itemList(model: Model, s: PanelState): HTMLElement {
  const items = model.content.items.items ?? [];
  return h(
    'section',
    { class: 'sect' },
    h('h3', {}, 'Đồ có sẵn'),
    h('div', { class: 'pt-checks' }, items.map((i) => checkbox(`${tx(i.name, i.id)} (${(i.grants ?? []).join(', ') || 'không cấp quyền'})`, s.items.has(i.id), (v) => (v ? s.items.add(i.id) : s.items.delete(i.id))))),
  );
}

function flagList(model: Model, s: PanelState): HTMLElement {
  const flags = [...model.flags.keys()].sort();
  return h(
    'details',
    { class: 'sect' },
    h('summary', {}, 'Cờ thêm (ngoài các cờ quest đã xong đặt)'),
    h('div', { class: 'pt-checks' }, flags.map((f) => checkbox(f, s.flags.has(f), (v) => (v ? s.flags.add(f) : s.flags.delete(f))))),
  );
}

function moneyInput(s: PanelState): HTMLElement {
  const input = h('input', { type: 'number', min: 0, step: 1000, value: s.xu });
  input.addEventListener('input', () => {
    s.xu = Math.max(0, Math.floor(Number(input.value) || 0));
  });
  return input;
}

function standSelect(model: Model, s: PanelState): HTMLElement {
  const select = h('select', {}, h('option', { value: '' }, '(chỗ xuất phát)'));
  const npcs = h('optgroup', { label: 'NPC' }, model.content.npcs.map((n) => h('option', { value: n.id }, `${n.name} (${n.id})`)));
  const places = h('optgroup', { label: 'Địa điểm' }, model.content.places.map((p) => h('option', { value: p.id }, `${tx(p.name, p.id)} (${p.id})`)));
  select.append(npcs, places);
  select.value = s.stand;
  select.addEventListener('change', () => {
    s.stand = select.value;
  });
  return select;
}

async function launch(quest: string, req: PlaytestRequest, status: HTMLElement, button: HTMLButtonElement): Promise<void> {
  // the tab is opened now, while the click still counts for the popup blocker, and pointed at the game after
  const tab = window.open('', '_blank');
  button.disabled = true;
  status.className = 'pt-status';
  status.textContent = 'Đang tạo nhân vật thử…';
  try {
    const res = await fetch(API, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(req) });
    if (res.status === 404) throw new Error('Server chưa bật dev tools: chạy SG_DEV_TOOLS=1 npm run server');
    const body = (await res.json()) as { error?: string; token: string; archetype: string; name: string; look: { body?: string; outfit: string; hat: string } };
    if (!res.ok) throw new Error(body.error ?? `lỗi ${res.status}`);
    status.textContent = `Đã tạo ${body.name}.`;
    if (tab) tab.location.href = gameUrl(quest, body);
    else status.append(' ', h('a', { href: gameUrl(quest, body), target: '_blank' }, 'Mở game'));
  } catch (err) {
    tab?.close();
    status.className = 'pt-status bad';
    status.textContent = err instanceof Error ? err.message : String(err);
  } finally {
    button.disabled = false;
  }
}
