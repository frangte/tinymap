// "Bạn là ai?": the archetype cards. Shown before the first dressing (and once to players from before archetypes),
// and again from the phone's Hồ sơ app to change archetype, with the current one marked and a way back. Hovering
// or focusing a card previews that archetype's starter look on the player.
import { el } from './hud';
import { ARCHETYPES, type Archetype } from './archetypes';
import { onLangChange, tx } from './i18n';
import { L } from './i18n/archetypePicker';

export interface PickerHandlers {
  /** a card is hovered or focused: show its starter look */
  preview(a: Archetype): void;
  /** @param change true when changing archetype, false for the first pick */
  pick(a: Archetype, change: boolean): void;
  /** changing archetype, the player kept the one they have (Huỷ, Esc, or their own card) */
  cancel(): void;
}

interface Card {
  button: HTMLButtonElement;
  badge: HTMLElement;
  now: HTMLElement;
  name: HTMLElement;
  pitch: HTMLElement;
}

export class ArchetypePicker {
  private readonly root = el('picker');
  private readonly title = el('picker-title');
  private readonly lede = el('picker-lede');
  private readonly cancelButton = el<HTMLButtonElement>('picker-cancel');
  private readonly cards = new Map<string, HTMLButtonElement>();
  private readonly texts: Array<{ a: Archetype; card: Card }> = [];
  /** the archetype being changed from, '' for a first pick */
  private current = '';

  constructor(private readonly on: PickerHandlers) {
    const list = el('picker-cards');
    for (const a of ARCHETYPES) {
      const b = document.createElement('button');
      b.type = 'button';
      b.className = 'arch-card';
      const head = document.createElement('span');
      head.className = 'arch-head';
      const badge = document.createElement('span');
      badge.className = 'badge';
      badge.style.background = a.badge.color;
      badge.style.color = a.badge.text;
      const now = document.createElement('span');
      now.className = 'arch-now';
      head.append(badge, now);
      const name = document.createElement('strong');
      const pitch = document.createElement('span');
      pitch.className = 'arch-pitch';
      b.append(head, name, pitch);
      this.texts.push({ a, card: { button: b, badge, now, name, pitch } });
      b.addEventListener('pointerenter', () => on.preview(a));
      b.addEventListener('focus', () => on.preview(a));
      b.addEventListener('click', () => {
        if (a.id === this.current) {
          this.dismiss();
          return;
        }
        const change = this.current !== '';
        this.close();
        on.pick(a, change);
      });
      list.appendChild(b);
      this.cards.set(a.id, b);
    }
    this.cancelButton.addEventListener('click', () => this.dismiss());
    this.label();
    onLangChange(() => this.label());
  }

  /** Every text on the cards and the heading, in the current language. */
  private label(): void {
    for (const { a, card } of this.texts) {
      card.badge.textContent = tx(a.badge.short);
      card.now.textContent = L.current;
      card.name.textContent = tx(a.name);
      card.pitch.textContent = tx(a.pitch);
    }
    this.title.textContent = this.current ? L.titleChange : L.titleFirst;
    this.lede.textContent = this.current ? L.ledeChange : L.ledeFirst;
  }

  get isOpen(): boolean {
    return !this.root.hidden;
  }

  /** Is it open to change archetype (rather than for the first pick, which cannot be skipped)? */
  get isChanging(): boolean {
    return this.isOpen && this.current !== '';
  }

  /** @param current the archetype being changed from; '' (the default) for the first pick */
  open(current = ''): void {
    this.current = current;
    this.label();
    this.cancelButton.hidden = !current;
    for (const [id, b] of this.cards) {
      const mine = id === current;
      b.classList.toggle('is-current', mine);
      if (mine) b.setAttribute('aria-current', 'true');
      else b.removeAttribute('aria-current');
    }
    this.root.hidden = false;
    document.body.classList.add('is-picking');
    // changing, the keyboard starts on the archetype played now (it previews what the player wears)
    const start = this.cards.get(current) ?? this.cards.values().next().value;
    start?.focus({ preventScroll: true });
  }

  close(): void {
    this.root.hidden = true;
    document.body.classList.remove('is-picking');
  }

  /** Changing archetype: keep the one we have. The first pick cannot be dismissed. */
  dismiss(): void {
    if (!this.isChanging) return;
    this.close();
    this.on.cancel();
  }
}
