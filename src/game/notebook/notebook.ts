// The Sổ tay: the game's own menu (docs/design/10-story.md, section 6), open from the first minute, with no phone and
// no money, over the map as well as in play. Everything there is to read is here: the Cẩm nang and its pages, the
// map, quests, achievements and stamps, the character. The phone keeps what a real phone is for. Sections draw with
// the phone's AppFactory (sections.ts); the Cẩm nang is guideBook.ts.
import { el } from '../hud';
import { onLangChange } from '../i18n';
import { L } from '../i18n/notebook';
import { bindHost } from '../phone/phone';
import type { AppContext, AppFactory, AppView, PhoneHost } from '../phone/apps';
import { icon } from '../phone/icons';
import { guideFor, ReadPages } from '../guide';
import type { Place } from '../places';
import { achievements, map, profile, quests } from './sections';
import { guideList, placePage, type GuideNav } from './guideBook';

export type NotebookTab = 'guide' | 'map' | 'quests' | 'achievements' | 'profile';

const TABS: ReadonlyArray<{ id: NotebookTab; icon: string; name: () => string }> = [
  { id: 'guide', icon: 'book', name: () => L.tabGuide },
  { id: 'map', icon: 'map', name: () => L.tabMap },
  { id: 'quests', icon: 'flag', name: () => L.tabQuests },
  { id: 'achievements', icon: 'trophy', name: () => L.tabAchievements },
  { id: 'profile', icon: 'profile', name: () => L.tabProfile },
];

const SECTIONS: Readonly<Record<Exclude<NotebookTab, 'guide'>, AppFactory>> = { map, quests, achievements, profile };

/** live sections (the map, a list that ticks off landmarks) redraw this often while open (ms) */
const REFRESH = 250;

/** What the game lends the notebook: the phone's host, and a way to the place a page is about. */
export interface NotebookHost extends PhoneHost {
  /** the button that takes the player to a place, in the current mode; null for none */
  goLabel(place: Place): string | null;
  goTo(place: Place): void;
}

export class Notebook {
  private readonly root = el('notebook');
  private readonly tabs = el('notebook-tabs');
  private readonly view = el('notebook-view');
  private readonly back = el<HTMLButtonElement>('notebook-back');
  private readonly toggle = el<HTMLButtonElement>('notebook-toggle');
  /** the Cẩm nang pages read in this browser */
  readonly read = new ReadPages();
  private tab: NotebookTab = 'guide';
  /** the place whose page is open, or null for the tab itself */
  private page: string | null = null;
  private current: AppView | null = null;
  private timer = 0;
  private readonly ctx: AppContext;
  private readonly nav: GuideNav;
  /** the notebook came out (the phone is put away then) */
  onOpen: () => void = () => {};

  constructor(host: NotebookHost) {
    this.ctx = { ...bindHost(host), notices: () => [], close: () => this.close() };
    this.nav = {
      openPage: (id) => this.openPage(id),
      read: this.read,
      seen: () => host.landmarksSeen(),
      goLabel: (place) => host.goLabel(place),
      goTo: (place) => host.goTo(place),
    };
    this.toggle.addEventListener('click', () => {
      if (this.isOpen) this.close();
      else this.open();
      // Space and Enter are the game's again
      this.toggle.blur();
    });
    el('notebook-close').addEventListener('click', () => this.close());
    this.back.addEventListener('click', () => this.show('guide', null));
    onLangChange(() => {
      this.drawTabs();
      if (this.isOpen) this.draw();
    });
    this.drawTabs();
  }

  get isOpen(): boolean {
    return !this.root.hidden;
  }

  /** Open on a tab (the last one by default). */
  open(tab: NotebookTab = this.tab): void {
    this.show(tab, null);
  }

  /** Open on a place's page in the Cẩm nang; a place without one opens the list. */
  openPage(placeId: string): void {
    this.show('guide', guideFor(placeId) ? placeId : null);
  }

  close(): void {
    if (!this.isOpen) return;
    this.dispose();
    this.root.hidden = true;
    this.toggle.setAttribute('aria-expanded', 'false');
    document.body.classList.remove('is-notebook');
    window.clearInterval(this.timer);
  }

  /** Redraw what is open (after the character changed, say). */
  rerender(): void {
    if (this.isOpen) this.draw();
  }

  private show(tab: NotebookTab, page: string | null): void {
    if (!this.isOpen) {
      this.onOpen();
      this.root.hidden = false;
      this.toggle.setAttribute('aria-expanded', 'true');
      document.body.classList.add('is-notebook');
      window.clearInterval(this.timer);
      this.timer = window.setInterval(() => this.current?.update?.(), REFRESH);
    }
    this.tab = tab;
    this.page = page;
    this.drawTabs();
    this.draw();
    this.view.scrollTop = 0;
  }

  private drawTabs(): void {
    this.tabs.replaceChildren(...TABS.map((t) => {
      const b = document.createElement('button');
      b.type = 'button';
      b.setAttribute('aria-pressed', String(t.id === this.tab));
      const name = document.createElement('span');
      name.textContent = t.name();
      b.append(icon(t.icon), name);
      b.addEventListener('click', () => this.show(t.id, null));
      return b;
    }));
  }

  private draw(): void {
    this.dispose();
    this.view.replaceChildren();
    this.view.dataset.tab = this.tab;
    const page = this.page ? guideFor(this.page) : undefined;
    this.back.hidden = !page;
    if (page) {
      placePage(this.view, page, this.nav);
      this.current = {};
    } else if (this.tab === 'guide') {
      this.current = guideList(this.view, this.nav);
    } else {
      this.current = SECTIONS[this.tab](this.view, this.ctx);
    }
  }

  private dispose(): void {
    this.current?.dispose?.();
    this.current = null;
  }
}
