-- The Crab driver stops being an archetype (docs/design/13-driver.md): driving is earned in play, by the A1 licence,
-- a bike and the Crab uniform. Every character that is a driver today gets what the job was made of, as the items
-- and capabilities that now give it, and becomes a Native Saigonese (the archetype `citizen`): a driver who has
-- been working the streets for a while is no newcomer (no Má call, no guesthouse), and a native keeps the street
-- quests and the neighbourhood tags. Their quest progress is untouched; the driver quests they finished stay done.
-- Only adds and updates, like every migration.

-- the bike and the uniform, free: they were part of the job
insert into owned_items (character_id, item_id, price, bought_at)
  select id, 'xe_so_110', 0, cast(strftime('%s', 'now') as integer) * 1000 from characters where archetype_id = 'driver'
  on conflict do nothing;
insert into owned_items (character_id, item_id, price, bought_at)
  select id, 'dong_phuc_crab', 0, cast(strftime('%s', 'now') as integer) * 1000 from characters where archetype_id = 'driver'
  on conflict do nothing;

-- a driver made after the free Ifone X of 0009 may have no phone; the Crab Driver app needs a smartphone
insert into owned_items (character_id, item_id, price, bought_at)
  select id, 'ifone_x', 0, cast(strftime('%s', 'now') as integer) * 1000 from characters
  where archetype_id = 'driver' and id not in (select character_id from owned_items where item_id in ('nokla_1280', 'ifone_x', 'ifone_11', 'ifone_15_pro'))
  on conflict do nothing;

-- the capabilities of those items, with the sources a purchase would have written (the same rows may exist already)
insert into capability_grants (character_id, capability, source, granted_at)
  select c.id, g.capability, g.source, cast(strftime('%s', 'now') as integer) * 1000
  from characters c, (
    select 'vehicle.motorbike.own' as capability, 'item:xe_so_110' as source
    union all select 'uniform.crab', 'item:dong_phuc_crab'
    union all select 'driver.app', 'item:dong_phuc_crab'
    union all select 'ridehail.drive', 'item:dong_phuc_crab'
    union all select 'vehicle.motorbike.ride', 'migration:driver'
  ) g
  where c.archetype_id = 'driver'
  on conflict do nothing;

insert into capability_grants (character_id, capability, source, granted_at)
  select c.id, g.capability, 'item:ifone_x', cast(strftime('%s', 'now') as integer) * 1000
  from characters c, (select 'phone.basic' as capability union all select 'phone.smart') g
  where c.archetype_id = 'driver'
    and exists (select 1 from owned_items o where o.character_id = c.id and o.item_id = 'ifone_x')
  on conflict do nothing;

update characters set archetype_id = 'citizen' where archetype_id = 'driver';
