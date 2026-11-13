"""Sài Gòn mini (map v5) blockout: builds a new .blend from scripts/map/saigon_mini_layout.json.

    /Applications/Blender.app/Contents/MacOS/Blender --background --factory-startup \\
        --python scripts/blender/build_saigon_mini.py -- models/saigon_mini.blend [--render renders/blockout]

A blockout for review (docs/map-v5.md, "Các bước sau khi duyệt"): terrain, beaches, river, mountains, streets,
pavements, house blocks made of simple boxes in the house-kit colours, the kept landmarks appended from
models/sg.tinymap.blend and moved whole, and plain volumes for the new landmarks. Materials are SD_<PAL key>, the
names the game's paint shader keys on. Nothing here is final art: the house kit and the new landmarks get their own
design rounds.

Needs shapely and mapbox_earcut for Blender's Python, installed next to the repo rather than into Blender:

    /Applications/Blender.app/Contents/Resources/5.1/python/bin/python3.13 -m pip install --target .blender-site shapely mapbox_earcut

(or point SG_PYLIBS at another folder holding them).
"""
import ast
import json
import math
import os
import random
import re
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.abspath(os.path.join(HERE, '..', '..'))
for _p in (os.environ.get('SG_PYLIBS'), os.path.join(ROOT, '.blender-site')):
    if _p and os.path.isdir(_p) and _p not in sys.path:
        sys.path.insert(0, _p)

import bpy  # noqa: E402
import numpy as np  # noqa: E402
import shapely  # noqa: E402
import shapely.affinity  # noqa: E402
from mathutils import Matrix, Vector  # noqa: E402
from shapely.geometry import LineString, MultiPolygon, Point, Polygon, box as sbox  # noqa: E402
from shapely.geometry.polygon import orient  # noqa: E402
from shapely.ops import nearest_points, unary_union  # noqa: E402
import mapbox_earcut as earcut  # noqa: E402

ARGV = sys.argv[sys.argv.index('--') + 1:] if '--' in sys.argv else []
OUT = os.path.abspath(ARGV[0]) if ARGV else os.path.join(ROOT, 'models', 'saigon_mini.blend')
RENDER = os.path.abspath(ARGV[ARGV.index('--render') + 1]) if '--render' in ARGV else None
WORKBENCH = '--workbench' in ARGV      # quick flat renders instead of EEVEE with ink
# SAIGON_LAYOUT=<file.json> builds another layout (a proposal) instead of the current one; the part scripts read it too
LAYOUT_FILE = os.path.abspath(os.environ.get('SAIGON_LAYOUT') or os.path.join(ROOT, 'scripts', 'map', 'saigon_mini_layout.json'))
LAYOUT = json.load(open(LAYOUT_FILE, encoding='utf-8'))
SRC_BLEND = os.path.join(ROOT, 'models', 'sg.tinymap.blend')
TX, TY = LAYOUT['toBlender']['x'], LAYOUT['toBlender']['y']
SEA = -0.45
GROUND_Z, ROAD_Z, WALK_Z = 0.0, 0.07, 0.18
FLOOR_BOTTOM = -2.6       # land slabs reach down to the river bed
SIDEWALK = 3.0

# Parts redrawn as real models (docs/map-v5-tasks.md): scripts/blender/mini/<name>.py, each with a run() that removes
# its own objects and builds them again, so it also runs alone on an open file. They run after the blockout, in order.
MINI_SCRIPTS = ['cau_ba_son', 'nui_nho', 'hai_dang', 'tuong_chua', 'dinh_thong_nhat', 'bui_vien', 'no_figures']
MODELLED_BRIDGES = {'cau_ba_son'}                  # built by a MINI_SCRIPTS entry instead of build_bridges
MODELLED_MOUNTAINS = {'nui_nho'}                   # terrain, forest, boulders and stairs by a MINI_SCRIPTS entry
MODELLED_LANDMARKS = {'tuong_chua', 'hai_dang', 'dinh_doc_lap', 'bui_vien'}   # no blockout volume: a MINI_SCRIPTS entry builds it

# docs/PROJECT.md palette, the 15b_frontage additions (wallCream, wallMint, wallSky) and the traffic-light keys (map-v5)
PAL = {
    'cream': '#EAD9AE', 'creamLight': '#F4EBD3', 'wallGrey': '#A0A7AA', 'wallTeal': '#5E9A96', 'wallOchre': '#C9A25E',
    'wallBeige': '#CFC1A0', 'wallPink': '#D29A86', 'opWhite': '#EEE7D7', 'opShade': '#D8CDB8', 'poYellow': '#E0B45A',
    'brick': '#B5523B', 'stone': '#E6DCC8', 'towerPale': '#DAD4C8', 'concrete': '#ABA69C',
    'wallCream': '#EFE3C8', 'wallMint': '#A9CDBB', 'wallSky': '#9DBFD2',
    'roofRed': '#A9492F', 'roofRedDark': '#7D3326', 'slate': '#646B74', 'spire': '#6F7780', 'tinBlue': '#8AA3B6',
    'poGreen': '#3E7F5F', 'clockBlue': '#4E7CA6', 'gold': '#C9A24A', 'turq': '#5BA9A1', 'archDark': '#3B3330',
    'glassBlue': '#7FA0B5', 'glassTeal': '#4F7A7C', 'finSilver': '#D3DAE0', 'bridgeWhite': '#EEF0EE', 'statueW': '#F2EFE8',
    'asphalt': '#6F6D69', 'paving': '#BDB4A2', 'curb': '#D3CBB8', 'roadLine': '#ECE6D4', 'grass': '#86A85E',
    'grassDark': '#62874A', 'sand': '#CDB98F', 'water': '#6FA9B6', 'rock': '#8E877A', 'rockDark': '#6C665B',
    'earthDark': '#5F4735', 'leaf': '#6E9B4C', 'leafLight': '#93B85F', 'leafDark': '#4D7539', 'bark': '#6B4C36',
    'lampGlow': '#FFE6A8', 'signRed': '#B83A2B', 'signGold': '#D9B45A', 'clothBlue': '#4D6E9A', 'shopDark': '#40342D',
    'sigRed': '#C8553D', 'sigAmber': '#E3A33B', 'sigGreen': '#5FA77A', 'sea': '#3F8399', 'foam': '#F1F3EE',
    # other keys the kept landmark models use (saigon_diorama_01_setup.py / edit_v4_buivien.py)
    'lotusTop': '#E6D29C', 'clockWhite': '#F3EEE2', 'creamShade': '#D9C394', 'roofRedLight': '#C0613F', 'brownText': '#5E3F26',
    'pavingDark': '#A69D8A', 'earth': '#8E6D4E', 'tinWhite': '#DDE4E7', 'lantern': '#C8372D', 'wood': '#8A5E3C', 'metal': '#6B6F73',
    'pipeYellow': '#C9A03D', 'clothRed': '#B84A3A', 'clothYellow': '#E0B648', 'skin': '#E7B897', 'hair': '#2E2622',
    'plasticBlue': '#4E86B8', 'plasticRed': '#C9443A', 'jarPlum': '#6E4F5E', 'acWhite': '#E3E1D8', 'interior': '#8D6A50',
    'flowerRed': '#D2483F', 'flowerPink': '#E08AA6', 'flowerYellow': '#EBC64C', 'white': '#F4F1EA', 'ink': '#2B211C', 'glassDark': '#4F6E80',
}


def log(*a):
    print('[saigon_mini]', *a, flush=True)


def lin(h):
    h = h.lstrip('#')
    c = [int(h[i:i + 2], 16) / 255.0 for i in (0, 2, 4)]
    return tuple(x / 12.92 if x <= 0.04045 else ((x + 0.055) / 1.055) ** 2.4 for x in c)


_NS = {}


def _safe_nodes(tree):
    """definitions only, as saigon_diorama_00_ns_loader.py does: no build step runs"""
    out = []
    for n in tree.body:
        if isinstance(n, (ast.Import, ast.ImportFrom, ast.FunctionDef, ast.ClassDef, ast.Assign, ast.AugAssign, ast.AnnAssign)):
            out.append(n)
        elif isinstance(n, ast.If) and all(isinstance(b, (ast.Assign, ast.Import, ast.ImportFrom)) for b in n.body):
            out.append(n)
        elif isinstance(n, ast.Expr) and isinstance(n.value, ast.Call) and isinstance(n.value.func, ast.Attribute) and n.value.func.attr == 'update':
            out.append(n)
    return out


def load_builders():
    """The production map's builders, from the Text blocks of models/sg.tinymap.blend: the Geo mesh builder, PAL and
    get_mat (the SD_Paint shader), the scene / ink setup and the art pass (01, 17), and the models: house_plain,
    shop_front, modern_block, tt_tower, house_bv, tree_sao, tree_me, cycad, globe_lamp, topiary, scooter, lantern..."""
    if _NS:
        return _NS
    with bpy.data.libraries.load(SRC_BLEND, link=False) as (src, dst):
        order = ['saigon_mini_lib.py'] + sorted(n for n in src.texts if n.startswith('saigon_diorama_') and n.endswith('.py')
                                                and n[15:17].isdigit() and int(n[15:17]) < 20)
        order.append('saigon_diorama_56_v5_new_models.py')
        dst.texts = [n for n in order if n in src.texts]
    loaded = [t for t in dst.texts if t is not None]
    texts = {t.name: t.as_string() for t in loaded}
    for t in loaded:
        bpy.data.texts.remove(t)
    _NS.update({'__name__': 'sdp', 'bpy': bpy})
    for name in order:
        if name not in texts:
            continue
        try:
            exec(compile(ast.Module(body=_safe_nodes(ast.parse(texts[name])), type_ignores=[]), name, 'exec'), _NS)
        except Exception as e:
            log('builders: rest of', name, 'skipped:', type(e).__name__, e)
    _NS['PAL'].update({k: v for k, v in PAL.items() if k not in _NS['PAL']})
    return _NS


def mat(key):
    return load_builders()['get_mat'](key)


def viewport_colours():
    """Workbench draws a material's viewport colour: give every SD_ material (ours and the appended ones) its PAL colour."""
    for m in bpy.data.materials:
        key = re.sub(r'^SD_|\.\d+$', '', m.name)
        if key in PAL:
            m.diffuse_color = (*lin(PAL[key]), 1)
            continue
        bsdf = next((n for n in m.node_tree.nodes if n.type == 'BSDF_PRINCIPLED'), None) if m.node_tree else None
        if bsdf and not bsdf.inputs['Base Color'].is_linked:
            m.diffuse_color = tuple(bsdf.inputs['Base Color'].default_value)


# ---------------------------------------------------------------------------------------------------------------
# plan paths -> Blender XY
# ---------------------------------------------------------------------------------------------------------------
def tb(p):
    return (p[0] - TX, TY - p[1])


def _bez(ctrl, step):
    L = sum(math.dist(ctrl[i], ctrl[i + 1]) for i in range(len(ctrl) - 1))
    n = max(2, int(math.ceil(L / step)))
    out = []
    for k in range(n + 1):
        t = k / n
        pts = list(ctrl)
        while len(pts) > 1:
            pts = [((1 - t) * a[0] + t * b[0], (1 - t) * a[1] + t * b[1]) for a, b in zip(pts, pts[1:])]
        out.append(pts[0])
    return out


def path_pts(d, step=1.0):
    """Sample an SVG path (absolute M L Q C S Z, one subpath) into Blender XY points."""
    toks = re.findall(r'[MLQCSZ]|-?\d+(?:\.\d+)?', d)
    pts, i, cmd, cur, c2prev = [], 0, None, None, None

    def nums(n):
        nonlocal i
        v = [float(t) for t in toks[i:i + n]]
        i += n
        return [(v[k], v[k + 1]) for k in range(0, n, 2)]

    while i < len(toks):
        if toks[i] in 'MLQCSZ':
            cmd = toks[i]
            i += 1
            if cmd == 'Z':
                if pts and pts[0] != pts[-1]:
                    pts.append(pts[0])
                continue
        if cmd == 'M':
            (p,) = nums(2)
            pts.append(p)
            cur, c2prev, cmd = p, None, 'L'
        elif cmd == 'L':
            (p,) = nums(2)
            pts += _bez([cur, p], step)[1:]
            cur, c2prev = p, None
        elif cmd == 'Q':
            c, p = nums(4)
            pts += _bez([cur, c, p], step)[1:]
            cur, c2prev = p, None
        elif cmd == 'C':
            c1, c2, p = nums(6)
            pts += _bez([cur, c1, c2, p], step)[1:]
            cur, c2prev = p, c2
        elif cmd == 'S':
            c2, p = nums(4)
            c1 = (2 * cur[0] - c2prev[0], 2 * cur[1] - c2prev[1]) if c2prev else cur
            pts += _bez([cur, c1, c2, p], step)[1:]
            cur, c2prev = p, c2
    return [tb(p) for p in pts]


def rect_poly(r):
    x0, y0 = tb((r[0], r[1]))
    x1, y1 = tb((r[2], r[3]))
    return sbox(min(x0, x1), min(y0, y1), max(x0, x1), max(y0, y1))


def polys(g):
    if g is None or g.is_empty:
        return []
    if isinstance(g, Polygon):
        return [g]
    if isinstance(g, MultiPolygon):
        return list(g.geoms)
    return [p for p in getattr(g, 'geoms', []) if isinstance(p, Polygon)]


# ---------------------------------------------------------------------------------------------------------------
# mesh builder: geometry gathered per material, one object per builder
# ---------------------------------------------------------------------------------------------------------------
class Builder:
    def __init__(self, name, coll):
        self.name, self.coll = name, coll
        self.V, self.F, self.M = [], [], []
        self.mats = {}

    def _m(self, key):
        if key not in self.mats:
            self.mats[key] = len(self.mats)
        return self.mats[key]

    def face(self, pts, key):
        b = len(self.V)
        self.V += [tuple(p) for p in pts]
        self.F.append(tuple(range(b, b + len(pts))))
        self.M.append(self._m(key))

    def mesh(self, verts, tris, key):
        b = len(self.V)
        self.V += [tuple(v) for v in verts]
        mi = self._m(key)
        for t in tris:
            self.F.append(tuple(b + int(k) for k in t))
            self.M.append(mi)

    def slab(self, poly, z0, z1, top, side=None, zfun=None):
        """an extruded polygon (holes allowed): top at z1 (or zfun(x, y)), walls down to z0"""
        side = side or top
        for p in polys(poly):
            p = orient(p, 1.0)
            if p.area < 0.05:
                continue
            rings = [np.asarray(p.exterior.coords)[:-1]] + [np.asarray(r.coords)[:-1] for r in p.interiors]
            verts = np.concatenate(rings)
            ends = np.cumsum([len(r) for r in rings]).astype(np.uint32)
            tris = earcut.triangulate_float64(verts, ends).reshape(-1, 3)
            zs = [zfun(x, y) for x, y in verts] if zfun else [z1] * len(verts)
            top_v = [(x, y, z) for (x, y), z in zip(verts, zs)]
            fixed = []
            for a, b, c in tris:
                (ax, ay), (bx, by), (cx, cy) = verts[a], verts[b], verts[c]
                fixed.append((a, b, c) if (bx - ax) * (cy - ay) - (by - ay) * (cx - ax) >= 0 else (a, c, b))
            self.mesh(top_v, fixed, top)
            if z0 is None:
                continue
            k = 0
            for r in rings:
                n = len(r)
                for i in range(n):
                    a, b = r[i], r[(i + 1) % n]
                    za, zb = zs[k + i], zs[k + (i + 1) % n]
                    self.face([(a[0], a[1], z0), (b[0], b[1], z0), (b[0], b[1], zb), (a[0], a[1], za)], side)
                k += n

    def box(self, x0, y0, z0, x1, y1, z1, key, top=None):
        self.oriented_box((x0 + x1) / 2, (y0 + y1) / 2, z0, x1 - x0, y1 - y0, z1 - z0, 0.0, key, top)

    def oriented_box(self, cx, cy, z0, w, d, h, yaw, key, top=None):
        c, s = math.cos(yaw), math.sin(yaw)
        P = [(cx + c * x - s * y, cy + s * x + c * y) for x, y in ((-w / 2, -d / 2), (w / 2, -d / 2), (w / 2, d / 2), (-w / 2, d / 2))]
        lo = [(x, y, z0) for x, y in P]
        hi = [(x, y, z0 + h) for x, y in P]
        self.face(hi, top or key)
        self.face(lo[::-1], key)
        for i in range(4):
            j = (i + 1) % 4
            self.face([lo[i], lo[j], hi[j], hi[i]], key)

    def cylinder(self, cx, cy, z0, r, h, key, n=8, r_top=None, top=None):
        rt = r if r_top is None else r_top
        lo = [(cx + r * math.cos(2 * math.pi * i / n), cy + r * math.sin(2 * math.pi * i / n), z0) for i in range(n)]
        hi = [(cx + rt * math.cos(2 * math.pi * i / n), cy + rt * math.sin(2 * math.pi * i / n), z0 + h) for i in range(n)]
        if rt > 1e-4:
            self.face(hi, top or key)
        else:
            hi = [(cx, cy, z0 + h)] * n
        self.face(lo[::-1], key)
        for i in range(n):
            j = (i + 1) % n
            if rt > 1e-4:
                self.face([lo[i], lo[j], hi[j], hi[i]], key)
            else:
                self.face([lo[i], lo[j], hi[0]], key)

    def build(self):
        if not self.F:
            return None
        me = bpy.data.meshes.new(self.name)
        me.from_pydata(self.V, [], self.F)
        for key, _ in sorted(self.mats.items(), key=lambda kv: kv[1]):
            me.materials.append(mat(key))
        me.polygons.foreach_set('material_index', self.M)
        me.validate(clean_customdata=False)
        me.update()
        ob = bpy.data.objects.new(self.name, me)
        self.coll.objects.link(ob)
        return ob


