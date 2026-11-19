// The card that says there is no phone yet (docs/design/11-items.md): nobody arrives in Saigon with one, and buying
// one at Anh Bảy's shop is the first quest. It shows in two ways: at once when the player presses the phone button
// without a phone (`show`), and once per visit on arrival, waiting behind the story card and the other cards the
// way tips do (`arrive`). "Dẫn đường tới tiệm" starts the quest tracker's guide to the shop's NPC; with no such NPC
// (an older map or content) there is no button. Text only via textContent.
import { el } from './hud';
import { onLangChange } from './i18n';
import { L } from './i18n/items';

/** how often a waiting card looks again whether the screen is free (ms) */
const RETRY_MS = 600;

type Reason = 'button' | 'arrival';

export interface PhoneCardDeps {
  /** nothing else has the screen (play mode, no panel, picker, story card, notebook, conversation, tip) */
  free(): boolean;
  /** the shop can be walked to: its NPC is known */
  canGuide(): boolean;
  /** the guide to the shop is on now */
  guiding(): boolean;
  /** start (or stop) guiding to the shop */
  toggleGuide(): void;
}

export class PhoneCard {
  private readonly root = el('phone-card');
  private readonly eyebrow = el('phone-card-eyebrow');
  private readonly title = el('phone-card-title');
  private readonly body = el('phone-card-body');
  private readonly guide = el<HTMLButtonElement>('phone-card-guide');
  private readonly closeButton = el<HTMLButtonElement>('phone-card-close');
  private reason: Reason = 'button';
  /** the arrival card has been shown (or given up on) this visit */
  private told = false;
  private waiting = false;
  private timer = 0;

  constructor(private readonly d: PhoneCardDeps) {
    this.closeButton.addEventListener('click', () => this.close());
    this.guide.addEventListener('click', () => {
      d.toggleGuide();
      this.close();
    });
    onLangChange(() => this.fill());
  }

  get isOpen(): boolean {
    return !this.root.hidden;
  }

  /** The phone button or key was pressed with no phone: say so now. */
  show(): void {
    window.clearTimeout(this.timer);
    this.waiting = false;
    this.reason = 'button';
    this.open();
  }

  /** A visit begins with no phone: say it once, as soon as the screen is free. Later calls this visit do nothing. */
  arrive(): void {
    if (this.told) return;
    this.told = true;
    this.waiting = true;
    this.next();
  }

  /** A phone was bought: nothing left to say. */
  satisfied(): void {
    this.waiting = false;
    window.clearTimeout(this.timer);
    this.close();
  }

  close(): void {
    window.clearTimeout(this.timer);
    if (!this.isOpen) return;
    this.root.hidden = true;
    this.waiting = false;
  }

  private next(): void {
    window.clearTimeout(this.timer);
    if (!this.waiting) return;
    if (!this.d.free()) {
      this.timer = window.setTimeout(() => this.next(), RETRY_MS);
      return;
    }
    this.waiting = false;
    this.reason = 'arrival';
    this.open();
  }

  private open(): void {
    this.fill();
    this.root.hidden = false;
  }

  private fill(): void {
    const arrival = this.reason === 'arrival';
    this.eyebrow.textContent = arrival ? L.firstQuestEyebrow : L.noPhoneEyebrow;
    this.title.textContent = arrival ? L.firstQuestTitle : L.noPhoneTitle;
    this.body.textContent = arrival ? L.firstQuestBody : L.noPhoneBody;
    this.guide.hidden = !this.d.canGuide();
    this.guide.textContent = this.d.guiding() ? L.stopGuiding : L.guideToShop;
    this.closeButton.setAttribute('aria-label', L.cardClose);
  }
}
