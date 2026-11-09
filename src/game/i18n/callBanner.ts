// Text of src/game/callBanner.ts: the banner a Citizen gets when a Traveler asks the way.
import { catalog } from './index';

const vi = {
  who: (from: string) => `${from} hỏi đường`,
  where: (dest: string, away: string) => `Tới ${dest} · ${away}`,
  metres: (m: number) => `cách bạn ${m} m`,
  far: 'ở xa',
  pressE: 'Bấm E để chỉ đường',
  closer: 'Lại gần để chỉ đường',
};

const en: typeof vi = {
  who: (from) => `${from} is asking the way`,
  where: (dest, away) => `To ${dest} · ${away}`,
  metres: (m) => `${m} m from you`,
  far: 'far away',
  pressE: 'Press E to show the way',
  closer: 'Get closer to show the way',
};

export const L = catalog({ vi, en });
