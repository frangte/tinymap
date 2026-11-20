import { renderDetail } from './detail';
import { dialogueInfo, dialogueList, renderDialogue } from './dialogue-view';
import { h } from './dom';
import { loadContent } from './content';
import { buildModel, type Model } from './model';
import { renderFlags, renderProblems } from './panels';
import { renderOverview } from './overview';
import { findProblems } from './problems';
import { originColor, originName, state } from './state';

type Route =
  | { view: 'overview'; quest: string }
  | { view: 'dialogue'; id: string; node: string }
  | { view: 'problems' }
  | { view: 'flags' };

const useFixture = new URLSearchParams(location.search).has('fixture');
const model: Model = buildModel(loadContent(useFixture));
const problems = findProblems(model);
const app = document.getElementById('app')!;

function parseRoute(): Route {
  const [view, a, b] = location.hash.replace(/^#\/?/, '').split('/');
  if (view === 'quest') return { view: 'overview', quest: decodeURIComponent(a ?? '') };
  if (view === 'dialogue') return { view: 'dialogue', id: a ?? '', node: b ?? '' };
  if (view === 'problems' || view === 'flags') return { view };
  return { view: 'overview', quest: '' };
}

const content = h('main', { class: 'content' });
const tabs = h('nav', { class: 'tabs' });
const filters = h('div', { class: 'filters' });

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

function render(): void {
  const route = parseRoute();
  renderTabs(route);
  filters.classList.toggle('hidden', route.view !== 'overview' && route.view !== 'dialogue');
  filters.classList.toggle('overview-only', route.view === 'dialogue');
  const old = content.querySelector<HTMLElement>('.stage');
  const scroll = old ? { left: old.scrollLeft, top: old.scrollTop } : { left: 0, top: 0 };
  content.className = `content view-${route.view}`;
  if (route.view === 'overview') {
    const selected = model.quests.get(route.quest);
    const stage = h('div', { class: 'stage' }, renderOverview(model, selected?.id ?? ''));
    content.replaceChildren(stage, ...(selected ? [renderDetail(model, selected)] : []));
    stage.scrollLeft = scroll.left;
    stage.scrollTop = scroll.top;
    if (selected) requestAnimationFrame(() => revealCard(stage, selected.id));
  } else if (route.view === 'dialogue') {
    const g = model.dialogues.graphs.get(route.id);
    content.replaceChildren(dialogueList(model, route.id), g ? h('div', { class: 'dialogue-pane' }, dialogueInfo(model, g), renderDialogue(model, g, route.node)) : h('p', { class: 'panel' }, `Không có lời thoại "${route.id}"`));
  } else if (route.view === 'problems') {
    content.replaceChildren(h('div', { class: 'scroll' }, renderProblems(problems)));
  } else {
    content.replaceChildren(h('div', { class: 'scroll' }, renderFlags(model)));
  }
}

function boot(): void {
  buildFilters();
  const title = h('div', { class: 'brand' }, 'Xưởng truyện', h('small', {}, useFixture ? 'mẫu' : 'chỉ xem'));
  app.replaceChildren(h('header', { class: 'topbar' }, title, tabs, filters), content);
  window.addEventListener('hashchange', render);
  render();
}

boot();
