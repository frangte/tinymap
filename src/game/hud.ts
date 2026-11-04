// DOM overlay: start menu (with the loader), place card, map stops, tool buttons, landmark toasts, hints.
// Which elements show in which mode is CSS: <body data-mode> against each element's data-modes list.
import type { Place } from './places';
import type { JoystickView } from './input';
import type { MapStop } from './mapView';

export type Mode = 'menu' | 'map' | 'play';

function el<T extends HTMLElement>(id: string): T {
  const e = document.getElementById(id);
  if (!e) throw new Error(`#${id} missing in index.html`);
  return e as T;
}

const EYEBROW: Record<Mode, string> = { menu: 'Sài Gòn Q1', map: 'Sài Gòn Q1 · bản đồ', play: 'Sài Gòn Q1 · đang ở' };

export class Hud {
  readonly stage = el<HTMLDivElement>('stage');
  readonly joystick: JoystickView = { base: el('joy'), knob: el('knob') };
  readonly inkButton = el<HTMLButtonElement>('ink');
  readonly soundButton = el<HTMLButtonElement>('sound');
  readonly respawnButton = el<HTMLButtonElement>('respawn');
  readonly toPlayButton = el<HTMLButtonElement>('to-play');
  readonly toMapButton = el<HTMLButtonElement>('to-map');
  readonly modeMapButton = el<HTMLButtonElement>('mode-map');
  readonly modePlayButton = el<HTMLButtonElement>('mode-play');
  private readonly eyebrow = el('where-eyebrow');
  private readonly where = el('where');
  private readonly whereNote = el('wherenote');
  private readonly toastEl = el('toast');
  private readonly hint = el('hint-play');
  private readonly chips = el('chips');
  private chipEls: HTMLButtonElement[] = [];
  private toastTimer = 0;

  setMode(mode: Mode): void {
    document.body.dataset.mode = mode;
    this.eyebrow.textContent = EYEBROW[mode];
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
    el('menu-status').textContent = 'Chọn cách vào Quận 1:';
    this.modeMapButton.disabled = false;
    this.modePlayButton.disabled = false;
    document.body.classList.add('is-ready');
  }

  fail(message: string): void {
    el('menu-status').textContent = message;
    el('menu-status').classList.add('is-error');
    el('loadrow').hidden = true;
    this.modeMapButton.disabled = true;
    this.modePlayButton.disabled = true;
  }

  setPlace(p: Place | null): void {
    this.where.textContent = p ? p.name : 'Quận 1';
    this.whereNote.textContent = p ? p.note : 'Sài Gòn';
  }

  /** One chip per map stop; `onPick` gets the index. */
  buildStops(stops: readonly MapStop[], onPick: (i: number) => void): void {
    this.chips.replaceChildren();
    this.chipEls = stops.map((s, i) => {
      const b = document.createElement('button');
      b.type = 'button';
      b.className = 'chip';
      b.setAttribute('aria-pressed', 'false');
      const name = document.createElement('span');
      name.textContent = s.name;
      const note = document.createElement('span');
      note.className = 'yr';
      note.textContent = s.note;
      b.append(name, note);
      b.addEventListener('click', () => onPick(i));
      this.chips.appendChild(b);
      return b;
    });
  }

  /** Mark the selected stop (or none with -1) and show it on the place card. */
  selectStop(i: number, stop: MapStop | null): void {
    this.chipEls.forEach((c, k) => c.setAttribute('aria-pressed', String(k === i)));
    if (stop) {
      this.where.textContent = stop.name;
      this.whereNote.textContent = stop.note === 'Quận 1' ? 'Sài Gòn' : `xây năm ${stop.note}`;
    }
  }

  toast(p: Place): void {
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
  }
}
