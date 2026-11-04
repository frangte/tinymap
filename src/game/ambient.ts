// Ambient sound made in code (no audio files): surf from filtered noise that swells and recedes, birdsong chirps
// and the odd gull call. Starts on the first click (browsers only allow audio after a user gesture).

const STORE_KEY = 'sgq1.sound';

function makeNoise(ctx: AudioContext, seconds: number, brown: boolean): AudioBuffer {
  const len = Math.floor(ctx.sampleRate * seconds);
  const buf = ctx.createBuffer(2, len, ctx.sampleRate);
  for (let ch = 0; ch < 2; ch++) {
    const d = buf.getChannelData(ch);
    let last = 0;
    for (let i = 0; i < len; i++) {
      const w = Math.random() * 2 - 1;
      if (brown) {
        last = (last + 0.02 * w) / 1.02;
        d[i] = last * 3.5;
      } else {
        d[i] = w;
      }
    }
    // fade the loop seam
    for (let i = 0; i < 2000; i++) {
      const k = i / 2000;
      d[i] *= k;
      d[len - 1 - i] *= k;
    }
  }
  return buf;
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

export class Ambient {
  private ctx: AudioContext | null = null;
  private master: GainNode | null = null;
  private surf: GainNode | null = null;
  private surfFilter: BiquadFilterNode | null = null;
  private wash: GainNode | null = null;
  private on = readPref();
  /** 0 inland … 1 on the beach */
  private coast = 0.4;
  private nextSwell = 0;
  private nextBird = 0;
  private nextGull = 0;

  get enabled(): boolean {
    return this.on;
  }

  /** Create the audio graph; call from a click handler. Safe to call again. */
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
    master.gain.value = this.on ? 0.9 : 0;
    master.connect(ctx.destination);
    this.master = master;

    // low rumbling surf
    const rumble = ctx.createBufferSource();
    rumble.buffer = makeNoise(ctx, 6, true);
    rumble.loop = true;
    const lp = ctx.createBiquadFilter();
    lp.type = 'lowpass';
    lp.frequency.value = 420;
    const surf = ctx.createGain();
    surf.gain.value = 0.0;
    rumble.connect(lp).connect(surf).connect(master);
    rumble.start();
    this.surf = surf;
    this.surfFilter = lp;

    // the hiss of water running up the sand
    const hiss = ctx.createBufferSource();
    hiss.buffer = makeNoise(ctx, 4, false);
    hiss.loop = true;
    const bp = ctx.createBiquadFilter();
    bp.type = 'bandpass';
    bp.frequency.value = 2400;
    bp.Q.value = 0.5;
    const wash = ctx.createGain();
    wash.gain.value = 0.0;
    hiss.connect(bp).connect(wash).connect(master);
    hiss.start();
    this.wash = wash;

    this.nextSwell = ctx.currentTime + 0.5;
    this.nextBird = ctx.currentTime + 2;
    this.nextGull = ctx.currentTime + 6;
  }

  toggle(): boolean {
    this.on = !this.on;
    writePref(this.on);
    if (this.on) this.start();
    if (this.ctx && this.master) this.master.gain.setTargetAtTime(this.on ? 0.9 : 0, this.ctx.currentTime, 0.2);
    return this.on;
  }

  /** How close the listener is to the beach (0 inland, 1 at the water). */
  setCoast(k: number): void {
    this.coast = Math.max(0, Math.min(1, k));
  }

  /** Schedule the next swells and calls; cheap enough to call every frame. */
  update(): void {
    const ctx = this.ctx;
    if (!ctx || !this.on || ctx.state !== 'running' || !this.surf || !this.wash || !this.surfFilter) return;
    const t = ctx.currentTime;
    if (t >= this.nextSwell) {
      // one wave: build for ~2.5 s, break, drain away
      const len = 6 + Math.random() * 4;
      const peak = (0.18 + 0.5 * this.coast) * (0.75 + Math.random() * 0.4);
      const g = this.surf.gain;
      g.cancelScheduledValues(t);
      g.setValueAtTime(g.value, t);
      g.linearRampToValueAtTime(peak, t + len * 0.38);
      g.linearRampToValueAtTime(peak * 0.25, t + len);
      this.surfFilter.frequency.cancelScheduledValues(t);
      this.surfFilter.frequency.setValueAtTime(320, t);
      this.surfFilter.frequency.linearRampToValueAtTime(700 + 400 * this.coast, t + len * 0.4);
      this.surfFilter.frequency.linearRampToValueAtTime(300, t + len);
      const w = this.wash.gain;
      w.cancelScheduledValues(t);
      w.setValueAtTime(w.value, t);
      w.linearRampToValueAtTime(0.012, t + len * 0.36);
      w.linearRampToValueAtTime(0.05 * this.coast + 0.012, t + len * 0.5);
      w.linearRampToValueAtTime(0.004, t + len * 0.95);
      this.nextSwell = t + len * (0.85 + Math.random() * 0.2);
    }
    if (t >= this.nextBird) {
      this.chirp(t, 1 - this.coast * 0.6);
      this.nextBird = t + 2.5 + Math.random() * 6;
    }
    if (t >= this.nextGull && this.coast > 0.3) {
      this.gull(t, this.coast);
      this.nextGull = t + 9 + Math.random() * 14;
    }
  }

  private voice(pan: number): { out: GainNode; ctx: AudioContext } | null {
    const ctx = this.ctx;
    if (!ctx || !this.master) return null;
    const out = ctx.createGain();
    out.gain.value = 0;
    const p = ctx.createStereoPanner();
    p.pan.value = pan;
    out.connect(p).connect(this.master);
    return { out, ctx };
  }

  /** a small songbird: 2-4 quick rising notes */
  private chirp(t: number, level: number): void {
    const v = this.voice(Math.random() * 1.6 - 0.8);
    if (!v) return;
    const n = 2 + Math.floor(Math.random() * 3);
    const base = 2600 + Math.random() * 1600;
    const o = v.ctx.createOscillator();
    o.type = 'sine';
    o.connect(v.out);
    let s = t;
    for (let i = 0; i < n; i++) {
      const f = base * (1 + i * 0.06);
      o.frequency.setValueAtTime(f * 0.82, s);
      o.frequency.exponentialRampToValueAtTime(f * 1.18, s + 0.07);
      v.out.gain.setValueAtTime(0, s);
      v.out.gain.linearRampToValueAtTime(0.035 * level, s + 0.015);
      v.out.gain.linearRampToValueAtTime(0, s + 0.08);
      s += 0.11 + Math.random() * 0.05;
    }
    o.start(t);
    o.stop(s + 0.05);
  }

  /** a distant gull: two falling, slightly rough cries */
  private gull(t: number, level: number): void {
    const v = this.voice(Math.random() * 1.4 - 0.7);
    if (!v) return;
    const o = v.ctx.createOscillator();
    o.type = 'triangle';
    const bp = v.ctx.createBiquadFilter();
    bp.type = 'bandpass';
    bp.frequency.value = 1500;
    bp.Q.value = 1.2;
    o.connect(bp).connect(v.out);
    let s = t;
    for (let i = 0; i < 2; i++) {
      o.frequency.setValueAtTime(1350, s);
      o.frequency.exponentialRampToValueAtTime(820, s + 0.32);
      v.out.gain.setValueAtTime(0, s);
      v.out.gain.linearRampToValueAtTime(0.03 * level, s + 0.04);
      v.out.gain.linearRampToValueAtTime(0, s + 0.34);
      s += 0.42;
    }
    o.start(t);
    o.stop(s + 0.05);
  }
}
