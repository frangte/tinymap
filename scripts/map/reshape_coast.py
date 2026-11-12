"""Made the 2026-11-12 layout (approved by Khoa, now scripts/map/saigon_mini_layout.json): a smaller island with a natural, curving coast, and Thủ Thiêm laid out (docs/map-v5-tasks.md, T8, T13).

    python3 scripts/map/reshape_coast.py [out.json]      (needs shapely; default out: scripts/map/saigon_mini_layout_proposal.json)

Reads scripts/map/saigon_mini_layout_v1.json (the square island) and writes a new layout; kept to show how the
current one was made and to redo it from v1 if the coast points change:
- the coast: a smooth closed curve through hand-placed points (no straight sides, no square corners), a little
  inside the old rectangle; Vũng Tàu's capes and bays kept where they were,
- one coastal road (Đường ven biển) following the north and west coast 8 m inland, replacing the two straight edge
  roads; the grid streets that ran to the edge now stop at it,
- "inland": the land the house blocks may use (the coast less the sea wall, promenade and coastal road),
- Thủ Thiêm: the race-track loop replaced by a curving boulevard, a riverside park with a cycle path and flower
  fields, Công viên Sáng tạo at the Ba Son landing, a square and river-bus pier at Cầu Mống, a central lake with a
  path round it, a cluster of four different towers in the north, the Thủ Thiêm church and shophouse blocks in the
  south.
Plan coordinates as in the layout file: x east, y south, metres.
"""
import copy
import json
import math
import os
import sys

from shapely.geometry import LineString, Point, Polygon
from shapely.ops import substring

HERE = os.path.dirname(os.path.abspath(__file__))
SRC = os.path.join(HERE, 'saigon_mini_layout_v1.json')     # the square-island layout this was made from
OUT = sys.argv[1] if len(sys.argv) > 1 and not sys.argv[1].startswith('--') else os.path.join(HERE, 'saigon_mini_layout_proposal.json')
L = json.load(open(SRC, encoding='utf-8'))
P = copy.deepcopy(L)

# ---- the coast: clockwise from the north-west, through points placed by hand (x east, y south)
COAST_PTS = [
    (72, 19), (128, 11), (186, 8), (240, 14), (268, 18), (300, 8), (334, 8),              # north: city bulge, small bay, river mouth
    (384, 15), (428, 26), (453, 50), (461, 88), (456, 134), (465, 182), (456, 230),      # Thủ Thiêm: rounded north-east cape
    (446, 276), (432, 316), (437, 350), (452, 382), (450, 410), (430, 425),              # east coast in, round to the river mouth
    (380, 421), (320, 423), (262, 423), (252, 433), (238, 449), (214, 455), (192, 446),  # Bãi Sau, Núi Nhỏ cape
    (180, 428), (172, 404), (150, 384), (118, 381), (92, 388), (70, 402), (54, 394),     # Bãi Trước, Núi Lớn
    (34, 387), (16, 376), (11, 346), (15, 306), (21, 270), (33, 236), (24, 200),         # Bãi Dâu, west coast with a bay
    (14, 162), (17, 118), (25, 80), (42, 44),                                            # north-west cut back
]


def catmull_rom_path(pts, closed=True, tension=0.5):
    """an SVG path of cubic Béziers through every point (centripetal-free Catmull-Rom)"""
    n = len(pts)
    seg = []
    rng = range(n) if closed else range(n - 1)
    for i in rng:
        p0, p1 = pts[(i - 1) % n], pts[i]
        p2, p3 = pts[(i + 1) % n], pts[(i + 2) % n]
        if not closed:
            p0 = pts[max(i - 1, 0)]
            p3 = pts[min(i + 2, n - 1)]
        c1 = (p1[0] + (p2[0] - p0[0]) * tension / 3, p1[1] + (p2[1] - p0[1]) * tension / 3)
        c2 = (p2[0] - (p3[0] - p1[0]) * tension / 3, p2[1] - (p3[1] - p1[1]) * tension / 3)
        seg.append('C %.1f,%.1f %.1f,%.1f %.1f,%.1f' % (*c1, *c2, *p2))
    return 'M %.1f,%.1f ' % pts[0] + ' '.join(seg) + (' Z' if closed else '')


