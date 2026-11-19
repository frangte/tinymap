// The shop sheet: opened from a conversation whose line names a shop (dialogueSheet.ts). It lists the shop's items in
// file order with price, what they open and an owned mark, and buys with `buy`; the server decides (far, owned,
// balance) and the sheet tells each answer in a line. The conversation stays under it: closing the sheet (×, Esc,
// the backdrop) shows it again. Text only goes in via textContent.
import { el } from './hud';
import { buyFailure, itemsOf, opens, shopById, type Item, type Owned } from './items';
import { money, onLangChange, tx } from './i18n';
import { L } from './i18n/items';
import type { Result } from '../net/protocol';

export interface ShopDeps {
  /** ask the server to buy; the answer is its result */
  buy(item: string): Promise<Result>;
  owned: Owned;
  /** our balance in xu, null when unknown */
  balance(): number | null;
  /** the sheet closed: give the keyboard back to the game */
  done(): void;
}

const FAILURE_TEXT = {
  owned: () => L.failOwned,
  balance: () => L.failBalance,
  far: () => L.failFar,
  target: () => L.failTarget,
  other: () => L.failOther,
};

export class ShopSheet {
  private readonly root = el('shop');
  private readonly title = el('shop-title');
  private readonly balance = el('shop-balance');
  private readonly note = el('shop-note');
  private readonly list = el('shop-list');
  private readonly status = el('shop-status');
  private readonly closeButton = el<HTMLButtonElement>('shop-close');
  private readonly doneButton = el<HTMLButtonElement>('shop-done');
  private shopId = '';
  /** the item being bought, '' for none: the buttons wait for its answer */
  private buying = '';
  /** the last line told, kept to draw it again in another language */
  private told: (() => string) | null = null;
  /** bumped on every open: an answer that arrives after the sheet was closed says nothing */
  private session = 0;

  constructor(private readonly d: ShopDeps) {
    const close = (): void => {
      this.close();
      d.done();
    };
    this.closeButton.addEventListener('click', close);
    this.doneButton.addEventListener('click', close);
    // a tap on the dim backdrop (not on the card) closes it
    this.root.addEventListener('pointerdown', (e) => {
      if (e.target === this.root) close();
    });
    this.root.addEventListener('keydown', (e) => this.trapTab(e));
    d.owned.onChange(() => this.render());
    onLangChange(() => this.render());
  }

  get isOpen(): boolean {
    return !this.root.hidden;
  }

  /** Show a shop's wares (ids in items.json). A shop this build does not know stays shut. */
  open(shop: string): void {
    if (!shopById(shop)) return;
    this.shopId = shop;
    this.buying = '';
    this.told = null;
    this.session++;
    this.root.hidden = false;
    document.body.classList.add('is-shopping');
    this.render();
    // the first thing to press, or the close button when everything is owned
    const first = this.list.querySelector<HTMLButtonElement>('button:not(:disabled)');
    (first ?? this.doneButton).focus({ preventScroll: true });
  }

  /** Put the sheet away (the conversation's end, leaving play, Esc). */
  close(): void {
    if (!this.isOpen) return;
    this.root.hidden = true;
    document.body.classList.remove('is-shopping');
    this.session++;
    this.buying = '';
  }

  /** The balance changed (a purchase, a fare): redraw it and the buttons that depend on it. */
  refresh(): void {
    if (this.isOpen) this.render();
  }

  private render(): void {
    if (!this.isOpen) return;
    const shop = shopById(this.shopId);
    if (!shop) return;
    this.title.textContent = tx(shop.name);
    this.root.setAttribute('aria-label', L.shopAria);
    this.closeButton.setAttribute('aria-label', L.shopClose);
    this.doneButton.textContent = L.shopDone;
    const balance = this.d.balance();
    this.balance.textContent = balance === null ? L.balanceNone : L.balance(money(balance));
    this.note.textContent = L.gameNote;
    const hand = this.d.owned.phone();
    this.list.replaceChildren(...itemsOf(this.shopId).map((item) => this.row(item, balance, hand?.id === item.id)));
    this.status.textContent = this.told ? this.told() : '';
  }

  private row(item: Item, balance: number | null, inHand: boolean): HTMLElement {
    const li = document.createElement('li');
    li.className = 'shop-item';
    const owned = this.d.owned.has(item.id);
    li.classList.toggle('is-owned', owned);

    const head = document.createElement('div');
    head.className = 'shop-item-head';
    const name = document.createElement('strong');
    name.textContent = tx(item.name);
    const price = document.createElement('span');
    price.className = 'shop-price';
    price.textContent = money(item.price);
    head.append(name, price);

    const about = document.createElement('p');
    about.className = 'shop-about';
    about.textContent = tx(item.about);

    li.append(head, about);
    const kind = opens(item);
    if (kind !== 'none') {
      const o = document.createElement('p');
      o.className = 'shop-opens';
      o.textContent = kind === 'smart' ? L.opensSmart : L.opensBasic;
      li.append(o);
    }

    if (owned) {
      const mark = document.createElement('span');
      mark.className = 'shop-mark';
      mark.textContent = inHand ? `${L.owned} · ${L.inHand}` : L.owned;
      li.append(mark);
      return li;
    }
    const poor = balance !== null && balance < item.price;
    const buy = document.createElement('button');
    buy.type = 'button';
    buy.className = poor ? 'shop-buy' : 'shop-buy btn-primary';
    buy.disabled = poor || this.buying !== '';
    buy.textContent = poor ? L.tooPoor : this.buying === item.id ? L.buying : L.buy;
    buy.addEventListener('click', () => this.buy(item));
    li.append(buy);
    return li;
  }

  private buy(item: Item): void {
    if (this.buying) return;
    this.buying = item.id;
    this.told = null;
    this.render();
    const session = this.session;
    void this.d.buy(item.id).then((r) => {
      if (session !== this.session) return;
      this.buying = '';
      this.told = r.ok ? () => L.bought(tx(item.name)) : FAILURE_TEXT[buyFailure(r.code)];
      this.render();
      // the list was redrawn: keep the keyboard on the sheet
      if (document.activeElement === document.body) this.doneButton.focus({ preventScroll: true });
    });
  }

  /** Tab stays inside the sheet while it is open. */
  private trapTab(e: KeyboardEvent): void {
    if (e.key !== 'Tab') return;
    const items = [...this.root.querySelectorAll<HTMLButtonElement>('button:not(:disabled)')];
    if (items.length === 0) return;
    const first = items[0];
    const last = items[items.length - 1];
    if (e.shiftKey && document.activeElement === first) {
      e.preventDefault();
      last.focus();
    } else if (!e.shiftKey && document.activeElement === last) {
      e.preventDefault();
      first.focus();
    }
  }
}
