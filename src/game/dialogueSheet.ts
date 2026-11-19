// The conversation sheet: who is talking, their line and big buttons for the answers (1-9 on a keyboard). NPCs
// speak Vietnamese: in English mode each line and answer has its English under it, in Vietnamese mode not. The
// server has already picked the line for our archetype and the answers on offer; a line without answers leaves
// only goodbye. In a group only the leader's buttons work, the others watch the conversation move. Walking on is
// always allowed: going too far ends the conversation.
// Text only goes in via textContent.
import { el } from './hud';
import type { TalkMode, Text } from '../net/protocol';
import { lang, money, onLangChange, tx, type Bilingual } from './i18n';
import { feeById } from './items';
import { GOODBYE, L } from './i18n/dialogueSheet';
import { L as ITEMS_L } from './i18n/items';

export interface SheetView {
  name: string;
  /** what the NPC does, in both languages */
  role: Bilingual;
  mode: TalkMode;
  lead: boolean;
  /** players sharing the conversation */
  size: number;
  say: Text;
  choices: readonly Bilingual[];
  /** the shop this line opens (items.json), '' for none */
  shop: string;
  /** the fee this line asks for (items.json), '' for none */
  fee: string;
  /** a phone call: the sheet looks like one */
  call: boolean;
}

export interface SheetEvents {
  /** an answer was picked: its index among the choices */
  choose(index: number): void;
  /** the shop button was pressed: the shop's id */
  shop(id: string): void;
  /** the fee button was pressed: the fee's id */
  fee(id: string): void;
  /** closed (×, goodbye, Esc) */
  close(): void;
  /** a button was pressed: give the keyboard back to the game */
  done(): void;
}

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
  /** what the sheet shows, to draw it again in another language */
  private view: SheetView | null = null;

  constructor(private readonly on: SheetEvents) {
    el<HTMLButtonElement>('talk-close').addEventListener('click', () => {
      on.close();
      on.done();
    });
    // the line is always Vietnamese, whatever the page's language
    this.say.lang = 'vi';
    onLangChange(() => {
      if (this.view && this.isOpen) this.draw(this.view);
    });
  }

  get isOpen(): boolean {
    return !this.root.hidden;
  }

  show(v: SheetView): void {
    this.view = v;
    this.draw(v);
    if (this.root.hidden) {
      this.root.hidden = false;
      document.body.classList.add('is-talking');
    }
    // a new line pops in again
    this.say.classList.remove('is-new');
    void this.say.offsetWidth;
    this.say.classList.add('is-new');
  }

  /** Fill the sheet in the language of the moment. */
  private draw(v: SheetView): void {
    this.name.textContent = v.name;
    this.role.textContent = tx(v.role);
    this.root.classList.toggle('is-call', v.call);
    this.group.hidden = v.mode !== 'group' && !v.call;
    this.group.classList.toggle('is-call', v.call);
    this.group.textContent = v.call ? L.call : v.size > 1 ? L.groupOf(v.size) : L.group;
    this.say.textContent = v.say.vi;
    const gloss = lang() === 'en' ? v.say.en : '';
    this.gloss.textContent = gloss;
    this.gloss.hidden = gloss === '';
    this.final = v.choices.length === 0;
    this.wait.hidden = v.lead || this.final;
    this.buttons = this.final ? [this.button(0, GOODBYE, true)] : v.choices.map((c, i) => this.button(i, c, v.lead));
    this.choices.replaceChildren(...this.buttons);
    // a line that names a shop also offers its wares, above the answers
    if (v.shop !== '') this.choices.prepend(this.shopButton(v.shop));
    if (v.fee !== '') this.choices.prepend(this.feeButton(v.fee));
  }

  hide(): void {
    if (this.root.hidden) return;
    this.root.hidden = true;
    document.body.classList.remove('is-talking');
    this.choices.replaceChildren();
    this.buttons = [];
    this.view = null;
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

  private feeButton(id: string): HTMLButtonElement {
    const b = document.createElement('button');
    b.type = 'button';
    b.className = 'talk-choice talk-shop btn-primary';
    const fee = feeById(id);
    b.textContent = ITEMS_L.payFee(money(fee?.amount ?? 0));
    b.addEventListener('click', () => this.on.fee(id));
    return b;
  }

  private shopButton(shop: string): HTMLButtonElement {
    const b = document.createElement('button');
    b.type = 'button';
    b.className = 'talk-choice talk-shop btn-primary';
    b.textContent = ITEMS_L.seeWares;
    b.addEventListener('click', () => this.on.shop(shop));
    return b;
  }

  private button(i: number, t: Bilingual, enabled: boolean): HTMLButtonElement {
    const b = document.createElement('button');
    b.type = 'button';
    b.className = this.final ? 'talk-choice is-bye' : 'talk-choice';
    b.disabled = !enabled;
    const key = document.createElement('kbd');
    key.className = 'mouse-only';
    key.setAttribute('aria-hidden', 'true');
    key.textContent = String(i + 1);
    const vi = document.createElement('span');
    vi.lang = 'vi';
    vi.textContent = t.vi;
    b.append(key, vi);
    if (t.en && lang() === 'en') {
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
