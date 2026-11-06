// Every vehicle on the island, drawn: parked ones where the server says, ridden ones under their rider (our own
// player or a remote one, wherever it is drawn this frame). Online the list comes from the server (welcome,
// vehicle, vehicle_gone); offline the game parks one local bike for a player whose archetype starts with one.
import * as THREE from 'three';
import { Motorbike } from './motorbike';
import type { InkPass } from './ink';
import type { Pose, VehicleInfo } from '../net/protocol';
import { damp, wrapAngle } from './coords';

/** a bike further than this from the camera is hidden in play mode (m), like other players */
const VISIBLE_DIST = 80;
/** the id of the bike parked offline; server ids are positive */
export const LOCAL_VEHICLE = -1;

/** Where a rider is drawn this frame. */
export interface RiderView {
  position: THREE.Vector3;
  heading: number;
  /** forward ground speed (m/s) */
  speed: number;
  /** roll into the turn (rad) */
  lean: number;
}

interface Entry {
  info: VehicleInfo;
  bike: Motorbike;
  /** heading last frame, for the steering angle */
  lastHeading: number;
  steer: number;
}

export class Vehicles {
  private readonly all = new Map<number, Entry>();
  /** owner id → vehicle id, so the per-frame lookups do not walk the map */
  private readonly byOwner = new Map<number, number>();

  constructor(private readonly scene: THREE.Scene, private readonly ink: InkPass) {}

  get(id: number): VehicleInfo | undefined {
    return this.all.get(id)?.info;
  }

  /** The vehicle this player owns (online: by player id; offline: the local one). */
  ownedBy(owner: number): VehicleInfo | undefined {
    const id = this.byOwner.get(owner);
    return id === undefined ? undefined : this.all.get(id)?.info;
  }

  /** The vehicle this player rides, if any. */
  riddenBy(rider: number): VehicleInfo | undefined {
    for (const e of this.all.values()) if (e.info.rider === rider) return e.info;
    return undefined;
  }

  /** Replace every vehicle with the server's list (welcome). */
  reset(list: readonly VehicleInfo[]): void {
    for (const id of [...this.all.keys()]) this.remove(id);
    for (const v of list) this.upsert(v);
  }

  /** A vehicle arrived or changed. */
  upsert(info: VehicleInfo): void {
    const e = this.all.get(info.id);
    this.byOwner.set(info.owner, info.id);
    if (e) {
      e.info = { ...info, pose: [...info.pose] };
      return;
    }
    const bike = new Motorbike();
    bike.root.name = `Vehicle ${info.id}`;
    bike.root.visible = false;
    this.scene.add(bike.root);
    this.ink.track(bike.root);
    this.all.set(info.id, { info: { ...info, pose: [...info.pose] }, bike, lastHeading: info.pose[3], steer: 0 });
  }

  remove(id: number): void {
    const e = this.all.get(id);
    if (!e) return;
    this.all.delete(id);
    if (this.byOwner.get(e.info.owner) === id) this.byOwner.delete(e.info.owner);
    this.ink.untrack(e.bike.root);
    this.scene.remove(e.bike.root);
    e.bike.dispose();
  }

  /** Park a vehicle (our own, as we get off): it stays here until the server says otherwise. */
  park(id: number, pose: Readonly<Pose>, rider = 0): void {
    const e = this.all.get(id);
    if (!e) return;
    e.info.pose = [pose[0], pose[1], pose[2], pose[3]];
    e.info.rider = rider;
  }

  /**
   * @param riderView where the rider with this id is drawn now, or null when they are not (out of view)
   * @param viewer in play mode the camera position: further bikes are hidden; null shows all (map view)
   */
  update(dt: number, riderView: (id: number) => RiderView | null, viewer: THREE.Vector3 | null): void {
    const far = VISIBLE_DIST * VISIBLE_DIST;
    for (const e of this.all.values()) {
      const root = e.bike.root;
      const rider = e.info.rider !== 0 ? riderView(e.info.rider) : null;
      if (e.info.rider !== 0 && !rider) {
        root.visible = false;
        continue;
      }
      if (rider) {
        root.position.copy(rider.position);
        root.rotation.y = rider.heading;
        const turn = dt > 0 ? wrapAngle(rider.heading - e.lastHeading) / dt : 0;
        e.lastHeading = rider.heading;
        e.steer += (Math.max(-0.5, Math.min(0.5, turn * 0.35)) - e.steer) * damp(8, dt);
        e.bike.update(dt, rider.speed, e.steer, rider.lean);
      } else {
        const [x, y, z, h] = e.info.pose;
        root.position.set(x, y, z);
        root.rotation.y = h;
        e.lastHeading = h;
        e.steer += (0.35 - e.steer) * damp(4, dt); // parked with the bars turned, on its stand
        e.bike.update(dt, 0, e.steer, 0);
      }
      root.visible = viewer === null || viewer.distanceToSquared(root.position) < far;
    }
  }
}
