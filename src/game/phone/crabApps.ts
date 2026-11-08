// The Crab apps (docs/design/08-ride-hailing.md): Crab books a Crab Bike for anyone, Crab Driver puts a driver on
// duty and lists the offers. Both draw what rides.ts holds, and redraw only when what they show changes shape (a
// trip's state, the offers on the list); the parts that tick (a countdown, how far the driver is) are rewritten in
// place a few times a second, so buttons stay under the finger and keep the keyboard focus.
import { CUT_PERCENT, DESTINATIONS, OFFER_RADIUS, TIPS, fareTo, placeName, xu, type Rides } from '../rides';
import { distance } from '../places';
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
  return d === null ? 'chưa thấy trên đường' : d < 4 ? 'ở ngay đây' : `cách ${Math.round(d)} m`;
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
  return h('p', 'crab-wallet', rides.balance === null ? 'Ví Crab: chưa có' : `Ví Crab: ${xu(rides.balance)}`);
}

function driverCard(p: RidePerson, sub: HTMLElement): HTMLElement {
  const card = h('div', 'driver-card');
  const stars = p.trips > 0 ? `★ ${p.stars.toFixed(1)} · ${p.trips} cuốc` : 'Tài xế mới';
  card.append(h('strong', '', p.name), h('span', '', stars), sub);
  return card;
}

