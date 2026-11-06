// Asking the way, the co-op quest (docs/design/README.md, D18; server/internal/directions). A Traveler asks the
// Citizens online the way to a landmark (from the Cẩm nang); a Citizen gets a call (a banner and a notice), walks
// up to the Traveler (the tracker points the way) and answers with the action button. Nobody coming opens the
// fallback: Chú Sáu answers instead. Nothing here decides anything: the server checks who may ask and answer,
// and how close they stand.
import * as THREE from 'three';
import data from '../../server/content/directions.json';
import type { WayAsk, WayCall, WayClientMsg, WayError, WayServerMsg } from '../net/wayProtocol';
import { PLACES } from './places';
import { NPC_DEFS } from './npcs';
import type { Guide } from './rides';

/** the button offers to answer a little inside the server's reach, which has the last word (m) */
const ANSWER_NEAR = data.reach - 0.5;
/** after an answer (or the fallback), the tracker shows the way for this long, or until we get there (ms) */
const SHOW_WAY_MS = 120_000;
/** who answers when no Citizen does (the quest's fallback, server/content/quests.json) */
const FALLBACK_NPC = 'chu_sau';
/** this close to Chú Sáu the tracker has done its job (m) */
const AT_NPC = 4;

const ERRORS: Record<WayError, string> = {
  no_account: 'Hỏi đường cần một tài khoản. Tải lại trang để thử lại.',
  not_allowed: 'Bạn không hỏi hoặc chỉ đường được lúc này.',
  busy: 'Bạn đang hỏi đường rồi.',
  unknown_place: 'Chỉ hỏi đường tới các công trình được thôi.',
  too_soon: 'Đợi một chút rồi hỏi lại nhé.',
  gone: 'Người đó không còn hỏi đường nữa.',
  too_far: 'Lại gần hơn chút nữa để chỉ đường.',
  already_helped: 'Hôm nay bạn đã chỉ đường cho người này rồi.',
};

export interface DirectionsDeps {
  net: { sendWay(m: WayClientMsg): boolean; serverNow(): number };
  /** another player's avatar, when the server keeps them in view */
  other(id: number): { position: THREE.Vector3 } | null;
  self(): THREE.Vector3;
  say(text: string): void;
  notify(title: string, text: string): void;
}

/** A place's name, or its id when unknown. */
export function landmarkName(id: string): string {
  return PLACES.find((p) => p.id === id)?.name ?? id;
}

/** where the tracker points once the way is known */
interface Shown {
  goal: string;
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

  constructor(private readonly d: DirectionsDeps) {}

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
        if (!this.calls.has(call.id)) this.d.notify('Có người hỏi đường', `${call.from.name} hỏi đường tới ${landmarkName(call.dest)}. Tới chỗ họ để chỉ đường.`);
        this.calls.set(call.id, call);
        break;
      }
      case 'way_gone':
        this.calls.delete(m.id);
        break;
      case 'way_done':
        this.calls.delete(m.id);
        this.d.say(`Bạn đã chỉ đường tới ${landmarkName(m.dest)} cho ${m.to.name}. Cảm ơn bạn!`);
        break;
      case 'way_error':
        this.d.say(ERRORS[m.code]);
        break;
    }
    this.changed();
  }

  /** What our own ask became. */
  private ours(m: WayAsk): void {
    const place = PLACES.find((p) => p.id === m.dest);
    const name = place?.name ?? m.dest;
    switch (m.state) {
      case 'asking':
        this.ask = m;
        this.shown = null;
        break;
      case 'answered': {
        this.ask = null;
        const helper = m.helper?.name ?? 'Một người Sài Gòn';
        this.d.say(`${helper} chỉ đường cho bạn tới ${name}.`);
        if (place) this.shown = { goal: `Tới ${name} (${helper} chỉ)`, until: Date.now() + SHOW_WAY_MS, at: place.pts[0], near: place.r };
        break;
      }
      case 'unanswered': {
        this.ask = null;
        this.fallback = true;
        this.d.say(m.why === 'nobody'
          ? 'Chưa có người Sài Gòn nào đang chơi. Hỏi Chú Sáu xe ôm ở bùng binh nhé!'
          : 'Không ai tới kịp. Hỏi Chú Sáu xe ôm ở bùng binh nhé!');
        const npc = NPC_DEFS.find((n) => n.id === FALLBACK_NPC);
        if (npc) this.shown = { goal: `Hỏi ${npc.name} đường tới ${name}`, until: Date.now() + SHOW_WAY_MS, at: npc.at, near: AT_NPC };
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
    if (!this.d.net.sendWay({ t: 'way_ask', dest })) this.d.say('Đang mất kết nối, chưa hỏi đường được.');
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
    return c ? `Chỉ đường cho ${c.from.name}` : null;
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
      return { title: 'Hỏi đường', goal: `${c.from.name} hỏi đường tới ${landmarkName(c.dest)}`, target: () => this.d.other(c.from.id)?.position ?? null };
    }
    if (this.ask) return { title: 'Hỏi đường', goal: 'Đang chờ người Sài Gòn tới chỉ đường…', target: () => null };
    const s = this.shown;
    if (s) return { title: 'Hỏi đường', goal: s.goal, target: () => this.to.set(s.at[0], 0, -s.at[1]) };
    return null;
  }

  private changed(): void {
    for (const fn of this.listeners) fn();
  }
}
