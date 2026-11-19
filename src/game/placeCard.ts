// The info card in the street (docs/design/10-story.md, section 10): under the place card, top left, while the
// player is at a place with a Cẩm nang page (or the map shows one). One line about the place and "Đọc thêm" (I),
// which opens its page in the Sổ tay; × hides it until the next place. Text only via textContent.
import { el } from './hud';
import { guideFor } from './guide';
import type { Place } from './places';
import { onLangChange, tx } from './i18n';

export class PlaceCard {
  private readonly root = el('place-card');
  private readonly summary = el('place-card-summary');
  private place: Place | null = null;
  /** the place whose card was closed: it stays closed until the player is somewhere else */
  private closed: Place | null = null;

  constructor(private readonly read: (placeId: string) => void) {
    el<HTMLButtonElement>('place-card-read').addEventListener('click', (e) => {
      this.readNow();
      (e.currentTarget as HTMLButtonElement).blur();
    });
    el<HTMLButtonElement>('place-card-close').addEventListener('click', (e) => {
      this.closed = this.place;
      this.fill();
      (e.currentTarget as HTMLButtonElement).blur();
    });
    onLangChange(() => this.fill());
  }

  /** The place the card is about now (null: none). */
  show(place: Place | null): void {
    if (place === this.place) return;
    this.place = place;
    if (this.closed !== place) this.closed = null;
    this.fill();
  }

  /** I, or the button: open the page of the place on the card; false when no card shows. */
  readNow(): boolean {
    if (this.root.hidden || !this.place) return false;
    this.read(this.place.id);
    return true;
  }

  private fill(): void {
    const page = this.place ? guideFor(this.place.id) : undefined;
    this.root.hidden = !page || this.closed === this.place;
    if (page) this.summary.textContent = tx(page.summary);
  }
}
