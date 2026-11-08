-- Wallets in xu, the ledger of every change to a balance, and Crab Bike rides (docs/design/08-ride-hailing.md,
-- server/internal/ridehail). Only adds, like every migration.

create table wallets (
  character_id text primary key references characters(id) on delete cascade,
  balance      integer not null check (balance >= 0),
  -- the day (Saigon time) of the last daily top-up
  topped_up_on text,
  updated_at   integer not null
) strict;

create table ledger (
  id           text primary key,
  character_id text not null references characters(id) on delete cascade,
  delta        integer not null,
  -- start, topup, ride_hold, ride_refund, ride_earn, ride_tip_out, ride_tip_in
  reason       text not null,
  ride_id      text,
  created_at   integer not null,
  -- a retried ride step never moves money twice (rows without a ride, like top-ups, are not limited)
  unique (ride_id, reason, character_id)
) strict;
create index ledger_by_character on ledger(character_id, created_at);

create table rides (
  id           text primary key,
  passenger_id text not null references characters(id),
  driver_id    text references characters(id),
  dest_place   text not null,
  pickup_x     real not null,
  pickup_z     real not null,
  fare         integer not null,
  -- requested, completed, cancelled, expired, ended
  state        text not null,
  end_reason   text,
  -- what the driver got (the fare minus the platform's cut, or part of it)
  paid         integer,
  stars        integer check (stars between 1 and 5),
  tip          integer,
  requested_at integer not null,
  accepted_at  integer,
  boarded_at   integer,
  finished_at  integer
) strict;
create index rides_by_driver on rides(driver_id, finished_at);
