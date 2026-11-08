-- Quests and achievements (server/content/quests.json, run by server/internal/quest): progress, the rewards
-- already given (a retry never gives one twice), and what was earned. Only adds, like every migration.

create table quest_progress (
  character_id text not null references characters(id) on delete cascade,
  -- a quest id, or "achievement:<id>"
  quest_id     text not null,
  -- '' for a quest done once; a day for dailies (later)
  cycle        text not null default '',
  def_version  integer not null,
  state        text not null check (state in ('active', 'done')),
  -- {"<objective id>": {"n": 3, "seen": ["…"]}}
  objectives   text not null default '{}' check (json_valid(objectives)),
  started_at   integer not null,
  completed_at integer,
  primary key (character_id, quest_id, cycle)
) strict;

create table reward_grants (
  character_id text not null references characters(id) on delete cascade,
  -- "quest:<id>:<cycle>" or "achievement:<id>"
  source       text not null,
  granted_at   integer not null,
  primary key (character_id, source)
) strict;

-- titles and stamps
create table earned (
  character_id text not null references characters(id) on delete cascade,
  kind         text not null check (kind in ('title', 'stamp')),
  item         text not null,
  source       text not null,
  earned_at    integer not null,
  primary key (character_id, kind, item)
) strict;

create table capability_grants (
  character_id text not null references characters(id) on delete cascade,
  capability   text not null,
  source       text not null,
  granted_at   integer not null,
  primary key (character_id, capability, source)
) strict;

-- the title the character carries (quests.json titles), null for none
alter table characters add column title_id text;
