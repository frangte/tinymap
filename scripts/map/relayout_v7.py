"""Made the layout v7 (2026-11-12, agreed with Khoa): Vũng Tàu moved to the east bank, the land south of the city cut.

    python3 scripts/map/relayout_v7.py [out.json]      (needs shapely; default out: scripts/map/saigon_mini_layout.json)

Reads scripts/map/saigon_mini_layout_v2.json (the 2026-11-12 curving-coast layout with Vũng Tàu in the south) and
writes the v7 layout. Kept to show how v7 was made and to redo it if the numbers below change.

Why (game design review, docs/map-v5-tasks.md): the island was too big for a walking game, with dead lawns in Thủ Thiêm
and round Núi Lớn, and Vũng Tàu at the far end of the map. v7:
- the river runs straight south to the sea; the west bank (the city) ends at a south coast with the coastal road
  carried round it (Đường ven biển now rings the city: north, west and south, with Tôn Đức Thắng on the river);
- cut: Núi Lớn, Bãi Dâu, Bạch Dinh, the old Vũng Tàu town and the lawns between them;
- the east bank: Thủ Thiêm in the north (unchanged: towers, Công viên Sáng tạo, lawn, lake, church), Vũng Tàu in the
  south as a peninsula on the real pattern: Núi Nhỏ at the tip with the statue and the lighthouse, Bãi Sau along the
  east coast (Thùy Vân, hotels), Bãi Trước on the west shore at the river mouth (Quang Trung, boats), Hòn Bà offshore;
- Núi Nhỏ, Hạ Long, the stairs, the statue and the lighthouse are moved whole (NUI_MOVE; the part scripts read it as
  the mountain's 'designMove', so their approved shapes do not change);
- Bùi Viện: the street moves 8 m south (y 270) and widens to 15 m so the v4 street block (bars, string lights, the
  gate) fits between công viên 23/9 and Trần Hưng Đạo; scripts/blender/mini/bui_vien.py puts it there.
Plan coordinates as in the layout file: x east, y south, metres.
"""
import copy
import json
import math
import os
import re
import sys

from shapely.geometry import LineString, Point, Polygon
from shapely.ops import substring

HERE = os.path.dirname(os.path.abspath(__file__))
SRC = os.path.join(HERE, 'saigon_mini_layout_v2.json')
OUT = sys.argv[1] if len(sys.argv) > 1 and not sys.argv[1].startswith('--') else os.path.join(HERE, 'saigon_mini_layout.json')
L = json.load(open(SRC, encoding='utf-8'))
P = copy.deepcopy(L)
NUI_MOVE = (189.0, 0.0)          # Núi Nhỏ and everything on it, from v2 (plan metres)

# ---- the coast, clockwise from the north-west (north and Thủ Thiêm as in v2)
COAST_PTS = [
    (72, 19), (128, 11), (186, 8), (240, 14), (268, 18), (300, 8), (334, 8),
    (384, 15), (428, 26), (453, 50), (461, 88), (456, 134), (465, 182), (456, 230),
    (461, 256), (470, 284), (476, 320), (476, 356), (471, 386), (462, 408), (451, 423),     # Bãi Sau, east coast
    (441, 433), (427, 449), (403, 455), (381, 446), (369, 428), (361, 404),                  # Núi Nhỏ cape (v2 + NUI_MOVE)
    (350, 384), (338, 366), (330, 353), (327, 344),                                          # Bãi Trước, river mouth
    (298, 345), (250, 349), (190, 347), (130, 349), (70, 347), (38, 343), (21, 328), (15, 306),  # the city's south coast
    (21, 270), (33, 236), (24, 200), (14, 162), (17, 118), (25, 80), (42, 44),
]


def catmull_rom_path(pts, closed=True, tension=0.5):
    n = len(pts)
    seg = []
    for i in (range(n) if closed else range(n - 1)):
        p0, p1, p2, p3 = pts[(i - 1) % n], pts[i], pts[(i + 1) % n], pts[(i + 2) % n]
        if not closed:
            p0, p3 = pts[max(i - 1, 0)], pts[min(i + 2, n - 1)]
        c1 = (p1[0] + (p2[0] - p0[0]) * tension / 3, p1[1] + (p2[1] - p0[1]) * tension / 3)
        c2 = (p2[0] - (p3[0] - p1[0]) * tension / 3, p2[1] - (p3[1] - p1[1]) * tension / 3)
        seg.append('C %.1f,%.1f %.1f,%.1f %.1f,%.1f' % (*c1, *c2, *p2))
    return 'M %.1f,%.1f ' % pts[0] + ' '.join(seg) + (' Z' if closed else '')


