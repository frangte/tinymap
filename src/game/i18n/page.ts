// The text written into index.html: an element carries data-i18n="key" (its text), data-i18n-title,
// data-i18n-aria (aria-label) or data-i18n-placeholder, and translatePage() fills them in from this catalog, at
// start and after every language switch. Text the game sets from code (the place card, the menu status, the
// chat channel…) has no key here: its module owns it. scripts/i18n-check.mjs checks every key in index.html.
import { catalog, onLangChange } from './index';

const vi = {
  // start menu
  menuEyebrow: 'Sài Gòn · bản thu nhỏ',
  menuLede: 'Sài Gòn vẽ tay, thu nhỏ giữa biển: từ chợ Bến Thành đi Lê Lợi tới Nhà hát, lên Nhà thờ Đức Bà, qua cầu sang Thủ Thiêm, xuống phố lồng đèn Chợ Lớn, bến cảng Khánh Hội, rồi ra biển Vũng Tàu.',
  modePlay: 'Vào game',
  modePlayNote: 'Đi bộ tự do từ cổng chợ Bến Thành',
  modeMap: 'Xem bản đồ',
  modeMapNote: 'Xoay, zoom, bay tới từng công trình',
  language: 'Ngôn ngữ',
  // tool buttons
  sound: 'Âm thanh',
  soundTitle: 'Bật hoặc tắt âm thanh khu vực (tiếng sóng ở bãi biển)',
  ink: 'Nét mực',
  inkTitle: 'Bật hoặc tắt nét mực',
  dress: 'Đổi đồ',
  dressTitle: 'Đổi nhân vật và trang phục',
  toMap: 'Bản đồ',
  toMapTitle: 'Xem toàn bộ bản đồ',
  toPlay: 'Vào game',
  toPlayTitle: 'Đi bộ khắp Sài Gòn',
  /** the switch names the other language, in that language, so anyone can find it */
  otherLanguage: 'English',
  otherLanguageTitle: 'Switch to English',
  // banners
  takeRide: 'Nhận cuốc',
  skip: 'Bỏ qua',
  // dressing panel and the archetype picker
  character: 'Nhân vật',
  chooseCharacter: 'Chọn nhân vật',
  namePlaceholder: 'Khách 123',
  displayName: 'Tên hiển thị',
  gender: 'Giới tính',
  outfit: 'Trang phục',
  hat: 'Nón',
  cancel: 'Huỷ',
  dressDone: 'Vào phố',
  // the arrival card
  yourStory: 'Câu chuyện của bạn',
  todo: 'Việc cần làm',
  start: 'Bắt đầu',
  // conversations
  stopTalking: 'Thôi nói chuyện',
  stopTalkingTitle: 'Thôi nói chuyện (Esc)',
  groupWait: 'Người dẫn nhóm đang chọn câu hỏi…',
  answers: 'Trả lời',
  // chat
  chat: 'Trò chuyện',
  message: 'Tin nhắn',
  send: 'Gửi',
  // the phone
  phone: 'Điện thoại',
  phoneHome: 'Về màn hình chính',
  phoneClose: 'Cất điện thoại',
  // action pad
  emotes: 'Điệu bộ',
  jump: 'Nhảy',
  // map dock
  landmarks: 'Công trình',
  // key hints: the key cap stays, the words after it change
  keyWalk: 'đi',
  keyRun: 'chạy',
  keyJump: 'nhảy',
  keyEmotes: 'điệu bộ',
  keyTalk: 'nói chuyện, lên xe',
  keyPhone: 'điện thoại',
  keyChat: 'chat',
  keyOutfit: 'đổi áo',
  keyMenu: 'menu',
  dragToTurn: 'kéo chuột để xoay',
  scrollToZoom: 'cuộn để zoom',
  touchStoryMove: 'Kéo bên trái để đi, vuốt bên phải để xoay',
  touchStoryPhone: 'Nút tròn góc dưới phải là điện thoại',
  touchWalk: 'Kéo bên trái để đi, đẩy hết để chạy',
  touchTurn: 'Vuốt bên phải để xoay, chụm hai ngón để zoom',
  touchActions: 'Nút mũi tên để nhảy, mặt cười để làm điệu bộ',
  mapHintMouse: 'Kéo để xoay · cuộn để zoom · chuột phải để kéo bản đồ ·',
  mapHintTouch: 'Vuốt để xoay · chụm hai ngón để zoom · kéo hai ngón để di chuyển',
};

