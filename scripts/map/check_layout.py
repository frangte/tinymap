"""Checks a Sài Gòn mini layout for the mistakes a player notices at once (Khoa's review of v8, 2026-11-12):

    python3 scripts/map/check_layout.py [layout.json]        (needs shapely; default: scripts/map/saigon_mini_layout.json)

- road ends that stop in the grass or short of the road they should join (a road must end on another road, a
  roundabout, a bridge or a walk; an end on a bridge must be the bridge's own end);
- footbridges that come down on a carriageway, car bridges that come down inside a junction;
- roads running through a building (a landmark footprint, a house lot, a plaza);
- walks that end nowhere;
- house lots too small to hold a row of houses, or with no street or walk to face;
- landmarks with no street, walk or square in front of them.
Prints one line per problem and exits 1 if there is any. Plan coordinates: x east, y south, metres.
"""
import json
import math
import os
import sys

from shapely.geometry import LineString, Point, Polygon, box
from shapely.ops import unary_union

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
from plan_paths import sample  # noqa: E402

SRC = sys.argv[1] if len(sys.argv) > 1 else os.path.join(HERE, 'saigon_mini_layout.json')
L = json.load(open(SRC, encoding='utf-8'))
SIDEWALK = 3.0
problems = []


def line(d, step=0.5):
    return LineString(sample(d, step))


def name(r, k):
    return r.get('name') or r.get('id') or 'road #%d (%s…)' % (k, r['path'][:24])


roads = [(name(r, k), line(r['path']), r['w'], r) for k, r in enumerate(L['roads'])]
walks = [(name(w, k), line(w['path']), w['w'], w) for k, w in enumerate(L['walks'])]
bridges = [(b.get('name', b['id']), line(b['path']), b['w'], b) for b in L['bridges']]
rings = [(Point(ra['c']), ra['r'] + 5) for ra in L['roundabouts']]
carriage = unary_union([ln.buffer(w / 2) for _, ln, w, _ in roads] + [p.buffer(r) for p, r in rings])
paved = unary_union([carriage, *[ln.buffer(w / 2 + SIDEWALK) for _, ln, w, _ in roads],
                     *[ln.buffer(w / 2) for _, ln, w, _ in walks],
                     *[box(*r) for r in L.get('plazas', [])],
                     *[line(p['path']).buffer(p['w'] / 2) for p in L.get('promenades', [])]])
foot = {}
for lm in L['landmarks']:
    if 'at' in lm and 'size' in lm:
        (cx, cy), (w, d) = lm['at'], lm['size']
        foot[lm['id']] = box(cx - w / 2, cy - d / 2, cx + w / 2, cy + d / 2)
parks = unary_union([box(*p['rect']) for p in L['parks']] or [Polygon()])
for k, (x0, y0, x1, y1, _h) in enumerate(L.get('towers', [])):
    foot['tower_%d' % k] = box(x0, y0, x1, y1)
flats = unary_union([Point(x, y).buffer(r) for x, y, r in L.get('flats', [])] or [Polygon()])
MODELLED_BRIDGES = {'cau_ba_son'}        # a part script builds them, with their own approaches
coast = Polygon(sample(L['coast'], 0.8)).buffer(0)


# ---- road ends
def end_dir(ln, at_start):
    c = list(ln.coords)
    a, b = (c[1], c[0]) if at_start else (c[-2], c[-1])
    dx, dy = b[0] - a[0], b[1] - a[1]
    dl = math.hypot(dx, dy) or 1.0
    return dx / dl, dy / dl


for k, (nm, ln, w, r) in enumerate(roads):
    for at_start in (True, False):
        if r.get('endsAt') and not at_start:
            continue                                  # joins a road a part script builds (Đường Hải Đăng)
        p = Point(ln.coords[0] if at_start else ln.coords[-1])
        others = [o for j, (_, o, _, _) in enumerate(roads) if j != k]
        d_road = min((o.distance(p) for o in others), default=99)
        d_ring = min((p.distance(c) - rr for c, rr in rings), default=99)
        d_bridge_end = min((min(p.distance(Point(b.coords[0])), p.distance(Point(b.coords[-1]))) for _, b, _, _ in bridges), default=99)
        d_walk = min((o.distance(p) for _, o, _, _ in walks), default=99)
        d_bridge = min((b.distance(p) for _, b, _, _ in bridges), default=99)     # on a bridge's line (it carries on)
        if d_road < 0.8 or d_ring < 0 or d_bridge_end < 1.0 or d_walk < 0.8 or d_bridge < 0.3:
            continue
        if not coast.buffer(-2).contains(p):
            continue                                      # runs off the island (a bridge end in the sea is reported below)
        problems.append('road end      %-22s at (%.0f, %.0f): nearest road %.1f m, roundabout %.1f m' % (nm, p.x, p.y, d_road, d_ring))

