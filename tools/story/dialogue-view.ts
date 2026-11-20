import { badge, h, svg } from './dom';
import { listenersOf, hashDialogue, hashQuest, type Model } from './model';
import { layoutLayered } from './layout';
import { attachPanZoom } from './panzoom';
import { originColor, tx } from './state';
import { DIALOGUE_END, type Choice, type DialogueNodeDef } from './types';
import type { DialogueGraph } from './dialogues';

const NODE_WIDTH = 300;
const LAYER_GAP = 110;
const ROW_GAP = 22;
const HEADER_Y = 20;

function variantChip(model: Model, key: string): HTMLElement {
  if (key.startsWith('flag:')) return badge(key, 'flag', 'Biến thể khi có cờ này (chọn trước)');
  const chip = badge(key, model.origins.some((o) => o.id === key) ? 'origin' : 'cap', 'Biến thể câu thoại');
  if (model.origins.some((o) => o.id === key)) chip.style.setProperty('--c', originColor(model, key));
  return chip;
}

function answerBadges(model: Model, c: Choice, dialogue: string): HTMLElement[] {
  const out: HTMLElement[] = [];
  if (c.emit) {
    const heard = listenersOf(model, dialogue, c.emit);
    out.push(badge(`gửi ${c.emit}`, heard.length ? 'signal' : 'signal quiet', heard.length ? `Nghe: ${heard.map((l) => l.owner).join(', ')}` : 'Chưa ai nghe tín hiệu này'));
  }
  c.requires?.forEach((x) => out.push(badge(`cần ${x}`, 'cap')));
  c.unless?.forEach((x) => out.push(badge(`trừ ${x}`, 'cap neg')));
  if (c.requires_quest) out.push(h('a', { class: 'badge quest', href: hashQuest(c.requires_quest), title: 'Chỉ hiện khi đang làm quest này' }, `quest ${c.requires_quest}`));
  c.if_flags?.forEach((x) => out.push(badge(`cờ ${x}`, 'flag')));
  c.unless_flags?.forEach((x) => out.push(badge(`không cờ ${x}`, 'flag neg')));
  if (c.mark) out.push(badge(`+1 ${c.mark}`, 'mark', 'Cộng một điểm vào bảng điểm của cuộc nói chuyện'));
  if (c.goto_if) out.push(badge(`${c.goto_if.mark} ≥ ${c.goto_if.at_least} → ${c.goto_if.goto}`, 'goto-if', 'Nếu đủ điểm thì đi tới ô này thay vì ô thường'));
  return out;
}

function nodeBox(model: Model, g: DialogueGraph, id: string, node: DialogueNodeDef): HTMLElement {
  const box = h('div', { class: `dnode${g.reachable.has(id) ? '' : ' orphan'}`, id: `n-${id}`, 'data-node': id });
  const head = h('div', { class: 'dhead' }, h('b', {}, id));
  if (id === g.def.start) head.append(badge('bắt đầu', 'start'));
  if (id === g.def.call) head.append(badge('gọi điện', 'start'));
  if (node.emit) head.append(badge(`gửi ${node.emit}`, 'signal', 'Tự gửi khi cuộc nói chuyện tới ô này'));
  if (node.shop) head.append(badge(`shop ${node.shop}`, 'item'));
  if (node.fee) head.append(badge(`phí ${node.fee}`, 'item'));
  if (node.emote) head.append(badge(node.emote, 'emote'));
  if (node.reset_marks) head.append(badge('xóa điểm', 'mark'));
  if (!node.choices?.length) head.append(badge('kết thúc', 'end'));
  box.append(head, h('div', { class: 'say' }, tx(node.say.default)));
  const variants = Object.keys(node.say).filter((k) => k !== 'default');
  if (variants.length) box.append(h('div', { class: 'variants', title: 'Câu thoại có biến thể theo xuất phát, quyền hay cờ' }, 'biến thể: ', variants.map((k) => variantChip(model, k))));
  const answers = h('ol', { class: 'answers' });
  (node.choices ?? []).forEach((c, i) => {
    const row = h('li', { class: 'answer', 'data-choice': i }, h('span', { class: 'atext' }, tx(c.text)));
    const badges = answerBadges(model, c, g.id);
    if (c.goto === DIALOGUE_END) badges.push(badge('→ kết thúc', 'end'));
    if (badges.length) row.append(h('div', { class: 'abadges' }, badges));
    answers.append(row);
  });
  box.append(answers);
  return box;
}

interface Box {
  el: HTMLElement;
  x: number;
  y: number;
  w: number;
  h: number;
}

function edgeShape(from: Box, rowY: number, to: Box): string {
  const sx = from.x + from.w;
  const sy = from.y + rowY;
  const tx = to.x;
  const ty = to.y + HEADER_Y;
  if (tx > sx + 8) {
    const dx = Math.max(40, (tx - sx) / 2);
    return `M${sx},${sy} C${sx + dx},${sy} ${tx - dx},${ty} ${tx},${ty}`;
  }
  const loop = 70;
  return `M${sx},${sy} C${sx + loop},${sy} ${tx - loop},${ty} ${tx},${ty}`;
}

