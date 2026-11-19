// Saigon's weather over HTTP, for the start menu: the socket only opens once the player joins, so the page asks
// GET /api/weather (the same message the socket sends) to show the real sky from the first look.
import { apiUrl } from './auth';
import { parseServerMsg, type WeatherReport } from './protocol';

const FETCH_TIMEOUT = 8000;

/** The weather now from the server next to `socket`, or null (offline, a static host, an older server). */
export async function fetchWeather(socket: string): Promise<WeatherReport | null> {
  const api = apiUrl(socket);
  if (!api) return null;
  const abort = new AbortController();
  const timer = window.setTimeout(() => abort.abort(), FETCH_TIMEOUT);
  try {
    const res = await fetch(`${api}/weather`, { cache: 'no-store', signal: abort.signal });
    if (!res.ok) return null;
    // the same text the socket would send: parseServerMsg checks it the same way
    const m = parseServerMsg(await res.text());
    if (m?.t !== 'weather') return null;
    const { t: _t, ...w } = m;
    return w;
  } catch {
    return null;
  } finally {
    window.clearTimeout(timer);
  }
}
