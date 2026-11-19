// Text of Saigon's clock: the line on the start menu and the time on the place card (src/game/daylight/clockView.ts).
import { catalog } from './index';

const vi = {
  clockTitle: 'Giờ Sài Gòn',
  clockAria: (time: string) => `Giờ Sài Gòn: ${time}`,
  night: (time: string) => `${time} ở Sài Gòn · thành phố đã lên đèn`,
  dawn: (time: string) => `${time} ở Sài Gòn · trời hửng sáng`,
  day: (time: string) => `${time} ở Sài Gòn · ban ngày`,
  dusk: (time: string) => `${time} ở Sài Gòn · chiều tà`,
};

const en: typeof vi = {
  clockTitle: 'Saigon time',
  clockAria: (time) => `Saigon time: ${time}`,
  night: (time) => `${time} in Saigon · the city lights are on`,
  dawn: (time) => `${time} in Saigon · first light`,
  day: (time) => `${time} in Saigon · daytime`,
  dusk: (time) => `${time} in Saigon · sunset`,
};

export const L = catalog({ vi, en });
