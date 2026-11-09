// A ride offer for a driver on duty (docs/design/08-ride-hailing.md): who, where to, the fare, how far the pickup
// is, and a bar running down until it expires. It sits over the game, so a driver on the bike can take it without
// the phone. Text only via textContent; it follows a language switch.
import { el } from './hud';
import { onLangChange } from './i18n';
import { L } from './i18n/offerBanner';
import { OFFER_MS, placeName, xu, type Rides } from './rides';
import type { RideOffer } from '../net/rideProtocol';
import type * as THREE from 'three';

export class OfferBanner {
  private readonly root = el('offer');
  private readonly who = el('offer-who');
  private readonly where = el('offer-where');
  private readonly bar = el('offer-bar');
  private shown: RideOffer | null = null;

  constructor(private readonly rides: Rides, private readonly self: () => THREE.Vector3) {
    el<HTMLButtonElement>('offer-take').addEventListener('click', () => {
      if (this.shown) rides.accept(this.shown.offer);
    });
    el<HTMLButtonElement>('offer-skip').addEventListener('click', () => {
      if (this.shown) rides.decline(this.shown.offer);
    });
    rides.onChange(() => this.refresh());
    onLangChange(() => this.write());
  }

  /** A few times a second in play mode: the bar runs down, the pickup distance follows the driver. */
  update(): void {
    const o = this.shown;
    if (!o) return;
    const left = Math.max(0, o.expires - this.rides.now());
    this.bar.style.transform = `scaleX(${Math.min(1, left / OFFER_MS).toFixed(3)})`;
    const p = this.self();
    const d = Math.round(Math.hypot(o.pickup[0] - p.x, o.pickup[1] - p.z));
    const text = L.where(placeName(o.dest), xu(o.fare), d);
    if (this.where.textContent !== text) this.where.textContent = text;
  }

  private refresh(): void {
    const o = this.rides.nextOffer();
    if (o === this.shown) return;
    this.shown = o;
    this.root.hidden = !o;
    if (!o) return;
    this.write();
  }

  /** The offer's text, in the current language. */
  private write(): void {
    if (!this.shown) return;
    this.who.textContent = L.who(this.shown.passenger.name);
    this.update();
  }
}
