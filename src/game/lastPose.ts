// Where the player stood when the page was last closed, in this browser, so the next visit starts there before
// the server has answered. The server keeps its own copy (with the account) and has the last word: its welcome
// moves the player if the two disagree.
// v2: Sài Gòn mini (layout v8); a spot saved on the v4 island means nothing on it
const KEY = 'sg.pose.v2';

export interface SavedPose {
  x: number;
  z: number;
  h: number;
}

export function loadPose(): SavedPose | null {
  try {
    const v: unknown = JSON.parse(localStorage.getItem(KEY) ?? 'null');
    if (typeof v !== 'object' || v === null) return null;
    const { x, z, h } = v as Record<string, unknown>;
    return typeof x === 'number' && typeof z === 'number' && typeof h === 'number' && [x, z, h].every(Number.isFinite) ? { x, z, h } : null;
  } catch {
    return null; // storage blocked (a private window): start at home
  }
}

export function savePose(x: number, z: number, h: number): void {
  try {
    localStorage.setItem(KEY, JSON.stringify({ x: Math.round(x * 100) / 100, z: Math.round(z * 100) / 100, h: Math.round(h * 1000) / 1000 }));
  } catch {
    // storage full or blocked: the server still has it, for players with an account
  }
}
