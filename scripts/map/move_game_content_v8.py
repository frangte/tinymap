"""Moves the game's content from the v4 island to Sài Gòn mini (layout v8): the places of the Cẩm nang app, the NPCs,
and where each archetype starts. Ids stay the same, so quests and saved characters keep working. Also writes the
phone map's land and roads (src/game/phone/mapLand.json). Run once, and again after changing the layout:

    python3 scripts/map/move_game_content_v8.py      (needs shapely; edits server/content/*.json in place)

Every point comes from the layout (scripts/map/saigon_mini_layout.json, plan metres) through to_blender(); NPCs and
spawns stand on a pavement or square next to their place, facing where they faced on the v4 island (the street, the
market gate, the group). Only the numbers and a few notes that described the old island change; the files keep their
layout, one record per line where they had it.
"""
import json
import os
import re
import sys

from shapely.geometry import LineString, MultiPolygon, Point, Polygon
from shapely.ops import substring, unary_union

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.abspath(os.path.join(HERE, '..', '..'))
sys.path.insert(0, HERE)
from plan_paths import sample  # noqa: E402

CONTENT = os.path.join(ROOT, 'server', 'content')
MAP_LAND = os.path.join(ROOT, 'src', 'game', 'phone', 'mapLand.json')
L = json.load(open(os.path.join(HERE, 'saigon_mini_layout.json'), encoding='utf-8'))
TX, TY = L['toBlender']['x'], L['toBlender']['y']
LM = {lm['id']: lm for lm in L['landmarks']}
ROAD = {r['name']: r for r in L['roads'] if r.get('name')}
WALK = {w['name']: w for w in L['walks'] if w.get('name')}


def to_blender(p):
    return [round(p[0] - TX, 1), round(TY - p[1], 1)]


def fmt(v):
    return ('%g' % v)


def pt(p):
    return '[%s, %s]' % (fmt(p[0]), fmt(p[1]))


COAST = Polygon(sample(L['coast'], 0.8)).buffer(0)
# the dry sand: the beach paths run along the waterline, the sand above the sea is the next 5–10 m inland
SAND = 8.0


def inland(path, d=SAND):
    """a beach path moved `d` m inland, onto the dry sand, west to east / south to north as drawn"""
    ln = LineString(sample(path, 0.5))
    for side in ('left', 'right'):
        off = ln.parallel_offset(d, side, join_style=2)
        if off.geom_type == 'LineString' and COAST.contains(off.interpolate(0.5, normalized=True)):
            if Point(off.coords[0]).distance(Point(ln.coords[0])) > Point(off.coords[-1]).distance(Point(ln.coords[0])):
                off = LineString(list(off.coords)[::-1])
            return off
    raise SystemExit('no inland side for ' + path[:30])


def sea_front():
    """Vũng Tàu's sea front, one polyline: Bãi Trước (west to east) on its sand, Hạ Long round the foot of Núi Nhỏ,
    Bãi Sau (south to north) on its sand"""
    bch = {b['id']: b['path'] for b in L['beaches']}
    front = list(inland(bch['bai_truoc']).coords)
    front = front[::-1] if front[0][0] > front[-1][0] else front
    back = list(inland(bch['bai_sau']).coords)
    back = back[::-1] if back[0][1] < back[-1][1] else back
    # Hạ Long round the hill's south tip, from the end of Bãi Trước to the start of Bãi Sau (it is drawn the other way)
    hl = LineString(sample(ROAD['Hạ Long']['path'], 0.5))
    a, b = sorted((hl.project(Point(back[0])), hl.project(Point(front[-1]))))
    ha_long = list(substring(hl, a, b).coords)[::-1]
    ln = LineString(front + ha_long + back).simplify(1.5)
    return [to_blender(c) for c in ln.coords]


def line_pts(path, tol=1.0, upto=None):
    """a street's centreline in Blender XY, simplified"""
    ln = LineString(sample(path, 0.5)).simplify(tol)
    pts = [to_blender(c) for c in ln.coords]
    return pts if upto is None else pts[:upto]


