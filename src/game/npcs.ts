// The NPCs: Cô Ba at the market gate, Chú Tư at his café and the rest (server/content/npcs.json, the file the Go
// server places them from). Each is one figure everyone shares. Online the server says where each one is (welcome)
// and how it turns and gestures (snapshot rows, like a player's); offline they still stand at their spots, they
// just have nothing to say. An NPC with a vehicle has it parked beside them, for the look of it.
import * as THREE from 'three';
import data from '../../server/content/npcs.json';
import { RemotePlayer } from './remotePlayers';
import { Motorbike } from './motorbike';
import { parseLook, type Look } from './outfits';
import type { InkPass } from './ink';
import type { Text } from './archetypes';
import type { NpcInfo, SnapRow, TalkMode } from '../net/protocol';

/** in play mode, NPCs further than this from the camera are hidden, like players (m) */
const VISIBLE_DIST = 80;
/** the game offers to talk a little inside the server's reach, so the press is not refused (m) */
const REACH_SLACK = 0.4;
/** an NPC's bike is parked this far to their right (m), as the server parks a player's */
const PARK_SIDE = 1.2;
/** offline NPCs get ids below this one (counting down); the server's are positive */
const LOCAL_ID = -100;

export interface NpcDef {
  id: string;
  name: string;
  role: Text;
  look: Look;
  /** Blender XY metres: where they stand, and the point they face */
  at: readonly [number, number];
  face: readonly [number, number];
  mode: TalkMode;
  /** how close a player must stand to start talking (m) */
  reach: number;
  /** a vehicle kind parked beside them, '' for none */
  vehicle: string;
}

const MODES: readonly TalkMode[] = ['private', 'public', 'group'];

export const NPC_DEFS: readonly NpcDef[] = data.npcs.map((n): NpcDef => {
  const look = parseLook(n.look);
  const mode = MODES.find((m) => m === n.mode);
  if (!look || !mode || n.at.length !== 2 || n.face.length !== 2) throw new Error(`server/content/npcs.json: bad npc "${n.id}"`);
  const vehicle = 'vehicle' in n && typeof n.vehicle === 'string' ? n.vehicle : '';
  return { id: n.id, name: n.name, role: n.role, look, at: [n.at[0], n.at[1]], face: [n.face[0], n.face[1]], mode, reach: n.reach, vehicle };
});

const byKey = new Map(NPC_DEFS.map((d) => [d.id, d]));

/** heading 0 faces three.js +Z; Blender (x, y) is three (x, -y) */
function restHeading(d: NpcDef): number {
  return Math.atan2(d.face[0] - d.at[0], -(d.face[1] - d.at[1]));
}

export class Npc {
  /** the figure, moved like another player's (same smoothing, gestures from the state flags) */
  readonly actor: RemotePlayer;

  constructor(readonly id: number, readonly def: NpcDef, floorAt: (x: number, z: number) => number) {
    this.actor = new RemotePlayer(id, def.name, def.look, floorAt);
    this.actor.avatar.root.name = `NPC ${def.id}`;
  }

  get position(): THREE.Vector3 {
    return this.actor.position;
  }
}

export class Npcs {
  private readonly all = new Map<number, Npc>();
  private readonly props: Motorbike[] = [];

  constructor(private readonly scene: THREE.Scene, private readonly ink: InkPass, private readonly floorAt: (x: number, z: number) => number) {
    for (const d of NPC_DEFS) if (d.vehicle) this.park(d);
    this.local();
  }

  get(id: number): Npc | undefined {
    return this.all.get(id);
  }

  values(): IterableIterator<Npc> {
    return this.all.values();
  }

  /** Offline (or before the welcome): every NPC at its own spot, with an id of our own. */
  local(): void {
    this.clear();
    const now = performance.now();
    NPC_DEFS.forEach((d, i) => {
      const x = d.at[0];
      const z = -d.at[1];
      this.add(LOCAL_ID - i, d, now, [x, this.floorAt(x, z), z, restHeading(d)]);
    });
  }

  /** The server's NPCs (welcome). One this game does not know (a newer server) is left out. */
  reset(list: readonly NpcInfo[]): void {
    this.clear();
    const now = performance.now();
    for (const n of list) {
      const d = byKey.get(n.npc);
      if (d) this.add(n.id, d, now, n.pose);
    }
  }

  /** NPC rows of a snapshot: how they turn and gesture. One that leaves our view stays where it was last seen. */
  applySnap(rows: readonly SnapRow[], receivedAt: number): void {
    for (const [id, x, y, z, h, s, age] of rows) this.all.get(id)?.actor.push(receivedAt - age, x, y, z, h, s);
  }

  /** The NPC to talk to from here: the nearest one we stand within reach of, or null. */
  nearest(from: THREE.Vector3): Npc | null {
    let best: Npc | null = null;
    let bestD = Infinity;
    for (const n of this.all.values()) {
      const d = Math.hypot(n.position.x - from.x, n.position.z - from.z);
      if (d < n.def.reach - REACH_SLACK && d < bestD) {
        best = n;
        bestD = d;
      }
    }
    return best;
  }

  /** @param viewer in play mode the camera position: further NPCs are hidden; null shows all (map view) */
  update(dt: number, now: number, clock: number, viewer: THREE.Vector3 | null): void {
    const far = VISIBLE_DIST * VISIBLE_DIST;
    for (const n of this.all.values()) {
      const placed = n.actor.update(dt, now, clock);
      n.actor.avatar.root.visible = placed && (viewer === null || viewer.distanceToSquared(n.position) < far);
    }
  }

  private add(id: number, d: NpcDef, now: number, pose: readonly [number, number, number, number]): void {
    const n = new Npc(id, d, this.floorAt);
    // a pose from the past is shown at once, and held until the server says otherwise
    n.actor.push(now - 1000, pose[0], pose[1], pose[2], pose[3]);
    this.all.set(id, n);
    this.scene.add(n.actor.avatar.root);
    this.ink.track(n.actor.avatar.root);
  }

  private clear(): void {
    for (const n of this.all.values()) {
      this.ink.untrack(n.actor.avatar.root);
      this.scene.remove(n.actor.avatar.root);
      n.actor.avatar.dispose();
    }
    this.all.clear();
  }

  /** A bike on its stand to the NPC's right, the bars turned. */
  private park(d: NpcDef): void {
    const bike = new Motorbike();
    const h = restHeading(d);
    const x = d.at[0] - Math.cos(h) * PARK_SIDE;
    const z = -d.at[1] + Math.sin(h) * PARK_SIDE;
    bike.root.name = `NPC ${d.id} bike`;
    bike.root.position.set(x, this.floorAt(x, z), z);
    bike.root.rotation.y = h;
    bike.update(0, 0, 0.35, 0);
    this.scene.add(bike.root);
    this.ink.track(bike.root);
    this.props.push(bike);
  }
}
