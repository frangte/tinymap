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

type AppFactory = (root: HTMLElement, ctx: AppContext) => AppView;

function h<K extends keyof HTMLElementTagNameMap>(tag: K, cls: string, text?: string): HTMLElementTagNameMap[K] {
  const e = document.createElement(tag);
  e.className = cls;
  if (text !== undefined) e.textContent = text;
  return e;
}

function button(label: string, onClick: () => void, primary = false): HTMLButtonElement {
  const b = h('button', primary ? 'btn-primary' : '', label);
  b.type = 'button';
  b.addEventListener('click', onClick);
  return b;
}

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
  root.append(facts);
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
  return {};
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
  root.append(m.canvas, h('p', 'app-note', 'Chấm màu là người đang ở ngoài phố, theo vai của họ.'));
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
  const seen = ctx.landmarksSeen();
  root.append(h('p', 'app-text', `Đã ghé ${seen.size} / ${LANDMARKS.length} công trình trong chuyến này. Đi tới gần để đánh dấu.`));
  const ol = h('ol', 'app-list');
  for (const l of LANDMARKS) {
    const li = h('li', seen.has(l) ? 'landmark is-seen' : 'landmark');
    li.append(h('strong', '', l.name), h('span', '', l.note));
    ol.append(li);
  }
  root.append(ol);
  return {};
};

const community: AppFactory = (root, ctx) => {
  const intro = h('p', 'app-text');
  const ol = h('ol', 'app-list');
  root.append(intro, ol);
  const draw = (): void => {
    const me = ctx.self().position;
    const people = [...ctx.others()]
      .map((r) => ({ r, d: Math.hypot(r.position.x - me.x, r.position.z - me.z) }))
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
      li.append(h('small', '', `${Math.round(d)} m`));
      return li;
    }));
  };
  draw();
  return { update: draw };
};

const crabDriver: AppFactory = (root) => {
  const card = h('div', 'driver-card');
  card.append(h('strong', '', 'Chưa có cuốc xe'), h('span', '', 'Khách đặt xe qua app Crab sẽ hiện ở đây khi tính năng chở khách ra mắt.'));
  root.append(card, h('p', 'app-text', 'Trong lúc chờ: chạy dọc Lê Lợi, Đồng Khởi và Hàm Nghi cho thuộc đường Quận 1.'));
  return {};
};

export const APP_VIEWS: Readonly<Record<string, AppFactory>> = {
  profile,
  chat,
  map,
  notices,
  guide,
  community,
  crab_driver: crabDriver,
};