def sample_bezier_path(d, step=1.0):
    """plan points of a path made of absolute M L Q C S Z commands"""
    import re
    toks = re.findall(r'[MLQCSZ]|-?\d+(?:\.\d+)?', d)
    out, i, cur, cmd, c2prev = [], 0, None, None, None

    def bez(ctrl):
        n = max(2, int(sum(math.dist(ctrl[k], ctrl[k + 1]) for k in range(len(ctrl) - 1)) / step))
        res = []
        for k in range(1, n + 1):
            t = k / n
            pts = list(ctrl)
            while len(pts) > 1:
                pts = [((1 - t) * a[0] + t * b[0], (1 - t) * a[1] + t * b[1]) for a, b in zip(pts, pts[1:])]
            res.append(pts[0])
        return res

    def num(k):
        return [(float(toks[i + 2 * j]), float(toks[i + 2 * j + 1])) for j in range(k)]
    while i < len(toks):
        if toks[i] in 'MLQCSZ':
            cmd = toks[i]
            i += 1
            if cmd == 'Z':
                continue
        if cmd == 'M':
            (cur,) = num(1)
            out.append(cur)
            i += 2
            cmd, c2prev = 'L', None
        elif cmd == 'L':
            (p,) = num(1)
            out += bez([cur, p])
            cur, c2prev = p, None
            i += 2
        elif cmd == 'Q':
            c, p = num(2)
            out += bez([cur, c, p])
            cur, c2prev = p, None
            i += 4
        elif cmd == 'C':
            c1, c2, p = num(3)
            out += bez([cur, c1, c2, p])
            cur, c2prev = p, c2
            i += 6
        elif cmd == 'S':
            c2, p = num(2)
            c1 = (2 * cur[0] - c2prev[0], 2 * cur[1] - c2prev[1]) if c2prev else cur
            out += bez([cur, c1, c2, p])
            cur, c2prev = p, c2
            i += 4
    return out


def poly_path(coords, closed=True, nd=1):
    pts = list(coords)
    if closed and pts[0] == pts[-1]:
        pts = pts[:-1]
    fmt = '%.' + str(nd) + 'f,%.' + str(nd) + 'f'
    return 'M ' + ' L '.join(fmt % p for p in pts) + (' Z' if closed else '')


P['coast'] = catmull_rom_path(COAST_PTS)
coast = Polygon(sample_bezier_path(P['coast'], 0.8)).buffer(0)

# ---- land use bands from the coast inwards: sea wall + promenade 0..4 m, coastal road 4..12 m, pavement to 15 m
ROAD_IN, ROAD_W, INLAND = 8.0, 8.0, 15.0
ring = coast.buffer(-ROAD_IN).exterior
inland = coast.buffer(-INLAND, join_style='round')
P['inland'] = poly_path(inland.exterior.simplify(0.4).coords)

# the coastal road: along the north and west coast, from Tôn Đức Thắng's north end round to Trần Hưng Đạo's west end
a = ring.project(Point(286, 22))
b = ring.project(Point(18, 296))
if b > a:
    line = substring(ring, a, b)
else:
    line = LineString(list(substring(ring, a, ring.length).coords) + list(substring(ring, 0, b).coords)[1:])
# keep it running west from Tôn Đức Thắng (the ring runs clockwise in plan = west along the north coast)
coast_road = line.simplify(0.35)
inside_road = Polygon(ring)

roads = []
for r in L['roads']:
    if r['path'] in ('M 18,22 L 286,22', 'M 18,22 L 18,296', 'M 338,22 L 470,22', 'M 338,66 L 470,66'):
        continue        # the straight edge roads (replaced by the coastal road) and Thủ Thiêm's old roads
    pts = sample_bezier_path(r['path'], 0.5)
    touches_edge = min(p[0] for p in pts) <= 20 or min(p[1] for p in pts) <= 24
    ln = LineString(pts)
    if touches_edge and not inside_road.contains(ln):
        cl = ln.intersection(inside_road)
        if cl.is_empty or cl.length < 2:
            continue
        cl = max(getattr(cl, 'geoms', [cl]), key=lambda g: g.length).simplify(0.2)
        straight = r['path'].count('L') == 1 and not any(c in r['path'] for c in 'QCS')
        coords = [cl.coords[0], cl.coords[-1]] if straight else list(cl.coords)
        r = dict(r, path=poly_path(coords, closed=False))
        if 'label' in r and not inland.contains(Point(*r['label'])):
            x0, y0 = coords[0]
            r['label'] = [round(x0 + 6, 1), round(y0 - 1.4, 1)] if not r.get('vertical') else [round(x0 + 1.4, 1), round(y0 + 16, 1)]
    roads.append(r)
roads.insert(0, {'name': 'Đường ven biển', 'path': poly_path(coast_road.coords, closed=False), 'w': ROAD_W,
                 'label': [96, 26.6], 'coastal': True})

