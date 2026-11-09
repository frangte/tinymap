// The name a new player gets until they pick one (profile.ts). Other players see it as written.
import { catalog } from './index';

const vi = { guestName: (n: number) => `Khách ${n}` };
const en: typeof vi = { guestName: (n) => `Guest ${n}` };

export const L = catalog({ vi, en });
