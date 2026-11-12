"""Layout v8 (2026-11-12, approved by Khoa as a whole the same day, then the junctions he marked fixed): Sài Gòn mini
redesigned as one readable game map. It is the current layout.

    python3 scripts/map/relayout_v8.py [out.json]      (needs shapely; default out: scripts/map/saigon_mini_layout.json)
    python3 scripts/map/check_layout.py                 (road ends, bridge landings, roads into buildings: must be 0)

Reads layout v7 (scripts/map/saigon_mini_layout_v7.json, kept as it was) for the landmark records and the approved
models (Núi Nhỏ, Cầu Ba Son, Bùi Viện) and lays the map out again. Design notes: docs/map-v8-design.md. In short:

- Water gives the structure (Lynch's edges): Sông Sài Gòn comes in from the north, runs past the city and bends east
  round Thủ Thiêm to the sea, as the real river does; Kênh Bến Nghé leaves it at the bend and runs south-west to the
  sea. Three land masses, each one district with its own character: the city (Quận 1, Chợ Lớn), Thủ Thiêm (new town,
  parks, towers), and Khánh Hội – Vũng Tàu (port, beaches, the hill).
- Five bridges are the nodes between them and make one sightseeing loop: Cầu Ba Son, the Nguyễn Huệ footbridge (cầu
  đi bộ lá dừa nước), Cầu Thủ Thiêm 3, Cầu Mống and Cầu Ông Lãnh.
- Axes end on landmarks (Disney's weenies): Đồng Khởi from Nhà thờ Đức Bà to the river; the park axis from Dinh Thống
  Nhất through Công viên 30/4 to Công xã Paris and Hồ Con Rùa; Lê Duẩn from Hồ Con Rùa to Cầu Ba Son and Thủ Thiêm's
  tower; Nguyễn Huệ from UBND across the footbridge to Thủ Thiêm; Lê Lợi between Nhà hát and Bến Thành.
- A street hierarchy: boulevards (12 m, trees), streets (8–10 m), walking streets; fewer, bigger house blocks; parks
  at the nodes (30/4, Tao Đàn, 23/9) instead of leftover lawns.
- Tall landmarks visible from anywhere: Bitexco and the Thủ Thiêm tower in the middle, the Christ statue and the
  lighthouse on Núi Nhỏ in the south-east, the Ba Son pylon in the north.
Plan coordinates as in the layout file: x east, y south, metres.
"""
import copy
import json
import math
import os
import sys

from shapely.geometry import LineString, Point, Polygon, box
from shapely.ops import nearest_points, substring, unary_union

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
from plan_paths import catmull_rom_path, move_path, poly_path, sample  # noqa: E402

SRC = os.path.join(HERE, 'saigon_mini_layout_v7.json')
OUT = sys.argv[1] if len(sys.argv) > 1 and not sys.argv[1].startswith('--') else os.path.join(HERE, 'saigon_mini_layout.json')
L = json.load(open(SRC, encoding='utf-8'))
LM = {lm['id']: lm for lm in L['landmarks']}
P = {'about': '', 'toBlender': L['toBlender']}

# ------------------------------------------------------------------------------------------------ water and land
COAST_PTS = [
    (72, 19), (128, 11), (186, 8), (240, 14), (268, 18), (300, 8), (334, 8),                    # north: the city, river
    (384, 15), (428, 26), (453, 50), (461, 88), (456, 134), (465, 182), (462, 226), (468, 256),  # Thủ Thiêm
    (478, 296), (480, 330), (477, 362), (471, 388), (462, 408), (451, 423),                      # river mouth, Bãi Sau
    (441, 433), (427, 449), (403, 455), (381, 446), (369, 428), (361, 404),                      # Núi Nhỏ cape (v7)
    (334, 398), (305, 388), (278, 384), (254, 380), (238, 370),                                  # Bãi Trước, facing SW
    (224, 354), (210, 338),                                                                      # Khánh Hội west shore
    (176, 324), (138, 326), (100, 325), (62, 322), (36, 316), (20, 300),                         # the city's south coast
    (20, 270), (33, 236), (24, 200), (14, 162), (17, 118), (25, 80), (42, 44),                   # west coast
]
RIVER = 'M 312,-40 L 312,160 C 312,214 318,244 346,262 C 376,281 420,282 455,276 C 478,272 500,272 540,274'
CANAL = 'M 307,230 C 290,248 264,266 242,282 C 222,296 204,308 182,326'
RIVER_W, CANAL_W = 28, 16

