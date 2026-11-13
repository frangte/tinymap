// The arrival stories (stories.ts) in Vietnamese and English. The Vietnamese is the source; the English follows it
// line for line, todo item for todo item.
import type { Lang } from './index';

export interface Story {
  /** where the player is and how they got there */
  opening: string;
  /** what to do, in order */
  todo: readonly string[];
  /** the one thing to know about the tools */
  tip: string;
}

const vi = {
  traveler: {
    opening: 'Bạn vừa xuống xe ở bùng binh Quách Thị Trang, lần đầu tới Sài Gòn: balo trên vai, chưa quen ai, và cả Quận 1 ở ngay trước mặt.',
    todo: [
      'Học vài câu chào với Cô Ba ở cổng nam chợ Bến Thành.',
      'Gọi một ly cà phê sữa đá ở quán Chú Tư trên Lê Lai.',
      'Ghé Nhà thờ Đức Bà, Bưu điện và Nhà hát; ăn vặt bên Hồ Con Rùa.',
      'Hỏi đường một người Sài Gòn đang chơi, rồi làm bài kiểm tra cuối để thành Người Sài Gòn chính hiệu.',
    ],
    tip: 'Nhiệm vụ tự bắt đầu và hiện ở góc trên trái. Điện thoại có Cẩm nang các công trình khắp Sài Gòn.',
  },
  citizen: {
    opening: 'Nhà bạn ở dãy nhà phố Lê Lai. Sáng nào cũng cà phê quán Chú Tư, hàng xóm ai cũng quen, góc phố nào cũng thuộc.',
    todo: [
      'Uống cà phê sáng ở quán Chú Tư, rồi chào hai người quanh phố.',
      'Nghe chuyện khu phố từ ba người hàng xóm.',
      'Khi du khách hỏi đường, một lời gọi hiện lên: đi tới chỗ họ và chỉ đường. Mỗi lần giúp, uy tín khu phố tăng.',
    ],
    tip: 'Điện thoại có app Hàng Xóm: ai đang ở ngoài phố lúc này. Nhấn Enter để chat với người quanh bạn.',
  },
  driver: {
    opening: 'Áo khoác vàng, mũ bảo hiểm, chiếc xe số đậu ngay cạnh bạn trên đường Hàm Nghi. Bạn thuộc từng con đường Quận 1.',
    todo: [
      'Lên xe và chạy thử một vòng.',
      'Chạy thuộc Lê Lợi, Đồng Khởi rồi Hàm Nghi qua các điểm trên đường.',
      'Bật nhận cuốc trong app Crab Driver và chở khách khắp phố.',
    ],
    tip: 'Nhấn E để lên hoặc xuống xe. Cuốc xe mới hiện ở giữa màn hình: nhận nhanh, tài xế nào nhận trước thì được.',
  },
} satisfies Record<string, Story>;

const fallbackVi: Story = {
  opening: 'Bạn đang ở Sài Gòn, một thành phố vẽ tay giữa biển: Quận 1, Thủ Thiêm, Chợ Lớn, Khánh Hội và biển Vũng Tàu.',
  todo: ['Đi bộ khắp phố: mỗi công trình bạn tới sẽ tự giới thiệu.', 'Bắt chuyện với người quanh bạn.'],
  tip: 'Nhiệm vụ hiện ở góc trên trái; điện thoại ở góc dưới phải.',
};

/** the same archetypes as `vi` (tsc checks it) */
const en: typeof vi = {
  traveler: {
    opening: 'You have just stepped off at the Quách Thị Trang roundabout, your first time in Saigon: a backpack on your shoulders, nobody you know yet, and all of District 1 right in front of you.',
    todo: [
      'Learn a few greetings from Cô Ba at the south gate of Bến Thành Market.',
      'Order an iced milk coffee (cà phê sữa đá) at Chú Tư’s café on Lê Lai.',
      'Visit Notre-Dame Cathedral, the Post Office and the Opera House; grab a snack by Turtle Lake.',
      'Ask a Saigonese player the way, then take the final test to become a True Saigonese.',
    ],
    tip: 'Quests start on their own and show in the top left corner. Your phone has a Guidebook to the landmarks all over Saigon.',
  },
  citizen: {
    opening: 'You live in the shophouses on Lê Lai. Coffee at Chú Tư’s every morning; you know every neighbour and every street corner.',
    todo: [
      'Have your morning coffee at Chú Tư’s, then say hello to two people around the street.',
      'Hear the neighbourhood news from three neighbours.',
      'When a traveller asks the way, a call pops up: walk over and show them. Every time you help, your standing in the neighbourhood grows.',
    ],
    tip: 'The Neighbours app on your phone shows who is out on the street right now. Press Enter to chat with the people around you.',
  },
  driver: {
    opening: 'Yellow jacket, helmet, and your motorbike parked right beside you on Hàm Nghi Street. You know every street in District 1.',
    todo: [
      'Get on the bike and take it for a spin.',
      'Learn Lê Lợi, Đồng Khởi and Hàm Nghi by riding past the points along each street.',
      'Go on duty in the Crab Driver app and carry passengers all over town.',
    ],
    tip: 'Press E to get on or off the bike. New rides pop up in the middle of the screen: be quick, the first driver to take one gets it.',
  },
};

const fallbackEn: Story = {
  opening: 'You are in Saigon, a hand-drawn city in the sea: District 1, Thủ Thiêm, Chợ Lớn, Khánh Hội and the beaches of Vũng Tàu.',
  todo: ['Walk the streets: every landmark you reach introduces itself.', 'Strike up a chat with the people around you.'],
  tip: 'Quests show in the top left; your phone is in the bottom right.',
};

/** every story in each language, and the one for archetypes this build does not know */
export const STORIES: Readonly<Record<Lang, { byArchetype: Readonly<Record<string, Story>>; fallback: Story }>> = {
  vi: { byArchetype: vi, fallback: fallbackVi },
  en: { byArchetype: en, fallback: fallbackEn },
};
