import { DIALOGUE_END, type Choice, type DialogueDef, type RawContent } from './types';

/** An arrow of a dialogue graph: an answer (or its goto_if) leading to another node. */
export interface DialogueEdge {
  from: string;
  /** index of the answer in the node's choices */
  choice: number;
  to: string;
  /** goto_if: taken instead of the answer's goto once a tally has been reached */
  conditional: boolean;
}

export interface DialogueGraph {
  id: string;
  def: DialogueDef;
  /** NPC ids (npcs.json) and contact ids (contacts.json) whose dialogue this is */
  speakers: string[];
  edges: DialogueEdge[];
  /** nodes reachable from the start or the call node */
  reachable: Set<string>;
}

/** A signal an answer or a node sends. */
export interface Emission {
  dialogue: string;
  node: string;
  /** index of the answer, or -1 for a node's own emit */
  choice: number;
  signal: string;
}

export interface DialogueIndex {
  graphs: Map<string, DialogueGraph>;
  /** NPC or contact id → dialogue id */
  npcDialogue: Map<string, string>;
  emissions: Emission[];
}

/** Where a conversation goes after an answer: its goto, and the goto_if branch when there is one. */
export function choiceTargets(choice: Choice): { to: string; conditional: boolean }[] {
  const targets = [{ to: choice.goto, conditional: false }];
  if (choice.goto_if) targets.push({ to: choice.goto_if.goto, conditional: true });
  return targets;
}

function buildGraph(def: DialogueDef): DialogueGraph {
  const edges: DialogueEdge[] = [];
  for (const [from, node] of Object.entries(def.nodes)) {
    (node.choices ?? []).forEach((choice, index) => {
      for (const t of choiceTargets(choice)) {
        if (t.to !== DIALOGUE_END) edges.push({ from, choice: index, to: t.to, conditional: t.conditional });
      }
    });
  }
  const reachable = new Set<string>();
  const queue = [def.start, ...(def.call ? [def.call] : [])].filter((id) => id in def.nodes);
  queue.forEach((id) => reachable.add(id));
  while (queue.length > 0) {
    const id = queue.shift()!;
    for (const e of edges) {
      if (e.from === id && e.to in def.nodes && !reachable.has(e.to)) {
        reachable.add(e.to);
        queue.push(e.to);
      }
    }
  }
  return { id: def.id, def, speakers: [], edges, reachable };
}

/** Indexes every dialogue: its graph, who speaks it, and the signals it sends. */
export function buildDialogueIndex(content: RawContent): DialogueIndex {
  const graphs = new Map<string, DialogueGraph>();
  for (const def of content.dialogues) graphs.set(def.id, buildGraph(def));
  const npcDialogue = new Map<string, string>();
  const speak = (who: string, dialogue: string | undefined) => {
    if (!dialogue) return;
    npcDialogue.set(who, dialogue);
    graphs.get(dialogue)?.speakers.push(who);
  };
  content.npcs.forEach((n) => speak(n.id, n.dialogue));
  content.contacts.forEach((c) => c.dialogue && speak(c.id, c.dialogue));
  const emissions: Emission[] = [];
  for (const g of graphs.values()) {
    for (const [node, def] of Object.entries(g.def.nodes)) {
      if (def.emit) emissions.push({ dialogue: g.id, node, choice: -1, signal: def.emit });
      (def.choices ?? []).forEach((c, choice) => {
        if (c.emit) emissions.push({ dialogue: g.id, node, choice, signal: c.emit });
      });
    }
  }
  return { graphs, npcDialogue, emissions };
}