P['coast'] = catmull_rom_path(COAST_PTS)
coast = Polygon(sample(P['coast'], 0.8)).buffer(0)
river_ln = LineString(sample(RIVER, 0.5))
canal_ln = LineString(sample(CANAL, 0.5))
water = unary_union([river_ln.buffer(RIVER_W / 2, cap_style='flat'), canal_ln.buffer(CANAL_W / 2, cap_style='flat')])
land = coast.difference(water)
_parts = sorted(getattr(land, 'geoms', [land]), key=lambda p: -p.area)
CITY = next(p for p in _parts if p.contains(Point(150, 150)))
THU_THIEM = next(p for p in _parts if p.contains(Point(400, 130)))
SOUTH = next(p for p in _parts if p.contains(Point(380, 340)))
P['river'] = {'path': RIVER, 'width': RIVER_W, 'shallows': 38}
P['canals'] = [{'id': 'ben_nghe', 'name': 'Kênh Bến Nghé', 'path': CANAL, 'width': CANAL_W, 'shallows': 22}]
INLAND = 15.0
P['inland'] = poly_path(coast.buffer(-INLAND, join_style='round').exterior.simplify(0.4).coords)


def ring_road(poly, a, b, off=8.0, avoid=None):
    """the land's edge road (off m inside) from the point nearest a to the point nearest b, the way that keeps
    clear of `avoid` (a point the road must not pass near)"""
    ring = poly.buffer(-off).exterior
    s0, s1 = ring.project(Point(a)), ring.project(Point(b))

    def fwd(u, v):
        if v > u:
            return list(substring(ring, u, v).coords)
        return list(substring(ring, u, ring.length).coords) + list(substring(ring, 0, v).coords)[1:]
    cands = [fwd(s0, s1), list(reversed(fwd(s1, s0)))]
    if avoid is not None:
        cands.sort(key=lambda c: -LineString(c).distance(Point(avoid)))
    return LineString(cands[0]).simplify(0.35)


def split(line, at_point):
    s = line.project(Point(at_point))
    return substring(line, 0, s), substring(line, s, line.length)


def path_of(line):
    return poly_path(list(line.coords), closed=False)


def canal_bridge(id_, nm, near, w, deck, walk=False):
    """a bridge square across Kênh Bến Nghé near a point: a footbridge from pavement to pavement (it must not come
    down on a carriageway), a car bridge from the middle of Võ Văn Kiệt to the middle of Bến Vân Đồn, level with the
    roads for the junction and the pavement (flat), then a short ramp up over the water"""
    c = canal_ln.interpolate(canal_ln.project(Point(near)))
    s = canal_ln.project(c)
    a, b = canal_ln.interpolate(max(0, s - 1)), canal_ln.interpolate(min(canal_ln.length, s + 1))
    tx, ty = b.x - a.x, b.y - a.y
    tl = math.hypot(tx, ty)
    nx, ny = -ty / tl, tx / tl                 # across the canal
    if nx + ny < 0:
        nx, ny = -nx, -ny                      # pointing to the Khánh Hội side (south-east)
    if walk:
        d0, d1 = CANAL_W / 2 + 2.0, CANAL_W / 2 + 3.0     # on the bank-side pavement each side
        return {'id': id_, 'name': nm, 'path': 'M %.1f,%.1f L %.1f,%.1f' % (c.x - nx * d0, c.y - ny * d0, c.x + nx * d1, c.y + ny * d1),
                'w': w, 'deck': deck, 'walk': True, 'ramp': 7}
    probe = LineString([(c.x - nx * 40, c.y - ny * 40), (c.x + nx * 40, c.y + ny * 40)])
    p0 = probe.intersection(LineString(sample(VVK_PATH, 0.5)))
    p1 = probe.intersection(LineString(sample(BVD_PATH, 0.5)))
    return {'id': id_, 'name': nm, 'path': 'M %.1f,%.1f L %.1f,%.1f' % (p0.x, p0.y, p1.x, p1.y), 'w': w, 'deck': deck,
            'flat': [8, 8], 'ramp': 5}


