// Text of the quest tracker under the place card (src/game/questTracker.ts).
import { catalog } from './index';

const vi = {
  /** in place of the distance once the player is there */
  here: 'ở đây',
  /** what a screen reader says for the tracker button */
  label: (quest: string, goal: string) => `Nhiệm vụ: ${quest}. ${goal}. Mở danh sách nhiệm vụ`,
};

const en: typeof vi = {
  here: 'here',
  label: (quest, goal) => `Quest: ${quest}. ${goal}. Open the quest list`,
};

export const L = catalog({ vi, en });
