// The beach: soft, slow surf made in code. A quiet bed of far-off surf runs under waves that swell, break gently
// and drain back over the sand with a hiss. Every wave is its own short voice, so waves overlap like real ones
// and nothing is ever cut off mid-swell.
import type { Soundscape } from './ambient';
import { noiseBuffer } from './noise';

/** seconds a wave lasts, from the first swell to the last of the drain */
const WAVE_LEN: readonly [number, number] = [8, 11.5];
/** the next wave starts this far into the last one (fraction of its length) */
const WAVE_GAP: readonly [number, number] = [0.68, 0.9];
const WAVE_PEAK = 0.34;
const FOAM_PEAK = 0.03;
const BED_LEVEL = 0.07;
/** points in an envelope curve */
const CURVE_POINTS = 64;

const between = (r: readonly [number, number]): number => r[0] + Math.random() * (r[1] - r[0]);

/** A smooth swell: rises over `rise` of its length, then falls away with a long tail; scaled to `peak`. */
function swell(rise: number, peak: number, floor = 0): Float32Array {
  const c = new Float32Array(CURVE_POINTS);
  for (let i = 0; i < CURVE_POINTS; i++) {
    const u = i / (CURVE_POINTS - 1);
    const k = u < rise ? (1 - Math.cos((Math.PI * u) / rise)) / 2 : ((1 + Math.cos((Math.PI * (u - rise)) / (1 - rise))) / 2) ** 1.6;
    c[i] = floor + (peak - floor) * k;
  }
  return c;
}

export function beachSound(ctx: AudioContext, out: AudioNode): Soundscape {
  const deep = noiseBuffer(ctx, 9, 'brown');
  const hiss = noiseBuffer(ctx, 7, 'pink');

  // far-off surf, always there under the waves
  const bed = ctx.createBufferSource();
  bed.buffer = deep;
  bed.loop = true;
  const bedLp = ctx.createBiquadFilter();
  bedLp.type = 'lowpass';
  bedLp.frequency.value = 190;
  bedLp.Q.value = 0.3;
  const bedGain = ctx.createGain();
  bedGain.gain.value = BED_LEVEL;
  bed.connect(bedLp).connect(bedGain).connect(out);
  bed.start();

  const wave = (t: number, len: number): void => {
    const size = 0.75 + Math.random() * 0.4;
    const pan = ctx.createStereoPanner();
    pan.pan.value = Math.random() * 1.2 - 0.6;
    pan.connect(out);

    // the body of the wave: low noise that opens up as it swells and closes as it drains
    const body = ctx.createBufferSource();
    body.buffer = deep;
    body.loop = true;
    const lp = ctx.createBiquadFilter();
    lp.type = 'lowpass';
    lp.Q.value = 0.4;
    lp.frequency.setValueCurveAtTime(swell(0.42, 520 + 260 * size, 210), t, len);
    const bodyGain = ctx.createGain();
    bodyGain.gain.setValueCurveAtTime(swell(0.4, WAVE_PEAK * size), t, len);
    body.connect(lp).connect(bodyGain).connect(pan);

    // the foam running up the sand: a soft hiss that peaks just after the break and drains slowly
    const foam = ctx.createBufferSource();
    foam.buffer = hiss;
    foam.loop = true;
    const bp = ctx.createBiquadFilter();
    bp.type = 'bandpass';
    bp.frequency.value = 1500 + Math.random() * 700;
    bp.Q.value = 0.35;
    const soft = ctx.createBiquadFilter();
    soft.type = 'lowpass';
    soft.frequency.value = 4200;
    const foamGain = ctx.createGain();
    foamGain.gain.setValueCurveAtTime(swell(0.5, FOAM_PEAK * size), t, len);
    foam.connect(bp).connect(soft).connect(foamGain).connect(pan);

    // start each voice somewhere new in the noise, so no two waves sound alike
    body.start(t, Math.random() * deep.duration);
    foam.start(t, Math.random() * hiss.duration);
    body.stop(t + len);
    foam.stop(t + len);
    foam.onended = () => pan.disconnect();
  };

  let next = ctx.currentTime + 0.3;
  return {
    update(t: number): void {
      // a wave missed while the area was quiet starts now, not in a burst of catch-up
      if (next < t - 1) next = t;
      while (next <= t + 0.1) {
        const len = between(WAVE_LEN);
        wave(Math.max(next, ctx.currentTime), len);
        next += len * between(WAVE_GAP);
      }
    },
  };
}
