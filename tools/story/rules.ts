import { allObjectives, dialogueTitle, hashDialogue, hashQuest, questTitle, type Model } from './model';
import type { Problem } from './problems';
import { DEFAULT_VARIANT, ID_PATTERN, LIMITS, QUEST_ID_PATTERN, SIGNAL_PATTERN } from './schema';
import { DIALOGUE_END, type QuestDef, type Text } from './types';

/**
 * The server's own content rules (server/content/quests.go, flags.go, cast.go), as far as they can be told from the
 * page, so an edit shows its mistake at once. The server stays the judge: the editor also runs its real checks.
 */

type Out = Problem[];

const KIND = {
  text: 'Câu thiếu tiếng Anh hay tiếng Việt, hoặc quá dài',
  quest: 'Quest sai luật của server',
  dialogue: 'Lời thoại sai luật của server',
};

const length = (s: string): number => [...s].length;

function textIssue(t: Text | undefined, max: number): string {
  if (!t?.vi) return 'thiếu tiếng Việt';
  if (!t.en) return 'thiếu tiếng Anh';
  if (length(t.vi) > max || length(t.en) > max) return `dài hơn ${max} ký tự`;
  for (const s of [t.vi, t.en]) {
    if (s !== s.trim() || s.includes('  ')) return 'có khoảng trắng thừa (đầu, cuối hay đôi)';
    if (/[\u0000-\u001f\u007f]/.test(s)) return 'có ký tự điều khiển hay xuống dòng';
  }
  return '';
}

function questRules(model: Model, q: QuestDef, out: Out): void {
  const where = { text: `${questTitle(q)} (${q.id})`, hash: hashQuest(q.id) };
  const err = (text: string, kind = KIND.quest) => out.push({ severity: 'error', kind, text, where });
  if (!QUEST_ID_PATTERN.test(q.id)) err('id phải là chữ thường, số, _ và dấu chấm');
  for (const [name, t, max] of [['tên', q.title, LIMITS.questTitle], ['mô tả', q.about, LIMITS.about]] as const) {
    const issue = textIssue(t, max);
    if (issue) err(`${name}: ${issue}`, KIND.text);
  }
  const daily = q.repeat === 'daily';
  const outcomes = q.outcomes ?? [];
  if (outcomes.length > 0 && (outcomes.length < LIMITS.outcomesMin || outcomes.length > LIMITS.outcomesMax)) err(`kết cục: cần ${LIMITS.outcomesMin} đến ${LIMITS.outcomesMax} (một kết cục thì chỉ là một mục tiêu), đang có ${outcomes.length}`);
  if (daily && (outcomes.length > 0 || q.rewards?.flags?.length || q.requires_flags?.length || q.unless_flags?.length)) err('quest hằng ngày không có kết cục hay cờ (cờ không bao giờ xóa, quest hằng ngày thì làm lại mỗi ngày)');
  if (q.objectives.length === 0 && outcomes.length === 0) err('cần ít nhất một mục tiêu (hay kết cục)');
  if (q.objectives.length > LIMITS.objectives) err(`nhiều nhất ${LIMITS.objectives} mục tiêu`);
  const seen = new Set<string>();
  for (const o of allObjectives(q)) {
    if (!o.id || !ID_PATTERN.test(o.id)) err(`mục tiêu "${o.id ?? ''}": cần id chữ thường, số, _`);
    else if (seen.has(o.id)) err(`id "${o.id}" trùng (mục tiêu và kết cục dùng chung id)`);
    seen.add(o.id ?? '');
    const issue = textIssue(o.text, LIMITS.objectiveText);
    if (issue && (q.objectives.includes(o) || o.text?.vi)) err(`mục tiêu "${o.id}": chữ ${issue}`, KIND.text);
  }
  for (const o of outcomes) {
    const issue = textIssue(o.title, LIMITS.title * 2);
    if (issue) err(`kết cục "${o.id}": tên ${issue}`, KIND.text);
    if ((o.flags?.length ?? 0) > LIMITS.flags) err(`kết cục "${o.id}": nhiều nhất ${LIMITS.flags} cờ`);
    if (o.rewards?.flags?.length) err(`kết cục "${o.id}": rewards.flags dành cho quest, kết cục có cờ riêng`);
    if ((q.rewards?.xu ?? 0) + (o.rewards?.xu ?? 0) > LIMITS.xuQuest) err(`kết cục "${o.id}": xu của nó cộng xu của quest phải nhiều nhất ${LIMITS.xuQuest}`);
  }
  for (const [name, list] of [['rewards.flags', q.rewards?.flags], ['requires_flags', q.requires_flags], ['unless_flags', q.unless_flags]] as const) {
    if ((list?.length ?? 0) > LIMITS.flags) err(`${name}: nhiều nhất ${LIMITS.flags} cờ`);
  }
  for (const f of q.requires_flags ?? []) {
    if (q.unless_flags?.includes(f)) err(`cờ "${f}" vừa trong requires_flags vừa trong unless_flags: quest không bao giờ được mời`);
    const setters = [...model.quests.values()].filter((p) => p.flagsSet.includes(f)).map((p) => p.id);
    if (setters.length === 1 && setters[0] === q.id) err(`cờ "${f}" chỉ do chính quest này bật: nó không bao giờ được mời`);
  }
  const xuMax = q.job ? LIMITS.xuJob : daily ? LIMITS.xuDaily : LIMITS.xuQuest;
  if ((q.rewards?.xu ?? 0) < 0 || (q.rewards?.xu ?? 0) > xuMax) err(`thưởng xu phải từ 0 đến ${xuMax}`);
  if (daily && (q.rewards?.titles?.length || q.rewards?.capability)) err('quest hằng ngày thưởng xu và tem, không thưởng danh hiệu hay quyền');
  const titles = new Set(((model.content.quests as unknown as { titles?: { id: string }[] }).titles ?? []).map((t) => t.id));
  for (const t of q.rewards?.titles ?? []) if (!titles.has(t)) err(`danh hiệu "${t}" không có trong titles`);
  const contacts = new Set(model.content.contacts.map((c) => c.id));
  const messages = [...(q.texts?.on_start ?? []), ...(q.texts?.on_done ?? []), ...outcomes.flatMap((o) => o.texts?.on_done ?? [])];
  for (const m of messages) {
    if (!contacts.has(m.from)) err(`tin nhắn từ "${m.from}": không có trong danh bạ (contacts.json)`);
    const issue = textIssue(m.text, LIMITS.message);
    if (issue) err(`tin nhắn từ "${m.from}": ${issue}`, KIND.text);
  }
}

