// Dressing panel: display name, body (Nam / Nữ), outfit and hat, previewed live on the player.
// Opens by itself the first time someone enters play, and again from the "Đổi đồ" button.
import { BODIES, BODY_NAMES, DEFAULT_OUTFIT, HATS, HAT_NAMES, OUTFITS, outfitStyle, type Body, type HatId, type Look, type OutfitId } from './outfits';
import { NAME_MAX, cleanName, randomName, type Profile } from './profile';
import { el } from './hud';

export interface DressHandlers {
  /** every change, so the avatar can show it at once */
  preview(look: Look): void;
  /** confirmed: the cleaned name and the chosen look */
  done(profile: Profile): void;
  /** closed without saving; the caller restores the previous look */
  cancel(): void;
}

function choice(label: string, onPick: () => void): HTMLButtonElement {
  const b = document.createElement('button');
  b.type = 'button';
  b.setAttribute('aria-pressed', 'false');
  b.textContent = label;
  b.addEventListener('click', onPick);
  return b;
}

export class DressPanel {
  private readonly root = el('dress');
  private readonly nameInput = el<HTMLInputElement>('dress-name');
  private readonly cancelButton = el<HTMLButtonElement>('dress-cancel');
  private readonly doneButton = el<HTMLButtonElement>('dress-done');
  private readonly bodyButtons = new Map<Body, HTMLButtonElement>();
  private readonly outfitButtons = new Map<OutfitId, { button: HTMLButtonElement; top: HTMLElement; bottom: HTMLElement }>();
  private readonly hatButtons = new Map<HatId, HTMLButtonElement>();
  private look: Look = { body: 'man', outfit: 'thun', hat: 'nonla' };
  private first = false;
  /** the first time, switching body also switches to that body's default outfit until one is picked */
  private outfitPicked = false;

  constructor(private readonly on: DressHandlers) {
    this.nameInput.maxLength = NAME_MAX;
    const bodies = el('dress-body');
    for (const body of BODIES) {
      const b = choice(BODY_NAMES[body], () => this.pickBody(body));
      this.bodyButtons.set(body, b);
      bodies.appendChild(b);
    }
    const outfits = el('dress-outfit');
    for (const outfit of OUTFITS) {
      const b = choice('', () => this.pick({ outfit }, true));
      b.className = 'outfit';
      const sw = document.createElement('span');
      sw.className = 'sw';
      sw.setAttribute('aria-hidden', 'true');
      const top = document.createElement('i');
      const bottom = document.createElement('i');
      sw.append(top, bottom);
      const name = document.createElement('span');
      name.textContent = outfitStyle(outfit, 'man').name;
      b.append(sw, name);
      this.outfitButtons.set(outfit, { button: b, top, bottom });
      outfits.appendChild(b);
    }
    const hats = el('dress-hat');
    for (const hat of HATS) {
      const b = choice(HAT_NAMES[hat], () => this.pick({ hat }, false));
      this.hatButtons.set(hat, b);
      hats.appendChild(b);
    }
    this.doneButton.addEventListener('click', () => this.finish());
    this.cancelButton.addEventListener('click', () => this.dismiss());
    this.nameInput.addEventListener('keydown', (e) => {
      if (e.key === 'Enter') this.finish();
    });
  }

  get isOpen(): boolean {
    return !this.root.hidden;
  }

  /** @param first the first visit: no cancel (closing keeps the shown look) */
  open(profile: Profile, first: boolean): void {
    this.look = { ...profile.look };
    this.first = first;
    this.outfitPicked = !first;
    this.nameInput.value = profile.name;
    this.cancelButton.hidden = first;
    this.render();
    this.root.hidden = false;
    document.body.classList.add('is-dressing');
    // focus a choice rather than the name field, so phones do not pop the keyboard up
    this.bodyButtons.get(this.look.body)?.focus({ preventScroll: true });
  }

  /** Esc / the cancel button: drop the changes, or keep them on the first visit. */
  dismiss(): void {
    if (!this.isOpen) return;
    if (this.first) {
      this.finish();
      return;
    }
    this.hide();
    this.on.cancel();
  }

  private finish(): void {
    const name = cleanName(this.nameInput.value) || randomName();
    this.hide();
    this.on.done({ name, look: { ...this.look } });
  }

  private hide(): void {
    this.root.hidden = true;
    document.body.classList.remove('is-dressing');
    this.nameInput.blur();
  }

  private pickBody(body: Body): void {
    this.pick(this.outfitPicked ? { body } : { body, outfit: DEFAULT_OUTFIT[body] }, false);
  }

  private pick(change: Partial<Look>, isOutfit: boolean): void {
    if (isOutfit) this.outfitPicked = true;
    this.look = { ...this.look, ...change };
    this.render();
    this.on.preview({ ...this.look });
  }

  private render(): void {
    const { body, outfit, hat } = this.look;
    for (const [k, b] of this.bodyButtons) b.setAttribute('aria-pressed', String(k === body));
    for (const [k, b] of this.hatButtons) b.setAttribute('aria-pressed', String(k === hat));
    for (const [k, o] of this.outfitButtons) {
      const s = outfitStyle(k, body);
      o.button.setAttribute('aria-pressed', String(k === outfit));
      o.top.style.background = s.top;
      o.bottom.style.background = s.bottom;
    }
  }
}
