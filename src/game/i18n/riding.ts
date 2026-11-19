// Text of riding.ts: getting on and off the motorbike (the action button and the notices).
import { catalog } from './index';

const vi = {
  getOff: 'Xuống xe',
  gettingOn: 'Đang lên xe…',
  getOn: 'Lên xe',
  mountFailed: 'Không lên xe được, thử lại nhé.',
  mountRefused: 'Không lên xe được: lại gần xe của bạn hơn nhé. Chưa có bằng A1 thì cũng chưa chạy được, ra Thủ Thiêm gặp thầy Mười.',
};

const en: typeof vi = {
  getOff: 'Get off',
  gettingOn: 'Getting on…',
  getOn: 'Get on',
  mountFailed: "Couldn't get on, please try again.",
  mountRefused: "Couldn't get on: get closer to your bike. Without an A1 licence you cannot ride it either: see Thầy Mười in Thủ Thiêm.",
};

export const L = catalog({ vi, en });