/** Whether the phone can do anything about trips now; if not, a note why. */
function unavailable(root: HTMLElement, ctx: AppContext, rides: Rides): boolean {
  if (!ctx.online()) root.append(h('p', 'app-note', 'Đang mất kết nối, chưa đặt xe được.'));
  else if (rides.balance === null) root.append(h('p', 'app-note', 'Đặt xe cần một tài khoản có ví, mà lần này bạn đang chơi không có tài khoản. Tải lại trang để thử lại.'));
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

/** what a Crab app shows, as a string that changes when it must be redrawn */
function shapeOf(ctx: AppContext, rides: Rides): string {
  const r = rides.ride;
  return JSON.stringify([ctx.online(), rides.balance, rides.duty, rides.rating, r?.id, r?.state, r?.end, r?.driver?.id, [...rides.offers.keys()].sort()]);
}

// ---- Crab: booking

function destinations(root: HTMLElement, ctx: AppContext, rides: Rides, ticks: Live[]): void {
  const me = ctx.self().position;
  root.append(h('p', 'app-text', 'Chọn nơi muốn tới. Giá tính theo đường chim bay, trả trước từ ví; huỷ trước khi lên xe được hoàn tiền.'));
  const ol = h('ol', 'app-list');
  const here = (id: string): boolean => {
    const p = DESTINATIONS.find((d) => d.id === id);
    return !!p && distance(p, me.x, -me.z) <= p.r + HERE_SLACK;
  };
  const sorted = [...DESTINATIONS].filter((p) => !here(p.id)).sort((a, b) => distance(a, me.x, -me.z) - distance(b, me.x, -me.z));
  for (const p of sorted) {
    const li = h('li', 'crab-dest');
    const info = h('span', 'crab-dest-info');
    info.append(h('strong', '', p.name), live('crab-dest-fare', () => {
      const at = ctx.self().position;
      return `${Math.round(distance(p, at.x, -at.z))} m · ${xu(fareTo(p, at.x, at.z))}`;
    }, ticks));
    const book = keyed(button('Đặt xe', () => rides.book(p.id), true), `book:${p.id}`);
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
  root.append(h('p', 'app-text', r.end === 'completed' ? `Đã tới ${placeName(r.dest)}. Đã trả ${xu(r.fare)}.` : `Bạn đã xuống xe giữa đường. Đã trả ${xu(r.fare)}.`));
  if (!r.driver) return;
  const box = h('section', 'crab-rate');
  box.append(h('strong', 'app-section', `Đánh giá ${r.driver.name}`));
  const row = h('div', 'crab-stars');
  row.setAttribute('role', 'group');
  row.setAttribute('aria-label', 'Số sao');
  const starButtons = STARS.map((n) => {
    const b = keyed(button('★', () => {
      choice.stars = n;
      sync();
    }), `star:${n}`);
    b.setAttribute('aria-label', `${n} sao`);
    row.append(b);
    return b;
  });
  const tips = h('div', 'seg');
  tips.setAttribute('role', 'group');
  tips.setAttribute('aria-label', 'Tiền tip');
  const tipButtons = TIPS.map((t) => {
    const b = keyed(button(t === 0 ? 'Không tip' : `+${xu(t)}`, () => {
      choice.tip = t;
      sync();
    }), `tip:${t}`);
    tips.append(b);
    return [t, b] as const;
  });
  const send = keyed(button('Gửi đánh giá', () => rides.rate(choice.stars, choice.tip), true), 'rate');
  const sync = (): void => {
    starButtons.forEach((b, i) => b.setAttribute('aria-pressed', String(i < choice.stars)));
    for (const [t, b] of tipButtons) {
      b.setAttribute('aria-pressed', String(t === choice.tip));
      b.disabled = rides.balance !== null && t > rides.balance;
    }
    send.disabled = rides.rating;
    send.textContent = rides.rating ? 'Đang gửi…' : 'Gửi đánh giá';
  };
  sync();
  const actions = h('div', 'app-actions');
  actions.append(send, keyed(button('Bỏ qua', () => rides.skipRating()), 'skip'));
  box.append(row, tips, actions);
  root.append(box);
}

function booked(root: HTMLElement, rides: Rides, r: Ride, ticks: Live[], choice: Choice): void {
  const dest = placeName(r.dest);
  const left = (): string => `Còn ${Math.max(0, Math.ceil((r.deadline - rides.now()) / 1000))} giây`;
  switch (r.state) {
    case 'requested': {
      const card = h('div', 'driver-card');
      card.append(h('strong', '', 'Đang tìm tài xế…'), h('span', '', `Tới ${dest} · ${xu(r.fare)}`), live('', left, ticks));
      root.append(card, keyed(button('Huỷ chuyến', () => rides.cancel()), 'cancel'));
      return;
    }
    case 'accepted': {
      const d = r.driver;
      if (d) root.append(driverCard(d, live('', () => `Đang tới đón bạn · ${far(rides.howFar(d.id))}`, ticks)));
      root.append(h('p', 'app-text', `Tới ${dest} · ${xu(r.fare)}. Xe tới và dừng cạnh bạn thì bấm Lên xe Crab (phím E).`),
        keyed(button('Huỷ chuyến', () => rides.cancel()), 'cancel'));
      return;
    }
    case 'on_trip': {
      if (r.driver) root.append(driverCard(r.driver, h('span', '', `Đang chở bạn tới ${dest}`)));
      root.append(h('p', 'app-note', 'Xuống xe giữa đường vẫn tính đủ tiền chuyến.'));
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
    if (r?.role === 'rider') root.append(h('p', 'app-note', 'Bạn đang chở khách: xem trong app Crab Driver.'));
    else if (r) booked(root, rides, r, ticks, choice);
    else destinations(root, ctx, rides, ticks);
  });
  draw();
  return { update: () => ticks.forEach((f) => f()), dispose: rides.onChange(draw) };
};

// ---- Crab Driver: duty and offers

function driving(root: HTMLElement, rides: Rides, r: Ride, ticks: Live[]): void {
  const card = h('div', 'driver-card');
  const dest = placeName(r.dest);
  if (r.state === 'accepted') {
    card.append(h('strong', '', `Đón ${r.passenger.name}`), h('span', '', `Tới ${dest} · ${xu(r.fare)}`),
      live('', () => far(rides.howFar(r.passenger.id)), ticks));
    root.append(card, h('p', 'app-text', 'Chạy tới chỗ khách theo mũi tên, dừng xe cạnh khách để khách lên.'),
      keyed(button('Huỷ cuốc', () => rides.cancel()), 'cancel'));
    return;
  }
  card.append(h('strong', '', `Chở ${r.passenger.name} tới ${dest}`), h('span', '', `${xu(r.fare)}, bạn nhận ${100 - CUT_PERCENT} %`));
  root.append(card, h('p', 'app-text', 'Tới nơi thì dừng xe và bấm Trả khách (phím E). Xuống xe giữa đường là huỷ cuốc, khách được hoàn tiền.'));
}

function offers(root: HTMLElement, rides: Rides, ticks: Live[]): void {
  const list = [...rides.offers.values()].sort((a, b) => a.expires - b.expires);
  if (list.length === 0) {
    root.append(h('p', 'app-text', `Đang chờ khách… Khách đặt xe trong vòng ${OFFER_RADIUS} m sẽ hiện ở đây và trên màn hình.`));
    return;
  }
  const ol = h('ol', 'app-list');
  for (const o of list) {
    const li = h('li', 'crab-dest');
    const info = h('span', 'crab-dest-info');
    info.append(h('strong', '', `${o.passenger.name} → ${placeName(o.dest)}`), live('crab-dest-fare', () => {
      const secs = Math.max(0, Math.ceil((o.expires - rides.now()) / 1000));
      return `${xu(o.fare)} · đón cách ${Math.round(rides.howFarTo(o.pickup[0], o.pickup[1]))} m · ${secs} giây`;
    }, ticks));
    li.append(info, keyed(button('Nhận', () => rides.accept(o.offer), true), `take:${o.offer}`));
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
      root.append(h('p', 'app-note', 'Bạn đang đi Crab: xem trong app Crab.'));
      return;
    }
    const toggle = keyed(button(rides.duty ? 'Tắt nhận cuốc' : 'Bật nhận cuốc', () => rides.setDuty(!rides.duty), !rides.duty), 'duty');
    toggle.setAttribute('aria-pressed', String(rides.duty));
    root.append(toggle, h('p', 'app-note', 'Nhận cuốc khi đang ngồi trên xe của bạn; xuống xe là tắt nhận cuốc.'));
    if (rides.duty) offers(root, rides, ticks);
    else root.append(h('p', 'app-text', 'Trong lúc chờ: chạy dọc Lê Lợi, Đồng Khởi và Hàm Nghi cho thuộc đường Quận 1.'));
  });
  draw();
  return { update: () => ticks.forEach((f) => f()), dispose: rides.onChange(draw) };
};
