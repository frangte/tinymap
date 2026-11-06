// Chat box, bottom left in play mode: the last few lines over the street, and an input that opens with Enter / T
// (or the Chat action on phones). Lines are shown when the server sends them back, our own too, so nothing
// appears twice; join / leave notes are added here from the player events. Text only goes in via textContent.
import { el } from './hud';
import { CHAT_MAX, type Channel, type ChatLine } from '../net/protocol';
import { badge } from './badge';

export interface ChatHandlers {
  /** @returns false when the line could not be sent (offline) */
  send(text: string, ch: Channel): boolean;
  /** the input took focus: the player must stop walking */
  opened(): void;
  /** the input closed: give the keyboard back to the game */
  closed(): void;
}

/** lines kept in the log (the closed box shows the last few, see style.css) */
const KEEP = 50;
/** the log fades out after this long without a new line, unless open (ms) */
const QUIET_AFTER = 12000;
const NOTE_FOR = 2600;
/** name colours, picked by player id (style.css: .chat-log b[data-c]) */
const COLOURS = 6;

/** the channel button's label: who will read the line */
const CH_LABEL: Record<Channel, string> = { global: 'Cả phố', nearby: 'Gần đây' };

const NOTE = {
  offline: 'Mất kết nối, chưa chat được…',
  rate: 'Chậm lại chút…',
} as const;

export class ChatBox {
  private readonly root = el('chat');
  private readonly log = el<HTMLOListElement>('chat-log');
  private readonly form = el<HTMLFormElement>('chat-form');
  private readonly input = el<HTMLInputElement>('chat-input');
  private readonly note = el('chat-note');
  private readonly chButton = el<HTMLButtonElement>('chat-ch');
  private ch: Channel = 'global';
  private online = false;
  /** server time of the newest line shown, so a reconnect's history does not repeat lines */
  private lastAt = 0;
  /** what was sent last, put back in the field if the server refuses it for being too fast */
  private lastSent = '';
  private quietTimer = 0;
  private noteTimer = 0;

  constructor(private readonly on: ChatHandlers) {
    this.input.maxLength = CHAT_MAX;
    this.form.addEventListener('submit', (e) => {
      e.preventDefault();
      this.submit();
    });
    this.input.addEventListener('keydown', (e) => {
      if (e.key !== 'Escape') return;
      e.preventDefault();
      e.stopPropagation();
      this.close();
    });
    // clicking back into the game (or anywhere outside the box) closes it
    // tapping "Gửi" must not take focus from the field first (that would close the box before it sends)
    const sendButton = this.form.querySelector('button[type=submit]');
    for (const b of [sendButton, this.chButton]) for (const type of ['pointerdown', 'mousedown']) b?.addEventListener(type, (e) => e.preventDefault());
    this.chButton.addEventListener('click', () => this.setChannel(this.ch === 'global' ? 'nearby' : 'global'));
    this.setChannel('global');
    this.root.addEventListener('focusout', (e) => {
      const next = e.relatedTarget;
      if (!(next instanceof Node && this.root.contains(next))) this.close(false);
    });
    // phones: keep the box above the on-screen keyboard (the layout viewport does not shrink for it on iOS)
    const vv = window.visualViewport;
    if (vv) {
      const fit = (): void => {
        const kb = Math.max(0, Math.round(window.innerHeight - vv.height - vv.offsetTop));
        document.documentElement.style.setProperty('--kb', `${kb}px`);
      };
      vv.addEventListener('resize', fit);
      vv.addEventListener('scroll', fit);
    }
    this.setOnline(false);
  }

  get isOpen(): boolean {
    return !this.form.hidden;
  }

  open(): void {
    if (this.isOpen) return;
    if (!this.online) {
      this.flash(NOTE.offline);
      return;
    }
    this.form.hidden = false;
    this.root.classList.add('is-open');
    document.body.classList.add('is-chatting');
    this.wake();
    this.on.opened();
    this.input.focus({ preventScroll: true });
    this.scrollToEnd();
  }

