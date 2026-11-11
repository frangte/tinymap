// Text of src/game/dialogueSheet.ts: the conversation sheet's own words (the lines come from the server).
import { catalog, type Bilingual } from './index';

/** the answer on a line with none: in both languages, like the answers the server sends */
export const GOODBYE: Bilingual = { vi: 'Tạm biệt', en: 'Goodbye' };

const vi = {
  group: 'Nhóm',
  groupOf: (n: number) => `Nhóm · ${n} người`,
};

const en: typeof vi = {
  group: 'Group',
  groupOf: (n) => `Group · ${n} people`,
};

export const L = catalog({ vi, en });
