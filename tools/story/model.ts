import { buildDialogueIndex, type DialogueIndex } from './dialogues';
import type { Archetype, Objective, QuestDef, RawContent, Text } from './types';

/** How a quest is shown: the story proper, or the repeating and passive ones that go in their own group. */
export type QuestKind = 'story' | 'daily' | 'job' | 'achievement';

export interface QuestNode {
  id: string;
  def: QuestDef;
  kind: QuestKind;
  line: string;
  /** archetypes (origins) the quest's tags_any offers it to */
  origins: Set<string>;
  /** the origins that can actually reach it: offered, and every required quest reachable by them too */
  effective: Set<string>;
  /** flags it sets: its rewards, and each outcome's own plus `<quest>:<outcome>` */
  flagsSet: string[];
}

export type EdgeKind = 'requires' | 'cap' | 'flag' | 'unless';

export interface QuestEdge {
  kind: EdgeKind;
  from: string;
  to: string;
  /** what the arrow says: the capability, the outcome's title, the flag */
  label: string;
  outcome?: string;
}

/** A signal a quest (or the collection, or a shop) waits for from an NPC's dialogue. */
export interface SignalListener {
  signal: string;
  npc: string;
  source: 'quest' | 'collection' | 'haggle' | 'engine';
  /** quest id, collection entry id, shop id */
  owner: string;
  objective?: string;
}

export type CapSource =
  | { kind: 'quest'; quest: string }
  | { kind: 'item'; id: string }
  | { kind: 'fee'; id: string }
  | { kind: 'archetype'; id: string };

/** A place in the content, for a link: what to say and where the page goes. */
export interface Place {
  text: string;
  hash: string;
}

export interface FlagUse {
  where: Place;
  how: string;
}

export interface FlagInfo {
  id: string;
  sets: FlagUse[];
  reads: FlagUse[];
}

export interface Model {
  content: RawContent;
  origins: Archetype[];
  quests: Map<string, QuestNode>;
  /** questline ids of the story group, in story order */
  storyLines: string[];
  edges: QuestEdge[];
  dialogues: DialogueIndex;
  listeners: SignalListener[];
  capSources: Map<string, CapSource[]>;
  flags: Map<string, FlagInfo>;
}

/** The questlines in story order; ones not listed follow in the order the file has them. */
const LINE_ORDER = ['chuong_0_que', 'chuong_0_nuoc_ngoai', 'chuong_1', 'song_o_pho', 'chuong_3', 'tay_lai', 'khach_crab', 'chuong_4', 'chuong_5', 'chuong_6', 'viec_vat'];

/** Signals the Go engine reads itself (server/internal/driving), not a quest. */
const ENGINE_SIGNALS: { npc: string; signal: string }[] = [
  { npc: 'thay_muoi', signal: 'thi_bat_dau' },
  { npc: 'thay_muoi', signal: 'thuc_hanh_bat_dau' },
];

export const hashQuest = (id: string): string => `#/quest/${id}`;
export const hashDialogue = (id: string, node?: string): string => `#/dialogue/${id}${node ? `/${node}` : ''}`;

export const questTitle = (def: { title?: Text; id: string }): string => def.title?.vi ?? def.id;

function kindOf(def: QuestDef): QuestKind {
  if (def.job) return 'job';
  return def.repeat === 'daily' ? 'daily' : 'story';
}

/** Every objective of a quest, its outcomes included. */
export function allObjectives(def: QuestDef): Objective[] {
  return [...def.objectives, ...(def.outcomes ?? [])];
}

export function outcomeFlag(quest: string, outcome: string): string {
  return `${quest}:${outcome}`;
}

function offeredTo(tags: string[] | undefined, origins: Archetype[]): Set<string> {
  if (!tags || tags.length === 0) return new Set(origins.map((o) => o.id));
  return new Set(origins.filter((o) => o.tags.some((t) => tags.includes(t))).map((o) => o.id));
}

function collectQuests(content: RawContent): QuestDef[] {
  const all: QuestDef[] = [];
  all.push(...content.quests.quests);
  for (const a of content.quests.achievements ?? []) {
    all.push({
      id: a.id,
      title: a.title,
      about: a.about,
      tags_any: a.tags_any,
      questline: 'thanh_tich',
      flow: 'sequence',
      objectives: [a.objective],
      rewards: a.rewards,
    });
  }
  return all;
}

/** Walks `requires` to find who can really get to each quest; a cycle or an unknown quest leaves it empty. */
function computeEffective(quests: Map<string, QuestNode>): void {
  const state = new Map<string, 'busy' | 'done'>();
  const visit = (id: string): Set<string> => {
    const node = quests.get(id);
    if (!node) return new Set();
    if (state.get(id) === 'done') return node.effective;
    if (state.get(id) === 'busy') return new Set();
    state.set(id, 'busy');
    let result = new Set(node.origins);
    for (const req of node.def.requires ?? []) {
      const reqOrigins = visit(req);
      result = new Set([...result].filter((o) => reqOrigins.has(o)));
    }
    node.effective = result;
    state.set(id, 'done');
    return result;
  };
  for (const id of quests.keys()) visit(id);
}