SCENE = WORLD = ENV = None


def collection(name, parent=None):
    c = bpy.data.collections.new(name)
    (parent or SCENE.collection).children.link(c)
    return c


# ---------------------------------------------------------------------------------------------------------------
# the plan
# ---------------------------------------------------------------------------------------------------------------
L = LAYOUT
coast = Polygon(path_pts(L['coast'], 1.0)).buffer(0)
river_line = LineString(path_pts(L['river']['path'], 1.0))
river = river_line.buffer(L['river']['width'] / 2, cap_style='flat')
# canals (layout v8: Kênh Bến Nghé) are water like the river: cut from the land, with a bed and banks
for _c in L.get('canals', []):
    river = river.union(LineString(path_pts(_c['path'], 1.0)).buffer(_c['width'] / 2, cap_style='flat'))
land = coast.difference(river)
# where house blocks may stand: inside the coastal road and its pavement (layout 'inland', 2026-11-12 coast)
inland = Polygon(path_pts(L['inland'], 1.0)).buffer(0) if L.get('inland') else coast

road_geo = []
for r in L['roads']:
    line = LineString(path_pts(r['path'], 0.8))
    road_geo.append((line, r['w']))
# round caps (layout v8 review): two roads meeting end to end at an angle (a ring road split into two names, Quốc lộ 51
# into Hạ Long) close without a notch; the layout puts every road end on another road (scripts/map/check_layout.py)


def _road_band(k, ln, w):
    """road k's surface w wide: a round end where it ends on its own or meets another road end to end, a flat one
    where it runs into the side of another road (review 2026-11-13: a round end there bulged past the far kerb)"""
    body = ln.buffer(w / 2, cap_style='flat', join_style='round')
    ends = []
    for end in (ln.coords[0], ln.coords[-1]):
        pt = Point(end)
        t_junction = any(o.distance(pt) < 0.6 and min(Point(o.coords[0]).distance(pt), Point(o.coords[-1]).distance(pt)) > 1.0
                         for j, (o, _) in enumerate(road_geo) if j != k)
        if not t_junction:
            ends.append(pt.buffer(w / 2, 24))
    return unary_union([body] + ends)


roads = unary_union([_road_band(k, ln, w) for k, (ln, w) in enumerate(road_geo)])
walk_outer = [_road_band(k, ln, w + 2 * SIDEWALK) for k, (ln, w) in enumerate(road_geo)]
for lp in L['loops']:
    x0, y0, x1, y1 = lp['rect']
    rr = sbox(*tb((x0, y1)), *tb((x1, y0))).buffer(0)
    core = rr.buffer(-lp['r']).buffer(lp['r'])
    ring = core.buffer(lp['w'] / 2).difference(core.buffer(-lp['w'] / 2))
    roads = roads.union(ring)
    walk_outer.append(core.buffer(lp['w'] / 2 + SIDEWALK).difference(core.buffer(-lp['w'] / 2 - SIDEWALK)))
rings_geo = []
for ra in L['roundabouts']:
    c = Point(tb(ra['c']))
    island = c.buffer(ra['r'], 32)
    ring = c.buffer(ra['r'] + 5, 32).difference(island)
    roads = roads.union(ring)
    walk_outer.append(c.buffer(ra['r'] + 5 + SIDEWALK, 32))
    rings_geo.append((ra, island))
roads = roads.difference(unary_union([isl for _, isl in rings_geo]))


def _tangent(ln, pt):
    t = ln.project(pt)
    q, o = ln.interpolate(min(ln.length, t + 0.6)), ln.interpolate(max(0.0, t - 0.6))
    d = math.hypot(q.x - o.x, q.y - o.y) or 1.0
    return (q.x - o.x) / d, (q.y - o.y) / d


def junction_areas():
    """where two roads cross or one runs into another (not where one road goes on under another name), with 2 m
    round it, and the roundabouts: no lane marks there, crossings outside it (review r5: dashes ran across the
    junctions, zebra stripes of two arms piled up on each other)"""
    bands = [ln.buffer(w / 2, cap_style='flat') for ln, w in road_geo]
    tree = shapely.STRtree(bands)
    out = []
    for i, (ln, w) in enumerate(road_geo):
        for j in tree.query(bands[i], predicate='intersects'):
            if j <= i:
                continue
            x = bands[i].intersection(bands[int(j)])
            if x.is_empty or x.area < 1.0:
                continue
            c = x.centroid
            (ax, ay), (bx, by) = _tangent(ln, c), _tangent(road_geo[int(j)][0], c)
            if abs(ax * bx + ay * by) > math.cos(math.radians(25)):
                continue
            out.append(x.buffer(2.0))
    out += [c_.buffer(ra['r'] + 7, 32) for ra, c_ in ((ra, Point(tb(ra['c']))) for ra in L['roundabouts'])]
    return unary_union(out or [Polygon()])


JUNCTIONS = junction_areas()


def zebra_spots():
    """a zebra crossing on each arm of the signalled crossings, just outside the junction, never on another one:
    (centre, unit along the road, half its width, outline)"""
    out = []
    for (sx, sy) in (tb(p_) for p_ in L['signals']):
        for ln, w in road_geo:
            if w < 8 or ln.distance(Point(sx, sy)) > 1.5:
                continue
            t0 = ln.project(Point(sx, sy))
            # a signal away from any junction is a mid-block crossing: one zebra, at the lights
            mid = JUNCTIONS.distance(Point(sx, sy)) > 6.0
            for sgn in ((1,) if mid else (-1, 1)):
                for off in (np.arange(0.0, 6.01, 0.5) if mid else np.arange(4.0, 18.01, 0.5)):
                    t = t0 + sgn * off
                    if t < 1.5 or t > ln.length - 1.5:
                        break
                    p = ln.interpolate(t)
                    dx, dy = _tangent(ln, p)
                    hw = w / 2 - 0.7
                    rect = Polygon([(p.x + dx * a * 1.6 - dy * c * hw, p.y + dy * a * 1.6 + dx * c * hw)
                                    for a, c in ((-1, -1), (1, -1), (1, 1), (-1, 1))])
                    if rect.intersects(JUNCTIONS) or any(rect.distance(z[4]) < 1.0 for z in out):
                        continue
                    out.append((p, dx, dy, hw, rect))
                    break
    return out


ZEBRAS = zebra_spots()
walk_lines = [(LineString(path_pts(w['path'], 0.8)), w) for w in L['walks']]
walks = unary_union([ln.buffer(w['w'] / 2, cap_style='flat') for ln, w in walk_lines])
plazas = unary_union([rect_poly(r) for r in L.get('plazas', [])] or [Polygon()])

lm_foot = {}
for lm in L['landmarks']:
    if 'size' in lm and 'at' in lm:
        cx, cy = tb(lm['at'])
        w, d = lm['size']
        lm_foot[lm['id']] = sbox(cx - w / 2, cy - d / 2, cx + w / 2, cy + d / 2)
ring_keepout = unary_union([Point(tb(ra['c'])).buffer(ra['r'] + 5 + SIDEWALK, 24) for ra in L['roundabouts']])
lm_keepout = unary_union([f.buffer(2.0) for f in lm_foot.values()] + [ring_keepout])

mountains = []
for mt in L['mountains']:
    outline = Polygon(path_pts(mt['outline'], 1.0)).buffer(0)
    mountains.append((mt, outline))
beach_polys = []
for b in L['beaches']:
    band = LineString(path_pts(b['path'], 0.8)).buffer(b['width'] / 2, cap_style='flat')
    beach_polys.append((b, band))
beach_all = unary_union([p for _, p in beach_polys])
bridge_lines = [(LineString(path_pts(b['path'], 0.5)), b) for b in L['bridges']]


# ---------------------------------------------------------------------------------------------------------------
# terrain
# ---------------------------------------------------------------------------------------------------------------
def smoothstep(a, b, x):
    t = np.clip((x - a) / (b - a), 0.0, 1.0)
    return t * t * (3 - 2 * t)


def lot_list():
    """(plan rect, house kind) for every house block: the grid cells (inset) and the extra lots"""
    out = []
    ins = L['grid']['inset']
    for cell in L['grid']['cells']:
        xs, ys = cell['x'], cell['y']
        for i in range(len(xs) - 1):
            for j in range(len(ys) - 1):
                out.append(([xs[i] + ins, ys[j] + ins, xs[i + 1] - ins, ys[j + 1] - ins], cell['kind']))
    return out + [(lt['rect'], lt['kind']) for lt in L['lots']]


LOTS = lot_list()


# ---------------------------------------------------------------------------------------------------------------
# city blocks (review 2026-11-13): every piece of land the streets close in, that is not a park, a square, a
# landmark's forecourt, a beach, the hill or a river bank, is a block of houses of its district. Rows of tube houses
# stand on the sidewalk line and face only the streets, walks and squares in front of them; a corner house turns
# each street corner with a shop front on both streets; long rows are cut by hẻm (alleys) through the block, lined
# with smaller houses; side and back walls that show get windows, AC units and now and then a painted sign. What is
# left inside a block is a yard with a tree, never a blank fill slab. The layout's lots only say which kind of house
# a block gets where they lie.
# ---------------------------------------------------------------------------------------------------------------
PROM = unary_union([LineString(path_pts(p['path'], 0.8)).buffer(p['w'] / 2, cap_style='flat') for p in L['promenades']] or [Polygon()])
SIDEWALKS = unary_union(walk_outer)
FRONT_SURFACE = SIDEWALKS.difference(roads).union(walks).union(plazas).union(PROM)
STREET_CUT = SIDEWALKS.union(walks).union(plazas).union(PROM).union(lm_keepout).union(river)
OPEN_LAND = unary_union([rect_poly(pk['rect']) for pk in L['parks']]
                        + [shapely.affinity.scale(Point(tb(lk['ellipse'][:2])).buffer(1.0, 24), lk['ellipse'][2] + 3, lk['ellipse'][3] + 3)
                           for lk in L.get('lakes', [])]
                        + [Point(tb((x, y))).buffer(r + 2, 24) for x, y, r in L.get('flats', [])])
TOWERS = unary_union([rect_poly(t[:4]).buffer(5.0) for t in L.get('towers', [])] or [Polygon()])
NO_HOUSES = unary_union([STREET_CUT, OPEN_LAND, TOWERS, beach_all.buffer(3.0), river.buffer(7.0),
                         unary_union([f.buffer(7.0) for f in lm_foot.values()] or [Polygon()]),
                         unary_union([o.buffer(4.0) for _, o in mountains] or [Polygon()]),
                         unary_union([ln.buffer(b_['w'] / 2 + 6) for ln, b_ in bridge_lines] or [Polygon()])])
_FRONT_P = FRONT_SURFACE.buffer(0.25)
shapely.prepare(_FRONT_P)
# per kind: house width range, the depth of a row, storeys, and the spacing of hẻm along a long row
HOUSE = {'np': (4.0, 5.4, 14.0), 'hem': (3.6, 4.4, 10.0), 'bar': (4.0, 5.0, 10.0), 'cl': (4.4, 4.8, 12.0),
         'ks': (6.4, 8.4, 12.0), 'vt': (6.0, 8.5, 11.0), 'tt': (12.0, 18.0, 15.0)}
HEM_EVERY = {'np': (22.0, 30.0), 'hem': (14.0, 20.0), 'cl': (24.0, 32.0)}
HEM_W = 2.8
CORNER_KINDS = {'np', 'hem'}
GAPS = {'ks': (0.2, 2.0, 2.0), 'vt': (0.45, 2.0, 4.0), 'tt': (1.0, 8.0, 14.0)}   # chance of a gap after a house, its width
DISTRICT = {'TRUNG': 'np', 'NHÀ PHỐ': 'hem', 'CHỢ LỚN': 'cl', 'THỦ THIÊM': 'tt', 'KHÁNH HỘI': 'np', 'VŨNG TÀU': 'vt'}
AREA_PTS = [(tb((a[1], a[2])), next((k for n_, k in DISTRICT.items() if a[0].startswith(n_)), 'np')) for a in L.get('areas', [])]


def district_kind(x, y):
    return min(AREA_PTS, key=lambda a: math.hypot(a[0][0] - x, a[0][1] - y))[1] if AREA_PTS else 'np'


def _front(x, y):
    return shapely.contains_xy(_FRONT_P, x, y)


def _ray_depth(p, x, y, nx, ny):
    """distance from (x, y) on p's outline along the inward normal to the far side of p, and the point there"""
    ray = LineString([(x + nx * 0.05, y + ny * 0.05), (x + nx * 160, y + ny * 160)])
    hit = ray.intersection(p.boundary)
    pts = [hit] if hit.geom_type == 'Point' else [q for q in getattr(hit, 'geoms', []) if q.geom_type == 'Point']
    ds = sorted(math.hypot(q.x - x, q.y - y) for q in pts if math.hypot(q.x - x, q.y - y) > 0.3)
    if not ds:
        return 0.0, None
    return ds[0], (x + nx * ds[0], y + ny * ds[0])


def _quad(ax, ay, ux, uy, nx, ny, t0, t1, d0, d1):
    return Polygon([(ax + ux * t0 + nx * d0, ay + uy * t0 + ny * d0), (ax + ux * t1 + nx * d0, ay + uy * t1 + ny * d0),
                    (ax + ux * t1 + nx * d1, ay + uy * t1 + ny * d1), (ax + ux * t0 + nx * d1, ay + uy * t0 + ny * d1)])


def _row(rng, kind, s0, s1):
    """widths (and gaps, as negative numbers) filling a stretch of a row exactly"""
    wmin, wmax, _ = HOUSE[kind]
    span = s1 - s0
    if span < wmin * 0.8:
        return []
    gap = GAPS.get(kind)
    out = []
    while True:
        out.append(rng.uniform(wmin, wmax))
        if gap and rng.random() < gap[0]:
            out.append(-rng.uniform(gap[1], gap[2]))
        if sum(abs(v) for v in out) >= span:
            break
    while out and out[-1] < 0:
        out.pop()
    tot = sum(abs(v) for v in out)
    if tot > span * 1.12 and len(out) > 1:
        out.pop()
        while out and out[-1] < 0:
            out.pop()
        tot = sum(abs(v) for v in out)
    k = span / tot
    return [v * k for v in out]


