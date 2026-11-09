// Names and chat bubbles over heads: HTML labels on a layer above the canvas, moved every frame to the projected
// head position. HTML rather than 3D text keeps Vietnamese diacritics sharp at any size and needs no InkPass
// tracking or textures. Text only ever goes in through textContent.
// Everyone, us included, shows a name; while someone has a chat bubble up, the bubble takes the name's place.
// NPCs show what they do under the name, so they read as people to go and talk to.
// Labels are not hidden behind buildings (no per-frame raycasts against the city): they fade out with distance.
import * as THREE from 'three';
import { clamp, smoothstep } from './coords';
import type { RemotePlayers } from './remotePlayers';
import type { Npcs } from './npcs';
import { archetypeById } from './archetypes';
import { titleName } from './quests';
import { tx } from './i18n';

/** the local player's key: server ids start at 1 */
export const SELF = 0;
/** label anchor above the feet, just over a nón lá (m) */
const HEAD = 1.95;
/** labels fade out between these distances from the camera (m) */
const FADE_FROM = 20;
const FADE_TO = 30;
/** a bubble stays this long, plus a little per character (ms) */
const BUBBLE_MIN = 4000;
const BUBBLE_PER_CHAR = 60;
const BUBBLE_MAX = 10000;
/** gap kept between two labels pushed apart (px) */
const GAP = 3;

class Tag {
  readonly root = document.createElement('div');
  private readonly bubble = document.createElement('p');
  private readonly label = document.createElement('span');
  private role: HTMLElement | null = null;
  private titleEl: HTMLElement | null = null;
  private name = '';
  private dot: string | null = null;
  private bubbleUntil = 0;
  // last written styles, so a still frame writes nothing
  private transform = '';
  private opacity = -1;
  private z = -1;
  private shown = false;
  /** unscaled size, re-measured after the text changes (offset sizes ignore the transform) */
  private w = -1;
  private h = -1;
  // this frame's placement, settled by NameTags before it is written
  x = 0;
  y = 0;
  scale = 1;
  alpha = 1;
  dist = 0;

  constructor(layer: HTMLElement, readonly self: boolean) {
    this.root.className = self ? 'tag is-self' : 'tag';
    this.root.hidden = true;
    this.bubble.className = 'bubble';
    this.bubble.hidden = true;
    this.label.className = 'tag-name';
    this.root.append(this.bubble, this.label);
    layer.appendChild(this.root);
  }

  setName(name: string): void {
    if (name === this.name) return;
    this.name = name;
    this.label.textContent = name;
    this.w = this.h = -1;
  }

  /** what an NPC does, in small print under the name */
  setRole(text: string): void {
    if (!this.role) {
      this.role = document.createElement('small');
      this.role.className = 'tag-role';
      this.root.classList.add('is-npc');
      this.root.append(this.role);
    }
    if (this.role.textContent === text) return;
    this.role.textContent = text;
    this.w = this.h = -1;
  }

  /** a player's title in small print under the name (null: none) */
  setTitle(text: string | null): void {
    if ((this.titleEl?.textContent ?? null) === text || (!this.titleEl && text === null)) return;
    if (text === null) {
      this.titleEl?.remove();
      this.titleEl = null;
    } else {
      if (!this.titleEl) {
        this.titleEl = document.createElement('small');
        this.titleEl.className = 'tag-title';
        this.root.append(this.titleEl);
      }
      this.titleEl.textContent = text;
    }
    this.w = this.h = -1;
  }

  /** a dot in the archetype's colour before the name (null: none) */
  setDot(color: string | null): void {
    if (color === this.dot) return;
    this.dot = color;
    this.label.classList.toggle('has-dot', color !== null);
    if (color) this.label.style.setProperty('--dot', color);
    this.w = this.h = -1;
  }

  get width(): number {
    if (this.w < 0) this.measure();
    return this.w * this.scale;
  }

  get height(): number {
    if (this.h < 0) this.measure();
    return this.h * this.scale;
  }

  private measure(): void {
    this.w = this.root.offsetWidth;
    this.h = this.root.offsetHeight;
  }

  say(text: string, now: number): void {
    this.bubble.textContent = text;
    this.bubble.hidden = false;
    this.root.classList.add('is-speaking');
    this.bubbleUntil = now + Math.min(BUBBLE_MAX, BUBBLE_MIN + BUBBLE_PER_CHAR * Array.from(text).length);
    this.w = this.h = -1;
    // restart the pop-in animation
    this.bubble.classList.remove('is-new');
    void this.bubble.offsetWidth;
    this.bubble.classList.add('is-new');
  }

  /** take an expired bubble down: the name shows again */
  private expire(now: number): void {
    if (this.bubble.hidden || now <= this.bubbleUntil) return;
    this.bubble.hidden = true;
    this.root.classList.remove('is-speaking');
    this.w = this.h = -1;
  }

  hide(): void {
    if (!this.shown) return;
    this.shown = false;
    this.root.hidden = true;
  }

  /** shown this frame at its projected spot; `write` puts it on screen once overlaps are settled */
  show(x: number, y: number, scale: number, alpha: number, dist: number, now: number): void {
    this.expire(now);
    if (!this.shown) {
      this.shown = true;
      this.root.hidden = false;
      this.w = this.h = -1;
    }
    this.x = x;
    this.y = y;
    this.scale = scale;
    this.alpha = alpha;
    this.dist = dist;
  }

