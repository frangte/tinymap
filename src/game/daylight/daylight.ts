// The city follows Saigon's real time of day (D24): the sun stands where it stands over District 1 now, the
// colours of the light, the sky, the sea and the ponds follow its height (looks.ts), and after dark the moon takes
// over, the street lamps light pools on the ground and windows, shops and lanterns glow (paint.ts NIGHT).
// Worked out once a second; the shadow is re-baked when the light has turned enough to show.
import * as THREE from 'three';
import { fromBlender } from '../coords';
import { NIGHT } from '../paint';
import { setLightDirection, type SceneParts } from '../world';
import { SaigonClock, saigonHours } from './clock';
import { lookFor, newLook, nightness, phaseOf, windowsLit, type Phase } from './looks';
import { sunPosition } from './sun';

const RAD = Math.PI / 180;
const DEG = 180 / Math.PI;
const UPDATE_MS = 1000;
/** re-bake the shadow once the light has turned this much (the sun moves a quarter of a degree a minute) */
const TURN_COS = Math.cos(0.3 * RAD);
/** a lower sun still lights from its bearing but at this height: its shadows would run off the shadow map */
const MIN_LIGHT_ALT = 12;
/** the sun gives the light up below this height and the moon takes it (degrees) */
const HANDOVER_ALT = -2;
/** moonlight: high in the south-south-west, the same every night */
const MOON = { alt: 55, az: 200 };

/** The colours a water surface takes from the look (the uniform colours of sea.ts and water.ts). */
export interface WaterColours {
  deep: THREE.Color;
  shallow: THREE.Color;
  /** foam on the sea, the rim of a pond */
  edge: THREE.Color;
  /** haze over the sea, sky reflected in a pond */
  far: THREE.Color;
}

function smooth(t: number): number {
  const x = THREE.MathUtils.clamp(t, 0, 1);
  return x * x * (3 - 2 * x);
}

/** toward a body `alt` degrees high on compass bearing `az` (north is the map's +Y, east +X), three.js axes */
function towards(alt: number, az: number, out: THREE.Vector3): THREE.Vector3 {
  const a = alt * RAD;
  const b = az * RAD;
  return fromBlender(Math.sin(b) * Math.cos(a), Math.cos(b) * Math.cos(a), Math.sin(a), out);
}

export class Daylight {
  readonly clock: SaigonClock;
  /** the time the city shows, as of the last update */
  now = new Date();
  /** the sun's height (degrees) and whether it is before noon, as of the last update */
  altitude = 0;
  morning = true;
  private readonly look = newLook();
  private readonly dir = new THREE.Vector3();
  private readonly bakedDir = new THREE.Vector3();
  private readonly diff = new THREE.Color();
  private readonly waters: Array<{ colours: WaterColours; kind: 'sea' | 'pond' }> = [];
  private nextAt = -Infinity;

  constructor(
    private readonly parts: SceneParts,
    clock: SaigonClock,
  ) {
    this.clock = clock;
  }

  get phase(): Phase {
    return phaseOf(this.altitude, this.morning);
  }

  /** Tint this water with the time of day (the sea, or the ponds of the map). */
  addWater(colours: WaterColours, kind: 'sea' | 'pond'): void {
    this.waters.push({ colours, kind });
    this.nextAt = -Infinity;
  }

  /** Stop the clock at `hh:mm` in Saigon (null: real time again); applied on the next update. */
  set(hhmm: string | null): boolean {
    const ok = this.clock.set(hhmm);
    this.nextAt = -Infinity;
    return ok;
  }

  /** Call every frame. True when the light has turned and the shadow map should be re-baked. */
  update(nowMs: number): boolean {
    if (nowMs < this.nextAt) return false;
    this.nextAt = nowMs + UPDATE_MS;
    this.now = this.clock.now();
    const sun = sunPosition(this.now);
    this.altitude = sun.altitude * DEG;
    const azimuth = sun.azimuth * DEG;
    this.morning = azimuth < 180;
    const look = lookFor(this.altitude, this.morning, this.look);

    // the lit side: the sun while it is up, then the moon; neither at the handover, so the switch never shows
    const sunUp = this.altitude > HANDOVER_ALT;
    const strength = sunUp ? smooth((this.altitude - HANDOVER_ALT) / 6) : 1 - smooth((this.altitude + 8) / (HANDOVER_ALT + 8));
    if (sunUp) towards(Math.max(this.altitude, MIN_LIGHT_ALT), azimuth, this.dir);
    else towards(MOON.alt, MOON.az, this.dir);
    const { sun: light, ambient, sky } = this.parts;
    ambient.color.copy(look.shade);
    this.diff.copy(look.light).sub(look.shade);
    light.color.setRGB(Math.max(0, this.diff.r), Math.max(0, this.diff.g), Math.max(0, this.diff.b)).multiplyScalar(strength);
    sky.paint(look.skyTop, look.skyMid, look.skyLow);
    for (const w of this.waters) {
      const sea = w.kind === 'sea';
      w.colours.deep.copy(sea ? look.seaDeep : look.pondDeep);
      w.colours.shallow.copy(sea ? look.seaShallow : look.pondShallow);
      w.colours.edge.copy(sea ? look.seaFoam : look.pondRim);
      w.colours.far.copy(sea ? look.seaHaze : look.pondSky);
    }
    NIGHT.uNight.value = nightness(this.altitude);
    NIGHT.uLit.value = windowsLit(saigonHours(this.now));

    if (this.dir.dot(this.bakedDir) > TURN_COS) return false;
    this.bakedDir.copy(this.dir);
    setLightDirection(this.dir);
    return true;
  }
}
