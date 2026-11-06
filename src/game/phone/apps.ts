// The phone's apps, one view each, keyed by the ids in server/content/apps.json. A view draws into the phone's
// screen when its app opens; live ones (the map, who is around) also get update() a few times a second.
// Text only ever goes in through textContent.
import type * as THREE from 'three';
import { archetypeById } from '../archetypes';
import { PLACES, type Place } from '../places';
import { BODY_NAMES, HAT_NAMES, outfitStyle } from '../outfits';
import type { Profile } from '../profile';
import type { RemotePlayer } from '../remotePlayers';
import { NEARBY_RADIUS, type Channel } from '../../net/protocol';
import { MiniMap } from './miniMap';
import { badge } from '../badge';
import { countOf, current, isDone, type QuestLog } from '../questLog';
import { STAMPS, countText, titleName } from '../quests';
import { xu, type Rides } from '../rides';
import { button, h } from './dom';
import { crab, crabDriver } from './crabApps';

export interface Notice {
  title: string;
  text: string;
  /** Date.now() when it came */
  at: number;
}

/** What the game lends the phone. */
export interface PhoneHost {
  profile(): Profile;
  online(): boolean;
  /** the player, three.js metres and heading */
  self(): { position: THREE.Vector3; heading: number };
  others(): Iterable<RemotePlayer>;
  landmarksSeen(): ReadonlySet<Place>;
  channel(): Channel;
  setChannel(ch: Channel): void;
  openChat(): void;
  dress(): void;
  switchBody(): void;
  /** the player's quests as the server last told it */
  quests(): QuestLog;
  /** carry another earned title ('' for none) */
  chooseTitle(id: string): void;
  /** Crab Bike trips: the wallet, duty, the trip and the offers */
  rides(): Rides;
}

export interface AppContext extends PhoneHost {
  notices(): readonly Notice[];
  /** put the phone away (before handing over to the chat box or the dressing panel) */
  close(): void;
}

export interface AppView {
  update?(): void;
  dispose?(): void;
}

export type AppFactory = (root: HTMLElement, ctx: AppContext) => AppView;

const LANDMARKS = PLACES.filter((p) => p.kind === 'landmark');

const profile: AppFactory = (root, ctx) => {
  const p = ctx.profile();
  const a = archetypeById(p.archetype);
  const head = h('div', 'app-head');
  head.append(h('strong', 'app-name', p.name));
  const b = badge(p.archetype);
  if (b) head.append(b);
  root.append(head);
  if (a) root.append(h('p', 'app-text', a.pitch.vi));
  const facts = h('dl', 'app-facts');
  const fact = (k: string, v: string): void => facts.append(h('dt', '', k), h('dd', '', v));
  fact('Vai', a ? a.name.vi : 'Chưa chọn');
  fact('Trang phục', outfitStyle(p.look.outfit, p.look.body).name);
  fact('Nón', HAT_NAMES[p.look.hat]);
  const balance = ctx.rides().balance;
  if (balance !== null) fact('Ví', xu(balance));
  root.append(facts);
  const log = ctx.quests();
  let stop: (() => void) | undefined;
  if (log.titles.length > 0) {
    // the title shown under the name, among those earned
    const field = h('label', 'field');
    field.append(h('span', '', 'Danh hiệu'));
    const select = document.createElement('select');
    select.className = 'title-select';
    const none = new Option('Không mang danh hiệu', '');
    select.append(none, ...log.titles.map((id) => new Option(titleName(id) ?? id, id)));
    // the server has the last word: the picker shows what it says is carried, and waits while offline
    const sync = (): void => {
      select.value = log.title;
      select.disabled = !ctx.online();
    };
    sync();
    select.addEventListener('change', () => ctx.chooseTitle(select.value));
    field.append(select);
    root.append(field);
    stop = log.onChange(sync);
  }
  const actions = h('div', 'app-actions');
  const other = BODY_NAMES[p.look.body === 'man' ? 'woman' : 'man'];
  actions.append(
    button('Đổi đồ', () => {
      ctx.close();
      ctx.dress();
    }, true),
    button(`Đổi sang ${other}`, () => ctx.switchBody()),
  );
  root.append(actions);
  return { dispose: stop };
};