def _plan_block(p, kind, rng, plan):
    wmin, wmax, D0 = HOUSE[kind]
    coords = list(p.exterior.coords)[:-1]
    E = []
    for i in range(len(coords)):
        (ax, ay), (bx, by) = coords[i], coords[(i + 1) % len(coords)]
        ln = math.hypot(bx - ax, by - ay)
        if ln < 0.05:
            continue
        ux, uy = (bx - ax) / ln, (by - ay) / ln
        nx, ny = -uy, ux                                   # inward: the outline runs anticlockwise
        e = dict(a=(ax, ay), u=(ux, uy), n=(nx, ny), len=ln, front=False, D=0.0, cs=False, ce=False)
        if ln >= 3.0:
            hits, depths, far_front = 0, [], 0
            for f in (0.2, 0.5, 0.8):
                px, py = ax + ux * ln * f, ay + uy * ln * f
                hits += _front(px - nx * 1.2, py - ny * 1.2)
                d, q = _ray_depth(p, px, py, nx, ny)
                depths.append(d)
                if q is not None and _front(q[0] + nx * 1.2, q[1] + ny * 1.2):
                    far_front += 1
            e['front'] = hits >= 2
            avail = min(depths)
            if e['front']:
                if far_front >= 2:
                    D = min(D0, avail / 2)                 # back to back with the row on the other street
                else:
                    D = min(D0 * 1.15, avail)
                    if avail - D < 4.0:
                        D = avail                          # no yard thinner than 4 m behind a row
                e['D'] = D if D >= 4.5 else 0.0
                e['front'] = e['D'] > 0
        E.append(e)
    m = len(E)
    if not any(e['front'] for e in E):
        return
    for i in range(m):                                     # corners: a left turn of 60..120 degrees between two rows
        A, B = E[i - 1], E[i]
        cross = A['u'][0] * B['u'][1] - A['u'][1] * B['u'][0]
        dot = A['u'][0] * B['u'][0] + A['u'][1] * B['u'][1]
        if kind in CORNER_KINDS and A['front'] and B['front'] and cross > 0 and abs(dot) < 0.5 \
                and B['len'] > A['D'] + wmin and A['len'] > B['D'] + wmin:
            B['cs'] = True
            A['ce'] = True
    # hẻm: across the longest row, through the whole block
    alleys = []
    if kind in HEM_EVERY:
        rows = sorted((e for e in E if e['front']), key=lambda e: -e['len'])
        e = rows[0]
        j = E.index(e)
        if e['len'] >= 1.4 * HEM_EVERY[kind][0]:
            (ax, ay), (ux, uy), (nx, ny) = e['a'], e['u'], e['n']
            t = (E[j - 1]['D'] if e['cs'] else 0) + rng.uniform(9.0, 13.0)
            t1 = e['len'] - (E[(j + 1) % m]['D'] if e['ce'] else 0) - 8.0
            while t < t1:
                x, y = ax + ux * t, ay + uy * t
                strip = LineString([(x - nx * 2.0, y - ny * 2.0), (x + nx * 160, y + ny * 160)]).buffer(HEM_W / 2, cap_style='flat')
                parts = [q for q in polys(strip.intersection(p)) if q.distance(Point(x, y)) < 0.5]
                if parts and parts[0].area > HEM_W * 2 * e['D']:
                    alleys.append(dict(poly=parts[0], a=(x, y), u=(nx, ny)))
                t += rng.uniform(*HEM_EVERY[kind])
    alley_all = unary_union([al['poly'] for al in alleys]) if alleys else Polygon()
    plan['alleys'] += [al['poly'] for al in alleys]
    placed, mine = [], []

    def put(foot, h):
        if foot.intersection(p).area < 0.85 * foot.area or foot.intersection(alley_all).area > 0.03 * foot.area:
            return False
        if any(foot.intersection(q).area > 0.08 * foot.area for q in placed):
            return False
        placed.append(foot)
        h['foot'] = foot
        plan['houses'].append(h)
        mine.append(h)
        return True
    for i, e in enumerate(E):
        if not e['front']:
            continue
        (ax, ay), (ux, uy), (nx, ny) = e['a'], e['u'], e['n']
        D, ln = e['D'], e['len']
        A, C = E[i - 1], E[(i + 1) % m]
        ang = math.atan2(uy, ux)
        t0, t1 = 0.0, ln
        if e['cs']:                                        # the corner house: narrow on this street, deep along row A
            w = min(A['D'], rng.uniform(5.5, 7.5))
            if put(_quad(ax, ay, ux, uy, nx, ny, 0, w, 0, D), dict(kind=kind, role='corner', cx=ax + ux * w / 2, cy=ay + uy * w / 2,
                                                                   ang=ang, w=w, D=D, side_len=D)):
                t0 = w
        if e['ce']:
            t1 = ln - C['D']
        band = _quad(ax, ay, ux, uy, nx, ny, t0, t1, 0, D)
        cuts = []
        for al in alleys:
            q = al['poly'].intersection(band)
            if q.area > 0.5:
                ts = [(c[0] - ax) * ux + (c[1] - ay) * uy for c in polys(q)[0].exterior.coords]
                cuts.append((min(ts) - 0.1, max(ts) + 0.1))
        free, s = [], t0
        for c0, c1 in sorted(cuts):
            if c0 > s:
                free.append((s, c0))
            s = max(s, c1)
        if t1 > s:
            free.append((s, t1))
        for s0, s1 in free:
            t = s0
            for w in _row(rng, kind, s0, s1):
                if w > 0:
                    put(_quad(ax, ay, ux, uy, nx, ny, t, t + w, 0, D),
                        dict(kind=kind, role='row', cx=ax + ux * (t + w / 2), cy=ay + uy * (t + w / 2), ang=ang, w=w, D=D))
                t += abs(w)
    # houses along the hẻm, facing it, behind the rows of the streets
    for al in alleys:
        (x, y), (ux, uy) = al['a'], al['u']
        length = _ray_depth(p, x, y, ux, uy)[0]
        for sgn in (1, -1):
            nx, ny = -uy * sgn, ux * sgn                   # away from the alley
            ox, oy = x + nx * HEM_W / 2, y + ny * HEM_W / 2
            depth = 8.0
            t = 0.0
            hk = 'hem'
            for w in _row(rng, hk, 0.0, length):
                if w > 0:
                    cx, cy = ox + ux * (t + w / 2), oy + uy * (t + w / 2)
                    ang = math.atan2(-nx, ny)              # the house's depth (local +y) runs away from the alley
                    lx, ly = math.cos(ang), math.sin(ang)
                    foot = Polygon([(cx - lx * w / 2, cy - ly * w / 2), (cx + lx * w / 2, cy + ly * w / 2),
                                    (cx + lx * w / 2 + nx * depth, cy + ly * w / 2 + ny * depth),
                                    (cx - lx * w / 2 + nx * depth, cy - ly * w / 2 + ny * depth)])
                    put(foot, dict(kind=hk, role='hem', cx=cx, cy=cy, ang=ang, w=w, D=depth))
                t += abs(w)
    # which side and back walls show: nothing of this block stands against them
    allfoot = unary_union(placed) if placed else Polygon()
    for h in mine:
        others = unary_union([q for q in placed if q is not h['foot']] or [Polygon()])
        c, s_ = math.cos(h['ang']), math.sin(h['ang'])
        X = lambda lx, ly: (h['cx'] + c * lx - s_ * ly, h['cy'] + s_ * lx + c * ly)  # noqa: E731
        hw, D = h['w'] / 2, h['D']

        def shows(x0, y0, x1, y1):
            strip = Polygon([X(x0, y0), X(x1, y0), X(x1, y1), X(x0, y1)])
            return strip.intersection(others).area < 0.4 * strip.area
        h['sides'] = [sd for sd, (x0, x1) in ((-1, (-hw - 0.7, -hw - 0.1)), (1, (hw + 0.1, hw + 0.7))) if shows(x0, 0.8, x1, D - 0.8)]
        if h['role'] == 'corner':
            h['sides'] = [sd for sd in h['sides'] if sd != -1]    # the left side is the second shop front
        h['back'] = shows(-hw + 0.3, D + 0.1, hw - 0.3, D + 0.7)
    rest = p.difference(allfoot.buffer(0.3)).difference(alley_all.buffer(0.2))
    plan['yards'] += [q for q in polys(rest) if q.area > 15 and not q.buffer(-1.0).is_empty]


def plan_blocks():
    """every house of the map: the free land between the streets, cut into blocks, each planned by its kind"""
    rng = random.Random(2026)
    plan = dict(houses=[], alleys=[], yards=[], blocks=[])
    free = land.buffer(-1.0).intersection(inland).difference(NO_HOUSES)
    lots = [(rect_poly(r), k) for r, k in LOTS]
    for p in polys(free):
        if p.area < 250 or p.buffer(-4.0).is_empty:
            continue
        p = orient(p.simplify(0.5), 1.0)
        if not p.is_valid or p.is_empty or p.geom_type != 'Polygon':
            continue
        over = sorted(((p.intersection(lp).area, k) for lp, k in lots), reverse=True)
        dk = district_kind(*p.centroid.coords[0])
        kind = over[0][1] if over and over[0][0] > 0.15 * min(p.area, 400) else dk
        if dk == 'tt':
            kind = 'tt'                                    # Thủ Thiêm is all new buildings
        n_h, n_y = len(plan['houses']), len(plan['yards'])
        _plan_block(p, kind, rng, plan)
        if len(plan['houses']) > n_h:
            plan['blocks'].append(p)
        else:                                              # nothing fits (review r5: a bare paved strip by the river)
            del plan['yards'][n_y:]
    return plan


PLAN = plan_blocks()
# traffic islands: pieces of pavement the roads close in on every side (two roads meeting at a sharp angle) get a lawn
# and a tree instead of a bare paved triangle
_r_edge = roads.buffer(0.4)
# medians: where the sidewalks of two roads meeting at a sharp angle overlap in a long wedge, a lawn with trees
_bands = [b_.difference(roads) for b_ in walk_outer[:len(road_geo)]]
MEDIANS = []
for _i in range(len(_bands)):
    for _j in range(_i + 1, len(_bands)):
        if not _bands[_i].intersects(_bands[_j]):
            continue
        for _q in polys(_bands[_i].intersection(_bands[_j]).difference(walks).difference(plazas).difference(PROM)):
            if _q.area > 30 and _q.buffer(-1.0).area > 6:
                MEDIANS.append(_q.buffer(-1.0))
ISLANDS = [q for q in polys(SIDEWALKS.difference(roads).difference(walks).difference(plazas).difference(PROM))
           if 20 < q.area < 600 and not q.interiors and q.exterior.intersection(_r_edge).length > 0.85 * q.exterior.length
           and q.buffer(-1.2).area > 4]
HOUSE_FOOT = unary_union([h['foot'] for h in PLAN['houses']] or [Polygon()])
ALLEYS = unary_union(PLAN['alleys'] or [Polygon()])

# where the ground stays level: streets, pavements, lots, landmarks, shores, river banks, beaches, mountain feet
FLAT = unary_union([roads.buffer(3.5), unary_union(walk_outer), walks, plazas, lm_keepout,
                    unary_union(PLAN['blocks'] or [Polygon()]).buffer(2.0), coast.exterior.buffer(6.0), river.buffer(6.0),
                    beach_all.buffer(3.0), unary_union([o.buffer(2.0) for _, o in mountains]),
                    unary_union([ln.buffer(b['w'] / 2 + 4) for ln, b in bridge_lines]),
                    # lakes and flower fields sit on level ground, and so do the 'flats' circles (forecourts)
                    unary_union([shapely.affinity.scale(Point(tb(lk['ellipse'][:2])).buffer(1.0, 24), lk['ellipse'][2] + 3, lk['ellipse'][3] + 3)
                                 for lk in L.get('lakes', [])] or [Polygon()]),
                    unary_union([rect_poly(pk['rect']).buffer(1.5) for pk in L['parks'] if pk.get('kind') == 'flowers'] or [Polygon()]),
                    unary_union([Point(tb((x, y))).buffer(r, 24) for x, y, r in L.get('flats', [])] or [Polygon()])])
shapely.prepare(FLAT)


def rolling_z(xy):
    """gentle rolling grass (up to ~1.5 m) where nothing is built, level near anything that is"""
    x, y = xy[:, 0], xy[:, 1]
    n = (0.6 * np.sin(x / 21.0 + 1.3) * np.cos(y / 17.0 - 0.7) + 0.35 * np.sin((x + 2 * y) / 9.5 + 2.1) * np.cos((x - y) / 12.0)
         + 0.18 * np.sin(x / 5.3 - y / 7.1))
    hills = np.clip(n + 0.3, 0.0, None) * 2.1
    return hills * smoothstep(1.5, 14.0, shapely.distance(FLAT, shapely.points(xy)))


def tri_grid(poly, step):
    """points on a grid inside poly plus its outlines (holes too), Delaunay triangles kept inside -> (xy, tris)"""
    minx, miny, maxx, maxy = poly.bounds
    gx, gy = np.meshgrid(np.arange(minx + step / 2, maxx, step), np.arange(miny + step / 2, maxy, step))
    pts = np.c_[gx.ravel(), gy.ravel()]
    pts = pts[shapely.contains_xy(poly.buffer(-step * 0.3), pts[:, 0], pts[:, 1])]
    ring = []
    for p in polys(poly):
        for r in [p.exterior, *p.interiors]:
            ln = LineString(r.coords)
            n = max(8, int(ln.length / step))
            ring += [ln.interpolate(i / n, normalized=True).coords[0] for i in range(n)]
    allp = np.vstack([pts, np.asarray(ring)]) if len(pts) else np.asarray(ring)
    tg = shapely.delaunay_triangles(shapely.multipoints(allp))
    tri = shapely.get_coordinates(tg).reshape(-1, 4, 2)[:, :3, :]
    cen = tri.mean(axis=1)
    keep = shapely.contains_xy(poly, cen[:, 0], cen[:, 1])
    index = {(round(x, 4), round(y, 4)): i for i, (x, y) in enumerate(allp)}
    tris = [tuple(index[(round(x, 4), round(y, 4))] for x, y in t) for t in tri[keep]]
    return allp, tris


def ccw(xy, tris):
    out = []
    for a, b, c in tris:
        (ax, ay), (bx, by), (cx, cy) = xy[a], xy[b], xy[c]
        out.append((a, b, c) if (bx - ax) * (cy - ay) - (by - ay) * (cx - ax) >= 0 else (a, c, b))
    return out


def build_terrain(coll):
    g = Builder('IS_Ground', coll)
    # the sea bed under everything, so water has depth
    g.face([(-320, -300, -3.6), (320, -300, -3.6), (320, 300, -3.6), (-320, 300, -3.6)], 'sand')
    g.slab(river.intersection(coast.buffer(30)), None, -2.4, 'earthDark')
    # grass: the land less the beaches, rolling a little where nothing is built; parks a darker green
    grass = land.difference(beach_all)
    xy, tris = tri_grid(grass, 3.0)
    z = rolling_z(xy)
    tris = ccw(xy, tris)
    lawn = unary_union([rect_poly(pk['rect']) for pk in L['parks'] if pk.get('kind') == 'lawn'] or [Polygon()])
    cen = np.array([[(xy[a][0] + xy[b][0] + xy[c][0]) / 3, (xy[a][1] + xy[b][1] + xy[c][1]) / 3] for a, b, c in tris])
    in_lawn = shapely.contains_xy(lawn, cen[:, 0], cen[:, 1]) if not lawn.is_empty else np.zeros(len(tris), bool)
    V = [(x, y, zz) for (x, y), zz in zip(xy, z)]
    for t, lw in zip(tris, in_lawn):
        g.face([V[t[0]], V[t[1]], V[t[2]]], 'grassDark' if lw else 'grass')
    # the sea wall and the river banks
    for p in polys(grass):
        for r in [p.exterior, *p.interiors]:
            cs = list(r.coords)
            for a, b in zip(cs, cs[1:]):
                g.face([(a[0], a[1], FLOOR_BOTTOM), (b[0], b[1], FLOOR_BOTTOM), (b[0], b[1], GROUND_Z), (a[0], a[1], GROUND_Z)], 'rock')
    # beaches: sand sloping from the grass down under the sea
    for b, band in beach_polys:
        poly = band.intersection(coast.buffer(b['width'] / 2)).difference(river)
        bxy, btris = tri_grid(poly, 1.6)
        pts = shapely.points(bxy)
        sgn = shapely.distance(coast.exterior, pts) * np.where(shapely.contains_xy(coast, bxy[:, 0], bxy[:, 1]), 1.0, -1.0)
        half = b['width'] / 2
        # wet sand at the waterline, dry 2 m up the beach (review 2026-11-13: half the sand was under the sea)
        bz = np.where(sgn >= 0, -0.6 + 0.47 * np.clip(sgn / 2.5, 0, 1) + 0.15 * np.clip(sgn / half, 0, 1),
                      -0.6 + 0.26 * np.maximum(sgn, -half))
        g.mesh([(x, y, zz) for (x, y), zz in zip(bxy, bz)], ccw(bxy, btris), 'sand')
    g.build()


def mountain_height(mt, outline, xy, road_poly):
    """a rounded hill: a broad dome and soft peaks (cosine profiles, no cone tips), a level top for the statue or the
    lighthouse, a slow undulation so the slopes are not perfect, flaring out at the foot and flattened by roads"""
    pts = shapely.points(xy)
    inside = shapely.distance(outline.exterior, pts) * np.where(shapely.contains_xy(outline, xy[:, 0], xy[:, 1]), 1.0, -1.0)
    c = np.asarray(outline.centroid.coords[0])
    rmax = max(math.dist(c, q) for q in outline.exterior.coords)
    d0 = np.hypot(xy[:, 0] - c[0], xy[:, 1] - c[1])
    hmax = max(p[2] for p in mt['peaks'])
    z = 0.45 * hmax * 0.5 * (1 + np.cos(np.pi * np.clip(d0 / (rmax * 1.1), 0, 1)))
    peaks = []
    for px, py, ph, pr in mt['peaks']:
        X, Y = tb((px, py))
        d = np.hypot(xy[:, 0] - X, xy[:, 1] - Y)
        peaks.append((d, ph))
        z = np.maximum(z, ph * 0.5 * (1 + np.cos(np.pi * np.clip(d / (pr * 1.8), 0, 1))))
    z = z + 0.9 * np.sin(xy[:, 0] / 7.3 + 0.4) * np.cos(xy[:, 1] / 6.1 - 1.2) * smoothstep(2.0, 9.0, z)
    for d, ph in peaks:
        w = smoothstep(5.5, 8.5, d)
        z = ph * (1 - w) + z * w
    z = z * smoothstep(0.0, 14.0, inside)
    if road_poly is not None:
        z = z * smoothstep(1.5, 9.0, shapely.distance(road_poly, pts))
    return np.maximum(z, 0.0)


def build_mountains(coll):
    ns = load_builders()
    rocks = ns['Geo'](17)
    rng = random.Random(17)
    for mt, outline in mountains:
        if mt['id'] in MODELLED_MOUNTAINS:
            continue
        g = Builder('IS_' + mt['id'], coll)
        xy, tris = tri_grid(outline, 2.6)
        z = mountain_height(mt, outline, xy, roads)
        tris = ccw(xy, tris)
        V = [(x, y, zz) for (x, y), zz in zip(xy, z)]
        tops = [tb((px, py)) for px, py, _, _ in mt['peaks']]
        for t in tris:
            a, b, c = (Vector(V[k]) for k in t)
            n = (b - a).cross(c - a).normalized()
            cx, cy, zc = (a.x + b.x + c.x) / 3, (a.y + b.y + c.y) / 3, (a.z + b.z + c.z) / 3
            slope = math.degrees(math.acos(max(-1.0, min(1.0, n.z))))
            hx = (cx * 0.37 + cy * 0.61) % 9
            if any(math.dist((cx, cy), q) < 5.2 for q in tops):
                key = 'stone'                                  # the paved top round the statue / lighthouse
            elif slope > 48 or hx > 8.65:
                key = 'rock'
            elif zc < 0.35:
                key = 'grass'
            else:
                key = 'leafDark' if hx < 3.2 else ('leaf' if hx < 6.6 else 'grassDark')
            g.face([V[t[0]], V[t[1]], V[t[2]]], key)
        g.build()
        # boulders on the slopes
        minx, miny, maxx, maxy = outline.bounds
        hmax = max(p[2] for p in mt['peaks'])
        for _ in range(int(outline.area / 220)):
            x, y = rng.uniform(minx, maxx), rng.uniform(miny, maxy)
            if not outline.buffer(-4).contains(Point(x, y)):
                continue
            zz = ground_z(x, y)
            if 2.0 < zz < hmax - 5:
                rocks.ico(rng.choice(['rock', 'rockDark', 'concrete']), rng.uniform(0.8, 2.0), ns['T'](x, y, zz - 0.3), sub=1, jit=0.3, sc=(1, rng.uniform(0.7, 1.2), 0.65))
        if 'stairs' in mt:
            s = Builder('LF_' + mt['id'] + '_Stairs', coll)
            line = LineString(path_pts(mt['stairs'], 0.6))
            ribbon = line.buffer(1.3, cap_style='flat', join_style='mitre')

            def zf(x, y, mt=mt, outline=outline):
                return float(mountain_height(mt, outline, np.array([[x, y]]), roads)[0]) + 0.18
            s.slab(ribbon.segmentize(0.8), None, 0.0, 'stone', zfun=zf)
            s.build()
    for x, y, rad in L['rocks']:
        X, Y = tb((x, y))
        rocks.ico(rng.choice(['rock', 'rockDark']), rad, ns['T'](X, Y, -0.6), sub=1, jit=0.3, sc=(1, 0.9, 0.7))
    ns['make_obj'](rocks, 'IS_Boulders', coll, sub=0, angle=30, recenter=True)


