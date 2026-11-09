// The Crab apps (docs/design/08-ride-hailing.md): Crab books a Crab Bike for anyone, Crab Driver puts a driver on
// duty and lists the offers. Both draw what rides.ts holds, and redraw only when what they show changes shape (a
// trip's state, the offers on the list); the parts that tick (a countdown, how far the driver is) are rewritten in
// place a few times a second, so buttons stay under the finger and keep the keyboard focus. A language switch
// redraws the whole app.
import { CUT_PERCENT, DESTINATIONS, OFFER_RADIUS, TIPS, fareTo, placeName, xu, type Rides } from '../rides';
import { distance, placeTitle } from '../places';
import { lang, onLangChange } from '../i18n';
import { L } from '../i18n/crabApps';
import { button, h } from './dom';
import type { AppContext, AppFactory } from './apps';
import type { Ride, RidePerson } from '../../net/rideProtocol';

/** a place this close past its reach still counts as where we are (the place label's own slack is 3 m) */
const HERE_SLACK = 5;
/** star buttons in the rating, and the stars picked before the passenger chooses */
const STARS = [1, 2, 3, 4, 5];
const STARS_DEFAULT = 5;

/** something rewritten in place by update() */
type Live = () => void;

/** what the passenger picked in the rating so far: kept across redraws, for one trip */
interface Choice {
  ride: string;
  stars: number;
  tip: number;
}

function far(d: number | null): string {
  return d === null ? L.farUnseen : d < 4 ? L.farHere : L.farAway(Math.round(d));
}

/** text that follows f() a few times a second */
function live(cls: string, f: () => string, into: Live[]): HTMLElement {
  const e = h('span', cls, f());
  into.push(() => {
    const t = f();
    if (e.textContent !== t) e.textContent = t;
  });
  return e;
}

/** a button the focus comes back to after a redraw (by its key) */
function keyed(b: HTMLButtonElement, key: string): HTMLButtonElement {
  b.dataset.key = key;
  return b;
}

function walletLine(rides: Rides): HTMLElement {
  return h('p', 'crab-wallet', rides.balance === null ? L.walletNone : L.wallet(xu(rides.balance)));
}

function driverCard(p: RidePerson, sub: HTMLElement): HTMLElement {
  const card = h('div', 'driver-card');
  const stars = p.trips > 0 ? L.driverStars(p.stars.toFixed(1), p.trips) : L.newDriver;
  card.append(h('strong', '', p.name), h('span', '', stars), sub);
  return card;
}

/** Whether the phone can do anything about trips now; if not, a note why. */
function unavailable(root: HTMLElement, ctx: AppContext, rides: Rides): boolean {
  if (!ctx.online()) root.append(h('p', 'app-note', L.offline));
  else if (rides.balance === null) root.append(h('p', 'app-note', L.noAccount));
  else return false;
  return true;
}

/** Draw into root when what it shows changes shape; the focused button keeps the focus. */
function redrawer(root: HTMLElement, shape: () => string, draw: () => void): () => void {
  let drawn = '';
  return () => {
    const now = shape();
    if (now === drawn) return;
    drawn = now;
    const focused = document.activeElement instanceof HTMLElement && root.contains(document.activeElement) ? document.activeElement.dataset.key : undefined;
    draw();
    if (focused) root.querySelector<HTMLElement>(`[data-key="${CSS.escape(focused)}"]`)?.focus({ preventScroll: true });
  };
}

/** what a Crab app shows, as a string that changes when it must be redrawn (the language too) */
function shapeOf(ctx: AppContext, rides: Rides): string {
  const r = rides.ride;
  return JSON.stringify([lang(), ctx.online(), rides.balance, rides.duty, rides.rating, r?.id, r?.state, r?.end, r?.driver?.id, [...rides.offers.keys()].sort()]);
}

// ---- Crab: booking

