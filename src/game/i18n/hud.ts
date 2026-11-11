// The HUD (hud.ts): the menu's status line, the place card, the multiplayer status and the map stops.
import { catalog } from './index';

const vi = {
  loading: 'Đang dựng Quận 1…',
  ready: 'Chọn cách vào Quận 1:',
  eyebrowMenu: 'Sài Gòn Q1',
  eyebrowMap: 'Sài Gòn Q1 · bản đồ',
  eyebrowPlay: 'Sài Gòn Q1 · đang ở',
  district: 'Quận 1',
  city: 'Sài Gòn',
  overview: 'Toàn cảnh',
  onlineMany: (n: number) => `${n} người đang ở Q1`,
  onlineAlone: 'Chỉ có bạn ở Q1',
  connecting: 'Đang kết nối…',
  offline: 'Mất kết nối, đang thử lại…',
  full: 'Phố đông quá, chờ chút…',
  replaced: 'Bạn đang chơi ở tab khác. Tải lại trang để chơi ở đây.',
  banned: 'Tài khoản đang bị khóa vì vi phạm nội quy.',
  kicked: 'Người điều hành đã mời bạn ra. Tải lại trang để vào lại.',
};

const en: typeof vi = {
  loading: 'Building District 1…',
  ready: 'Choose how to enter District 1:',
  eyebrowMenu: 'Sài Gòn Q1',
  eyebrowMap: 'Sài Gòn Q1 · map',
  eyebrowPlay: 'Sài Gòn Q1 · you are at',
  district: 'District 1',
  city: 'Saigon',
  overview: 'Overview',
  onlineMany: (n) => `${n} people in District 1`,
  onlineAlone: 'Only you in District 1',
  connecting: 'Connecting…',
  offline: 'Connection lost, trying again…',
  full: 'The street is full, wait a moment…',
  replaced: 'You are playing in another tab. Reload the page to play here.',
  banned: 'This account is locked for breaking the rules.',
  kicked: 'A moderator asked you to leave. Reload the page to come back.',
};

export const L = catalog({ vi, en });
