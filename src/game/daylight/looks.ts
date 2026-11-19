// The colours of the city through the day, keyed by the sun's height: the light and shadow tints of the two-tone
// lighting (world.ts), the paper sky, the sea and the ponds. Day is exactly the look the game always had; golden
// hour, blue hour and night are painted for this map (paper and ink at night too: dark, but never black).
import * as THREE from 'three';

interface Palette {
  /** lit side (the sun, or the moon at night) */
  light: string;
  /** shadow side and ambient */
  shade: string;
  skyTop: string;
  skyMid: string;
  skyLow: string;
  seaDeep: string;
  seaShallow: string;
  seaFoam: string;
  seaHaze: string;
  pondDeep: string;
  pondShallow: string;
  pondRim: string;
  pondSky: string;
}

type Key = keyof Palette;
export type Look = Record<Key, THREE.Color>;

const DAY: Palette = {
  light: '#FFF2DC', shade: '#717BA2',
  skyTop: '#A9BCC6', skyMid: '#D9D6CB', skyLow: '#EFE4D0',
  seaDeep: '#3F8399', seaShallow: '#86CBC6', seaFoam: '#F3F1E7', seaHaze: '#DCDCD0',
  pondDeep: '#3E8E9C', pondShallow: '#8FD0C8', pondRim: '#EEF4EC', pondSky: '#D8E9E6',
};
/** sun a few degrees up after dawn: peach light, pink haze */
const GOLDEN_AM: Palette = {
  light: '#FFE2C8', shade: '#787BAA',
  skyTop: '#A6B9CE', skyMid: '#E7D3CC', skyLow: '#F3DACA',
  seaDeep: '#3D7A92', seaShallow: '#90C3BF', seaFoam: '#F6EDE4', seaHaze: '#E9DAD0',
  pondDeep: '#3D8292', pondShallow: '#97C9C0', pondRim: '#F4ECE4', pondSky: '#EAD9D2',
};
/** the same height before sunset: ochre light, long violet shadows */
const GOLDEN_PM: Palette = {
  light: '#FFD49C', shade: '#6E6C9E',
  skyTop: '#9FB2C6', skyMid: '#E6CBA8', skyLow: '#F5D2A0',
  seaDeep: '#3A7290', seaShallow: '#8DBDB6', seaFoam: '#F8E8D0', seaHaze: '#EBCFA9',
  pondDeep: '#3B7C8C', pondShallow: '#9CC6B8', pondRim: '#F6E8D2', pondSky: '#F0D2A8',
};
/** sun just below the horizon before dawn */
const BLUE_AM: Palette = {
  light: '#EAB2A4', shade: '#575F90',
  skyTop: '#4A5A88', skyMid: '#A196B2', skyLow: '#EDC0AA',
  seaDeep: '#2E5272', seaShallow: '#61879C', seaFoam: '#D3CCD6', seaHaze: '#B4A5B6',
  pondDeep: '#30546F', pondShallow: '#638A9D', pondRim: '#D6CDD4', pondSky: '#AA9DB4',
};
/** and after sunset, when the lamps come on */
const BLUE_PM: Palette = {
  light: '#E7A08A', shade: '#525A8C',
  skyTop: '#3F4D7A', skyMid: '#8F81A2', skyLow: '#E2A98E',
  seaDeep: '#2B4B6C', seaShallow: '#5C7F97', seaFoam: '#CBC3CF', seaHaze: '#A994A8',
  pondDeep: '#2E4E69', pondShallow: '#5F8398', pondRim: '#D0C5CD', pondSky: '#9D8EA9',
};
/** moonlight on indigo paper */
const NIGHT: Palette = {
  light: '#9DA9D4', shade: '#454D7C',
  skyTop: '#141B32', skyMid: '#252F4F', skyLow: '#3D486B',
  seaDeep: '#15273E', seaShallow: '#2D4A63', seaFoam: '#8D95AA', seaHaze: '#2E3957',
  pondDeep: '#192B42', pondShallow: '#324F67', pondRim: '#8E98AE', pondSky: '#3A4767',
};

const KEYS = Object.keys(DAY) as Key[];

function toLook(p: Palette): Look {
  const out = {} as Look;
  for (const k of KEYS) out[k] = new THREE.Color(p[k]);
  return out;
}

const LOOKS = {
  day: toLook(DAY),
  goldenAm: toLook(GOLDEN_AM),
  goldenPm: toLook(GOLDEN_PM),
  blueAm: toLook(BLUE_AM),
  bluePm: toLook(BLUE_PM),
  night: toLook(NIGHT),
};

/** sun heights (degrees) where each look is at its fullest; in between they blend */
const AT = { night: -9, blue: -3, golden: 5, day: 22 };

export function newLook(): Look {
  return toLook(DAY);
}

function smooth(t: number): number {
  const x = THREE.MathUtils.clamp(t, 0, 1);
  return x * x * (3 - 2 * x);
}

function blend(a: Look, b: Look, t: number, out: Look): Look {
  const s = smooth(t);
  for (const k of KEYS) out[k].lerpColors(a[k], b[k], s);
  return out;
}

/** The look for a sun `altitude` degrees high; `morning` picks the dawn colours over the dusk ones. */
export function lookFor(altitude: number, morning: boolean, out: Look): Look {
  const blue = morning ? LOOKS.blueAm : LOOKS.bluePm;
  const golden = morning ? LOOKS.goldenAm : LOOKS.goldenPm;
  if (altitude <= AT.night) return blend(LOOKS.night, LOOKS.night, 0, out);
  if (altitude < AT.blue) return blend(LOOKS.night, blue, (altitude - AT.night) / (AT.blue - AT.night), out);
  if (altitude < AT.golden) return blend(blue, golden, (altitude - AT.blue) / (AT.golden - AT.blue), out);
  if (altitude < AT.day) return blend(golden, LOOKS.day, (altitude - AT.golden) / (AT.day - AT.golden), out);
  return blend(LOOKS.day, LOOKS.day, 0, out);
}

/** How dark it is (0 by day, 1 at night): lamps, lit windows and signs come on with it, a little before sunset. */
export function nightness(altitude: number): number {
  return 1 - smooth((altitude + 6) / 8);
}

/** [hour, share] points, straight lines between them */
type Curve = ReadonlyArray<readonly [number, number]>;

/** Share of windows with a light on by the hour in Saigon: most in the evening, few deep in the night. */
const WINDOWS_LIT: Curve = [[0, 0.32], [1, 0.2], [4, 0.12], [5.5, 0.26], [7, 0.2], [17, 0.4], [20, 0.55], [23, 0.42], [24, 0.32]];

function onCurve(curve: Curve, hour: number): number {
  for (let i = 1; i < curve.length; i++) {
    const [h1, v1] = curve[i];
    const [h0, v0] = curve[i - 1];
    if (hour <= h1) return v0 + ((v1 - v0) * (hour - h0)) / (h1 - h0);
  }
  return curve[curve.length - 1][1];
}

export function windowsLit(hour: number): number {
  return onCurve(WINDOWS_LIT, hour);
}

export type Phase = 'night' | 'dawn' | 'day' | 'dusk';

/** In words, for the clock line on the menu. */
export function phaseOf(altitude: number, morning: boolean): Phase {
  if (altitude < -6) return 'night';
  if (altitude < 8) return morning ? 'dawn' : 'dusk';
  return 'day';
}
