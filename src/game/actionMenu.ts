// Phones: a round button at the bottom right that opens the play actions (change outfit, switch Nam / Nữ, chat),
// since there is no keyboard for Enter and the top-right tool row is short on room. CSS shows it on touch only.
import { el } from './hud';
import { BODY_NAMES, type Body } from './outfits';

export interface ActionHandlers {
  dress(): void;
  switchBody(): void;
  chat(): void;
}

export class ActionMenu {
  private readonly root = el('actions');
  private readonly list = el('actions-list');
  private readonly toggle = el<HTMLButtonElement>('actions-toggle');
  private readonly bodyButton = el<HTMLButtonElement>('act-body');

  constructor(on: ActionHandlers) {
    this.toggle.addEventListener('click', () => (this.isOpen ? this.close() : this.open()));
    const act = (id: string, fn: () => void): void => {
      el<HTMLButtonElement>(id).addEventListener('click', () => {
        this.close();
        fn();
      });
    };
    act('act-dress', on.dress);
    act('act-body', on.switchBody);
    act('act-chat', on.chat);
    // a tap anywhere else (the street, another card) folds it away
    window.addEventListener('pointerdown', (e) => {
      if (this.isOpen && !(e.target instanceof Node && this.root.contains(e.target))) this.close();
    });
  }

  get isOpen(): boolean {
    return !this.list.hidden;
  }

  /** label the switch with the body it changes to */
  setBody(current: Body): void {
    this.bodyButton.textContent = `Đổi sang ${BODY_NAMES[current === 'man' ? 'woman' : 'man']}`;
  }

  open(): void {
    this.list.hidden = false;
    this.toggle.setAttribute('aria-expanded', 'true');
    this.toggle.setAttribute('aria-label', 'Đóng menu');
  }

  close(): void {
    if (!this.isOpen) return;
    this.list.hidden = true;
    this.toggle.setAttribute('aria-expanded', 'false');
    this.toggle.setAttribute('aria-label', 'Mở menu');
  }
}
