// Text of the start menu's "install the app" button and the steps it unfolds (src/install.ts). The names of the
// browsers' own menu items are quoted as each system shows them in that language.
import { catalog } from './index';

const vi = {
  button: 'Cài game vào điện thoại',
  note: 'Mở toàn màn hình, mở ngay, chơi được khi không có mạng',
  stepsLabel: 'Cách cài',
  iosShare: 'Bấm nút Chia sẻ (ô vuông có mũi tên lên) ở thanh dưới hoặc góc trên. Trên iOS mới, nó nằm trong nút ⋯.',
  iosAdd: 'Kéo xuống, chọn "Thêm vào MH chính".',
  iosDone: 'Bấm "Thêm". Từ giờ mở game bằng biểu tượng trên màn hình chính.',
  androidMenu: 'Mở menu của trình duyệt: nút ⋮ ở góc trên, hoặc ≡ ở thanh dưới.',
  androidAdd: 'Chọn "Cài đặt ứng dụng" hoặc "Thêm vào màn hình chính".',
  androidDone: 'Xác nhận. Từ giờ mở game bằng biểu tượng trên màn hình chính.',
  inAppLead: 'Trang đang mở trong một ứng dụng khác (Zalo, Facebook…), ở đây chưa cài được.',
  inAppMenu: 'Bấm nút ⋯ hoặc ⋮ ở góc màn hình.',
  inAppOpen: 'Chọn "Mở bằng trình duyệt" (Safari hoặc Chrome).',
  inAppAgain: (button: string) => `Trong trình duyệt, bấm lại "${button}".`,
};

const en: typeof vi = {
  button: 'Install on your phone',
  note: 'Full screen, opens at once, plays offline',
  stepsLabel: 'How to install',
  iosShare: 'Tap the Share button (a square with an arrow up), in the bottom bar or at the top. In newer iOS it is under the ⋯ button.',
  iosAdd: 'Scroll down and pick "Add to Home Screen".',
  iosDone: 'Tap "Add". From now on, open the game from its icon on the home screen.',
  androidMenu: 'Open the browser menu: ⋮ at the top, or ≡ in the bottom bar.',
  androidAdd: 'Pick "Install app" or "Add to Home screen".',
  androidDone: 'Confirm. From now on, open the game from its icon on the home screen.',
  inAppLead: 'This page is open inside another app (Zalo, Facebook…), which cannot install it.',
  inAppMenu: 'Tap the ⋯ or ⋮ button in a corner of the screen.',
  inAppOpen: 'Pick "Open in browser" (Safari or Chrome).',
  inAppAgain: (button) => `In the browser, tap "${button}" again.`,
};

export const L = catalog({ vi, en });
