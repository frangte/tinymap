// What each archetype is told on arrival (storyCard.ts): who they are, what to do first, and the one tip that
// gets them going. Game-side text only, so it is not in server/content/archetypes.json (the server never reads it);
// the quests it points at are server/content/quests.json and the design in docs/design/01-gameplay.md.

export interface Story {
  /** where the player is and how they got there */
  opening: string;
  /** what to do, in order */
  todo: readonly string[];
  /** the one thing to know about the tools */
  tip: string;
}

const STORIES: Record<string, Story> = {
  traveler: {
    opening: 'Bạn vừa xuống xe ở bùng binh Quách Thị Trang, lần đầu tới Sài Gòn: balo trên vai, chưa quen ai, và cả Quận 1 ở ngay trước mặt.',
    todo: [
      'Học vài câu chào với Cô Ba ở cổng nam chợ Bến Thành.',
      'Gọi một ly cà phê sữa đá ở quán Chú Tư trên Lê Lai.',
      'Ghé Nhà thờ Đức Bà, Bưu điện và Nhà hát; ăn vặt bên Hồ Con Rùa.',
      'Hỏi đường một người Sài Gòn đang chơi, rồi làm bài kiểm tra cuối để thành Người Sài Gòn chính hiệu.',
    ],
    tip: 'Nhiệm vụ tự bắt đầu và hiện ở góc trên trái. Điện thoại có Cẩm nang các công trình của Quận 1.',
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
};

const FALLBACK: Story = {
  opening: 'Bạn đang ở Quận 1, Sài Gòn, trên một hòn đảo vẽ tay.',
  todo: ['Đi bộ khắp phố: mỗi công trình bạn tới sẽ tự giới thiệu.', 'Bắt chuyện với người quanh bạn.'],
  tip: 'Nhiệm vụ hiện ở góc trên trái; điện thoại ở góc dưới phải.',
};

/** The story for an archetype id; a generic one for ids this build does not know. */
export function storyFor(archetype: string): Story {
  return STORIES[archetype] ?? FALLBACK;
}
