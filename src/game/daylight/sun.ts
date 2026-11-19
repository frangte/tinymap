// Where the sun stands over Saigon at a given moment: the low-precision solar position of the Astronomical Almanac
// (good to about a hundredth of a degree for centuries around 2000, far finer than a toon shadow needs).
const RAD = Math.PI / 180;
/** the Date of 2000-01-01 12:00 UTC (J2000.0), in days since the Unix epoch */
const J2000_DAYS = 10957.5;
const DAY_MS = 86_400_000;

/** District 1, in front of Chợ Bến Thành */
export const SAIGON = { lat: 10.7725, lon: 106.698 };

export interface SunPlace {
  /** height above the horizon (radians, negative below it), without refraction */
  altitude: number;
  /** compass bearing (radians from north, clockwise: east is π/2) */
  azimuth: number;
}

export function sunPosition(date: Date, latDeg = SAIGON.lat, lonDeg = SAIGON.lon): SunPlace {
  const d = date.getTime() / DAY_MS - J2000_DAYS;
  const g = (357.529 + 0.98560028 * d) * RAD; // mean anomaly
  const q = 280.459 + 0.98564736 * d; // mean longitude (degrees)
  const lambda = (q + 1.915 * Math.sin(g) + 0.02 * Math.sin(2 * g)) * RAD; // ecliptic longitude
  const eps = (23.439 - 0.00000036 * d) * RAD; // obliquity of the ecliptic
  const ra = Math.atan2(Math.cos(eps) * Math.sin(lambda), Math.cos(lambda));
  const dec = Math.asin(Math.sin(eps) * Math.sin(lambda));
  const siderealDeg = 280.46061837 + 360.98564736629 * d + lonDeg; // local sidereal angle
  const hourAngle = siderealDeg * RAD - ra;
  const lat = latDeg * RAD;
  const altitude = Math.asin(Math.sin(lat) * Math.sin(dec) + Math.cos(lat) * Math.cos(dec) * Math.cos(hourAngle));
  const azimuth = Math.atan2(-Math.sin(hourAngle), Math.tan(dec) * Math.cos(lat) - Math.sin(lat) * Math.cos(hourAngle));
  return { altitude, azimuth: (azimuth + 2 * Math.PI) % (2 * Math.PI) };
}