# ------------------------------------------------------------------------------------------------ the city (Quận 1)
city_ring = ring_road(CITY, (286, 22), (288, 228), avoid=(300, 120))          # north, west and south coast, canal
coastal, vvk = split(city_ring, (186, 316))                                   # Võ Văn Kiệt from the canal mouth
VVK_PATH = path_of(vvk)
roads = [
    {'name': 'Đường ven biển', 'path': path_of(coastal), 'w': 8, 'label': [96, 26.6], 'coastal': True},
    {'name': 'Võ Văn Kiệt', 'path': path_of(vvk), 'w': 10},
    {'name': 'Tôn Đức Thắng', 'path': 'M 286,22 L 286,228', 'w': 10, 'label': [287.4, 132], 'vertical': True},
    {'name': 'Lê Duẩn', 'path': 'M 38,66 L 286,66', 'w': 12, 'label': [44, 64.6], 'boulevard': True},
    {'name': 'Đồng Khởi', 'path': 'M 151,104 L 286,104', 'w': 10, 'label': [252, 102.6], 'boulevard': True},
    {'name': 'Lê Lợi', 'path': 'M 240,104 L 240,197', 'w': 10, 'label': [241.4, 176], 'vertical': True, 'boulevard': True},
    {'name': 'Hàm Nghi', 'path': 'M 249,206 L 286,206', 'w': 10, 'label': [255, 204.6]},
    {'name': 'Lê Lai', 'path': 'M 30,236 L 223.8,235.9 L 226.7,235.5 L 229.3,234.6 L 231.6,233.2 L 233.5,231.4 L 235.1,229.2 L 236.4,226.6 L 237.4,223.5 L 238.0,220.0',
     'w': 10, 'label': [62, 234.6]},
    {'name': 'Phạm Ngọc Thạch', 'path': 'M 140,22 L 140,93', 'w': 9, 'label': [141.4, 50], 'vertical': True},
    {'name': 'Lê Thánh Tôn', 'path': 'M 196,22 L 196,176', 'w': 9, 'label': [197.4, 128], 'vertical': True},
    {'name': 'Lý Tự Trọng', 'path': 'M 171,176 L 240,176', 'w': 8, 'label': [205, 174.6]},
    {'name': 'Nam Kỳ Khởi Nghĩa', 'path': 'M 100,142 L 100,236', 'w': 9, 'label': [101.4, 226], 'vertical': True},
    {'path': 'M 30,142 L 100,142', 'w': 8},
    {'path': 'M 59,30 L 59,66', 'w': 7},
    {'name': 'Đề Thám', 'path': 'M 182,236 L 182,318', 'w': 8, 'label': [183.4, 312], 'vertical': True},
    {'path': 'M 171,176 L 171,236', 'w': 8},
]
walks = [
    {'name': 'Phố đi bộ Nguyễn Huệ', 'path': 'M 189,140 L 286,140', 'w': 16, 'planters': True, 'label': [205, 142.6]},
    {'name': 'Công viên 30/4', 'path': 'M 140,115 L 140,172', 'w': 8},
    {'name': 'Bùi Viện', 'path': 'M 80,270 L 182,270', 'w': 15, 'label': [104, 272.4]},     # from the Chợ Lớn square to Đề Thám
    {'path': 'M 32,226 C 48,212 60,196 92,184', 'w': 3},          # paths through Tao Đàn
    {'path': 'M 62,182 L 62,228', 'w': 3},
    {'path': 'M 110,118 L 128,164', 'w': 2.5}, {'path': 'M 168,118 L 151,164', 'w': 2.5},     # and 30/4
]

# ------------------------------------------------------------------------------------------------ Thủ Thiêm
roads += [
    {'name': 'Mai Chí Thọ', 'path': 'M 338,66 L 392,66 C 418,66 430,80 430,106 L 430,252', 'w': 10, 'label': [435.5, 200],
     'vertical': True, 'boulevard': True},
]
walks += [
    {'name': 'Quanh hồ', 'path': 'M 364,206 C 364,190 378,180 394,180 C 410,180 420,190 420,206 C 420,222 410,230 394,230 C 378,230 364,222 364,206 Z', 'w': 4},
    {'path': 'M 362,206 L 364,206', 'w': 5},
    {'path': 'M 337,140 L 358,140', 'w': 7},                 # the footbridge landing into the park
    {'path': 'M 372,140 L 404,140', 'w': 5},                 # on along the axis, past the pool, to the tower
]

