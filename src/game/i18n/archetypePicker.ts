// "Bạn từ đâu tới?", the origin cards (archetypePicker.ts).
import { catalog } from './index';

const vi = {
  titleFirst: 'Bạn từ đâu tới?',
  titleChange: 'Đổi vai',
  ledeFirst: 'Ai cũng đi dạo khắp phố và đọc cả Sổ tay từ phút đầu. Chọn xuất phát của bạn: mỗi người đi một con đường. Muốn đổi sau này thì mở Sổ tay (phím B), mục Nhân vật.',
  ledeChange: 'Đổi vai ngay tại chỗ, mặc đồ của vai mới. Nhiệm vụ, danh hiệu và ví vẫn giữ nguyên. Nghề tài xế Crab sau này sẽ là nghề kiếm được trong game; giờ bạn thử trước.',
  current: 'Vai hiện tại',
};

const en: typeof vi = {
  titleFirst: 'Where are you from?',
  titleChange: 'Change role',
  ledeFirst: 'Everyone can walk all over town and read the whole Notebook from the first minute. Pick where you start: each one has its own road. To change later, open the Notebook (B), Character section.',
  ledeChange: 'Change role right here and put on the new role’s clothes. Your quests, titles and wallet stay as they are. Crab driving will later be a job you earn in the game; for now you can try it.',
  current: 'Your role now',
};

export const L = catalog({ vi, en });
