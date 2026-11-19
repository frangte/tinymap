// Saigon's time on screen: a line on the start menu ("22:15 ở Sài Gòn · 28°C · đang mưa"), so a dark or rainy city
// reads as the real one and not as a fault, and the time on the place card in the map and in play.
import type { WeatherKind } from '../../net/protocol';
import { el } from '../hud';
import { onLangChange } from '../i18n';
import { L } from '../i18n/daylight';
import { minuteOf, saigonHHMM } from './clock';
import type { Phase } from './looks';

export class ClockView {
  private readonly menuLine = el('menu-clock');
  private readonly cardTime = el('where-clock');
  private shown = '';
  private time = '';
  private phase: Phase = 'day';
  private kind: WeatherKind = 'clear';
  private temp: number | null = null;

  constructor() {
    onLangChange(() => this.fill());
  }

  /** Show `date` in Saigon, its time of day and weather (redrawn only when one of them changes). */
  update(date: Date, phase: Phase, kind: WeatherKind, temp: number | null): void {
    const t = temp === null ? null : Math.round(temp);
    const key = `${minuteOf(date)} ${phase} ${kind} ${t}`;
    if (key === this.shown) return;
    this.shown = key;
    this.time = saigonHHMM(date);
    this.phase = phase;
    this.kind = kind;
    this.temp = t;
    this.fill();
  }

  private fill(): void {
    if (!this.time) return;
    const what = this.kind === 'clear' ? L[this.phase] : L[this.kind];
    this.menuLine.textContent = L.line(this.time, what, this.temp);
    this.cardTime.textContent = this.time;
    this.cardTime.title = L.clockTitle;
    this.cardTime.setAttribute('aria-label', L.clockAria(this.time));
  }
}
