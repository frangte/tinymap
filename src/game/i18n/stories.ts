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
  newcomer: {
    opening: 'Bạn vừa xuống xe ở bến xe Chợ Lớn: ba lô trên vai, ít tiền trong túi, và cả Sài Gòn chưa biết mặt. Má dặn lên tới nơi thì mua cái điện thoại mới gọi về.',
    todo: [
      'Đi bộ cho quen đường: tới công trình nào thì thẻ thông tin hiện dưới tên địa điểm. Mở Sổ tay (phím B) để xem bản đồ.',
      'Vào Quận 1, chào hỏi người quanh phố và nghe họ kể chuyện.',
      'Việc đầu tiên: mua một cái điện thoại để gọi về cho má.',
      'Rồi lập nghiệp: tìm chỗ ở, kiếm việc làm. Chạy Crab cũng là một nghề.',
    ],
    tip: 'Nhiệm vụ chỉ gợi ý, không ai bắt bạn làm. Chưa có điện thoại bạn vẫn đi khắp phố và đọc hết Sổ tay được. Nhấn Enter để chat với người quanh bạn.',
  },
  citizen: {
    opening: 'Bạn sinh ra và lớn lên ở dãy nhà phố Lê Lai: nhà ở ngay đây, hàng xóm ai cũng quen, góc phố nào cũng thuộc. Điện thoại cũ vừa hư, má cho ít tiền mua cái mới.',
    todo: [
      'Uống cà phê sáng ở quán Chú Tư, rồi chào hai người quanh phố.',
      'Mua điện thoại mới, rồi nghe chuyện khu phố từ ba người hàng xóm.',
      'Làm thổ địa: khi du khách hỏi đường, một lời gọi hiện lên trên điện thoại. Đi tới chỗ họ và chỉ đường; mỗi lần giúp, uy tín khu phố tăng.',
    ],
    tip: 'Nhiệm vụ chỉ gợi ý, không ai bắt bạn làm. Ai cũng đi khắp phố và đọc Sổ tay (phím B) được từ phút đầu. Nhấn Enter để chat với người quanh bạn.',
  },
  traveler: {
    opening: 'Bạn vừa xuống xe buýt ở phố đi bộ Bùi Viện, lần đầu tới Sài Gòn: ba lô trên vai, ít tiền trong túi, đèn neon và mùi đồ ăn khắp nơi.',
    todo: [
      'Đi dạo khắp phố: tới công trình nào thì thẻ thông tin hiện dưới tên địa điểm, bấm Đọc thêm để biết lịch sử và mẹo.',
      'Mở Sổ tay (phím B): Cẩm nang, bản đồ, nhiệm vụ; nhờ dẫn đường tới nơi bạn muốn.',
      'Nói chuyện với người địa phương để học vài câu tiếng Việt, và ăn thử món ngon, như ly cà phê sữa đá ở quán Chú Tư trên Lê Lai.',
      'Mua một cái điện thoại để đặt Crab và hỏi đường.',
    ],
    tip: 'Nhiệm vụ chỉ gợi ý, không ai bắt bạn làm. Chưa có điện thoại bạn vẫn đi khắp phố và đọc hết Sổ tay được.',
  },
} satisfies Record<string, Story>;

const fallbackVi: Story = {
  opening: 'Bạn đang ở Sài Gòn, một thành phố vẽ tay giữa biển: Quận 1, Thủ Thiêm, Chợ Lớn, Khánh Hội và biển Vũng Tàu.',
  todo: ['Đi bộ khắp phố: mỗi công trình bạn tới sẽ tự giới thiệu.', 'Bắt chuyện với người quanh bạn.'],
  tip: 'Nhiệm vụ hiện ở góc trên trái; điện thoại ở góc dưới phải.',
};

/** the same archetypes as `vi` (tsc checks it) */
const en: typeof vi = {
  newcomer: {
    opening: 'You have just stepped off the bus at Chợ Lớn station: a backpack on your shoulders, little money in your pocket, and all of Saigon still strange to you. Mum said to buy a new phone once you got there, and call home.',
    todo: [
      'Walk around to learn the streets: at a landmark an info card shows under the place name. Open the Notebook (B) for the map.',
      'Head into District 1, say hello to people on the street and listen to their stories.',
      'First job: buy a phone so you can call Mum.',
      'Then make a life here: find a place to stay and some work. Riding a Crab is a living too.',
    ],
    tip: 'Quests only suggest, nobody makes you do them. Without a phone you can still walk all over town and read the whole Notebook. Press Enter to chat with the people around you.',
  },
  citizen: {
    opening: 'You were born and raised on the Lê Lai shophouse row: your home is right here, you know every neighbour and every street corner. Your old phone just broke, and Mum gave you a little money for a new one.',
    todo: [
      'Have your morning coffee at Chú Tư’s, then say hello to two people around the street.',
      'Buy a new phone, then hear the neighbourhood news from three neighbours.',
      'Be the thổ địa, the neighbourhood’s local guide: when a traveller asks the way, a call pops up on your phone. Walk over and show them; every time you help, your standing in the neighbourhood grows.',
    ],
    tip: 'Quests only suggest, nobody makes you do them. Everyone can walk all over town and read the Notebook (B) from the first minute. Press Enter to chat with the people around you.',
  },
  traveler: {
    opening: 'You have just stepped off the bus on Bùi Viện walking street, your first time in Saigon: a backpack on your shoulders, a little money in your pocket, neon lights and the smell of street food everywhere.',
    todo: [
      'Walk around town: at a landmark an info card shows under the place name; tap Read more for its history and tips.',
      'Open the Notebook (B): the guidebook, the map, your quests; ask it to show you the way to anywhere.',
      'Chat with the locals to pick up some Vietnamese, and try the food, like an iced milk coffee (cà phê sữa đá) at Chú Tư’s on Lê Lai.',
      'Buy a phone so you can book a Crab and ask the way.',
    ],
    tip: 'Quests only suggest, nobody makes you do them. Without a phone you can still walk all over town and read the whole Notebook.',
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
