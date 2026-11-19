// Text around the Cẩm nang's pages (src/game/notebook/guideBook.ts) and the info card in the street
// (src/game/placeCard.ts). The pages themselves are content: server/content/guide.json.
import { catalog } from './index';

const vi = {
  /** locale for the "checked on" day */
  dateLocale: 'vi-VN',
  // the list
  intro: 'Mỗi công trình có một trang: lịch sử, có gì để xem, mẹo, và cách tới đó ngoài đời. Đi tới gần, hoặc bấm vào tên để đọc.',
  progress: (seen: number, all: number, read: number, pages: number) => `Đã ghé ${seen} / ${all} công trình trong chuyến này · đã đọc ${read} / ${pages} trang`,
  visited: 'Đã ghé',
  read: 'Đã đọc',
  soon: 'Sắp có trang',
  // a page
  built: 'Xây dựng',
  hours: 'Giờ mở cửa',
  ticket: 'Vé',
  history: 'Lịch sử',
  see: 'Có gì để xem',
  tips: 'Mẹo',
  gettingThere: 'Tới đó ngoài đời',
  food: 'Ăn uống gần đây',
  maps: 'Mở trên Google Maps',
  walkThere: 'Dẫn đường tới đây',
  flyThere: 'Bay tới đây',
  stopGuiding: 'Thôi dẫn đường',
  checked: (day: string) => `Kiểm tra lần cuối ngày ${day}. Giờ mở cửa và giá vé có thể đã đổi.`,
  sources: 'Nguồn',
  /** the quest tracker while it shows the way to a landmark */
  trackerTitle: 'Cẩm nang',
  goTo: (place: string) => `Đi tới ${place}`,
};

const en: typeof vi = {
  dateLocale: 'en-GB',
  intro: 'Every landmark has a page: its history, what to see, tips, and how to get there in real life. Walk up to one, or tap its name to read.',
  progress: (seen, all, read, pages) => `Visited ${seen} / ${all} landmarks this trip · read ${read} / ${pages} pages`,
  visited: 'Visited',
  read: 'Read',
  soon: 'Page coming soon',
  built: 'Built',
  hours: 'Opening hours',
  ticket: 'Tickets',
  history: 'History',
  see: 'What to see',
  tips: 'Tips',
  gettingThere: 'Getting there for real',
  food: 'Food and drink nearby',
  maps: 'Open in Google Maps',
  walkThere: 'Show me the way',
  flyThere: 'Fly there',
  stopGuiding: 'Stop showing the way',
  checked: (day) => `Last checked on ${day}. Opening hours and prices may have changed.`,
  sources: 'Sources',
  trackerTitle: 'Guidebook',
  goTo: (place) => `Go to ${place}`,
};

export const L = catalog({ vi, en });