# ------------------------------------------------------------------------------------------------ Khánh Hội – Vũng Tàu
NUI_MOVE = (189.0, 0.0)
nui = next(m for m in L['mountains'] if m['id'] == 'nui_nho')     # already at its v7 place (designMove)
halong = next(r for r in L['roads'] if r.get('name') == 'Hạ Long')
south_ring = ring_road(SOUTH, (432, 300), (368, 408), off=12.0, avoid=(470, 360))
ntt, rest = split(south_ring, (309, 262))                         # Nguyễn Tất Thành along the river
bvd, _ = split(rest, (222, 330))                                   # Bến Vân Đồn along the canal
BVD_PATH = path_of(bvd)
# Quang Trung, the seafront boulevard of Bãi Trước, further in so the beach is wide: 17 m from the water
_qt_ring = ring_road(SOUTH, (432, 300), (368, 408), off=17.0, avoid=(470, 360))
qt = LineString([bvd.coords[-1]] + list(substring(_qt_ring, _qt_ring.project(Point(226, 338)), _qt_ring.length).coords)).simplify(0.35)
roads += [
    {'name': 'Nguyễn Tất Thành', 'path': path_of(ntt), 'w': 10},
    {'name': 'Bến Vân Đồn', 'path': path_of(bvd), 'w': 10},
    {'name': 'Quang Trung', 'path': path_of(qt), 'w': 10, 'boulevard': True},
    {'name': 'Thùy Vân', 'path': 'M 430,305 C 430,320 455,320 455,336 L 455,374 Q 455,390 445,398', 'w': 10,
     'label': [458.5, 372], 'vertical': True},
    {'name': 'Quốc lộ 51', 'path': 'M 389,300 L 389,360', 'w': 8, 'label': [390.4, 352], 'vertical': True},
    {'name': 'Ba Mươi Tháng Tư', 'path': path_of(LineString([(389, 338), qt.interpolate(qt.project(Point(228, 338))).coords[0]])),
     'w': 8, 'label': [300, 336.6]},
    {'path': 'M 344,286 L 344,338', 'w': 7},
    dict(halong),
    # the start of Đường Hải Đăng (nui_nho.py traces it up the hill from (398, 347)) joined to Quốc lộ 51
    {'path': 'M 389,344.6 L 398.5,347.1', 'w': 4, 'endsAt': 'nui_nho_road'},
]

