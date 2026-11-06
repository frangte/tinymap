// Other players: one Pedestrian each, placed from server snapshots. Every pose is put on the sender's own timeline
// (receive time minus the server-reported age) and the avatar is drawn RENDER_DELAY in the past, blending between
// the two samples around that moment, so movement stays smooth through network jitter. The gait speed comes from
// that blended motion, so the walk / run animation matches what others actually see; the state flags of each
// sample say whether they are jumping, riding or playing an emote.
//
// Everyone on the island is listed here (join / leave), but the server only sends positions of the players near
// us: one that leaves our view is hidden until it comes back.
import * as THREE from 'three';
import { idlePose, Pedestrian, type AvatarPose } from './character';
import { clamp, damp, wrapAngle } from './coords';
import type { InkPass } from './ink';
import type { Look } from './outfits';
import { emoteByCode } from './motion';
import { emoteOf, FLAG_AIR, FLAG_RIDE, type PlayerInfo, type SnapRow } from '../net/protocol';

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
  /** state flags */
  s: number;
}

export class RemotePlayer {
  readonly avatar: Pedestrian;
  /** where it is drawn now (feet), three.js metres */
  readonly position = new THREE.Vector3();
  heading = 0;
  speed = 0;
  /** archetype id, '' for none */
  arch = '';
  /** roll into a turn while riding (rad), worked out from how fast the heading turns */
  lean = 0;
  readonly pose: AvatarPose = idlePose();
  private readonly samples: Sample[] = [];
  private placed = false;
  private yawRate = 0;
  private emoteCode = 0;

  /** @param floorAt the ground height under a point, for the shadow of someone jumping */
  constructor(readonly id: number, public name: string, look: Look, private readonly floorAt: (x: number, z: number) => number) {
    this.avatar = new Pedestrian(look);
    this.avatar.root.name = `Remote ${id}`;
    this.avatar.root.visible = false;
  }

  /** Whether it is in our view: the server has sent where it is, and it has not left since. */
  get inView(): boolean {
    return this.samples.length > 0;
  }

  /** Add a pose received at `t` (performance.now() timeline). Repeats of the last pose are ignored. */
  push(t: number, x: number, y: number, z: number, h: number, flags = 0): void {
    const s = this.samples;
    const last = s[s.length - 1];
    if (last && last.x === x && last.y === y && last.z === z && last.h === h && last.s === flags) return;
    if (last) {
      t = Math.max(t, last.t + 1);
      // set off after standing still: hold the old spot until just before the new sample
      if (t - last.t > 2.5 * SEND_INTERVAL) s.push({ ...last, t: t - SEND_INTERVAL });
    }
    s.push({ t, x, y, z, h, s: flags });
    if (s.length > MAX_SAMPLES) s.splice(0, s.length - MAX_SAMPLES);
  }

  /** It left our view: hide it until the server sends it again. */
  forget(): void {
    this.samples.length = 0;
    this.placed = false;
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
    const turn = this.placed && dt > 0 ? wrapAngle(h - this.heading) / dt : 0;
    this.yawRate += (turn - this.yawRate) * damp(8, dt);
    this.placed = true;
    this.position.set(x, y, z);
    this.heading = h;
    this.flags(a.s, dt);
    const lean = this.pose.ride ? clamp(-this.yawRate * this.speed * 0.07, -0.42, 0.42) : 0;
    this.lean += (lean - this.lean) * damp(6, dt);
    const root = this.avatar.root;
    root.position.copy(this.position);
    root.rotation.y = h;
    root.rotation.z = this.lean;
    this.avatar.setHeight(this.pose.air ? y - this.floorAt(x, z) : 0);
    this.avatar.update(dt, this.speed, clock, this.pose);
    return true;
  }

  /** Turn the state flags of the sample being shown into the avatar's pose. */
  private flags(s: number, dt: number): void {
    this.pose.air = (s & FLAG_AIR) !== 0;
    this.pose.ride = (s & FLAG_RIDE) !== 0;
    const code = emoteOf(s);
    if (code !== this.emoteCode) {
      this.emoteCode = code;
      this.pose.emote = emoteByCode(code)?.id ?? '';
      this.pose.emoteTime = 0;
    } else {
      this.pose.emoteTime += dt;
    }
  }
}

export class RemotePlayers {
  private readonly players = new Map<number, RemotePlayer>();

  constructor(private readonly scene: THREE.Scene, private readonly ink: InkPass, private readonly floorAt: (x: number, z: number) => number) {}

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
    const r = new RemotePlayer(info.id, info.name, info.look, this.floorAt);
    r.arch = info.arch;
    // an older server sent the pose with the player; a current one sends it in the next snapshot
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

  /**
   * @param selfId an older server put our own row in every snapshot; skip it
   * @param out players that left our view
   */
  applySnap(rows: SnapRow[], receivedAt: number, selfId: number | null, out: readonly number[]): void {
    for (const id of out) this.players.get(id)?.forget();
    for (const [id, x, y, z, h, s, age] of rows) {
      if (id === selfId) continue;
      this.players.get(id)?.push(receivedAt - age, x, y, z, h, s);
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
