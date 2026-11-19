// The tip card (docs/design/10-story.md, section 10): a one-line travel tip the first time its situation happens.
// One at a time; a tip that comes while something else has the screen (the dressing panel, the archetype picker,
// the arrival card, the notebook) waits in a queue. It hides by itself after a few seconds, or with ×.
// Text only via textContent.
import { el } from './hud';
import { onLangChange, tx } from './i18n';
import { L } from './i18n/tips';
import { TIPS, TIP_BOOK, triggers, type Tip, type TipEvent } from './tips';

/** how long a tip stays (ms) */
const SHOW_MS = 9000;
/** how often a waiting tip looks again whether the screen is free (ms) */
const RETRY_MS = 600;

export class TipCard {
  private readonly root = el('tip-card');
  private readonly title = el('tip-card-title');
  private readonly body = el('tip-card-body');
  private readonly queue: Tip[] = [];
  private shown: Tip | null = null;
  private timer = 0;

  /** `free`: nothing else has the screen (play mode, no panel, picker, arrival card or notebook open) */
  constructor(private readonly free: () => boolean) {
    el<HTMLButtonElement>('tip-card-close').addEventListener('click', (e) => {
      this.hide();
      (e.currentTarget as HTMLButtonElement).blur();
    });
    onLangChange(() => this.fill());
  }

  /** Something happened in play: queue the tips it shows that this player has not seen. */
  happened(ev: TipEvent): void {
    for (const tip of TIPS) {
      if (!triggers(tip, ev) || TIP_BOOK.has(tip.id) || this.shown === tip || this.queue.includes(tip)) continue;
      this.queue.push(tip);
    }
    this.next();
  }

  private next(): void {
    window.clearTimeout(this.timer);
    if (this.shown) return;
    if (this.queue.length === 0) return;
    if (!this.free()) {
      this.timer = window.setTimeout(() => this.next(), RETRY_MS);
      return;
    }
    const tip = this.queue.shift();
    if (!tip) return;
    this.shown = tip;
    TIP_BOOK.add(tip.id);
    this.fill();
    this.root.hidden = false;
    this.timer = window.setTimeout(() => this.hide(), SHOW_MS);
  }

  private hide(): void {
    window.clearTimeout(this.timer);
    this.shown = null;
    this.root.hidden = true;
    this.next();
  }

  private fill(): void {
    if (!this.shown) return;
    this.title.textContent = `${L.eyebrow} · ${tx(this.shown.title)}`;
    this.body.textContent = tx(this.shown.text);
  }
}
