// "Bạn là ai?": the archetype cards. Shown before the first dressing (and once to players from before archetypes),
// and again from the phone's Hồ sơ app to change archetype, with the current one marked and a way back. Hovering
// or focusing a card previews that archetype's starter look on the player.
import { el } from './hud';
import { ARCHETYPES, type Archetype } from './archetypes';

export interface PickerHandlers {
  /** a card is hovered or focused: show its starter look */
  preview(a: Archetype): void;
  /** @param change true when changing archetype, false for the first pick */
  pick(a: Archetype, change: boolean): void;
  /** changing archetype, the player kept the one they have (Huỷ, Esc, or their own card) */
  cancel(): void;
}

const LEDE_FIRST = 'Cả Quận 1 chung một con phố, mỗi vai sống ở đó một kiểu. Muốn đổi vai sau này thì vào app Hồ sơ trên điện thoại.';
const LEDE_CHANGE = 'Đổi vai ngay tại chỗ, mặc đồ của vai mới. Nhiệm vụ, danh hiệu và ví vẫn giữ nguyên.';

export class ArchetypePicker {
  private readonly root = el('picker');
  private readonly title = el('picker-title');
  private readonly lede = el('picker-lede');
  private readonly cancelButton = el<HTMLButtonElement>('picker-cancel');
  private readonly cards = new Map<string, HTMLButtonElement>();
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
      badge.textContent = a.badge.short;
      badge.style.background = a.badge.color;
      badge.style.color = a.badge.text;
      const now = document.createElement('span');
      now.className = 'arch-now';
      now.textContent = 'Vai hiện tại';
      head.append(badge, now);
      const name = document.createElement('strong');
      name.textContent = a.name.vi;
      const pitch = document.createElement('span');
      pitch.className = 'arch-pitch';
      pitch.textContent = a.pitch.vi;
      b.append(head, name, pitch);
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
    this.title.textContent = current ? 'Đổi vai' : 'Bạn là ai?';
    this.lede.textContent = current ? LEDE_CHANGE : LEDE_FIRST;
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
