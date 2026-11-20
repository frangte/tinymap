import { renderDetail } from './detail';
import { dialogueInfo, dialogueList, renderDialogue } from './dialogue-view';
import { h } from './dom';
import type { Fixture } from './content';
import { publishLists } from './fields';
import { buildModel, hashDialogue, hashQuest, type Model } from './model';
import { renderFlags, renderProblems } from './panels';
import { renderOverview } from './overview';
import { renderPlaytest } from './playtest';
import { findProblems, type Problem } from './problems';
import { renderQuestEditor } from './quest-edit';
import { newQuest } from './ops';
import { serverProblems } from './server-problems';
import { originColor, originName, state } from './state';
import { Store } from './store';

type Route =
  | { view: 'overview'; quest: string }
  | { view: 'dialogue'; id: string; node: string }
  | { view: 'problems' }
  | { view: 'flags' }
  | { view: 'playtest'; quest: string };

const fixtureFile = import.meta.glob('./fixture.json', { eager: true, import: 'default' }) as Record<string, Fixture>;
const useFixture = new URLSearchParams(location.search).has('fixture');
/** `?fixture` lays sample content over the real one, so it is only for looking: nothing there is saved */
const store = new Store(useFixture, useFixture ? Object.values(fixtureFile)[0] : undefined);
const app = document.getElementById('app')!;

let model: Model;
let problems: Problem[] = [];

