// Saigon's weather in the game (D24, step 2): what the server reports (server/internal/weather, from Open-Meteo),
// eased in so a shower starts and stops over several seconds. The ground stays wet a while after the rain, and a
// storm flashes now and then with its thunder a moment later. Without a report (offline, the single-file build)
// the sky is clear. `?weather=rain` (clear, cloudy, drizzle, rain, storm) shows one whatever the server says.
import { WEATHER_KINDS, type WeatherKind, type WeatherReport } from '../../net/protocol';

/** how heavy the rain and how cloudy the sky each kind is, when nothing more is known (?weather=) */
const PRESETS: Record<WeatherKind, { rain: number; cloud: number }> = {
  clear: { rain: 0, cloud: 0.1 },
  cloudy: { rain: 0, cloud: 0.85 },
  drizzle: { rain: 0.25, cloud: 0.9 },
  rain: { rain: 0.7, cloud: 0.95 },
  storm: { rain: 1, cloud: 1 },
};
/** seconds for the rain and the clouds to get most of the way to a new report */
const RAIN_EASE = 12;
const CLOUD_EASE = 25;
/** the ground is soaked after this many seconds of rain, and dry this many after it stops */
const SOAK_S = 25;
const DRY_S = 420;
/** seconds between lightning strikes in a storm */
const STRIKE_GAP: readonly [number, number] = [7, 22];
/** seconds from the flash to its thunder: near strikes rumble at once and loud */
const THUNDER_DELAY: readonly [number, number] = [0.4, 3.2];
/** how fast a flash fades (1/s) */
const FLASH_FADE = 14;

function asKind(v: string | null): WeatherKind | null {
  return WEATHER_KINDS.find((k) => k === v) ?? null;
}

function between(r: readonly [number, number]): number {
  return r[0] + Math.random() * (r[1] - r[0]);
}

export class Weather {
  /** the weather as reported (or forced): what the menu says */
  kind: WeatherKind = 'clear';
  /** °C, null when unknown */
  temp: number | null = null;
  /** eased toward the report, 0–1 */
  rain = 0;
  cloud = 0;
  /** how wet the ground is, 0–1 */
  wet = 0;
  /** the light of a lightning strike, 0–1, fading fast */
  flash = 0;
  /** called when a strike's thunder should be heard; `near` is 1 for a strike close by */
  onThunder: ((near: number) => void) | null = null;
  private target = PRESETS.clear;
  private forced: WeatherKind | null;
  private reported: WeatherReport | null = null;
  private settled = false;
  private nextStrike = 0;
  private readonly peaks: number[] = [];
  private thunder: { at: number; near: number } | null = null;

  constructor(param: string | null) {
    this.forced = asKind(param);
    if (this.forced) this.apply(this.forced, null);
  }

  /** A report from the server; the first one shows at once, later ones ease in. */
  report(r: WeatherReport): void {
    this.reported = r;
    if (!this.forced) this.apply(r.kind, r);
  }

  /** Show `kind` whatever the server says, or follow it again with null. False for a word that is no weather. */
  set(kind: string | null): boolean {
    if (kind === null) {
      this.forced = null;
      if (this.reported) this.apply(this.reported.kind, this.reported);
      else this.apply('clear', null);
      return true;
    }
    const k = asKind(kind);
    if (!k) return false;
    this.forced = k;
    this.apply(k, null);
    return true;
  }

  /** How grey and flat the light is (0 a clear day, 1 a downpour). */
  get overcast(): number {
    return Math.min(1, Math.max(this.cloud * 0.7, this.rain * 0.9 + (this.kind === 'storm' ? 0.1 : 0)));
  }

  /** Every frame: `dt` seconds since the last, `clock` seconds since the start. */
  update(dt: number, clock: number): void {
    this.rain += (this.target.rain - this.rain) * Math.min(1, dt / RAIN_EASE);
    this.cloud += (this.target.cloud - this.cloud) * Math.min(1, dt / CLOUD_EASE);
    if (this.rain > 0.05) this.wet = Math.min(1, this.wet + (dt / SOAK_S) * (0.4 + this.rain));
    else this.wet = Math.max(0, this.wet - dt / DRY_S);
    this.lightning(clock);
  }

  private apply(kind: WeatherKind, r: WeatherReport | null): void {
    this.kind = kind;
    this.temp = r?.temp ?? null;
    this.target = r ? { rain: r.rain, cloud: r.cloud } : PRESETS[kind];
    if (this.settled) return;
    // the first weather of a visit is simply there, rain on the ground included
    this.settled = true;
    this.rain = this.target.rain;
    this.cloud = this.target.cloud;
    this.wet = this.rain > 0.05 ? 1 : 0;
  }

  private lightning(clock: number): void {
    if (this.kind === 'storm' && this.rain > 0.5 && clock >= this.nextStrike) {
      if (this.nextStrike > 0) {
        // a strike flickers: two or three flashes close together
        this.peaks.push(clock, clock + 0.11);
        if (Math.random() < 0.5) this.peaks.push(clock + 0.3);
        const delay = between(THUNDER_DELAY);
        this.thunder = { at: clock + delay, near: 1 - (delay - THUNDER_DELAY[0]) / (THUNDER_DELAY[1] - THUNDER_DELAY[0]) };
      }
      this.nextStrike = clock + between(STRIKE_GAP);
    }
    let flash = 0;
    for (let i = this.peaks.length - 1; i >= 0; i--) {
      const age = clock - this.peaks[i];
      if (age > 1) this.peaks.splice(i, 1);
      else if (age >= 0) flash = Math.max(flash, Math.exp(-age * FLASH_FADE));
    }
    this.flash = flash;
    if (this.thunder && clock >= this.thunder.at) {
      this.onThunder?.(this.thunder.near);
      this.thunder = null;
    }
  }
}
