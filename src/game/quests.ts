// Quests, achievements, titles and stamps as the game shows them: server/content/quests.json, the file the Go
// server runs them from (its tests keep it valid). The server says how far a character got (the `progress`
// message); the text, and where an objective is on the map, come from here.
import data from '../../server/content/quests.json';
import type { Text } from './archetypes';
import { PLACES } from './places';
import { NPC_DEFS } from './npcs';

export type Flow = 'sequence' | 'all' | 'any';

export interface Objective {
  id: string;
  type: string;
  /** how many times (seconds for stay_place, metres for ride_distance) */
  count: number;
  text: Text;
  /** where it is, Blender XY: one point, or a polyline for a street; null for nowhere in particular */
  at: ReadonlyArray<readonly [number, number]> | null;
}

export interface QuestDef {
  id: string;
  questline: string;
  title: Text;
  about: Text;
  flow: Flow;
  objectives: readonly Objective[];
}

export interface AchievementDef {
  id: string;
  title: Text;
  about: Text;
  hidden: boolean;
  count: number;
  /** what the count is of: metres for a ride, otherwise things */
  type: string;
}

const FLOWS: readonly Flow[] = ['sequence', 'all', 'any'];

/** Where an objective's `at` (a place or an NPC id) is, in Blender XY. */
function spotOf(at: string | undefined): ReadonlyArray<readonly [number, number]> | null {
  if (!at) return null;
  const npc = NPC_DEFS.find((n) => n.id === at);
  if (npc) return [npc.at];
  return PLACES.find((p) => p.id === at)?.pts ?? null;
}

interface RawObjective {
  id?: string;
  type: string;
  count?: number;
  text?: Text;
  at?: string;
}

function objective(o: RawObjective): Objective {
  return { id: o.id ?? '', type: o.type, count: Math.max(1, o.count ?? 1), text: o.text ?? { vi: '' }, at: spotOf(o.at) };
}

export const QUESTS: readonly QuestDef[] = data.quests.map((q): QuestDef => {
  const flow = FLOWS.find((f) => f === q.flow);
  if (!flow) throw new Error(`server/content/quests.json: bad flow in "${q.id}"`);
  return { id: q.id, questline: q.questline, title: q.title, about: q.about, flow, objectives: q.objectives.map(objective) };
});

export const ACHIEVEMENTS: readonly AchievementDef[] = data.achievements.map((a): AchievementDef => ({
  id: a.id,
  title: a.title,
  about: a.about,
  hidden: 'hidden' in a && a.hidden === true,
  count: Math.max(1, ('count' in a.objective ? a.objective.count : undefined) ?? 1),
  type: a.objective.type,
}));

export const STAMPS: ReadonlyArray<{ id: string; name: Text }> = data.stamps;

const titles = new Map(data.titles.map((t) => [t.id, t.name]));

/** A title's name, null for none or an unknown id. */
export function titleName(id: string): string | null {
  return id ? (titles.get(id)?.vi ?? null) : null;
}

export function questById(id: string): QuestDef | undefined {
  return QUESTS.find((q) => q.id === id);
}

/** "12 / 20 giây", "2 / 3", "" for something done once. */
export function countText(o: Objective | AchievementDef, n: number): string {
  if (o.count <= 1) return '';
  if (o.type === 'stay_place') return `${Math.min(n, o.count)} / ${o.count} giây`;
  if (o.type === 'ride_distance') return `${(Math.min(n, o.count) / 1000).toFixed(1)} / ${(o.count / 1000).toFixed(1)} km`;
  return `${Math.min(n, o.count)} / ${o.count}`;
}
