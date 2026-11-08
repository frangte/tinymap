-- Accounts (anonymous guests for now), the tokens that identify them, and their characters.
-- Times are unix milliseconds; ids are ULIDs. Only the SHA-256 of a token is stored, never the token.
-- Later migrations only add (tables, nullable columns): see store.go.

create table accounts (
  id         text primary key,
  created_at integer not null
) strict;

create table auth_tokens (
  token_hash blob primary key,
  account_id text not null references accounts(id) on delete cascade,
  created_at integer not null,
  last_seen  integer
) strict;
create index auth_tokens_by_account on auth_tokens(account_id);

create table characters (
  id         text primary key,
  account_id text not null references accounts(id) on delete cascade,
  name       text not null,
  appearance text not null check (json_valid(appearance)),
  created_at integer not null,
  updated_at integer not null
) strict;
create index characters_by_account on characters(account_id);
