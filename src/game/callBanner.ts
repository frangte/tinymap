// A Traveler asking the way, for a Citizen (D18): who asks, where to, how far they stand, and a bar running down
// until the ask stops waiting. Answering is the action button once close; "Bỏ qua" only hides the call here.
// Text only via textContent.
import data from '../../server/content/directions.json';
import { el } from './hud';
import { landmarkName, type Directions } from './directions';
import type { WayCall } from '../net/wayProtocol';

const WAIT_MS = data.wait_seconds * 1000;

export class CallBanner {
  private readonly root = el('way-call');
  private readonly who = el('way-call-who');
  private readonly where = el('way-call-where');
  private readonly hint = el('way-call-hint');
  private readonly bar = el('way-call-bar');
  private shown: WayCall | null = null;

  constructor(private readonly directions: Directions, private readonly now: () => number) {
    el<HTMLButtonElement>('way-call-skip').addEventListener('click', () => {
      if (this.shown) directions.dismiss(this.shown.id);
    });
    directions.onChange(() => this.refresh());
  }

  /** A few times a second in play mode: the bar runs down, the distance follows the player. */
  update(): void {
    const c = this.shown;
    if (!c) return;
    const left = Math.max(0, c.until - this.now());
    this.bar.style.transform = `scaleX(${Math.min(1, left / WAIT_MS).toFixed(3)})`;
    const d = this.directions.distanceTo(c);
    const where = `Tới ${landmarkName(c.dest)} · ${Number.isFinite(d) ? `cách bạn ${Math.round(d)} m` : 'ở xa'}`;
    if (this.where.textContent !== where) this.where.textContent = where;
    const hint = this.directions.label() ? 'Bấm E để chỉ đường' : 'Lại gần để chỉ đường';
    if (this.hint.textContent !== hint) this.hint.textContent = hint;
  }

  private refresh(): void {
    const c = this.directions.nextCall();
    if (c === this.shown) return;
    this.shown = c;
    this.root.hidden = !c;
    if (!c) return;
    this.who.textContent = `${c.from.name} hỏi đường`;
    this.update();
  }
}
