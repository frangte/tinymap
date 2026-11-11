// Text of src/game/directions.ts: asking the way and showing it (D18).
import { catalog } from './index';

const vi = {
  errNoAccount: 'Hỏi đường cần một tài khoản. Tải lại trang để thử lại.',
  errNotAllowed: 'Bạn không hỏi hoặc chỉ đường được lúc này.',
  errBusy: 'Bạn đang hỏi đường rồi.',
  errUnknownPlace: 'Chỉ hỏi đường tới các công trình được thôi.',
  errTooSoon: 'Đợi một chút rồi hỏi lại nhé.',
  errGone: 'Người đó không còn hỏi đường nữa.',
  errTooFar: 'Lại gần hơn chút nữa để chỉ đường.',
  errAlreadyHelped: 'Hôm nay bạn đã chỉ đường cho người này rồi.',
  /** a Citizen's notice when a Traveler asks */
  callTitle: 'Có người hỏi đường',
  callText: (from: string, dest: string) => `${from} hỏi đường tới ${dest}. Tới chỗ họ để chỉ đường.`,
  helped: (dest: string, to: string) => `Bạn đã chỉ đường tới ${dest} cho ${to}. Cảm ơn bạn!`,
  /** the helper when the server sends no name */
  someSaigonese: 'Một người Sài Gòn',
  answered: (helper: string, dest: string) => `${helper} chỉ đường cho bạn tới ${dest}.`,
  goShown: (dest: string, helper: string) => `Tới ${dest} (${helper} chỉ)`,
  nobodyPlaying: 'Chưa có người Sài Gòn nào đang chơi. Hỏi Chú Sáu xe ôm ở bùng binh nhé!',
  nobodyCame: 'Không ai tới kịp. Hỏi Chú Sáu xe ôm ở bùng binh nhé!',
  askNpc: (npc: string, dest: string) => `Hỏi ${npc} đường tới ${dest}`,
  offline: 'Đang mất kết nối, chưa hỏi đường được.',
  /** the action button next to an asker */
  answer: (from: string) => `Chỉ đường cho ${from}`,
  /** the tracker's title */
  tracker: 'Hỏi đường',
  callGoal: (from: string, dest: string) => `${from} hỏi đường tới ${dest}`,
  waiting: 'Đang chờ người Sài Gòn tới chỉ đường…',
};

const en: typeof vi = {
  errNoAccount: 'Asking the way needs an account. Reload the page to try again.',
  errNotAllowed: "You can't ask or show the way right now.",
  errBusy: "You're already asking the way.",
  errUnknownPlace: 'You can only ask the way to landmarks.',
  errTooSoon: 'Wait a moment, then ask again.',
  errGone: "That person isn't asking the way any more.",
  errTooFar: 'Get a little closer to show the way.',
  errAlreadyHelped: "You've already shown this person the way today.",
  callTitle: 'Someone is asking the way',
  callText: (from, dest) => `${from} is asking the way to ${dest}. Go over to them to show the way.`,
  helped: (dest, to) => `You showed ${to} the way to ${dest}. Thank you!`,
  someSaigonese: 'A Saigonese',
  answered: (helper, dest) => `${helper} showed you the way to ${dest}.`,
  goShown: (dest, helper) => `Go to ${dest} (shown by ${helper})`,
  nobodyPlaying: 'No Saigonese is playing right now. Ask Chú Sáu, the xe ôm driver at the roundabout!',
  nobodyCame: 'Nobody made it in time. Ask Chú Sáu, the xe ôm driver at the roundabout!',
  askNpc: (npc, dest) => `Ask ${npc} the way to ${dest}`,
  offline: "You're offline, so you can't ask the way yet.",
  answer: (from) => `Show ${from} the way`,
  tracker: 'Asking the way',
  callGoal: (from, dest) => `${from} is asking the way to ${dest}`,
  waiting: 'Waiting for a Saigonese to come and show you the way…',
};

export const L = catalog({ vi, en });
