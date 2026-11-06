// WebSocket client for the multiplayer server: says hello with the player's profile and account token, streams the
// player's pose, and reconnects with backoff. The game keeps working offline (a static host, the server down): it
// just walks alone.
import type { Profile } from '../game/profile';
import { wrapAngle } from '../game/coords';
import type { Look } from '../game/outfits';
import { apiUrl, clearToken, fetchGuestToken, loadToken, saveToken } from './auth';
import { parseServerMsg, type ChatLine, type ClientMsg, type PlayerInfo, type SnapRow } from './protocol';

export type NetStatus = 'connecting' | 'online' | 'offline' | 'full';

export interface NetEvents {
  status(s: NetStatus): void;
  /** joined (again): everyone else who is here, and the recent chat */
  welcome(players: PlayerInfo[], chat: ChatLine[]): void;
  join(p: PlayerInfo): void;
  leave(id: number): void;
  profile(id: number, name: string, look: Look): void;
  /** `receivedAt` is performance.now() when the frame arrived */
  snap(rows: SnapRow[], receivedAt: number): void;
  chat(line: ChatLine): void;
  /** the last chat line was refused for being too fast */
  rateLimited(): void;
  /** the connection dropped: every remote player is gone until the next welcome */
  reset(): void;
}

/** the server's state rate limit is 20/s; 10/s is plenty for walking */
const SEND_INTERVAL = 100;
/** do not resend a pose that moved less than this (m / rad) */
const MOVE_EPS = 0.01;
const TURN_EPS = 0.01;
const BACKOFF_MIN = 1000;
const BACKOFF_MAX = 15000;
/** when the server is full, wait this long before trying again (ms) */
const FULL_RETRY = 30000;

/**
 * Where the multiplayer server is, first match wins:
 *   1. `data-ws` on #stage in index.html     (change a deployed page without rebuilding)
 *   2. VITE_WS_URL at build time             (`VITE_WS_URL=wss://server.tinymap.live/ws npm run build`, or .env.production)
 *   3. ws(s)://<page host>/ws                (the page and the server on the same host, and the Vite dev proxy)
 * The server must accept the page's origin: see ALLOWED_ORIGINS in server/README.md.
 */
