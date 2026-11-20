import { h } from './dom';
import type { Model } from './model';
import type { Text } from './types';

/** Form widgets of the editor. Each commits on `change` (leaving the field), so a typing session is one undo step. */

export type Pick = 'flags' | 'quests' | 'signals' | 'emotes' | 'shops' | 'fees' | 'npcs' | 'places' | 'caps' | 'items' | 'itemkinds' | 'vehicles' | 'nodes' | 'variants' | 'contacts' | 'lines' | 'tags' | 'titles' | 'stamps' | 'placekinds' | 'roles' | 'tests';

const LISTS = 'story-lists';

/** Every id a picker may offer, taken from the content. Rebuilt when the model changes. */
export function pickOptions(model: Model): Record<Pick, string[]> {
  const c = model.content;
  const uniq = (xs: (string | undefined)[]) => [...new Set(xs.filter((x): x is string => !!x))].sort();
  const caps = [...(c.archetypes.capabilities ?? []).map((x) => x.id)];
  return {
    flags: [...model.flags.keys()].sort(),
    quests: [...model.quests.keys()].filter((id) => model.quests.get(id)!.kind !== 'achievement').sort(),
    signals: uniq(model.dialogues.emissions.map((e) => e.signal)),
    emotes: ['wave', 'bow', 'photo', 'exercise'],
    shops: uniq((c.items.shops ?? []).map((s) => s.id)),
    fees: uniq((c.items.fees ?? []).map((s) => s.id)),
    npcs: uniq([...c.npcs.map((n) => n.id), ...c.contacts.map((n) => n.id)]),
    places: uniq(c.places.map((p) => p.id)),
    caps: uniq(caps),
    items: uniq((c.items.items ?? []).map((i) => i.id)),
    itemkinds: uniq((c.items.items ?? []).map((i) => i.kind)),
    vehicles: ['xe_so_110'],
    nodes: [],
    variants: uniq([...c.archetypes.archetypes.map((a) => a.id), ...caps]),
    contacts: uniq(c.contacts.map((n) => n.id)),
    lines: model.storyLines,
    tags: uniq(c.archetypes.archetypes.flatMap((a) => a.tags)),
    titles: [],
    stamps: [],
    placekinds: ['landmark', 'street', 'area', 'district'],
    roles: ['rider', 'passenger'],
    tests: ['a1'],
  };
}

/** Fills the page's shared <datalist>s (one per picker kind) from the model. */
export function publishLists(model: Model, extra: Partial<Record<Pick, string[]>> = {}): void {
  let host = document.getElementById(LISTS);
  if (!host) {
    host = h('div', { id: LISTS, hidden: true });
    document.body.append(host);
  }
  const options = { ...pickOptions(model), ...extra };
  host.replaceChildren(...(Object.entries(options) as [Pick, string[]][]).map(([kind, ids]) => h('datalist', { id: `dl-${kind}` }, ids.map((id) => h('option', { value: id })))));
}

export interface InputOptions {
  list?: Pick;
  placeholder?: string;
  mono?: boolean;
  title?: string;
  /** pattern the value must match to be accepted; otherwise the field is marked and the change is not committed */
  pattern?: RegExp;
}

/** A one-line field. */
export function input(value: string, commit: (value: string) => void, options: InputOptions = {}): HTMLInputElement {
  const el = h('input', { type: 'text', value, placeholder: options.placeholder, list: options.list ? `dl-${options.list}` : undefined, class: `fld${options.mono ? ' mono' : ''}`, title: options.title, autocomplete: 'off', spellcheck: false });
  el.addEventListener('change', () => {
    const v = el.value.trim();
    if (options.pattern && v !== '' && !options.pattern.test(v)) {
      el.classList.add('invalid');
      return;
    }
    el.classList.remove('invalid');
    if (v !== value) commit(v);
  });
  return el;
}