def sample(d, step=1.0):
    """plan points of a path of absolute M L Q C S Z commands"""
    toks = re.findall(r'[MLQCSZ]|-?\d+(?:\.\d+)?', d)
    out, i, cur, cmd, c2prev = [], 0, None, None, None

    def bez(ctrl):
        n = max(2, int(sum(math.dist(ctrl[k], ctrl[k + 1]) for k in range(len(ctrl) - 1)) / step))
        res = []
        for k in range(1, n + 1):
            t = k / n
            q = list(ctrl)
            while len(q) > 1:
                q = [((1 - t) * a[0] + t * b[0], (1 - t) * a[1] + t * b[1]) for a, b in zip(q, q[1:])]
            res.append(q[0])
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
            cur, c2prev, i = p, None, i + 2
        elif cmd == 'Q':
            c, p = num(2)
            out += bez([cur, c, p])
            cur, c2prev, i = p, None, i + 4
        elif cmd == 'C':
            c1, c2, p = num(3)
            out += bez([cur, c1, c2, p])
            cur, c2prev, i = p, c2, i + 6
        elif cmd == 'S':
            c2, p = num(2)
            c1 = (2 * cur[0] - c2prev[0], 2 * cur[1] - c2prev[1]) if c2prev else cur
            out += bez([cur, c1, c2, p])
            cur, c2prev, i = p, c2, i + 4
    return out


def poly_path(coords, closed=True, nd=1):
    pts = list(coords)
    if closed and pts[0] == pts[-1]:
        pts = pts[:-1]
    fmt = '%.' + str(nd) + 'f,%.' + str(nd) + 'f'
    return 'M ' + ' L '.join(fmt % tuple(p) for p in pts) + (' Z' if closed else '')


def move_path(d, dx=NUI_MOVE[0], dy=NUI_MOVE[1]):
    """translate every coordinate pair of a path"""
    toks = re.findall(r'[MLQCSZ]|-?\d+(?:\.\d+)?', d)
    out, nums = [], []
    for t in toks:
        if t in 'MLQCSZ':
            out.append(t)
        else:
            nums.append(float(t))
            if len(nums) == 2:
                out.append('%g,%g' % (nums[0] + dx, nums[1] + dy))
                nums = []
    return ' '.join(out)


def mv(p):
    return [round(p[0] + NUI_MOVE[0], 1), round(p[1] + NUI_MOVE[1], 1)]


P['coast'] = catmull_rom_path(COAST_PTS)
coast = Polygon(sample(P['coast'], 0.8)).buffer(0)
ring = coast.exterior


def coast_between(a, b, step=0.6):
    """the coastline from the point nearest a to the point nearest b (the short way round), as a path"""
    s0, s1 = ring.project(Point(a)), ring.project(Point(b))
    if abs(s1 - s0) > ring.length / 2:
        raise ValueError('coast_between: the long way round')
    ln = substring(ring, min(s0, s1), max(s0, s1))
    pts = list(ln.simplify(0.3).coords)
    if s0 > s1:
        pts.reverse()
    return poly_path(pts, closed=False)


# ---- land the house blocks may use: the coast less the sea wall, promenade and coastal road
INLAND = 15.0
inland = coast.buffer(-INLAND, join_style='round')
P['inland'] = poly_path(inland.exterior.simplify(0.4).coords)

# ---- the river: straight south to the sea
P['river'] = dict(L['river'], path='M 312,-40 L 312,356')
P['hydrofoil'] = 'M 306,150 L 312,160 L 312,368 Q 312,380 326,382'

# ---- roads
ROAD_IN = 8.0                     # the coastal road runs 8 m inside the coast, as on the north and west
inner = coast.buffer(-ROAD_IN).exterior
a, b = inner.project(Point(286, 22)), inner.project(Point(286, 338))


def forward(s0, s1):
    """the ring from s0 to s1 in its own direction"""
    if s1 > s0:
        return list(substring(inner, s0, s1).coords)
    return list(substring(inner, s0, inner.length).coords) + list(substring(inner, 0, s1).coords)[1:]


# the way round the city (west of the river), not round the east bank
cands = [forward(a, b), list(reversed(forward(b, a)))]
coastal = LineString(min(cands, key=lambda c: max(p[0] for p in c)))
# keep the north and west of the coastal road as it was and carry it on round the south to Tôn Đức Thắng
old_coastal = next(r for r in L['roads'] if r.get('coastal'))
old_pts = sample(old_coastal['path'], 0.5)
s_join = coastal.project(Point(old_pts[-1]))
south = substring(coastal, s_join, coastal.length)
south_pts = list(south.simplify(0.35).coords)
south_pts[-1] = (286.0, round(south_pts[-1][1], 1))
coastal_path = old_coastal['path'] + ' ' + ' '.join('L %.1f,%.1f' % p for p in south_pts[1:])
SOUTH_Y = round(south_pts[-1][1], 1)

