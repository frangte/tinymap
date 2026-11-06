// The quest tracker under the place card: the followed quest, what to do next, and an arrow with the distance to
// where it is (relative to where the camera looks). Tapping it opens the quests app. While a Crab Bike trip goes
// on, a guide (rides.ts) takes the quest's place: the passenger to pick up, the destination. Text only via
// textContent.
import * as THREE from 'three';
import { el } from './hud';
import { current, countOf, type QuestLog } from './questLog';
import { countText, type Objective } from './quests';
import type { Guide } from './rides';

/** closer than this the arrow gives way to "ở đây" (m) */
const HERE = 6;
/** where to go and how far is worked out this often (ms); the arrow turns every frame */
const EVERY = 200;
/** the arrow is rewritten when it turned more than this (degrees) */
const TURN_STEP = 1;

export class QuestTracker {
  private readonly root = el<HTMLButtonElement>('tracker');
  private readonly quest = el('tracker-quest');
  private readonly goal = el('tracker-goal');
  private readonly arrow = el('tracker-arrow');
  private readonly dist = el('tracker-dist');
  private readonly to = new THREE.Vector3();
  private lastAt = -1e9;
  private lastText = '';
  /** the objectives to do now, worked out when the progress changes rather than every frame */
  private goals: Objective[] = [];
  /** shown instead of the quest while set */
  private guide: Guide | null = null;
  private hasTarget = false;
  /** the arrow's angle, unwrapped so it never spins the long way round through ±180° (degrees) */
  private angle = 0;
  private shownAngle = NaN;
  // scratch for the nearest point on a segment
  private px = 0;
  private py = 0;

  constructor(private readonly log: QuestLog, open: () => void) {
    this.root.addEventListener('click', () => {
      open();
      // Space and Enter are the game's again (a focused button would take them)
      this.root.blur();
    });
    log.onChange(() => this.refresh());
    this.refresh();
  }

  /** Point somewhere else than the quest (null: back to the quest). */
  guideTo(g: Guide | null): void {
    this.guide = g;
    this.refresh();
  }

  /** a guide is showing (a tap opens its app rather than the quests) */
  get guiding(): boolean {
    return this.guide !== null;
  }

  /** Every frame in play mode: the arrow turns with the camera; where to go is worked out a few times a second. */
  update(now: number, player: THREE.Vector3, forward: THREE.Vector3): void {
    if (this.root.hidden) return;
    if (now - this.lastAt >= EVERY) {
      this.lastAt = now;
      this.findTarget(player);
    }
    if (!this.hasTarget || this.arrow.hidden) return;
    const dx = this.to.x - player.x;
    const dz = this.to.z - player.z;
    // the angle from where the camera looks to the target, on the ground (clockwise positive, as CSS rotates)
    const a = (Math.atan2(forward.x * dz - forward.z * dx, forward.x * dx + forward.z * dz) * 180) / Math.PI;
    this.angle += ((((a - this.angle) % 360) + 540) % 360) - 180;
    if (Math.abs(this.angle - this.shownAngle) < TURN_STEP) return;
    this.shownAngle = this.angle;
    this.arrow.style.transform = `rotate(${this.angle.toFixed(1)}deg)`;
  }

  /** The nearest place to go among the objectives to do now (a street: its nearest point), and how far it is. */
  private findTarget(player: THREE.Vector3): void {
    // Blender (x, y) is three (x, -y)
    const x = player.x;
    const y = -player.z;
    let best = Infinity;
    const g = this.guide?.target();
    if (g) {
      best = Math.hypot(g.x - player.x, g.z - player.z);
      this.to.set(g.x, 0, g.z);
    }
    for (const o of this.guide ? [] : this.goals) {
      const at = o.at;
      if (!at) continue;
      for (let i = 0; i < at.length; i++) {
        const b = at[i];
        const a = i > 0 ? at[i - 1] : b;
        this.nearestOnSegment(x, y, a[0], a[1], b[0], b[1]);
        const d = Math.hypot(this.px - x, this.py - y);
        if (d < best) {
          best = d;
          this.to.set(this.px, 0, -this.py);
        }
      }
    }
    this.hasTarget = Number.isFinite(best);
    this.arrow.hidden = !this.hasTarget || best < HERE;
    const dist = !this.hasTarget ? '' : best < HERE ? 'ở đây' : `${Math.round(best)} m`;
    if (this.dist.textContent !== dist) this.dist.textContent = dist;
  }

  private nearestOnSegment(px: number, py: number, ax: number, ay: number, bx: number, by: number): void {
    const vx = bx - ax;
    const vy = by - ay;
    const l2 = vx * vx + vy * vy;
    const t = l2 > 0 ? Math.max(0, Math.min(1, ((px - ax) * vx + (py - ay) * vy) / l2)) : 0;
    this.px = ax + vx * t;
    this.py = ay + vy * t;
  }

  private refresh(): void {
    const g = this.guide;
    if (g) {
      this.root.hidden = false;
      this.show(g.title, g.goal, `${g.title}: ${g.goal}`);
      return;
    }
    const q = this.log.tracked();
    this.root.hidden = !q;
    this.goals = q ? current(q) : [];
    if (!q) return;
    const goals = this.goals;
    const o = goals[0];
    const more = goals.length > 1 ? ` (+${goals.length - 1})` : '';
    const count = o ? countText(o, countOf(q, o)) : '';
    const goal = (o?.text.vi ?? '') + more + (count ? ` · ${count}` : '');
    this.show(q.def.title.vi, goal, `Nhiệm vụ: ${q.def.title.vi}. ${goal}. Mở danh sách nhiệm vụ`);
  }

  private show(title: string, goal: string, label: string): void {
    const text = `${title}|${goal}`;
    if (text === this.lastText) return;
    this.lastText = text;
    this.quest.textContent = title;
    this.goal.textContent = goal;
    this.root.setAttribute('aria-label', label);
    this.lastAt = -1e9;
  }
}