P['roads'] = roads
P['walks'] = walks
P['loops'] = []
P['roundabouts'] = [
    {'id': 'quach_thi_trang', 'c': [240, 206], 'r': 9, 'inner': 'grassDark'},
    {'id': 'ho_con_rua', 'c': [140, 66], 'r': 11, 'inner': 'paving', 'model': True},
    {'id': 'cong_xa_paris', 'c': [140, 104], 'r': 6, 'inner': 'grassDark'},
    {'id': 'lam_son', 'c': [240, 104], 'r': 5, 'inner': 'paving'},
]
P['bridges'] = [
    {'id': 'cau_ba_son', 'name': 'Cầu Ba Son', 'path': 'M 280,66 L 344,66', 'w': 9, 'deck': 3.2, 'pylon': [330, 66]},
    {'id': 'cau_di_bo', 'name': 'Cầu đi bộ Nguyễn Huệ – Thủ Thiêm', 'path': 'M 294.5,140 L 337,140', 'w': 6, 'wMid': 11, 'deck': 2.8, 'walk': True},
    {'id': 'cau_thu_thiem_3', 'name': 'Cầu Thủ Thiêm 3', 'path': 'M 430,252 L 430,305', 'w': 10, 'deck': 2.4, 'flat': [0, 8], 'ramp': 12},
    canal_bridge('cau_mong', 'Cầu Mống', (296, 241), 6, 2.2, walk=True),
    canal_bridge('cau_ong_lanh', 'Cầu Ông Lãnh', (250, 276), 9, 1.8),
]
P['promenades'] = [
    {'path': 'M 295,22 L 295,224', 'w': 6},                                         # Bến Bạch Đằng, the city bank
    {'path': 'M 338,22 L 338,232', 'w': 6},                                         # Thủ Thiêm riverside park
    {'path': 'M 352,104 L 352,232', 'w': 3, 'kind': 'cycle'},
]
P['mountains'] = [nui]
P['beaches'] = []
P['parks'] = [
    {'rect': [108, 114, 131, 168], 'kind': 'trees', 'name': 'Công viên 30/4'},
    {'rect': [149, 114, 170, 168], 'kind': 'trees', 'name': 'Công viên 30/4'},
    {'rect': [131, 114, 149, 168], 'kind': 'lawn'},
    {'rect': [104, 184, 118, 230], 'kind': 'trees'}, {'rect': [162, 184, 167, 230], 'kind': 'trees'},
    {'rect': [30, 182, 94, 228], 'kind': 'trees', 'name': 'Công viên Tao Đàn'},
    {'rect': [106, 241, 176, 252], 'kind': 'trees', 'name': 'Công viên 23/9'},
    {'rect': [104, 292, 176, 309], 'kind': 'trees', 'name': 'Bến Bình Đông'},
    {'rect': [372, 106, 400, 168], 'kind': 'lawn', 'r': 8},
    {'rect': [358, 106, 372, 128], 'kind': 'trees'}, {'rect': [358, 152, 372, 172], 'kind': 'trees'},
    {'rect': [340, 232, 360, 254], 'kind': 'trees'}, {'rect': [400, 254, 428, 262], 'kind': 'trees'},
    {'rect': [341, 108, 349, 132], 'kind': 'flowers'},
    {'rect': [341, 148, 349, 172], 'kind': 'flowers'},
    {'rect': [362, 236, 396, 252], 'kind': 'flowers'},
    {'rect': [439, 112, 451, 244], 'kind': 'trees'},
    {'rect': [366, 28, 398, 58], 'kind': 'trees'},                                    # Thủ Thiêm north: a garden by the towers
    {'rect': [172, 109, 190, 126], 'kind': 'trees'},                                  # UBND's garden
    {'rect': [349, 300, 384, 332], 'kind': 'trees', 'name': 'Rừng dừa nước'},
    {'rect': [250, 344, 296, 368], 'kind': 'trees', 'name': 'Công viên Bãi Trước'},

]
P['lakes'] = [{'ellipse': [392, 205, 20, 15], 'fountain': True}, {'ellipse': [388, 130, 9, 4.5]}, {'ellipse': [388, 150, 9, 4.5]}]
P['plazas'] = [[218, 183, 234, 229], [346, 72, 388, 100], [342, 196, 362, 216], [338, 128, 372, 152], [300, 264, 316, 278],
               [50, 263, 82, 281]]                                                   # Quảng trường Chợ Lớn, in front of the temple
# Thủ Thiêm's tall tower stands at the end of the Nguyễn Huệ axis, across the river (the map's weenie); three lower
# towers stay in the north by the Ba Son landing
P['towers'] = [[404, 128, 424, 152, 72]] + [t for t in L['towers'] if t[4] < 70]
P['grid'] = {'comment': 'v8: every house block is an explicit lot', 'inset': 7, 'cells': []}
P['lots'] = [
    # north of Lê Duẩn
    {'rect': [64, 30, 133, 59], 'kind': 'hem'},
    {'rect': [147, 29, 189, 59], 'kind': 'np'}, {'rect': [203, 29, 250, 59], 'kind': 'np'},
    # Lê Duẩn – Đồng Khởi
    {'rect': [44, 73, 80, 97], 'kind': 'hem'}, {'rect': [203, 73, 226, 97], 'kind': 'np'}, {'rect': [254, 73, 279, 97], 'kind': 'np'},
    # Đồng Khởi – Nguyễn Huệ
    {'rect': [42, 111, 93, 138], 'kind': 'hem'}, {'rect': [37, 149, 93, 174], 'kind': 'hem'}, {'rect': [203, 111, 233, 133], 'kind': 'np'}, {'rect': [247, 111, 279, 133], 'kind': 'np'},
    # Nguyễn Huệ – Hàm Nghi

    # Lê Lai – the coast (Chợ Lớn, Bùi Viện)
    # Chợ Lớn: shophouses round the temple square, the bus station on the coast road
    {'rect': [38, 264, 49, 302], 'kind': 'cl'}, {'rect': [77, 244, 95, 260], 'kind': 'cl'},
    {'rect': [84, 280, 97, 306], 'kind': 'cl'},
    {'rect': [104, 255, 117, 286], 'kind': 'bar'},
    {'rect': [189, 243, 232, 262], 'kind': 'np'}, {'rect': [247, 213, 280, 228], 'kind': 'np'},
    # Khánh Hội: the port quarter behind Nhà Rồng
    {'rect': [272, 296, 296, 330], 'kind': 'np'}, {'rect': [318, 296, 338, 330], 'kind': 'np'},
    # Vũng Tàu: hotels facing Bãi Trước, the town between the two beaches
    {'rect': [300, 345, 325, 370], 'kind': 'ks'}, {'rect': [331, 345, 355, 370], 'kind': 'ks'}, {'rect': [361, 345, 384, 370], 'kind': 'ks'},
    {'rect': [396, 312, 424, 330], 'kind': 'ks'},
    # Thủ Thiêm: new apartment blocks along Mai Chí Thọ
    {'rect': [405, 106, 423, 121], 'kind': 'ks'}, {'rect': [405, 159, 423, 172], 'kind': 'ks'},
]

