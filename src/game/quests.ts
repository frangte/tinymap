// Quests, achievements, titles and stamps as the game shows them: server/content/quests.json, the file the Go
// server runs them from (its tests keep it valid). The server says how far a character got (the `progress`
// message); the text, and where an objective is on the map, come from here.
import data from '../../server/content/quests.json';
import type { Text } from './archetypes';
import { PLACES } from './places';
import { tx } from './i18n';
import { L } from './i18n/quests';
import { NPC_DEFS } from './npcs';

export type Flow = 'sequence' | 'all' | 'any';

/** where something is, Blender XY: one point, or a polyline for a street */
export type Spot = ReadonlyArray<readonly [number, number]>;

/** An errand objective (an odd job, docs/design/13-jobs.md): taken from `giver`, then stop after stop. */
export interface Errand {
  /** the NPC id the job is taken from */
  giver: string;
  /** where each stop is, null for an unknown place */
  stops: readonly (Spot | null)[];
  /** the time limit (s), 0 for none */
  seconds: number;
  /** the stops come one after the other (the tracker points to the next); otherwise in any order */
  fixed: boolean;
}

export interface Objective {
  id: string;
  type: string;
  /** how many times (seconds for stay_place, metres for ride_distance) */
  count: number;
  text: Text;
  /** where it is, Blender XY: one point, or a polyline for a street; null for nowhere in particular */
  at: ReadonlyArray<readonly [number, number]> | null;
  /** set for an errand objective */
  errand: Errand | null;
}

export interface QuestDef {
  id: string;
  questline: string;
  title: Text;
  about: Text;
  flow: Flow;
  objectives: readonly Objective[];
  /** starts over every day (midnight in Saigon); the server only sends today's */
  daily: boolean;
  /** an odd job: a daily done once a day, apart from the streak of dailies */
  job: boolean;
  /** xu paid on completion (0 for none) */
  xu: number;
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
function spotOf(at: string | undefined): Spot | null {
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
  params?: unknown;
}

interface RawStop {
  place?: string;
  npc?: string;
}

interface RawErrand {
  npc: string;
  stops: RawStop[];
  seconds?: number;
  order?: string;
}

function isRawErrand(p: unknown): p is RawErrand {
  return typeof p === 'object' && p !== null && 'npc' in p && 'stops' in p && Array.isArray(p.stops);
}

function errandOf(o: RawObjective): Errand | null {
  if (o.type !== 'errand' || !isRawErrand(o.params)) return null;
  const p = o.params;
  return { giver: p.npc, stops: p.stops.map((s) => spotOf(s.place ?? s.npc)), seconds: p.seconds ?? 0, fixed: p.order !== 'free' };
}

function objective(o: RawObjective): Objective {
  return { id: o.id ?? '', type: o.type, count: Math.max(1, o.count ?? 1), text: o.text ?? { vi: '' }, at: spotOf(o.at), errand: errandOf(o) };
}

export const QUESTS: readonly QuestDef[] = data.quests.map((q): QuestDef => {
  const flow = FLOWS.find((f) => f === q.flow);
  if (!flow) throw new Error(`server/content/quests.json: bad flow in "${q.id}"`);
  const daily = 'repeat' in q && q.repeat === 'daily';
  const job = 'job' in q && q.job === true;
  const xu = 'xu' in q.rewards && typeof q.rewards.xu === 'number' ? q.rewards.xu : 0;
  return { id: q.id, questline: q.questline, title: q.title, about: q.about, flow, objectives: q.objectives.map(objective), daily, job, xu };
});

export const ACHIEVEMENTS: readonly AchievementDef[] = data.achievements.map((a): AchievementDef => ({
  id: a.id,
  title: a.title,
  about: a.about,
  hidden: 'hidden' in a && a.hidden === true,
  count: Math.max(1, ('count' in a.objective ? a.objective.count : undefined) ?? 1),
  type: a.objective.type,
}));

export interface StampDef {
  id: string;
  name: Text;
}

const stampSet = (s: (typeof data.stamps)[number]): string => ('set' in s && typeof s.set === 'string' ? s.set : '');

/** every stamp, any set (news can name any) */
export const STAMPS: readonly StampDef[] = data.stamps;
/** the stamp book (Sổ tem): stamps outside any set; empty since the Traveler's questline left the game (2026-11-19) */
export const BOOK_STAMPS: readonly StampDef[] = data.stamps.filter((s) => stampSet(s) === '');
/** stamps for days of dailies in a row */
export const STREAK_STAMPS: readonly StampDef[] = data.stamps.filter((s) => stampSet(s) === 'streak');

const titles = new Map(data.titles.map((t) => [t.id, t.name]));

/** A title's name, null for none or an unknown id. */
export function titleName(id: string): string | null {
  const t = id ? titles.get(id) : undefined;
  return t ? tx(t) : null;
}

export function questById(id: string): QuestDef | undefined {
  return QUESTS.find((q) => q.id === id);
}

/** "12 / 20 giây" ("12 / 20 s"), "2 / 3", "" for something done once. */
export function countText(o: Objective | AchievementDef, n: number): string {
  if (o.count <= 1) return '';
  if (o.type === 'stay_place') return L.seconds(Math.min(n, o.count), o.count);
  if (o.type === 'ride_distance') return `${(Math.min(n, o.count) / 1000).toFixed(1)} / ${(o.count / 1000).toFixed(1)} km`;
  return `${Math.min(n, o.count)} / ${o.count}`;
}
