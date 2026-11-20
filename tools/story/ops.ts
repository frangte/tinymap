import { copyLayout } from './format';
import { DIALOGUE_END, type Choice, type DialogueDef, type DialogueNodeDef, type Objective, type Outcome, type QuestDef, type RawContent, type Text } from './types';
import { DEFAULT_VARIANT, KEY_ORDER } from './schema';

/**
 * The changes the editor makes to the content, as plain functions on the content objects. Each keeps the key order
 * the files use (`KEY_ORDER`), and leaves out a field that is empty rather than writing `[]` or `""`.
 */

type Obj = Record<string, unknown>;

const empty = (v: unknown): boolean => v === undefined || v === null || v === '' || v === false || (Array.isArray(v) && v.length === 0);

/**
 * Sets a field, or removes it when the value is empty. A new field goes after the last field before it in `order`
 * that the object has, so entries stay in the order the existing ones use.
 */
export function setField(obj: object, key: string, value: unknown, order: readonly string[]): void {
  const o = obj as Obj;
  if (empty(value)) {
    delete o[key];
    return;
  }
  if (key in o) {
    o[key] = value;
    return;
  }
  const later = order.slice(order.indexOf(key) + 1);
  const moved = Object.keys(o).filter((k) => later.includes(k));
  const held = moved.map((k) => [k, o[k]] as const);
  moved.forEach((k) => delete o[k]);
  o[key] = value;
  for (const [k, v] of held) o[k] = v;
}

const text = (vi = '', en = ''): Text => ({ vi, en });

// ---- dialogue ------------------------------------------------------------------------------------------------

/** Another node or choice of the dialogue to take the layout of a new one from. */
function sampleChoice(def: DialogueDef): Choice | undefined {
  for (const n of Object.values(def.nodes)) if (n.choices?.length) return n.choices[0];
  return undefined;
}

export function setSay(node: DialogueNodeDef, variant: string, lang: 'vi' | 'en', value: string): void {
  const line = (node.say[variant] ??= text());
  line[lang] = value;
}

export function addVariant(node: DialogueNodeDef, key: string): void {
  if (key in node.say) return;
  node.say[key] = text();
  copyLayout(node.say[key], node.say[DEFAULT_VARIANT]);
}

export function removeVariant(node: DialogueNodeDef, key: string): void {
  if (key !== DEFAULT_VARIANT) delete node.say[key];
}

/** A new node, the first line empty and no choices (it ends the conversation). */
export function addNode(def: DialogueDef, id: string): void {
  const node: DialogueNodeDef = { say: { [DEFAULT_VARIANT]: text() } };
  const template = Object.values(def.nodes)[0];
  def.nodes[id] = node;
  copyLayout(node, template);
}

export function deleteNode(def: DialogueDef, id: string): void {
  delete def.nodes[id];
}

/** The choices (and their goto_if) that lead to a node, for the warning before it is deleted. */
export function nodeReferrers(def: DialogueDef, id: string): { node: string; choice: number }[] {
  const out: { node: string; choice: number }[] = [];
  for (const [nid, n] of Object.entries(def.nodes)) {
    (n.choices ?? []).forEach((c, i) => {
      if (c.goto === id || c.goto_if?.goto === id) out.push({ node: nid, choice: i });
    });
  }
  return out;
}

export function setStart(def: DialogueDef, id: string): void {
  def.start = id;
}

export function setCall(def: DialogueDef, id: string): void {
  setField(def, 'call', id, KEY_ORDER.dialogue);
}

export function setNodeField(node: DialogueNodeDef, key: 'emote' | 'shop' | 'fee' | 'emit' | 'reset_marks', value: string | boolean): void {
  setField(node, key, value, KEY_ORDER.node);
}

export function addChoice(def: DialogueDef, node: DialogueNodeDef): void {
  const choice: Choice = { text: text(), goto: DIALOGUE_END };
  copyLayout(choice, node.choices?.[node.choices.length - 1] ?? sampleChoice(def));
  if (node.choices) node.choices.push(choice);
  else {
    const list = [choice];
    copyLayout(list, Object.values(def.nodes).find((n) => n.choices?.length)?.choices);
    setField(node, 'choices', list, KEY_ORDER.node);
  }
}

export function deleteChoice(node: DialogueNodeDef, index: number): void {
  node.choices?.splice(index, 1);
  if (node.choices && node.choices.length === 0) delete node.choices;
}

export function moveChoice(node: DialogueNodeDef, index: number, delta: -1 | 1): void {
  const list = node.choices;
  const to = index + delta;
  if (!list || to < 0 || to >= list.length) return;
  [list[index], list[to]] = [list[to], list[index]];
}

export function setChoiceText(choice: Choice, lang: 'vi' | 'en', value: string): void {
  choice.text[lang] = value;
}

export function setChoiceField(choice: Choice, key: 'goto' | 'emit' | 'requires' | 'unless' | 'requires_quest' | 'if_flags' | 'unless_flags' | 'mark', value: string | string[]): void {
  setField(choice, key, value, KEY_ORDER.choice);
}

export function setGotoIf(choice: Choice, value: Choice['goto_if'] | undefined): void {
  setField(choice, 'goto_if', value, KEY_ORDER.choice);
}

