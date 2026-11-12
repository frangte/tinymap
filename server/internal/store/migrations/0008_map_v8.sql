-- The game moved to a new map, Sài Gòn mini (layout v8): a spot saved on the old island means nothing on it, so
-- every character starts its next visit at its archetype's home. Changes data only, no schema: an older server
-- binary still works against this database (it would place everyone at home too).
update characters set last_x = null, last_y = null, last_z = null, last_h = null;
