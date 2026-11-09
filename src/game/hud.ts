// DOM overlay: start menu (with the loader), place card, map stops, tool buttons, landmark toasts, hints.
// The dressing panel has its own controller (dressPanel.ts).
// Which elements show in which mode is CSS: <body data-mode> against each element's data-modes list.
import { placeById, placeNote, placeTitle, type Place } from './places';
import type { JoystickView } from './input';
import type { MapStop } from './mapView';
import type { NetStatus } from '../net/client';
import { onLangChange, tx } from './i18n';
import { L } from './i18n/hud';

export type Mode = 'menu' | 'map' | 'play';

export function el<T extends HTMLElement>(id: string): T {
  const e = document.getElementById(id);
  if (!e) throw new Error(`#${id} missing in index.html`);
  return e as T;
}

const ONLINE_TEXT: Record<NetStatus, (n: number) => string> = {
  online: (n) => (n > 1 ? L.onlineMany(n) : L.onlineAlone),
  connecting: () => L.connecting,
  offline: () => L.offline,
  full: () => L.full,
  replaced: () => L.replaced,
  banned: () => L.banned,
  kicked: () => L.kicked,
};

const EYEBROW: Record<Mode, () => string> = { menu: () => L.eyebrowMenu, map: () => L.eyebrowMap, play: () => L.eyebrowPlay };

/** A stop's name and the small print on its chip / under the place card. */
function stopText(s: MapStop): { name: string; chip: string; note: string } {
  const p = s.place ? placeById(s.place) : undefined;
  if (!p) return { name: L.overview, chip: L.district, note: L.city };
  return { name: placeTitle(p), chip: s.year ?? tx(p.note), note: placeNote(p) };
}

/** what the place card shows: where the player is, a map stop, or nothing in particular */
type CardShows = { kind: 'place'; place: Place | null } | { kind: 'stop'; stop: MapStop };

export class Hud {
  readonly stage = el<HTMLDivElement>('stage');
  readonly joystick: JoystickView = { base: el('joy'), knob: el('knob') };
  readonly inkButton = el<HTMLButtonElement>('ink');
  readonly soundButton = el<HTMLButtonElement>('sound');
  readonly respawnButton = el<HTMLButtonElement>('respawn');
  readonly dressButton = el<HTMLButtonElement>('dress-open');
  readonly toPlayButton = el<HTMLButtonElement>('to-play');
  readonly toMapButton = el<HTMLButtonElement>('to-map');
  readonly modeMapButton = el<HTMLButtonElement>('mode-map');
  readonly modePlayButton = el<HTMLButtonElement>('mode-play');
  private readonly eyebrow = el('where-eyebrow');
  private readonly where = el('where');
  private readonly whereNote = el('wherenote');
  private readonly online = el('online');
  private readonly onlineText = el('online-text');
  private readonly toastEl = el('toast');
  private readonly hint = el('hint-play');
  private readonly chips = el('chips');
  private chipEls: HTMLButtonElement[] = [];
  private toastTimer = 0;
  // what is on screen, to draw again in another language
  private mode: Mode = 'menu';
  private status: () => string = () => L.loading;
  private net: { status: NetStatus; players: number } | null = null;
  private card: CardShows = { kind: 'place', place: null };
  private stops: { list: readonly MapStop[]; onPick: (i: number) => void; selected: number } | null = null;

  constructor() {
    this.redraw();
    onLangChange(() => this.redraw());
  }

  private redraw(): void {
    this.eyebrow.textContent = EYEBROW[this.mode]();
    el('menu-status').textContent = this.status();
    if (this.net) this.onlineText.textContent = ONLINE_TEXT[this.net.status](this.net.players);
    this.drawCard();
    if (this.stops) {
      const { list, onPick, selected } = this.stops;
      this.buildStops(list, onPick);
      this.selectStop(selected, list[selected] ?? null);
    }
  }

  private drawCard(): void {
    if (this.card.kind === 'stop') {
      const t = stopText(this.card.stop);
      this.where.textContent = t.name;
      this.whereNote.textContent = t.note;
      return;
    }
    const p = this.card.place;
    this.where.textContent = p ? placeTitle(p) : L.district;
    this.whereNote.textContent = p ? placeNote(p) : L.city;
  }

