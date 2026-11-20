import { h, svg } from './dom';
import { lineName, questCard } from './cards';
import type { EdgeKind, Model, QuestEdge, QuestNode } from './model';
import { state } from './state';
import { input } from './fields';
import { QUEST_ID_PATTERN } from './schema';

/** What the overview needs to add a quest to a questline. */
export interface OverviewEditor {
  newQuest(line: string, id: string): void;
}

const BOW = 34;
const EDGE_ARROWS: EdgeKind[] = ['requires', 'cap', 'flag', 'unless'];

/** Quests of a line in the order they can be done: by the length of their requires chain, then file order. */
function orderLine(model: Model, quests: QuestNode[]): QuestNode[] {
  const inLine = new Set(quests.map((q) => q.id));
  const depth = new Map<string, number>();
  const walk = (q: QuestNode, seen: Set<string>): number => {
    const known = depth.get(q.id);
    if (known !== undefined) return known;
    if (seen.has(q.id)) return 0;
    seen.add(q.id);
    let d = 0;
    for (const r of q.def.requires ?? []) {
      const req = model.quests.get(r);
      if (req && inLine.has(r)) d = Math.max(d, walk(req, seen) + 1);
    }
    depth.set(q.id, d);
    return d;
  };
  quests.forEach((q) => walk(q, new Set()));
  return quests.map((q, i) => ({ q, i })).sort((a, b) => depth.get(a.q.id)! - depth.get(b.q.id)! || a.i - b.i).map((x) => x.q);
}

function column(model: Model, line: string, quests: QuestNode[], selected: string, editor?: OverviewEditor): HTMLElement {
  const col = h('section', { class: 'column' }, h('h2', {}, lineName(line), h('small', {}, line)));
  for (const q of orderLine(model, quests)) col.append(questCard(model, q, { selected: q.id === selected }));
  if (editor) {
    const id = input('', (v) => {
      if (!QUEST_ID_PATTERN.test(v) || model.quests.has(v)) {
        id.classList.add('invalid');
        return;
      }
      editor.newQuest(line, v);
    }, { placeholder: '+ quest mới: nhập id rồi Enter', mono: true, title: 'id dạng story.ten_quest (chữ thường, số, _ và dấu chấm)' });
    col.append(h('div', { class: 'newquest no-pan' }, id));
  }
  return col;
}

function sideGroup(model: Model, selected: string): HTMLElement {
  const side = [...model.quests.values()].filter((q) => q.kind !== 'story');
  const group = h('details', { class: 'side-group' }, h('summary', {}, `Hằng ngày, việc vặt, thành tích (${side.length})`));
  if (state.showSide) group.setAttribute('open', '');
  group.addEventListener('toggle', () => (state.showSide = group.open));
  const kinds: [string, string][] = [['daily', 'Hằng ngày'], ['job', 'Việc vặt'], ['achievement', 'Thành tích']];
  const grid = h('div', { class: 'side-grid' });
  for (const [kind, name] of kinds) {
    const quests = side.filter((q) => q.kind === kind);
    if (quests.length === 0) continue;
    const col = h('section', { class: 'column' }, h('h2', {}, name, h('small', {}, `${quests.length}`)));
    quests.forEach((q) => col.append(questCard(model, q, { selected: q.id === selected })));
    grid.append(col);
  }
  group.append(grid);
  return group;
}

const rect = (el: Element, origin: DOMRect) => {
  const r = el.getBoundingClientRect();
  return { l: r.left - origin.left, r: r.right - origin.left, t: r.top - origin.top, b: r.bottom - origin.top };
};

/** `spread` (0..1) moves the start down the source card, so arrows leaving one card side by side do not overlap. */
function edgePath(from: ReturnType<typeof rect>, to: ReturnType<typeof rect>, spread = 0.5): { d: string; mid: [number, number]; end: [number, number] } {
  const fcx = (from.l + from.r) / 2;
  const tcx = (to.l + to.r) / 2;
  const sameColumn = Math.abs(fcx - tcx) < 40;
  if (sameColumn) {
    const adjacent = to.t >= from.b && to.t - from.b < 36;
    if (adjacent) return { d: `M${fcx},${from.b} L${tcx},${to.t}`, mid: [fcx, (from.b + to.t) / 2], end: [tcx, to.t] };
    const sy = from.t + (from.b - from.t) * spread;
    const ty = (to.t + to.b) / 2;
    const x = Math.max(from.r, to.r);
    return { d: `M${from.r},${sy} C${x + BOW * 2},${sy} ${x + BOW * 2},${ty} ${to.r},${ty}`, mid: [x + BOW * 1.5, (sy + ty) / 2], end: [to.r, ty] };
  }
  const forward = fcx < tcx;
  const sx = forward ? from.r : from.l;
  const tx = forward ? to.l : to.r;
  const sy = from.t + (from.b - from.t) * spread;
  const ty = (to.t + to.b) / 2;
  const dx = Math.max(40, Math.abs(tx - sx) / 2);
  const dir = forward ? 1 : -1;
  return { d: `M${sx},${sy} C${sx + dx * dir},${sy} ${tx - dx * dir},${ty} ${tx},${ty}`, mid: [(sx + tx) / 2, (sy + ty) / 2], end: [tx, ty] };
}