# ---- the places (server/content/places.json): landmarks at their footprint, streets along their centreline
lm_at = lambda i: to_blender(LM[i]['at'])  # noqa: E731
PLACES = {
    'cho_ben_thanh': dict(pts=[lm_at('cho_ben_thanh')]),
    'nha_hat_thanh_pho': dict(pts=[lm_at('nha_hat')]),
    'quang_truong_lam_son': dict(pts=[to_blender((240, 99))]),                     # between the Opera House and Lam Sơn
    'nha_tho_duc_ba': dict(pts=[lm_at('nha_tho_duc_ba')]),
    'buu_dien_thanh_pho': dict(pts=[lm_at('buu_dien')]),
    'cong_xa_paris': dict(pts=[to_blender((140, 104))]),
    'ho_con_rua': dict(pts=[to_blender((140, 66))]),
    'bitexco': dict(pts=[lm_at('bitexco')]),
    'day_nha_pho_le_lai': dict(pts=[to_blender((210, 249))]),                      # the shophouses on Lê Lai's south side
    'pho_di_bo_nguyen_hue': dict(pts=line_pts(WALK['Phố đi bộ Nguyễn Huệ']['path']),
                                 note={'vi': 'từ UBND ra sông', 'en': 'from City Hall to the river'}),
    'duong_le_loi': dict(pts=line_pts(ROAD['Lê Lợi']['path'])),
    'duong_ham_nghi': dict(pts=line_pts(ROAD['Hàm Nghi']['path']),
                           note={'vi': 'chợ ra sông', 'en': 'market to the river'}),
    'duong_le_lai': dict(pts=line_pts(ROAD['Lê Lai']['path'], 1.5)),
    'duong_le_thanh_ton': dict(pts=line_pts(ROAD['Lê Thánh Tôn']['path']),
                               note={'vi': 'ngang trước UBND', 'en': 'past the front of City Hall'}),
    'duong_dong_khoi': dict(pts=line_pts(ROAD['Đồng Khởi']['path']),
                            note={'vi': 'Nhà thờ ra sông', 'en': 'Cathedral to the river'}),
    'duong_pham_ngoc_thach': dict(pts=[to_blender((140, 104)), to_blender((140, 66))]),
    'cong_vien_23_9': dict(pts=[to_blender((112, 246.5)), to_blender((172, 246.5))], r=8),
    # the ring road (9 + 5 m) and its pavement: being at the roundabout is being on it or on its pavement
    'bung_binh_quach_thi_trang': dict(pts=[to_blender((240, 206))], r=18),
    'bai_bien': dict(pts=sea_front(), note={'vi': 'Vũng Tàu: Bãi Trước, Bãi Sau', 'en': 'Vũng Tàu, front and back beaches'}),
}

# ---- NPCs (npcs.json) and archetype spawns (archetypes.json): plan points next to their place
NPCS = {
    # Cô Ba sells by the market's main gate (east, on the square), facing the square
    'co_ba': (to_blender((222, 200)), to_blender((230, 200))),
    # Chú Tư at his café among the Lê Lai shophouses, facing the street
    'chu_tu': (to_blender((205, 241)), to_blender((205, 236))),
    # Dì Năm on the Hồ Con Rùa square, where she stood on the v4 island (same offset from the centre: the model is
    # only moved), facing out
    'di_nam': (to_blender((143.5, 75.5)), to_blender((148, 82))),
    # Chị Hạnh's group on Lam Sơn, in front of the Opera House (within the square's 8 m), she faces them
    'chi_hanh': (to_blender((233, 97)), to_blender((227, 104))),
    # Chú Sáu waits with his bike on the market square by the roundabout, watching it
    'chu_sau': (to_blender((226, 214)), to_blender((240, 206))),
}
SPAWNS = {
    'traveler': (to_blender((224, 206)), to_blender((200, 206))),     # the square, looking at the market's gate
    'citizen': (to_blender((212, 241)), to_blender((212, 252))),      # outside home on Lê Lai, facing the house
    'driver': (to_blender((262, 213)), to_blender((286, 213))),       # the bike stand on Hàm Nghi, facing the river
}


