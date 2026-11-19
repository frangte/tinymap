// Text of Saigon's clock and weather: the line on the start menu and the time on the place card
// (src/game/daylight/clockView.ts).
import { catalog } from './index';

const vi = {
  clockTitle: 'Giờ Sài Gòn',
  clockAria: (time: string) => `Giờ Sài Gòn: ${time}`,
  /** the menu line: the time, the temperature when known, then the weather (or the time of day when fair) */
  line: (time: string, what: string, temp: number | null) => `${time} ở Sài Gòn${temp === null ? '' : ` · ${temp}°C`} · ${what}`,
  night: 'thành phố đã lên đèn',
  dawn: 'trời hửng sáng',
  day: 'ban ngày',
  dusk: 'chiều tà',
  cloudy: 'nhiều mây',
  drizzle: 'mưa lâm râm',
  rain: 'đang mưa',
  storm: 'mưa dông',
};

const en: typeof vi = {
  clockTitle: 'Saigon time',
  clockAria: (time) => `Saigon time: ${time}`,
  line: (time, what, temp) => `${time} in Saigon${temp === null ? '' : ` · ${temp}°C`} · ${what}`,
  night: 'the city lights are on',
  dawn: 'first light',
  day: 'daytime',
  dusk: 'sunset',
  cloudy: 'cloudy',
  drizzle: 'drizzle',
  rain: 'raining',
  storm: 'thunderstorm',
};

export const L = catalog({ vi, en });
