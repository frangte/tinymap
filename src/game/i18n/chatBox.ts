// Text of src/game/chatBox.ts: the chat box, its channel button, its notes and the menu on a player's name.
import { catalog } from './index';

const vi = {
  chGlobal: 'Cả phố',
  chNearby: 'Gần đây',
  channelAria: (ch: string) => `Gửi tới: ${ch}. Bấm để đổi`,
  placeholder: 'Nói gì đó…',
  offline: 'Mất kết nối, chưa chat được…',
  rate: 'Chậm lại chút…',
  slow: 'Kênh Cả phố đang đông: vài giây mới gửi được một dòng.',
  repeat: 'Dòng này vừa gửi rồi.',
  newPlayer: 'Chơi thêm vài phút nữa mới chat Cả phố được. Kênh Gần đây vẫn mở.',
  muted: 'Bạn đang bị tắt chat.',
  /** another player's name in the log, for screen readers */
  personAria: (name: string) => `${name}: tắt tiếng hoặc báo cáo`,
  /** the mark on a line only the players around heard */
  near: 'gần',
  mute: (name: string) => `Tắt tiếng ${name}`,
  mutedNote: (name: string) => `Đã tắt tiếng ${name}. Bật lại trong app Trò chuyện.`,
  report: 'Báo cáo',
  cancel: 'Thôi',
};

const en: typeof vi = {
  chGlobal: 'Everyone',
  chNearby: 'Nearby',
  channelAria: (ch) => `Send to: ${ch}. Press to change`,
  placeholder: 'Say something…',
  offline: "Offline, can't chat yet…",
  rate: 'Slow down a little…',
  slow: 'Everyone is busy: one line every few seconds.',
  repeat: 'You just sent that line.',
  newPlayer: 'Play a few more minutes before chatting to Everyone. Nearby is open.',
  muted: 'Your chat is muted.',
  personAria: (name) => `${name}: mute or report`,
  near: 'near',
  mute: (name) => `Mute ${name}`,
  mutedNote: (name) => `${name} is muted. Unmute them in the Chat app.`,
  report: 'Report',
  cancel: 'Never mind',
};

export const L = catalog({ vi, en });
