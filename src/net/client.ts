// WebSocket client for the multiplayer server: says hello with the player's profile and account token, streams the
// player's pose, and reconnects with backoff. The game keeps working offline (a static host, the server down): it
// just walks alone.
import type { Profile } from '../game/profile';
import { wrapAngle } from '../game/coords';
import type { Look } from '../game/outfits';
import { apiUrl, clearToken, fetchGuestToken, loadToken, saveToken } from './auth';
import { parseServerMsg, type Channel, type ChatLine, type ClientMsg, type PlayerInfo, type Pose, type SnapRow, type VehicleInfo } from './protocol';

/** replaced: the same character is playing in another tab; this one stops trying */
export type NetStatus = 'connecting' | 'online' | 'offline' | 'full' | 'replaced';

export interface NetEvents {
  status(s: NetStatus): void;
  /**
   * joined (again): everyone else who is here, the recent chat, our archetype as the server keeps it, our phone
   * apps (null from an older server), where the server placed us (null from an older one) and the vehicles
   */
  welcome(players: PlayerInfo[], chat: ChatLine[], arch: string, apps: string[] | null, pose: Pose | null, vehicles: VehicleInfo[]): void;
  join(p: PlayerInfo): void;
  leave(id: number): void;
  profile(id: number, name: string, look: Look): void;
  /** `receivedAt` is performance.now() when the frame arrived; `out` are players that left our view */
  snap(rows: SnapRow[], receivedAt: number, out: number[]): void;
  /** our last move was refused: go back to this pose */
  correct(pose: Pose): void;
  /** a vehicle arrived or changed (someone got on or off) */
  vehicle(v: VehicleInfo): void;
  vehicleGone(id: number): void;
  /** getting on a vehicle was refused */
  mountRefused(): void;
  chat(line: ChatLine): void;
  /** the last chat line was refused for being too fast */
  rateLimited(): void;
  /** the connection dropped: every remote player is gone until the next welcome */
  reset(): void;
}

/** the server's state rate limit is 20/s; 10/s is plenty for walking */
const SEND_INTERVAL = 100;
/** do not resend a pose that moved less than this (m / rad), unless the flags changed */
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
  private lastFlags = -1;
  /** corrections applied on this connection; every state echoes it, so the server can tell stale ones */
  private corrections = 0;
  /** the same character joined elsewhere: stop reconnecting */
  private replaced = false;
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
      if (this.started && !this.ws && !this.replaced) this.connectNow();
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

  /** Call every frame; sends at most every SEND_INTERVAL ms, and only when the pose or the flags changed. */
  sendState(x: number, y: number, z: number, h: number, s: number, now: number): void {
    if (!this.online || now - this.lastSend < SEND_INTERVAL) return;
    const p = this.lastPose;
    // NaN (nothing sent yet on this connection) never compares as "unchanged"
    const still = Math.abs(x - p[0]) < MOVE_EPS && Math.abs(y - p[1]) < MOVE_EPS && Math.abs(z - p[2]) < MOVE_EPS && Math.abs(wrapAngle(h - p[3])) < TURN_EPS;
    if (still && s === this.lastFlags) return;
    p[0] = x;
    p[1] = y;
    p[2] = z;
    p[3] = h;
    this.lastFlags = s;
    this.lastSend = now;
    this.send({ t: 'state', x: round2(x), y: round2(y), z: round2(z), h: Math.round(h * 1000) / 1000, s, k: this.corrections });
  }

  /** Tell the server we went back to our start (the respawn button). */
  sendRespawn(x: number, y: number, z: number, h: number): void {
    if (!this.online) return;
    this.lastPose.fill(NaN);
    this.send({ t: 'respawn', x: round2(x), y: round2(y), z: round2(z), h: Math.round(h * 1000) / 1000, k: this.corrections });
  }

  /** Ask to get on a vehicle; the answer is a `vehicle` event with us as the rider, or `mountRefused`. */
  sendMount(vehicle: number): boolean {
    if (!this.online) return false;
    this.send({ t: 'mount', v: vehicle });
    return true;
  }

  sendDismount(): void {
    if (this.online) this.send({ t: 'dismount' });
  }

  /** false when offline (the line was not sent) */
  sendChat(text: string, ch: Channel): boolean {
    if (!this.online) return false;
    this.send({ t: 'chat', ch, text });
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
      const hello: Extract<ClientMsg, { t: 'hello' }> = { t: 'hello', name: p.name, look: p.look };
      if (this.token) hello.token = this.token;
      if (p.archetype) hello.arch = p.archetype;
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
        this.lastFlags = -1;
        this.corrections = 0;
        this.on.welcome(m.players, m.chat, m.arch, m.apps, m.pose, m.vehicles);
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
        this.on.snap(m.rows, performance.now(), m.out);
        break;
      case 'correct':
        this.corrections = m.k;
        this.lastPose.fill(NaN);
        this.on.correct(m.pose);
        break;
      case 'vehicle': {
        const { t: _t, ...v } = m;
        this.on.vehicle(v);
        break;
      }
      case 'vehicle_gone':
        this.on.vehicleGone(m.id);
        break;
      case 'chat':
        this.on.chat(m);
        break;
      case 'error':
        if (m.code === 'full') this.full = true;
        else if (m.code === 'rate') this.on.rateLimited();
        else if (m.code === 'mount') this.on.mountRefused();
        else if (m.code === 'replaced') this.replaced = true;
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
    if (this.replaced) {
      // another tab took over this character; reconnecting would only take it back and start a tug of war
      this.on.status('replaced');
      return;
    }
    const wait = this.full ? FULL_RETRY : this.backoff;
    this.on.status(this.full ? 'full' : 'offline');
    this.full = false;
    this.backoff = Math.min(BACKOFF_MAX, this.backoff * 2);
    // jitter, so a restarted server is not hit by every client in the same millisecond
    this.retryTimer = window.setTimeout(() => this.connect(), wait * (0.8 + Math.random() * 0.4));
  }
}
