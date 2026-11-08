-- The archetype a character chose (server/content/archetypes.json); '' until it has chosen one.
-- Once set it stays (docs/design/README.md, D1).
alter table characters add column archetype_id text not null default '';