// ---- quests ----------------------------------------------------------------------------------------------------

export function setQuestField(q: QuestDef | Outcome | Objective, key: string, value: unknown, order: readonly string[] = KEY_ORDER.quest): void {
  setField(q, key, value, order);
}

export function setQuestText(q: QuestDef, key: 'title' | 'about', lang: 'vi' | 'en', value: string): void {
  const t = ((q as unknown as Obj)[key] ??= text()) as Text;
  t[lang] = value;
}

/** Every objective id of a quest, its outcomes included (they share one id space). */
export function objectiveIds(q: QuestDef): string[] {
  return [...q.objectives, ...(q.outcomes ?? [])].map((o) => o.id ?? '');
}

export function freshId(taken: string[], base: string): string {
  if (!taken.includes(base)) return base;
  for (let n = 2; ; n++) if (!taken.includes(`${base}_${n}`)) return `${base}_${n}`;
}

export function newObjective(q: QuestDef, withTitle: boolean): Objective | Outcome {
  const id = freshId(objectiveIds(q), withTitle ? 'ket_cuc' : 'buoc');
  const base: Objective = { id, type: 'talk_npc', params: {}, text: text() };
  const out = withTitle ? ({ ...base, title: text() } as Outcome) : base;
  const like = withTitle ? q.outcomes?.[0] : q.objectives[0];
  copyLayout(out, like);
  return out;
}

export function addObjective(q: QuestDef): void {
  q.objectives.push(newObjective(q, false));
}

/** Adds an outcome; a quest with outcomes keeps at least two, so the first call adds two. */
export function addOutcome(q: QuestDef): void {
  if (!q.outcomes) setField(q, 'outcomes', [newObjective(q, true) as Outcome], KEY_ORDER.quest);
  else q.outcomes.push(newObjective(q, true) as Outcome);
}

export function removeAt<T>(list: T[], index: number): void {
  list.splice(index, 1);
}

export function dropOutcomes(q: QuestDef): void {
  delete q.outcomes;
}

export function setObjectiveType(o: Objective, type: string): void {
  o.type = type;
  o.params = {};
}

export function setParam(o: Objective, key: string, value: unknown): void {
  const params = (o.params ??= {});
  if (empty(value)) delete params[key];
  else params[key] = value;
}

export function newQuest(content: RawContent, line: string, id: string): QuestDef {
  const q: QuestDef = {
    id,
    version: 1,
    questline: line,
    title: text(),
    about: text(),
    flow: 'sequence',
    objectives: [{ id: 'buoc', type: 'talk_npc', params: {}, text: text() }],
    rewards: {},
  };
  const list = content.quests.quests;
  const sample = list.filter((x) => x.questline === line).pop() ?? list[list.length - 1];
  copyLayout(q, sample);
  const last = list.map((x, i) => (x.questline === line ? i : -1)).reduce((a, b) => Math.max(a, b), -1);
  list.splice(last >= 0 ? last + 1 : list.length, 0, q);
  return q;
}

export function deleteQuest(content: RawContent, id: string): void {
  content.quests.quests = content.quests.quests.filter((q) => q.id !== id);
}

/** Everything in the content that mentions a quest, for the warning before it is deleted. */
export function questReferrers(content: RawContent, id: string): { where: string; hash: string }[] {
  const out: { where: string; hash: string }[] = [];
  const flagOf = (f: string) => f.startsWith(`${id}:`);
  for (const q of content.quests.quests) {
    const here = { hash: `#/quest/${q.id}` };
    if (q.requires?.includes(id)) out.push({ ...here, where: `${q.id}: requires` });
    if ([...(q.requires_flags ?? []), ...(q.unless_flags ?? [])].some(flagOf)) out.push({ ...here, where: `${q.id}: đọc cờ của quest này` });
    for (const o of [...q.objectives, ...(q.outcomes ?? [])]) {
      if (o.type === 'quest_done' && o.params?.quest === id) out.push({ ...here, where: `${q.id}: mục tiêu ${o.id ?? ''} (quest_done)` });
    }
  }
  for (const d of content.dialogues) {
    for (const [nid, n] of Object.entries(d.nodes)) {
      const hash = `#/dialogue/${d.id}/${nid}`;
      for (const key of Object.keys(n.say)) if (key.startsWith('flag:') && flagOf(key.slice(5))) out.push({ hash, where: `${d.id}/${nid}: biến thể ${key}` });
      (n.choices ?? []).forEach((c, i) => {
        if (c.requires_quest === id) out.push({ hash, where: `${d.id}/${nid}: câu trả lời ${i + 1} (requires_quest)` });
        if ([...(c.if_flags ?? []), ...(c.unless_flags ?? [])].some(flagOf)) out.push({ hash, where: `${d.id}/${nid}: câu trả lời ${i + 1} đọc cờ` });
      });
    }
  }
  return out;
}

/** Adds `req` to a quest's requires (the drag from one card to another). */
export function addRequires(q: QuestDef, req: string): void {
  if (q.requires?.includes(req) || q.id === req) return;
  setField(q, 'requires', [...(q.requires ?? []), req], KEY_ORDER.quest);
}
