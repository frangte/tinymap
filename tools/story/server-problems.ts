import { hashDialogue, hashQuest, type Model } from './model';
import type { Problem } from './problems';
import type { CheckState } from './store';

const KIND = 'Server từ chối nội dung này';

/** Where a server error message points: `quests.json: quest "x": …`, `dialogue/chu_tam.json: node "greet": …`. */
function whereOf(model: Model, message: string): Problem['where'] {
  const quest = /^quests\.json: (?:quest|achievement) "([^"]+)"/.exec(message);
  if (quest && model.quests.has(quest[1])) return { text: quest[1], hash: hashQuest(quest[1]) };
  const dialogue = /^dialogue\/([a-z0-9_]+)\.json: (?:node "([^"]+)")?/.exec(message);
  if (dialogue && model.dialogues.graphs.has(dialogue[1])) {
    const node = dialogue[2] && model.dialogues.graphs.get(dialogue[1])!.def.nodes[dialogue[2]] ? dialogue[2] : undefined;
    return { text: node ? `${dialogue[1]}/${node}` : dialogue[1], hash: hashDialogue(dialogue[1], node) };
  }
  const file = /^([a-z0-9_]+\.json)/.exec(message);
  return { text: file?.[1] ?? 'server', hash: '#/problems' };
}

/** What the server's own checks said about the draft, as problems on the quest or node they name. */
export function serverProblems(model: Model, check: CheckState): Problem[] {
  if (check.phase !== 'done') return [];
  const { reply } = check;
  if (reply.ok === null) return [{ severity: 'info', kind: 'Chưa chạy được kiểm tra của server', text: reply.note ?? 'không rõ vì sao', where: { text: 'server', hash: '#/problems' } }];
  return reply.errors.map((message) => ({ severity: 'error' as const, kind: KIND, text: message, where: whereOf(model, message) }));
}