  /** @param refocus give focus back to the game (false when focus already moved somewhere else) */
  close(refocus = true): void {
    if (!this.isOpen) return;
    this.form.hidden = true;
    this.root.classList.remove('is-open');
    document.body.classList.remove('is-chatting');
    this.input.blur();
    this.wake();
    this.scrollToEnd();
    if (refocus) this.on.closed();
  }

  get channel(): Channel {
    return this.ch;
  }

  /** Where the next line goes: everyone, or only the players around. */
  setChannel(ch: Channel): void {
    this.ch = ch;
    this.chButton.textContent = CH_LABEL[ch];
    this.chButton.dataset.ch = ch;
    this.chButton.setAttribute('aria-label', `Gửi tới: ${CH_LABEL[ch]}. Bấm để đổi`);
  }

  setOnline(online: boolean): void {
    this.online = online;
    this.input.disabled = !online;
    this.input.placeholder = online ? 'Nói gì đó…' : NOTE.offline;
    if (!online) this.close(false);
  }

  /** The recent chat from a welcome (oldest first). After a reconnect only lines newer than the log are added. */
  history(lines: ChatLine[], selfId: number | null): void {
    for (const l of lines) if (l.at > this.lastAt) this.line(l, selfId);
  }

  line(l: ChatLine, selfId: number | null): void {
    this.lastAt = Math.max(this.lastAt, l.at);
    const li = document.createElement('li');
    const name = document.createElement('b');
    name.textContent = l.name;
    name.dataset.c = l.npc ? 'npc' : l.id === selfId ? 'self' : String(l.id % COLOURS);
    li.append(name);
    if (l.npc) li.classList.add('is-npc');
    const b = l.npc ? null : badge(l.arch);
    if (b) li.append(' ', b);
    if (l.ch === 'nearby') {
      // only the players around heard it
      li.classList.add('is-near');
      const near = document.createElement('i');
      near.textContent = 'gần';
      li.append(' ', near);
    }
    li.append(document.createTextNode(` ${l.text}`));
    this.append(li);
  }

  /** a note from the street: someone arrived, left, changed name */
  system(text: string): void {
    const li = document.createElement('li');
    li.className = 'sys';
    li.textContent = text;
    this.append(li);
  }

  rateLimited(): void {
    if (this.isOpen && this.input.value === '') this.input.value = this.lastSent;
    this.flash(NOTE.rate);
  }

  private submit(): void {
    const text = this.input.value.replace(/\s+/g, ' ').trim();
    if (text === '') {
      this.close();
      return;
    }
    if (!this.on.send(text, this.ch)) {
      this.flash(NOTE.offline);
      return;
    }
    this.lastSent = text;
    this.input.value = '';
    // a phone keeps the keyboard up for the reply; a keyboard player gets back to walking
    if (!matchMedia('(pointer: coarse)').matches) this.close();
  }

  private append(li: HTMLLIElement): void {
    this.log.appendChild(li);
    while (this.log.childElementCount > KEEP) this.log.firstElementChild?.remove();
    this.wake();
    this.scrollToEnd();
  }

  private scrollToEnd(): void {
    this.log.scrollTop = this.log.scrollHeight;
  }

  /** show the log again, and fade it after a quiet spell */
  private wake(): void {
    this.root.classList.remove('is-quiet');
    window.clearTimeout(this.quietTimer);
    this.quietTimer = window.setTimeout(() => {
      if (!this.isOpen) this.root.classList.add('is-quiet');
    }, QUIET_AFTER);
  }

  private flash(text: string): void {
    this.note.textContent = text;
    this.note.hidden = false;
    this.wake();
    window.clearTimeout(this.noteTimer);
    this.noteTimer = window.setTimeout(() => {
      this.note.hidden = true;
    }, NOTE_FOR);
  }
}
