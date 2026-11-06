// The conversation sheet: who is talking, their line (with the English under it when there is one) and big
// buttons for the answers (1-9 on a keyboard). The server has already picked the line for our archetype and the
// answers on offer; a line without answers leaves only goodbye. In a group only the leader's buttons work, the
// others watch the conversation move. Walking on is always allowed: going too far ends the conversation.
// Text only goes in via textContent.
import { el } from './hud';
import type { TalkMode, Text } from '../net/protocol';

export interface SheetView {
  name: string;
  role: string;
  mode: TalkMode;
  lead: boolean;
  /** players sharing the conversation */
  size: number;
  say: Text;
  choices: readonly Text[];
}

export interface SheetEvents {
  /** an answer was picked: its index among the choices */
  choose(index: number): void;
  /** closed (×, goodbye, Esc) */
  close(): void;
  /** a button was pressed: give the keyboard back to the game */
  done(): void;
}

const GOODBYE = 'Tạm biệt';

export class DialogueSheet {
  private readonly root = el('talk');
  private readonly name = el('talk-name');
  private readonly role = el('talk-role');
  private readonly group = el('talk-group');
  private readonly say = el('talk-say');
  private readonly gloss = el('talk-gloss');
  private readonly wait = el('talk-wait');
  private readonly choices = el('talk-choices');
  private buttons: HTMLButtonElement[] = [];
  private final = false;

  constructor(private readonly on: SheetEvents) {
    el<HTMLButtonElement>('talk-close').addEventListener('click', () => {
      on.close();
      on.done();
    });
  }

  get isOpen(): boolean {
    return !this.root.hidden;
  }

  show(v: SheetView): void {
    this.name.textContent = v.name;
    this.role.textContent = v.role;
    this.group.hidden = v.mode !== 'group';
    this.group.textContent = v.size > 1 ? `Nhóm · ${v.size} người` : 'Nhóm';
    this.say.textContent = v.say.vi;
    this.gloss.textContent = v.say.en;
    this.gloss.hidden = v.say.en === '';
    this.final = v.choices.length === 0;
    this.wait.hidden = v.lead || this.final;
    this.buttons = this.final ? [this.button(0, { vi: GOODBYE, en: '' }, true)] : v.choices.map((c, i) => this.button(i, c, v.lead));
    this.choices.replaceChildren(...this.buttons);
    if (this.root.hidden) {
      this.root.hidden = false;
      document.body.classList.add('is-talking');
    }
    // a new line pops in again
    this.say.classList.remove('is-new');
    void this.say.offsetWidth;
    this.say.classList.add('is-new');
  }

  hide(): void {
    if (this.root.hidden) return;
    this.root.hidden = true;
    document.body.classList.remove('is-talking');
    this.choices.replaceChildren();
    this.buttons = [];
  }

  /** While an answer is on its way, the buttons wait. */
  setBusy(busy: boolean): void {
    this.root.classList.toggle('is-busy', busy);
  }

  /**
   * A digit key (1-9): press that button. False when there is no such button (the key is the game's then).
   */
  press(n: number): boolean {
    const b = this.buttons[n - 1];
    if (!this.isOpen || !b) return false;
    if (!b.disabled) b.click();
    return true;
  }

  private button(i: number, t: Text, enabled: boolean): HTMLButtonElement {
    const b = document.createElement('button');
    b.type = 'button';
    b.className = this.final ? 'talk-choice is-bye' : 'talk-choice';
    b.disabled = !enabled;
    const key = document.createElement('kbd');
    key.className = 'mouse-only';
    key.setAttribute('aria-hidden', 'true');
    key.textContent = String(i + 1);
    const vi = document.createElement('span');
    vi.textContent = t.vi;
    b.append(key, vi);
    if (t.en) {
      const en = document.createElement('small');
      en.lang = 'en';
      en.textContent = t.en;
      b.append(en);
    }
    b.addEventListener('click', () => {
      if (this.root.classList.contains('is-busy')) return;
      if (this.final) this.on.close();
      else this.on.choose(i);
      this.on.done();
    });
    return b;
  }
}