function marker(id: string, color: string): SVGElement {
  return svg('marker', { id, viewBox: '0 0 10 10', refX: 9, refY: 5, markerWidth: 7, markerHeight: 7, orient: 'auto-start-reverse' }, svg('path', { d: 'M0,1 L10,5 L0,9 z', fill: color }));
}

/** Draws the arrows over the cards once the cards are on the page and have their sizes. */
function drawEdges(model: Model, root: HTMLElement, layer: SVGElement, labels: SVGElement, selected: string): void {
  const origin = root.getBoundingClientRect();
  for (const l of [layer, labels]) {
    l.setAttribute('width', String(root.scrollWidth));
    l.setAttribute('height', String(root.scrollHeight));
  }
  labels.replaceChildren();
  const defs = svg('defs', {}, ...EDGE_ARROWS.map((k) => marker(`arrow-${k}`, `var(--edge-${k})`)), marker('arrow-hot', 'var(--accent)'));
  layer.replaceChildren(defs);
  const cards = new Map<string, HTMLElement>();
  root.querySelectorAll<HTMLElement>('[data-quest]').forEach((c) => cards.set(c.dataset.quest!, c));
  const wanted = (e: QuestEdge) => (e.kind === 'unless' ? state.show.flag : state.show[e.kind]);
  const labelled: [number, number, string][] = [];
  const shown = model.edges.filter(wanted);
  const leaving = (e: QuestEdge) => shown.filter((x) => x.from === e.from && x.kind !== 'requires' && x.kind !== 'cap');
  for (const e of shown) {
    const a = cards.get(e.from);
    const b = cards.get(e.to);
    if (!a || !b || a.offsetParent === null || b.offsetParent === null) continue;
    const siblings = leaving(e);
    const spread = siblings.length > 1 && e.kind !== 'requires' && e.kind !== 'cap' ? (siblings.indexOf(e) + 1) / (siblings.length + 1) : 0.5;
    const { d, end } = edgePath(rect(a, origin), rect(b, origin), spread);
    const hot = selected !== '' && (e.from === selected || e.to === selected);
    const faded = a.classList.contains('dim') && b.classList.contains('dim');
    const path = svg('path', { d, class: `edge ${e.kind}${hot ? ' hot' : ''}${selected && !hot ? ' quiet' : ''}${faded ? ' faded' : ''}`, 'marker-end': `url(#arrow-${hot ? 'hot' : e.kind})` });
    path.append(svg('title', {}, `${e.from} → ${e.to}${e.label ? ` (${e.label})` : ''}`));
    layer.append(path);
    if ((e.kind === 'flag' || e.kind === 'unless') && e.label) labelled.push([end[0] - 50, end[1] - 9, `${e.kind === 'unless' ? 'không: ' : ''}${e.label}`]);
    else if (hot && e.kind === 'cap') labelled.push([end[0] - 50, end[1] - 9, e.label]);
  }
  for (const [x, y, text] of labelled) {
    const t = svg('text', { x, y, class: 'edge-label', 'text-anchor': 'middle', 'dominant-baseline': 'middle' }, text);
    labels.append(t);
    const box = (t as unknown as SVGGraphicsElement).getBBox();
    labels.insertBefore(svg('rect', { x: box.x - 4, y: box.y - 1, width: box.width + 8, height: box.height + 2, rx: 3, class: 'edge-label-bg' }), t);
  }
}

/** The overview map: questlines as columns, quests as cards, arrows between them. */
export function renderOverview(model: Model, selected: string, editor?: OverviewEditor): HTMLElement {
  const root = h('div', { class: 'overview' });
  const layer = svg('svg', { class: 'edge-layer' });
  const labels = svg('svg', { class: 'edge-layer labels' });
  const columns = h('div', { class: 'columns' });
  for (const line of model.storyLines) {
    const quests = [...model.quests.values()].filter((q) => q.kind === 'story' && q.line === line);
    columns.append(column(model, line, quests, selected, editor));
  }
  root.append(layer, columns, sideGroup(model, selected), labels);
  const redraw = () => drawEdges(model, root, layer, labels, selected);
  requestAnimationFrame(redraw);
  root.addEventListener('toggle', redraw, true);
  new ResizeObserver(redraw).observe(columns);
  return root;
}
