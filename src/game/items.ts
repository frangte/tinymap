// What can be bought and what the player owns (server/content/items.json, docs/design/11-items.md). The server sells
// and keeps the list (welcome.items, then `items` after each purchase); this reads it, works out the phone in hand
// and the capabilities items give. An item is bought once; of several phones the one in hand is the last owned in
// the file's order.
import data from '../../server/content/items.json';
import type { Text } from './archetypes';

export interface Item {
  id: string;
  kind: string;
  shop: string;
  /** whole xu, shown as đồng */
  price: number;
  grants: readonly string[];
  /** capabilities the buyer needs first (the Crab uniform): the server refuses a purchase without them */
  requires: readonly string[];
  /** a bike's vehicle (vehicles.json), '' for anything else */
  vehicle: string;
  name: Text;
  about: Text;
}

export interface Shop {
  id: string;
  /** the NPC's id in npcs.json: the shop is where they stand */
  npc: string;
  name: Text;
  /** a bargain the customers can make: the dialogue signal that starts it and the percent it takes off */
  haggle: { signal: string; percent: number } | null;
}

/** Money paid each time to an NPC for something not kept (the licence test); the dialogue's fee button pays it. */
export interface Fee {
  id: string;
  npc: string;
  /** whole xu */
  amount: number;
  name: Text;
  about: Text;
}

export const ITEMS: readonly Item[] = data.items.map((i) => ({
  id: i.id,
  kind: i.kind,
  shop: i.shop,
  price: i.price,
  grants: i.grants,
  requires: ('requires' in i ? i.requires : undefined) ?? [],
  vehicle: ('vehicle' in i ? i.vehicle : undefined) ?? '',
  name: i.name,
  about: i.about,
}));

export const SHOPS: readonly Shop[] = data.shops.map((s) => ({
  id: s.id,
  npc: s.npc,
  name: s.name,
  haggle: ('haggle' in s ? s.haggle : null) ?? null,
}));

export const FEES: readonly Fee[] = data.fees.map((f) => ({ id: f.id, npc: f.npc, amount: f.amount, name: f.name, about: f.about }));

export function feeById(id: string): Fee | undefined {
  return FEES.find((f) => f.id === id);
}

/** The capability every phone gives (calls, texts, notifications), and the one a smartphone adds (every app). */
export const PHONE_BASIC = 'phone.basic';
export const PHONE_SMART = 'phone.smart';

export function itemById(id: string): Item | undefined {
  return ITEMS.find((i) => i.id === id);
}

export function shopById(id: string): Shop | undefined {
  return SHOPS.find((s) => s.id === id);
}

/** The shop that sells phones (the first one in file order): where the first quest sends the player. */
export function phoneShop(): Shop | undefined {
  const phone = ITEMS.find((i) => i.kind === 'phone');
  return phone ? shopById(phone.shop) : undefined;
}

/** A shop's wares, in file order. */
export function itemsOf(shop: string): Item[] {
  return ITEMS.filter((i) => i.shop === shop);
}

/** Why the server refused a purchase (its result codes); anything else is a lost answer */
export type BuyFailure = 'owned' | 'balance' | 'far' | 'target' | 'locked' | 'other';

export function buyFailure(code: string): BuyFailure {
  return code === 'owned' || code === 'balance' || code === 'far' || code === 'target' || code === 'locked' ? code : 'other';
}

/** What the player owns, as the server last said. */
export class Owned {
  private ids = new Set<string>();
  private readonly listeners = new Set<() => void>();
  /** the server has said (online with an account); until then the game does not gate the phone */
  known = false;

  onChange(fn: () => void): () => void {
    this.listeners.add(fn);
    return () => this.listeners.delete(fn);
  }

  /** Take the server's list (every item owned). Ids this build does not know are kept: they are still owned. */
  set(ids: readonly string[]): void {
    this.known = true;
    this.ids = new Set(ids);
    for (const fn of this.listeners) fn();
  }

  has(id: string): boolean {
    return this.ids.has(id);
  }

  /** Owned items this build knows, in file order. */
  list(): Item[] {
    return ITEMS.filter((i) => this.ids.has(i.id));
  }

  /** The phone in hand: the last owned phone in file order, null for none. */
  phone(): Item | null {
    const phones = this.list().filter((i) => i.kind === 'phone');
    return phones[phones.length - 1] ?? null;
  }

  /** The bike in hand: the last owned bike in file order, null for none. */
  bike(): Item | null {
    const bikes = this.list().filter((i) => i.kind === 'bike');
    return bikes[bikes.length - 1] ?? null;
  }

  /** The capabilities the items give. */
  caps(): Set<string> {
    const out = new Set<string>();
    for (const i of this.list()) for (const g of i.grants) out.add(g);
    return out;
  }

  /** The phone may come out: a phone is owned, or the server never said what is owned (offline, an older server). */
  get phoneUsable(): boolean {
    return !this.known || this.caps().has(PHONE_BASIC);
  }
}

/** What an item opens, from the capabilities it grants: a smartphone every app, any other phone the basics. */
export type Opens = 'smart' | 'basic' | 'bike' | 'uniform' | 'none';

export function opens(item: Item): Opens {
  if (item.kind === 'bike') return 'bike';
  if (item.kind === 'uniform') return 'uniform';
  if (item.grants.includes(PHONE_SMART)) return 'smart';
  if (item.grants.includes(PHONE_BASIC)) return 'basic';
  return 'none';
}
