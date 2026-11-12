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
from shapely.ops import unary_union  # noqa: E402
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
roads = unary_union([ln.buffer(w / 2, cap_style='round', join_style='round') for ln, w in road_geo])
walk_outer = [ln.buffer(w / 2 + SIDEWALK, cap_style='round', join_style='round') for ln, w in road_geo]
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
# where the ground stays level: streets, pavements, lots, landmarks, shores, river banks, beaches, mountain feet
FLAT = unary_union([roads.buffer(3.5), unary_union(walk_outer), walks, plazas, lm_keepout,
                    unary_union([rect_poly(r) for r, _ in LOTS]).buffer(2.0), coast.exterior.buffer(6.0), river.buffer(6.0),
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
        bz = np.where(sgn >= 0, -0.6 + 0.62 * np.clip(sgn / half, 0, 1), -0.6 + 0.26 * np.maximum(sgn, -half))
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
    walkway = walkway.difference(roads).intersection(on_land).difference(beach_all)
    r = Builder('IX_Roads', coll)
    r.slab(road_top, -0.1, ROAD_Z, 'asphalt', 'curb')
    r.slab(walkway, -0.1, WALK_Z, 'paving', 'curb')
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
    for ln, w in road_geo:
        if w < 9:
            continue
        n = int(ln.length // 4)
        for i in range(n):
            a = ln.interpolate(i * 4 + 0.5)
            b = ln.interpolate(i * 4 + 2.5)
            if not road_top.contains(a) or not road_top.contains(b) or bridge_cut.contains(a):
                continue
            seg = LineString([a, b]).buffer(0.09, cap_style='flat')
            m.slab(seg, None, ROAD_Z + 0.005, 'roadLine')
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


def build_house(ns, g, rng, kind, w, D):
    """one house in its local frame (facade at y = 0 facing -Y, depth +Y), with the production map's builders"""
    T = ns['T']
    if kind == 'ks':
        ns['modern_block'](g, w, D, rng.choice([3, 4, 4, 5, 6]), rng, palette=rng.choice(KS_PALETTES))
        return
    if kind == 'np' and w >= 5.2 and rng.random() < 0.22:
        ns['modern_block'](g, w, D, rng.choice([2, 3, 4]), rng)
        return
    if kind == 'bar' and 'house_bv' in ns:
        ns['house_bv'](ns, g, w, D, rng, floors=rng.choice([1, 2, 2, 3]), paint=rng.choice(ns.get('BV_PAINT', NP_WALLS)),
                       roof='flat', sign=rng.choice(ns.get('BV_SIGNS', ['BAR'])))
        return
    if kind == 'cl':
        ns['house_plain'](g, w, D, rng, floors=1, paint=rng.choice(CL_WALLS), roof='tile')
        for x in (-w / 4, w / 4):
            ns['lantern'](g, T(x, -1.1, 2.7))
        return
    floors = rng.choice([1, 2, 2, 3, 3, 4]) if kind == 'np' else rng.choice([1, 1, 2])
    wall = rng.choice(NP_WALLS if kind == 'np' else HEM_WALLS)
    top = ns['house_plain'](g, w, D, rng, floors=floors, paint=wall, roof='flat')
    if rng.random() < (0.6 if kind == 'np' else 0.35):
        tum(g, w, D, top - 0.85, wall)


def build_houses(coll):
    ns = load_builders()
    T, Rm = ns['T'], ns['Rm']
    rng = random.Random(2026)
    cut = roads.union(walks).union(lm_keepout).union(river).union(plazas)
    geos = {}
    back = Builder('SH_Backs', coll)
    count = 0
    for rect, kind in LOTS:
        wmin, wmax, D0 = FRONT[kind]
        g = geos.setdefault(kind, ns['Geo'](len(geos) + 1))
        poly = rect_poly(rect).difference(cut).intersection(land.buffer(-1.0)).intersection(inland)
        for p in polys(poly):
            if p.area < 20:
                continue
            p = orient(p.simplify(0.3), 1.0)
            minx, miny, maxx, maxy = p.bounds
            D = min(D0, 0.5 * min(maxx - minx, maxy - miny))
            if D < 4.5:
                D = min(D0, min(maxx - minx, maxy - miny))
            if D < 3.0:
                continue
            placed = []
            coords = list(p.exterior.coords)
            for (ax, ay), (bx, by) in zip(coords, coords[1:]):
                el = math.hypot(bx - ax, by - ay)
                if el < 3.4:
                    continue
                ux, uy = (bx - ax) / el, (by - ay) / el
                nx, ny = -uy, ux
                t = 0.0
                while t < el - 3.0:
                    w = min(rng.uniform(wmin, wmax), el - t)
                    if w < 3.0:
                        break
                    if kind == 'ks' and rng.random() < 0.2:
                        t += w + 2.0       # a gap between hotels: an alley to the beach
                        continue
                    sx, sy = ax + ux * t, ay + uy * t
                    foot = Polygon([(sx, sy), (sx + ux * w, sy + uy * w), (sx + ux * w + nx * D, sy + uy * w + ny * D), (sx + nx * D, sy + ny * D)])
                    if foot.intersection(p).area > 0.8 * foot.area and all(foot.intersection(q).area < 0.12 * foot.area for q in placed):
                        placed.append(foot)
                        depth = len(g.st)
                        g.push(T(sx + ux * w / 2, sy + uy * w / 2, WALK_Z) @ Rm(0, 0, math.atan2(uy, ux)))
                        try:
                            build_house(ns, g, rng, kind, w, D)
                            count += 1
                        except Exception as e:
                            log('house failed:', kind, type(e).__name__, e)
                        del g.st[depth:]
                    t += w
            rest = p.difference(unary_union(placed).buffer(0.4)) if placed else p
            for q in polys(rest.buffer(-1.0)):
                if q.area > 30:
                    back.slab(q, WALK_Z, WALK_Z + rng.choice([2, 3]) * 3.1, 'concrete', rng.choice(['wallBeige', 'cream', 'wallGrey']))
    for kind, g in geos.items():
        ns['make_obj'](g, 'SH_' + kind, coll, sub=0, angle=30, recenter=True)
    back.build()
    log('houses', count)


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


def build_new_landmarks(coll):
    ns = load_builders()
    rng = random.Random(42)
    gm = ns['Geo'](42)
    b = Builder('LM_Blockout', coll)
    for lm in L['landmarks']:
        if lm.get('models'):
            continue
        if lm['id'] == 'chung_cu_42':
            cx, cy = tb(lm['at'])
            gm.push(face_matrix(ns, cx, cy, lm['size'][1], lm['face']))
            ns['modern_block'](gm, lm['size'][0], lm['size'][1], 8, rng, palette=('cream', 'glassTeal', 'creamLight'))
            gm.pop()
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
    houses = unary_union([rect_poly(r) for r, _ in LOTS])
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
    for ln, w in road_geo:                    # street trees on the main streets
        if w < 10:
            continue
        for x, y, dx, dy in road_points(ln, 13.0, 5.0):
            for sgn in (1, -1):
                X, Y = x - dy * sgn * (w / 2 + 1.6), y + dx * sgn * (w / 2 + 1.6)
                if ok(X, Y, lots_ok=True):
                    street_tree(g, ns, X, Y, WALK_Z, rng)
                    placed += 1
    flowers = Builder('CX_Flowers', coll)
    for pk in L['parks']:                     # flower fields: low rows of green with a yellow top (sunflowers)
        if pk.get('kind') != 'flowers':
            continue
        poly = rect_poly(pk['rect']).intersection(land.buffer(-0.5)).difference(roads)
        minx, miny, maxx, maxy = poly.bounds
        along_y = (maxy - miny) > (maxx - minx)
        flowers.slab(poly.buffer(-0.2), -0.1, GROUND_Z + 0.05, 'earth')
        k = 0.0
        span = (maxx - minx) if along_y else (maxy - miny)
        while k + 0.6 < span:
            if along_y:
                row = sbox(minx + k + 0.25, miny + 0.6, minx + k + 0.8, maxy - 0.6).intersection(poly)
            else:
                row = sbox(minx + 0.6, miny + k + 0.25, maxx - 0.6, miny + k + 0.8).intersection(poly)
            flowers.slab(row, GROUND_Z, GROUND_Z + 0.55, 'flowerYellow', 'leaf')
            k += 1.15
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
    # zebra crossings on each arm of the signalled crossings
    for (sx, sy) in sig:
        for ln, w in road_geo:
            if w < 8 or ln.distance(Point(sx, sy)) > 1.5:
                continue
            t0 = ln.project(Point(sx, sy))
            for sgn in (-1, 1):
                t = t0 + sgn * 10.0
                if t < 0 or t > ln.length:
                    continue
                p = ln.interpolate(t)
                q = ln.interpolate(min(ln.length, t + 0.5))
                o = ln.interpolate(max(0.0, t - 0.5))
                dx, dy = q.x - o.x, q.y - o.y
                dl = math.hypot(dx, dy) or 1.0
                dx, dy = dx / dl, dy / dl
                k = -(w / 2 - 0.7)
                while k <= w / 2 - 0.7:
                    b.oriented_box(p.x - dy * k, p.y + dx * k, ROAD_Z, 2.8, 0.45, 0.012, math.atan2(dy, dx), 'roadLine')
                    k += 0.9
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
        n = int(2 * math.pi * (ra['r'] - 1.0) / 1.1)
        for k in range(n):
            a = 2 * math.pi * k / n
            g.ico(rng.choice(['flowerRed', 'flowerPink', 'flowerYellow', 'flowerRed']), 0.3,
                  T(X + math.cos(a) * (ra['r'] - 0.8), Y + math.sin(a) * (ra['r'] - 0.8), z + 0.15), sub=1, jit=0.1)
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
        if o.name.startswith(('CX_Lawn', 'CX_Lakes', 'LM_Bridges', 'LM_CauBaSon', 'IX_NuiNho')) and o.modifiers.get('SD_HandWobble'):
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