function destinations(root: HTMLElement, ctx: AppContext, rides: Rides, ticks: Live[]): void {
  const me = ctx.self().position;
  root.append(h('p', 'app-text', L.chooseDest));
  const ol = h('ol', 'app-list');
  const here = (id: string): boolean => {
    const p = DESTINATIONS.find((d) => d.id === id);
    return !!p && distance(p, me.x, -me.z) <= p.r + HERE_SLACK;
  };
  const sorted = [...DESTINATIONS].filter((p) => !here(p.id)).sort((a, b) => distance(a, me.x, -me.z) - distance(b, me.x, -me.z));
  for (const p of sorted) {
    const li = h('li', 'crab-dest');
    const info = h('span', 'crab-dest-info');
    info.append(h('strong', '', placeTitle(p)), live('crab-dest-fare', () => {
      const at = ctx.self().position;
      return `${Math.round(distance(p, at.x, -at.z))} m · ${xu(fareTo(p, at.x, at.z))}`;
    }, ticks));
    const book = keyed(button(L.book, () => rides.book(p.id), true), `book:${p.id}`);
    // the fare follows the player: so does whether the wallet covers it
    const afford = (): void => {
      const at = ctx.self().position;
      book.disabled = rides.balance !== null && rides.balance < fareTo(p, at.x, at.z);
    };
    afford();
    ticks.push(afford);
    li.append(info, book);
    ol.append(li);
  }
  root.append(ol);
}

function rating(root: HTMLElement, rides: Rides, r: Ride, choice: Choice): void {
  root.append(h('p', 'app-text', r.end === 'completed' ? L.arrivedPaid(placeName(r.dest), xu(r.fare)) : L.alightedPaid(xu(r.fare))));
  if (!r.driver) return;
  const box = h('section', 'crab-rate');
  box.append(h('strong', 'app-section', L.rateDriver(r.driver.name)));
  const row = h('div', 'crab-stars');
  row.setAttribute('role', 'group');
  row.setAttribute('aria-label', L.starsGroup);
  const starButtons = STARS.map((n) => {
    const b = keyed(button('★', () => {
      choice.stars = n;
      sync();
    }), `star:${n}`);
    b.setAttribute('aria-label', L.stars(n));
    row.append(b);
    return b;
  });
  const tips = h('div', 'seg');
  tips.setAttribute('role', 'group');
  tips.setAttribute('aria-label', L.tipGroup);
  const tipButtons = TIPS.map((t) => {
    const b = keyed(button(t === 0 ? L.noTip : `+${xu(t)}`, () => {
      choice.tip = t;
      sync();
    }), `tip:${t}`);
    tips.append(b);
    return [t, b] as const;
  });
  const send = keyed(button(L.sendRating, () => rides.rate(choice.stars, choice.tip), true), 'rate');
  const sync = (): void => {
    starButtons.forEach((b, i) => b.setAttribute('aria-pressed', String(i < choice.stars)));
    for (const [t, b] of tipButtons) {
      b.setAttribute('aria-pressed', String(t === choice.tip));
      b.disabled = rides.balance !== null && t > rides.balance;
    }
    send.disabled = rides.rating;
    send.textContent = rides.rating ? L.sending : L.sendRating;
  };
  sync();
  const actions = h('div', 'app-actions');
  actions.append(send, keyed(button(L.skip, () => rides.skipRating()), 'skip'));
  box.append(row, tips, actions);
  root.append(box);
}

function booked(root: HTMLElement, rides: Rides, r: Ride, ticks: Live[], choice: Choice): void {
  const dest = placeName(r.dest);
  const left = (): string => L.secondsLeft(Math.max(0, Math.ceil((r.deadline - rides.now()) / 1000)));
  switch (r.state) {
    case 'requested': {
      const card = h('div', 'driver-card');
      card.append(h('strong', '', L.finding), h('span', '', L.toFare(dest, xu(r.fare))), live('', left, ticks));
      root.append(card, keyed(button(L.cancelTrip, () => rides.cancel()), 'cancel'));
      return;
    }
    case 'accepted': {
      const d = r.driver;
      if (d) root.append(driverCard(d, live('', () => L.comingFor(far(rides.howFar(d.id))), ticks)));
      root.append(h('p', 'app-text', L.boardHow(dest, xu(r.fare))),
        keyed(button(L.cancelTrip, () => rides.cancel()), 'cancel'));
      return;
    }
    case 'on_trip': {
      if (r.driver) root.append(driverCard(r.driver, h('span', '', L.carrying(dest))));
      root.append(h('p', 'app-note', L.fullFare));
      return;
    }
    default:
      if (choice.ride !== r.id) Object.assign(choice, { ride: r.id, stars: STARS_DEFAULT, tip: TIPS[0] ?? 0 });
      rating(root, rides, r, choice);
  }
}

