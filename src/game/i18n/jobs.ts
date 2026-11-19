// Text of the odd jobs' goal lines (src/game/jobs.ts): what the tracker and the quest cards say about a job before
// it is taken, while it runs against the clock, and when the time is up.
import { catalog } from './index';

const vi = {
  /** the job is not taken yet: where to go to take it */
  take: (giver: string) => `Nhận việc: nói chuyện với ${giver}`,
  /** the time is up: the run failed and can be taken again */
  late: (giver: string) => `Hết giờ! Quay lại ${giver} để nhận việc lại`,
  /** the clock of a run (m:ss) */
  left: (clock: string) => `còn ${clock}`,
  /** the job is done for today */
  doneToday: 'Xong hôm nay',
  /** the section of the quest list */
  title: 'Việc vặt',
  intro: 'Mỗi việc làm được một lần mỗi ngày, ai có điện thoại cũng nhận được. Nói chuyện với người giao việc để nhận.',
};

const en: typeof vi = {
  take: (giver) => `Take the job: talk to ${giver}`,
  late: (giver) => `Out of time! Go back to ${giver} to take the job again`,
  left: (clock) => `${clock} left`,
  doneToday: 'Done for today',
  title: 'Odd jobs',
  intro: 'Each job can be done once a day, and anyone with a phone can take them. Talk to whoever gives the job to take it.',
};

export const L = catalog({ vi, en });
