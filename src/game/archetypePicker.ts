// "Bạn là ai?": the archetype cards shown once, before the first dressing (and once to players from before
// archetypes). Hovering or focusing a card previews that archetype's starter look on the player. The choice is kept
// for good (the server keeps the first archetype a character picks), which the card says.
import { el } from './hud';
import { ARCHETYPES, type Archetype } from './archetypes';

export interface PickerHandlers {
  /** a card is hovered or focused: show its starter look */
  preview(a: Archetype): void;
  pick(a: Archetype): void;
}

export class ArchetypePicker {
  private readonly root = el('picker');

  constructor(on: PickerHandlers) {
    const cards = el('picker-cards');
    for (const a of ARCHETYPES) {
      const b = document.createElement('button');
      b.type = 'button';
      b.className = 'arch-card';
      const badge = document.createElement('span');
      badge.className = 'badge';
      badge.textContent = a.badge.short;
      badge.style.background = a.badge.color;
      badge.style.color = a.badge.text;
      const name = document.createElement('strong');
      name.textContent = a.name.vi;
      const pitch = document.createElement('span');
      pitch.className = 'arch-pitch';
      pitch.textContent = a.pitch.vi;
      b.append(badge, name, pitch);
      b.addEventListener('pointerenter', () => on.preview(a));
      b.addEventListener('focus', () => on.preview(a));
      b.addEventListener('click', () => {
        this.close();
        on.pick(a);
      });
      cards.appendChild(b);
    }
  }

  get isOpen(): boolean {
    return !this.root.hidden;
  }

  open(): void {
    this.root.hidden = false;
    document.body.classList.add('is-picking');
    this.root.querySelector<HTMLButtonElement>('.arch-card')?.focus({ preventScroll: true });
  }

  close(): void {
    this.root.hidden = true;
    document.body.classList.remove('is-picking');
  }
}
