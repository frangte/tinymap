// The practical of the A1 licence as the player lives it (docs/design/13-driver.md): the school's bike is lent, the
// clock starts when the player gets on, and the quest tracker points at each checkpoint in turn. The server runs
// the test (checkpoints, time, getting off); this only shows what it says. The countdown is worked out from the
// seconds the server last reported, so it needs no more messages than one per checkpoint.
import * as THREE from 'three';
import type { DriveTest } from '../net/protocol';
import type { Guide } from './rides';
import { L } from './i18n/driveTest';

export interface DriveTestDeps {
  /** a line in the chat log */
  notify(text: string): void;
  /** the guide changed (or its countdown moved): the tracker should read it again */
  onChange(): void;
}

export class DriveTestRun {
  private state: DriveTest | null = null;
  /** performance.now() when the server's state arrived */
  private at = 0;
  private lastSecond = -1;
  private readonly target = new THREE.Vector3();

  constructor(private readonly d: DriveTestDeps) {}

  /** The server's word on the practical. */
  handle(t: DriveTest, now: number): void {
    this.at = now;
    this.lastSecond = -1;
    if (t.state === 'pass') {
      this.state = null;
      this.d.notify(L.passed);
    } else if (t.state === 'fail') {
      this.state = null;
      this.d.notify(L.failed(t.reason));
    } else {
      this.state = t;
      if (t.state === 'lent') this.d.notify(L.lent);
    }
    this.d.onChange();
  }

  /** The connection dropped: the test is over (the server drops it too). */
  reset(): void {
    if (!this.state) return;
    this.state = null;
    this.d.onChange();
  }

  get running(): boolean {
    return this.state !== null;
  }

  /** Seconds left on the clock now, null before it started. */
  left(now: number): number | null {
    const s = this.state;
    if (!s || s.state !== 'run') return null;
    return Math.max(0, s.left - (now - this.at) / 1000);
  }

  /** Every frame: the tracker's line follows the clock, once a second. */
  tick(now: number): void {
    const left = this.left(now);
    if (left === null) return;
    const second = Math.ceil(left);
    if (second === this.lastSecond) return;
    this.lastSecond = second;
    this.d.onChange();
  }

  /** Where the tracker points while the practical goes on; null when it is not. */
  guide(now: () => number): Guide | null {
    const s = this.state;
    if (!s) return null;
    const self = this;
    return {
      get title() {
        return L.title;
      },
      get goal() {
        if (s.state === 'lent') return L.getOn;
        const left = Math.ceil(self.left(now()) ?? s.left);
        return L.ride(s.index + 1, s.total, `${Math.floor(left / 60)}:${String(left % 60).padStart(2, '0')}`);
      },
      target: () => (s.state === 'run' && s.next ? this.target.set(s.next[0], 0, s.next[1]) : null),
    };
  }
}
