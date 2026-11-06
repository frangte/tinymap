// Area sound, made in code (no audio files). Each area in server/content/places.json can have its own soundscape:
// it fades in as the player comes into the area and out as they leave it, so every part of the island can sound
// different. Only the beach has one for now; everywhere else is quiet. Starts on the first click or key (browsers
// only allow audio after a user gesture).
import { distance, PLACES, type Place } from '../places';
import { beachSound } from './beach';

/** A sound for one area. It plays into the bus it was made with; the mixer fades that bus. */
export interface Soundscape {
  /** schedule what comes next (ctx.currentTime); called every frame while the area can be heard */
  update(t: number): void;
}

export type MakeSoundscape = (ctx: AudioContext, out: AudioNode) => Soundscape;

/** which area (places.json id) sounds like what; areas not listed are quiet */
const AREA_SOUNDS: Readonly<Partial<Record<string, MakeSoundscape>>> = {
  bai_bien: beachSound,
};

/** metres past an area's reach over which its sound fades away */
const FADE = 20;
const MASTER = 0.9;
/** an area fainter than this is not heard: its sound stops scheduling */
const AUDIBLE = 0.01;
/** a level change smaller than this is not worth a new fade */
const STEP = 0.02;
/** seconds for a fade to get most of the way (setTargetAtTime) */
const FADE_TIME = 0.5;
const STORE_KEY = 'sgq1.sound';

/** Where the listener is, in Blender XY (metres). */
export interface Ear {
  x: number;
  y: number;
}

interface Layer {
  place: Place;
  make: MakeSoundscape;
  bus: GainNode | null;
  scape: Soundscape | null;
  level: number;
}

function readPref(): boolean {
  try {
    return localStorage.getItem(STORE_KEY) !== 'off';
  } catch {
    return true;
  }
}

function writePref(on: boolean): void {
  try {
    localStorage.setItem(STORE_KEY, on ? 'on' : 'off');
  } catch {
    // storage blocked: the choice just lasts for this visit
  }
}

function smooth(e0: number, e1: number, x: number): number {
  const t = Math.max(0, Math.min(1, (x - e0) / (e1 - e0)));
  return t * t * (3 - 2 * t);
}

export class Ambient {
  private ctx: AudioContext | null = null;
  private master: GainNode | null = null;
  private on = readPref();
  private readonly layers: Layer[] = PLACES.flatMap((place) => {
    const make = AREA_SOUNDS[place.id];
    return make ? [{ place, make, bus: null, scape: null, level: 0 }] : [];
  });

  get enabled(): boolean {
    return this.on;
  }

  /** Create the audio graph; call from a click or key handler. Safe to call again. */
  start(): void {
    if (this.ctx) {
      void this.ctx.resume();
      return;
    }
    const Ctor = window.AudioContext ?? (window as unknown as { webkitAudioContext?: typeof AudioContext }).webkitAudioContext;
    if (!Ctor) return;
    const ctx = new Ctor();
    this.ctx = ctx;
    const master = ctx.createGain();
    master.gain.value = this.on ? MASTER : 0;
    master.connect(ctx.destination);
    this.master = master;
    for (const l of this.layers) {
      const bus = ctx.createGain();
      bus.gain.value = 0;
      bus.connect(master);
      l.bus = bus;
      l.level = 0;
      l.scape = l.make(ctx, bus);
    }
  }

  toggle(): boolean {
    this.on = !this.on;
    writePref(this.on);
    if (this.on) this.start();
    if (this.ctx && this.master) this.master.gain.setTargetAtTime(this.on ? MASTER : 0, this.ctx.currentTime, 0.2);
    return this.on;
  }

  /** Fade each area by how far `ear` is from it (null: nowhere, all quiet); cheap enough for every frame. */
  update(ear: Ear | null): void {
    const ctx = this.ctx;
    if (!ctx || !this.on || ctx.state !== 'running') return;
    const t = ctx.currentTime;
    for (const l of this.layers) {
      if (!l.bus || !l.scape) continue;
      const r = l.place.r;
      const want = ear ? 1 - smooth(r, r + FADE, distance(l.place, ear.x, ear.y)) : 0;
      // snap the last bit to 0 or 1, or a slow walk would never finish the fade
      if (Math.abs(want - l.level) > STEP || (want !== l.level && (want === 0 || want === 1))) {
        l.bus.gain.setTargetAtTime(want, t, FADE_TIME);
        l.level = want;
      }
      if (l.level > AUDIBLE) l.scape.update(t);
    }
  }
}
