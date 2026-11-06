// Talking to NPCs. The game offers "Nói chuyện" next to an NPC (E, or the action button); the server decides:
// it starts the conversation (or puts us in the NPC's group) and sends each line, and it ends the conversation
// when we close it, walk away or start another. The game only shows what the server sends: lines, answers and
// what they lead to never live here. Offline the NPCs stand there with nothing to say.
import type { NetClient } from '../net/client';
import type { EndReason, Interaction } from '../net/protocol';
import type { Player } from './player';
import type { Npc, Npcs } from './npcs';
import { DialogueSheet } from './dialogueSheet';

/** why the server would not start a conversation, as the player reads it */
const REFUSED: Record<string, (name: string) => string> = {
  far: (n) => `Lại gần ${n} thêm chút nữa nha.`,
  full: (n) => `Nhóm của ${n} đông quá, chờ chút nha.`,
  mute: (n) => `${n} đang bận.`,
};
const NOT_HEARD = 'Chưa nói chuyện được, thử lại nha.';

export interface TalkDeps {
  net: NetClient;
  npcs: Npcs;
  player: Player;
  /** a line in the chat log (system note) */
  notify(text: string): void;
  /** a sheet button was pressed: give the keyboard back to the game */
  done(): void;
}

export class Talk {
  private readonly sheet: DialogueSheet;
  /** the conversation we are in, as the server last showed it */
  private current: Interaction | null = null;
  /** a greeting (interact) or an answer (choose) on its way */
  private greeting = false;
  private answering = false;
  /** bumped whenever the conversation is dropped: an answer to a request from before is ignored */
  private generation = 0;
  /** conversations we walked out of: a line of theirs already on the way must not open the sheet again */
  private readonly left = new Set<number>();

  constructor(private readonly d: TalkDeps) {
    this.sheet = new DialogueSheet({
      choose: (i) => this.choose(i),
      close: () => this.close(),
      done: () => d.done(),
    });
  }

  get isOpen(): boolean {
    return this.current !== null;
  }

  /** The NPC the action button would talk to: the nearest in reach, on foot, online, not the one we talk to. */
  near(): Npc | null {
    if (this.d.player.riding || !this.d.net.isOnline) return null;
    const n = this.d.npcs.nearest(this.d.player.position);
    return n && n.id !== this.current?.n ? n : null;
  }

  /** The label of the action button, null for nobody to talk to. */
  label(): string | null {
    const n = this.near();
    if (!n) return null;
    return this.greeting ? 'Đang chào…' : `Nói chuyện · ${n.def.name}`;
  }

  /** E / the action button next to an NPC. */
  use(): void {
    const n = this.near();
    if (!n || this.greeting) return;
    this.greeting = true;
    void this.d.net.interact(n.id).then((r) => {
      this.greeting = false;
      // a late yes (past the timeout) still opens the sheet when its line arrives: no false alarm then
      if (r.ok || (r.code === 'timeout' && this.isOpen)) return;
      const say = REFUSED[r.code];
      this.d.notify(say ? say(n.def.name) : NOT_HEARD);
    });
  }

  /** The server shows where a conversation is. */
  interaction(i: Interaction): void {
    if (this.left.has(i.sid)) return;
    const n = this.d.npcs.get(i.n);
    if (!n) {
      // an NPC this game does not know (a newer server): walk out rather than stay in its conversation unseen
      this.left.add(i.sid);
      this.d.net.leaveTalk(i.sid);
      return;
    }
    if (this.current && this.current.sid !== i.sid) this.generation++;
    this.current = i;
    this.answering = false;
    this.sheet.setBusy(false);
    this.sheet.show({ name: n.def.name, role: n.def.role.vi, mode: i.mode, lead: i.lead, size: i.size, say: i.say, choices: i.choices });
  }

  /** The server ended a conversation. */
  ended(sid: number, reason: EndReason): void {
    this.left.delete(sid);
    if (this.current?.sid !== sid) return;
    const n = this.d.npcs.get(this.current.n);
    this.drop();
    if (reason === 'far' && n) this.d.notify(`Bạn đã đi xa ${n.def.name}.`);
  }

  /** A digit key while talking presses that answer; false leaves the key to the game (emotes). */
  press(digit: number): boolean {
    return this.isOpen && this.sheet.press(digit);
  }

  /** Esc, ×, goodbye, or something else taking over the screen: walk out of the conversation. */
  close(): void {
    const c = this.current;
    if (!c) return;
    this.drop();
    this.left.add(c.sid);
    this.d.net.leaveTalk(c.sid);
  }

  /** The connection dropped (or a new welcome): every conversation is over. */
  reset(): void {
    this.drop();
    this.left.clear();
    this.greeting = false;
  }

  private choose(index: number): void {
    const c = this.current;
    if (!c || !c.lead || this.answering) return;
    this.answering = true;
    this.sheet.setBusy(true);
    const gen = this.generation;
    void this.d.net.choose(c.sid, c.step, index).then((r) => {
      if (gen !== this.generation || this.current?.sid !== c.sid) return;
      this.answering = false;
      this.sheet.setBusy(false);
      // a refused answer (the line moved on, or the conversation ended) leaves the sheet as the server last showed it
      if (!r.ok && (r.code === 'timeout' || r.code === 'offline')) this.d.notify(NOT_HEARD);
    });
  }

  private drop(): void {
    this.current = null;
    this.answering = false;
    this.generation++;
    this.sheet.setBusy(false);
    this.sheet.hide();
  }
}
