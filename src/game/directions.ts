// Asking the way, the co-op quest (docs/design/README.md, D18; server/internal/directions). A Traveler asks the
// Citizens online the way to a landmark (from the Cẩm nang); a Citizen gets a call (a banner and a notice), walks
// up to the Traveler (the tracker points the way) and answers with the action button. Nobody coming opens the
// fallback: Chú Sáu answers instead. Nothing here decides anything: the server checks who may ask and answer,
// and how close they stand.
import * as THREE from 'three';
import data from '../../server/content/directions.json';
import type { WayAsk, WayCall, WayClientMsg, WayError, WayServerMsg } from '../net/wayProtocol';
import { placeById, placeTitle } from './places';
import { NPC_DEFS } from './npcs';
import type { Guide } from './rides';
import { onLangChange } from './i18n';
import { L } from './i18n/directions';

/** the button offers to answer a little inside the server's reach, which has the last word (m) */
const ANSWER_NEAR = data.reach - 0.5;
/** after an answer (or the fallback), the tracker shows the way for this long, or until we get there (ms) */
const SHOW_WAY_MS = 120_000;
/** who answers when no Citizen does (the quest's fallback, server/content/quests.json) */
const FALLBACK_NPC = 'chu_sau';
/** this close to Chú Sáu the tracker has done its job (m) */
const AT_NPC = 4;

const ERRORS: Record<WayError, () => string> = {
  no_account: () => L.errNoAccount,
  not_allowed: () => L.errNotAllowed,
  busy: () => L.errBusy,
  unknown_place: () => L.errUnknownPlace,
  too_soon: () => L.errTooSoon,
  gone: () => L.errGone,
  too_far: () => L.errTooFar,
  already_helped: () => L.errAlreadyHelped,
};

export interface DirectionsDeps {
  net: { sendWay(m: WayClientMsg): boolean; serverNow(): number };
  /** another player's avatar, when the server keeps them in view */
  other(id: number): { position: THREE.Vector3 } | null;
  self(): THREE.Vector3;
  say(text: string): void;
  notify(title: string, text: string): void;
}

/** A place's name in the player's language, or its id when unknown. */
export function landmarkName(id: string): string {
  const p = placeById(id);
  return p ? placeTitle(p) : id;
}

/** where the tracker points once the way is known */
interface Shown {
  /** what the tracker says, in the language of the moment */
  goal: () => string;
  until: number;
  /** Blender XY of where to go */
  at: readonly [number, number];
  /** within this much of it we are there (m) */
  near: number;
}

export class Directions {
  /** our ask, as a Traveler, while it waits */
  ask: WayAsk | null = null;
  /** asks we were called to, as a Citizen */
  readonly calls = new Map<string, WayCall>();
  /** nobody came to the last ask: the quest takes Chú Sáu's directions */
  fallback = false;
  private shown: Shown | null = null;
  private readonly listeners = new Set<() => void>();
  private readonly to = new THREE.Vector3();

  constructor(private readonly d: DirectionsDeps) {
    // whatever shows our guidance draws it again in the new language
    onLangChange(() => this.changed());
  }

  onChange(fn: () => void): () => void {
    this.listeners.add(fn);
    return () => this.listeners.delete(fn);
  }

  /** A welcome or a dropped connection: asks and calls ended with it. */
  reset(): void {
    this.ask = null;
    this.calls.clear();
    this.changed();
  }

  handle(m: WayServerMsg): void {
    switch (m.t) {
      case 'way':
        this.ours(m);
        break;
      case 'way_call': {
        const { t: _t, ...call } = m;
        if (!this.calls.has(call.id)) this.d.notify(L.callTitle, L.callText(call.from.name, landmarkName(call.dest)));
        this.calls.set(call.id, call);
        break;
      }
      case 'way_gone':
        this.calls.delete(m.id);
        break;
      case 'way_done':
        this.calls.delete(m.id);
        this.d.say(L.helped(landmarkName(m.dest), m.to.name));
        break;
      case 'way_error':
        this.d.say(ERRORS[m.code]());
        break;
    }
    this.changed();
  }

