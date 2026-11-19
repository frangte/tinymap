-- Items a character bought (server/content/items.json, server/internal/shop): one row per item, bought once. The
-- purchase itself is one transaction in the store: this row, the balance down, a ledger line (reason 'buy') and a
-- capability_grants row per capability the item gives, with source 'item:<id>'. Only adds, like every migration.

create table owned_items (
  character_id text not null references characters(id) on delete cascade,
  item_id      text not null,
  -- what it cost in xu, 0 for the free one below
  price        integer not null check (price >= 0),
  bought_at    integer not null,
  primary key (character_id, item_id)
) strict;

-- Every character from before items gets an Ifone X for free (docs/design/10-story.md, section 13: everyone has a
-- smartphone), so what they could do yesterday they still can: the phone apps, Crab, chat.
insert into owned_items (character_id, item_id, price, bought_at)
  select id, 'ifone_x', 0, cast(strftime('%s', 'now') as integer) * 1000 from characters;

insert into capability_grants (character_id, capability, source, granted_at)
  select id, 'phone.basic', 'item:ifone_x', cast(strftime('%s', 'now') as integer) * 1000 from characters where true
  on conflict do nothing;

insert into capability_grants (character_id, capability, source, granted_at)
  select id, 'phone.smart', 'item:ifone_x', cast(strftime('%s', 'now') as integer) * 1000 from characters where true
  on conflict do nothing;
