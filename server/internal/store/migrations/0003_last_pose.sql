-- Where a character stood when its player last left: three.js metres and the heading in radians, as on the wire
-- (server/README.md). The next visit starts there. Null until a visit has ended.
alter table characters add column last_x real;
alter table characters add column last_y real;
alter table characters add column last_z real;
alter table characters add column last_h real;