# ---- Thủ Thiêm (east bank), inside the new coast
roads += [
    {'name': 'Mai Chí Thọ', 'path': 'M 338,66 L 392,66 C 418,66 430,80 430,106 L 430,250 C 430,282 412,300 388,302', 'w': 10,
     'label': [435.5, 200], 'vertical': True},
    {'path': 'M 392,66 L 392,32', 'w': 7},
]
P['loops'] = []
P['roads'] = roads
P['promenades'] = [
    {'path': 'M 338,22 L 338,236 C 338,282 368,302 404,334 S 446,370 452,392', 'w': 6},
    {'path': 'M 295,22 L 295,232', 'w': 6},
    {'path': 'M 352,104 L 352,232 C 352,262 372,284 398,300', 'w': 3, 'kind': 'cycle'},
]
P['walks'] = L['walks'] + [
    {'name': 'Quanh hồ', 'path': 'M 364,206 C 364,190 378,180 394,180 C 410,180 420,190 420,206 C 420,222 410,230 394,230 C 378,230 364,222 364,206 Z', 'w': 4},
    {'path': 'M 362,206 L 364,206', 'w': 5},
]
P['parks'] = [pk for pk in L['parks'] if pk['rect'][0] < 330] + [
    {'rect': [358, 106, 422, 168], 'kind': 'lawn', 'r': 10},
    {'rect': [341, 110, 349, 160], 'kind': 'flowers'},
    {'rect': [341, 238, 360, 262], 'kind': 'flowers'},
    {'rect': [362, 238, 396, 262], 'kind': 'trees'},
    {'rect': [404, 326, 436, 372], 'kind': 'trees'},
]
P['lakes'] = [{'ellipse': [392, 205, 20, 15], 'fountain': True}]
P['plazas'] = L.get('plazas', []) + [[346, 72, 388, 100], [342, 196, 362, 216], [374, 296, 392, 310]]
P['towers'] = [[368, 32, 386, 50, 70], [402, 40, 418, 56, 46], [430, 52, 444, 66, 38], [438, 84, 452, 100, 30]]
P['lots'] = [lt for lt in L['lots'] if lt['rect'][0] < 330 or lt['rect'][1] < 300] + [
    {'rect': [362, 270, 420, 292], 'kind': 'np'},
    {'rect': [396, 306, 424, 322], 'kind': 'np'},
]
P['lots'] = [lt for lt in P['lots'] if lt['rect'] != [420, 296, 478, 330]]
P['landmarks'] = [lm for lm in L['landmarks'] if lm['id'] != 'thu_thiem'] + [
    {'n': 19, 'id': 'thu_thiem', 'name': 'Công viên bờ sông Thủ Thiêm', 'status': 'new', 'mark': [362, 136]},
    {'n': 32, 'id': 'cong_vien_sang_tao', 'name': 'Công viên Sáng tạo (sân khấu, gian hàng)', 'status': 'new',
     'at': [380, 86], 'size': [12, 8], 'face': 'W', 'mark': [366, 86], 'color': 'creamLight', 'roof': 'tinBlue', 'h': 5},
    {'n': 33, 'id': 'thap_thu_thiem', 'name': 'Tháp Thủ Thiêm (tháp điểm nhấn ~70 m)', 'status': 'new', 'mark': [377, 41]},
    {'n': 34, 'id': 'nha_tho_thu_thiem', 'name': 'Nhà thờ Thủ Thiêm', 'status': 'new', 'at': [410, 250], 'size': [10, 18],
     'face': 'W', 'mark': [410, 262], 'color': 'opWhite', 'roof': 'slate', 'h': 10},
    {'n': 36, 'id': 'rung_dua_nuoc', 'name': 'Rừng dừa nước ven sông', 'status': 'new', 'mark': [420, 349]},
    {'n': 35, 'id': 'ben_buyt_song', 'name': 'Bến buýt đường sông', 'status': 'new', 'at': [333, 214], 'size': [5, 14],
     'face': 'W', 'mark': [333, 224], 'color': 'stone', 'roof': 'tinBlue', 'h': 3},
]
P['rocks'] = L['rocks']
if '--compare' in sys.argv:
    P['compareCoast'] = L['coast']          # drawn dashed by layout_svg.mjs, for the review only
P['about'] = L['about'] + ' Proposal 2026-11-12 (scripts/map/reshape_coast.py): curving coast, coastal road, Thủ Thiêm.'

json.dump(P, open(OUT, 'w', encoding='utf-8'), ensure_ascii=False, indent=1)
old = Polygon(sample_bezier_path(L['coast'].replace('Q', 'C'), 0.8)) if False else None
print('wrote', OUT, 'coast area %.0f m2' % coast.area)
