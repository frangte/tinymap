// Where the player stood when the page was last closed, in this browser, so the next visit starts there before
// the server has answered. The server keeps its own copy (with the account) and has the last word: its welcome
// moves the player if the two disagree.
// v2: Sài Gòn mini (layout v8); a spot saved on the v4 island means nothing on it. The height came later: a spot
// saved without one is not used (on a bridge or the hill, the ground under it is not where the player stood).
const KEY = 'sg.pose.v2';

export interface SavedPose {
  x: number;
  /** the height of the floor the player stood on; undefined for a spot saved before heights were kept */
  y?: number;
  z: number;
  h: number;
}

export function loadPose(): SavedPose | null {
  try {
    const v: unknown = JSON.parse(localStorage.getItem(KEY) ?? 'null');
    if (typeof v !== 'object' || v === null) return null;
    const { x, y, z, h } = v as Record<string, unknown>;
    if (!(typeof x === 'number' && typeof z === 'number' && typeof h === 'number' && [x, z, h].every(Number.isFinite))) return null;
    return typeof y === 'number' && Number.isFinite(y) ? { x, y, z, h } : { x, z, h };
  } catch {
    return null; // storage blocked (a private window): start at home
  }
}

export function savePose(at: { x: number; y: number; z: number }, h: number): void {
  const r = (v: number): number => Math.round(v * 100) / 100;
  try {
    localStorage.setItem(KEY, JSON.stringify({ x: r(at.x), y: r(at.y), z: r(at.z), h: Math.round(h * 1000) / 1000 }));
  } catch {
    // storage full or blocked: the server still has it, for players with an account
  }
}