  setMode(mode: Mode): void {
    document.body.dataset.mode = mode;
    this.mode = mode;
    this.eyebrow.textContent = EYEBROW[mode]();
    if (mode !== 'play') {
      window.clearTimeout(this.toastTimer);
      this.toastEl.hidden = true;
    }
  }

  progress(loaded: number, total: number): void {
    el('loadbar').style.width = `${Math.min(100, (loaded / Math.max(1, total)) * 100)}%`;
    el('loadmb').textContent = `${(loaded / 1e6).toFixed(1)} / ${(total / 1e6).toFixed(1)} MB`;
  }

  ready(): void {
    el('loadrow').hidden = true;
    this.status = () => L.ready;
    el('menu-status').textContent = this.status();
    this.modeMapButton.disabled = false;
    this.modePlayButton.disabled = false;
    document.body.classList.add('is-ready');
  }

  /** Show why the map cannot open; `message` gives the text in the current language. */
  fail(message: () => string): void {
    this.status = message;
    el('menu-status').textContent = message();
    el('menu-status').classList.add('is-error');
    el('loadrow').hidden = true;
    this.modeMapButton.disabled = true;
    this.modePlayButton.disabled = true;
  }

  /** Multiplayer status under the place name; `players` counts everyone here, this player included. */
  setOnline(status: NetStatus, players: number): void {
    this.online.hidden = false;
    this.online.dataset.status = status;
    this.net = { status, players };
    this.onlineText.textContent = ONLINE_TEXT[status](players);
  }

  setPlace(p: Place | null): void {
    this.card = { kind: 'place', place: p };
    this.drawCard();
  }

  /** One chip per map stop; `onPick` gets the index. */
  buildStops(stops: readonly MapStop[], onPick: (i: number) => void): void {
    this.stops = { list: stops, onPick, selected: this.stops?.selected ?? -1 };
    this.chips.replaceChildren();
    this.chipEls = stops.map((s, i) => {
      const t = stopText(s);
      const b = document.createElement('button');
      b.type = 'button';
      b.className = 'chip';
      b.setAttribute('aria-pressed', 'false');
      const name = document.createElement('span');
      name.textContent = t.name;
      const note = document.createElement('span');
      note.className = 'yr';
      note.textContent = t.chip;
      b.append(name, note);
      b.addEventListener('click', () => onPick(i));
      this.chips.appendChild(b);
      return b;
    });
  }

  /** Mark the selected stop (or none with -1) and show it on the place card. */
  selectStop(i: number, stop: MapStop | null): void {
    this.chipEls.forEach((c, k) => c.setAttribute('aria-pressed', String(k === i)));
    if (this.stops) this.stops.selected = i;
    if (stop) {
      this.card = { kind: 'stop', stop };
      this.drawCard();
    }
  }

  /** A card that pops in and fades: a landmark first seen, a quest completed (text already in the language). */
  toast(p: { name: string; note: string }): void {
    this.toastEl.replaceChildren();
    const name = document.createElement('strong');
    name.textContent = p.name;
    const note = document.createElement('span');
    note.textContent = p.note;
    this.toastEl.append(name, note);
    // re-show from display:none so the stamp animation plays again
    this.toastEl.hidden = true;
    this.toastEl.classList.remove('is-out');
    void this.toastEl.offsetWidth;
    this.toastEl.hidden = false;
    window.clearTimeout(this.toastTimer);
    this.toastTimer = window.setTimeout(() => {
      this.toastEl.classList.add('is-out');
      this.toastTimer = window.setTimeout(() => {
        this.toastEl.hidden = true;
      }, 600);
    }, 3200);
  }

  /** Fade the play hint once the player has found their feet. */
  hideHint(): void {
    this.hint.classList.add('is-out');
    // then out of the layout, so the chat above it drops into the corner
    window.setTimeout(() => {
      this.hint.hidden = true;
    }, 900);
  }
}
