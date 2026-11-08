// The account token. On the first visit the server makes a guest account (POST /api/auth/guest); the token is
// kept in localStorage and sent in every hello, so the server knows it is the same player next time. Storage can
// be blocked (private windows): the token then lasts for this visit only, like the profile (src/game/profile.ts).
// Without a token at all (the server down, a static host, an older server) the game still plays, as a guest.

const STORAGE_KEY = 'sg.token.v1';
/** the server issues 43 base64url characters; protocol.TokenMax allows up to 64 */
const TOKEN_RE = /^[A-Za-z0-9_-]{20,64}$/;
const FETCH_TIMEOUT = 5000;

export function loadToken(): string | null {
  try {
    const t = localStorage.getItem(STORAGE_KEY);
    return t && TOKEN_RE.test(t) ? t : null;
  } catch {
    return null; // storage blocked
  }
}

export function saveToken(token: string): void {
  try {
    localStorage.setItem(STORAGE_KEY, token);
  } catch {
    // storage blocked: the token lasts for this visit
  }
}

export function clearToken(): void {
  try {
    localStorage.removeItem(STORAGE_KEY);
  } catch {
    // storage blocked: nothing was saved
  }
}

/** The HTTP API next to the game socket (ws(s)://host/ws → http(s)://host/api), or null if the URL is unusable. */
export function apiUrl(socket: string): string | null {
  try {
    const u = new URL(socket);
    if (u.protocol !== 'ws:' && u.protocol !== 'wss:') return null;
    u.protocol = u.protocol === 'wss:' ? 'https:' : 'http:';
    u.pathname = `${u.pathname.replace(/\/ws\/?$/, '')}/api`;
    u.search = '';
    u.hash = '';
    return u.toString();
  } catch {
    return null;
  }
}

/** A new guest token, or null when the server cannot be reached or gives none (the game then plays without one). */
export async function fetchGuestToken(api: string): Promise<string | null> {
  const abort = new AbortController();
  const timer = window.setTimeout(() => abort.abort(), FETCH_TIMEOUT);
  try {
    // a POST without a body or custom headers: a simple request, so a server on another domain needs no preflight
    const res = await fetch(`${api}/auth/guest`, { method: 'POST', cache: 'no-store', signal: abort.signal });
    if (!res.ok) return null;
    const body: unknown = await res.json();
    const token = typeof body === 'object' && body !== null ? (body as { token?: unknown }).token : undefined;
    return typeof token === 'string' && TOKEN_RE.test(token) ? token : null;
  } catch {
    return null; // offline, a static host, an older server, a timeout
  } finally {
    window.clearTimeout(timer);
  }
}