const chat: AppFactory = (root, ctx) => {
  root.append(h('p', 'app-text', `Cả phố: ai đang ở Quận 1 cũng đọc được. Gần đây: chỉ những người quanh bạn, trong ${NEARBY_RADIUS} m.`));
  const seg = h('div', 'seg');
  const choices: Array<[Channel, string]> = [['global', 'Cả phố'], ['nearby', 'Gần đây']];
  const sync = (): void => {
    for (const [ch, b] of buttons) b.setAttribute('aria-pressed', String(ctx.channel() === ch));
  };
  const buttons = choices.map(([ch, label]) => {
    const b = button(label, () => {
      ctx.setChannel(ch);
      sync();
    });
    seg.append(b);
    return [ch, b] as const;
  });
  sync();
  root.append(seg, button('Viết tin nhắn', () => {
    ctx.close();
    ctx.openChat();
  }, true));
  if (!ctx.online()) root.append(h('p', 'app-note', 'Đang mất kết nối, chưa chat được.'));
  return {};
};

const map: AppFactory = (root, ctx) => {
  const m = new MiniMap();
  root.append(m.canvas, h('p', 'app-note', 'Chấm màu là người đang ở ngoài phố, theo vai của họ. Hình thoi vàng là người quen để ghé nói chuyện: Cô Ba, Chú Tư, Dì Năm…'));
  const draw = (): void => {
    const s = ctx.self();
    m.draw({ x: s.position.x, z: s.position.z, heading: s.heading }, ctx.others());
  };
  // after layout, so the canvas has its size
  requestAnimationFrame(draw);
  return { update: draw };
};

const clock = (at: number): string => {
  const mins = Math.round((Date.now() - at) / 60000);
  if (mins < 1) return 'vừa xong';
  if (mins < 60) return `${mins} phút trước`;
  return new Date(at).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' });
};

const notices: AppFactory = (root, ctx) => {
  const list = ctx.notices();
  if (list.length === 0) {
    root.append(h('p', 'app-note', 'Chưa có thông báo nào.'));
    return {};
  }
  const ol = h('ol', 'app-list');
  for (const n of list) {
    const li = h('li', 'notice');
    li.append(h('strong', '', n.title), h('span', '', n.text), h('small', '', clock(n.at)));
    ol.append(li);
  }
  root.append(ol);
  return {};
};

const guide: AppFactory = (root, ctx) => {
  const log = ctx.quests();
  // the stamp book: one stamp per quest of the Traveler's questline (redrawn when one is earned)
  const stamps = h('div', 'stamp-section');
  const drawStamps = (): void => {
    stamps.replaceChildren();
    if (!log.known) return;
    const earned = new Set(log.stamps);
    stamps.append(h('strong', 'app-section', `Sổ tem · ${earned.size} / ${STAMPS.length}`));
    const book = h('ol', 'stamp-book');
    book.setAttribute('role', 'list');
    for (const s of STAMPS) {
      const li = h('li', earned.has(s.id) ? 'stamp is-earned' : 'stamp');
      li.append(h('span', '', s.name.vi), h('span', 'sr-only', earned.has(s.id) ? ' (đã có)' : ' (chưa có)'));
      book.append(li);
    }
    stamps.append(book);
  };
  drawStamps();
  root.append(stamps);
  const stop = log.onChange(drawStamps);
  const seen = ctx.landmarksSeen();
  root.append(h('p', 'app-text', `Đã ghé ${seen.size} / ${LANDMARKS.length} công trình trong chuyến này. Đi tới gần để đánh dấu.`));
  const ol = h('ol', 'app-list');
  for (const l of LANDMARKS) {
    const li = h('li', seen.has(l) ? 'landmark is-seen' : 'landmark');
    li.append(h('strong', '', l.name), h('span', '', l.note));
    ol.append(li);
  }
  root.append(ol);
  return { dispose: stop };
};

const community: AppFactory = (root, ctx) => {
  const intro = h('p', 'app-text');
  const ol = h('ol', 'app-list');
  root.append(intro, ol);
  const draw = (): void => {
    const me = ctx.self().position;
    // the server only says where the players near us are: the others are somewhere further off
    const people = [...ctx.others()]
      .map((r) => ({ r, d: r.inView ? Math.hypot(r.position.x - me.x, r.position.z - me.z) : Infinity }))
      .sort((a, b) => a.d - b.d);
    intro.textContent = !ctx.online()
      ? 'Đang mất kết nối, chưa thấy ai.'
      : people.length === 0
        ? 'Chưa có ai khác ngoài phố. Rủ bạn bè vào chơi nhé!'
        : `${people.length} người đang ở ngoài phố, gần nhất trước.`;
    ol.replaceChildren(...people.map(({ r, d }) => {
      const li = h('li', 'neighbour');
      li.append(h('strong', '', r.name));
      const b = badge(r.arch);
      if (b) li.append(b);
      li.append(h('small', '', Number.isFinite(d) ? `${Math.round(d)} m` : 'ở xa'));
      return li;
    }));
  };
  draw();
  return { update: draw };
};

