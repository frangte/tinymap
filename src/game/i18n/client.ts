// Text of src/net/client.ts: what the connection itself tells the player.
import { catalog } from './index';

const vi = {
  reported: 'Đã gửi báo cáo cho người điều hành. Cảm ơn bạn!',
};

const en: typeof vi = {
  reported: 'Your report went to the moderators. Thank you!',
};

export const L = catalog({ vi, en });
