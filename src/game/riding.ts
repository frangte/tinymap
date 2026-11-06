// Getting on and off the motorbike, online and off. Online the server decides: the game asks to mount and only
// sits the player on the bike once the server says it is the rider; getting off happens at once and the server
// is told. Offline (no server, or the connection dropped) a player whose archetype starts with a bike gets a
// local one parked beside them, so the driver can still ride.
import { LOCAL_VEHICLE, Vehicles } from './vehicles';
import type { Player } from './player';
import { rideTuning } from './player';
import { vehicleKind } from './motion';
import type { NetClient } from '../net/client';
import type { Pose, VehicleInfo } from '../net/protocol';
import type { NavGrid } from './nav';

/** how close to the bike the button offers to get on (m); the server allows 2.5 */
const MOUNT_REACH = 2.2;
/** offline, the local bike is parked this far to the player's right (m), as the server parks it */
const PARK_SIDE = 1.4;
/** the local player's id for the local bike while offline */
const OFFLINE_SELF = -1;
/** a mount the server has not answered by then is given up (ms): the server drops presses that come too fast */
const PENDING_FOR = 2000;
/** how often to tell the server again that we got off, while it still has us riding (ms) */
const RESYNC_EVERY = 1000;

export interface RidingDeps {
  player: Player;
  vehicles: Vehicles;
  net: NetClient;
  nav: NavGrid;
  /** the vehicle kind the player's archetype starts with, '' for none */
  starter(): string;
  notify(text: string): void;
}

export class Riding {
  /** the vehicle we asked the server to get on, 0 for none, and when */
  private pending = 0;
  private pendingAt = 0;
  private resyncAt = 0;

  constructor(private readonly d: RidingDeps) {}

  /** Our id as a rider: the server's while online, a local one offline. */
  get self(): number {
    return this.d.net.selfId ?? OFFLINE_SELF;
  }

  /** The label of the action button: get off, get on (next to our bike), or null for nothing to do. */
  label(): string | null {
    if (this.d.player.riding) return 'Xuống xe';
    if (this.pending !== 0) return 'Đang lên xe…';
    return this.nearby() ? 'Lên xe' : null;
  }

  /** E / the action button. */
  use(): void {
    if (this.d.player.riding) {
      this.getOff();
      return;
    }
    const v = this.nearby();
    if (!v || this.pending !== 0) return;
    if (v.id !== LOCAL_VEHICLE && this.d.net.sendMount(v.id)) {
      this.pending = v.id;
      this.pendingAt = performance.now();
    } else if (v.id === LOCAL_VEHICLE) {
      this.sitOn(v);
    }
  }

  /** Get off before a respawn (the server takes the player off too, even if the respawn does not land). */
  leave(): void {
    if (this.d.player.riding) this.getOff();
  }

  /**
   * Every frame: give up a mount the server never answered, and if the server still has us riding when we are
   * on foot (a dismount it dropped), tell it again.
   */
  update(now: number): void {
    if (this.pending !== 0 && now - this.pendingAt > PENDING_FOR) {
      this.pending = 0;
      this.d.notify('Không lên xe được, thử lại nhé.');
    }
    if (this.d.player.riding || this.pending !== 0 || !this.d.net.isOnline || now - this.resyncAt < RESYNC_EVERY) return;
    if (this.d.vehicles.riddenBy(this.self)) {
      this.resyncAt = now;
      this.d.net.sendDismount();
    }
  }

  /** After a respawn offline, the local bike comes along. */
  bringBike(): void {
    if (this.d.vehicles.get(LOCAL_VEHICLE)) this.parkLocal();
  }

  // ---- the server

  /** A welcome: the server's vehicles replace ours, and a new arrival is on foot. */
  welcome(list: readonly VehicleInfo[]): void {
    this.pending = 0;
    if (this.d.player.riding) this.d.player.dismount();
    this.d.vehicles.reset(list);
  }

  vehicle(v: VehicleInfo): void {
    this.d.vehicles.upsert(v);
    const p = this.d.player;
    if (v.rider === this.self && this.pending === v.id) {
      this.pending = 0;
      this.sitOn(v);
    } else if (p.riding?.id === v.id && v.rider !== this.self) {
      p.dismount(); // the server took us off
    }
  }

  gone(id: number): void {
    if (this.d.player.riding?.id === id) this.d.player.dismount();
    if (this.pending === id) this.pending = 0;
    this.d.vehicles.remove(id);
  }

  refused(): void {
    this.pending = 0;
    this.d.notify('Không lên xe được: lại gần xe của bạn hơn nhé.');
  }

  /** The connection dropped: the server's vehicles go, and a local bike takes over (we stay on it if riding). */
  offline(): void {
    this.pending = 0;
    const p = this.d.player;
    const riding = p.riding;
    this.d.vehicles.reset([]);
    if (!this.d.starter()) {
      if (riding) p.dismount();
      return;
    }
    this.parkLocal();
    if (riding) {
      p.riding = { id: LOCAL_VEHICLE, tuning: riding.tuning };
      this.d.vehicles.park(LOCAL_VEHICLE, [p.position.x, p.position.y, p.position.z, p.heading], OFFLINE_SELF);
    }
  }

  /** Offline from the start: park the local bike, if the archetype has one. */
  start(): void {
    if (this.d.net.isOnline || !this.d.starter()) return;
    this.parkLocal();
  }

  // ----

  /** Our own parked vehicle within reach, if any. */
  private nearby(): VehicleInfo | null {
    const v = this.d.vehicles.ownedBy(this.self);
    if (!v || v.rider !== 0) return null;
    const p = this.d.player.position;
    return Math.hypot(v.pose[0] - p.x, v.pose[2] - p.z) <= MOUNT_REACH ? v : null;
  }

  private sitOn(v: VehicleInfo): void {
    const kind = vehicleKind(v.kind);
    if (!kind) return;
    const [x, , z, h] = v.pose;
    this.d.player.mount(v.id, rideTuning(kind.speed), x, z, h);
    this.d.vehicles.park(v.id, v.pose, this.self);
  }

  private getOff(): void {
    const id = this.d.player.riding?.id ?? 0;
    const parked = this.d.player.dismount();
    this.d.vehicles.park(id, parked, 0);
    if (id !== LOCAL_VEHICLE) this.d.net.sendDismount();
  }

  /** Park the local bike at the player's right hand, on a spot where it fits. */
  private parkLocal(): void {
    const kind = this.d.starter();
    const p = this.d.player;
    const h = p.heading;
    // three.js heading h faces (sin h, cos h); the right hand is (-cos h, sin h)
    const fit = this.d.nav.nearestStanding(p.position.x - Math.cos(h) * PARK_SIDE, p.position.z + Math.sin(h) * PARK_SIDE, 0.45, 4);
    const [x, z] = fit ?? [p.position.x, p.position.z];
    const pose: Pose = [x, this.d.nav.floorAt(x, z), z, h];
    if (this.d.vehicles.get(LOCAL_VEHICLE)) this.d.vehicles.park(LOCAL_VEHICLE, pose);
    else this.d.vehicles.upsert({ id: LOCAL_VEHICLE, kind, owner: OFFLINE_SELF, rider: 0, pose });
  }
}
