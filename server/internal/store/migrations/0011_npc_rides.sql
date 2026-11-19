-- Trips with an NPC passenger (docs/design/08-ride-hailing.md, "NPC passengers", server/internal/ridehail). The
-- passenger has no wallet: the fare's driver share is paid by the game, in the ledger with the reason 'ride_npc'
-- (once per trip, by the ledger's unique key). Only completed trips are stored. Only adds, like every migration.

create table npc_rides (
  id           text primary key,
  driver_id    text not null references characters(id) on delete cascade,
  -- the passenger's id in passengers.json: content, so not a foreign key
  passenger    text not null,
  dest_place   text not null,
  pickup_x     real not null,
  pickup_z     real not null,
  fare         integer not null,
  -- what the driver got (the fare minus the platform's cut); 0 once the driver is over the daily cap
  paid         integer not null,
  stars        integer not null check (stars between 1 and 5),
  -- the Saigon day it ended on, for the daily cap
  day          text not null,
  accepted_at  integer,
  boarded_at   integer,
  finished_at  integer not null
) strict;
create index npc_rides_by_driver on npc_rides(driver_id, day);
