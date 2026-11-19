// Saigon's time on screen: a line on the start menu ("22:15 ở Sài Gòn · thành phố đã lên đèn"), so a dark city at
// night reads as the real one and not as a fault, and the time on the place card in the map and in play.
import { el } from '../hud';
import { onLangChange } from '../i18n';
import { L } from '../i18n/daylight';
import { minuteOf, saigonHHMM } from './clock';
import type { Phase } from './looks';

export class ClockView {
  private readonly menuLine = el('menu-clock');
  private readonly cardTime = el('where-clock');
  private shownMinute = -1;
  private shownPhase: Phase | null = null;
  private time = '';
  private phase: Phase = 'day';

  constructor() {
    onLangChange(() => this.fill());
  }

  /** Show `date` in Saigon (redrawn when the minute or the phase changes). */
  update(date: Date, phase: Phase): void {
    const minute = minuteOf(date);
    if (minute === this.shownMinute && phase === this.shownPhase) return;
    this.shownMinute = minute;
    this.shownPhase = phase;
    this.time = saigonHHMM(date);
    this.phase = phase;
    this.fill();
  }

  private fill(): void {
    if (!this.time) return;
    this.menuLine.textContent = L[this.phase](this.time);
    this.cardTime.textContent = this.time;
    this.cardTime.title = L.clockTitle;
    this.cardTime.setAttribute('aria-label', L.clockAria(this.time));
  }
}