# ------------------------------------------------------------------------------------------------ landmarks
def lm(id_, **kw):
    d = copy.deepcopy(LM[id_]) if id_ in LM else {'id': id_}
    d.update(kw)
    return d


P['landmarks'] = [
    lm('cho_ben_thanh'), lm('bung_binh_quach_thi_trang'), lm('nha_hat'), lm('buu_dien'), lm('ho_con_rua'), lm('ubnd'),
    lm('nha_tho_duc_ba', at=[108, 104], size=[38, 23], face='E', mark=[108, 104]),
    lm('nguyen_hue'), lm('chung_cu_42'), lm('bitexco'),
    lm('ben_bach_dang', at=[304, 176], mark=[302, 192]),
    lm('cau_ba_son'),
    lm('bui_vien', at=[144, 270], mark=[128, 270]),
    lm('cong_vien_23_9', mark=[140, 246]),
    lm('ben_nha_rong', at=[307, 290], size=[16, 22], face='N', mark=[307, 290], status='new'),
    lm('dinh_doc_lap', at=[140, 205], size=[40, 22], face='N', drive=33, mark=[140, 205]),
    lm('cho_lon', at=[66, 252], size=[16, 16], face='S', mark=[66, 252]),
    lm('ben_xe', at=[66, 297], size=[28, 18], face='S', mark=[66, 297]),
    lm('bac_thang_nui_nho'), lm('tuong_chua'), lm('hai_dang'),
    lm('bai_truoc', mark=[262, 372]), lm('bai_sau', mark=[468, 346]),
    lm('hon_ba'), lm('mui_nghinh_phong'),
    lm('me_linh'), lm('cong_xa_paris'),
    lm('thu_thiem', mark=[372, 140]),
    lm('cong_vien_sang_tao'), lm('thap_thu_thiem', mark=[414, 140], name='Tháp Thủ Thiêm (cuối trục Nguyễn Huệ, ~72 m)'),
    lm('nha_tho_thu_thiem', at=[405, 241], mark=[405, 241]),
    lm('ben_buyt_song', at=[333, 190], mark=[333, 190]),
    lm('rung_dua_nuoc', mark=[360, 300]),
    {'n': 37, 'id': 'cau_mong', 'name': 'Cầu Mống (cầu đi bộ, Eiffel 1894)', 'status': 'new', 'mark': [296, 241]},
    {'n': 38, 'id': 'cau_di_bo', 'name': 'Cầu đi bộ lá dừa nước', 'status': 'new', 'mark': [312, 134]},
    {'n': 39, 'id': 'cong_vien_30_4', 'name': 'Công viên 30/4', 'status': 'new', 'mark': [140, 128]},
    {'n': 40, 'id': 'tao_dan', 'name': 'Công viên Tao Đàn', 'status': 'new', 'mark': [62, 205]},
    {'n': 41, 'id': 'cau_thu_thiem_3', 'name': 'Cầu Thủ Thiêm 3', 'status': 'new', 'mark': [436, 280]},
]
P['flats'] = [[140, 177, 14]]
# Tuyến tham quan: one loop through every district and over the five bridges (drawn on the plan; later the game's
# guide route). Bến Bạch Đằng -> Nguyễn Huệ -> UBND -> Nhà thờ, Bưu điện -> Công viên 30/4 -> Dinh -> Bến Thành ->
# Bùi Viện -> Cầu Ông Lãnh -> Khánh Hội -> Bãi Trước -> Núi Nhỏ -> Bãi Sau -> Cầu Thủ Thiêm 3 -> Thủ Thiêm ->
# cầu đi bộ -> Bến Bạch Đằng
P['tour'] = poly_path([(300, 182), (290, 176), (290, 146), (196, 146), (196, 110), (151, 110), (144, 116), (144, 176),
                       (176, 176), (176, 232), (186, 232), (186, 262), (176, 266), (112, 266), (112, 274), (186, 274),
                       (186, 300), (200, 291), (238, 263), (262, 294), (250, 304), (226, 334), (250, 366), (290, 374),
                       (330, 384), (362, 396), (372, 420), (392, 446), (420, 444), (446, 420), (452, 380), (452, 330),
                       (434, 310), (434, 250), (424, 214), (394, 232), (366, 212), (352, 160), (338, 144), (290, 144),
                       (300, 182)], closed=False)
