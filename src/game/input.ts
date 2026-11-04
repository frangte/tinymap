// Keyboard (WASD / arrows, Shift to run), mouse drag + wheel for the camera, and on touch screens a
// floating joystick on the left half with one-finger orbit and two-finger pinch on the right.
import { clamp } from './coords';

export interface MoveIntent {
  /** strafe, -1 (left) … 1 (right) */
  x: number;
  /** forward, -1 (back) … 1 (forward) */
  y: number;
  run: boolean;
}

export interface LookDelta {
  /** radians to add to the camera yaw */
  yaw: number;
  /** radians to add to the camera elevation */
  pitch: number;
  /** log-distance change; positive zooms out */
  zoom: number;
}

const KEYS: Record<string, [number, number]> = {
  KeyW: [0, 1], ArrowUp: [0, 1],
  KeyS: [0, -1], ArrowDown: [0, -1],
  KeyA: [-1, 0], ArrowLeft: [-1, 0],
  KeyD: [1, 0], ArrowRight: [1, 0],
};
const JOY_RADIUS = 56;
const RUN_AT = 0.9;

export interface JoystickView {
  base: HTMLElement;
  knob: HTMLElement;
}

interface Drag {
  x: number;
  y: number;
}

export class InputController {
  private readonly keys = new Set<string>();
  private shift = false;
  private readonly look: LookDelta = { yaw: 0, pitch: 0, zoom: 0 };
  private readonly drags = new Map<number, Drag>();
  private pinch = 0;
  private joyId: number | null = null;
  private joyX = 0;
  private joyY = 0;
  private joyOx = 0;
  private joyOy = 0;
  /** performance.now() of the last camera gesture */
  lastLook = -1e9;
  /** performance.now() of the first movement input, or null until the player moves */
  firstMove: number | null = null;
  private readonly off: Array<() => void> = [];
  private on = true;

  constructor(private readonly surface: HTMLElement, private readonly joy: JoystickView) {
    this.listen(window, 'keydown', (e) => this.onKey(e as KeyboardEvent, true));
    this.listen(window, 'keyup', (e) => this.onKey(e as KeyboardEvent, false));
    this.listen(window, 'blur', () => this.reset());
    this.listen(document, 'visibilitychange', () => this.reset());
    this.listen(surface, 'pointerdown', (e) => this.onDown(e as PointerEvent));
    this.listen(surface, 'pointermove', (e) => this.onMove(e as PointerEvent));
    this.listen(surface, 'pointerup', (e) => this.onUp(e as PointerEvent));
    this.listen(surface, 'pointercancel', (e) => this.onUp(e as PointerEvent));
    this.listen(surface, 'contextmenu', (e) => e.preventDefault());
    this.listen(surface, 'wheel', (e) => this.onWheel(e as WheelEvent), { passive: false });
  }

  private listen(t: EventTarget, type: string, fn: (e: Event) => void, opts?: AddEventListenerOptions): void {
    t.addEventListener(type, fn, opts);
    this.off.push(() => t.removeEventListener(type, fn, opts));
  }

  /** While disabled (map view, menu, camera flights) every key and gesture is ignored. */
  get enabled(): boolean {
    return this.on;
  }

  set enabled(v: boolean) {
    if (v === this.on) return;
    this.on = v;
    this.reset();
    this.look.yaw = this.look.pitch = this.look.zoom = 0;
  }

  dispose(): void {
    this.off.forEach((f) => f());
    this.off.length = 0;
  }

  private reset(): void {
    this.keys.clear();
    this.shift = false;
    this.drags.clear();
    this.endJoystick();
  }

  private onKey(e: KeyboardEvent, down: boolean): void {
    if (!this.on) return;
    if (e.key === 'Shift') this.shift = down;
    if (!(e.code in KEYS)) return;
    if (down && (e.metaKey || e.ctrlKey || e.altKey)) return;
    if (down) {
      this.keys.add(e.code);
      this.markMove();
    } else {
      this.keys.delete(e.code);
    }
    this.shift = e.shiftKey;
    e.preventDefault();
  }

  private markMove(): void {
    if (this.firstMove === null) this.firstMove = performance.now();
  }

