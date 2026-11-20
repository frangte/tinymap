export interface PanZoom {
  /** the size of the content, in its own pixels */
  setContent(width: number, height: number): void;
  fit(): void;
  /** scales to a readable size and centres on a box */
  focusOn(x: number, y: number, w: number, h: number): void;
  /** 100% with a point of the content at the top left of the frame */
  anchorAt(x: number, y: number): void;
  /** where a screen point is in the content's own pixels */
  toWorld(clientX: number, clientY: number): { x: number; y: number };
  state(): ViewState;
  setState(view: ViewState): void;
  /** called after every pan or zoom */
  onChange(fn: (view: ViewState) => void): void;
}

export interface ViewState {
  scale: number;
  tx: number;
  ty: number;
}

const MIN_SCALE = 0.1;
const MAX_SCALE = 2.5;
const FIT_MARGIN = 24;
const FOCUS_SCALE = 1;
/** a long graph is shown at a size that can be read from its first node, not squeezed into the frame */
const MIN_FIT_SCALE = 0.6;

/** Wheel to zoom at the pointer, drag the background to pan; `world` is the transformed layer inside `viewport`. */
export function attachPanZoom(viewport: HTMLElement, world: HTMLElement): PanZoom {
  let scale = 1;
  let tx = 0;
  let ty = 0;
  let content = { w: 1, h: 1 };
  let onScale: (percent: number) => void = () => {};
  let changed: (view: ViewState) => void = () => {};
  const apply = () => {
    changed({ scale, tx, ty });
    world.style.transform = `translate(${tx}px, ${ty}px) scale(${scale})`;
    onScale(Math.round(scale * 100));
  };
  const zoomAt = (px: number, py: number, factor: number) => {
    const next = Math.min(MAX_SCALE, Math.max(MIN_SCALE, scale * factor));
    tx = px - ((px - tx) * next) / scale;
    ty = py - ((py - ty) * next) / scale;
    scale = next;
    apply();
  };
  viewport.addEventListener('wheel', (ev) => {
    ev.preventDefault();
    const r = viewport.getBoundingClientRect();
    if (ev.ctrlKey || ev.metaKey || Math.abs(ev.deltaY) > 40 || ev.deltaX === 0) zoomAt(ev.clientX - r.left, ev.clientY - r.top, Math.exp(-ev.deltaY * 0.0015));
    else {
      tx -= ev.deltaX;
      ty -= ev.deltaY;
      apply();
    }
  }, { passive: false });
  let drag: { x: number; y: number } | null = null;
  viewport.addEventListener('pointerdown', (ev) => {
    if ((ev.target as HTMLElement).closest('a, button, input, textarea, select, details, .handle, .no-pan')) return;
    drag = { x: ev.clientX, y: ev.clientY };
    viewport.setPointerCapture(ev.pointerId);
    viewport.classList.add('panning');
  });
  viewport.addEventListener('pointermove', (ev) => {
    if (!drag) return;
    tx += ev.clientX - drag.x;
    ty += ev.clientY - drag.y;
    drag = { x: ev.clientX, y: ev.clientY };
    apply();
  });
  const stop = () => {
    drag = null;
    viewport.classList.remove('panning');
  };
  viewport.addEventListener('pointerup', stop);
  viewport.addEventListener('pointercancel', stop);

  const buttons = document.createElement('div');
  buttons.className = 'zoom-buttons';
  const button = (label: string, title: string, on: () => void) => {
    const b = document.createElement('button');
    b.textContent = label;
    b.title = title;
    b.addEventListener('click', on);
    buttons.append(b);
  };
  const centre = () => [viewport.clientWidth / 2, viewport.clientHeight / 2] as const;
  const api: PanZoom = {
    toWorld(clientX, clientY) {
      const r = viewport.getBoundingClientRect();
      return { x: (clientX - r.left - tx) / scale, y: (clientY - r.top - ty) / scale };
    },
    state: () => ({ scale, tx, ty }),
    setState(view) {
      ({ scale, tx, ty } = view);
      apply();
    },
    onChange(fn) {
      changed = fn;
    },
    setContent(width, height) {
      content = { w: width, h: height };
    },
    fit() {
      const vw = viewport.clientWidth - FIT_MARGIN * 2;
      const vh = viewport.clientHeight - FIT_MARGIN * 2;
      scale = Math.min(1, vw / content.w, vh / content.h);
      scale = Math.max(MIN_FIT_SCALE, scale);
      tx = FIT_MARGIN;
      ty = FIT_MARGIN + Math.max(0, (vh - content.h * scale) / 2);
      if (content.w * scale > vw) ty = FIT_MARGIN;
      apply();
    },
    anchorAt(x, y) {
      scale = 1;
      tx = FIT_MARGIN - x;
      ty = FIT_MARGIN - y;
      apply();
    },
    focusOn(x, y, w, h) {
      scale = FOCUS_SCALE;
      tx = viewport.clientWidth / 2 - (x + w / 2) * scale;
      ty = viewport.clientHeight / 2 - (y + h / 2) * scale;
      apply();
    },
  };
  button('+', 'Phóng to', () => zoomAt(...centre(), 1.25));
  button('−', 'Thu nhỏ', () => zoomAt(...centre(), 0.8));
  button('Vừa khung', 'Vừa cả sơ đồ', () => api.fit());
  button('100%', 'Về cỡ thật (100%)', () => {
    scale = 1;
    apply();
  });
  const label = buttons.lastElementChild as HTMLButtonElement;
  onScale = (percent) => (label.textContent = `${percent}%`);
  viewport.append(buttons);
  return api;
}
