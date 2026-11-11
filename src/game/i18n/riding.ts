// Text of riding.ts: getting on and off the motorbike (the action button and the notices).
import { catalog } from './index';

const vi = {
  getOff: 'Xuống xe',
  gettingOn: 'Đang lên xe…',
  getOn: 'Lên xe',
  mountFailed: 'Không lên xe được, thử lại nhé.',
  mountRefused: 'Không lên xe được: lại gần xe của bạn hơn nhé.',
};

const en: typeof vi = {
  getOff: 'Get off',
  gettingOn: 'Getting on…',
  getOn: 'Get on',
  mountFailed: "Couldn't get on, please try again.",
  mountRefused: "Couldn't get on: get closer to your bike.",
};

export const L = catalog({ vi, en });
