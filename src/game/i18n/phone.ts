// Text of the phone itself (src/game/phone/phone.ts) and its map (src/game/phone/miniMap.ts).
import { catalog } from './index';

const vi = {
  /** the title over the home screen */
  home: 'Điện thoại',
  /** what a screen reader says for the phone's map */
  mapLabel: 'Bản đồ Quận 1',
};

const en: typeof vi = {
  home: 'Phone',
  mapLabel: 'Map of District 1',
};

export const L = catalog({ vi, en });
