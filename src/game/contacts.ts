// The phone's numbers and texts (server/content/contacts.json, docs/design/12-phone.md). The server keeps who has
// which number and every text (`phone` on joining, then `contact` and `text` as they come); this holds what it said
// and names the contacts: an NPC's from npcs.json, Má's from contacts.json.
import data from '../../server/content/contacts.json';
import { NPC_DEFS } from './npcs';
import type { Text } from './archetypes';
import type { PhoneText } from '../net/protocol';
import { tx } from './i18n';

export interface ContactDef {
  id: string;
  /** NPCs' names are not translated */
  name: Text;
  role: Text;
}

/** An entry of contacts.json: an NPC's has only its id, a voice with no body has a name and a role too. */
interface ContactRow {
  id: string;
  name?: Text;
  role?: Text;
}

export const CONTACT_DEFS: readonly ContactDef[] = (data.contacts as ContactRow[]).flatMap((c) => {
  const npc = NPC_DEFS.find((n) => n.id === c.id);
  if (npc) return [{ id: c.id, name: { vi: npc.name, en: npc.name }, role: npc.role }];
  return c.name && c.role ? [{ id: c.id, name: c.name, role: c.role }] : [];
});

export function contactDef(id: string): ContactDef | undefined {
  return CONTACT_DEFS.find((c) => c.id === id);
}

/** A contact's name as it shows, its id for one this build does not know. */
export function contactName(id: string): string {
  const c = contactDef(id);
  return c ? tx(c.name) : id;
}

/** The numbers and texts the player has, as the server last said. */
export class PhoneBook {
  private numbers: string[] = [];
  private messages: PhoneText[] = [];
  private readonly listeners = new Set<() => void>();

  onChange(fn: () => void): () => void {
    this.listeners.add(fn);
    return () => this.listeners.delete(fn);
  }

  private changed(): void {
    for (const fn of this.listeners) fn();
  }

  /** Take everything the server says (on joining, and when a phone comes: the texts that waited). */
  set(contacts: readonly string[], texts: readonly PhoneText[]): void {
    this.numbers = [...contacts];
    this.messages = [...texts];
    this.changed();
  }

  /** The numbers we have that this build knows, in the order we got them. */
  contacts(): ContactDef[] {
    return this.numbers.flatMap((id) => contactDef(id) ?? []);
  }

  addContact(id: string): void {
    if (this.numbers.includes(id)) return;
    this.numbers.push(id);
    this.changed();
  }

  addText(t: PhoneText): void {
    if (this.messages.some((m) => m.id === t.id)) return;
    this.messages.push(t);
    this.changed();
  }

  /** The texts, oldest first. */
  texts(): readonly PhoneText[] {
    return this.messages;
  }

  unread(): number {
    return this.messages.filter((m) => !m.read).length;
  }

  /** Mark every text read (the server is told separately); false when there was nothing unread. */
  markRead(): boolean {
    if (this.unread() === 0) return false;
    this.messages = this.messages.map((m) => ({ ...m, read: true }));
    this.changed();
    return true;
  }

  /** The connection dropped: nothing is known until the next welcome. */
  clear(): void {
    this.numbers = [];
    this.messages = [];
    this.changed();
  }
}