  /** What our own ask became. */
  private ours(m: WayAsk): void {
    const place = placeById(m.dest);
    const name = (): string => landmarkName(m.dest);
    switch (m.state) {
      case 'asking':
        this.ask = m;
        this.shown = null;
        break;
      case 'answered': {
        this.ask = null;
        const helper = (): string => m.helper?.name ?? L.someSaigonese;
        this.d.say(L.answered(helper(), name()));
        if (place) this.shown = { goal: () => L.goShown(name(), helper()), until: Date.now() + SHOW_WAY_MS, at: place.pts[0], near: place.r };
        break;
      }
      case 'unanswered': {
        this.ask = null;
        this.fallback = true;
        this.d.say(m.why === 'nobody' ? L.nobodyPlaying : L.nobodyCame);
        const npc = NPC_DEFS.find((n) => n.id === FALLBACK_NPC);
        if (npc) this.shown = { goal: () => L.askNpc(npc.name, name()), until: Date.now() + SHOW_WAY_MS, at: npc.at, near: AT_NPC };
        break;
      }
      case 'cancelled':
        this.ask = null;
        break;
    }
  }

  /** A few times a second: calls run out, and the way shown ends once we get there. */
  update(): void {
    const now = this.d.net.serverNow();
    let changed = false;
    for (const [id, c] of this.calls) {
      if (c.until > 0 && c.until < now) {
        this.calls.delete(id);
        changed = true;
      }
    }
    const s = this.shown;
    if (s) {
      const me = this.d.self();
      if (Date.now() > s.until || Math.hypot(me.x - s.at[0], -me.z - s.at[1]) <= s.near) {
        this.shown = null;
        changed = true;
      }
    }
    if (changed) this.changed();
  }

  // ---- commands (the answers come back through handle)

  askWay(dest: string): void {
    if (!this.d.net.sendWay({ t: 'way_ask', dest })) this.d.say(L.offline);
  }

  cancel(): void {
    this.d.net.sendWay({ t: 'way_cancel' });
  }

  /** Stop showing a call (the server keeps it: another Citizen may still come). */
  dismiss(id: string): void {
    if (this.calls.delete(id)) this.changed();
  }

  /** The call to show first: the nearest asker we can see, else the oldest. */
  nextCall(): WayCall | null {
    let best: WayCall | null = null;
    let bestD = Infinity;
    for (const c of this.calls.values()) {
      const d = this.distanceTo(c);
      if (!best || d < bestD) {
        best = c;
        bestD = d;
      }
    }
    return best;
  }

  /** How far the asker of a call is (Infinity when out of view). */
  distanceTo(c: WayCall): number {
    const o = this.d.other(c.from.id);
    if (!o) return Infinity;
    const me = this.d.self();
    return Math.hypot(o.position.x - me.x, o.position.z - me.z);
  }

  // ---- the action button: answering an asker standing by

  private callInReach(): WayCall | null {
    const c = this.nextCall();
    return c && this.distanceTo(c) <= ANSWER_NEAR ? c : null;
  }

  label(): string | null {
    const c = this.callInReach();
    return c ? L.answer(c.from.name) : null;
  }

  use(): boolean {
    const c = this.callInReach();
    if (!c) return false;
    this.d.net.sendWay({ t: 'way_answer', ask: c.id });
    return true;
  }

  // ---- guidance

  /** What the tracker shows: the asker to walk to, our ask waiting, or the way we were shown; null for the quest. */
  guide(): Guide | null {
    const c = this.nextCall();
    if (c) {
      return { title: L.tracker, goal: L.callGoal(c.from.name, landmarkName(c.dest)), target: () => this.d.other(c.from.id)?.position ?? null };
    }
    if (this.ask) return { title: L.tracker, goal: L.waiting, target: () => null };
    const s = this.shown;
    if (s) return { title: L.tracker, goal: s.goal(), target: () => this.to.set(s.at[0], 0, -s.at[1]) };
    return null;
  }

  private changed(): void {
    for (const fn of this.listeners) fn();
  }
}
