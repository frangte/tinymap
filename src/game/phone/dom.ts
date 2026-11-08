// Small DOM helpers for the phone's apps. Text only ever goes in through textContent.

export function h<K extends keyof HTMLElementTagNameMap>(tag: K, cls: string, text?: string): HTMLElementTagNameMap[K] {
  const e = document.createElement(tag);
  e.className = cls;
  if (text !== undefined) e.textContent = text;
  return e;
}

export function button(label: string, onClick: () => void, primary = false): HTMLButtonElement {
  const b = h('button', primary ? 'btn-primary' : '', label);
  b.type = 'button';
  b.addEventListener('click', onClick);
  return b;
}
