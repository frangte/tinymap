import { badge, h } from './dom';
import { hashQuest, questTitle, type Model, type QuestNode } from './model';
import { originColor, originName, originShort, state, tx } from './state';
import type { Rewards } from './types';

/** Questline names for the column headings (the one place to name a questline; `lineName` falls back to a readable id). */
const LINE_NAMES: Record<string, string> = {
  chuong_0_que: 'Chương 0 · Từ quê lên',
  chuong_0_nuoc_ngoai: 'Chương 0 · Người nước ngoài',
  chuong_1: 'Chương 1 · Điện thoại và Má',
  song_o_pho: 'Sống ở phố',
  chuong_3: 'Chương 3 · Cầm lái',
  tay_lai: 'Tay lái',
  khach_crab: 'Khách đi Crab',
  viec_vat: 'Việc vặt',
  daily: 'Hằng ngày',
  thanh_tich: 'Thành tích',
};

/** A questline's heading: the name above, else the id read as words ("chuong_4" → "Chương 4"). */
export function lineName(id: string): string {
  const known = LINE_NAMES[id];
  if (known) return known;
  const chapter = /^chuong_(\d+)(?:_(.+))?$/.exec(id);
  if (chapter) return `Chương ${chapter[1]}${chapter[2] ? ` · ${chapter[2].replace(/_/g, ' ')}` : ''}`;
  const words = id.replace(/_/g, ' ');
  return words.charAt(0).toUpperCase() + words.slice(1);
}

/** What each objective type means, in a few words. */
export const OBJECTIVE_NAMES: Record<string, string> = {
  dialogue_signal: 'chọn câu trả lời (tín hiệu)',
  talk_npc: 'nói chuyện với NPC',
  reach_place: 'đi tới nơi',
  stay_place: 'ở lại một nơi',
  buy_item: 'mua đồ',
  emote_at: 'làm điệu bộ tại nơi',
  greet: 'chào người khác',
  give_way: 'chỉ đường cho khách',
  pass_test: 'thi',
  mount: 'lên xe',
  ride_through: 'chạy xe qua nơi',
  ride_distance: 'chạy xe đủ quãng đường',
  trip: 'đi một cuốc Crab',
  errand: 'việc vặt (nhiều điểm)',
};

/** The origins a quest is offered to, as small coloured labels; a quest for everyone gets one grey label. */
export function originChips(model: Model, q: QuestNode): HTMLElement {
  const wrap = h('span', { class: 'chips' });
  if (q.origins.size === model.origins.length) {
    wrap.append(badge('mọi người', 'origin-all', 'Mọi xuất phát đều được mời'));
    return wrap;
  }
  for (const id of q.origins) {
    const chip = badge(originShort(model, id), 'origin', `Được mời: ${originName(model, id)}`);
    chip.style.setProperty('--c', originColor(model, id));
    wrap.append(chip);
  }
  return wrap;
}

export function rewardsSummary(r: Rewards | undefined): string[] {
  if (!r) return [];
  const out: string[] = [];
  if (r.xu) out.push(`${r.xu.toLocaleString('vi-VN')}đ`);
  if (r.titles?.length) out.push(`danh hiệu ×${r.titles.length}`);
  if (r.stamps?.length) out.push(`tem ×${r.stamps.length}`);
  if (r.capability) out.push(`+${r.capability}`);
  if (r.flags?.length) out.push(`cờ ×${r.flags.length}`);
  return out;
}

/** Whether a quest fits the search box (id or either title). */
export function matchesSearch(q: QuestNode, search: string): boolean {
  const s = search.trim().toLowerCase();
  if (!s) return true;
  return [q.id, q.def.title.vi, q.def.title.en].some((t) => t?.toLowerCase().includes(s));
}

export interface CardOptions {
  selected: boolean;
}

/** A quest card of the overview. */
export function questCard(model: Model, q: QuestNode, options: CardOptions): HTMLElement {
  const def = q.def;
  const dim = (state.origin !== '' && !q.effective.has(state.origin)) || !matchesSearch(q, state.search);
  const only = q.origins.size === 1 ? [...q.origins][0] : '';
  const card = h('a', { class: `card kind-${q.kind}${dim ? ' dim' : ''}${options.selected ? ' selected' : ''}`, href: hashQuest(q.id), 'data-quest': q.id });
  if (only) card.style.setProperty('--c', originColor(model, only));
  const meta = h('div', { class: 'badges' }, originChips(model, q));
  for (const c of def.requires_caps ?? []) meta.append(badge(`cần ${c}`, 'cap', 'Chỉ mời khi đang có quyền này'));
  for (const c of def.unless_caps ?? []) meta.append(badge(`trừ ${c}`, 'cap neg', 'Không mời khi đã có quyền này'));
  for (const f of def.requires_flags ?? []) meta.append(badge(`cờ ${f}`, 'flag', 'Cần cờ này'));
  for (const f of def.unless_flags ?? []) meta.append(badge(`không cờ ${f}`, 'flag neg', 'Bị chặn bởi cờ này'));
  if (def.repeat) meta.append(badge(def.job ? 'việc vặt' : 'hằng ngày', 'repeat'));
  if (q.kind === 'achievement') meta.append(badge('thành tích', 'repeat'));
  const foot = h('div', { class: 'foot' });
  const objectives = def.objectives.length;
  foot.append(h('span', {}, `${objectives} mục tiêu`));
  if (def.outcomes?.length) foot.append(badge(`${def.outcomes.length} kết cục`, 'outcome'));
  const rewards = rewardsSummary(def.rewards);
  if (rewards.length) foot.append(h('span', { class: 'rewards' }, rewards.join(' · ')));
  card.append(h('div', { class: 'title' }, tx(def.title, questTitle(def))), h('div', { class: 'id' }, q.id), meta, foot);
  return card;
}