const questsApp: AppFactory = (root, ctx) => {
  const log = ctx.quests();
  let doneOpen = false;
  const draw = (): void => {
    // a redraw (progress ticking while the app is open) keeps the place and what was unfolded
    const scroll = root.scrollTop;
    root.replaceChildren();
    if (!log.known) {
      root.append(h('p', 'app-note', ctx.online()
        ? 'Nhiệm vụ cần lưu tiến độ, mà lần này bạn đang chơi không có tài khoản. Tải lại trang để thử lại.'
        : 'Đang mất kết nối, chưa xem được nhiệm vụ.'));
      return;
    }
    const active = log.active();
    const tracked = log.tracked();
    if (active.length === 0) root.append(h('p', 'app-note', 'Xong hết nhiệm vụ rồi! Cứ đi dạo, sẽ có thêm.'));
    for (const q of active) {
      const now = current(q);
      const card = h('section', q === tracked ? 'quest-card is-tracked' : 'quest-card');
      card.append(h('strong', 'quest-title', q.def.title.vi), h('p', 'app-note', q.def.about.vi));
      const goals = h('ul', 'quest-goals');
      for (const o of q.def.objectives) {
        const li = h('li', isDone(q, o) ? 'is-done' : now.includes(o) ? 'is-now' : '');
        li.append(h('span', '', o.text.vi));
        const c = countText(o, countOf(q, o));
        if (c) li.append(h('small', '', c));
        goals.append(li);
      }
      card.append(goals);
      card.append(q === tracked ? h('small', 'quest-tracking', 'Đang theo dõi trên màn hình') : button('Theo dõi', () => log.track(q.def.id)));
      root.append(card);
    }
    const done = log.done();
    if (done.length > 0) {
      const box = document.createElement('details');
      box.className = 'quest-done';
      box.open = doneOpen;
      box.addEventListener('toggle', () => {
        doneOpen = box.open;
      });
      box.append(h('summary', '', `Đã xong (${done.length})`));
      const ol = h('ol', 'app-list');
      for (const d of done) ol.append(h('li', 'landmark is-seen', d.title.vi));
      box.append(ol);
      root.append(box);
    }
    root.scrollTop = scroll;
  };
  draw();
  return { dispose: log.onChange(draw) };
};

const achievementsApp: AppFactory = (root, ctx) => {
  const log = ctx.quests();
  const draw = (): void => {
    root.replaceChildren();
    if (!log.known) {
      root.append(h('p', 'app-note', ctx.online() ? 'Thành tích cần một tài khoản để lưu.' : 'Đang mất kết nối, chưa xem được thành tích.'));
      return;
    }
    const ol = h('ol', 'app-list');
    for (const { def, state } of log.achievements()) {
      const done = state?.done === true;
      const secret = def.hidden && !done;
      const li = h('li', done ? 'achievement is-done' : 'achievement');
      li.append(h('strong', '', secret ? '???' : def.title.vi), h('span', '', secret ? 'Một thành tích bí mật.' : def.about.vi));
      if (!done && def.count > 1) {
        const n = Math.min(state?.n ?? 0, def.count);
        const bar = h('span', 'meter');
        bar.setAttribute('role', 'progressbar');
        bar.setAttribute('aria-valuemin', '0');
        bar.setAttribute('aria-valuemax', String(def.count));
        bar.setAttribute('aria-valuenow', String(n));
        bar.setAttribute('aria-label', def.title.vi);
        const fill = h('i', '');
        fill.style.width = `${Math.round((100 * n) / def.count)}%`;
        bar.append(fill);
        li.append(bar, h('small', '', countText(def, state?.n ?? 0)));
      }
      ol.append(li);
    }
    root.append(ol);
  };
  draw();
  return { dispose: log.onChange(draw) };
};

export const APP_VIEWS: Readonly<Record<string, AppFactory>> = {
  profile,
  quests: questsApp,
  achievements: achievementsApp,
  chat,
  map,
  notices,
  guide,
  community,
  crab,
  crab_driver: crabDriver,
};
