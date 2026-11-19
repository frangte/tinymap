// Rain and thunder, made in code like the surf (beach.ts). The rain is two loops of noise: a bright hiss of drops
// and a low wash of water running off; its bus is faded by the weather (Ambient.setRain). Thunder is a burst of
// deep noise that cracks when the strike was near and only rumbles when it was far.
import { noiseBuffer } from './noise';

const HISS_LEVEL = 0.2;
const WASH_LEVEL = 0.16;

export interface RainSound {
  /** a thunderclap now; `near` is 1 close by (loud, a crack first), 0 far off (a low rumble) */
  thunder(near: number): void;
}

export function rainSound(ctx: AudioContext, out: AudioNode): RainSound {
  const pinkNoise = noiseBuffer(ctx, 6, 'pink');
  const brownNoise = noiseBuffer(ctx, 8, 'brown');

  const hiss = ctx.createBufferSource();
  hiss.buffer = pinkNoise;
  hiss.loop = true;
  const hp = ctx.createBiquadFilter();
  hp.type = 'highpass';
  hp.frequency.value = 900;
  const lp = ctx.createBiquadFilter();
  lp.type = 'lowpass';
  lp.frequency.value = 7500;
  const hissGain = ctx.createGain();
  hissGain.gain.value = HISS_LEVEL;
  hiss.connect(hp).connect(lp).connect(hissGain).connect(out);
  hiss.start();

  const wash = ctx.createBufferSource();
  wash.buffer = brownNoise;
  wash.loop = true;
  const washLp = ctx.createBiquadFilter();
  washLp.type = 'lowpass';
  washLp.frequency.value = 520;
  const washGain = ctx.createGain();
  washGain.gain.value = WASH_LEVEL;
  wash.connect(washLp).connect(washGain).connect(out);
  wash.start();

  return {
    thunder(near: number): void {
      const t = ctx.currentTime;
      const len = 3.5 + Math.random() * 2.5;
      const src = ctx.createBufferSource();
      src.buffer = brownNoise;
      src.loop = true;
      const low = ctx.createBiquadFilter();
      low.type = 'lowpass';
      low.Q.value = 0.6;
      // a near strike opens bright for the crack, then closes into the rumble
      low.frequency.setValueAtTime(140 + 900 * near, t);
      low.frequency.exponentialRampToValueAtTime(110 + 90 * near, t + 0.6 + len * 0.3);
      const g = ctx.createGain();
      const peak = 0.35 + 0.55 * near;
      g.gain.setValueAtTime(0.0001, t);
      g.gain.exponentialRampToValueAtTime(peak, t + 0.04 + (1 - near) * 0.5);
      // rolling: a few swells on the way down
      for (let k = 1; k <= 3; k++) {
        const at = t + (len * k) / 5;
        g.gain.exponentialRampToValueAtTime(peak * (0.25 + Math.random() * 0.35) * (1 - k * 0.18), at);
      }
      g.gain.exponentialRampToValueAtTime(0.0001, t + len);
      src.connect(low).connect(g).connect(out);
      src.start(t, Math.random() * 6);
      src.stop(t + len + 0.1);
    },
  };
}
