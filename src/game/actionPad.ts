// The buttons over the bottom right corner, above the phone: jump (touch screens; Space on a keyboard), the
// action at hand ("Lên xe" next to your bike, "Xuống xe" on it; E), and the emotes (1-9), which open as a row.
import { el } from './hud';
import type { Emote } from './motion';

export interface ActionPadEvents {
  jump(): void;
  use(): void;
  emote(code: number): void;
  /** a button was pressed: give the keyboard back to the game */
  done(): void;
}

export class ActionPad {
  private readonly useButton = el<HTMLButtonElement>('act-use');
  private readonly useText = el('act-use-text');
  private readonly emoteButton = el<HTMLButtonElement>('act-emote');
  private readonly menu = el('emote-menu');
  private useLabel: string | null = null;

  constructor(private readonly on: ActionPadEvents) {
    el<HTMLButtonElement>('act-jump').addEventListener('click', () => {
      on.jump();
      on.done();
    });
    this.useButton.addEventListener('click', () => {
      on.use();
      on.done();
    });
    this.emoteButton.addEventListener('click', () => (this.menu.hidden ? this.openMenu() : this.closeMenu()));
  }

  /** The emotes the player may play; keys 1-9 follow this order. */
  setEmotes(list: readonly Emote[]): void {
    this.menu.replaceChildren(...list.map((e, i) => {
      const b = document.createElement('button');
      b.type = 'button';
      b.className = 'emote';
      const key = document.createElement('kbd');
      key.className = 'mouse-only';
      key.textContent = String(i + 1);
      b.append(key, document.createTextNode(e.name.vi));
      b.addEventListener('click', () => {
        this.closeMenu();
        this.on.emote(e.code);
        this.on.done();
      });
      return b;
    }));
    this.emoteButton.hidden = list.length === 0;
  }

  /** The action at hand, null for none (the button hides). */
  setUse(label: string | null): void {
    if (label === this.useLabel) return;
    this.useLabel = label;
    this.useButton.hidden = label === null;
    if (label !== null) this.useText.textContent = label;
  }

  closeMenu(): void {
    this.menu.hidden = true;
    this.emoteButton.setAttribute('aria-expanded', 'false');
  }

  private openMenu(): void {
    this.menu.hidden = false;
    this.emoteButton.setAttribute('aria-expanded', 'true');
  }
}