P['signals'] = [[196, 66], [240, 66], [196, 104], [100, 66], [100, 236], [182, 236], [286, 66], [286, 104], [286, 206],
                [140, 236]]
P['rocks'] = [r for r in L['rocks'] if r[1] > 300] + [[233, 372, 2.4], [228, 366, 1.8], [240, 378, 1.6], [470, 250, 2.0], [475, 258, 1.5]]
P['boats'] = [[262, 402, 20], [284, 410, -10], [244, 396, 35], [306, 414, 5], [228, 388, 50], [313, 196, 90], [262, 268, 37]]
P['hydrofoil'] = 'M 306,182 L 314,214 C 300,240 268,262 244,280 C 222,296 200,312 188,330 Q 220,392 280,414'
P['areas'] = [['TRUNG TÂM', 200, 62, 6], ['NHÀ PHỐ &amp; HẺM', 36, 147, 5], ['CHỢ LỚN', 20, 336, 6], ['THỦ THIÊM', 372, 124, 7],
              ['KHÁNH HỘI', 266, 324, 5], ['VŨNG TÀU', 391, 334, 7]]
P['seaLabel'] = [80, 400]

# beaches on the coastline itself
ring = coast.exterior


def coast_between(a, b):
    s0, s1 = ring.project(Point(a)), ring.project(Point(b))
    ln = substring(ring, min(s0, s1), max(s0, s1))
    pts = list(ln.simplify(0.3).coords)
    return poly_path(pts if s0 < s1 else pts[::-1], closed=False)


P['beaches'] = [
    {'id': 'bai_sau', 'path': coast_between((462, 410), (479, 318)), 'width': 24},
    {'id': 'bai_truoc', 'path': coast_between((356, 403), (244, 376)), 'width': 26},
]
# beach umbrellas on the dry sand, about 5 m in from the water, every 9 m (Bãi Sau) or 11 m (Bãi Trước, calmer)
umb = []
for b, step, skip in ((P['beaches'][0], 9, None), (P['beaches'][1], 11, (288, 384))):
    sau = LineString(sample(b['path'], 0.5))
    for k in range(int((sau.length - 6) / step) + 1):
        s_ = 4 + step * k
        p, q = sau.interpolate(s_), sau.interpolate(min(sau.length, s_ + 0.5))
        if skip and math.dist((p.x, p.y), skip) < 9:          # keep the pier clear
            continue
        dx, dy = q.x - p.x, q.y - p.y
        dl = math.hypot(dx, dy) or 1.0
        nx, ny = -dy / dl, dx / dl
        off = 4.5 + (k * 7) % 3 * 0.6
        if not coast.contains(Point(p.x + nx * off, p.y + ny * off)):
            nx, ny = -nx, -ny
        umb.append([round(p.x + nx * off, 1), round(p.y + ny * off, 1)])
P['umbrellas'] = umb
# piers: the hydrofoil pier on Bãi Trước (tàu cánh ngầm from Bạch Đằng), a fishing jetty at Chợ Lớn
P['piers'] = [{'id': 'cau_tau_bai_truoc', 'path': 'M 291,378 L 281,412', 'w': 4, 'kiosk': True},
              {'id': 'ben_binh_dong', 'path': 'M 120,318 L 118,338', 'w': 3}]
