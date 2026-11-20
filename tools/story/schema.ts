/**
 * What the content may hold, for the editor's forms: the field order the files use (a field the editor adds goes
 * where the existing entries have it), the limits the server enforces (server/content/*.go), and the objective types
 * of the quest engine with the params each needs (server/internal/quest/objective/*.go).
 */

export const KEY_ORDER = {
  dialogue: ['id', 'about', 'start', 'call', 'nodes'],
  node: ['say', 'emote', 'shop', 'fee', 'emit', 'choices', 'reset_marks'],
  choice: ['text', 'goto', 'emit', 'requires', 'unless', 'requires_quest', 'if_flags', 'unless_flags', 'mark', 'goto_if'],
  quest: ['id', 'version', 'questline', 'title', 'about', 'tags_any', 'requires_caps', 'unless_caps', 'requires', 'requires_flags', 'unless_flags', 'repeat', 'pool', 'job', 'flow', 'objectives', 'outcomes', 'rewards', 'texts'],
  objective: ['id', 'type', 'params', 'count', 'text', 'at'],
  outcome: ['id', 'type', 'params', 'count', 'text', 'at', 'title', 'flags', 'rewards', 'texts'],
  rewards: ['xu', 'titles', 'stamps', 'capability', 'flags'],
} as const;

/** Limits the server checks (server/content). */
export const LIMITS = {
  /** a line's characters: a node's say, a choice, a quest title or about, an objective's text, a message */
  say: 320,
  choice: 90,
  choices: 6,
  title: 32,
  questTitle: 64,
  about: 200,
  objectiveText: 200,
  message: 160,
  messages: 3,
  flags: 6,
  objectives: 8,
  outcomesMin: 2,
  outcomesMax: 6,
  xuQuest: 200_000,
  xuDaily: 20_000,
  xuJob: 40_000,
} as const;

export const FLOWS = ['sequence', 'all', 'any'] as const;
export const DEFAULT_VARIANT = 'default';
export const ID_PATTERN = /^[a-z0-9]+(_[a-z0-9]+)*$/;
export const QUEST_ID_PATTERN = /^[a-z0-9]+(_[a-z0-9]+)*(\.[a-z0-9]+(_[a-z0-9]+)*)*$/;
export const SIGNAL_PATTERN = ID_PATTERN;

/** Where a param's value comes from: the picker offers the content's own ids. */
export type ParamKind = 'npc' | 'npcs' | 'place' | 'signal' | 'item' | 'itemkind' | 'emote' | 'fee' | 'quest' | 'vehicle' | 'test' | 'role' | 'placekind';

export interface ParamDef {
  key: string;
  kind: ParamKind;
  /** an objective may leave it out when false */
  required: boolean;
}

export interface ObjectiveType {
  type: string;
  name: string;
  params: ParamDef[];
  /** what `count` means for the type, when it means something */
  count?: string;
}

const p = (key: string, kind: ParamKind, required = true): ParamDef => ({ key, kind, required });

export const OBJECTIVE_TYPES: ObjectiveType[] = [
  { type: 'dialogue_signal', name: 'chọn câu trả lời (tín hiệu)', params: [p('npc', 'npc'), p('signal', 'signal'), p('also', 'npcs', false)] },
  { type: 'talk_npc', name: 'nói chuyện với NPC', params: [p('npc', 'npc', false)], count: 'số NPC khác nhau' },
  { type: 'reach_place', name: 'đi tới nơi', params: [p('place', 'place', false), p('kind', 'placekind', false)], count: 'số nơi khác nhau' },
  { type: 'stay_place', name: 'ở lại một nơi', params: [p('place', 'place')], count: 'số giây' },
  { type: 'buy_item', name: 'mua đồ', params: [p('item', 'item', false), p('kind', 'itemkind', false)] },
  { type: 'pay_fee', name: 'đóng phí', params: [p('fee', 'fee')] },
  { type: 'emote_at', name: 'làm điệu bộ tại nơi', params: [p('emote', 'emote'), p('place', 'place')] },
  { type: 'greet', name: 'chào người khác', params: [p('emote', 'emote', false)], count: 'số người' },
  { type: 'give_way', name: 'chỉ đường cho khách', params: [], count: 'số lần' },
  { type: 'ask_way', name: 'hỏi đường', params: [p('npc', 'npc', false), p('signal', 'signal', false)] },
  { type: 'pass_test', name: 'thi', params: [p('test', 'test')] },
  { type: 'mount', name: 'lên xe', params: [p('vehicle', 'vehicle', false)] },
  { type: 'ride_through', name: 'chạy xe qua nơi', params: [p('place', 'place')] },
  { type: 'ride_distance', name: 'chạy xe đủ quãng đường', params: [], count: 'số mét' },
  { type: 'trip', name: 'đi một cuốc Crab', params: [p('role', 'role')] },
  { type: 'quest_done', name: 'xong một quest khác', params: [p('quest', 'quest')] },
  { type: 'daily_streak', name: 'làm hằng ngày liền nhiều ngày', params: [], count: 'số ngày' },
  { type: 'errand', name: 'việc vặt (nhiều điểm)', params: [] },
];

export const objectiveType = (type: string): ObjectiveType | undefined => OBJECTIVE_TYPES.find((t) => t.type === type);

export const TRIP_ROLES = ['rider', 'passenger'];
export const PLACE_KINDS = ['landmark', 'street', 'area', 'district'];
/** objective types whose params are too nested for pickers (errand: stops, seconds, order): edited as JSON */
export const RAW_PARAM_TYPES = ['errand'];
export const TESTS = ['a1'];