function capSources(content: RawContent, quests: Map<string, QuestNode>): Map<string, CapSource[]> {
  const map = new Map<string, CapSource[]>();
  const add = (cap: string, source: CapSource) => map.set(cap, [...(map.get(cap) ?? []), source]);
  for (const a of content.archetypes.archetypes) a.capabilities?.forEach((c) => add(c, { kind: 'archetype', id: a.id }));
  for (const i of content.items.items ?? []) i.grants?.forEach((c) => add(c, { kind: 'item', id: i.id }));
  for (const f of content.items.fees ?? []) f.grants?.forEach((c) => add(c, { kind: 'fee', id: f.id }));
  for (const q of quests.values()) {
    const caps = [q.def.rewards?.capability, ...(q.def.outcomes ?? []).map((o) => o.rewards?.capability)];
    for (const c of caps) if (c) add(c, { kind: 'quest', quest: q.id });
  }
  return map;
}

/** Capabilities every item a buy_item objective could mean grants (an item by id, or any of a kind). */
function boughtCaps(content: RawContent, o: Objective): string[] {
  if (o.type !== 'buy_item') return [];
  const items = content.items.items ?? [];
  const bought = items.filter((i) => i.id === o.params?.item || (o.params?.kind !== undefined && i.kind === o.params.kind));
  if (bought.length === 0) return [];
  return (bought[0].grants ?? []).filter((c) => bought.every((i) => i.grants?.includes(c)));
}

function buildEdges(content: RawContent, quests: Map<string, QuestNode>): QuestEdge[] {
  const edges: QuestEdge[] = [];
  const granting = new Map<string, Set<string>>();
  const grant = (cap: string, quest: string) => granting.set(cap, (granting.get(cap) ?? new Set()).add(quest));
  for (const q of quests.values()) {
    if (q.def.rewards?.capability) grant(q.def.rewards.capability, q.id);
    q.def.outcomes?.forEach((o) => o.rewards?.capability && grant(o.rewards.capability, q.id));
    for (const o of q.def.objectives) boughtCaps(content, o).forEach((c) => grant(c, q.id));
  }
  for (const q of quests.values()) {
    for (const r of q.def.requires ?? []) {
      if (quests.has(r)) edges.push({ kind: 'requires', from: r, to: q.id, label: '' });
    }
    for (const cap of q.def.requires_caps ?? []) {
      for (const from of granting.get(cap) ?? []) if (from !== q.id) edges.push({ kind: 'cap', from, to: q.id, label: cap });
    }
    const flagEdges = (flags: string[] | undefined, kind: EdgeKind) => {
      for (const flag of flags ?? []) {
        for (const p of quests.values()) {
          for (const o of p.def.outcomes ?? []) {
            const sets = [outcomeFlag(p.id, o.id ?? ''), ...(o.flags ?? [])];
            if (sets.includes(flag)) edges.push({ kind, from: p.id, to: q.id, label: o.title?.vi ?? o.id ?? flag, outcome: o.id });
          }
          if (p.def.rewards?.flags?.includes(flag)) edges.push({ kind, from: p.id, to: q.id, label: flag });
        }
      }
    };
    flagEdges(q.def.requires_flags, 'flag');
    flagEdges(q.def.unless_flags, 'unless');
  }
  return edges;
}

/** Every signal wait in the content: quests (dialogue_signal, errand stops), the collection and shop haggling. */
function buildListeners(content: RawContent, quests: Map<string, QuestNode>): SignalListener[] {
  const out: SignalListener[] = [];
  const fromObjective = (o: Objective, source: 'quest' | 'collection', owner: string) => {
    const p = o.params ?? {};
    const npc = typeof p.npc === 'string' ? p.npc : '';
    const signal = typeof p.signal === 'string' ? p.signal : '';
    const also = Array.isArray(p.also) ? (p.also as string[]) : [];
    if (o.type === 'dialogue_signal' && signal) {
      for (const n of [npc, ...also]) out.push({ signal, npc: n, source, owner, objective: o.id });
    }
    if (o.type === 'errand') {
      if (signal) out.push({ signal, npc, source, owner, objective: o.id });
      const stops = Array.isArray(p.stops) ? (p.stops as { npc?: string; signal?: string }[]) : [];
      for (const s of stops) if (s.npc && s.signal) out.push({ signal: s.signal, npc: s.npc, source, owner, objective: o.id });
    }
  };
  for (const q of quests.values()) allObjectives(q.def).forEach((o) => fromObjective(o, 'quest', q.id));
  for (const e of content.collection) fromObjective(e.objective, 'collection', e.id);
  for (const s of content.items.shops ?? []) {
    if (s.haggle && s.npc) out.push({ signal: s.haggle.signal, npc: s.npc, source: 'haggle', owner: s.id });
  }
  for (const e of ENGINE_SIGNALS) out.push({ ...e, source: 'engine', owner: 'server/internal/driving' });
  return out;
}

