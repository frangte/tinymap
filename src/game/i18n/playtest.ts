// The marker of a test character (src/playtestBadge.ts): this tab plays a character the story workshop made, not the
// player's own.
import { catalog } from './index';

const vi = {
  badge: 'NHÂN VẬT THỬ',
  note: 'Tab này chơi một nhân vật thử, không động tới nhân vật đã lưu của bạn.',
};

const en: typeof vi = {
  badge: 'TEST CHARACTER',
  note: 'This tab plays a test character; it does not touch your saved character.',
};

export const L = catalog({ vi, en });