P['about'] = ('Sài Gòn mini layout v8 (scripts/map/relayout_v8.py, docs/map-v8-design.md): the river bends round '
              'Thủ Thiêm, Kênh Bến Nghé, three districts joined by five bridges, axes ending on landmarks. Units: metres, '
              'plan x east, y south; Blender XY = (x - toBlender.x, toBlender.y - y).')

def _pts(r):
    """a road's centreline as a list of points: exact for M/L paths, sampled for curves"""
    d = r['path']
    if not any(c in d for c in 'QCS'):
        import re
        nums = [float(t) for t in re.findall(r'-?\d+(?:\.\d+)?', d)]
        return [(nums[i], nums[i + 1]) for i in range(0, len(nums), 2)]
    return list(LineString(sample(d, 0.5)).simplify(0.15).coords)


def tidy_roads(roads, bridges, roundabouts, rounds=3):
    """Khoa's review (2026-11-12): roads stopped short of the road they join, or ran a few metres past it, and the arms
    of the roundabouts stopped at the ring, leaving notches. For every road end: into a roundabout, go to its centre;
    already on another road (or a bridge), keep; a junction within the last 20 m, cut the stub there; another road
    straight ahead within 15 m, carry on to it; another road within 4 m, move the end onto it."""
    rings = [(Point(ra['c']), ra['r'] + 5 + 4) for ra in roundabouts]
    for _ in range(rounds):
        changed = 0
        for k, r in enumerate(roads):
            pts = _pts(r)
            others = [LineString(_pts(o)) for j, o in enumerate(roads) if j != k] + \
                     [LineString(sample(b['path'], 0.5)) for b in bridges]
            other = unary_union(others)
            for at_start in (True, False):
                if r.get('endsAt'):
                    if at_start is False:
                        continue                                    # this end joins a road built by a part script
                ln = LineString(pts)
                end = Point(pts[0] if at_start else pts[-1])
                ring = next((c for c, rr in rings if 0.5 < end.distance(c) <= rr), None)
                if ring is not None:
                    pts = [ring.coords[0]] + pts if at_start else pts + [ring.coords[0]]
                    changed += 1
                    continue
                if other.distance(end) < 0.3:
                    continue
                # a stub past a junction: the road crosses another one in its last 20 m
                s0, s1 = (0.0, min(20.0, ln.length)) if at_start else (max(0.0, ln.length - 20.0), ln.length)
                tail = substring(ln, s0, s1)
                cross = tail.intersection(other)
                cps = [g for g in getattr(cross, 'geoms', [cross]) if g.geom_type == 'Point' and g.distance(end) > 0.3]
                if cps:
                    cut = min(cps, key=lambda g: g.distance(end))
                    s = ln.project(cut)
                    keep = substring(ln, s, ln.length) if at_start else substring(ln, 0, s)
                    pts = list(keep.coords)
                    changed += 1
                    continue
                # carry on straight to the next road
                a_, b_ = (pts[1], pts[0]) if at_start else (pts[-2], pts[-1])
                dx, dy = b_[0] - a_[0], b_[1] - a_[1]
                dl = math.hypot(dx, dy) or 1.0
                ray = LineString([end.coords[0], (end.x + dx / dl * 15, end.y + dy / dl * 15)])
                hit = ray.intersection(other)
                hp = [g for g in getattr(hit, 'geoms', [hit]) if g.geom_type == 'Point']
                if hp:
                    q = min(hp, key=lambda g: g.distance(end)).coords[0]
                    pts = [q] + pts if at_start else pts + [q]
                    changed += 1
                    continue
                if other.distance(end) < 4.0:
                    q = nearest_points(other, end)[0].coords[0]
                    pts = [q] + pts[1:] if at_start else pts[:-1] + [q]
                    changed += 1
            if pts != _pts(r):
                r['path'] = poly_path([(round(x, 1), round(y, 1)) for x, y in pts], closed=False)
        if not changed:
            break


tidy_roads(P['roads'], P['bridges'], P['roundabouts'])

if __name__ == '__main__':
    json.dump(P, open(OUT, 'w', encoding='utf-8'), ensure_ascii=False, indent=1)
    print('wrote', OUT)
    print('land masses m2: city %.0f, Thủ Thiêm %.0f, Khánh Hội–Vũng Tàu %.0f, total %.0f' % (CITY.area, THU_THIEM.area, SOUTH.area, land.area))