DROP = {'Thùy Vân (Bãi Sau)', 'Hạ Long', 'Trần Phú', 'Mai Chí Thọ'}
roads = []
for r in L['roads']:
    if r.get('name') in DROP:
        continue
    if r.get('coastal'):
        r = dict(r, path=coastal_path)
    elif r['path'] == 'M 286,250 C 286,292 316,316 342,344 L 366,372 Q 378,388 395,396':
        continue                                              # the old road down the curving river to Bãi Sau
    elif r.get('name') == 'Tôn Đức Thắng':
        r = dict(r, path='M 286,22 L 286,%g' % SOUTH_Y)
    elif r.get('name') == 'Quốc lộ 51':
        r = {'path': 'M 200,296 L 200,%g' % SOUTH_Y, 'w': 8}  # now a city street to the south coast
    elif r['path'] == 'M 59,296 L 59,330':
        r = dict(r, path='M 59,296 L 59,%g' % SOUTH_Y)
    roads.append(r)
old_halong = next(r for r in L['roads'] if r.get('name') == 'Hạ Long')
roads += [
    {'name': 'Mai Chí Thọ', 'path': 'M 338,66 L 392,66 C 418,66 430,80 430,106 L 430,262', 'w': 10,
     'label': [435.5, 200], 'vertical': True},
    {'name': 'Thùy Vân', 'path': 'M 430,262 C 430,276 455,274 455,292 L 455,374 Q 455,390 445,398', 'w': 10,
     'label': [458.5, 360], 'vertical': True},
    {'path': 'M 346,262 L 430,262', 'w': 8},                                        # Thủ Thiêm | Vũng Tàu
    {'name': 'Quang Trung', 'path': 'M 346,262 L 346,332 C 346,352 366,360 389,360', 'w': 8,
     'label': [347.4, 300], 'vertical': True},
    {'name': 'Quốc lộ 51', 'path': 'M 389,262 L 389,360', 'w': 8, 'label': [390.4, 300], 'vertical': True},
    dict(old_halong, path=move_path(old_halong['path'])),
]
P['roads'] = roads

# ---- walks, promenades
P['walks'] = []
for w in L['walks']:
    if w.get('name') == 'Bùi Viện':
        w = dict(w, path='M 100,270 L 280,270', w=15, label=[200, 272.4])
    P['walks'].append(w)
P['promenades'] = [
    {'path': 'M 338,22 L 338,338', 'w': 6},
    {'path': 'M 295,22 L 295,%g' % (SOUTH_Y - 4), 'w': 6},
    {'path': 'M 352,104 L 352,232', 'w': 3, 'kind': 'cycle'},
]

# ---- beaches: on the coastline itself
P['beaches'] = [
    {'id': 'bai_sau', 'path': coast_between((459, 413), (467, 276)), 'width': 24},
    {'id': 'bai_truoc', 'path': coast_between((364, 413), (329, 350)), 'width': 18},
]
P['rocks'] = [[*mv((252, 434)), 2.6], [*mv((182, 434)), 2.2], [*mv((226, 451)), 2.4],
              [466, 264, 2.2], [472, 272, 1.6], [462, 258, 1.8]]                      # and the headland north of Bãi Sau
# beach umbrellas on the dry sand of Bãi Sau, about 4.5 m in from the water, every 9 m
sau = LineString(sample(P['beaches'][0]['path'], 0.5))
umb = []
for k in range(int((sau.length - 6) / 9) + 1):
    s = 4 + 9 * k
    p, q = sau.interpolate(s), sau.interpolate(min(sau.length, s + 0.5))
    dx, dy = q.x - p.x, q.y - p.y
    dl = math.hypot(dx, dy) or 1.0
    nx, ny = -dy / dl, dx / dl
    off = 4.5 + (k * 7) % 3 * 0.6
    if not coast.contains(Point(p.x + nx * off, p.y + ny * off)):
        nx, ny = -nx, -ny
    umb.append([round(p.x + nx * off, 1), round(p.y + ny * off, 1)])
P['umbrellas'] = umb
P['boats'] = [[312, 372, -20], [318, 388, 10], [305, 398, -5], [320, 404, 25], [309, 414, -15], [312, 172, 90]]

# ---- mountains: Núi Lớn cut; Núi Nhỏ moved whole
P['mountains'] = []
for m in L['mountains']:
    if m['id'] != 'nui_nho':
        continue
    P['mountains'].append(dict(m, outline=move_path(m['outline']), stairs=move_path(m['stairs']),
                               peaks=[[*mv(p[:2]), *p[2:]] for p in m['peaks']],
                               designMove=list(NUI_MOVE)))

