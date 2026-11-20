import { OBJECTIVE_NAMES, lineName, originChips, rewardsSummary } from './cards';
import { badge, h, link, type Child } from './dom';
import { speakerName, dialogueTitle, emissionsFor, hashDialogue, hashQuest, outcomeFlag, questTitle, type CapSource, type Model, type QuestNode } from './model';
import { state, tx } from './state';
import type { Objective, QuestText, Rewards } from './types';

const section = (title: string, ...body: Child[]): HTMLElement => h('section', { class: 'sect' }, h('h3', {}, title), ...body);

function questLink(model: Model, id: string): HTMLElement {
  const q = model.quests.get(id);
  return link(q ? `${questTitle(q.def)}` : id, hashQuest(id), q ? '' : 'missing');
}

function capSourceText(model: Model, s: CapSource): Child {
  switch (s.kind) {
    case 'quest':
      return ['quest ', questLink(model, s.quest)];
    case 'item':
      return `đồ ${s.id}`;
    case 'fee':
      return `phí ${s.id}`;
    case 'archetype':
      return `xuất phát ${s.id}`;
  }
}

/** Where an objective points: an NPC (to its dialogue) or a place. */
function atText(model: Model, at: string): Child {
  const dialogue = model.dialogues.npcDialogue.get(at);
  if (dialogue) return link(speakerName(model, at), hashDialogue(dialogue), 'npc');
  const place = model.content.places.find((p) => p.id === at);
  return place ? `${place.name?.vi ?? at} (${at})` : at;
}

function paramText(key: string, value: unknown): string {
  if (typeof value === 'object' && value !== null) return `${key}: ${JSON.stringify(value)}`;
  return `${key}: ${String(value)}`;
}

/** The dialogue nodes and answers that send the signal an objective waits for. */
export function signalSources(model: Model, o: Objective): HTMLElement | null {
  const p = o.params ?? {};
  const signals: { npc: string; signal: string }[] = [];
  if (o.type === 'dialogue_signal' && typeof p.npc === 'string' && typeof p.signal === 'string') {
    for (const npc of [p.npc, ...(Array.isArray(p.also) ? (p.also as string[]) : [])]) signals.push({ npc, signal: p.signal });
  }
  if (o.type === 'errand') {
    if (typeof p.npc === 'string' && typeof p.signal === 'string') signals.push({ npc: p.npc, signal: p.signal });
    for (const s of (Array.isArray(p.stops) ? p.stops : []) as { npc?: string; signal?: string }[]) {
      if (s.npc && s.signal) signals.push({ npc: s.npc, signal: s.signal });
    }
  }
  if (signals.length === 0) return null;
  const list = h('ul', { class: 'sources' });
  for (const s of signals) {
    const found = emissionsFor(model, s.npc, s.signal);
    if (found.length === 0) {
      list.append(h('li', { class: 'bad' }, `"${s.signal}" từ ${s.npc}: không câu trả lời nào gửi`));
      continue;
    }
    for (const e of found) {
      const node = model.dialogues.graphs.get(e.dialogue)?.def.nodes[e.node];
      const answer = e.choice >= 0 ? tx(node?.choices?.[e.choice]?.text) : '(ô này tự gửi khi tới)';
      list.append(h('li', {}, badge(s.signal, 'signal'), ' ', link(`${dialogueTitle(model, e.dialogue)} / ${e.node}`, hashDialogue(e.dialogue, e.node)), h('span', { class: 'quote' }, ` “${answer}”`)));
    }
  }
  return list;
}

function objectiveBlock(model: Model, o: Objective, index: number): HTMLElement {
  const block = h('li', { class: 'objective' });
  const head = h('div', { class: 'ohead' }, h('b', {}, `${index + 1}. ${OBJECTIVE_NAMES[o.type] ?? o.type}`), badge(o.type, 'mono'));
  if (o.count && o.count > 1) head.append(badge(`×${o.count}`, 'count'));
  block.append(head);
  if (o.text) block.append(h('div', { class: 'tracker' }, tx(o.text)));
  const params = Object.entries(o.params ?? {}).filter(([k]) => k !== 'stops');
  if (params.length) block.append(h('div', { class: 'params' }, params.map(([k, v]) => badge(paramText(k, v), 'mono'))));
  if (o.at) block.append(h('div', { class: 'at' }, 'chỉ tới: ', atText(model, o.at)));
  if (o.id) block.append(h('div', { class: 'oid' }, `id ${o.id}`));
  const sources = signalSources(model, o);
  if (sources) block.append(sources);
  return block;
}

