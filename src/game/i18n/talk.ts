// Text of src/game/talk.ts: talking to NPCs, as the action button and the chat log put it.
import { catalog } from './index';

const vi = {
  /** why the server would not start a conversation */
  far: (npc: string) => `Lại gần ${npc} thêm chút nữa nha.`,
  full: (npc: string) => `Nhóm của ${npc} đông quá, chờ chút nha.`,
  busy: (npc: string) => `${npc} đang bận.`,
  notHeard: 'Chưa nói chuyện được, thử lại nha.',
  /** the action button */
  greeting: 'Đang chào…',
  talkTo: (npc: string) => `Nói chuyện · ${npc}`,
  walkedAway: (npc: string) => `Bạn đã đi xa ${npc}.`,
};

const en: typeof vi = {
  far: (npc) => `Get a little closer to ${npc}.`,
  full: (npc) => `${npc}'s group is full, wait a moment.`,
  busy: (npc) => `${npc} is busy.`,
  notHeard: "Couldn't start talking, try again.",
  greeting: 'Saying hello…',
  talkTo: (npc) => `Talk · ${npc}`,
  walkedAway: (npc) => `You walked away from ${npc}.`,
};

export const L = catalog({ vi, en });
