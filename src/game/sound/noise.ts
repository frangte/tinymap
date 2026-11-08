// Loopable stereo noise for sounds made in code. The end is cross-faded into the start, so a looping source has
// no seam to hear (a steady bed would otherwise pulse once a loop).

export type NoiseColour = 'brown' | 'pink';

/** samples cross-faded over the loop point */
const SEAM = 4096;

function brown(out: Float32Array): void {
  let last = 0;
  for (let i = 0; i < out.length; i++) {
    last = (last + 0.02 * (Math.random() * 2 - 1)) / 1.02;
    out[i] = last * 3.5;
  }
}

/** Paul Kellet's pink filter: -3 dB an octave, softer than white */
function pink(out: Float32Array): void {
  let b0 = 0, b1 = 0, b2 = 0, b3 = 0, b4 = 0, b5 = 0, b6 = 0;
  for (let i = 0; i < out.length; i++) {
    const w = Math.random() * 2 - 1;
    b0 = 0.99886 * b0 + w * 0.0555179;
    b1 = 0.99332 * b1 + w * 0.0750759;
    b2 = 0.969 * b2 + w * 0.153852;
    b3 = 0.8665 * b3 + w * 0.3104856;
    b4 = 0.55 * b4 + w * 0.5329522;
    b5 = -0.7616 * b5 - w * 0.016898;
    out[i] = (b0 + b1 + b2 + b3 + b4 + b5 + b6 + w * 0.5362) * 0.11;
    b6 = w * 0.115926;
  }
}

export function noiseBuffer(ctx: AudioContext | OfflineAudioContext, seconds: number, colour: NoiseColour): AudioBuffer {
  const len = Math.floor(ctx.sampleRate * seconds);
  const buf = ctx.createBuffer(2, len, ctx.sampleRate);
  const raw = new Float32Array(len + SEAM);
  for (let ch = 0; ch < 2; ch++) {
    if (colour === 'brown') brown(raw);
    else pink(raw);
    const d = buf.getChannelData(ch);
    d.set(raw.subarray(0, len));
    // the first samples fade from what would follow the end into what starts the buffer (equal power)
    for (let i = 0; i < SEAM; i++) {
      const k = i / SEAM;
      d[i] = raw[i] * Math.sqrt(k) + raw[len + i] * Math.sqrt(1 - k);
    }
  }
  return buf;
}
