// The phone's apps, one view each, keyed by the ids in server/content/apps.json. A view draws into the phone's
// screen when its app opens; live ones (who is around) also get update() a few times a second. What is there to read
// (the character, the map, quests, achievements, the Cẩm nang) moved to the Sổ tay (src/game/notebook/), which
// draws its sections with the same AppFactory. Text only ever goes in through textContent.
import type * as THREE from 'three';
import { PLACES, placeById, placeTitle, type Place } from '../places';
import type { Profile } from '../profile';
import type { RemotePlayer } from '../remotePlayers';
import { NEARBY_RADIUS, type Channel } from '../../net/protocol';
import { badge } from '../badge';
import type { QuestLog } from '../questLog';
import type { Rides } from '../rides';
import { button, h } from './dom';
import type { Directions } from '../directions';
import type { Mutes } from '../mutes';
import type { Owned } from '../items';
import type { PhoneBook } from '../contacts';
import { calls, messages } from './callApps';
import { crab, crabDriver } from './crabApps';
import { L } from '../i18n/phoneApps';

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
  /** open the archetype cards to change archetype */
  changeArchetype(): void;
  /** the player's quests as the server last told it */
  quests(): QuestLog;
  /** carry another earned title ('' for none) */
  chooseTitle(id: string): void;
  /** Crab Bike trips: the wallet, duty, the trip and the offers */
  rides(): Rides;
  /** asking the way: our ask as a Traveler, the calls as a Citizen */
  directions(): Directions;
  /** players muted here (their chat hidden) */
  mutes(): Mutes;
  /** what the player owns: the phone in hand and the items */
  owned(): Owned;
  /** the numbers and texts the player has */
  book(): PhoneBook;
  /** ring a contact (the conversation opens as a call) */
  call(contact: string): void;
  /** the messages were opened: tell the server every text is read */
  readTexts(): void;
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

/** a destination's name, its id for one this build does not know */
const destName = (id: string): string => {
  const p = placeById(id);
  return p ? placeTitle(p) : id;
};

const chat: AppFactory = (root, ctx) => {
  root.append(h('p', 'app-text', L.channelsAbout(NEARBY_RADIUS)));
  const seg = h('div', 'seg');
  const choices: Array<[Channel, string]> = [['global', L.channelGlobal], ['nearby', L.channelNearby]];
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
  root.append(seg, button(L.writeMessage, () => {
    ctx.close();
    ctx.openChat();
  }, true));
  if (!ctx.online()) root.append(h('p', 'app-note', L.chatOffline));
  // personal mutes: whom this player chose not to hear, and how to bring them back
  const muted = h('section', 'mute-section');
  const mutes = ctx.mutes();
  const drawMuted = (): void => {
    muted.replaceChildren(h('strong', 'app-section', L.muteTitle), h('p', 'app-note', L.muteAbout));
    const names = mutes.list();
    if (names.length === 0) return;
    const ol = h('ol', 'app-list');
    for (const name of names) {
      const li = h('li', 'landmark');
      li.append(h('strong', '', name), button(L.unmute, () => mutes.remove(name)));
      ol.append(li);
    }
    muted.append(ol);
  };
  drawMuted();
  root.append(muted);
  return { dispose: mutes.onChange(drawMuted) };
};

const clock = (at: number): string => {
  const mins = Math.round((Date.now() - at) / 60000);
  if (mins < 1) return L.justNow;
  if (mins < 60) return L.minutesAgo(mins);
  return new Date(at).toLocaleTimeString(L.timeLocale, { hour: '2-digit', minute: '2-digit' });
};

const notices: AppFactory = (root, ctx) => {
  const list = ctx.notices();
  if (list.length === 0) {
    root.append(h('p', 'app-note', L.noNotices));
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

/** "2:41" */
function clockLeft(ms: number): string {
  const s = Math.max(0, Math.ceil(ms / 1000));
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`;
}

/** Asking a Citizen online the way to a landmark (D18): the landmarks to pick, or the ask waiting. */
function askSection(ctx: AppContext): { root: HTMLElement; update(): void; dispose(): void } {
  const directions = ctx.directions();
  const root = h('section', 'ask-section');
  const status = h('p', 'app-note');
  const draw = (): void => {
    root.replaceChildren(h('strong', 'app-section', L.askTitle));
    const a = directions.ask;
    if (a) {
      root.append(h('p', 'app-text', L.asking(destName(a.dest))), status, button(L.askCancel, () => directions.cancel()));
      update();
      return;
    }
    root.append(h('p', 'app-note', directions.fallback ? L.askFallback : L.askPick));
    const grid = h('div', 'ask-places');
    for (const l of LANDMARKS) grid.append(button(placeTitle(l), () => directions.askWay(l.id)));
    root.append(grid);
  };
  const update = (): void => {
    const a = directions.ask;
    if (!a) return;
    const text = L.askLeft(clockLeft(a.until - ctx.rides().now()));
    if (status.textContent !== text) status.textContent = text;
  };
  draw();
  return { root, update, dispose: directions.onChange(draw) };
}

/** Hỏi đường: asking a Citizen online the way (D18); the Cẩm nang itself is in the Sổ tay. */
const guide: AppFactory = (root, ctx) => {
  const ask = askSection(ctx);
  root.append(ask.root);
  return { update: ask.update, dispose: ask.dispose };
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
      ? L.communityOffline
      : people.length === 0
        ? L.communityEmpty
        : L.communityCount(people.length);
    ol.replaceChildren(...people.map(({ r, d }) => {
      const li = h('li', 'neighbour');
      li.append(h('strong', '', r.name));
      const b = badge(r.arch);
      if (b) li.append(b);
      li.append(h('small', '', Number.isFinite(d) ? `${Math.round(d)} m` : L.farAway));
      return li;
    }));
  };
  draw();
  return { update: draw };
};

export const APP_VIEWS: Readonly<Record<string, AppFactory>> = {
  calls,
  messages,
  chat,
  notices,
  guide,
  community,
  crab,
  crab_driver: crabDriver,
};
