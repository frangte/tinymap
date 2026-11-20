/**
 * The content formats the story map reads (server/content/*.json; the Go structs are in server/content/*.go).
 * Every field the flags and outcomes contract adds is optional: no content uses it yet.
 */

/** A text in both languages. */
export interface Text {
  vi?: string;
  en?: string;
}

export type Lang = 'vi' | 'en';

export interface Rewards {
  titles?: string[];
  stamps?: string[];
  capability?: string;
  xu?: number;
  /** story flags set when the quest completes */
  flags?: string[];
}

export interface Objective {
  id?: string;
  type: string;
  params?: Record<string, unknown>;
  count?: number;
  text?: Text;
  at?: string;
}

export interface QuestText {
  from: string;
  text: Text;
}

export interface QuestTexts {
  on_start?: QuestText[];
  on_done?: QuestText[];
}

/** One way a quest can end: an objective plus what finishing it with this outcome does. */
export interface Outcome extends Objective {
  title?: Text;
  flags?: string[];
  rewards?: Rewards;
  texts?: QuestTexts;
}

export interface QuestDef {
  id: string;
  version?: number;
  questline?: string;
  title: Text;
  about?: Text;
  tags_any?: string[];
  unless_caps?: string[];
  requires_caps?: string[];
  requires?: string[];
  requires_flags?: string[];
  unless_flags?: string[];
  repeat?: string;
  pool?: string;
  job?: boolean;
  flow?: string;
  objectives: Objective[];
  outcomes?: Outcome[];
  rewards?: Rewards;
  texts?: QuestTexts;
}

export interface AchievementDef {
  id: string;
  title: Text;
  about?: Text;
  hidden?: boolean;
  tags_any?: string[];
  objective: Objective;
  rewards?: Rewards;
}

export interface QuestsFile {
  quests: QuestDef[];
  achievements?: AchievementDef[];
}

export interface Archetype {
  id: string;
  tags: string[];
  name: Text;
  badge?: { short?: Text; color?: string; text?: string };
  capabilities?: string[];
}

export interface ArchetypesFile {
  archetypes: Archetype[];
  capabilities?: { id: string; about?: string }[];
}

export interface NpcDef {
  id: string;
  name: string;
  role?: Text;
  place?: string;
  dialogue: string;
}

export interface ContactDef {
  id: string;
  name?: Text;
  role?: Text;
  dialogue?: string;
  start_for?: string[];
}

export interface ItemDef {
  id: string;
  kind?: string;
  shop?: string;
  grants?: string[];
  name?: Text;
}

export interface FeeDef {
  id: string;
  npc?: string;
  grants?: string[];
  name?: Text;
}

export interface ShopDef {
  id: string;
  npc?: string;
  name?: Text;
  haggle?: { signal: string; percent?: number };
}

export interface ItemsFile {
  shops?: ShopDef[];
  fees?: FeeDef[];
  items?: ItemDef[];
}

export interface CollectionEntry {
  id: string;
  kind?: string;
  title?: Text;
  objective: Objective;
}

export interface PlaceDef {
  id: string;
  name?: Text;
}

export interface Choice {
  text: Text;
  goto: string;
  emit?: string;
  requires?: string[];
  unless?: string[];
  mark?: string;
  goto_if?: { mark: string; at_least: number; goto: string };
  requires_quest?: string;
  if_flags?: string[];
  unless_flags?: string[];
}

export interface DialogueNodeDef {
  say: Record<string, Text>;
  emote?: string;
  shop?: string;
  fee?: string;
  emit?: string;
  reset_marks?: boolean;
  choices?: Choice[];
}

export interface DialogueDef {
  id: string;
  about?: string;
  start: string;
  call?: string;
  nodes: Record<string, DialogueNodeDef>;
}

/** Everything the page reads, as parsed JSON. */
export interface RawContent {
  quests: QuestsFile;
  archetypes: ArchetypesFile;
  npcs: NpcDef[];
  contacts: ContactDef[];
  items: ItemsFile;
  collection: CollectionEntry[];
  places: PlaceDef[];
  dialogues: DialogueDef[];
}

/** The node id a goto uses to end the conversation (server/content/cast.go, DialogueEnd). */
export const DIALOGUE_END = 'end';
