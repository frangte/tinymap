-- Moderation (docs/design/README.md, D17; server/internal/moderation): who moderates, mutes and bans on the
-- account (every character of it), chat reports with the lines around them, and a log of what moderators did.
-- Only adds, like every migration.

-- set by `sg-server mod add <name>`; a moderator's characters get the moderation.act capability
alter table accounts add column moderator integer not null default 0;
-- unix milliseconds; null or past for none
alter table accounts add column muted_until integer;
alter table accounts add column banned_until integer;

create table chat_reports (
  id         integer primary key,
  reporter   text not null references characters(id) on delete cascade,
  target     text not null references characters(id) on delete cascade,
  -- the last lines of the two, oldest first: [{"name":…,"ch":…,"text":…,"at":…}]
  context    text not null check (json_valid(context)),
  created_at integer not null,
  handled_by text,
  handled_at integer
) strict;
create index chat_reports_open on chat_reports(handled_at, created_at);

create table moderation_log (
  id        integer primary key,
  moderator text not null,
  target    text not null,
  -- mute, unmute, ban, unban, kick, close_report
  action    text not null,
  until     integer,
  at        integer not null
) strict;
