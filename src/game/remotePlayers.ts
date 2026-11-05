// Other players: one Pedestrian each, placed from server snapshots. Every pose is put on the sender's own timeline
// (receive time minus the server-reported age) and the avatar is drawn RENDER_DELAY in the past, blending between
// the two samples around that moment, so movement stays smooth through network jitter. The gait speed comes from
// that blended motion, so the walk / run animation matches what others actually see.
import * as THREE from 'three';
import { Pedestrian } from './character';
import { damp, wrapAngle } from './coords';
import type { InkPass } from './ink';
import type { Look } from './outfits';
import type { PlayerInfo, SnapRow } from '../net/protocol';

/**
 * Draw others this far in the past, so the next sample has always arrived (ms). Worst case: the sender's send
 * interval (100 ms, up to ~133 ms on a 30 fps sender, which sends on frame boundaries) plus the wait for the
 * server's next 12 Hz snapshot (up to 83 ms) ≈ 216 ms. Measured: 150 ms stalled about one frame in 17.
 */
const RENDER_DELAY = 230;
/** senders stream at 10 Hz; a longer silence means they stood still (ms) */
const SEND_INTERVAL = 100;
const MAX_SAMPLES = 24;
/** a jump longer than this between two samples is a respawn, not a walk (m) */
const TELEPORT = 8;
/** in play mode, others further than this from the camera are hidden to save draw calls (m) */
const VISIBLE_DIST = 80;

interface Sample {
  t: number;
  x: number;
  y: number;
  z: number;
  h: number;
}

export class RemotePlayer {
  readonly avatar: Pedestrian;
  /** where it is drawn now (feet), three.js metres */
  readonly position = new THREE.Vector3();
  heading = 0;
  speed = 0;
  private readonly samples: Sample[] = [];
  private placed = false;

  constructor(readonly id: number, public name: string, look: Look) {
    this.avatar = new Pedestrian(look);
    this.avatar.root.name = `Remote ${id}`;
    this.avatar.root.visible = false;
  }

  /** Add a pose received at `t` (performance.now() timeline). Repeats of the last pose are ignored. */
  push(t: number, x: number, y: number, z: number, h: number): void {
    const s = this.samples;
    const last = s[s.length - 1];
    if (last && last.x === x && last.y === y && last.z === z && last.h === h) return;
    if (last) {
      t = Math.max(t, last.t + 1);
      // set off after standing still: hold the old spot until just before the new sample
      if (t - last.t > 2.5 * SEND_INTERVAL) s.push({ ...last, t: t - SEND_INTERVAL });
    }
    s.push({ t, x, y, z, h });
    if (s.length > MAX_SAMPLES) s.splice(0, s.length - MAX_SAMPLES);
  }

  /** @returns false while there is no pose yet */
  update(dt: number, now: number, clock: number): boolean {
    const s = this.samples;
    if (s.length === 0) return false;
    const t = now - RENDER_DELAY;
    while (s.length > 1 && s[1].t <= t) s.shift();
    const a = s[0];
    const b = s.length > 1 ? s[1] : a;
    let x = a.x;
    let y = a.y;
    let z = a.z;
    let h = a.h;
    if (b !== a && t > a.t) {
      if (Math.hypot(b.x - a.x, b.z - a.z) > TELEPORT) {
        ({ x, y, z, h } = b);
      } else {
        const k = (t - a.t) / (b.t - a.t);
        x = a.x + (b.x - a.x) * k;
        y = a.y + (b.y - a.y) * k;
        z = a.z + (b.z - a.z) * k;
        h = wrapAngle(a.h + wrapAngle(b.h - a.h) * k);
      }
    }

    const moved = this.placed ? Math.hypot(x - this.position.x, z - this.position.z) : 0;
    const measured = dt > 0 && moved < TELEPORT ? moved / dt : 0;
    this.speed += (measured - this.speed) * damp(10, dt);
    this.placed = true;
    this.position.set(x, y, z);
    this.heading = h;
    const root = this.avatar.root;
    root.position.copy(this.position);
    root.rotation.y = h;
    this.avatar.update(dt, this.speed, clock);
    return true;
  }
}

export class RemotePlayers {
  private readonly players = new Map<number, RemotePlayer>();

  constructor(private readonly scene: THREE.Scene, private readonly ink: InkPass) {}

  get count(): number {
    return this.players.size;
  }

  get(id: number): RemotePlayer | undefined {
    return this.players.get(id);
  }

  values(): IterableIterator<RemotePlayer> {
    return this.players.values();
  }

  add(info: PlayerInfo, now: number): void {
    this.remove(info.id);
    const r = new RemotePlayer(info.id, info.name, info.look);
    if (info.pose) r.push(now - RENDER_DELAY, ...info.pose);
    this.players.set(info.id, r);
    this.scene.add(r.avatar.root);
    this.ink.track(r.avatar.root);
  }

  remove(id: number): void {
    const r = this.players.get(id);
    if (!r) return;
    this.players.delete(id);
    this.ink.untrack(r.avatar.root);
    this.scene.remove(r.avatar.root);
    r.avatar.dispose();
  }

  clear(): void {
    for (const id of [...this.players.keys()]) this.remove(id);
  }

  setProfile(id: number, name: string, look: Look): void {
    const r = this.players.get(id);
    if (!r) return;
    r.name = name;
    r.avatar.setLook(look);
  }

  /** @param selfId our own row is in every snapshot; skip it */
  applySnap(rows: SnapRow[], receivedAt: number, selfId: number | null): void {
    for (const [id, x, y, z, h, age] of rows) {
      if (id === selfId) continue;
      this.players.get(id)?.push(receivedAt - age, x, y, z, h);
    }
  }

  /**
   * @param viewer in play mode the camera position: avatars further away are hidden; null shows everyone (map view)
   */
  update(dt: number, now: number, clock: number, viewer: THREE.Vector3 | null): void {
    const far = VISIBLE_DIST * VISIBLE_DIST;
    for (const r of this.players.values()) {
      const placed = r.update(dt, now, clock);
      r.avatar.root.visible = placed && (viewer === null || viewer.distanceToSquared(r.position) < far);
    }
  }
}
