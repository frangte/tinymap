// The player's quests as the server last told it (the `progress` message), and which one the tracker follows.
// Nothing here decides progress: the server does, from what it saw the player do.
import type { AchievementState, Progress, QuestState } from '../net/protocol';
import { ACHIEVEMENTS, QUESTS, type AchievementDef, type Objective, type QuestDef } from './quests';

const TRACK_KEY = 'sg.track.v1';

export interface ActiveQuest {
  def: QuestDef;
  state: QuestState;
}

/** Where an errand objective (an odd job) stands: not taken, on its way (seconds left; null without a time limit), or out of time. */
export type RunState = { kind: 'idle' } | { kind: 'run'; left: number | null } | { kind: 'late' };

export class QuestLog {
  private progress: Progress | null = null;
  /** when the progress arrived (performance.now, ms): the clock of a timed job counts down from there */
  private receivedAt = 0;
  private trackedId: string | null = null;
  private readonly listeners: Array<() => void> = [];

  constructor() {
    try {
      this.trackedId = localStorage.getItem(TRACK_KEY);
    } catch {
      // storage blocked: the tracker follows the first quest
    }
  }

  /** Whether the server sent any (a character with an account, online). */
  get known(): boolean {
    return this.progress !== null;
  }

  get titles(): readonly string[] {
    return this.progress?.titles ?? [];
  }

  get title(): string {
    return this.progress?.title ?? '';
  }

  get stamps(): readonly string[] {
    return this.progress?.stamps ?? [];
  }

  /** ids of the Sổ sưu tầm entries found */
  get collection(): readonly string[] {
    return this.progress?.collection ?? [];
  }

  /** days in a row with a daily done */
  get streak(): number {
    return this.progress?.streak ?? 0;
  }

  set(p: Progress): void {
    const before = this.jobsOn();
    this.progress = p;
    this.receivedAt = performance.now();
    // the job just taken is the one to follow
    for (const id of this.jobsOn()) {
      if (before.has(id)) continue;
      this.trackedId = id;
      try {
        localStorage.setItem(TRACK_KEY, id);
      } catch {
        // storage blocked: followed for this visit only
      }
    }
    this.changed();
  }

  /** ids of the odd jobs whose run is on */
  private jobsOn(): Set<string> {
    const on = new Set<string>();
    for (const state of this.progress?.quests ?? []) {
      if (!state.done && state.left && Object.keys(state.left).length > 0) on.add(state.id);
    }
    return on;
  }

  /** How an errand objective stands now (the server says when a run began; the clock runs here). */
  runOf(q: ActiveQuest, o: Objective): RunState {
    const left = q.state.left?.[o.id];
    if (left === undefined) return { kind: 'idle' };
    if (left < 0) return { kind: 'run', left: null };
    const remaining = left - (performance.now() - this.receivedAt) / 1000;
    return remaining > 0 ? { kind: 'run', left: Math.ceil(remaining) } : { kind: 'late' };
  }

  /** Offline, or the connection dropped: nothing known until the next progress. */
  clear(): void {
    if (!this.progress) return;
    this.progress = null;
    this.changed();
  }

  /** Hear every change; call the returned function to stop. */
  onChange(fn: () => void): () => void {
    this.listeners.push(fn);
    return () => {
      const i = this.listeners.indexOf(fn);
      if (i >= 0) this.listeners.splice(i, 1);
    };
  }

  /** Running quests, the tracked one first, then in the order of quests.json. */
  active(): ActiveQuest[] {
    const out: ActiveQuest[] = [];
    for (const state of this.progress?.quests ?? []) {
      const def = QUESTS.find((q) => q.id === state.id);
      if (def && !state.done) out.push({ def, state });
    }
    const t = this.trackedId;
    return out.sort((a, b) => Number(b.def.id === t) - Number(a.def.id === t) || QUESTS.indexOf(a.def) - QUESTS.indexOf(b.def));
  }

  /** Quests done for good, in the order of quests.json (today's dailies are in dailies()). */
  done(): QuestDef[] {
    const ids = new Set((this.progress?.quests ?? []).filter((q) => q.done).map((q) => q.id));
    return QUESTS.filter((q) => ids.has(q.id) && !q.daily);
  }

  /** Today's dailies (not the odd jobs), running or done, in the order of quests.json. */
  dailies(): ActiveQuest[] {
    return this.today((def) => def.daily && !def.job);
  }

  /** Today's odd jobs, running or done, in the order of quests.json. */
  jobs(): ActiveQuest[] {
    return this.today((def) => def.job);
  }

  private today(pick: (def: QuestDef) => boolean): ActiveQuest[] {
    const states = new Map((this.progress?.quests ?? []).map((q) => [q.id, q]));
    const out: ActiveQuest[] = [];
    for (const def of QUESTS) {
      const state = states.get(def.id);
      if (pick(def) && state) out.push({ def, state });
    }
    return out;
  }

  achievements(): Array<{ def: AchievementDef; state: AchievementState | null }> {
    const states = new Map((this.progress?.achievements ?? []).map((a) => [a.id, a]));
    // achievements not open to this character (another archetype's) are left out
    return ACHIEVEMENTS.filter((a) => states.has(a.id)).map((def) => ({ def, state: states.get(def.id) ?? null }));
  }

  /** The quest the tracker follows: the one picked, or the first running. */
  tracked(): ActiveQuest | null {
    return this.active()[0] ?? null;
  }

  track(id: string): void {
    this.trackedId = id;
    try {
      localStorage.setItem(TRACK_KEY, id);
    } catch {
      // storage blocked: picked for this visit only
    }
    this.changed();
  }

  private changed(): void {
    for (const fn of this.listeners) fn();
  }
}

/** How far an objective got. */
export function countOf(q: ActiveQuest, o: Objective): number {
  return Object.prototype.hasOwnProperty.call(q.state.n, o.id) ? q.state.n[o.id] : 0;
}

export function isDone(q: ActiveQuest, o: Objective): boolean {
  return countOf(q, o) >= o.count;
}

/**
 * The objectives to do now: in a sequence the first not done (the others wait their turn); otherwise every one not
 * done yet.
 */
export function current(q: ActiveQuest): Objective[] {
  const left = q.def.objectives.filter((o) => !isDone(q, o));
  return q.def.flow === 'sequence' ? left.slice(0, 1) : left;
}
