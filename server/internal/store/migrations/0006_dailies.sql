-- Daily quests (server/content/quests.json, "repeat": "daily"): a quest_progress row per day (its cycle) and the
-- streak of days in a row with a daily done. Quest rewards in xu go through the ledger with the reason
-- 'quest_reward' (reward_grants keeps them to one per quest and day). Only adds, like every migration.

alter table characters add column streak_days integer not null default 0;
-- the day (Saigon time) of the last daily done, null for never
alter table characters add column streak_day text;
