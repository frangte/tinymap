import { badge, h, svg } from './dom';
import { dialogueTitle, listenersOf, hashDialogue, hashQuest, type Model } from './model';
import { layoutLayered } from './layout';
import { attachPanZoom, type PanZoom, type ViewState } from './panzoom';
import { dialogueToolbar, editableBody, nodeTools, type DialogueEditor } from './dialogue-edit';
import { originColor, state, tx } from './state';
import { DIALOGUE_END, type Choice, type DialogueDef, type DialogueNodeDef } from './types';
import type { DialogueGraph } from './dialogues';

const NODE_WIDTH = 300;
const EDIT_NODE_WIDTH = 420;
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

function nodeBox(model: Model, g: DialogueGraph, id: string, node: DialogueNodeDef, editor?: DialogueEditor): HTMLElement {
  const marked = state.marks.has(hashDialogue(g.id, id));
  const box = h('div', { class: `dnode${g.reachable.has(id) ? '' : ' orphan'}${marked ? ' broken' : ''}${editor ? ' editing' : ''}`, id: `n-${id}`, 'data-node': id });
  const head = h('div', { class: 'dhead' }, h('b', {}, id));
  if (id === g.def.start) head.append(badge('bắt đầu', 'start'));
  if (id === g.def.call) head.append(badge('gọi điện', 'start'));
  if (node.emit) head.append(badge(`gửi ${node.emit}`, 'signal', 'Tự gửi khi cuộc nói chuyện tới ô này'));
  if (node.shop) head.append(badge(`shop ${node.shop}`, 'item'));
  if (node.fee) head.append(badge(`phí ${node.fee}`, 'item'));
  if (node.emote) head.append(badge(node.emote, 'emote'));
  if (node.reset_marks) head.append(badge('xóa điểm', 'mark'));
  if (!node.choices?.length) head.append(badge('kết thúc', 'end'));
  if (editor) {
    head.append(nodeTools(editor, g.def, id));
    box.append(head, ...editableBody(model, editor, g.def, node));    return box;
  }
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

/** Where each dialogue's view was left, so an edit redraws it in place. */
const views = new Map<string, ViewState>();
const lastFocus = new Map<string, string>();

/** The graph of one NPC's dialogue: layered left to right from the start node, panning and zooming. */
export function renderDialogue(model: Model, g: DialogueGraph, focus: string, editor?: DialogueEditor): HTMLElement {
  const width = editor ? EDIT_NODE_WIDTH : NODE_WIDTH;
  const viewport = h('div', { class: 'dialogue-canvas' });
  const world = h('div', { class: 'world' });
  const layer = svg('svg', { class: 'edge-layer' });
  world.append(layer);
  const boxes = new Map<string, Box>();
  const ids = Object.keys(g.def.nodes);
  for (const id of ids) {
    const el = nodeBox(model, g, id, g.def.nodes[id], editor);
    el.style.width = `${width}px`;
    world.append(el);
    boxes.set(id, { el, x: 0, y: 0, w: width, h: 0 });
  }
  viewport.append(world);
  const pz = attachPanZoom(viewport, world);
  pz.onChange((view) => views.set(g.id, view));
  let first = true;
  const place = () => {
    ids.forEach((id) => (boxes.get(id)!.h = boxes.get(id)!.el.offsetHeight));
    const roots = [g.def.start, ...(g.def.call ? [g.def.call] : [])];
    const heights = new Map(ids.map((id) => [id, boxes.get(id)!.h]));
    const placed = layoutLayered(ids, g.edges.filter((e) => e.to in g.def.nodes), roots, heights, ROW_GAP);
    let right = 0;
    let bottom = 0;
    for (const id of ids) {
      const b = boxes.get(id)!;
      const p = placed.get(id)!;
      b.x = p.layer * (width + LAYER_GAP);
      b.y = p.y;
      b.el.style.transform = `translate(${b.x}px, ${b.y}px)`;
      right = Math.max(right, b.x + b.w);
      bottom = Math.max(bottom, b.y + b.h);
    }
    drawEdges(g, boxes, layer, right, bottom, width);
    pz.setContent(right, bottom);
    if (!first) return;
    first = false;
    const saved = views.get(g.id);
    const target = boxes.get(focus);
    if (target) target.el.classList.add('focus');
    const refocus = target !== undefined && focus !== lastFocus.get(g.id);
    lastFocus.set(g.id, focus);
    if (saved && !refocus) pz.setState(saved);
    else if (target) pz.focusOn(target.x, target.y, target.w, target.h);
    else {
      const start = boxes.get(g.def.start);
      pz.anchorAt(start?.x ?? 0, start?.y ?? 0);
    }
  };
  requestAnimationFrame(place);
  let scheduled = 0;
  const observer = new ResizeObserver(() => {
    cancelAnimationFrame(scheduled);
    scheduled = requestAnimationFrame(place);
  });
  boxes.forEach((b) => observer.observe(b.el));
  attachHover(viewport);
  if (editor) attachConnect(viewport, layer, pz, boxes, g, editor);
  return viewport;
}

/** Dragging a choice's handle onto a node (or the "kết thúc" target) makes that its goto. */
function attachConnect(viewport: HTMLElement, layer: SVGElement, pz: PanZoom, boxes: Map<string, Box>, g: DialogueGraph, editor: DialogueEditor): void {
  const endDrop = h('div', { class: 'end-drop no-pan' }, 'Thả vào đây: kết thúc cuộc nói chuyện');
  viewport.append(endDrop);
  viewport.addEventListener('pointerdown', (ev) => {
    const handle = (ev.target as HTMLElement).closest<HTMLElement>('.handle');
    if (!handle) return;
    ev.preventDefault();
    ev.stopPropagation();
    const fromEl = handle.closest<HTMLElement>('.dnode')!;
    const fromId = fromEl.dataset.node!;
    const from = boxes.get(fromId)!;
    const choice = Number(handle.dataset.choice);
    const sx = from.x + from.w;
    const sy = from.y + (handle.getBoundingClientRect().top - fromEl.getBoundingClientRect().top) / pz.state().scale + handle.offsetHeight / 2;
    const path = svg('path', { class: 'dedge dragging' });
    layer.append(path);
    viewport.classList.add('connecting');
    let hover = '';
    const over = (x: number, y: number): string => {
      const hit = document.elementsFromPoint(x, y);
      if (hit.some((e) => e === endDrop)) return DIALOGUE_END;
      return hit.find((e) => e.classList.contains('dnode'))?.getAttribute('data-node') ?? '';
    };
    const move = (e: PointerEvent) => {
      const p = pz.toWorld(e.clientX, e.clientY);
      path.setAttribute('d', `M${sx},${sy} C${sx + 80},${sy} ${p.x - 80},${p.y} ${p.x},${p.y}`);
      hover = over(e.clientX, e.clientY);
      boxes.forEach((b, id) => b.el.classList.toggle('drop', id === hover));
      endDrop.classList.toggle('drop', hover === DIALOGUE_END);
    };
    const up = () => {
      document.removeEventListener('pointermove', move);
      document.removeEventListener('pointerup', up);
      viewport.classList.remove('connecting');
      path.remove();
      boxes.forEach((b) => b.el.classList.remove('drop'));
      endDrop.classList.remove('drop');
      if (hover) editor.commit(() => setChoiceGoto(g.def, fromId, choice, hover));
    };
    document.addEventListener('pointermove', move);
    document.addEventListener('pointerup', up);
    move(ev);
  });
}

function setChoiceGoto(def: DialogueDef, node: string, choice: number, target: string): void {
  const c = def.nodes[node]?.choices?.[choice];
  if (c) c.goto = target;
}

function drawEdges(g: DialogueGraph, boxes: Map<string, Box>, layer: SVGElement, width: number, height: number, nodeWidth: number): void {
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
    const anchor = row.querySelector<HTMLElement>('.handle') ?? row;
    const rowY = anchor.offsetTop + anchor.offsetHeight / 2 + (anchor.offsetParent === from.el ? 0 : (anchor.offsetParent as HTMLElement | null)?.offsetTop ?? 0);
    const back = to.x <= from.x;
    const far = to.x - from.x > (nodeWidth + LAYER_GAP) * 1.5;
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
    list.append(h('a', { href: hashDialogue(g.id), class: g.id === current ? 'current' : '' }, h('b', {}, dialogueTitle(model, g.id)), h('small', {}, `${g.id} · ${Object.keys(g.def.nodes).length} ô`)));
  }
  return list;
}

export function dialogueInfo(model: Model, g: DialogueGraph, editor?: DialogueEditor): HTMLElement {
  const info = h('div', { class: 'dinfo' }, h('h2', {}, dialogueTitle(model, g.id)), h('span', { class: 'id' }, `${g.id}.json`));
  const role = model.content.npcs.find((n) => n.id === g.speakers[0])?.role ?? model.content.contacts.find((c) => c.id === g.speakers[0])?.role;
  if (role) info.append(h('span', { class: 'role' }, tx(role)));
  if (g.def.about) info.append(h('p', { class: 'about' }, g.def.about));
  if (editor) info.append(dialogueToolbar(editor, g.def));
  return info;
}