function parseRoute(): Route {
  const [view, a, b] = location.hash.replace(/^#\/?/, '').split('/');
  if (view === 'quest') return { view: 'overview', quest: decodeURIComponent(a ?? '') };
  if (view === 'dialogue') return { view: 'dialogue', id: a ?? '', node: b ?? '' };
  if (view === 'playtest') return { view, quest: decodeURIComponent(a ?? '') };
  if (view === 'problems' || view === 'flags') return { view };
  return { view: 'overview', quest: '' };
}

const content = h('main', { class: 'content' });
const tabs = h('nav', { class: 'tabs' });
const filters = h('div', { class: 'filters' });
const status = h('div', { class: 'status' });
const banner = h('div', { class: 'banner hidden' });

function tab(label: string, hash: string, active: boolean, count?: number): HTMLElement {
  return h('a', { href: hash, class: active ? 'active' : '' }, label, count !== undefined && h('span', { class: 'count' }, count));
}

function renderTabs(route: Route): void {
  const errors = problems.filter((p) => p.severity !== 'info').length;
  tabs.replaceChildren(
    tab('Bản đồ truyện', '#/', route.view === 'overview'),
    tab('Lời thoại NPC', '#/dialogue/chu_sau', route.view === 'dialogue'),
    tab('Vấn đề', '#/problems', route.view === 'problems', errors),
    tab('Cờ', '#/flags', route.view === 'flags', model.flags.size),
    tab('Chơi thử', '#/playtest/', route.view === 'playtest'),
  );
}

function buildFilters(): void {
  const search = h('input', { type: 'search', placeholder: 'Tìm quest theo id hay tên', value: state.search });
  search.addEventListener('input', () => {
    state.search = search.value;
    render();
  });
  const origin = h('select', { title: 'Mờ đi những quest xuất phát này không bao giờ được mời' }, h('option', { value: '' }, 'Xem như: mọi người'));
  for (const o of model.origins) origin.append(h('option', { value: o.id }, `Xem như: ${originName(model, o.id)}`));
  origin.addEventListener('change', () => {
    state.origin = origin.value;
    origin.style.borderLeftColor = state.origin ? originColor(model, state.origin) : '';
    render();
  });
  const lang = h('div', { class: 'seg' });
  for (const l of ['vi', 'en'] as const) {
    const b = h('button', { class: state.lang === l ? 'on' : '' }, l.toUpperCase());
    b.addEventListener('click', () => {
      state.lang = l;
      lang.querySelectorAll('button').forEach((x) => x.classList.toggle('on', x === b));
      render();
    });
    lang.append(b);
  }
  const toggles = h('div', { class: 'toggles', title: 'Loại mũi tên trên bản đồ' });
  const labels = { requires: 'Cần quest', cap: 'Quyền', flag: 'Cờ và kết cục' } as const;
  for (const key of ['requires', 'cap', 'flag'] as const) {
    const box = h('input', { type: 'checkbox', checked: state.show[key] });
    box.addEventListener('change', () => {
      state.show[key] = box.checked;
      render();
    });
    toggles.append(h('label', { class: `t-${key}` }, box, labels[key]));
  }
  filters.append(search, origin, toggles, lang);
}

/** Brings the selected card into view when it is outside the part of the stage that shows. */
function revealCard(stage: HTMLElement, id: string): void {
  const card = stage.querySelector<HTMLElement>(`[data-quest="${CSS.escape(id)}"]`);
  if (!card) return;
  const c = card.getBoundingClientRect();
  const s = stage.getBoundingClientRect();
  if (c.left < s.left || c.right > s.right || c.top < s.top || c.bottom > s.bottom) card.scrollIntoView({ inline: 'center', block: 'center' });
}

/** One change of the draft as one undo step; `silent` when the field on screen already shows it. */
const editor = {
  commit: (change: () => void, silent = false) => store.edit(change, silent),
  closeQuest: () => {
    location.hash = '#/';
  },
  newQuest: (line: string, id: string) => {
    store.edit(() => newQuest(store.content, line, id));
    location.hash = hashQuest(id);
  },
};

function render(): void {
  const route = parseRoute();
  renderTabs(route);
  filters.classList.toggle('hidden', route.view !== 'overview' && route.view !== 'dialogue');
  filters.classList.toggle('overview-only', route.view === 'dialogue');
  const old = content.querySelector<HTMLElement>('.stage');
  const scroll = old ? { left: old.scrollLeft, top: old.scrollTop } : { left: 0, top: 0 };
  const oldSide = content.querySelector<HTMLElement>('.detail');
  const sideScroll = oldSide?.scrollTop ?? 0;
  const edit = store.readOnly ? undefined : editor;
  content.className = `content view-${route.view}`;
  if (route.view === 'overview') {
    const selected = model.quests.get(route.quest);
    const stage = h('div', { class: 'stage' }, renderOverview(model, selected?.id ?? '', edit));
    const side = selected ? (edit ? renderQuestEditor(model, edit, selected) : renderDetail(model, selected)) : undefined;
    content.replaceChildren(stage, ...(side ? [side] : []));
    stage.scrollLeft = scroll.left;
    stage.scrollTop = scroll.top;
    if (side && oldSide?.dataset.quest === selected?.id) side.scrollTop = sideScroll;
    if (side && selected) side.dataset.quest = selected.id;
    if (selected) requestAnimationFrame(() => revealCard(stage, selected.id));
  } else if (route.view === 'dialogue') {
    const g = model.dialogues.graphs.get(route.id);
    content.replaceChildren(dialogueList(model, route.id), g ? h('div', { class: 'dialogue-pane' }, dialogueInfo(model, g, edit), renderDialogue(model, g, route.node, edit)) : h('p', { class: 'panel' }, `Không có lời thoại "${route.id}"`));
  } else if (route.view === 'playtest') {
    content.replaceChildren(h('div', { class: 'scroll' }, renderPlaytest(model, route.quest)));
  } else if (route.view === 'problems') {
    content.replaceChildren(h('div', { class: 'scroll' }, renderProblems(problems)));
  } else {
    content.replaceChildren(h('div', { class: 'scroll' }, renderFlags(model)));
  }
}

/** The red marks on the cards and nodes on screen, without redrawing them (a field may be being typed in). */
function applyMarks(): void {
  content.querySelectorAll<HTMLElement>('[data-quest]').forEach((el) => el.classList.toggle('broken', state.marks.has(hashQuest(el.dataset.quest!))));
  const route = parseRoute();
  if (route.view !== 'dialogue') return;
  content.querySelectorAll<HTMLElement>('.dnode').forEach((el) => el.classList.toggle('broken', state.marks.has(hashDialogue(route.id, el.dataset.node))));
}

function refreshModel(): void {
  model = buildModel(store.content);
  problems = [...findProblems(model), ...serverProblems(model, store.check)];
  const rank = { error: 0, warn: 1, info: 2 } as const;
  problems.sort((a, b) => rank[a.severity] - rank[b.severity] || a.kind.localeCompare(b.kind));
  state.marks = new Set(problems.filter((p) => p.severity === 'error').map((p) => p.where.hash));
  publishLists(model);
}

function checkChip(): HTMLElement {
  const c = store.check;
  if (store.readOnly) return h('span', { class: 'chip-status' }, 'mẫu: chỉ xem');
  if (c.phase === 'idle') return h('a', { class: 'chip-status', href: '#/problems', onclick: () => void store.runCheck() }, 'server: chưa kiểm tra');
  if (c.phase === 'running') return h('span', { class: 'chip-status' }, 'server: đang kiểm tra…');
  if (c.reply.ok === null) return h('a', { class: 'chip-status warn', href: '#/problems', title: c.reply.note }, 'server: không kiểm tra được');
  if (c.reply.ok) return h('span', { class: 'chip-status ok', title: 'Server đọc được nội dung này khi khởi động' }, 'server: ổn');
  return h('a', { class: 'chip-status bad', href: '#/problems' }, `server: ${c.reply.errors.length} lỗi`);
}

function renderStatus(): void {
  const dirty = store.dirtyFiles();
  const undo = h('button', { class: 'btn', disabled: !store.canUndo, title: 'Hoàn tác (Ctrl/Cmd+Z)' }, 'Hoàn tác');
  const redo = h('button', { class: 'btn', disabled: !store.canRedo, title: 'Làm lại (Ctrl/Cmd+Shift+Z)' }, 'Làm lại');
  const save = h('button', { class: 'btn primary', disabled: dirty.length === 0, title: 'Lưu vào server/content (Ctrl/Cmd+S)' }, 'Lưu');
  undo.addEventListener('click', () => store.undo());
  redo.addEventListener('click', () => store.redo());
  save.addEventListener('click', () => void doSave());
  status.replaceChildren(
    ...(store.readOnly ? [checkChip()] : [h('span', { class: `dirty${dirty.length ? ' on' : ''}`, title: dirty.join(', ') }, dirty.length ? `● chưa lưu (${dirty.length} file)` : 'đã lưu'), checkChip(), undo, redo, save]),
  );
}

function showBanner(message: string, actions: { label: string; run: () => void }[] = []): void {
  banner.classList.remove('hidden');
  banner.replaceChildren(h('span', {}, message));
  for (const a of actions) {
    const b = h('button', { class: 'btn small' }, a.label);
    b.addEventListener('click', a.run);
    banner.append(b);
  }
  const close = h('button', { class: 'btn small' }, 'Đóng');
  close.addEventListener('click', () => banner.classList.add('hidden'));
  banner.append(close);
}

async function doSave(): Promise<void> {
  if (store.readOnly) return;
  (document.activeElement as HTMLElement | null)?.blur();
  if (store.dirtyFiles().length === 0) return;
  const reply = await store.runCheck();
  if (reply.ok === false) {
    const first = reply.errors.slice(0, 5).join('\n');
    if (!window.confirm(`Server từ chối nội dung này:\n\n${first}\n\nserver sẽ không khởi động được. Vẫn lưu?`)) return;
  }
  const result = await store.save();
  if (result.kind === 'saved') showBanner(`Đã lưu ${result.files.join(', ')}. Xem thay đổi bằng git diff.`);
  else if (result.kind === 'conflict') {
    showBanner(`Không lưu: ${result.files.join(', ')} đã đổi trên đĩa từ lúc tải (phiên khác đang sửa?). Chưa ghi gì cả.`, [
      {
        label: 'Tải lại từ đĩa (bỏ phần tôi đã sửa)',
        run: () => {
          if (window.confirm('Bỏ mọi thay đổi chưa lưu và đọc lại từ đĩa?')) void store.reload().then(() => banner.classList.add('hidden'));
        },
      },
    ]);
  } else if (result.kind === 'error') showBanner(`Không lưu được: ${result.message}`);
}

function onKey(ev: KeyboardEvent): void {
  if (!(ev.metaKey || ev.ctrlKey)) return;
  const key = ev.key.toLowerCase();
  const typing = (ev.target as HTMLElement | null)?.closest('input, textarea') !== null && (ev.target as HTMLElement | null)?.closest('input, textarea') !== undefined;
  if (key === 's') {
    ev.preventDefault();
    void doSave();
  } else if (key === 'z' && !typing && !store.readOnly) {
    ev.preventDefault();
    if (ev.shiftKey) store.redo();
    else store.undo();
  } else if (key === 'y' && !typing && !store.readOnly) {
    ev.preventDefault();
    store.redo();
  }
}

async function boot(): Promise<void> {
  app.replaceChildren(h('p', { class: 'panel' }, 'Đang đọc server/content…'));
  try {
    await store.load();
  } catch (e) {
    app.replaceChildren(h('p', { class: 'panel bad' }, `Không đọc được nội dung: ${e instanceof Error ? e.message : String(e)}. Mở trang này từ http://localhost (npm run story).`));
    return;
  }
  refreshModel();
  buildFilters();
  const title = h('div', { class: 'brand' }, 'Xưởng truyện', h('small', {}, useFixture ? 'mẫu' : 'sửa được'));
  app.replaceChildren(h('header', { class: 'topbar' }, title, tabs, filters, status), banner, content);
  store.subscribe((silent) => {
    refreshModel();
    renderStatus();
    const route = parseRoute();
    if (!silent || route.view === 'problems' || route.view === 'flags') render();
    else {
      renderTabs(route);
      applyMarks();
    }
  });
  window.addEventListener('hashchange', render);
  window.addEventListener('keydown', onKey);
  window.addEventListener('beforeunload', (ev) => {
    if (store.dirtyFiles().length === 0) return;
    ev.preventDefault();
    ev.returnValue = '';
  });
  renderStatus();
  render();
  if (!store.readOnly) void store.runCheck();
  (window as unknown as { storyStore: Store }).storyStore = store;
}

void boot();