export function socketUrl(stage: HTMLElement): string {
  const configured = stage.dataset.ws || import.meta.env.VITE_WS_URL;
  if (configured) {
    if (!/^wss?:\/\//.test(configured)) console.error(`multiplayer socket URL must start with ws:// or wss://, got "${configured}"`);
    else if (location.protocol === 'https:' && configured.startsWith('ws://')) console.error(`an https page cannot open ${configured}: use wss://`);
    return configured;
  }
  return `${location.protocol === 'https:' ? 'wss' : 'ws'}://${location.host}/ws`;
}

const round2 = (v: number): number => Math.round(v * 100) / 100;

export class NetClient {
  /** our id on the server while online */
  selfId: number | null = null;
  private ws: WebSocket | null = null;
  private profile: Profile | null = null;
  private started = false;
  private online = false;
  private full = false;
  private backoff = BACKOFF_MIN;
  private retryTimer = 0;
  private lastSend = -1e9;
  private readonly lastPose = [NaN, NaN, NaN, NaN];
  /** the guest account (src/net/auth.ts); null until the server has given one */
  private token: string | null = loadToken();
  /** the HTTP API next to the socket, null if the socket URL is unusable */
  private readonly api: string | null;
  /** asking for a guest token before opening the socket */
  private opening = false;

  constructor(private readonly url: string, private readonly on: NetEvents) {
    this.api = apiUrl(url);
    // back on the network: try now rather than at the end of the backoff
    window.addEventListener('online', () => {
      if (this.started && !this.ws) this.connectNow();
    });
  }

  get isOnline(): boolean {
    return this.online;
  }

  /** Connect (once) and stay connected; later calls just update the profile. */
  start(profile: Profile): void {
    this.profile = profile;
    if (this.started) return;
    this.started = true;
    this.connect();
  }

  setProfile(profile: Profile): void {
    this.profile = profile;
    if (this.online) this.send({ t: 'profile', name: profile.name, look: profile.look });
  }

  /** Call every frame; sends at most every SEND_INTERVAL ms, and only when the pose changed. */
  sendState(x: number, y: number, z: number, h: number, now: number): void {
    if (!this.online || now - this.lastSend < SEND_INTERVAL) return;
    const p = this.lastPose;
    // NaN (nothing sent yet on this connection) never compares as "unchanged"
    const still = Math.abs(x - p[0]) < MOVE_EPS && Math.abs(y - p[1]) < MOVE_EPS && Math.abs(z - p[2]) < MOVE_EPS && Math.abs(wrapAngle(h - p[3])) < TURN_EPS;
    if (still) return;
    p[0] = x;
    p[1] = y;
    p[2] = z;
    p[3] = h;
    this.lastSend = now;
    this.send({ t: 'state', x: round2(x), y: round2(y), z: round2(z), h: Math.round(h * 1000) / 1000 });
  }

  /** false when offline (the line was not sent) */
  sendChat(text: string): boolean {
    if (!this.online) return false;
    this.send({ t: 'chat', text });
    return true;
  }

  private send(m: ClientMsg): void {
    if (this.ws?.readyState === WebSocket.OPEN) this.ws.send(JSON.stringify(m));
  }

  private connectNow(): void {
    window.clearTimeout(this.retryTimer);
    this.connect();
  }

  private connect(): void {
    if (this.ws || this.opening) return;
    this.opening = true;
    this.on.status('connecting');
    void this.ensureToken().then(() => {
      this.opening = false;
      this.open();
    });
  }

  /** Get a guest account on the first connection. Without one (no server, an older server) the game still plays. */
  private async ensureToken(): Promise<void> {
    if (this.token || !this.api) return;
    const token = await fetchGuestToken(this.api);
    if (!token) return;
    this.token = token;
    saveToken(token);
  }

  private open(): void {
    let ws: WebSocket;
    try {
      ws = new WebSocket(this.url);
    } catch (err) {
      // a malformed data-ws URL: nothing to retry against
      console.error('cannot open the game socket', this.url, err);
      this.on.status('offline');
      return;
    }
    this.ws = ws;
    ws.onopen = () => {
      const p = this.profile;
      if (!p) return;
      const hello: ClientMsg = this.token ? { t: 'hello', token: this.token, name: p.name, look: p.look } : { t: 'hello', name: p.name, look: p.look };
      ws.send(JSON.stringify(hello));
    };
    ws.onmessage = (e: MessageEvent) => this.receive(e.data);
    // a failed connect fires error then close; close does the bookkeeping
    ws.onclose = () => this.closed(ws);
  }

  private receive(data: unknown): void {
    const m = parseServerMsg(data);
    if (!m) return;
    switch (m.t) {
      case 'welcome':
        this.online = true;
        this.selfId = m.id;
        this.backoff = BACKOFF_MIN;
        this.lastPose.fill(NaN);
        this.on.welcome(m.players, m.chat);
        this.on.status('online');
        break;
      case 'join':
        if (m.p.id !== this.selfId) this.on.join(m.p);
        break;
      case 'leave':
        this.on.leave(m.id);
        break;
      case 'profile':
        if (m.id !== this.selfId) this.on.profile(m.id, m.name, m.look);
        break;
      case 'snap':
        this.on.snap(m.rows, performance.now());
        break;
      case 'chat':
        this.on.chat(m);
        break;
      case 'error':
        if (m.code === 'full') this.full = true;
        else if (m.code === 'rate') this.on.rateLimited();
        else if (m.code === 'auth') {
          // the server no longer knows this account (its database was reset): forget it, the reconnect gets a new one
          this.token = null;
          clearToken();
        } else console.error('game server refused the connection:', m.code);
        break;
    }
  }

  private closed(ws: WebSocket): void {
    if (ws !== this.ws) return;
    this.ws = null;
    if (this.online) this.on.reset();
    this.online = false;
    this.selfId = null;
    const wait = this.full ? FULL_RETRY : this.backoff;
    this.on.status(this.full ? 'full' : 'offline');
    this.full = false;
    this.backoff = Math.min(BACKOFF_MAX, this.backoff * 2);
    // jitter, so a restarted server is not hit by every client in the same millisecond
    this.retryTimer = window.setTimeout(() => this.connect(), wait * (0.8 + Math.random() * 0.4));
  }
}
