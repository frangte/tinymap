// The in-game phone: a home screen of apps and one open app at a time. Which apps a player has comes from the
// server's welcome (worked out from the archetype's capabilities, server/content/apps.json); each is drawn here by
// its id (apps.ts), and ids this build does not know are skipped, so the server can roll an app out first.
import { el } from '../hud';
import { PHONE_APPS } from '../archetypes';
import { APP_VIEWS, type AppContext, type AppView, type Notice, type PhoneHost } from './apps';
import { icon } from './icons';

/** live apps (the map, who is around) redraw this often while open (ms) */
const REFRESH = 250;
const KEEP_NOTICES = 30;
const HOME_TITLE = 'Điện thoại';

export class Phone {
  private readonly root = el('phone');
  private readonly home = el('phone-home');
  private readonly screen = el('phone-view');
  private readonly title = el('phone-title');
  private readonly back = el<HTMLButtonElement>('phone-back');
  private readonly toggle = el<HTMLButtonElement>('phone-toggle');
  private readonly dot = el('phone-dot');
  private readonly notices: Notice[] = [];
  private readonly ctx: AppContext;
  private apps: string[] = [];
  private current: { id: string; view: AppView } | null = null;
  private timer = 0;

  constructor(host: PhoneHost) {
    this.ctx = { ...bind(host), notices: () => this.notices, close: () => this.close() };
    this.toggle.addEventListener('click', () => (this.isOpen ? this.close() : this.open()));
    el('phone-close').addEventListener('click', () => this.close());
    this.back.addEventListener('click', () => this.showHome());
  }

  get isOpen(): boolean {
    return !this.root.hidden;
  }

  /** The apps this player has, in phone order; ids without a view here are skipped. */
  setApps(ids: readonly string[]): void {
    this.apps = ids.filter((id) => id in APP_VIEWS);
    if (this.current && !this.apps.includes(this.current.id)) this.showHome();
    this.renderHome();
  }

  open(app?: string): void {
    this.root.hidden = false;
    this.toggle.setAttribute('aria-expanded', 'true');
    this.dot.hidden = true;
    if (app && this.apps.includes(app)) this.openApp(app);
    else this.showHome();
    window.clearInterval(this.timer);
    this.timer = window.setInterval(() => this.current?.view.update?.(), REFRESH);
  }

  close(): void {
    if (!this.isOpen) return;
    this.closeApp();
    this.root.hidden = true;
    this.toggle.setAttribute('aria-expanded', 'false');
    window.clearInterval(this.timer);
  }

  /** A notice for the Thông báo app, and a dot on the phone button until the phone is next opened. */
  notify(title: string, text: string): void {
    this.notices.unshift({ title, text, at: Date.now() });
    this.notices.length = Math.min(this.notices.length, KEEP_NOTICES);
    if (!this.isOpen) this.dot.hidden = false;
    else if (this.current?.id === 'notices') this.rerender();
  }

  /** Redraw the open app (the profile after a change of clothes, say). */
  rerender(): void {
    if (this.current) this.openApp(this.current.id);
  }

  private renderHome(): void {
    this.home.replaceChildren(...this.apps.map((id) => {
      const def = PHONE_APPS.find((a) => a.id === id);
      const b = document.createElement('button');
      b.type = 'button';
      b.className = 'app-tile';
      const name = document.createElement('span');
      name.textContent = def?.name.vi ?? id;
      b.append(icon(def?.icon ?? ''), name);
      b.addEventListener('click', () => this.openApp(id));
      return b;
    }));
  }

  private openApp(id: string): void {
    this.closeApp();
    this.home.hidden = true;
    this.screen.hidden = false;
    this.back.hidden = false;
    this.title.textContent = PHONE_APPS.find((a) => a.id === id)?.name.vi ?? id;
    this.screen.replaceChildren();
    this.screen.dataset.app = id;
    this.current = { id, view: APP_VIEWS[id](this.screen, this.ctx) };
  }

  private closeApp(): void {
    this.current?.view.dispose?.();
    this.current = null;
  }

  private showHome(): void {
    this.closeApp();
    this.screen.hidden = true;
    this.home.hidden = false;
    this.back.hidden = true;
    this.title.textContent = HOME_TITLE;
    this.home.querySelector<HTMLButtonElement>('.app-tile')?.focus({ preventScroll: true });
  }
}

/** the host's methods bound to it, so spreading them into the app context keeps `this` */
function bind(host: PhoneHost): PhoneHost {
  return {
    profile: () => host.profile(),
    online: () => host.online(),
    self: () => host.self(),
    others: () => host.others(),
    landmarksSeen: () => host.landmarksSeen(),
    channel: () => host.channel(),
    setChannel: (ch) => host.setChannel(ch),
    openChat: () => host.openChat(),
    dress: () => host.dress(),
    switchBody: () => host.switchBody(),
  };
}