function dialogueRules(model: Model, out: Out): void {
  const caps = new Set((model.content.archetypes.capabilities ?? []).map((c) => c.id));
  const archetypes = new Set(model.origins.map((o) => o.id));
  const emotes = new Set(['wave', 'bow', 'photo', 'exercise']);
  const shops = new Set((model.content.items.shops ?? []).map((s) => s.id));
  const fees = new Set((model.content.items.fees ?? []).map((s) => s.id));
  for (const g of model.dialogues.graphs.values()) {
    const name = dialogueTitle(model, g.id);
    const at = (node?: string) => ({ text: node ? `${name}/${node}` : name, hash: hashDialogue(g.id, node) });
    const err = (node: string | undefined, text: string, kind = KIND.dialogue) => out.push({ severity: 'error', kind, text: `${name}${node ? `/${node}` : ''}: ${text}`, where: at(node) });
    if (!(g.def.start in g.def.nodes)) err(undefined, `ô bắt đầu "${g.def.start}" không có`);
    if (g.def.call && !(g.def.call in g.def.nodes)) err(undefined, `ô gọi điện "${g.def.call}" không có`);
    for (const [id, node] of Object.entries(g.def.nodes)) {
      if (!ID_PATTERN.test(id) || id === DIALOGUE_END) err(id, `id ô phải là chữ thường, số, _ (và không là "${DIALOGUE_END}")`);
      if (!(DEFAULT_VARIANT in node.say)) err(id, 'thiếu câu mặc định (say.default)');
      for (const [key, t] of Object.entries(node.say)) {
        const issue = textIssue(t, LIMITS.say);
        if (issue) err(id, `câu "${key}": ${issue}`, KIND.text);
        if (!key.startsWith('flag:') && key !== DEFAULT_VARIANT && !archetypes.has(key) && !caps.has(key)) err(id, `câu "${key}": không phải xuất phát hay quyền (câu theo cờ có khóa flag:<cờ>)`);
        if (key.startsWith('flag:') && !model.flags.get(key.slice(5))?.sets.length) err(id, `câu "${key}": không quest nào bật cờ này`);
      }
      if (node.emote && !emotes.has(node.emote)) err(id, `điệu bộ "${node.emote}" không có`);
      if (node.shop && !shops.has(node.shop)) err(id, `shop "${node.shop}" không có`);
      if (node.fee && !fees.has(node.fee)) err(id, `phí "${node.fee}" không có`);
      if (node.emit && !SIGNAL_PATTERN.test(node.emit)) err(id, `tín hiệu "${node.emit}" phải là chữ thường, số, _`);
      const choices = node.choices ?? [];
      if (choices.length > LIMITS.choices) err(id, `nhiều nhất ${LIMITS.choices} câu trả lời`);
      choices.forEach((c, i) => {
        const n = `câu trả lời ${i + 1}`;
        const issue = textIssue(c.text, LIMITS.choice);
        if (issue) err(id, `${n}: ${issue}`, KIND.text);
        if (c.emit && !SIGNAL_PATTERN.test(c.emit)) err(id, `${n}: tín hiệu "${c.emit}" phải là chữ thường, số, _`);
        for (const k of [...(c.requires ?? []), ...(c.unless ?? [])]) if (!caps.has(k)) err(id, `${n}: quyền "${k}" không có`);
        if (c.requires_quest && !model.quests.has(c.requires_quest)) err(id, `${n}: quest "${c.requires_quest}" không có`);
        for (const f of c.if_flags ?? []) if (c.unless_flags?.includes(f)) err(id, `${n}: cờ "${f}" vừa trong if_flags vừa trong unless_flags`);
        for (const [name2, list] of [['if_flags', c.if_flags], ['unless_flags', c.unless_flags]] as const) {
          if ((list?.length ?? 0) > LIMITS.flags) err(id, `${n}: ${name2} nhiều nhất ${LIMITS.flags} cờ`);
          for (const f of list ?? []) if (!model.flags.get(f)?.sets.length) err(id, `${n}: cờ "${f}" không quest nào bật`);
        }
        if (c.goto_if) {
          if (!c.mark || c.goto_if.mark !== c.mark || c.goto_if.at_least < 1) err(id, `${n}: goto_if cần cùng mark với câu trả lời và at_least từ 1`);
          if (!(c.goto_if.goto in g.def.nodes)) err(id, `${n}: goto_if tới "${c.goto_if.goto}", ô này không có`);
        }
      });
    }
  }
}

/** Rules of quests.go, flags.go and cast.go that the page can check by itself. */
export function contentRules(model: Model, out: Out): void {
  for (const q of model.content.quests.quests) questRules(model, q, out);
  dialogueRules(model, out);
}
