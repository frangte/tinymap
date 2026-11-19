// What an odd job (an errand objective, docs/design/13-jobs.md) says about itself in the tracker and on its card: where
// to take it, what to do and how much time is left, or that the time is up. The server decides all of it; this only
// words it. Text only via textContent.
import { countOf, type ActiveQuest, type QuestLog } from './questLog';
import { countText, type Objective, type Spot } from './quests';
import { NPC_DEFS } from './npcs';
import { tx } from './i18n';
import { L } from './i18n/jobs';

export interface GoalLine {
  text: string;
  /** "1 / 3 · còn 1:42", "" for nothing to add */
  count: string;
  /** the line has a clock, so it needs redrawing every second */
  timed: boolean;
}

const SECONDS_PER_MINUTE = 60;

function clock(seconds: number): string {
  const s = String(seconds % SECONDS_PER_MINUTE).padStart(2, '0');
  return `${Math.floor(seconds / SECONDS_PER_MINUTE)}:${s}`;
}

function giverName(id: string): string {
  return NPC_DEFS.find((n) => n.id === id)?.name ?? id;
}

/** The words for one objective of a quest, as it stands now. */
export function goalLine(log: QuestLog, q: ActiveQuest, o: Objective): GoalLine {
  const e = o.errand;
  if (!e) return { text: tx(o.text), count: countText(o, countOf(q, o)), timed: false };
  const run = log.runOf(q, o);
  if (run.kind === 'idle') return { text: L.take(giverName(e.giver)), count: '', timed: false };
  if (run.kind === 'late') return { text: L.late(giverName(e.giver)), count: '', timed: false };
  const parts = [countText(o, countOf(q, o))];
  if (run.left !== null) parts.push(L.left(clock(run.left)));
  return { text: tx(o.text), count: parts.filter(Boolean).join(' · '), timed: run.left !== null };
}

/** Where the tracker points for one objective: the giver before the job is taken, then the next stop. */
export function goalSpot(log: QuestLog, q: ActiveQuest, o: Objective): Spot | null {
  const e = o.errand;
  if (!e) return o.at;
  if (log.runOf(q, o).kind !== 'run') return giverSpot(e.giver) ?? o.at;
  // stops in any order (the flyers): the street they are on
  if (!e.fixed) return o.at;
  return e.stops[Math.min(countOf(q, o), e.stops.length - 1)] ?? o.at;
}

function giverSpot(id: string): Spot | null {
  const npc = NPC_DEFS.find((n) => n.id === id);
  return npc ? [npc.at] : null;
}
