// Text of the phone's apps (src/game/phone/apps.ts). The sections that moved to the Sổ tay have theirs in notebook.ts.
import { catalog } from './index';

const vi = {
  /** locale for clock times in the notices */
  timeLocale: 'vi-VN',
  // chat
  channelsAbout: (radius: number) => `Cả phố: ai đang ở Quận 1 cũng đọc được. Gần đây: chỉ những người quanh bạn, trong ${radius} m.`,
  channelGlobal: 'Cả phố',
  channelNearby: 'Gần đây',
  writeMessage: 'Viết tin nhắn',
  chatOffline: 'Đang mất kết nối, chưa chat được.',
  muteTitle: 'Tắt tiếng và báo cáo',
  muteAbout: 'Bấm vào tên một người trong khung chat để tắt tiếng họ, hoặc báo cáo cho người điều hành.',
  unmute: 'Bật lại',
  // notices
  justNow: 'vừa xong',
  minutesAgo: (mins: number) => `${mins} phút trước`,
  noNotices: 'Chưa có thông báo nào.',
  // guide: asking the way
  askTitle: 'Hỏi đường người Sài Gòn',
  asking: (dest: string) => `Đang hỏi đường tới ${dest}. Người Sài Gòn đang chơi sẽ tới chỉ cho bạn.`,
  askCancel: 'Thôi, không hỏi nữa',
  askFallback: 'Lần trước không ai tới: Chú Sáu xe ôm ở bùng binh sẽ chỉ đường cho bạn. Hoặc hỏi lại:',
  askPick: 'Hỏi người Sài Gòn đang chơi đường tới:',
  askLeft: (left: string) => `Còn ${left} trước khi hỏi Chú Sáu.`,
  // neighbours
  communityOffline: 'Đang mất kết nối, chưa thấy ai.',
  communityEmpty: 'Chưa có ai khác ngoài phố. Rủ bạn bè vào chơi nhé!',
  communityCount: (n: number) => `${n} người đang ở ngoài phố, gần nhất trước.`,
  farAway: 'ở xa',
};

const en: typeof vi = {
  timeLocale: 'en-GB',
  channelsAbout: (radius) => `Everyone: anyone in District 1 can read it. Nearby: only the people around you, within ${radius} m.`,
  channelGlobal: 'Everyone',
  channelNearby: 'Nearby',
  writeMessage: 'Write a message',
  chatOffline: 'Connection lost, chat is not available yet.',
  muteTitle: 'Mute and report',
  muteAbout: 'Tap a name in the chat to mute that person, or to report them to the moderators.',
  unmute: 'Unmute',
  justNow: 'just now',
  minutesAgo: (mins) => `${mins} min ago`,
  noNotices: 'No notifications yet.',
  askTitle: 'Ask a Saigonese the way',
  asking: (dest) => `Asking the way to ${dest}. A Saigonese playing now will come and show you.`,
  askCancel: 'Never mind, stop asking',
  askFallback: 'Nobody came last time: Chú Sáu, the xe ôm driver at the roundabout, will show you the way. Or ask again:',
  askPick: 'Ask a Saigonese playing now the way to:',
  askLeft: (left) => `${left} left before asking Chú Sáu.`,
  communityOffline: 'Connection lost, nobody to see yet.',
  communityEmpty: 'Nobody else is out on the street. Invite your friends!',
  communityCount: (n) => (n === 1 ? '1 person out on the street.' : `${n} people out on the street, nearest first.`),
  farAway: 'far away',
};

export const L = catalog({ vi, en });
