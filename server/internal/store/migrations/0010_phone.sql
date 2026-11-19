-- The phone (server/content/contacts.json, server/internal/phone, docs/design/12-phone.md): the numbers a character
-- has and the text messages it got. Only adds, like every migration.

-- One row per number a character has (an NPC it talked to, a contact it started with, a sender that texted it).
-- contact_id is a content id, so it is not a foreign key: a number taken out of contacts.json stays stored and is
-- no longer shown.
create table contacts (
  character_id text not null references characters(id) on delete cascade,
  contact_id   text not null,
  since        integer not null,
  primary key (character_id, contact_id)
) strict;

-- One row per text message. id says which message it is (the quest, the cycle and the moment that sent it), so a
-- commit that is retried never sends one twice. The text is kept in both languages as it was sent: the content may
-- change later, the history does not. read_at is null until the player opened the Tin nhắn app.
create table texts (
  character_id text not null references characters(id) on delete cascade,
  id           text not null,
  sender       text not null,
  vi           text not null,
  en           text not null,
  sent_at      integer not null,
  read_at      integer,
  primary key (character_id, id)
) strict;
