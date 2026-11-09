// Text of offerBanner.ts: a ride offer for a driver on duty.
import { catalog } from './index';

const vi = {
  who: (passenger: string) => `Cuốc mới: ${passenger}`,
  where: (place: string, fare: string, metres: number) => `Tới ${place} · ${fare} · đón cách ${metres} m`,
};

const en: typeof vi = {
  who: (passenger) => `New ride: ${passenger}`,
  where: (place, fare, metres) => `To ${place} · ${fare} · pickup ${metres} m away`,
};

export const L = catalog({ vi, en });