/** The graph of one NPC's dialogue: layered left to right from the start node, panning and zooming. */
export function renderDialogue(model: Model, g: DialogueGraph, focus: string): HTMLElement {
  const viewport = h('div', { class: 'dialogue-canvas' });
  const world = h('div', { class: 'world' });
  const layer = svg('svg', { class: 'edge-layer' });
  world.append(layer);
  const boxes = new Map<string, Box>();
  const ids = Object.keys(g.def.nodes);
  for (const id of ids) {
    const el = nodeBox(model, g, id, g.def.nodes[id]);
    el.style.width = `${NODE_WIDTH}px`;
    world.append(el);
    boxes.set(id, { el, x: 0, y: 0, w: NODE_WIDTH, h: 0 });
  }
  viewport.append(world);
  const pz = attachPanZoom(viewport, world);
  const place = () => {
    ids.forEach((id) => (boxes.get(id)!.h = boxes.get(id)!.el.offsetHeight));
    const roots = [g.def.start, ...(g.def.call ? [g.def.call] : [])];
    const heights = new Map(ids.map((id) => [id, boxes.get(id)!.h]));
    const placed = layoutLayered(ids, g.edges.filter((e) => e.to in g.def.nodes), roots, heights, ROW_GAP);
    let width = 0;
    let height = 0;
    for (const id of ids) {
      const b = boxes.get(id)!;
      const p = placed.get(id)!;
      b.x = p.layer * (NODE_WIDTH + LAYER_GAP);
      b.y = p.y;
      b.el.style.transform = `translate(${b.x}px, ${b.y}px)`;
      width = Math.max(width, b.x + b.w);
      height = Math.max(height, b.y + b.h);
    }
    drawEdges(g, boxes, layer, width, height);
    pz.setContent(width, height);
    const target = boxes.get(focus);
    if (target) {
      target.el.classList.add('focus');
      pz.focusOn(target.x, target.y, target.w, target.h);
    } else pz.fit();
  };
  requestAnimationFrame(place);
  attachHover(viewport);
  return viewport;
}

function drawEdges(g: DialogueGraph, boxes: Map<string, Box>, layer: SVGElement, width: number, height: number): void {
  layer.setAttribute('width', String(width + 40));
  layer.setAttribute('height', String(height + 40));
  layer.replaceChildren(
    svg('defs', {}, ...['plain', 'cond'].map((k) => svg('marker', { id: `d-${k}`, viewBox: '0 0 10 10', refX: 9, refY: 5, markerWidth: 8, markerHeight: 8, orient: 'auto' }, svg('path', { d: 'M0,1 L10,5 L0,9 z', fill: `var(--edge-${k === 'plain' ? 'requires' : 'cap'})` })))),
  );
  for (const e of g.edges) {
    const from = boxes.get(e.from);
    const to = boxes.get(e.to);
    if (!from || !to) continue;
    const row = from.el.querySelectorAll<HTMLElement>('.answer')[e.choice];
    if (!row) continue;
    const rowY = row.offsetTop + row.offsetHeight / 2;
    const back = to.x <= from.x;
    const far = to.x - from.x > (NODE_WIDTH + LAYER_GAP) * 1.5;
    layer.append(svg('path', { d: edgeShape(from, rowY, to), class: `dedge${e.conditional ? ' cond' : ''}${back ? ' back' : ''}${far ? ' far' : ''}`, 'data-from': e.from, 'data-to': e.to, 'marker-end': `url(#d-${e.conditional ? 'cond' : 'plain'})` }));
  }
}

/** Hovering a node lights its arrows and dims the rest. */
function attachHover(viewport: HTMLElement): void {
  viewport.addEventListener('mouseover', (ev) => {
    const node = (ev.target as HTMLElement).closest<HTMLElement>('.dnode');
    viewport.querySelectorAll('.dedge').forEach((p) => {
      const hot = node !== null && (p.getAttribute('data-from') === node.dataset.node || p.getAttribute('data-to') === node.dataset.node);
      p.classList.toggle('hot', hot);
      p.classList.toggle('quiet', node !== null && !hot);
    });
  });
  viewport.addEventListener('mouseleave', () => viewport.querySelectorAll('.dedge').forEach((p) => p.classList.remove('hot', 'quiet')));
}

/** The left list of dialogues: one per NPC, with who speaks it. */
export function dialogueList(model: Model, current: string): HTMLElement {
  const list = h('nav', { class: 'dlist' });
  for (const g of model.dialogues.graphs.values()) {
    const names = g.speakers.map((id) => model.content.npcs.find((n) => n.id === id)?.name ?? id);
    list.append(h('a', { href: hashDialogue(g.id), class: g.id === current ? 'current' : '' }, h('b', {}, names[0] ?? g.id), h('small', {}, `${g.id} · ${Object.keys(g.def.nodes).length} ô`)));
  }
  return list;
}

export function dialogueInfo(model: Model, g: DialogueGraph): HTMLElement {
  const info = h('div', { class: 'dinfo' }, h('h2', {}, g.speakers.map((id) => model.content.npcs.find((n) => n.id === id)?.name ?? id).join(', ') || g.id), h('span', { class: 'id' }, `${g.id}.json`));
  const role = model.content.npcs.find((n) => n.id === g.speakers[0])?.role;
  if (role) info.append(h('span', { class: 'role' }, tx(role)));
  if (g.def.about) info.append(h('p', { class: 'about' }, g.def.about));
  return info;
}