  private onDown(e: PointerEvent): void {
    if (!this.on) return;
    this.surface.focus({ preventScroll: true });
    const leftHalf = e.clientX < window.innerWidth * 0.45;
    if (e.pointerType === 'touch' && leftHalf && this.joyId === null) {
      this.joyId = e.pointerId;
      this.joyOx = e.clientX;
      this.joyOy = e.clientY;
      this.joyX = this.joyY = 0;
      this.joy.base.style.transform = `translate(${e.clientX}px, ${e.clientY}px)`;
      this.joy.knob.style.transform = 'translate(0px, 0px)';
      this.joy.base.hidden = false;
    } else {
      this.drags.set(e.pointerId, { x: e.clientX, y: e.clientY });
      this.pinch = this.pinchDistance();
    }
    try {
      this.surface.setPointerCapture(e.pointerId);
    } catch {
      // the pointer is already gone (e.g. a synthetic event); dragging still works without capture
    }
  }

  private onMove(e: PointerEvent): void {
    if (e.pointerId === this.joyId) {
      let dx = e.clientX - this.joyOx;
      let dy = e.clientY - this.joyOy;
      const len = Math.hypot(dx, dy);
      if (len > JOY_RADIUS) {
        dx = (dx / len) * JOY_RADIUS;
        dy = (dy / len) * JOY_RADIUS;
      }
      this.joyX = dx / JOY_RADIUS;
      this.joyY = -dy / JOY_RADIUS;
      this.joy.knob.style.transform = `translate(${dx}px, ${dy}px)`;
      if (Math.hypot(this.joyX, this.joyY) > 0.2) this.markMove();
      return;
    }
    const d = this.drags.get(e.pointerId);
    if (!d) return;
    const dx = e.clientX - d.x;
    const dy = e.clientY - d.y;
    d.x = e.clientX;
    d.y = e.clientY;
    if (this.drags.size === 1) {
      const k = (Math.PI * 1.6) / Math.max(320, window.innerHeight);
      this.look.yaw -= dx * k;
      this.look.pitch += dy * k;
    } else if (this.drags.size === 2) {
      const dist = this.pinchDistance();
      if (this.pinch > 0 && dist > 0) this.look.zoom += Math.log(this.pinch / dist);
      this.pinch = dist;
    }
    this.lastLook = performance.now();
  }

  private onUp(e: PointerEvent): void {
    if (e.pointerId === this.joyId) {
      this.endJoystick();
    } else {
      this.drags.delete(e.pointerId);
      this.pinch = this.pinchDistance();
    }
    if (this.surface.hasPointerCapture(e.pointerId)) this.surface.releasePointerCapture(e.pointerId);
  }

  private endJoystick(): void {
    this.joyId = null;
    this.joyX = this.joyY = 0;
    this.joy.base.hidden = true;
  }

  private pinchDistance(): number {
    if (this.drags.size !== 2) return 0;
    const [a, b] = [...this.drags.values()];
    return Math.hypot(a.x - b.x, a.y - b.y);
  }

  private onWheel(e: WheelEvent): void {
    if (!this.on) return;
    e.preventDefault();
    const px = e.deltaMode === 1 ? e.deltaY * 16 : e.deltaY;
    this.look.zoom += clamp(px, -120, 120) * 0.0016;
    this.lastLook = performance.now();
  }

  /** Current movement intent, magnitude ≤ 1. */
  readMove(out: MoveIntent): MoveIntent {
    let x = 0;
    let y = 0;
    for (const k of this.keys) {
      const v = KEYS[k];
      x += v[0];
      y += v[1];
    }
    x = clamp(x, -1, 1);
    y = clamp(y, -1, 1);
    let run = this.shift;
    if (this.joyId !== null) {
      x += this.joyX;
      y += this.joyY;
      run = run || Math.hypot(this.joyX, this.joyY) > RUN_AT;
    }
    const len = Math.hypot(x, y);
    if (len > 1) {
      x /= len;
      y /= len;
    }
    out.x = x;
    out.y = y;
    out.run = run;
    return out;
  }

  /** Camera gestures since the last call. */
  consumeLook(out: LookDelta): LookDelta {
    out.yaw = this.look.yaw;
    out.pitch = this.look.pitch;
    out.zoom = this.look.zoom;
    this.look.yaw = this.look.pitch = this.look.zoom = 0;
    return out;
  }
}
