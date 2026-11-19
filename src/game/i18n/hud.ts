// The HUD (hud.ts): the menu's status line, the place card, the multiplayer status and the map stops.
import { catalog } from './index';

const vi = {
  loading: 'Đang dựng Sài Gòn…',
  ready: 'Chọn cách vào thành phố:',
  eyebrowMenu: 'Sài Gòn mini',
  eyebrowMap: 'Sài Gòn mini · bản đồ',
  eyebrowPlay: 'Sài Gòn mini · đang ở',
  /** where nothing on the map is named (places.json has an area for every district) */
  district: 'Sài Gòn',
  city: 'TP. Hồ Chí Minh',
  overview: 'Toàn cảnh',
  onlineMany: (n: number) => `${n} người đang ở Sài Gòn`,
  onlineAlone: 'Chỉ có bạn ở Sài Gòn',
  connecting: 'Đang kết nối…',
  offline: 'Mất kết nối, đang thử lại…',
  full: 'Phố đông quá, chờ chút…',
  replaced: 'Bạn đang chơi ở tab khác. Tải lại trang để chơi ở đây.',
  banned: 'Tài khoản đang bị khóa vì vi phạm nội quy.',
  kicked: 'Người điều hành đã mời bạn ra. Tải lại trang để vào lại.',
};

const en: typeof vi = {
  loading: 'Building Saigon…',
  ready: 'Choose how to enter the city:',
  eyebrowMenu: 'Saigon mini',
  eyebrowMap: 'Saigon mini · map',
  eyebrowPlay: 'Saigon mini · you are at',
  district: 'Saigon',
  city: 'Ho Chi Minh City',
  overview: 'Overview',
  onlineMany: (n) => `${n} people in Saigon`,
  onlineAlone: 'Only you in Saigon',
  connecting: 'Connecting…',
  offline: 'Connection lost, trying again…',
  full: 'The street is full, wait a moment…',
  replaced: 'You are playing in another tab. Reload the page to play here.',
  banned: 'This account is locked for breaking the rules.',
  kicked: 'A moderator asked you to leave. Reload the page to come back.',
};

export const L = catalog({ vi, en });