function buildFlags(quests: Map<string, QuestNode>, dialogues: DialogueIndex): Map<string, FlagInfo> {
  const flags = new Map<string, FlagInfo>();
  const get = (id: string): FlagInfo => {
    let f = flags.get(id);
    if (!f) flags.set(id, (f = { id, sets: [], reads: [] }));
    return f;
  };
  for (const q of quests.values()) {
    const here = (text = q.id): Place => ({ text, hash: hashQuest(q.id) });
    q.def.rewards?.flags?.forEach((f) => get(f).sets.push({ where: here(), how: 'phần thưởng khi xong quest' }));
    for (const o of q.def.outcomes ?? []) {
      const where = here(`${q.id} (kết cục ${o.id ?? '?'})`);
      get(outcomeFlag(q.id, o.id ?? '')).sets.push({ where, how: 'tự bật khi kết thúc bằng kết cục này' });
      o.flags?.forEach((f) => get(f).sets.push({ where, how: 'cờ của kết cục' }));
    }
    q.def.requires_flags?.forEach((f) => get(f).reads.push({ where: here(), how: 'quest cần cờ này (requires_flags)' }));
    q.def.unless_flags?.forEach((f) => get(f).reads.push({ where: here(), how: 'quest bị chặn bởi cờ này (unless_flags)' }));
  }
  for (const g of dialogues.graphs.values()) {
    for (const [nodeId, node] of Object.entries(g.def.nodes)) {
      for (const key of Object.keys(node.say)) {
        if (key.startsWith('flag:')) {
          get(key.slice(5)).reads.push({ where: { text: `${g.id}/${nodeId}`, hash: hashDialogue(g.id, nodeId) }, how: 'biến thể câu thoại' });
        }
      }
      (node.choices ?? []).forEach((c, i) => {
        const where = { text: `${g.id}/${nodeId} (câu trả lời ${i + 1})`, hash: hashDialogue(g.id, nodeId) };
        c.if_flags?.forEach((f) => get(f).reads.push({ where, how: 'câu trả lời chỉ hiện khi có cờ (if_flags)' }));
        c.unless_flags?.forEach((f) => get(f).reads.push({ where, how: 'câu trả lời ẩn khi có cờ (unless_flags)' }));
      });
    }
  }
  return flags;
}

/** Loads the content into the graph the views draw: quests, their links, dialogues, signals, flags. */
export function buildModel(content: RawContent): Model {
  const origins = content.archetypes.archetypes;
  const quests = new Map<string, QuestNode>();
  for (const def of collectQuests(content)) {
    const kind: QuestKind = def.questline === 'thanh_tich' ? 'achievement' : kindOf(def);
    const offered = offeredTo(def.tags_any, origins);
    const flagsSet = [
      ...(def.rewards?.flags ?? []),
      ...(def.outcomes ?? []).flatMap((o) => [outcomeFlag(def.id, o.id ?? ''), ...(o.flags ?? [])]),
    ];
    quests.set(def.id, { id: def.id, def, kind, line: def.questline ?? '', origins: offered, effective: new Set(offered), flagsSet });
  }
  computeEffective(quests);
  const lines: string[] = [];
  for (const q of quests.values()) if (q.kind === 'story' && !lines.includes(q.line)) lines.push(q.line);
  const rank = (l: string) => (LINE_ORDER.includes(l) ? LINE_ORDER.indexOf(l) : LINE_ORDER.length + lines.indexOf(l));
  const storyLines = lines.sort((a, b) => rank(a) - rank(b));
  const dialogues = buildDialogueIndex(content);
  return {
    content,
    origins,
    quests,
    storyLines,
    edges: buildEdges(content, quests),
    dialogues,
    listeners: buildListeners(content, quests),
    capSources: capSources(content, quests),
    flags: buildFlags(quests, dialogues),
  };
}

/** Emissions that satisfy a wait: the same signal, in the NPC's dialogue. */
export function emissionsFor(model: Model, npc: string, signal: string) {
  const dialogue = model.dialogues.npcDialogue.get(npc);
  return model.dialogues.emissions.filter((e) => e.signal === signal && e.dialogue === dialogue);
}

/** Who listens for a signal that a dialogue sends. */
export function listenersOf(model: Model, dialogue: string, signal: string): SignalListener[] {
  return model.listeners.filter((l) => l.signal === signal && model.dialogues.npcDialogue.get(l.npc) === dialogue);
}