def edit_places():
    p = os.path.join(CONTENT, 'places.json')
    s = open(p, encoding='utf-8').read()
    d = json.loads(s)
    d['about'] = d['about'].replace('on the v4 island', 'on the island (Sài Gòn mini, layout v8)')
    for pl in d['places']:
        new = PLACES.get(pl['id'])
        if new is None:
            raise SystemExit('no new place for ' + pl['id'])
        pl['pts'] = new['pts']
        if 'note' in new:
            pl['note'] = new['note']
        if 'r' in new:
            pl['r'] = new['r']
    # one place per line, as the file was
    lines = ['{', '  "about": %s,' % json.dumps(d['about'], ensure_ascii=False), '  "places": [']
    for k, pl in enumerate(d['places']):
        rec = json.dumps(pl, ensure_ascii=False, separators=(', ', ': '))
        lines.append('    ' + rec + (',' if k < len(d['places']) - 1 else ''))
    lines += ['  ]', '}', '']
    open(p, 'w', encoding='utf-8').write('\n'.join(lines))


def set_pair(text, start, key, value):
    """replace the first `"key": [x, y]` after offset `start`"""
    m = re.compile(r'"%s": \[[^\]]*\]' % key).search(text, start)
    return text[:m.start()] + '"%s": %s' % (key, pt(value)) + text[m.end():]


def edit_npcs():
    p = os.path.join(CONTENT, 'npcs.json')
    s = open(p, encoding='utf-8').read()
    for nid, (at, face) in NPCS.items():
        i = s.index('"id": "%s"' % nid)
        s = set_pair(s, i, 'at', at)
        s = set_pair(s, i, 'face', face)
    open(p, 'w', encoding='utf-8').write(s)
    json.loads(s)


def edit_archetypes():
    p = os.path.join(CONTENT, 'archetypes.json')
    s = open(p, encoding='utf-8').read()
    for aid, (at, face) in SPAWNS.items():
        i = s.index('"id": "%s"' % aid)
        j = s.index('"spawn"', i)
        s = set_pair(s, j, 'at', at)
        s = set_pair(s, j, 'face', face)
    open(p, 'w', encoding='utf-8').write(s)
    json.loads(s)


def write_map_land():
    """the phone map: land (the coast cut by the river and the canal) and every road, footbridge and bridge, in
    Blender XY, simplified to a metre"""
    water = [LineString(sample(L['river']['path'], 1.0)).buffer(L['river']['width'] / 2)]
    water += [LineString(sample(c['path'], 1.0)).buffer(c['width'] / 2) for c in L['canals']]
    land = COAST.difference(unary_union(water))
    polys = list(land.geoms) if isinstance(land, MultiPolygon) else [land]
    polys = [pg.simplify(1.0) for pg in polys if pg.area > 200]

    def bl(coords):
        return [to_blender(c) for c in coords]
    roads = [bl(LineString(sample(r['path'], 1.0)).simplify(1.0).coords) for r in L['roads']]
    roads += [bl(LineString(sample(b['path'], 1.0)).coords) for b in L['bridges']]
    walks = [bl(LineString(sample(w['path'], 1.0)).simplify(1.0).coords) for w in L['walks']]
    out = {
        'about': 'The phone map (src/game/phone/miniMap.ts): land, roads and walks of Sài Gòn mini, Blender XY metres. '
                 'Written by scripts/map/move_game_content_v8.py from scripts/map/saigon_mini_layout.json.',
        'land': [bl(pg.exterior.coords) for pg in polys],
        'roads': roads,
        'walks': walks,
    }
    with open(MAP_LAND, 'w', encoding='utf-8') as f:
        f.write(json.dumps(out, ensure_ascii=False, separators=(',', ':')) + '\n')
    return len(polys), len(roads), len(walks)


if __name__ == '__main__':
    edit_places()
    edit_npcs()
    edit_archetypes()
    print('moved', len(PLACES), 'places,', len(NPCS), 'NPCs,', len(SPAWNS), 'spawns; phone map land, roads, walks:', write_map_land())
