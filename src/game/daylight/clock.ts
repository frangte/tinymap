// Saigon's time of day, for every player wherever they are: the city in the game follows the real one (D24).
// `?time=17:30` stops the clock at that time of today in Saigon, to look at a sunset at noon or test the night.
/** Vietnam keeps UTC+7 all year (no daylight saving) */
const SAIGON_OFFSET_MS = 7 * 60 * 60 * 1000;
const HOUR_MS = 60 * 60 * 1000;
const MINUTE_MS = 60 * 1000;

function pad(n: number): string {
  return n < 10 ? `0${n}` : String(n);
}

/** `hh:mm` → that time today in Saigon (ms since the epoch); null for anything else */
function todayAt(hhmm: string): number | null {
  const m = /^(\d{1,2}):(\d{2})$/.exec(hhmm.trim());
  if (!m) return null;
  const h = Number(m[1]);
  const min = Number(m[2]);
  if (h > 23 || min > 59) return null;
  const local = new Date(Date.now() + SAIGON_OFFSET_MS);
  return Date.UTC(local.getUTCFullYear(), local.getUTCMonth(), local.getUTCDate(), h, min) - SAIGON_OFFSET_MS;
}

export class SaigonClock {
  /** the moment the clock is stopped at (ms since the epoch), or null to follow the real time */
  private stoppedAt: number | null;

  constructor(param: string | null) {
    this.stoppedAt = param ? todayAt(param) : null;
  }

  get stopped(): boolean {
    return this.stoppedAt !== null;
  }

  /** Stop at `hh:mm` today in Saigon, or follow the real time again with null. False when `hhmm` is not a time. */
  set(hhmm: string | null): boolean {
    if (hhmm === null) {
      this.stoppedAt = null;
      return true;
    }
    const at = todayAt(hhmm);
    if (at === null) return false;
    this.stoppedAt = at;
    return true;
  }

  now(): Date {
    return new Date(this.stoppedAt ?? Date.now());
  }
}

/** The time of day in Saigon as hours (14.5 is half past two in the afternoon). */
export function saigonHours(date: Date): number {
  return (((date.getTime() + SAIGON_OFFSET_MS) % (24 * HOUR_MS)) + 24 * HOUR_MS) % (24 * HOUR_MS) / HOUR_MS;
}

/** `hh:mm` in Saigon, 24-hour as Vietnam writes it. */
export function saigonHHMM(date: Date): string {
  const minutes = Math.floor(saigonHours(date) * 60 + 1e-6);
  return `${pad(Math.floor(minutes / 60))}:${pad(minutes % 60)}`;
}

/** minutes since the epoch, to tell when the clock text changes */
export function minuteOf(date: Date): number {
  return Math.floor(date.getTime() / MINUTE_MS);
}