export function numberInput(value: number | undefined, commit: (value: number | undefined) => void, placeholder = ''): HTMLInputElement {
  const el = h('input', { type: 'number', value: value ?? '', placeholder, class: 'fld num', min: 0 });
  el.addEventListener('change', () => {
    const n = el.value === '' ? undefined : Number(el.value);
    if (n !== value) commit(n);
  });
  return el;
}

function fit(el: HTMLTextAreaElement): void {
  el.style.height = 'auto';
  el.style.height = `${el.scrollHeight + 2}px`;
}

/** A multi-line text with a character counter against the server's limit. */
export function area(value: string, commit: (value: string) => void, max = 0, placeholder = ''): HTMLElement {
  const el = h('textarea', { rows: 1, class: 'fld area', placeholder, spellcheck: false });
  el.value = value;
  const count = h('span', { class: 'count-chars' });
  const refresh = () => {
    count.textContent = max ? `${[...el.value].length}/${max}` : '';
    count.classList.toggle('over', max > 0 && [...el.value].length > max);
    fit(el);
  };
  el.addEventListener('input', refresh);
  el.addEventListener('change', () => {
    if (el.value !== value) commit(el.value);
  });
  requestAnimationFrame(refresh);
  return h('div', { class: 'area-wrap' }, el, count);
}

/** Vietnamese and English side by side, both required by the server. */
export function textPair(value: Text | undefined, commit: (lang: 'vi' | 'en', value: string) => void, max = 0): HTMLElement {
  return h(
    'div',
    { class: 'pair' },
    h('label', { class: 'lang' }, h('b', {}, 'VI'), area(value?.vi ?? '', (v) => commit('vi', v), max, 'tiếng Việt')),
    h('label', { class: 'lang' }, h('b', {}, 'EN'), area(value?.en ?? '', (v) => commit('en', v), max, 'English')),
  );
}

export function select(options: { value: string; label?: string }[], value: string, commit: (value: string) => void, title?: string): HTMLSelectElement {
  const el = h('select', { class: 'fld', title });
  for (const o of options) {
    const opt = h('option', { value: o.value }, o.label ?? o.value);
    if (o.value === value) opt.setAttribute('selected', '');
    el.append(opt);
  }
  if (!options.some((o) => o.value === value)) el.append(h('option', { value, selected: true }, `${value} (không có)`));
  el.value = value;
  el.addEventListener('change', () => commit(el.value));
  return el;
}

/** A list of ids as removable chips with a field (and the picker's suggestions) to add one. */
export function chips(values: string[], commit: (values: string[]) => void, options: InputOptions & { max?: number; known?: (v: string) => boolean } = {}): HTMLElement {
  const wrap = h('div', { class: 'chips-edit' });
  values.forEach((v, i) => {
    const bad = options.known ? !options.known(v) : false;
    const x = h('button', { class: 'x', title: 'Bỏ', type: 'button' }, '×');
    x.addEventListener('click', () => commit(values.filter((_, j) => j !== i)));
    wrap.append(h('span', { class: `chip${bad ? ' bad' : ''}`, title: bad ? 'Không có trong nội dung' : undefined }, v, x));
  });
  if (options.max === undefined || values.length < options.max) {
    const add = h('input', { type: 'text', class: 'fld chip-input', placeholder: options.placeholder ?? '+ thêm', list: options.list ? `dl-${options.list}` : undefined, autocomplete: 'off', spellcheck: false });
    add.addEventListener('change', () => {
      const v = add.value.trim();
      if (v && !values.includes(v)) commit([...values, v]);
      else add.value = '';
    });
    wrap.append(add);
  }
  return wrap;
}

/** A labelled row of a form. */
export function row(label: string, control: Node, hint?: string): HTMLElement {
  return h('div', { class: 'frow' }, h('span', { class: 'flabel', title: hint }, label), control);
}

export function iconButton(label: string, title: string, on: () => void, cls = ''): HTMLButtonElement {
  const b = h('button', { class: `ibtn ${cls}`.trim(), title, type: 'button' }, label);
  b.addEventListener('click', (ev) => {
    ev.stopPropagation();
    on();
  });
  return b;
}
