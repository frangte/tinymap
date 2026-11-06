// How things move: server/content/movement.json, vehicles.json and emotes.json, the files the Go server checks
// moves against (its tests keep them valid). Speeds here are the ones the server allows, times its tolerance.
import movementData from '../../server/content/movement.json';
import vehicleData from '../../server/content/vehicles.json';
import emoteData from '../../server/content/emotes.json';
import type { Text } from './archetypes';

export interface Movement {
  walkSpeed: number;
  runSpeed: number;
  /** take-off speed of a jump (m/s) and the gravity that brings it down (m/s²) */
  jumpSpeed: number;
  gravity: number;
}

export const MOVEMENT: Readonly<Movement> = {
  walkSpeed: movementData.walk_speed,
  runSpeed: movementData.run_speed,
  jumpSpeed: movementData.jump_speed,
  gravity: movementData.gravity,
};

export interface VehicleKind {
  id: string;
  name: Text;
  /** top speed (m/s) */
  speed: number;
  seats: number;
  /** the capability seat 0 needs */
  requires: string;
}

export const VEHICLES: readonly VehicleKind[] = vehicleData.vehicles;

export function vehicleKind(id: string): VehicleKind | undefined {
  return VEHICLES.find((v) => v.id === id);
}

export interface Emote {
  id: string;
  /** what the state flags carry (bits 4-7) */
  code: number;
  name: Text;
  /** how long one plays; 0 loops until the player moves */
  seconds: number;
  requires?: string;
}

export const EMOTES: readonly Emote[] = emoteData.emotes;

export function emoteByCode(code: number): Emote | undefined {
  return code === 0 ? undefined : EMOTES.find((e) => e.code === code);
}

/** The emotes a player with these capabilities may play, in menu order. */
export function emotesFor(caps: ReadonlySet<string>): Emote[] {
  return EMOTES.filter((e) => !e.requires || caps.has(e.requires));
}
