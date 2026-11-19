// The calls and messages apps of a basic phone (docs/design/12-phone.md): the Danh bạ, where a number is a button
// that rings it, and the Tin nhắn, the texts NPCs sent. What an NPC says in a call is the dialogue sheet's, the
// same as face to face. Text only ever goes in through textContent.
import { contactName } from '../contacts';
import { lang, tx } from '../i18n';
import { L } from '../i18n/phoneApps';
import type { AppFactory, AppContext } from './apps';
import { button, h } from './dom';

const calls: AppFactory = (root, ctx) => {
  const intro = h('p', 'app-note');
  const ol = h('ol', 'app-list');
  root.append(intro, ol);
  const draw = (): void => {
    const list = ctx.book().contacts();
    intro.textContent = !ctx.online() ? L.callsOffline : list.length === 0 ? L.noContacts : L.callsAbout;
    ol.replaceChildren(...list.map((c) => {
      const li = h('li', 'contact');
      const who = h('div', 'contact-who');
      who.append(h('strong', '', tx(c.name)), h('small', '', tx(c.role)));
      const ring = button(L.call, () => {
        // the phone goes away so the conversation sheet is the whole screen
        ctx.close();
        ctx.call(c.id);
      }, true);
      ring.disabled = !ctx.online();
      ring.setAttribute('aria-label', L.callAria(tx(c.name)));
      li.append(who, ring);
      return li;
    }));
  };
  draw();
  return { dispose: ctx.book().onChange(draw) };
};

const clock = (at: number): string => new Date(at).toLocaleString(L.timeLocale, { day: 'numeric', month: 'numeric', hour: '2-digit', minute: '2-digit' });

const messages: AppFactory = (root, ctx: AppContext) => {
  const book = ctx.book();
  const ol = h('ol', 'app-list');
  root.append(ol);
  // the unread ones stand out on opening; they are read from then on
  const fresh = new Set(book.texts().filter((t) => !t.read).map((t) => t.id));
  const draw = (): void => {
    const texts = [...book.texts()].reverse();
    if (texts.length === 0) {
      ol.replaceChildren(h('p', 'app-note', L.noTexts));
      return;
    }
    ol.replaceChildren(...texts.map((t) => {
      const li = h('li', fresh.has(t.id) ? 'text is-unread' : 'text');
      const say = h('span', 'text-say', t.say.vi);
      say.lang = 'vi';
      li.append(h('strong', '', contactName(t.from)), say);
      if (lang() === 'en' && t.say.en) {
        const gloss = h('span', 'text-gloss', t.say.en);
        gloss.lang = 'en';
        li.append(gloss);
      }
      li.append(h('small', '', clock(t.at)));
      return li;
    }));
  };
  draw();
  if (book.markRead()) ctx.readTexts();
  // a text that arrives while the app is open is read at once
  return {
    dispose: book.onChange(() => {
      for (const t of book.texts()) if (!t.read) fresh.add(t.id);
      draw();
      if (book.markRead()) ctx.readTexts();
    }),
  };
};

export { calls, messages };
