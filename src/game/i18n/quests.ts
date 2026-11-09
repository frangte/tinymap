// Text of the quest counts (src/game/quests.ts).
import { catalog } from './index';

const vi = {
  /** an objective that takes seconds of staying somewhere */
  seconds: (n: number, of: number) => `${n} / ${of} giây`,
};

const en: typeof vi = {
  seconds: (n, of) => `${n} / ${of} s`,
};

export const L = catalog({ vi, en });