function rewardsBlock(r: Rewards | undefined): HTMLElement | null {
  const items = rewardsSummary(r);
  if (!r || items.length === 0) return null;
  const detail = [...(r.titles ?? []).map((t) => `danh hiệu ${t}`), ...(r.stamps ?? []).map((t) => `tem ${t}`), ...(r.flags ?? []).map((f) => `cờ ${f}`)];
  return h('div', { class: 'params' }, [...items, ...detail].map((t) => badge(t, 'reward')));
}

function textsBlock(title: string, texts: QuestText[] | undefined): HTMLElement | null {
  if (!texts?.length) return null;
  return h('div', {}, h('h4', {}, title), ...texts.map((t) => h('p', { class: 'sms' }, h('b', {}, `${t.from}: `), tx(t.text))));
}

function outcomesBlock(model: Model, q: QuestNode): HTMLElement | null {
  const outcomes = q.def.outcomes;
  if (!outcomes?.length) return null;
  const block = section('Kết cục', h('p', { class: 'hint' }, 'Khi xong các mục tiêu trên, các kết cục mở cùng lúc; cái nào xong trước thì quest kết thúc.'));
  outcomes.forEach((o, i) => {
    const box = h('div', { class: 'outcome' }, h('div', { class: 'ohead' }, h('b', {}, tx(o.title, o.id ?? `kết cục ${i + 1}`)), badge(outcomeFlag(q.id, o.id ?? ''), 'flag')));
    box.append(h('ol', { class: 'objectives' }, objectiveBlock(model, o, 0)));
    const extra = [rewardsBlock(o.rewards), ...(o.flags ?? []).map((f) => badge(`cờ ${f}`, 'flag')), textsBlock('Tin nhắn khi xong', o.texts?.on_done)];
    box.append(...extra.filter(Boolean) as Node[]);
    block.append(box);
  });
  return block;
}

/** The side panel for one quest. */
export function renderDetail(model: Model, q: QuestNode): HTMLElement {
  const def = q.def;
  const panel = h('aside', { class: 'detail' });
  panel.append(
    h('div', { class: 'detail-head' }, h('a', { class: 'close', href: '#/', title: 'Đóng' }, '×'), h('h2', {}, tx(def.title, def.id)), h('div', { class: 'id' }, `${def.id} · ${lineName(q.line)}`), originChips(model, q)),
  );
  if (def.about) panel.append(h('p', { class: 'about' }, tx(def.about)));
  const gates: Child[] = [];
  if (def.requires?.length) gates.push(h('div', {}, 'Cần xong: ', def.requires.flatMap((r, i) => [i ? ', ' : '', questLink(model, r)])));
  for (const cap of def.requires_caps ?? []) {
    const sources = model.capSources.get(cap) ?? [];
    gates.push(h('div', {}, badge(`cần ${cap}`, 'cap'), ' cấp bởi: ', sources.length ? sources.flatMap((s, i) => [i ? ', ' : '', capSourceText(model, s)]) : h('span', { class: 'bad' }, 'không ai')));
  }
  for (const cap of def.unless_caps ?? []) gates.push(h('div', {}, badge(`trừ ${cap}`, 'cap neg'), ' không mời khi đã có'));
  for (const f of def.requires_flags ?? []) gates.push(h('div', {}, badge(`cờ ${f}`, 'flag'), ' phải có'));
  for (const f of def.unless_flags ?? []) gates.push(h('div', {}, badge(`cờ ${f}`, 'flag neg'), ' không được có'));
  if (def.repeat) gates.push(h('div', {}, `lặp: ${def.repeat}${def.pool ? `, nhóm ${def.pool}` : ''}`));
  if (gates.length) panel.append(section('Điều kiện', ...gates));
  panel.append(section(`Mục tiêu (${def.flow ?? 'sequence'})`, h('ol', { class: 'objectives' }, def.objectives.map((o, i) => objectiveBlock(model, o, i)))));
  const outcomes = outcomesBlock(model, q);
  if (outcomes) panel.append(outcomes);
  const rewards = rewardsBlock(def.rewards);
  if (rewards) panel.append(section('Phần thưởng', rewards));
  const texts = [textsBlock('Tin nhắn khi bắt đầu', def.texts?.on_start), textsBlock('Tin nhắn khi xong', def.texts?.on_done)];
  if (texts.some(Boolean)) panel.append(section('Tin nhắn', ...texts));
  const next = model.edges.filter((e) => e.from === q.id);
  if (next.length) {
    panel.append(section('Mở ra', h('ul', {}, next.map((e) => h('li', {}, badge(e.kind === 'requires' ? 'xong quest' : e.kind === 'cap' ? e.label : e.label || 'cờ', e.kind), ' ', questLink(model, e.to))))));
  }
  panel.dataset.lang = state.lang;
  return panel;
}
