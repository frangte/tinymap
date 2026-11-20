/** A tiny element builder: `h('div', { class: 'x', onclick: fn }, 'text', child)`. */
export type Child = Node | string | number | null | false | undefined | Child[];
export type Attrs = Record<string, string | number | boolean | EventListener | undefined>;

function apply(el: Element, attrs: Attrs | undefined, children: Child[]): void {
  for (const [key, value] of Object.entries(attrs ?? {})) {
    if (value === undefined || value === false) continue;
    if (typeof value === 'function') el.addEventListener(key.slice(2), value);
    else el.setAttribute(key, value === true ? '' : String(value));
  }
  for (const child of children) {
    if (child === null || child === false || child === undefined) continue;
    if (Array.isArray(child)) apply(el, undefined, child);
    else el.append(child instanceof Node ? child : String(child));
  }
}

export function h<K extends keyof HTMLElementTagNameMap>(tag: K, attrs?: Attrs, ...children: Child[]): HTMLElementTagNameMap[K] {
  const el = document.createElement(tag);
  apply(el, attrs, children);
  return el;
}

const SVG_NS = 'http://www.w3.org/2000/svg';

export function svg(tag: string, attrs?: Attrs, ...children: Child[]): SVGElement {
  const el = document.createElementNS(SVG_NS, tag);
  apply(el, attrs, children);
  return el;
}

/** A small coloured label. */
export function badge(text: string, kind = '', title?: string): HTMLElement {
  return h('span', { class: `badge ${kind}`.trim(), title }, text);
}

/** A link to somewhere in the page (a hash route). */
export function link(text: string, hash: string, cls = ''): HTMLElement {
  return h('a', { class: cls, href: hash }, text);
}