# ---- parks, plazas, lots
P['parks'] = [
    {'rect': [107, 241, 226, 252], 'kind': 'trees'},          # công viên 23/9 (Bùi Viện's north row is south of it)
    {'rect': [226, 243, 279, 252], 'kind': 'trees'},
    {'rect': [25, 111, 93, 169], 'kind': 'lawn'},
    {'rect': [358, 106, 422, 168], 'kind': 'lawn', 'r': 10},
    {'rect': [341, 110, 349, 160], 'kind': 'flowers'},
    {'rect': [341, 236, 360, 255], 'kind': 'trees'},          # rừng dừa nước by the river
    {'rect': [362, 236, 396, 255], 'kind': 'flowers'},
    {'rect': [439, 112, 451, 250], 'kind': 'trees'},          # a sea grove east of Mai Chí Thọ
    {'rect': [396, 304, 428, 330], 'kind': 'trees'},          # Vũng Tàu: square below the town, at the foot of the hill
    {'rect': [353, 306, 382, 345], 'kind': 'trees'},          # công viên Bãi Trước
]
P['plazas'] = [r for r in L['plazas'] if r != [374, 296, 392, 310]]
lots = []
for lt in L['lots']:
    r = lt['rect']
    if lt['kind'] == 'bar' or lt['kind'] == 'ks' or r[0] >= 330:
        continue                                              # Bùi Viện, the old hotels and Thủ Thiêm's south lots
    if r == [22, 303, 112, 317]:
        lt = dict(lt, rect=[22, 303, 112, 329])
    if r == [208, 303, 262, 329]:
        lt = dict(lt, rect=[208, 303, 279, 329])
    lots.append(lt)
lots += [
    {'rect': [107, 253, 226, 262], 'kind': 'bar'},            # Bùi Viện west of the v4 block, north side
    {'rect': [107, 278, 226, 289], 'kind': 'bar'},            # and south side
    {'rect': [432, 280, 447, 333], 'kind': 'ks'},             # hotels on Thùy Vân, facing Bãi Sau
    {'rect': [396, 269, 428, 300], 'kind': 'np'},             # Vũng Tàu town
    {'rect': [353, 269, 382, 301], 'kind': 'np'},
]
P['lots'] = lots

# ---- landmarks
CUT = {'bai_dau', 'nui_lon', 'bach_dinh'}
lms = []
for lm in L['landmarks']:
    if lm['id'] in CUT:
        continue
    lm = dict(lm)
    if lm['id'] in ('tuong_chua', 'hai_dang', 'bac_thang_nui_nho', 'mui_nghinh_phong'):
        lm['mark'] = mv(lm['mark'])
        if 'at' in lm:
            lm['at'] = mv(lm['at'])
    elif lm['id'] == 'bai_truoc':
        lm['mark'] = [340, 384]
    elif lm['id'] == 'bai_sau':
        lm['mark'] = [468, 336]
    elif lm['id'] == 'hon_ba':
        lm.update(at=[490, 410], mark=[499, 402], tideTo=[465, 405])
    elif lm['id'] == 'bui_vien':
        lm.update(at=[252.5, 270], size=[52, 34.6], face='E', mark=[236, 270], status='keep', model='scripts/blender/mini/bui_vien.py',
                  name='Phố đi bộ Bùi Viện')
    elif lm['id'] == 'nha_tho_thu_thiem':
        lm.update(at=[405, 243], mark=[405, 255])
    elif lm['id'] == 'rung_dua_nuoc':
        lm['mark'] = [350, 246]
    lms.append(lm)
P['landmarks'] = lms
P['flats'] = L.get('flats', [])
P['areas'] = [['NHÀ PHỐ &amp; HẺM', 22, 232, 6], ['CHỢ LỚN', 22, 340, 6], ['THỦ THIÊM', 372, 124, 7], ['VŨNG TÀU', 383, 322, 7],
              ['TRUNG TÂM', 200, 60, 6]]
P['seaLabel'] = [120, 400]

P['about'] = (L['about'].split(' Coast reshaped')[0] + ' Coast reshaped 2026-11-12 (scripts/map/reshape_coast.py); '
              'layout v7 the same day (scripts/map/relayout_v7.py): Vũng Tàu on the east bank, the south cut, the river '
              'straight to the sea, Bùi Viện v4 back.')
if '--compare' in sys.argv:
    P['compareCoast'] = L['coast']
json.dump(P, open(OUT, 'w', encoding='utf-8'), ensure_ascii=False, indent=1)
print('wrote', OUT, 'coast area %.0f m2 (v2 %.0f m2)' % (coast.area, Polygon(sample(L['coast'], 0.8)).buffer(0).area),
      'south coastal road ends at y', SOUTH_Y)
