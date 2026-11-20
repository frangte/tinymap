-- Story flags and quest outcomes (docs/design/15-flags-outcomes.md). A flag is a fact about a character that a quest
-- set when it completed (rewards.flags, an outcome's flags, or the flag "<quest id>:<outcome id>" every outcome
-- sets); it is stored in the same transaction as the quest's completion, and in v1 never cleared. A quest that ended
-- in an outcome records which. Only adds, like every migration.

create table story_flags (
  character_id text not null references characters(id) on delete cascade,
  flag         text not null,
  set_at       integer not null,
  primary key (character_id, flag)
) strict;

-- the outcome a done quest ended with, '' for a quest without outcomes (or one still running)
alter table quest_progress add column outcome text not null default '';
