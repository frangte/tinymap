// Who a player can be, and what that gives them: server/content/archetypes.json and apps.json, the files the Go
// server reads too (its tests keep them valid). Code never asks which archetype a player is: it asks for a
// capability (an app, a uniform) and reads the archetype's data (badge, spawn, starter look).
import archetypeData from '../../server/content/archetypes.json';
import appData from '../../server/content/apps.json';
import type { HatId, Look, OutfitId } from './outfits';

export interface Text {
  vi: string;
  en?: string;
}

export interface Badge {
  /** label next to a name, in Vietnamese and English (show it with tx) */
  short: Text;
  /** background and text colours */
  color: string;
  text: string;
}

export interface Spawn {
  place: string;
  /** Blender XY metres */
  at: readonly [number, number];
  /** the point the player faces on arrival */
  face: readonly [number, number];
  /** label of the tool button that brings the player back here */
  button: Text;
}

export interface Archetype {
  id: string;
  name: Text;
  pitch: Text;
  badge: Badge;
  spawn: Spawn;
  look: { outfit: OutfitId; hat: HatId };
  /** the starter vehicle (server/content/vehicles.json), parked next to the player; '' for none */
  vehicle: string;
  capabilities: readonly string[];
}

export interface PhoneAppDef {
  id: string;
  name: Text;
  icon: string;
  requires: readonly string[];
}

const pair = (p: readonly number[]): readonly [number, number] => [p[0], p[1]] as const;

export const ARCHETYPES: readonly Archetype[] = archetypeData.archetypes.map((a) => ({
  id: a.id,
  name: a.name,
  pitch: a.pitch,
  badge: a.badge,
  spawn: { place: a.spawn.place, at: pair(a.spawn.at), face: pair(a.spawn.face), button: a.spawn.button },
  look: { outfit: a.look.outfit as OutfitId, hat: a.look.hat as HatId },
  vehicle: ('vehicle' in a ? a.vehicle : undefined) ?? '',
  capabilities: a.capabilities,
}));

export const PHONE_APPS: readonly PhoneAppDef[] = appData.apps.map((a) => ({
  id: a.id,
  name: a.name,
  icon: a.icon,
  requires: ('requires' in a ? a.requires : undefined) ?? [],
}));

interface Restriction {
  outfit?: string;
  hat?: string;
  requires: string;
}
const RESTRICTED: readonly Restriction[] = archetypeData.restricted_looks;
/** what a restricted part becomes when the player may not wear it (same as the server) */
const FALLBACK: { outfit: OutfitId; hat: HatId } = { outfit: 'thun', hat: 'none' };

/** The archetype with this id; undefined for '' (not chosen yet) and ids this build does not know. */
export function archetypeById(id: string): Archetype | undefined {
  return ARCHETYPES.find((a) => a.id === id);
}

/** The capabilities an archetype grants (none without one). */
export function grants(archetype: string): ReadonlySet<string> {
  return new Set(archetypeById(archetype)?.capabilities ?? []);
}

/** The phone apps a player with these capabilities has, in phone order (the server sends the same list). */
export function appsFor(caps: ReadonlySet<string>): string[] {
  return PHONE_APPS.filter((a) => a.requires.every((r) => caps.has(r))).map((a) => a.id);
}

/** May a player with these capabilities wear this outfit / hat? */
export function outfitAllowed(outfit: OutfitId, caps: ReadonlySet<string>): boolean {
  return RESTRICTED.every((r) => r.outfit !== outfit || caps.has(r.requires));
}
export function hatAllowed(hat: HatId, caps: ReadonlySet<string>): boolean {
  return RESTRICTED.every((r) => r.hat !== hat || caps.has(r.requires));
}

/** The look with every part the player may not wear swapped for an open one. */
export function allowedLook(look: Look, caps: ReadonlySet<string>): Look {
  return {
    body: look.body,
    outfit: outfitAllowed(look.outfit, caps) ? look.outfit : FALLBACK.outfit,
    hat: hatAllowed(look.hat, caps) ? look.hat : FALLBACK.hat,
  };
}