  write(): void {
    const { x, y, scale } = this;
    const t = `translate3d(${x.toFixed(1)}px, ${y.toFixed(1)}px, 0) translate(-50%, -100%) scale(${scale.toFixed(3)})`;
    if (t !== this.transform) {
      this.transform = t;
      this.root.style.transform = t;
    }
    const o = Math.round(this.alpha * 50) / 50;
    if (o !== this.opacity) {
      this.opacity = o;
      this.root.style.opacity = String(o);
    }
    // nearer labels drawn over further ones
    const z = Math.round(1000 - this.dist * 10);
    if (z !== this.z) {
      this.z = z;
      this.root.style.zIndex = String(z);
    }
  }

  dispose(): void {
    this.root.remove();
  }
}

export class NameTags {
  private readonly tags = new Map<number, Tag>();
  private readonly self: Tag;
  private width = 1;
  private height = 1;
  private readonly v = new THREE.Vector3();
  /** labels on screen this frame, nearest first */
  private readonly visible: Tag[] = [];

  constructor(private readonly layer: HTMLElement) {
    this.self = new Tag(layer, true);
  }

  /** the canvas size in CSS pixels (cached: reading it every frame would force a layout) */
  setSize(width: number, height: number): void {
    this.width = width;
    this.height = height;
  }

  /** our own name, as the server cleaned it */
  setSelfName(name: string): void {
    this.self.setName(name);
  }

  /** our own archetype ('' for none) */
  setSelfArch(arch: string): void {
    this.self.setDot(archetypeById(arch)?.badge.color ?? null);
  }

  /** our own title id ('' for none) */
  setSelfTitle(title: string): void {
    this.self.setTitle(titleName(title));
  }

  /** A chat bubble over a head: `SELF` for our own lines. Unknown ids are ignored (not placed yet). */
  say(id: number, text: string, now: number): void {
    (id === SELF ? this.self : this.tags.get(id))?.say(text, now);
  }

  /** Follow the players and the NPCs: labels are created, renamed and dropped from the lists themselves. */
  update(camera: THREE.PerspectiveCamera, now: number, others: RemotePlayers, npcs: Npcs, selfPos: THREE.Vector3): void {
    camera.updateMatrixWorld();
    this.visible.length = 0;
    for (const [id, tag] of this.tags) {
      if (!others.get(id) && !npcs.get(id)) {
        tag.dispose();
        this.tags.delete(id);
      }
    }
    for (const n of npcs.values()) {
      let tag = this.tags.get(n.id);
      if (!tag) {
        tag = new Tag(this.layer, false);
        tag.setName(n.def.name);
        this.tags.set(n.id, tag);
      }
      // every frame, so a language switch shows at once (it writes nothing while the text stays the same)
      tag.setRole(tx(n.def.role));
      if (n.actor.avatar.root.visible) this.project(tag, camera, n.position, now);
      else tag.hide();
    }
    for (const r of others.values()) {
      let tag = this.tags.get(r.id);
      if (!tag) {
        tag = new Tag(this.layer, false);
        this.tags.set(r.id, tag);
      }
      tag.setName(r.name);
      tag.setDot(archetypeById(r.arch)?.badge.color ?? null);
      tag.setTitle(titleName(r.title));
      if (r.avatar.root.visible) this.project(tag, camera, r.position, now);
      else tag.hide();
    }
    this.project(this.self, camera, selfPos, now);
    this.declutter();
    for (const tag of this.visible) tag.write();
  }

  /**
   * Players standing together would stack their labels on top of each other: nearest first, each label that
   * overlaps one already settled moves up above it (a few dozen labels at most, so pairwise is fine).
   * Our own label goes first and never moves, so it always sits on our own head.
   */
  private declutter(): void {
    const tags = this.visible;
    tags.sort((a, b) => Number(b.self) - Number(a.self) || a.dist - b.dist);
    for (let i = 1; i < tags.length; i++) {
      const t = tags[i];
      const w = t.width;
      const h = t.height;
      for (let moved = true, guard = 0; moved && guard < tags.length; guard++) {
        moved = false;
        for (let j = 0; j < i; j++) {
          const o = tags[j];
          // anchors are bottom-centre: overlap when the boxes share both spans
          if (Math.abs(t.x - o.x) * 2 < w + o.width && t.y > o.y - o.height - GAP && t.y - h < o.y) {
            t.y = o.y - o.height - GAP;
            moved = true;
          }
        }
      }
    }
  }

  private project(tag: Tag, camera: THREE.PerspectiveCamera, feet: THREE.Vector3, now: number): void {
    const v = this.v.copy(feet);
    v.y += HEAD;
    const dist = v.distanceTo(camera.position);
    const opacity = 1 - smoothstep(FADE_FROM, FADE_TO, dist);
    v.project(camera);
    // behind the camera, beyond the far plane, faded out or off screen
    if (opacity <= 0 || v.z > 1 || v.z < -1 || Math.abs(v.x) > 1.2 || Math.abs(v.y) > 1.2) {
      tag.hide();
      return;
    }
    const x = (v.x + 1) * 0.5 * this.width;
    const y = (1 - v.y) * 0.5 * this.height;
    // nearer labels slightly bigger
    tag.show(x, y, clamp(1.15 - dist / 40, 0.75, 1.1), opacity, dist, now);
    this.visible.push(tag);
  }
}