export const crab: AppFactory = (root, ctx) => {
  const rides = ctx.rides();
  let ticks: Live[] = [];
  const choice: Choice = { ride: '', stars: STARS_DEFAULT, tip: TIPS[0] ?? 0 };
  const draw = redrawer(root, () => shapeOf(ctx, rides), () => {
    ticks = [];
    root.replaceChildren(walletLine(rides));
    if (unavailable(root, ctx, rides)) return;
    const r = rides.ride;
    if (r?.role === 'rider') root.append(h('p', 'app-note', L.seeDriverApp));
    else if (r) booked(root, rides, r, ticks, choice);
    else destinations(root, ctx, rides, ticks);
  });
  return mount(draw, rides, () => ticks);
};

/** Draw now, redraw on a trip change or a language switch; the view's update runs the ticking parts. */
function mount(draw: () => void, rides: Rides, ticks: () => Live[]): { update(): void; dispose(): void } {
  draw();
  const stopRides = rides.onChange(draw);
  const stopLang = onLangChange(draw);
  return {
    update: () => ticks().forEach((f) => f()),
    dispose: () => {
      stopRides();
      stopLang();
    },
  };
}

// ---- Crab Driver: duty and offers

function driving(root: HTMLElement, rides: Rides, r: Ride, ticks: Live[]): void {
  const card = h('div', 'driver-card');
  const dest = placeName(r.dest);
  if (r.state === 'accepted') {
    card.append(h('strong', '', L.pickUp(r.passenger.name)), h('span', '', L.toFare(dest, xu(r.fare))),
      live('', () => far(rides.howFar(r.passenger.id)), ticks));
    root.append(card, h('p', 'app-text', L.pickupHow),
      keyed(button(L.cancelRide, () => rides.cancel()), 'cancel'));
    return;
  }
  card.append(h('strong', '', L.carryTo(r.passenger.name, dest)), h('span', '', L.yourShare(xu(r.fare), 100 - CUT_PERCENT)));
  root.append(card, h('p', 'app-text', L.dropOffHow));
}

function offers(root: HTMLElement, rides: Rides, ticks: Live[]): void {
  const list = [...rides.offers.values()].sort((a, b) => a.expires - b.expires);
  if (list.length === 0) {
    root.append(h('p', 'app-text', L.waiting(OFFER_RADIUS)));
    return;
  }
  const ol = h('ol', 'app-list');
  for (const o of list) {
    const li = h('li', 'crab-dest');
    const info = h('span', 'crab-dest-info');
    info.append(h('strong', '', `${o.passenger.name} → ${placeName(o.dest)}`), live('crab-dest-fare', () => {
      const secs = Math.max(0, Math.ceil((o.expires - rides.now()) / 1000));
      return L.offerInfo(xu(o.fare), Math.round(rides.howFarTo(o.pickup[0], o.pickup[1])), secs);
    }, ticks));
    li.append(info, keyed(button(L.take, () => rides.accept(o.offer), true), `take:${o.offer}`));
    ol.append(li);
  }
  root.append(ol);
}

export const crabDriver: AppFactory = (root, ctx) => {
  const rides = ctx.rides();
  let ticks: Live[] = [];
  const draw = redrawer(root, () => shapeOf(ctx, rides), () => {
    ticks = [];
    root.replaceChildren(walletLine(rides));
    if (unavailable(root, ctx, rides)) return;
    const r = rides.ride;
    if (r?.role === 'rider') {
      driving(root, rides, r, ticks);
      return;
    }
    if (r) {
      root.append(h('p', 'app-note', L.seeCrabApp));
      return;
    }
    const toggle = keyed(button(rides.duty ? L.dutyOff : L.dutyOn, () => rides.setDuty(!rides.duty), !rides.duty), 'duty');
    toggle.setAttribute('aria-pressed', String(rides.duty));
    root.append(toggle, h('p', 'app-note', L.dutyNote));
    if (rides.duty) offers(root, rides, ticks);
    else root.append(h('p', 'app-text', L.idleTip));
  });
  return mount(draw, rides, () => ticks);
};
