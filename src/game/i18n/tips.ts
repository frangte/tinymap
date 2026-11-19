// Text around the travel tips: the tip card (src/game/tipCard.ts) and the Sổ mẹo (notebook/guideBook.ts).
// The tips themselves are content: server/content/tips.json.
import { catalog } from './index';

const vi = {
  eyebrow: 'Mẹo',
  seen: 'Đã gặp',
  title: 'Sổ mẹo',
  intro: 'Những mẹo nhỏ khi đi dạo Sài Gòn. Mỗi mẹo hiện lên lần đầu bạn gặp tình huống đó; đọc trước cũng được.',
  progress: (seen: number, all: number) => `Đã gặp ${seen} / ${all} mẹo`,
};

const en: typeof vi = {
  eyebrow: 'Tip',
  seen: 'Seen',
  title: 'Tip book',
  intro: 'Small tips for getting around Saigon. Each one pops up the first time you meet its situation; you can read them in advance too.',
  progress: (seen, all) => `Seen ${seen} / ${all} tips`,
};

export const L = catalog({ vi, en });
