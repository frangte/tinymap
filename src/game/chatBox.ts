// Chat box, bottom left in play mode: the last few lines over the street, and an input that opens with Enter / T
// (or the Chat action on phones). Lines are shown when the server sends them back, our own too, so nothing
// appears twice; join / leave notes are added here from the player events. Text only goes in via textContent.
import { el } from './hud';
import { CHAT_MAX, type Channel, type ChatLine } from '../net/protocol';
import { badge } from './badge';
import { titleName } from './quests';
import { lang, onLangChange } from './i18n';
import { L } from './i18n/chatBox';

export interface ChatHandlers {
  /** @returns false when the line could not be sent (offline) */
  send(text: string, ch: Channel): boolean;
  /** the input took focus: the player must stop walking */
  opened(): void;
  /** the input closed: give the keyboard back to the game */
  closed(): void;
  /** whether a player's lines are hidden (personal mute) */
  muted(id: number, name: string): boolean;
  /** hide a player's lines from now on */
  mute(id: number, name: string): void;
  /** report a player to the moderators; false when it could not be sent */
  report(id: number): boolean;
}

/** lines kept in the log (the closed box shows the last few, see style.css) */
const KEEP = 50;
/** the log fades out after this long without a new line, unless open (ms) */
const QUIET_AFTER = 12000;
const NOTE_FOR = 2600;
/** name colours, picked by player id (style.css: .chat-log b[data-c]) */
const COLOURS = 6;

/** the channel button's label: who will read the line */
const CH_LABEL: Record<Channel, () => string> = { global: () => L.chGlobal, nearby: () => L.chNearby };

/** why the server did not send our last line */
export type ChatRefusal = 'slow' | 'repeat' | 'new_player' | 'muted';

const REFUSED: Record<ChatRefusal, () => string> = {
  slow: () => L.slow,
  repeat: () => L.repeat,
  new_player: () => L.newPlayer,
  muted: () => L.muted,
};

/** What a line says in the player's language: an NPC's English in English mode, otherwise the line as sent. */
export function spoken(l: ChatLine): string {
  return lang() === 'en' && l.en ? l.en : l.text;
}

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
    onLangChange(() => this.relabel());
  }

  get isOpen(): boolean {
    return !this.form.hidden;
  }

  open(): void {
    if (this.isOpen) return;
    if (!this.online) {
      this.flash(L.offline);
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
    this.chButton.dataset.ch = ch;
    this.relabel();
  }

  setOnline(online: boolean): void {
    this.online = online;
    this.input.disabled = !online;
    this.relabel();
    if (!online) this.close(false);
  }

  /** The channel button and the field's placeholder, in the language of the moment. */
  private relabel(): void {
    const label = CH_LABEL[this.ch]();
    this.chButton.textContent = label;
    this.chButton.setAttribute('aria-label', L.channelAria(label));
    this.input.placeholder = this.online ? L.placeholder : L.offline;
  }

  /** The recent chat from a welcome (oldest first). After a reconnect only lines newer than the log are added. */
  history(lines: ChatLine[], selfId: number | null): void {
    for (const l of lines) if (l.at > this.lastAt) this.line(l, selfId);
  }

  line(l: ChatLine, selfId: number | null): void {
    this.lastAt = Math.max(this.lastAt, l.at);
    if (!l.npc && l.id !== selfId && this.on.muted(l.id, l.name)) return;
    const li = document.createElement('li');
    const name = document.createElement('b');
    name.textContent = l.name;
    name.dataset.c = l.npc ? 'npc' : l.id === selfId ? 'self' : String(l.id % COLOURS);
    li.append(name);
    if (!l.npc && l.id !== selfId) {
      // another player's name opens what can be done about them: mute, report
      li.dataset.player = String(l.id);
      name.tabIndex = 0;
      name.setAttribute('role', 'button');
      name.setAttribute('aria-label', L.personAria(l.name));
      name.addEventListener('click', () => this.personMenu(li, l));
      name.addEventListener('keydown', (e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          this.personMenu(li, l);
        }
      });
    }
    if (l.npc) li.classList.add('is-npc');
    const b = l.npc ? null : badge(l.arch);
    if (b) li.append(' ', b);
    const title = titleName(l.title);
    if (title) {
      const t = document.createElement('span');
      t.className = 'chat-title';
      t.textContent = title;
      li.append(' ', t);
    }
    if (l.ch === 'nearby') {
      // only the players around heard it
      li.classList.add('is-near');
      const near = document.createElement('i');
      near.textContent = L.near;
      li.append(' ', near);
    }
    if (l.npc && l.en && lang() === 'en') {
      // an NPC speaks Vietnamese; its English follows in small print
      const said = document.createElement('span');
      said.lang = 'vi';
      said.textContent = l.text;
      const gloss = document.createElement('span');
      gloss.className = 'chat-en';
      gloss.textContent = l.en;
      li.append(' ', said, ' ', gloss);
    } else li.append(document.createTextNode(` ${l.text}`));
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
    this.flash(L.rate);
  }

  /** The server did not send our last line: say why, and give a slowed-down line back to retype. */
  refused(code: ChatRefusal): void {
    if (code === 'slow' && this.isOpen && this.input.value === '') this.input.value = this.lastSent;
    this.flash(REFUSED[code]());
  }

  /** Under a line: mute its player, report them, or close. A second tap on the name closes it. */
  private personMenu(li: HTMLLIElement, l: ChatLine): void {
    const open = li.querySelector('.chat-menu');
    this.log.querySelectorAll('.chat-menu').forEach((m) => m.remove());
    if (open) return;
    const menu = document.createElement('span');
    menu.className = 'chat-menu';
    const item = (label: string, act: () => void): HTMLButtonElement => {
      const b = document.createElement('button');
      b.type = 'button';
      b.textContent = label;
      b.addEventListener('click', (e) => {
        e.stopPropagation();
        menu.remove();
        act();
      });
      return b;
    };
    menu.append(
      item(L.mute(l.name), () => {
        this.on.mute(l.id, l.name);
        // their lines already shown go too
        this.log.querySelectorAll(`li[data-player="${l.id}"]`).forEach((x) => x.remove());
        this.system(L.mutedNote(l.name));
      }),
      item(L.report, () => {
        if (!this.on.report(l.id)) this.flash(L.offline);
      }),
      item(L.cancel, () => undefined),
    );
    li.append(menu);
    this.wake();
  }

  private submit(): void {
    const text = this.input.value.replace(/\s+/g, ' ').trim();
    if (text === '') {
      this.close();
      return;
    }
    if (!this.on.send(text, this.ch)) {
      this.flash(L.offline);
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
