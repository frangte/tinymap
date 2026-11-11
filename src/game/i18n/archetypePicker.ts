// "Bạn là ai?", the archetype cards (archetypePicker.ts).
import { catalog } from './index';

const vi = {
  titleFirst: 'Bạn là ai?',
  titleChange: 'Đổi vai',
  ledeFirst: 'Cả Quận 1 chung một con phố, mỗi vai sống ở đó một kiểu. Muốn đổi vai sau này thì vào app Hồ sơ trên điện thoại.',
  ledeChange: 'Đổi vai ngay tại chỗ, mặc đồ của vai mới. Nhiệm vụ, danh hiệu và ví vẫn giữ nguyên.',
  current: 'Vai hiện tại',
};

const en: typeof vi = {
  titleFirst: 'Who are you?',
  titleChange: 'Change role',
  ledeFirst: 'All of District 1 shares one street, and each role lives it in its own way. To change role later, open the Profile app on your phone.',
  ledeChange: 'Change role right here and put on the new role’s clothes. Your quests, titles and wallet stay as they are.',
  current: 'Your role now',
};

export const L = catalog({ vi, en });