def ground_z(x, y):
    """height of the ground at Blender (x, y): mountain, rolling grass or level"""
    xy = np.array([[x, y]])
    for mt, outline in mountains:
        if outline.contains(Point(x, y)):
            return float(mountain_height(mt, outline, xy, roads)[0])
    return float(rolling_z(xy)[0])


terrain_z = lambda p: ground_z(p[0], p[1])  # noqa: E731


# ---------------------------------------------------------------------------------------------------------------
# streets
# ---------------------------------------------------------------------------------------------------------------
def build_streets(coll):
    bridge_cut = unary_union([ln.buffer(b['w'] / 2 + 0.5, cap_style='flat') for ln, b in bridge_lines])
    on_land = land.buffer(-0.2)
    road_top = roads.intersection(on_land)
    # the car bridges' decks carry the road across their ends: no second road surface under them
    car_decks = unary_union([ln.buffer(b['w'] / 2, cap_style='flat') for ln, b in bridge_lines
                             if not b.get('walk') and b['id'] not in MODELLED_BRIDGES] or [Polygon()])
    road_top = road_top.difference(car_decks)
    shapely.prepare(road_top)
    walkway = unary_union(walk_outer).union(walks).union(plazas)
    walkway = walkway.union(unary_union([f.buffer(3.0) for f in lm_foot.values()]))
    walkway = walkway.union(unary_union(PLAN['blocks'] or [Polygon()])).union(ALLEYS)   # house blocks stand on a kerb, hẻm paved
    # a bridge that lands short of the pavement gets a paved path on to it (review r5: Cầu Mống ended on the grass)
    shapely.prepare(walkway)
    for ln, br in bridge_lines:
        for end in (ln.coords[0], ln.coords[-1]):
            pt = Point(end)
            gap = walkway.distance(pt)
            if 0.3 < gap < 15 and land.contains(pt):
                near = nearest_points(walkway, pt)[0]
                walkway = walkway.union(LineString([pt, near]).buffer(min(br['w'], 4.0) / 2, cap_style='round'))
    walkway = walkway.difference(roads).intersection(on_land).difference(beach_all)
    r = Builder('IX_Roads', coll)
    r.slab(road_top, -0.1, ROAD_Z, 'asphalt', 'curb')
    r.slab(walkway, -0.1, WALK_Z, 'paving', 'curb')
    for q in ISLANDS:
        r.slab(q.buffer(-1.0), WALK_Z - 0.02, WALK_Z + 0.08, 'grassDark', 'curb')
    for q in MEDIANS:
        r.slab(q, WALK_Z - 0.02, WALK_Z + 0.08, 'grassDark', 'curb')
    # islands of the roundabouts
    for ra, isl in rings_geo:
        if not ra.get('model'):
            r.slab(isl, -0.1, WALK_Z + 0.12, ra['inner'], 'curb')
    # riverside and Thủ Thiêm promenades
    prom = unary_union([LineString(path_pts(p['path'], 0.8)).buffer(p['w'] / 2, cap_style='flat') for p in L['promenades']
                        if p.get('kind') != 'cycle'])
    r.slab(prom.intersection(on_land).difference(roads), -0.1, WALK_Z, 'paving', 'curb')
    # cycle paths: a green lane
    cyc = unary_union([LineString(path_pts(p['path'], 0.8)).buffer(p['w'] / 2, cap_style='flat') for p in L['promenades']
                       if p.get('kind') == 'cycle'] or [Polygon()])
    if not cyc.is_empty:
        r.slab(cyc.intersection(on_land).difference(roads).difference(prom), -0.1, WALK_Z, 'poGreen', 'curb')
    r.build()
    # lane marks
    m = Builder('IX_RoadMarks', coll)
    no_dash = JUNCTIONS.union(unary_union([z[4].buffer(0.6) for z in ZEBRAS] or [Polygon()]))
    shapely.prepare(no_dash)
    for ln, w in road_geo:
        if w < 9:
            continue
        n = int(ln.length // 4)
        for i in range(n):
            a = ln.interpolate(i * 4 + 0.5)
            b = ln.interpolate(i * 4 + 2.5)
            if not road_top.contains(a) or not road_top.contains(b) or bridge_cut.contains(a) or no_dash.intersects(LineString([a, b])):
                continue
            seg = LineString([a, b]).buffer(0.09, cap_style='flat')
            m.slab(seg, None, ROAD_Z + 0.05, 'roadLine')
    # zebra crossings on each arm of the signalled crossings (ZEBRAS). Paint 5 cm thick: the GLB stores positions in
    # 14 bits over the whole map (3 cm steps), and thinner marks sank into the asphalt (review r11)
    for p, dx, dy, hw, _rect in ZEBRAS:
        k = -hw
        while k <= hw:
            m.oriented_box(p.x - dy * k, p.y + dx * k, ROAD_Z, 2.8, 0.45, 0.05, math.atan2(dy, dx), 'roadLine')
            k += 0.9
    m.build()


def build_bridges(coll):
    """blockout decks for the bridges that have no model of their own yet (see MINI_SCRIPTS)"""
    b = Builder('LM_Bridges', coll)
    for ln, br in bridge_lines:
        if br['id'] in MODELLED_BRIDGES:
            continue
        L_ = ln.length
        # 'ramp': metres from road level to the deck; 'flat': metres kept level with the road at each end (a car bridge
        # whose end is a junction stays at road level across it and its pavement, then rises over the water)
        ramp = br.get('ramp', 16.0)
        flat = br.get('flat', [0.0, 0.0])
        z_end = WALK_Z if br.get('walk') else ROAD_Z + 0.01
        n = int(L_ / 0.8)
        over = []
        left, right = [], []
        for i in range(n + 1):
            t = i * L_ / n
            p = ln.interpolate(t)
            q = ln.interpolate(min(L_, t + 0.3))
            o = ln.interpolate(max(0, t - 0.3))
            dx, dy = q.x - o.x, q.y - o.y
            dl = math.hypot(dx, dy) or 1.0
            nx, ny = -dy / dl, dx / dl
            k = min(t - flat[0], L_ - t - flat[1])
            z = z_end + (br['deck'] - z_end) * float(smoothstep(0, ramp, k))
            over.append(river.buffer(1.0).contains(p) or z > z_end + 0.6)
            # 'wMid': wider in the middle, a leaf in plan (cầu đi bộ lá dừa nước)
            hw = (br['w'] + (br.get('wMid', br['w']) - br['w']) * math.sin(math.pi * t / L_)) / 2
            left.append((p.x + nx * hw, p.y + ny * hw, z))
            right.append((p.x - nx * hw, p.y - ny * hw, z))
        key = 'bridgeWhite'
        top = 'paving' if br.get('walk') else 'asphalt'          # footbridges: a paved deck
        for i in range(n):
            a, bb, c, d = left[i], left[i + 1], right[i + 1], right[i]
            b.face([d, c, bb, a], top)
            for e0, e1 in ((a, bb), (c, d)):
                b.face([(e0[0], e0[1], e0[2] - 0.7), (e1[0], e1[1], e1[2] - 0.7), (e1[0], e1[1], e1[2]), (e0[0], e0[1], e0[2])], key)
            for e in (a, d):
                # railing posts only where the deck is over the water or up on its ramp, not across a junction
                if i % 3 == 0 and (over[i] or br.get('walk')):
                    b.box(e[0] - 0.08, e[1] - 0.08, e[2], e[0] + 0.08, e[1] + 0.08, e[2] + 1.1, 'bridgeWhite')
            # and a parapet between them (review r5: the player walked off the side of Thủ Thiêm 3's ramp between posts)
            if (over[i] and over[i + 1]) or br.get('walk'):
                for e0, e1, f0, f1 in ((a, bb, d, c), (d, c, a, bb)):
                    ux, uy = f0[0] - e0[0], f0[1] - e0[1]
                    ul = math.hypot(ux, uy) or 1.0
                    ux, uy = ux / ul * 0.16, uy / ul * 0.16
                    i0, i1 = (e0[0] + ux, e0[1] + uy, e0[2]), (e1[0] + ux, e1[1] + uy, e1[2])
                    up = lambda v: (v[0], v[1], v[2] + 0.75)  # noqa: E731
                    b.face([e0, e1, up(e1), up(e0)], 'bridgeWhite')
                    b.face([i1, i0, up(i0), up(i1)], 'bridgeWhite')
                    b.face([up(e0), up(e1), up(i1), up(i0)], 'bridgeWhite')
        if 'pylon' in br:
            px, py = tb(br['pylon'])
            prev = None
            for k in range(14):
                t = k / 13
                a = math.radians(8 + 100 * t)
                x = px - 18 * (1 - math.cos(a)) * 0.6
                z = br['deck'] + 30 * math.sin(a) * (1 - 0.15 * t)
                if prev:
                    b.oriented_box((prev[0] + x) / 2, py, min(prev[1], z), max(1.6, abs(x - prev[0]) + 1.2), 1.6, abs(z - prev[1]) + 1.2, 0.0, 'bridgeWhite')
                prev = (x, z)
            for k, off in enumerate((-22, -16, -10, -4, 6, 12)):
                x0 = px + off
                top = (px - 6, py, br['deck'] + 26)
                d = Vector(top) - Vector((x0, py, br['deck']))
                steps = 10
                for s_ in range(steps):
                    p0 = Vector((x0, py, br['deck'])) + d * (s_ / steps)
                    b.box(p0.x - 0.06, py - 0.06, p0.z, p0.x + 0.06, py + 0.06, p0.z + d.z / steps, 'archDark')
    # piers (layout v8): a timber deck on posts from the shore into the water, a ticket kiosk at the hydrofoil pier
    for pr in L.get('piers', []):
        pts = path_pts(pr['path'], 50.0)
        hw = pr['w'] / 2
        for (ax, ay), (bx, by) in zip(pts, pts[1:]):
            ln_ = math.hypot(bx - ax, by - ay)
            yaw = math.atan2(by - ay, bx - ax)
            b.oriented_box((ax + bx) / 2, (ay + by) / 2, 0.3, ln_, pr['w'], 0.25, yaw, 'wood')
            ux, uy = (bx - ax) / ln_, (by - ay) / ln_
            k = 0.0
            while k <= ln_:
                for sgn in (1, -1):
                    px_, py_ = ax + ux * k - uy * sgn * (hw - 0.2), ay + uy * k + ux * sgn * (hw - 0.2)
                    b.box(px_ - 0.12, py_ - 0.12, -2.4, px_ + 0.12, py_ + 0.12, 1.4, 'wood')
                k += 3.0
        if pr.get('kiosk'):
            (ex, ey) = pts[-1]
            b.box(ex - 1.6, ey - 1.6, 0.55, ex + 1.6, ey + 1.6, 3.0, 'creamLight', 'roofRed')
    b.build()


# ---------------------------------------------------------------------------------------------------------------
# houses (blockout boxes in the kit colours)
# ---------------------------------------------------------------------------------------------------------------
NP_WALLS = ['wallPink', 'cream', 'creamLight', 'wallTeal', 'wallOchre', 'wallMint', 'wallSky', 'wallCream', 'wallBeige']
HEM_WALLS = ['wallBeige', 'cream', 'wallMint', 'wallGrey', 'wallPink', 'creamLight', 'wallSky']
CL_WALLS = ['wallOchre', 'cream', 'wallBeige', 'wallPink']
KS_PALETTES = [('white', 'glassBlue', 'concrete'), ('wallSky', 'glassDark', 'white'), ('wallCream', 'glassTeal', 'white'),
               ('wallMint', 'glassBlue', 'white')]
# frontage min, max and depth (m) per house kind (docs/map-v5.md, "Bộ nhà")
FRONT = {'np': (4.0, 5.4, 11.0), 'hem': (3.6, 4.4, 9.0), 'bar': (4.0, 5.0, 10.0), 'cl': (4.4, 4.8, 12.0), 'ks': (6.4, 8.4, 12.0)}


def tum(g, w, D, Ht, wall):
    """the stair head on a flat roof, with a tin lid (a Saigon roof, docs/map-v5.md)"""
    hw = w / 2
    g.box(wall, -hw + 0.3, D - 3.6, Ht + 0.06, hw - 0.3, D - 0.5, Ht + 2.5, ch=0)
    g.box('tinBlue', -hw + 0.1, D - 3.8, Ht + 2.5, hw - 0.1, D - 0.3, Ht + 2.66, ch=0.02)
    g.box('archDark', -0.45, D - 3.63, Ht + 0.1, 0.45, D - 3.58, Ht + 2.1, ch=0)


def upper_facade(ns, g, rng, w, floors):
    """the floors above the shop of a tube house, as house_plain draws them: floor bands, a balcony with plants on
    every other floor, shuttered windows between, and the shop sign"""
    T = ns['T']
    hw = w / 2
    GF, F = 3.4, 3.0
    for i in range(floors):
        z0 = GF + i * F
        g.box('creamLight', -hw - 0.05, -0.18, z0, hw + 0.05, 0.0, z0 + 0.2, ch=0)
        if i % 2 == 0:
            g.box('creamLight', -hw + 0.15, -0.95, z0, hw - 0.15, 0.0, z0 + 0.16, ch=0)
            nb = max(2, int((w - 0.5) / 0.45))             # balusters as thin boxes: a tenth of the triangles of rods
            for k in range(nb + 1):
                x = -hw + 0.25 + k * (w - 0.5) / nb
                g.box('metal', x - 0.025, -0.93, z0 + 0.16, x + 0.025, -0.88, z0 + 1.02, ch=0)
            g.box('metal', -hw + 0.15, -0.95, z0 + 1.02, hw - 0.15, -0.85, z0 + 1.1, ch=0)
            g.box('wood', -0.85, -0.08, z0 + 0.16, 0.85, 0.0, z0 + 2.3, ch=0)
            for k in range(2):
                g.ico(rng.choice(['leaf', 'flowerPink', 'leafLight']), 0.32, T(-hw + 0.6 + k * (w - 1.2), -0.6, z0 + 0.55), sub=0, jit=0.1)
        else:
            for cx in (-w / 4, w / 4):
                g.box('creamLight', cx - 0.6, -0.12, z0 + 0.7, cx + 0.6, 0.0, z0 + 2.2, ch=0)
                g.box('glassTeal', cx - 0.45, -0.14, z0 + 0.85, cx + 0.45, -0.1, z0 + 2.05, ch=0)
                g.box('wallTeal', cx - 0.62 - 0.42, -0.08, z0 + 0.85, cx - 0.62, 0.0, z0 + 2.05, ch=0)
                g.box('wallTeal', cx + 0.62, -0.08, z0 + 0.85, cx + 0.62 + 0.42, 0.0, z0 + 2.05, ch=0)
    sc = rng.choice([('signRed', 'signGold'), ('clothBlue', 'white'), ('signGold', 'signRed'), ('wallTeal', 'white')])
    g.box(sc[0], -hw + 0.45, -0.28, GF - 0.52, hw - 0.45, -0.02, GF - 0.08, ch=0)
    g.text(rng.choice(SHOP_SIGNS), 0.27, sc[1], T(0, -0.3, GF - 0.3) @ ns['FACE_S'], extrude=0.025, max_w=w - 1.2)


def tube_house(ns, g, rng, w, D, floors, paint):
    """a tube house with a flat roof: the shop, the floors above (upper_facade), a parapet and a water tank"""
    T = ns['T']
    GF, F = 3.4, 3.0
    Ht = GF + F * floors
    hw = w / 2
    g.box(paint, -hw, 1.4, 0, hw, D, GF, ch=0)
    ns['shop_front'](g, w, GF, rng, wall=paint)
    g.box(paint, -hw, 0, GF, hw, D, Ht, ch=0)
    upper_facade(ns, g, rng, w, floors)
    g.box(paint, -hw, -0.05, Ht, hw, 0.3, Ht + 0.7, ch=0)
    g.box('creamLight', -hw - 0.05, -0.15, Ht + 0.7, hw + 0.05, 0.4, Ht + 0.85, ch=0)
    g.box(rng.choice(['concrete', 'concrete', 'pavingDark', 'stone']), -hw, 0.3, Ht, hw, D, Ht + 0.06, ch=0)
    g.cyl('white', 0.5, 1.2, 10, T(-hw + 1.0, D * 0.6, Ht + 0.06))
    roof_life(g, rng, w, D, Ht + 0.06)
    return Ht + 0.85


def roof_life(g, rng, w, D, z):
    """what a Saigon roof carries: potted plants, a tin awning over half of it, laundry"""
    T = Matrix.Translation
    hw = w / 2
    r = rng.random()
    if r < 0.3:                                            # a roof garden
        for k in range(rng.randint(3, 6)):
            g.ico(rng.choice(['leaf', 'leafLight', 'leafDark', 'flowerPink']), rng.uniform(0.3, 0.55),
                  T((rng.uniform(-hw + 0.6, hw - 0.6), rng.uniform(1.2, D - 0.8), z + 0.3)), sub=0, jit=0.15)
    elif r < 0.5:                                          # a tin awning on posts over the back half
        y0 = D * rng.uniform(0.45, 0.6)
        for x in (-hw + 0.3, hw - 0.3):
            for y in (y0 + 0.2, D - 0.3):
                g.box('metal', x - 0.05, y - 0.05, z, x + 0.05, y + 0.05, z + 2.3, ch=0)
        g.box('tinBlue', -hw + 0.1, y0, z + 2.3, hw - 0.1, D - 0.1, z + 2.42, ch=0)
    elif r < 0.65:                                         # laundry
        g.rod('metal', (-hw + 0.4, D * 0.5, z + 1.6), (hw - 0.4, D * 0.5, z + 1.6), 0.02, 4)
        x = -hw + 0.6
        while x < hw - 0.8:
            g.box(rng.choice(['clothRed', 'clothBlue', 'clothYellow', 'white']), x, D * 0.5 - 0.05, z + 0.95, x + 0.5, D * 0.5 + 0.05, z + 1.58, ch=0)
            x += 0.75


def corner_house(ns, g, rng, w, D, floors, paint):
    """a corner shophouse: a shop front and balconies on the front (-Y) and on the left side (-X), round the corner"""
    T, Rm = ns['T'], ns['Rm']
    GF, F = 3.4, 3.0
    Ht = GF + F * floors
    hw = w / 2
    side = T(-hw, D / 2, 0) @ Rm(0, 0, -math.pi / 2)       # the left wall as a facade: local y=0 on it, facing -X
    g.box(paint, -hw + 1.4, 1.4, 0, hw, D, GF, ch=0)
    ns['shop_front'](g, w, GF, rng, wall=paint)
    g.push(side)
    ns['shop_front'](g, D, GF, rng, wall=paint)
    g.pop()
    g.box(paint, -hw, 0, GF, hw, D, Ht, ch=0)
    upper_facade(ns, g, rng, w, floors)
    g.push(side)
    upper_facade(ns, g, rng, D, floors)
    g.pop()
    for M_, ww in ((None, w), (side, D)):                   # parapets on both street faces, the flat roof
        if M_ is not None:
            g.push(M_)
        g.box(paint, -ww / 2, -0.05, Ht, ww / 2, 0.3, Ht + 0.7, ch=0)
        g.box('creamLight', -ww / 2 - 0.05, -0.15, Ht + 0.7, ww / 2 + 0.05, 0.4, Ht + 0.85, ch=0)
        if M_ is not None:
            g.pop()
    g.box('concrete', -hw + 0.3, 0.3, Ht, hw, D, Ht + 0.06, ch=0)
    g.cyl('white', 0.5, 1.2, 10, T(hw - 1.0, D * 0.6, Ht + 0.06))
    return Ht + 0.85


def wall_details(ns, g, rng, w, D, floors, sides, back):
    """what shows on the bare walls of a tube house: windows and AC units on each floor of a side wall that faces a
    hẻm or open ground, a side door, now and then a painted sign; windows, AC units and laundry on a back wall"""
    T, Rm = ns['T'], ns['Rm']
    GF, F = 3.4, 3.0
    Ht = GF + F * floors
    hw = w / 2
    for s in sides:
        x = s * hw

        def bx(mat, y0, z0, y1, z1, d, ch=0.0):
            x0, x1 = sorted((x, x + s * d))
            g.box(mat, x0, y0, z0, x1, y1, z1, ch=ch)
        ys = [D * 0.62] if D < 8 else [D * 0.42, D * 0.78]
        for i in range(floors):
            z0 = GF + i * F
            for y in ys:
                if rng.random() < 0.85:
                    bx('creamLight', y - 0.55, z0 + 0.7, y + 0.55, z0 + 2.0, 0.08)
                    bx('glassTeal', y - 0.42, z0 + 0.82, y + 0.42, z0 + 1.88, 0.11)
            if rng.random() < 0.55:
                y = rng.uniform(1.4, D - 1.4)
                bx('acWhite', y - 0.4, z0 + 0.3, y + 0.4, z0 + 0.85, 0.42, 0.03)
        bx('wood', D * 0.55 - 0.45, 0.0, D * 0.55 + 0.45, 2.2, 0.06)
        bx('creamLight', D * 0.25 - 0.5, 1.0, D * 0.25 + 0.5, 2.0, 0.07)
        bx('glassTeal', D * 0.25 - 0.38, 1.1, D * 0.25 + 0.38, 1.9, 0.1)
        if Ht > 6.0 and rng.random() < 0.35:
            y0, y1 = 1.0, min(D - 1.5, 6.5)
            z0, z1 = GF + 0.4, min(Ht - 0.6, GF + 2.8)
            if y1 - y0 > 2.6 and z1 - z0 > 1.3:
                bg, fg = rng.choice(SIGN_COLOURS)
                bx(bg, y0, z0, y1, z1, 0.07)
                g.text(rng.choice(WALL_ADS), 0.55, fg, T(x + s * 0.09, (y0 + y1) / 2, (z0 + z1) / 2) @ Rm(math.pi / 2, 0, s * math.pi / 2),
                       extrude=0.03, max_w=(y1 - y0) - 0.5)
    if back:
        for i in range(floors + 1):
            z0 = GF + (i - 1) * F if i else 0.0
            for cx in ([-w / 4, w / 4] if w > 4.6 else [0.0]):
                g.box('creamLight', cx - 0.5, D, z0 + 0.8, cx + 0.5, D + 0.08, z0 + 2.0, ch=0)
                g.box('glassTeal', cx - 0.38, D + 0.02, z0 + 0.92, cx + 0.38, D + 0.11, z0 + 1.88, ch=0)
            if i and rng.random() < 0.5:
                cx = rng.uniform(-hw + 0.6, hw - 0.6)
                g.box('acWhite', cx - 0.4, D, z0 + 0.25, cx + 0.4, D + 0.42, z0 + 0.8, ch=0.03)
            if i and rng.random() < 0.3:                     # laundry on a pole
                g.rod('metal', (-hw + 0.3, D + 0.55, z0 + 2.3), (hw - 0.3, D + 0.55, z0 + 2.3), 0.02, 4)
                x = -hw + 0.5
                while x < hw - 0.8:
                    g.box(rng.choice(['clothRed', 'clothBlue', 'clothYellow', 'white']), x, D + 0.5, z0 + 1.6, x + 0.45, D + 0.6, z0 + 2.28, ch=0)
                    x += 0.7


def sidewalk_cafe(g, rng, w):
    """cà phê vỉa hè in front of a shop: a low table and plastic stools on the pavement, a parasol now and then"""
    cx = rng.uniform(-w / 2 + 1.0, w / 2 - 1.0)
    cy = -1.35
    col = rng.choice(['plasticBlue', 'plasticRed'])
    g.box(col, cx - 0.32, cy - 0.32, 0, cx + 0.32, cy + 0.32, 0.42, ch=0)
    for k in range(rng.choice([2, 3, 4])):
        a = k * math.pi / 2 + rng.uniform(-0.3, 0.3)
        x, y = cx + math.cos(a) * 0.65, cy + math.sin(a) * 0.65
        g.box(rng.choice(['plasticBlue', 'plasticRed', 'plasticRed']), x - 0.15, y - 0.15, 0, x + 0.15, y + 0.15, 0.28, ch=0)
    if rng.random() < 0.35:
        g.cyl('white', 0.03, 2.1, 5, Matrix.Translation((cx, cy, 0)))
        g.cone(rng.choice(['signRed', 'clothYellow', 'clothBlue', 'wallTeal']), 1.1, 0.45, 8, Matrix.Translation((cx, cy, 1.85)))
    return cx, cy


SHOP_SIGNS = ['PHỞ', 'CƠM TẤM', 'BÁNH MÌ', 'THUỐC BẮC', 'VẢI SỢI', 'CÀ PHÊ', 'HỦ TIẾU', 'TẠP HOÁ']
WALL_ADS = ['CÀ PHÊ', 'PHỞ', 'BÁNH MÌ', 'TẠP HOÁ', 'SỬA XE', 'HỦ TIẾU', 'NƯỚC MÍA', 'CƠM TẤM']
SIGN_COLOURS = [('signRed', 'signGold'), ('clothBlue', 'white'), ('wallTeal', 'white'), ('signGold', 'brownText'), ('poGreen', 'white')]


VT_WALLS = ['wallCream', 'creamLight', 'wallSky', 'wallMint', 'wallPink', 'wallOchre']


def build_house(ns, g, rng, kind, w, D, role='row'):
    """one house in its local frame (facade at y = 0 facing -Y, depth +Y); returns its storeys above the shop and
    its wall colour, or None for a model with its own walls all round"""
    if kind == 'ks':
        ns['modern_block'](g, w, D, rng.choice([3, 4, 4, 5, 6]), rng, palette=rng.choice(KS_PALETTES))
        return None
    if kind == 'tt':
        ns['modern_block'](g, w, D, rng.choice([4, 5, 6, 7, 8]), rng, palette=rng.choice(KS_PALETTES))
        return None
    if kind == 'np' and role == 'row' and w >= 5.2 and rng.random() < 0.18:
        ns['modern_block'](g, w, D, rng.choice([2, 3, 4]), rng)
        return None
    if kind == 'bar' and 'house_bv' in ns:
        ns['house_bv'](ns, g, w, D, rng, floors=rng.choice([1, 2, 2, 3]), paint=rng.choice(ns.get('BV_PAINT', NP_WALLS)),
                       roof='flat', sign=rng.choice(ns.get('BV_SIGNS', ['BAR'])))
        return None
    if kind == 'cl':
        ns['house_plain'](g, w, D, rng, floors=1, paint=rng.choice(CL_WALLS), roof='tile')
        for x in (-w / 4, w / 4):
            ns['lantern'](g, ns['T'](x, -1.1, 2.7))
        return 1, None
    if kind == 'vt':                                       # Vũng Tàu: villas and guest houses, half of them tiled
        floors = rng.choice([1, 1, 2, 2, 3])
        wall = rng.choice(VT_WALLS)
        if rng.random() < 0.5:
            ns['house_plain'](g, w, D, rng, floors=floors, paint=wall, roof='tile')
            return floors, wall
        tube_house(ns, g, rng, w, D, floors, wall)
        return floors, wall
    floors = rng.choice([1, 2, 2, 3, 3, 4]) if kind == 'np' else rng.choice([1, 1, 2])
    wall = rng.choice(NP_WALLS if kind == 'np' else HEM_WALLS)
    if role == 'corner':
        floors = max(floors, 2)
        top = corner_house(ns, g, rng, w, D, floors, wall)
    else:
        top = tube_house(ns, g, rng, w, D, floors, wall)
    if rng.random() < (0.55 if kind == 'np' else 0.3):
        tum(g, w, D, top - 0.85, wall)
    return floors, wall


def keep_clear_points():
    """where the game puts people (NPC posts, archetype spawns, server/content): nothing small gets built there"""
    base = os.path.join(os.path.dirname(os.path.abspath(__file__)), '..', '..', 'server', 'content')
    pts = []
    for f, key in (('npcs.json', 'npcs'), ('archetypes.json', 'archetypes')):
        try:
            for e in json.load(open(os.path.join(base, f), encoding='utf-8')).get(key, []):
                at = e.get('at') or (e.get('spawn') or {}).get('at')
                if at:
                    pts.append((float(at[0]), float(at[1])))
        except OSError:
            pass
    return pts


CAFES = []   # where build_houses put a sidewalk café: no street tree goes there
# the Bùi Viện model (MINI_SCRIPTS 'bui_vien') stands on the west half of its walking street from X -131.8 on: the
# lantern strings of the Chợ Lớn end stop short of it (it has its own lights)
BUI_VIEN_WEST = -134.0


def build_houses(coll):
    ns = load_builders()
    clear = keep_clear_points()
    T, Rm = ns['T'], ns['Rm']
    rng = random.Random(2026)
    geos = {}
    count = 0
    for h in PLAN['houses']:
        kind = h['kind']
        g = geos.setdefault(kind, ns['Geo'](len(geos) + 1))
        depth = len(g.st)
        g.push(T(h['cx'], h['cy'], WALK_Z) @ Rm(0, 0, h['ang']))
        try:
            got = build_house(ns, g, rng, kind, h['w'], h['D'], h['role'])
            if got is not None and kind in ('np', 'hem', 'vt') and h['role'] in ('row', 'corner') and h['w'] > 3.8 and rng.random() < 0.22:
                fx = h['cx'] + math.sin(h['ang']) * 1.35
                fy = h['cy'] - math.cos(h['ang']) * 1.35
                if all(math.hypot(fx - px, fy - py) > h['w'] / 2 + 3 for px, py in clear):
                    lx, ly = sidewalk_cafe(g, rng, h['w'])
                    ca, sa = math.cos(h['ang']), math.sin(h['ang'])
                    CAFES.append((h['cx'] + ca * lx - sa * ly, h['cy'] + sa * lx + ca * ly))
            if got is not None:
                floors = got[0]
                sides = h['sides'] if kind != 'cl' else []
                wall_details(ns, g, rng, h['w'], h['D'], floors, sides, h['back'] and kind != 'cl')
            count += 1
        except Exception as e:
            log('house failed:', kind, type(e).__name__, e)
        del g.st[depth:]
    for kind, g in geos.items():
        ns['make_obj'](g, 'SH_' + kind, coll, sub=0, angle=30, recenter=True)
    # the yards inside the blocks: a lawn with a tree or two
    yard = Builder('SH_Yards', coll)
    tg = ns['Geo'](11)
    for q in PLAN['yards']:
        lawn = q.buffer(-0.2)
        if lawn.is_empty:
            continue
        yard.slab(lawn, WALK_Z - 0.02, WALK_Z + 0.05, 'grassDark', 'curb')
        inner = q.buffer(-1.6)
        minx, miny, maxx, maxy = q.bounds
        for _ in range(max(1, int(q.area / 45))):
            for _try in range(12):
                x, y = rng.uniform(minx, maxx), rng.uniform(miny, maxy)
                if shapely.contains_xy(inner, x, y):
                    street_tree(tg, ns, x, y, WALK_Z + 0.05, rng)
                    break
    yard.build()
    ns['make_obj'](tg, 'CX_YardTrees', coll, sub=0, angle=30, recenter=True)
    log('houses', count, 'corners', sum(1 for h in PLAN['houses'] if h['role'] == 'corner'), 'hẻm', len(PLAN['alleys']),
        'yards', len(PLAN['yards']))


# ---------------------------------------------------------------------------------------------------------------
# landmarks: the kept models moved whole, plain volumes for the new ones
# ---------------------------------------------------------------------------------------------------------------
DIRS = {'E': (1, 0), 'W': (-1, 0), 'N': (0, 1), 'S': (0, -1)}         # Blender compass
PLAN_DIRS = {'E': (1, 0), 'W': (-1, 0), 'N': (0, 1), 'S': (0, -1)}    # plan face letters mean the same compass


def append_objects(names):
    with bpy.data.libraries.load(SRC_BLEND, link=False) as (src, dst):
        dst.objects = [n for n in names if n in src.objects]
    return [o for o in dst.objects if o is not None]


def place_models(coll):
    wanted = [n for lm in L['landmarks'] for n in lm.get('models', [])]
    objs = {o.name: o for o in append_objects(wanted)}
    missing = [n for n in wanted if n not in objs]
    if missing:
        log('missing models', missing)
    for lm in L['landmarks']:
        group = [objs[n] for n in lm.get('models', []) if n in objs]
        if not group:
            continue
        for o in group:
            coll.objects.link(o)
        bpy.context.view_layer.update()     # matrix_world is stale until the objects are in the scene
        pts = []
        for o in group:
            if o.type == 'MESH':
                mw = o.matrix_world
                pts += [mw @ v.co for v in o.data.vertices]
        P = np.array([[p.x, p.y, p.z] for p in pts])
        lo, hi = P.min(0), P.max(0)
        c = (lo + hi) / 2
        if lm.get('srcFace') == 'auto':
            # the original builders put a landmark's facade on its local -Y: turn that by the object's rotation
            rz = group[0].rotation_euler.z
            fs = np.array([math.sin(rz), -math.cos(rz)])
        else:
            fs = np.array(DIRS[lm.get('srcFace', 'S')], float)
        ft = np.array(PLAN_DIRS[lm.get('face', 'S')], float)
        th = math.atan2(ft[1], ft[0]) - math.atan2(fs[1], fs[0])
        tx, ty = tb(lm['at'])
        M = Matrix.Translation((tx, ty, 0)) @ Matrix.Rotation(th, 4, 'Z') @ Matrix.Translation((-c[0], -c[1], 0))
        for o in group:
            if o.parent is None or o.parent not in group:
                o.matrix_world = M @ o.matrix_world
        log('placed', lm['id'], 'from', [round(v, 1) for v in c[:2]], 'turn', round(math.degrees(th)), 'size', [round(v, 1) for v in (hi - lo)[:2]])


def face_matrix(ns, cx, cy, d, face):
    """local frame of a builder (facade at y = 0 facing -Y) for a footprint centred at (cx, cy), `d` deep, facing `face`"""
    fx, fy = PLAN_DIRS[face]
    yaw = math.atan2(fy, fx) + math.pi / 2
    return ns['T'](cx + fx * d / 2, cy + fy * d / 2, WALK_Z) @ ns['Rm'](0, 0, yaw)


# ---------------------------------------------------------------------------------------------------------------
# the new landmarks as low-poly models (review 2026-11-13), in place of the blockout boxes. Each builds in the local
# frame of face_matrix: facade at y = 0 facing -Y, x across the facade (w wide), y into the building (d deep).
# ---------------------------------------------------------------------------------------------------------------
def _gable(g, wall, roof, x0, x1, y0, y1, zw, zr, ov=0.5, t=0.22, ends=True):
    """walls and a pitched roof with its ridge along y (eaves on the x sides)"""
    xm = (x0 + x1) / 2
    if wall:
        g.box(wall, x0, y0, 0, x1, y1, zw, ch=0)
    if ends and wall:
        for y in (y0, y1 - 0.25):
            g.extrude(wall, [(x0, zw), (x1, zw), (xm, zr)], y, y + 0.25)
    s = (zr - zw) / max(0.1, xm - x0)
    for xe in (x0 - ov, x1 + ov):
        ze = zw - ov * s
        g.extrude(roof, [(xe, ze), (xm, zr), (xm, zr + t), (xe, ze + t)], y0 - ov, y1 + ov)


def _windows(g, xs, z0, z1, y, frame='opWhite', glass='glassDark', wd=1.1):
    for x in xs:
        g.box(frame, x - wd / 2 - 0.12, y - 0.1, z0 - 0.12, x + wd / 2 + 0.12, y, z1 + 0.12, ch=0)
        g.box(glass, x - wd / 2, y - 0.14, z0, x + wd / 2, y - 0.08, z1, ch=0)


def lm_ubnd(ns, g, rng, w, d):
    """UBND (Hôtel de Ville, 1909): cream walls and white trim, end pavilions and a central pavilion with a clock
    tower, slate roofs, an arcade on the ground floor"""
    T, Rm = ns['T'], ns['Rm']
    hw = w / 2
    W, TR, RF = 'cream', 'opWhite', 'slate'
    g.box('stone', -hw - 0.4, -1.4, 0, hw + 0.4, d + 0.3, 0.5, ch=0)
    g.box(W, -hw + 4.2, 1.0, 0.5, hw - 4.2, d - 1.0, 9.0, ch=0)
    g.hip(RF, -hw + 4.0, 0.8, hw - 4.0, d - 0.8, 9.0, 2.6)
    for sx in (-1, 1):
        x0, x1 = sorted((sx * hw, sx * (hw - 4.4)))
        g.box(W, x0, 0.0, 0.5, x1, d, 10.0, ch=0)
        g.box(TR, x0 - 0.15, -0.15, 9.6, x1 + 0.15, d + 0.15, 10.1, ch=0)
        g.hip(RF, x0 + 0.1, 0.1, x1 - 0.1, d - 0.1, 10.1, 3.4)
        xc = (x0 + x1) / 2
        _windows(g, (xc - 1.0, xc + 1.0), 1.6, 3.8, 0.0, wd=0.8)
        _windows(g, (xc - 1.0, xc + 1.0), 5.6, 8.2, 0.0, wd=0.8)
    g.box(TR, -hw + 4.2, 0.85, 4.6, hw - 4.2, 1.05, 4.95, ch=0)
    g.box(TR, -hw + 4.2, 0.85, 8.7, hw - 4.2, 1.1, 9.0, ch=0)
    x = -hw + 5.4
    while x < hw - 5.2:
        if abs(x) > 4.2:
            g.arch('archDark', x, 1.3, 0.5, 2.5, 0.88, 1.0, 6)
            _windows(g, (x,), 5.5, 7.9, 1.0, wd=0.9)
        x += 2.0
    g.box(W, -3.8, -0.8, 0.5, 3.8, 6.0, 11.2, ch=0)        # central pavilion
    g.box(TR, -4.0, -1.0, 10.8, 4.0, 6.2, 11.2, ch=0)
    g.box(TR, -4.0, -1.9, 4.6, 4.0, -0.8, 4.95, ch=0)      # balcony over the door
    for x in (-3.3, -1.1, 1.1, 3.3):
        g.cyl(TR, 0.26, 3.8, 8, T(x, -1.5, 4.95))
    g.box(TR, -4.0, -1.9, 8.75, 4.0, -0.8, 9.1, ch=0)
    g.arch('archDark', 0, 2.4, 0.5, 3.0, -0.9, -0.78, 8)
    _windows(g, (-1.6, 1.6), 5.6, 8.2, -0.8, wd=1.0)
    g.box(W, -2.1, 0.6, 11.2, 2.1, 4.8, 16.2, ch=0)        # clock tower
    g.box(TR, -2.3, 0.4, 15.8, 2.3, 5.0, 16.3, ch=0)
    g.cyl('clockWhite', 1.05, 0.14, 18, T(0, 0.6, 13.8) @ Rm(math.pi / 2, 0, 0))
    g.box('archDark', -0.06, 0.4, 13.8, 0.06, 0.45, 14.6, ch=0)
    g.box('archDark', -0.06, 0.4, 13.74, 0.55, 0.45, 13.86, ch=0)
    g.pyramid(RF, 0, 2.7, 16.3, 2.4, 2.4, 3.6)
    g.rod('gold', (0, 2.7, 19.7), (0, 2.7, 22.4), 0.07, 6)
    g.box('signRed', 0.05, 2.65, 21.2, 1.6, 2.75, 22.2, ch=0)   # the flag
    g.box('gold', -0.15, 2.6, 22.0, 0.15, 2.8, 22.3, ch=0)
    g.text('UBND THÀNH PHỐ', 0.42, 'gold', T(0, -0.86, 10.0) @ ns['FACE_S'], extrude=0.03, max_w=7.0)


def lm_chung_cu_42(ns, g, rng, w, d):
    """Chung cư 42 Nguyễn Huệ (1960s): an old nine-storey block whose flats are cafés and shops, each balcony its
    own colour and sign, laundry and AC units between, a stair tower at one end"""
    T = ns['T']
    hw = w / 2
    F, GF, n = 3.1, 4.0, 8
    H = GF + F * n
    g.box('wallGrey', -hw, 1.4, 0, hw - 3.0, d, H, ch=0)
    g.box('concrete', hw - 3.0, 0.0, 0, hw, d, H + 2.0, ch=0)            # stair tower
    for k in range(int(H / 2.2)):
        g.box('glassDark', hw - 2.2, -0.05, 1.2 + k * 2.2, hw - 0.8, 0.0, 2.2 + k * 2.2, ch=0)
    xs = [-hw + 0.3 + i * (w - 3.3) / 5 for i in range(6)]
    for i in range(5):                                                  # ground floor shops
        x0, x1 = xs[i], xs[i + 1]
        g.box('shopDark', x0 + 0.2, 1.3, 0.15, x1 - 0.2, 1.4, GF - 0.6, ch=0)
        bg, fg = rng.choice(SIGN_COLOURS)
        g.box(bg, x0 + 0.1, -0.1, GF - 0.9, x1 - 0.1, 0.1, GF - 0.2, ch=0)
        g.text(rng.choice(['CÀ PHÊ', 'TRÀ SỮA', 'SÁCH', 'THỜI TRANG', 'BÁNH']), 0.3, fg, T((x0 + x1) / 2, -0.12, GF - 0.55) @ ns['FACE_S'],
               extrude=0.02, max_w=(x1 - x0) - 0.6)
    g.box('concrete', -hw, 0.0, GF - 0.2, hw - 3.0, 1.4, GF, ch=0)
    cafe = ['signRed', 'poYellow', 'wallTeal', 'wallPink', 'wallMint', 'clothBlue', 'wallOchre', 'creamLight', 'signGold']
    for f in range(n):
        z0 = GF + f * F
        g.box('stone', -hw, -1.3, z0 - 0.15, hw - 3.0, 1.4, z0, ch=0)      # balcony slab
        for i in range(5):
            x0, x1 = xs[i], xs[i + 1]
            col = rng.choice(cafe)
            g.box(col, x0 + 0.08, 1.3, z0, x1 - 0.08, 1.4, z0 + F - 0.15, ch=0)
            g.box('glassDark', x0 + 0.6, 1.25, z0 + 0.3, x1 - 0.6, 1.32, z0 + 2.3, ch=0)
            g.box('creamLight', x0, -1.3, z0, x0 + 0.12, 1.4, z0 + F - 0.15, ch=0)
            if rng.random() < 0.55:
                bg, fg = rng.choice(SIGN_COLOURS)
                g.box(bg, x0 + 0.3, -1.38, z0 + 0.95, x1 - 0.3, -1.28, z0 + 1.55, ch=0)
            else:
                g.box('wallGrey', x0 + 0.1, -1.36, z0, x1 - 0.1, -1.28, z0 + 1.0, ch=0)
            if rng.random() < 0.6:
                g.ico(rng.choice(['leaf', 'leafLight', 'flowerPink']), 0.35, T(x0 + 0.6, -1.0, z0 + 1.3), sub=0, jit=0.1)
            if rng.random() < 0.4:
                g.box('acWhite', x1 - 1.1, -0.2, z0 + 2.0, x1 - 0.3, 0.2, z0 + 2.6, ch=0)
    for f in range(n + 1):                                   # the back and the west end: windows, AC units, laundry
        z0 = GF + (f - 1) * F if f else 0.6
        x = -hw + 1.0
        while x < hw - 3.6:
            g.box('creamLight', x, d, z0 + 0.7, x + 1.3, d + 0.08, z0 + 2.1, ch=0)
            g.box('glassDark', x + 0.15, d + 0.02, z0 + 0.85, x + 1.15, d + 0.11, z0 + 1.95, ch=0)
            if f and rng.random() < 0.3:
                g.box('acWhite', x + 1.5, d, z0 + 0.4, x + 2.3, d + 0.42, z0 + 0.95, ch=0)
            if f and rng.random() < 0.2:
                g.box(rng.choice(['clothRed', 'clothBlue', 'white']), x, d + 0.3, z0 + 1.7, x + 1.3, d + 0.4, z0 + 2.3, ch=0)
            x += 2.6
        for y in (2.5, 6.0, 9.5):
            g.box('creamLight', -hw - 0.08, y, z0 + 0.7, -hw, y + 1.3, z0 + 2.1, ch=0)
            g.box('glassDark', -hw - 0.11, y + 0.15, z0 + 0.85, -hw - 0.02, y + 1.15, z0 + 1.95, ch=0)
    g.box('creamLight', -hw - 0.1, -1.4, H - 0.1, hw - 3.0, d + 0.1, H + 0.6, ch=0)
    g.text('42', 0.9, 'signRed', T(hw - 1.5, -0.08, H + 0.9) @ ns['FACE_S'], extrude=0.05)
    for k in range(3):
        g.cyl('white', 0.6, 1.3, 10, T(-hw + 3 + k * 5, d * 0.6, H + 0.6))


def lm_ben_bach_dang(ns, g, rng, w, d):
    """Bến Bạch Đằng: a long tiled pavilion on slender posts along the riverside, benches under it"""
    T = ns['T']
    hw = w / 2
    g.box('pavingDark', -hw, 0, 0, hw, d, 0.12, ch=0)
    y = 1.2
    while y < d - 0.8:
        for x in (-hw + 0.8, hw - 0.8):
            g.cyl('wood', 0.16, 3.2, 8, T(x, y, 0.12))
        g.box('wood', -hw + 1.4, y + 0.6, 0.12, -hw + 3.0, y + 1.0, 0.55, ch=0)
        y += 3.6
    _gable(g, None, 'roofRed', -hw + 0.6, hw - 0.6, 0.8, d - 0.8, 3.3, 5.0, ov=0.6)
    g.box('wood', -hw + 0.6, 0.6, 3.2, hw - 0.6, d - 0.6, 3.35, ch=0)
    g.text('BẾN BẠCH ĐẰNG', 0.45, 'signGold', T(0, 0.4, 3.8) @ ns['FACE_S'], extrude=0.03, max_w=w - 1.4)


def lm_ben_nha_rong(ns, g, rng, w, d):
    """Bến Nhà Rồng (1863): a two-storey colonial house, red tiled roof with two dragons and the moon on the ridge,
    a small front tower"""
    T = ns['T']
    hw = w / 2
    W, TR = 'creamLight', 'opWhite'
    g.box('stone', -hw - 0.3, -0.6, 0, hw + 0.3, d * 0.7 + 0.3, 0.5, ch=0)
    D = d * 0.7
    g.box(W, -hw, 0.6, 0.5, hw, D, 8.0, ch=0)
    g.box(TR, -hw - 0.15, 0.45, 4.1, hw + 0.15, D + 0.15, 4.4, ch=0)
    g.box(TR, -hw - 0.2, 0.4, 7.8, hw + 0.2, D + 0.2, 8.1, ch=0)
    x = -hw + 1.4
    while x < hw - 1.0:
        if abs(x) > 2.4:
            g.arch('archDark', x, 1.0, 0.5, 2.4, 0.5, 0.6, 6)
            _windows(g, (x,), 5.0, 7.0, 0.6, wd=0.8)
        x += 2.2
    g.box(W, -2.2, -0.4, 0.5, 2.2, 3.0, 10.0, ch=0)        # front tower
    g.arch('archDark', 0, 1.6, 0.5, 2.6, -0.5, -0.38, 8)
    _windows(g, (0,), 5.2, 7.4, -0.4, wd=1.0)
    g.pyramid('roofRed', 0, 1.3, 10.0, 2.6, 2.3, 2.4)
    g.box(TR, -2.4, -0.6, 9.7, 2.4, 3.2, 10.0, ch=0)
    g.hip('roofRed', -hw - 0.5, 0.1, hw + 0.5, D + 0.5, 8.1, 3.2)
    zr = 8.1 + 3.2
    ry = (0.1 + D + 0.5) / 2
    for sx in (-1, 1):                                     # the two dragons, facing the moon in the middle
        x0 = sx * 1.0
        for k in range(5):
            xx = x0 + sx * k * 0.55
            g.box('poGreen', xx - 0.3, ry - 0.2, zr + 0.1 + 0.25 * math.sin(k * 1.4), xx + 0.3, ry + 0.2, zr + 0.55 + 0.25 * math.sin(k * 1.4), ch=0)
        g.box('gold', x0 - 0.25, ry - 0.25, zr + 0.4, x0 + 0.25, ry + 0.25, zr + 0.95, ch=0)
    g.ico('gold', 0.42, T(0, ry, zr + 0.6), sub=1)
    g.rod('metal', (hw - 1.0, -0.2, 0.5), (hw - 1.0, -0.2, 13.0), 0.06, 6)
    g.box('signRed', hw - 0.95, -0.25, 11.6, hw + 0.9, -0.15, 12.8, ch=0)
    g.box('gold', hw - 0.2, -0.27, 12.0, hw + 0.1, -0.13, 12.3, ch=0)


def lm_chua_ba(ns, g, rng, w, d):
    """Chùa Bà Thiên Hậu (Chợ Lớn, 1760): a low Cantonese temple, front hall and main hall around a small courtyard,
    green glazed roofs with figures on the ridges, red doors, lanterns and incense coils"""
    T = ns['T']
    hw = w / 2
    g.box('stone', -hw, 0, 0, hw, d, 0.35, ch=0)
    def hall(y0, y1, zw, zr):
        g.box('wallOchre', -hw + 0.5, y0, 0.35, hw - 0.5, y1, zw, ch=0)
        g.push(T(0, 0, 0) @ ns['Rm'](0, 0, math.pi / 2))
        _gable(g, None, 'poGreen', y0 - 0.2, y1 + 0.2, -hw + 0.5, hw - 0.5, zw, zr, ov=0.7, t=0.25)
        g.pop()
        g.box('roofRedDark', -hw + 0.2, (y0 + y1) / 2 - 0.25, zr, hw - 0.2, (y0 + y1) / 2 + 0.25, zr + 0.5, ch=0)
        for sx in (-1, 1):                                 # the ridge ends curl up
            g.box('roofRedDark', sx * (hw - 0.2) - 0.35, (y0 + y1) / 2 - 0.25, zr + 0.4, sx * (hw - 0.2) + 0.35, (y0 + y1) / 2 + 0.25, zr + 1.2, ch=0)
        for k in range(5):                                 # figures on the ridge
            x = -hw + 2.5 + k * (w - 5) / 4
            g.box(rng.choice(['gold', 'wallTeal', 'signRed', 'clothYellow']), x - 0.22, (y0 + y1) / 2 - 0.15, zr + 0.5, x + 0.22, (y0 + y1) / 2 + 0.15, zr + 1.05, ch=0)
    hall(1.2, 6.2, 4.4, 6.4)
    hall(9.6, d - 0.4, 5.2, 7.6)
    g.box('stone', -hw + 0.5, 6.2, 0.35, -hw + 1.5, 9.6, 3.0, ch=0)
    g.box('stone', hw - 1.5, 6.2, 0.35, hw - 0.5, 9.6, 3.0, ch=0)
    g.cyl('brick', 0.7, 1.0, 10, T(0, 7.9, 0.35))            # incense burner in the courtyard
    g.cyl('gold', 0.5, 0.3, 10, T(0, 7.9, 1.35))
    for x in (-3.4, 0.0, 3.4):                                # three red doors, gold signboard
        g.box('signRed', x - 0.9, 1.1, 0.35, x + 0.9, 1.2, 3.1, ch=0)
    g.box('archDark', -2.2, 1.0, 3.3, 2.2, 1.1, 4.1, ch=0)
    g.text('THIÊN HẬU', 0.42, 'gold', T(0, 0.98, 3.7) @ ns['FACE_S'], extrude=0.03, max_w=4.0)
    for x in (-hw + 1.4, -1.8, 1.8, hw - 1.4):
        g.cyl('signRed', 0.12, 3.6, 8, T(x, 0.6, 0.35))
    for x in (-5.0, -2.0, 2.0, 5.0):
        ns['lantern'](g, T(x, 0.3, 3.6))
    for k in range(3):                                        # incense coils hanging in the courtyard
        g.cone('wallOchre', 0.5, 0.9, 8, T(-2.2 + k * 2.2, 7.9, 2.8))


def lm_ben_xe(ns, g, rng, w, d):
    """Bến xe Chợ Lớn: a ticket office at one end, the platforms each under its own pitched tin canopy on a row of
    steel posts (review r9: one flat lid over all of them read as a warehouse), bus bays painted on the apron"""
    T, Rm = ns['T'], ns['Rm']
    hw = w / 2
    g.box('pavingDark', -hw, 0, 0, hw, d, 0.1, ch=0)
    g.box('wallCream', -hw, 3.0, 0.1, -hw + 8.0, d, 7.0, ch=0)        # office
    g.box('creamLight', -hw - 0.15, 2.85, 6.8, -hw + 8.15, d + 0.15, 7.3, ch=0)
    for z in (1.0, 4.2):
        _windows(g, (-hw + 2.0, -hw + 4.0, -hw + 6.0), z, z + 1.7, 3.0, frame='creamLight', glass='glassBlue', wd=1.2)
    g.box('clothBlue', -hw + 0.4, 2.8, 2.9, -hw + 7.6, 3.0, 3.8, ch=0)
    g.text('BẾN XE CHỢ LỚN', 0.38, 'white', T(-hw + 4.0, 2.78, 3.35) @ ns['FACE_S'], extrude=0.02, max_w=6.8)
    x0 = -hw + 9.0
    for k in range(4):                                         # platforms under the canopy
        y = 3.0 + k * (d - 4.0) / 4
        g.box('paving', x0, y, 0.1, hw - 0.5, y + 1.2, 0.3, ch=0)
        for x in (x0 + 0.5, (x0 + hw) / 2, hw - 1.0):
            g.cyl('metal', 0.12, 4.4, 8, T(x, y + 0.6, 0.3))
        # the canopy, its ridge along the platform (the builder's ridge runs along y: turned a quarter round)
        g.push(Rm(0, 0, math.pi / 2))
        _gable(g, None, 'tinBlue', y - 1.0, y + 2.2, -(hw + 0.3), -(x0 - 0.3), 4.55, 5.35, ov=0.2, t=0.12, ends=False)
        g.pop()
        g.box('tinWhite', x0 - 0.5, y + 0.5, 4.45, hw + 0.5, y + 0.7, 4.75, ch=0)
        g.box('wood', x0 + 2.0, y + 0.3, 0.3, x0 + 4.0, y + 0.9, 0.75, ch=0)
        bx = x0 + 1.0
        while bx < hw - 3.0:                                   # bus bays
            g.box('roadLine', bx, y + 1.4, 0.1, bx + 0.15, y + 1.4 + (d - 4.0) / 4 - 1.6, 0.115, ch=0)
            bx += 4.5


def lm_sang_tao(ns, g, rng, w, d):
    """the stage of Công viên Sáng tạo: a white roof slab on four columns over a raised platform, a teal back wall"""
    T = ns['T']
    hw = w / 2
    g.box('stone', -hw, 0, 0, hw, d, 0.9, ch=0)
    for x in (-hw + 0.5, hw - 0.5):
        for y in (0.6, d - 0.6):
            g.cyl('white', 0.2, 4.2, 8, T(x, y, 0.9))
    g.box('white', -hw - 0.6, -0.6, 5.1, hw + 0.6, d + 0.6, 5.5, ch=0)
    g.box('wallTeal', -hw + 0.3, d - 0.5, 0.9, hw - 0.3, d - 0.2, 4.6, ch=0)
    g.text('SÁNG TẠO', 0.6, 'white', T(0, d - 0.55, 3.4) @ ns['FACE_S'], extrude=0.03, max_w=w - 2)
    for k in range(3):
        g.box('signGold', -hw + 1.0, -1.2 - k * 0.45, 0, hw - 1.0, -0.75 - k * 0.45, 0.9 - k * 0.3, ch=0)


def lm_nha_tho_tt(ns, g, rng, w, d):
    """Nhà thờ Thủ Thiêm (1859): a small white church, red tiled nave, a bell tower over the door with a cross"""
    T = ns['T']
    hw = w / 2
    g.box('stone', -hw, 0, 0, hw, d, 0.3, ch=0)
    _gable(g, 'opWhite', 'roofRed', -hw + 0.5, hw - 0.5, 3.0, d - 0.3, 6.5, 9.5, ov=0.5)
    for y in (6.0, 9.5, 13.0):
        for x in (-hw + 0.45, hw - 0.45):
            g.arch('glassBlue', 0, 0.9, 2.6, 4.2, -0.06, 0.06, 6, T(x, y, 0) @ ns['Rm'](0, 0, math.pi / 2))
    g.box('opWhite', -2.2, 0.4, 0.3, 2.2, 3.4, 12.0, ch=0)
    g.arch('wood', 0, 1.6, 0.3, 2.2, 0.28, 0.4, 8)
    g.cyl('glassBlue', 0.7, 0.1, 12, T(0, 0.38, 6.2) @ ns['Rm'](math.pi / 2, 0, 0))
    g.box('archDark', -0.8, 0.35, 9.4, 0.8, 0.45, 11.0, ch=0)
    g.pyramid('roofRed', 0, 1.9, 12.0, 2.4, 1.7, 3.0)
    g.box('gold', -0.07, 1.85, 15.0, 0.07, 1.95, 16.8, ch=0)
    g.box('gold', -0.5, 1.85, 16.0, 0.5, 1.95, 16.15, ch=0)


def lm_ben_buyt_song(ns, g, rng, w, d):
    """the river bus stop: a shelter on the bank, a ramp down to the pontoon"""
    T = ns['T']
    hw = w / 2
    g.box('wood', -hw, 0, 0, hw, d, 0.15, ch=0)
    for x in (-hw + 0.3, hw - 0.3):
        for y in (0.4, d / 2, d - 0.4):
            g.cyl('metal', 0.08, 2.6, 6, T(x, y, 0.15))
    g.box('tinBlue', -hw - 0.3, -0.3, 2.75, hw + 0.3, d + 0.3, 2.95, ch=0)
    g.box('wood', -hw + 0.4, 1.0, 0.15, -hw + 1.0, d - 1.0, 0.6, ch=0)
    g.box('clothBlue', -1.6, -0.35, 2.3, 1.6, -0.25, 2.75, ch=0)
    g.text('BUÝT SÔNG', 0.3, 'white', T(0, -0.37, 2.52) @ ns['FACE_S'], extrude=0.02, max_w=3.0)


LM_OPEN = {'ben_bach_dang', 'ben_xe', 'cong_vien_sang_tao', 'ben_buyt_song'}     # open structures: walk under them
LM_MODELS = {'ubnd': lm_ubnd, 'chung_cu_42': lm_chung_cu_42, 'ben_bach_dang': lm_ben_bach_dang, 'ben_nha_rong': lm_ben_nha_rong,
             'cho_lon': lm_chua_ba, 'ben_xe': lm_ben_xe, 'cong_vien_sang_tao': lm_sang_tao, 'nha_tho_thu_thiem': lm_nha_tho_tt,
             'ben_buyt_song': lm_ben_buyt_song}


def build_new_landmarks(coll):
    ns = load_builders()
    rng = random.Random(42)
    gm = ns['Geo'](42)
    gp = ns['Geo'](43)
    b = Builder('LM_Blockout', coll)
    for lm in L['landmarks']:
        if lm.get('models'):
            continue
        if lm['id'] in LM_MODELS and 'size' in lm and 'at' in lm:
            cx, cy = tb(lm['at'])
            face = lm.get('face', 'S')
            w, d = (lm['size'][1], lm['size'][0]) if face in ('E', 'W') else lm['size']
            g_ = gp if lm['id'] in LM_OPEN else gm
            g_.push(face_matrix(ns, cx, cy, d, face))
            LM_MODELS[lm['id']](ns, g_, rng, w, d)
            g_.pop()
            continue
        if 'size' in lm and 'at' in lm and lm['id'] not in MODELLED_LANDMARKS:
            cx, cy = tb(lm['at'])
            w, d = lm['size']
            h = lm.get('h', 8)
            b.box(cx - w / 2, cy - d / 2, WALK_Z, cx + w / 2, cy + d / 2, WALK_Z + h, lm.get('color', 'concrete'), lm.get('roof', 'concrete'))
            fx, fy = PLAN_DIRS[lm.get('face', 'S')]
            # a dark band on the facade so the front reads
            if fx:
                x = cx + fx * (w / 2 + 0.05)
                b.box(x - 0.06, cy - d * 0.3, WALK_Z + 0.2, x + 0.06, cy + d * 0.3, WALK_Z + min(3.2, h * 0.4), 'archDark')
            else:
                y = cy + fy * (d / 2 + 0.05)
                b.box(cx - w * 0.3, y - 0.06, WALK_Z + 0.2, cx + w * 0.3, y + 0.06, WALK_Z + min(3.2, h * 0.4), 'archDark')
            if lm['id'] == 'chung_cu_42':
                for f in range(1, int(h / 3.3)):
                    y = cy + d / 2 + 0.6
                    b.box(cx - w / 2 + 0.3, cy + d / 2, WALK_Z + f * 3.3 - 0.1, cx + w / 2 - 3, y, WALK_Z + f * 3.3 + 0.05, 'stone')
                    b.box(cx - w / 2 + 0.3, y - 0.05, WALK_Z + f * 3.3, cx + w / 2 - 3, y, WALK_Z + f * 3.3 + 1.05, 'archDark')
        if lm.get('statueChrist') and lm['id'] not in MODELLED_LANDMARKS:
            X, Y = tb(lm['at'])
            z0 = terrain_z((X, Y))
            b.box(X - 3.2, Y - 3.2, z0, X + 3.2, Y + 3.2, z0 + 4.0, 'stone')
            b.cylinder(X, Y, z0 + 4.0, 1.4, 8.6, 'statueW', n=8, r_top=0.8)
            b.box(X - 3.6, Y - 0.45, z0 + 11.0, X + 3.6, Y + 0.45, z0 + 11.8, 'statueW')
            b.cylinder(X, Y, z0 + 12.6, 0.75, 1.3, 'statueW', n=8, r_top=0.55)
        if lm.get('lighthouse') and lm['id'] not in MODELLED_LANDMARKS:
            X, Y = tb(lm['at'])
            z0 = terrain_z((X, Y))
            b.cylinder(X, Y, z0, 2.2, 11.0, 'opWhite', n=10, r_top=1.7)
            b.cylinder(X, Y, z0 + 11.0, 2.5, 0.4, 'archDark', n=10)
            b.cylinder(X, Y, z0 + 11.4, 1.3, 1.8, 'lampGlow', n=10)
            b.cylinder(X, Y, z0 + 13.2, 1.7, 1.6, 'roofRed', n=10, r_top=0.0)
        if lm['id'] == 'hon_ba':
            X, Y = tb(lm['at'])
            b.cylinder(X, Y, -3.0, 8.0, 4.4, 'rock', n=7, r_top=4.5)
            b.box(X - 1.5, Y - 1.5, 1.4, X + 1.5, Y + 1.5, 3.6, 'roofRed', 'roofRedDark')
            # the low-tide path to the shore (layout 'tideTo'; north to Bãi Sau before v7)
            TX, TY = tb(lm['tideTo']) if 'tideTo' in lm else (X, Y + 26.0)
            dl = math.hypot(TX - X, TY - Y) or 1.0
            ux, uy = (TX - X) / dl, (TY - Y) / dl
            b.oriented_box(X + ux * (7.0 + dl) / 2, Y + uy * (7.0 + dl) / 2, -0.65, dl - 7.0, 2.4, 0.25, math.atan2(uy, ux), 'sand')
        # Mũi Nghinh Phong (#27) has no block: the cape becomes natural rocks with Núi Nhỏ (docs/map-v5-tasks.md, T2, T3)
        if lm.get('statue'):
            X, Y = tb(lm['at'])
            b.cylinder(X, Y, WALK_Z + 0.3, 1.0, 2.0, 'stone', n=8)
            b.cylinder(X, Y, WALK_Z + 2.3, 0.45, 2.2, 'statueW', n=8, r_top=0.3)
    for x0, y0, x1, y1, h in L['towers']:
        cx, cy = tb(((x0 + x1) / 2, (y0 + y1) / 2))
        w, d = abs(x1 - x0), abs(y1 - y0)
        gm.push(face_matrix(ns, cx, cy, d, 'W'))
        ns['tt_tower'](gm, w, d, max(6, int(h / 3.3)), rng)
        gm.pop()
    b.build()
    ns['make_obj'](gm, 'LM_NewBuildings', coll, sub=0, angle=30, recenter=True)
    ns['make_obj'](gp, 'LM_Pavilions', coll, sub=0, angle=30, recenter=True)


# ---------------------------------------------------------------------------------------------------------------
# nature and props
# ---------------------------------------------------------------------------------------------------------------
def street_tree(g, ns, x, y, z, rng):
    """tree_sao (the tall Saigon street tree) with lighter crowns: icospheres at one subdivision instead of two"""
    T = ns['T']
    h = rng.uniform(6.0, 8.0)
    g.cyl('bark', 0.3, h, 8, T(x, y, z), r2=0.2)
    for _ in range(3):
        a = rng.uniform(0, 2 * math.pi)
        g.rod('bark', (x, y, z + h - 1.2), (x + math.cos(a) * 1.4, y + math.sin(a) * 1.4, z + h + rng.uniform(0.8, 1.6)), 0.12, 6)
    for _ in range(rng.randint(5, 7)):
        a = rng.uniform(0, 2 * math.pi)
        r = rng.uniform(0.3, 1.8)
        g.ico(rng.choice(['leaf', 'leafDark', 'leafLight', 'leaf']), rng.uniform(1.3, 1.9),
              T(x + math.cos(a) * r, y + math.sin(a) * r, z + h + 1.4 + rng.uniform(-0.3, 1.4)), sub=1, jit=0.14, sc=(1, 1, 0.78))


def forest_tree(g, ns, x, y, z, rng, s=1.0):
    T = ns['T']
    h = rng.uniform(1.6, 2.6) * s
    g.cyl('bark', 0.16 * s, h, 6, T(x, y, z - 0.2), r2=0.1 * s)
    for _ in range(rng.randint(2, 3)):
        g.ico(rng.choice(['leafDark', 'leaf', 'leafDark', 'leafLight']), rng.uniform(1.0, 1.5) * s,
              T(x + rng.uniform(-0.7, 0.7) * s, y + rng.uniform(-0.7, 0.7) * s, z + h + rng.uniform(0.2, 1.0) * s), sub=1, jit=0.15, sc=(1, 1, 0.85))


def placeable():
    blocked = lm_keepout.union(river.buffer(2)).union(unary_union([ln.buffer(b_['w'] / 2 + 2) for ln, b_ in bridge_lines]))
    land_in = land.buffer(-1)
    houses = HOUSE_FOOT.buffer(0.5).union(ALLEYS)
    blocked = blocked.union(unary_union([Point(c).buffer(1.7) for c in CAFES] or [Polygon()]))
    for g_ in (blocked, land_in, roads, houses):
        shapely.prepare(g_)
    return lambda x, y, lots_ok=False: (shapely.contains_xy(land_in, x, y) and not shapely.contains_xy(blocked, x, y)
                                        and not shapely.contains_xy(roads, x, y) and (lots_ok or not shapely.contains_xy(houses, x, y)))


def road_points(ln, step, start=0.0):
    """(point, unit tangent) every `step` metres along a line"""
    out = []
    t = start
    while t < ln.length:
        p = ln.interpolate(t)
        q = ln.interpolate(min(ln.length, t + 0.5))
        o = ln.interpolate(max(0.0, t - 0.5))
        dx, dy = q.x - o.x, q.y - o.y
        dl = math.hypot(dx, dy) or 1.0
        out.append((p.x, p.y, dx / dl, dy / dl))
        t += step
    return out


def build_nature(coll):
    ns = load_builders()
    T, Sc = ns['T'], ns['Sc']
    rng = random.Random(7)
    g = ns['Geo'](7)
    ok = placeable()
    placed = 0
    for ln, w in road_geo:                    # street trees: every 13 m on the main streets, 21 m on the smaller ones
        if w < 7:
            continue
        for x, y, dx, dy in road_points(ln, 13.0 if w >= 10 else 21.0, 5.0):
            for sgn in (1, -1):
                X, Y = x - dy * sgn * (w / 2 + 1.6), y + dx * sgn * (w / 2 + 1.6)
                if ok(X, Y, lots_ok=True):
                    street_tree(g, ns, X, Y, WALK_Z, rng)
                    placed += 1
    flowers = Builder('CX_Flowers', coll)
    for pk in L['parks']:                     # flower gardens (as the Tết flower street on Nguyễn Huệ): raised planters
        if pk.get('kind') != 'flowers':       # of one colour each, on paving, with walks between and across them
            continue
        poly = rect_poly(pk['rect']).intersection(land.buffer(-0.5)).difference(roads)
        minx, miny, maxx, maxy = poly.bounds
        along_y = (maxy - miny) > (maxx - minx)
        flowers.slab(poly.buffer(-0.2), -0.1, GROUND_Z + 0.08, 'paving')
        span = (maxx - minx) if along_y else (maxy - miny)
        length = (maxy - miny) if along_y else (maxx - minx)
        BED, WALK = 1.7, 1.4
        n = max(1, int((span - WALK) // (BED + WALK)))
        k0 = (span - n * BED - (n - 1) * WALK) / 2
        m = max(1, int(round((length - 2 * WALK) / 7.5)))
        seg = (length - 2 * WALK - (m - 1) * WALK) / m
        for a in range(n):
            col = ['flowerRed', 'flowerYellow', 'flowerPink', 'flowerYellow'][a % 4]
            for c in range(m):
                k1 = k0 + a * (BED + WALK)
                t1 = WALK + c * (seg + WALK)
                if along_y:
                    bed = sbox(minx + k1, miny + t1, minx + k1 + BED, miny + t1 + seg)
                else:
                    bed = sbox(minx + t1, miny + k1, minx + t1 + seg, miny + k1 + BED)
                if not poly.buffer(-0.3).contains(bed):
                    continue
                # a planter 0.6 m high (more than a step: the flowers are not walked on), earth inside, flowers on it
                flowers.slab(bed, GROUND_Z, GROUND_Z + 0.6, 'curb')
                flowers.slab(bed.buffer(-0.14, join_style=2), GROUND_Z + 0.3, GROUND_Z + 0.7, col, side='leaf')
                bx0, by0, bx1, by1 = bed.bounds
                yy = by0 + 0.4
                while yy < by1 - 0.3:
                    xx = bx0 + 0.4 + (0.27 if int((yy - by0) / 0.55) % 2 else 0.0)
                    while xx < bx1 - 0.3:
                        g.ico(col if rng.random() < 0.8 else 'leafLight', 0.2, T(xx + rng.uniform(-0.05, 0.05), yy + rng.uniform(-0.05, 0.05), GROUND_Z + 0.74),
                              sub=0, jit=0.12, sc=(1, 1, 0.6))
                        xx += 0.55
                    yy += 0.55
    flowers.build()
    for pk in L['parks']:                     # parks: tamarind trees, a few tall ones
        if pk.get('kind') == 'flowers':
            continue
        poly = rect_poly(pk['rect'])
        minx, miny, maxx, maxy = poly.bounds
        for _ in range(int(poly.area / (55 if pk.get('kind') == 'trees' else 170))):
            x, y = rng.uniform(minx, maxx), rng.uniform(miny, maxy)
            if poly.contains(Point(x, y)) and ok(x, y, lots_ok=True):
                if rng.random() < 0.6:
                    ns['tree_me'](g, T(x, y, ground_z(x, y)), rng)
                else:
                    street_tree(g, ns, x, y, ground_z(x, y), rng)
                placed += 1
    for q in ISLANDS:                         # traffic islands: a tree in the lawn
        c = q.buffer(-1.0).representative_point()
        street_tree(g, ns, c.x, c.y, WALK_Z + 0.08, rng)
        placed += 1
    for q in MEDIANS:                         # medians: a tree every 12 m along the lawn
        inner = q.buffer(-0.8)
        if inner.is_empty:
            continue
        minx, miny, maxx, maxy = inner.bounds
        for _ in range(max(1, int(q.area / 60))):
            for _try in range(10):
                x, y = rng.uniform(minx, maxx), rng.uniform(miny, maxy)
                if shapely.contains_xy(inner, x, y):
                    street_tree(g, ns, x, y, WALK_Z + 0.08, rng)
                    placed += 1
                    break
    for b, band in beach_polys:               # palms along the top of the beaches
        line = LineString(path_pts(b['path'], 0.8))
        for x, y, dx, dy in road_points(line, 8.5, 3.0):
            for off in (b['width'] / 2 + 2.0, b['width'] / 2 - 1.0):
                X, Y = x - dy * off, y + dx * off
                if not coast.contains(Point(X, Y)):
                    X, Y = x + dy * off, y - dx * off
                if coast.buffer(-1).contains(Point(X, Y)) and ok(X, Y) and rng.random() < 0.7:
                    ns['cycad'](g, T(X, Y, ground_z(X, Y)) @ Sc(1.7), rng)
                    placed += 1
    for mt, outline in mountains:             # a light forest on the hills
        if mt['id'] in MODELLED_MOUNTAINS:
            continue
        minx, miny, maxx, maxy = outline.bounds
        hmax = max(p[2] for p in mt['peaks'])
        for _ in range(int(outline.area / 18)):
            x, y = rng.uniform(minx, maxx), rng.uniform(miny, maxy)
            if not outline.buffer(-3).contains(Point(x, y)) or shapely.contains_xy(roads, x, y):
                continue
            z = ground_z(x, y)
            if 0.6 < z < hmax - 4.5:
                forest_tree(g, ns, x, y, z, rng, rng.uniform(0.9, 1.4))
                placed += 1
    ns['make_obj'](g, 'CX_Trees', coll, sub=0, angle=30, recenter=True)
    w = Builder('CX_Lakes', coll)
    for lk in L.get('lakes', []):
        cx, cy, rx, ry = lk['ellipse']
        X, Y = tb((cx, cy))
        e = shapely.affinity.scale(Point(X, Y).buffer(1.0, 32), rx, ry)
        w.slab(e, None, GROUND_Z + 0.04, 'water')
        if lk.get('fountain'):                # a stone basin in the middle, a jet of water and its splash
            w.slab(Point(X, Y).buffer(3.2, 16).difference(Point(X, Y).buffer(2.6, 16)), -0.2, GROUND_Z + 0.5, 'stone')
            w.cylinder(X, Y, GROUND_Z + 0.1, 2.6, 0.2, 'foam', n=16)
            w.cylinder(X, Y, GROUND_Z + 0.3, 0.6, 2.8, 'foam', n=8, r_top=0.12)
    w.build()
    log('trees', placed)


def build_props(coll):
    ns = load_builders()
    T, Rm, Sc = ns['T'], ns['Rm'], ns['Sc']
    rng = random.Random(3)
    g = ns['Geo'](3)
    b = Builder('CX_Props', coll)
    ok = placeable()
    sig = [tb(p) for p in L['signals']]
    near_sig = lambda x, y, r: any(math.dist((x, y), q) < r for q in sig)  # noqa: E731
    lamps = 0
    for ln, w in road_geo:
        if w < 9:
            continue
        for k, (x, y, dx, dy) in enumerate(road_points(ln, 26.0, 11.0)):     # globe lamps, alternating sides
            sgn = 1 if k % 2 else -1
            X, Y = x - dy * sgn * (w / 2 + 0.7), y + dx * sgn * (w / 2 + 0.7)
            if ok(X, Y, lots_ok=True) and not near_sig(X, Y, 8):
                ns['globe_lamp'](g, T(X, Y, WALK_Z))
                lamps += 1
        # no parked scooters (layout v7): vehicles and people are NPCs in the game, added by code
    # Chợ Lớn's walking street: a string of red lanterns along each side, a step out from the shop fronts, as on Lương
    # Nhữ Học, the lantern street (review r5: 40 m of empty paving between the market rows). Not across the street:
    # strung across, they hung in the camera's way (review r11). Posts carry the wire; the middle stays open.
    shapely.prepare(walks)
    for ln, wk in walk_lines:
        for side in (-1, 1):
            off = side * (wk['w'] / 2 - 1.3)
            prev = None
            for k, (x, y, dx, dy) in enumerate(road_points(ln, 1.7, 0.8)):
                X, Y = x - dy * off, y + dx * off
                if district_kind(x, y) != 'cl' or not shapely.contains_xy(walks, x, y) or x > BUI_VIEN_WEST:
                    prev = None
                    continue
                zw = WALK_Z + 3.3
                if prev is None or k % 5 == 0:
                    g.cyl('archDark', 0.05, zw - WALK_Z + 0.15, 6, T(X, Y, WALK_Z))
                if prev is not None:
                    g.rod('ink', (prev[0], prev[1], zw), (X, Y, zw), 0.012, 3)
                    mx, my = (prev[0] + X) / 2, (prev[1] + Y) / 2
                    g.rod('ink', (mx, my, zw), (mx, my, zw - 0.3), 0.008, 3)
                    g.ico('signRed' if rng.random() < 0.8 else 'signGold', 0.2, T(mx, my, zw - 0.55), sub=0, jit=0.0, sc=(1, 1, 1.3))
                    g.cyl('signGold', 0.09, 0.08, 6, T(mx, my, zw - 0.33))
                prev = (X, Y)
    # roundabout gardens, as on the production map: a clipped topiary, sago palms, a ring of flowers
    for ra in L['roundabouts']:
        if ra.get('model'):
            continue
        X, Y = tb(ra['c'])
        z = WALK_Z + 0.12
        if ra['r'] >= 8:
            ns['topiary'](g, T(X, Y, z))
            for k in range(4):
                a = k * math.pi / 2 + math.pi / 4
                ns['cycad'](g, T(X + math.cos(a) * ra['r'] * 0.6, Y + math.sin(a) * ra['r'] * 0.6, z), rng)
        # the ring of flowers: a low bed round the rim in arcs of colour behind a clipped edge of box (review r5:
        # single clumps read as scattered sweets)
        disc = lambda rr: Point(X, Y).buffer(rr, 48)  # noqa: E731
        b.slab(disc(ra['r'] - 1.15).difference(disc(ra['r'] - 1.45)), z - 0.1, z + 0.34, 'leafDark')
        bed = disc(ra['r'] - 0.35).difference(disc(ra['r'] - 1.15))
        arcs = max(6, int(ra['r']))
        for k in range(arcs):
            a0, a1 = 2 * math.pi * k / arcs, 2 * math.pi * (k + 1) / arcs
            wedge = Polygon([(X, Y)] + [(X + math.cos(a0 + (a1 - a0) * f / 6) * (ra['r'] + 2), Y + math.sin(a0 + (a1 - a0) * f / 6) * (ra['r'] + 2))
                                        for f in range(7)])
            b.slab(bed.intersection(wedge), z - 0.1, z + 0.24, ['flowerRed', 'flowerYellow', 'flowerPink', 'flowerRed', 'white'][k % 5], 'leaf')
    # traffic lights
    for x, y in L['signals']:
        X, Y = tb((x + 6.5, y - 6.5))
        b.box(X - 0.1, Y - 0.1, WALK_Z, X + 0.1, Y + 0.1, WALK_Z + 5.0, 'slate')
        b.box(X - 0.25, Y - 0.25, WALK_Z + 3.7, X + 0.25, Y + 0.25, WALK_Z + 5.0, 'archDark')
        for k, key in enumerate(('sigRed', 'sigAmber', 'sigGreen')):
            z = WALK_Z + 4.75 - k * 0.42
            b.box(X - 0.16, Y - 0.32, z - 0.16, X + 0.16, Y - 0.25, z + 0.16, key)
    # boats anchored in the Bãi Trước bay (scenery, not traffic), umbrellas on Bãi Sau
    for x, y, a in L.get('boats', []):
        X, Y = tb((x, y))
        yaw = math.radians(-a)
        b.oriented_box(X, Y, SEA - 0.3, 6.5, 2.0, 0.9, yaw, 'wallTeal', 'clockBlue')
        b.oriented_box(X, Y, SEA + 0.6, 2.0, 1.4, 1.0, yaw, 'creamLight', 'roofRed')
    for x, y in L.get('umbrellas', []):
        X, Y = tb((x, y))
        g.cyl('white', 0.05, 2.3, 6, T(X, Y, -0.2))
        g.cone(rng.choice(['signRed', 'clothYellow', 'clothBlue', 'wallTeal']), 1.4, 0.6, 10, T(X, Y, 1.75))
    ns['make_obj'](g, 'CX_StreetLife', coll, sub=0, angle=30, recenter=True)
    b.build()
    log('lamps', lamps)


# ---------------------------------------------------------------------------------------------------------------
# preview: sea plane, light, cameras, renders
# ---------------------------------------------------------------------------------------------------------------
def build_preview(coll):
    b = Builder('PREVIEW_Sea', coll)
    b.face([(-4000, -4000, SEA), (4000, -4000, SEA), (4000, 4000, SEA), (-4000, 4000, SEA)], 'sea')
    b.build()


def camera(name, loc, look, lens=None, ortho=None):
    cd = bpy.data.cameras.new(name)
    if ortho:
        cd.type = 'ORTHO'
        cd.ortho_scale = ortho
    else:
        cd.lens = lens or 35
    cd.clip_end = 3000
    ob = bpy.data.objects.new(name, cd)
    ENV.objects.link(ob)
    ob.location = loc
    d = Vector(look) - Vector(loc)
    ob.rotation_euler = d.to_track_quat('-Z', 'Y').to_euler()
    return ob


def render_all(out_dir):
    os.makedirs(out_dir, exist_ok=True)
    sc = SCENE
    sc.render.image_settings.file_format = 'PNG'
    if not WORKBENCH:
        sc.eevee.taa_render_samples = 16
    else:
        workbench(sc)
    shoot(sc, out_dir)


def workbench(sc):
    sc.render.engine = 'BLENDER_WORKBENCH'
    sh = sc.display.shading
    sh.light = 'STUDIO'
    sh.color_type = 'MATERIAL'
    sh.show_object_outline = True
    sh.object_outline_color = lin('#2B211C')
    sh.show_cavity = True
    sh.cavity_type = 'WORLD'
    sh.show_shadows = True
    sh.shadow_intensity = 0.35
    sc.display.light_direction = (-0.45, -0.35, 0.82)
    sc.world = sc.world or bpy.data.worlds.new('World')
    sc.world.color = lin('#DCDCD0')
    sc.render.film_transparent = False
    sc.render.image_settings.file_format = 'PNG'
    sc.view_settings.view_transform = 'Standard'
    sc.view_settings.look = 'None'
    sc.view_settings.exposure = 0.25


SHOTS = []


def make_cameras():
    """the review shots; the scene camera is left on the centre so F12 / numpad 0 in Blender shows it"""
    T = lambda x, y: tb((x, y))  # noqa: E731
    shots = [
        ('00_top', (0, 0, 400), (0, 0, 0), dict(ortho=540), (1600, 1500)),
        ('01_overview', (-300, -420, 360), (0, -10, 0), dict(lens=34), (1800, 1100)),
        ('02_center', (*T(150, 300), 110), (*T(210, 150), 0), dict(lens=32), (1800, 1100)),
        ('03_vung_tau', (*T(300, 520), 95), (*T(410, 380), 8), dict(lens=30), (1800, 1100)),
        ('04_river', (*T(430, 210), 70), (*T(300, 100), 0), dict(lens=30), (1800, 1100)),
        ('05_street', (*T(60, 255), 22), (*T(140, 190), 6), dict(lens=28), (1800, 1100)),
    ]
    for name, loc, look, kw, res in shots:
        SHOTS.append((name, camera('CAM_' + name, loc, look, **kw), res))
    SCENE.camera = next(c for n, c, _ in SHOTS if n == '02_center')
    SCENE.render.resolution_x, SCENE.render.resolution_y = 1800, 1100


def shoot(sc, out_dir):
    for name, cam, res in SHOTS:
        sc.camera = cam
        sc.render.resolution_x, sc.render.resolution_y = res
        sc.render.filepath = os.path.join(out_dir, name + '.png')
        bpy.ops.render.render(write_still=True, scene=sc.name)
        log('rendered', sc.render.filepath)
    sc.camera = next(c for n, c, _ in SHOTS if n == '02_center')


def viewport_setup():
    """open the file looking at the whole island with the painted look (scene sun and paper world)"""
    for scr in bpy.data.screens:
        for area in scr.areas:
            if area.type != 'VIEW_3D':
                continue
            sp = area.spaces[0]
            sp.shading.type = 'MATERIAL'
            sp.shading.use_scene_lights = True
            sp.shading.use_scene_world = True
            sp.clip_end = 5000
            r3 = sp.region_3d
            r3.view_perspective = 'PERSP'
            r3.view_location = (0, -20, 0)
            r3.view_distance = 620
            r3.view_rotation = Vector((-0.5, -0.75, 0.45)).to_track_quat('Z', 'Y')


def run_mini_scripts():
    import runpy
    for name in MINI_SCRIPTS:
        part = runpy.run_path(os.path.join(HERE, 'mini', name + '.py'), run_name='saigon_mini_part')
        log(name, part['run']())


def main():
    global SCENE, WORLD, ENV
    for o in list(bpy.data.objects):
        bpy.data.objects.remove(o, do_unlink=True)
    bpy.context.scene.name = 'SaigonDiorama'          # setup_scene() picks it up instead of adding a second scene
    ns = load_builders()
    SCENE, WORLD, ENV = ns['setup_scene']()
    SCENE.unit_settings.system = 'METRIC'
    no_ink = collection('SD_NoInk')                   # water: no ink lines, no wobble (docs/PROJECT.md)
    c_lm = collection('SM_Landmarks', WORLD)
    place_models(c_lm)          # first, so our SD_ materials reuse the appended ones
    build_terrain(collection('SM_Ground', WORLD))
    build_mountains(collection('SM_Mountains', WORLD))
    build_streets(collection('SM_Streets', WORLD))
    build_bridges(c_lm)
    build_houses(collection('SM_Houses', WORLD))
    build_new_landmarks(c_lm)
    build_nature(collection('SM_Nature', WORLD))
    build_props(collection('SM_Props', WORLD))
    build_preview(collection('SM_Preview', no_ink))
    run_mini_scripts()
    try:
        ns['art_pass_shader']()
    except Exception as e:      # the appended SD_Paint already carries it when its node names differ
        log('art pass shader skipped:', e)
    ns['art_pass_geometry']()
    for o in WORLD.all_objects:   # flat ground pieces and the steel bridge stay straight
        # (and the streets: the wobble lifted the asphalt over its zebra stripes and lane marks, review r10)
        if o.name.startswith(('CX_Lawn', 'CX_Lakes', 'LM_Bridges', 'LM_CauBaSon', 'IX_NuiNho', 'IX_Roads', 'IX_RoadMarks')) and o.modifiers.get('SD_HandWobble'):
            o.modifiers.remove(o.modifiers['SD_HandWobble'])
    ns['setup_ink'](SCENE, WORLD, ENV)
    make_cameras()
    viewport_colours()
    viewport_setup()
    bpy.ops.wm.save_as_mainfile(filepath=OUT, compress=True)
    log('saved', OUT)
    if RENDER:
        render_all(RENDER)


main()