# ---- bridges
for nm, ln, w, b in bridges:
    if b['id'] in MODELLED_BRIDGES:
        continue
    for k_end, at_start in enumerate((True, False)):
        p = Point(ln.coords[0] if at_start else ln.coords[-1])
        if b.get('walk'):
            if carriage.buffer(-0.3).contains(p):
                problems.append('footbridge    %-22s comes down on a carriageway at (%.0f, %.0f)' % (nm, p.x, p.y))
        else:
            # an end either carries on as a road (a road ends there) or sits on a road's centreline (a T junction);
            # either way the deck stays level with the road ('flat') across the junction and its pavement
            ends = [Point(c) for _, o, _, _ in roads for c in (o.coords[0], o.coords[-1])]
            flat = b.get('flat', [0, 0])[k_end]
            on = [(nm2, w2) for nm2, o, w2, _ in roads if o.distance(p) < 0.6]
            if any(p.distance(e) < 1.0 for e in ends) and not on[1:]:
                continue
            if not on:
                problems.append('car bridge    %-22s end (%.0f, %.0f) does not continue as a road' % (nm, p.x, p.y))
            for nm2, w2 in on:
                if flat < w2 / 2 + SIDEWALK - 0.5 and not any(p.distance(e) < 1.0 for e in ends):
                    problems.append('car bridge    %-22s rises inside the junction with %s (flat %.0f m)' % (nm, nm2, flat))

# ---- roads through buildings
for nm, ln, w, r in roads:
    band = ln.buffer(w / 2 + 1.0)                      # the carriageway and a metre (a facade on the pavement is fine)
    for fid, f in foot.items():
        if band.intersection(f).area > 1.0:
            problems.append('road in house %-22s runs into landmark %s' % (nm, fid))
    for lt in L['lots']:
        ov = band.intersection(box(*lt['rect'])).area
        if ov > 0.25 * box(*lt['rect']).area:
            problems.append('road in lot   %-22s covers %.0f%% of lot %s' % (nm, 100 * ov / box(*lt['rect']).area, lt['rect']))

# ---- walks that end nowhere
for k, (nm, ln, w, wk) in enumerate(walks):
    if ln.is_ring:
        continue
    others = unary_union([carriage, *[o.buffer(ow / 2 + SIDEWALK) for _, o, ow, _ in roads],
                          *[o.buffer(ow / 2) for j, (_, o, ow, _) in enumerate(walks) if j != k],
                          *[box(*r) for r in L.get('plazas', [])], parks, flats, *foot.values(),
                          *[line(pr['path']).buffer(pr['w'] / 2) for pr in L.get('promenades', [])],
                          *[b.buffer(bw / 2 + 1) for _, b, bw, _ in bridges]])
    for at_start in (True, False):
        p = Point(ln.coords[0] if at_start else ln.coords[-1])
        if others.distance(p) > 1.0:
            problems.append('walk end      %-22s at (%.0f, %.0f) ends nowhere' % (nm, p.x, p.y))

# ---- lots
cut = unary_union([carriage.buffer(SIDEWALK), *[ln.buffer(w / 2) for _, ln, w, _ in walks], *[f.buffer(2) for f in foot.values()]])
for lt in L['lots']:
    r = box(*lt['rect'])
    left = r.difference(cut)
    if left.area < 150:
        problems.append('lot           %s %s: only %.0f m² left for houses' % (lt['kind'], lt['rect'], left.area))
    elif r.distance(paved) > 4.0:
        problems.append('lot           %s %s: no street or walk within 4 m' % (lt['kind'], lt['rect']))

# ---- landmarks need a way in
for fid, f in foot.items():
    if f.distance(paved) > 6.0:
        problems.append('landmark      %s: no street, walk or square within 6 m' % fid)

for p in problems:
    print(p)
print('%d problems in %s' % (len(problems), os.path.relpath(SRC)))
sys.exit(1 if problems else 0)
