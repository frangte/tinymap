// The language switch: "Tiếng Việt | English" on the start menu, and a tool button in the map and in play that
// names the other language in that language ("English" while the game is in Vietnamese), so a player who landed in
// the wrong one can still find it. Both call setLang (i18n/index.ts), which redraws every view.
import { el } from './hud';
import { LANGS, lang, onLangChange, setLang, type Lang } from './i18n';

function isLang(v: string | undefined): v is Lang {
  return LANGS.some((l) => l === v);
}

export class LangSwitch {
  private readonly menuButtons = [...el('lang-seg').querySelectorAll<HTMLButtonElement>('button[data-lang]')];

  constructor() {
    for (const b of this.menuButtons) {
      const to = b.dataset.lang;
      if (isLang(to)) b.addEventListener('click', () => setLang(to));
    }
    el<HTMLButtonElement>('lang').addEventListener('click', () => setLang(lang() === 'vi' ? 'en' : 'vi'));
    this.mark();
    onLangChange(() => this.mark());
  }

  /** Press the menu button of the language shown now. */
  private mark(): void {
    for (const b of this.menuButtons) b.setAttribute('aria-pressed', String(b.dataset.lang === lang()));
  }
}