const en: typeof vi = {
  menuEyebrow: 'Saigon · in miniature',
  menuLede: 'A hand-drawn Saigon in miniature, set in the sea: from Bến Thành Market along Lê Lợi to the Opera House, up to Notre-Dame Cathedral, across the river to Thủ Thiêm, down to the lantern street of Chợ Lớn and the old port of Khánh Hội, and out to the beaches of Vũng Tàu.',
  modePlay: 'Play',
  modePlayNote: 'Walk around freely from Bến Thành Market',
  modeMap: 'View the map',
  modeMapNote: 'Turn, zoom, fly to each landmark',
  language: 'Language',
  sound: 'Sound',
  soundTitle: 'Turn the area sounds on or off (the waves on the beach)',
  ink: 'Ink lines',
  inkTitle: 'Turn the ink lines on or off',
  dress: 'Clothes',
  dressTitle: 'Change your character and clothes',
  toMap: 'Map',
  toMapTitle: 'See the whole map',
  toPlay: 'Play',
  toPlayTitle: 'Walk around Saigon',
  otherLanguage: 'Tiếng Việt',
  otherLanguageTitle: 'Chuyển sang tiếng Việt',
  takeRide: 'Take the ride',
  skip: 'Skip',
  character: 'Character',
  chooseCharacter: 'Choose your character',
  namePlaceholder: 'Guest 123',
  displayName: 'Display name',
  gender: 'Gender',
  outfit: 'Outfit',
  hat: 'Hat',
  cancel: 'Cancel',
  dressDone: 'Head out',
  yourStory: 'Your story',
  todo: 'What to do',
  start: 'Start',
  stopTalking: 'Leave the conversation',
  stopTalkingTitle: 'Leave the conversation (Esc)',
  groupWait: 'The group leader is choosing a question…',
  answers: 'Answers',
  chat: 'Chat',
  message: 'Message',
  send: 'Send',
  phone: 'Phone',
  phoneHome: 'Back to the home screen',
  phoneClose: 'Put the phone away',
  emotes: 'Emotes',
  jump: 'Jump',
  landmarks: 'Landmarks',
  keyWalk: 'walk',
  keyRun: 'run',
  keyJump: 'jump',
  keyEmotes: 'emotes',
  keyTalk: 'talk, get on a bike',
  keyPhone: 'phone',
  keyChat: 'chat',
  keyOutfit: 'change outfit',
  keyMenu: 'menu',
  dragToTurn: 'drag to look around',
  scrollToZoom: 'scroll to zoom',
  touchStoryMove: 'Drag on the left to walk, swipe on the right to look around',
  touchStoryPhone: 'The round button bottom right is your phone',
  touchWalk: 'Drag on the left to walk, push all the way to run',
  touchTurn: 'Swipe on the right to look around, pinch to zoom',
  touchActions: 'Arrow button to jump, smiley for emotes',
  mapHintMouse: 'Drag to turn · scroll to zoom · right-drag to pan ·',
  mapHintTouch: 'Swipe to turn · pinch to zoom · drag with two fingers to move',
};

export type PageKey = keyof typeof vi;

const L = catalog({ vi, en });

function isKey(k: string | undefined): k is PageKey {
  return k !== undefined && Object.prototype.hasOwnProperty.call(vi, k);
}

/** The text for a data-i18n key; the key itself (and a console note) for one this catalog lacks. */
function text(key: string | undefined, el: Element): string {
  if (isKey(key)) return L[key];
  console.warn(`index.html: no text for data-i18n key "${key}"`, el);
  return key ?? '';
}

const ATTRS: ReadonlyArray<readonly [string, string]> = [
  ['i18nTitle', 'title'],
  ['i18nAria', 'aria-label'],
  ['i18nPlaceholder', 'placeholder'],
];

/** Fill in every keyed element and attribute under `root` in the current language. */
export function translatePage(root: ParentNode = document): void {
  for (const el of root.querySelectorAll<HTMLElement>('[data-i18n]')) el.textContent = text(el.dataset.i18n, el);
  for (const [data, attr] of ATTRS) {
    const sel = `[data-${data.replace(/[A-Z]/g, (c) => `-${c.toLowerCase()}`)}]`;
    for (const el of root.querySelectorAll<HTMLElement>(sel)) el.setAttribute(attr, text(el.dataset[data], el));
  }
}

/** Translate the page now and again after every switch. */
export function startPageTranslation(): void {
  translatePage();
  onLangChange(() => translatePage());
}
