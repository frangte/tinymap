// Text of the licence practical (src/game/driveTest.ts): the tracker's line and what is said when it ends.
import { catalog, money } from './index';

/** the retake fee (server/content/items.json, thi_lai_a1), as the examiner says it */
const RETAKE = 50_000;

const vi = {
  title: 'Thi thực hành A1',
  lent: 'Xe tập lái của thầy Mười đậu bên cạnh bạn. Lên xe (E) là bắt đầu tính giờ, đừng xuống xe nghen!',
  getOn: 'Lên xe tập lái bên cạnh bạn (E)',
  ride: (n: number, of: number, left: string) => `Điểm ${n}/${of} · còn ${left}`,
  passed: 'Đậu thực hành rồi! Bằng A1 là của bạn. Giờ đi mua xe nào.',
  failed: (reason: string) => {
    const why = reason === 'time' ? 'Hết giờ rồi.' : reason === 'off' ? 'Bạn xuống xe giữa chừng nên rớt.' : 'Bạn không lên xe nên thầy lấy xe lại.';
    return `${why} Thi lại thì đóng lệ phí thi lại ${money(RETAKE)} với thầy Mười.`;
  },
};

const en: typeof vi = {
  title: 'A1 practical',
  lent: "Thầy Mười's school bike is parked beside you. Get on (E) and the clock starts: do not get off!",
  getOn: 'Get on the school bike beside you (E)',
  ride: (n, of, left) => `Checkpoint ${n}/${of} · ${left} left`,
  passed: 'Practical passed! The A1 licence is yours. Now go and buy a bike.',
  failed: (reason) => {
    const why = reason === 'time' ? 'Time is up.' : reason === 'off' ? 'You got off halfway, so you failed.' : 'You never got on, so he took the bike back.';
    return `${why} To try again, pay Thầy Mười the ${money(RETAKE)} retake fee.`;
  },
};

export const L = catalog({ vi, en });
