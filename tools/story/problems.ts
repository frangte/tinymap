import { DIALOGUE_END } from './types';
import { dialogueTitle, emissionsFor, speakerName, hashDialogue, hashQuest, listenersOf, questTitle, type Model, type Place } from './model';

export type Severity = 'error' | 'warn' | 'info';

export interface Problem {
  severity: Severity;
  /** the kind of problem, for the group heading */
  kind: string;
  text: string;
  where: Place;
}

const KIND = {
  waiting: 'Quest chờ tín hiệu không ai gửi',
  unheard: 'Tín hiệu gửi đi mà không ai nghe',
  unreachable: 'Ô thoại không tới được',
  dangling: 'Nối tới ô không có',
  noOrigin: 'Quest không xuất phát nào được mời',
  neverStarts: 'Quest không bao giờ bắt đầu được',
  noCap: 'Quyền cần mà không nơi nào cấp',
  flagRead: 'Cờ được đọc mà không ai bật',
  flagSet: 'Cờ được bật mà không ai đọc',
  unusedDialogue: 'Lời thoại không NPC nào dùng',
};

function questProblems(model: Model, out: Problem[]): void {
  const originName = (id: string) => model.origins.find((o) => o.id === id)?.name.vi ?? id;
  const capGivers = model.capSources;
  for (const q of model.quests.values()) {
    const where = { text: `${questTitle(q.def)} (${q.id})`, hash: hashQuest(q.id) };
    if (q.origins.size === 0) {
      out.push({ severity: 'warn', kind: KIND.noOrigin, text: `tags_any ${JSON.stringify(q.def.tags_any)} không khớp xuất phát nào`, where });
      continue;
    }
    for (const r of q.def.requires ?? []) {
      const req = model.quests.get(r);
      if (!req) out.push({ severity: 'error', kind: KIND.neverStarts, text: `cần quest "${r}" mà quest đó không có`, where });
      else if (req.def.repeat === 'daily') out.push({ severity: 'error', kind: KIND.neverStarts, text: `cần "${r}", là quest hằng ngày (xong chỉ được một ngày)`, where });
    }
    if (q.effective.size === 0) {
      out.push({
        severity: 'error',
        kind: KIND.neverStarts,
        text: `mời ${[...q.origins].map(originName).join(', ')}, nhưng chuỗi requires không xuất phát nào đi hết được`,
        where,
      });
    }
    for (const cap of q.def.requires_caps ?? []) {
      if (!capGivers.has(cap)) out.push({ severity: 'warn', kind: KIND.noCap, text: `cần quyền "${cap}", không có item, phí, phần thưởng hay xuất phát nào cấp`, where });
    }
  }
}

function signalProblems(model: Model, out: Problem[]): void {
  for (const q of model.quests.values()) {
    for (const l of model.listeners.filter((x) => x.source === 'quest' && x.owner === q.id)) {
      if (emissionsFor(model, l.npc, l.signal).length > 0) continue;
      const dialogue = model.dialogues.npcDialogue.get(l.npc);
      const text = dialogue
        ? `chờ tín hiệu "${l.signal}" từ ${speakerName(model, l.npc)}, nhưng lời thoại ${dialogueTitle(model, dialogue)} không câu nào gửi`
        : `chờ tín hiệu "${l.signal}" từ "${l.npc}", mà NPC này không có lời thoại`;
      out.push({ severity: 'error', kind: KIND.waiting, text, where: { text: `${questTitle(q.def)} (${q.id}${l.objective ? `.${l.objective}` : ''})`, hash: hashQuest(q.id) } });
    }
  }
  const seen = new Set<string>();
  for (const e of model.dialogues.emissions) {
    const key = `${e.dialogue}/${e.signal}`;
    if (seen.has(key) || listenersOf(model, e.dialogue, e.signal).length > 0) continue;
    seen.add(key);
    out.push({ severity: 'info', kind: KIND.unheard, text: `"${e.signal}" (${dialogueTitle(model, e.dialogue)}/${e.node}): không quest, sổ sưu tầm hay trả giá nào nghe`, where: { text: `${dialogueTitle(model, e.dialogue)}/${e.node}`, hash: hashDialogue(e.dialogue, e.node) } });
  }
}

function dialogueProblems(model: Model, out: Problem[]): void {
  for (const g of model.dialogues.graphs.values()) {
    if (g.speakers.length === 0) out.push({ severity: 'info', kind: KIND.unusedDialogue, text: `${dialogueTitle(model, g.id)} (${g.id}): không NPC hay danh bạ nào trỏ tới`, where: { text: g.id, hash: hashDialogue(g.id) } });
    for (const id of Object.keys(g.def.nodes)) {
      if (!g.reachable.has(id)) out.push({ severity: 'warn', kind: KIND.unreachable, text: `${dialogueTitle(model, g.id)}/${id}: không đi tới được từ ô đầu hay ô gọi điện`, where: { text: `${dialogueTitle(model, g.id)}/${id}`, hash: hashDialogue(g.id, id) } });
    }
    for (const e of g.edges) {
      if (!(e.to in g.def.nodes) && e.to !== DIALOGUE_END) {
        out.push({ severity: 'error', kind: KIND.dangling, text: `${dialogueTitle(model, g.id)}/${e.from} (câu trả lời ${e.choice + 1}) nối tới "${e.to}", ô này không có`, where: { text: `${dialogueTitle(model, g.id)}/${e.from}`, hash: hashDialogue(g.id, e.from) } });
      }
    }
  }
}

function flagProblems(model: Model, out: Problem[]): void {
  for (const f of model.flags.values()) {
    if (f.sets.length === 0) {
      out.push({ severity: 'warn', kind: KIND.flagRead, text: `cờ "${f.id}" được đọc nhưng không quest nào bật`, where: f.reads[0].where });
    } else if (f.reads.length === 0 && !f.id.includes(':')) {
      out.push({ severity: 'info', kind: KIND.flagSet, text: `cờ "${f.id}" được bật nhưng chưa ai đọc`, where: f.sets[0].where });
    }
  }
}

/** Everything wrong or suspicious in the content, most serious first. */
export function findProblems(model: Model): Problem[] {
  const out: Problem[] = [];
  questProblems(model, out);
  signalProblems(model, out);
  dialogueProblems(model, out);
  flagProblems(model, out);
  const rank: Record<Severity, number> = { error: 0, warn: 1, info: 2 };
  return out.sort((a, b) => rank[a.severity] - rank[b.severity] || a.kind.localeCompare(b.kind));
}
