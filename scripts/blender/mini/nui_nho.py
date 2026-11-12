"""Núi Nhỏ (Vũng Tàu) for Sài Gòn mini, map v5: docs/map-v5-tasks.md, T3.

A gentler hill than the blockout dome, true to the real one (Núi Nhỏ / Tao Phùng: two summits, the lighthouse on the
higher one, the Christ statue on the lower southern one). Real proportions at the map's scale would give a hill of
about 14 m on this footprint (170 m over ~1 km); the landmarks on top are enlarged instead, as the style rules say.

- Terrain: a ridge running north-south between two summits, the ground rising from the Hạ Long road with no cut
  (the foot starts at road level and flares out), a soft undulation on the slopes, smooth normals.
- North summit (lighthouse, T4) and south summit (statue, T5): level paved terraces.
- Đường Hải Đăng: a 4 m road winding up from the Quốc lộ 51 / Hạ Long corner to the lighthouse terrace.
- Stairs from the Hạ Long road on the Bãi Sau side up to the statue terrace, with landings and low side walls.
- A footpath along the ridge joining the two terraces.
- Forest in clumps (not sprinkled), clearings round the terraces and along the paths, granite outcrops, and the rocks
  of Mũi Nghinh Phong in the sea at the southern tip.

Objects (all rebuilt by run()): IS_nui_nho, CX_NuiNho_Trees, CX_NuiNho_Rocks, IX_NuiNho_Road, LF_nui_nho_Stairs,
LF_NuiNho_Paths. run() also takes the old blockout's trees off CX_Trees and its boulders off IS_Boulders.

Run in the open models/saigon_mini.blend (exec the file and call run(), then save), or let build_saigon_mini.py call
it (MINI_SCRIPTS).

Layout v7 moved the hill whole to the tip of the east bank: the numbers below are the design as approved (Blender XY
of layout v2) and the layout's 'designMove' (plan metres) carries everything to where it stands now. The hill is
built in the design frame (the layout's outline, roads and the island's ground are brought back into it) and the
finished meshes are moved; hai_dang.py and tuong_chua.py do the same with MX, MY.
"""
import math
import os
import sys

import numpy as np
from mathutils import Vector

HERE = os.path.dirname(os.path.abspath(__file__))
if HERE not in sys.path:
    sys.path.insert(0, HERE)
import kit  # noqa: E402
import layout as LY  # noqa: E402
import shapely  # noqa: E402
import shapely.affinity  # noqa: E402
from shapely.geometry import LineString, Point  # noqa: E402
from shapely.ops import unary_union  # noqa: E402

# ---- design (Blender XY, metres) ----
PN, HN = (-30.0, -151.0), 14.0        # north summit: the lighthouse (higher, like the real one)
PS, HS = (-26.0, -186.0), 12.0        # south summit: the statue, looking out to sea
SADDLE = 8.5                           # lowest point of the ridge between them
TERRACE_N, TERRACE_S = 6.5, 7.5        # paved terraces round the summits
GRID = 1.8                             # terrain triangle size
ROAD_W = 4.0
ROAD_START, ROAD_HEADING = (-42.0, -116.0), -15.0   # off Quốc lộ 51 at the north foot, heading east
ROAD_GRADE = 0.15
# stairs from the Hạ Long road on the Bãi Sau side to the statue terrace (three flights, two landings); they follow
# the ground, like the real ones
STAIR_PTS = [(-0.5, -190.0), (-6.5, -183.5), (-11.0, -192.5), (-18.8, -189.5)]
STAIR_W = 2.6

OBJECTS = ['IS_nui_nho', 'CX_NuiNho_Trees', 'CX_NuiNho_Rocks', 'IX_NuiNho_Road', 'LF_nui_nho_Stairs', 'LF_NuiNho_Paths']

_mt, OUTLINE_WORLD = LY.mountains['nui_nho']
MX, MY = _mt.get('designMove', (0.0, 0.0))[0], -_mt.get('designMove', (0.0, 0.0))[1]    # design frame -> world (Blender XY)
OUTLINE = shapely.affinity.translate(OUTLINE_WORLD, -MX, -MY)
ROADS = shapely.affinity.translate(LY.roads, -MX, -MY)
# the blockout outline, let out to the north-east onto the empty lawn so the north flank runs down gently towards the
# city (the real hill's north side meets the town); less the roads round it
NORTH_FOOT = shapely.box(-39.0, -128.0, 9.0, -104.0).buffer(8.0).buffer(-8.0)
_fp = OUTLINE.union(NORTH_FOOT).buffer(6.0).buffer(-6.0).difference(ROADS.buffer(1.2))
FOOT = max(getattr(_fp, 'geoms', [_fp]), key=lambda p: p.area)
shapely.prepare(FOOT)
RIDGE = LineString([PN, PS])
# the summit core: both terraces and the ridge between them, all at the top height
CORE = unary_union([RIDGE.buffer(0.8), Point(PN).buffer(TERRACE_N, 32), Point(PS).buffer(TERRACE_S, 32)])
shapely.prepare(CORE)
STAIRS = LineString(STAIR_PTS)


def _top(xy):
    """height of the summit core above each point: the terraces level, the ridge dipping to the saddle"""
    a, b = np.array(PN), np.array(PS)
    d = b - a
    ln = float(np.hypot(*d))
    u = np.clip(((xy - a) @ d) / (d @ d), 0.0, 1.0) * ln
    v = np.clip((u - TERRACE_N) / (ln - TERRACE_N - TERRACE_S), 0.0, 1.0)
    return (1 - v) * HN + v * HS - ((HN + HS) / 2 - SADDLE) * 4 * v * (1 - v)


def base_height(xy):
    """the hill without paths: eased from the edge of the footprint (road level) up to the summit core"""
    xy = np.atleast_2d(np.asarray(xy, dtype=float))
    pts = shapely.points(xy)
    inside = shapely.contains_xy(FOOT, xy[:, 0], xy[:, 1])
    de = shapely.distance(FOOT.exterior, pts) * inside
    dp = shapely.distance(CORE, pts)
    t = np.clip(de / np.maximum(de + dp, 1e-6), 0.0, 1.0) ** 0.85
    s = t * t * (3 - 2 * t)
    z = _top(xy) * s
    # a soft undulation on the mid slopes so they are not perfect
    z += 0.5 * np.sin(xy[:, 0] / 6.3 + 0.7) * np.cos(xy[:, 1] / 5.1 - 0.4) * 4 * s * (1 - s)
    return np.where(inside, z, 0.0)


def _trace_road():
    """Đường Hải Đăng: walk up the hill a metre at a time, steering so the road keeps to the ground at a steady grade
    (it winds round the north summit like the real one) until it reaches the lighthouse terrace"""
    inner = FOOT.buffer(-(ROAD_W / 2 + 1.5))
    shapely.prepare(inner)
    stairs_zone = STAIRS.buffer(STAIR_W / 2 + 6.0)
    p = np.array(ROAD_START, dtype=float)
    h = math.radians(ROAD_HEADING)
    pts, z0 = [tuple(p)], LY.ROAD_Z
    s = 0.0
    for _ in range(400):
        target = z0 + ROAD_GRADE * (s + 1.0)
        best = None
        for dh in np.linspace(-0.5, 0.5, 21):
            hh = h + dh
            q = p + np.array([math.cos(hh), math.sin(hh)])
            if not inner.contains(Point(q)) and s > 9:      # the first metres leave Quốc lộ 51
                continue
            cost = (float(base_height([q])[0]) - target) ** 2 + 2.0 * dh * dh
            if stairs_zone.contains(Point(q)) or Point(q).distance(Point(PS)) < TERRACE_S + 4:
                cost += 100.0
            for k, r in enumerate(pts[:-14]):
                dd = math.dist(q, r)
                if dd < ROAD_W + 4.5:
                    cost += 40.0 * (ROAD_W + 4.5 - dd)
            if best is None or cost < best[0]:
                best = (cost, q, hh)
        if best is None:
            break
        _, p, h = best
        pts.append(tuple(p))
        s += 1.0
        if math.dist(p, PN) <= TERRACE_N + 0.6 or target >= HN:
            break
    # aim the last stretch at the terrace and smooth the corners
    end = np.array(PN) + (np.array(pts[-1]) - np.array(PN)) / max(1e-6, math.dist(pts[-1], PN)) * (TERRACE_N - 0.4)
    pts.append(tuple(end))
    for _ in range(2):
        out = [pts[0]]
        for a, b in zip(pts, pts[1:]):
            out += [(0.75 * a[0] + 0.25 * b[0], 0.75 * a[1] + 0.25 * b[1]), (0.25 * a[0] + 0.75 * b[0], 0.25 * a[1] + 0.75 * b[1])]
        pts = out + [pts[-1]]
    return LineString(pts).simplify(0.05)


ROAD = _trace_road()
ROAD_TOP = HN + 0.12


def road_z(s):
    s = np.clip(np.asarray(s, dtype=float), 0.0, ROAD.length)
    a = s / ROAD.length
    return LY.ROAD_Z + (ROAD_TOP - LY.ROAD_Z) * a


def _stair_profile():
    """stairs follow the ground (monotonic), level at the two landings"""
    n = 120
    ss = np.linspace(0, STAIRS.length, n)
    P = np.array([STAIRS.interpolate(v).coords[0] for v in ss])
    z = np.maximum.accumulate(base_height(P))
    z[0] = LY.ROAD_Z + 0.11
    z[-1] = HS + 0.12
    z = np.maximum.accumulate(z)
    corners = [STAIRS.project(Point(p)) for p in STAIR_PTS[1:-1]]
    for c in corners:
        m = np.abs(ss - c) < 1.3
        if m.any():
            z[m] = z[m].mean()
    z = np.maximum.accumulate(z)
    return ss, z


_SS, _SZ = _stair_profile()


def stair_z(s):
    return np.interp(np.asarray(s, dtype=float), _SS, _SZ)


def heights(xy):
    """final ground: the hill with the road and the stairs cut into it"""
    xy = np.atleast_2d(np.asarray(xy, dtype=float))
    z = base_height(xy)
    pts = shapely.points(xy)
    for line, prof, half in ((ROAD, road_z, ROAD_W / 2 + 0.6), (STAIRS, stair_z, STAIR_W / 2 + 0.6)):
        d = shapely.distance(line, pts)
        s = shapely.line_locate_point(line, pts)
        w = 1 - kit.smoothstep(half, half + 5.0, d)
        z = z * (1 - w) + (prof(s) - 0.04) * w
    return z


_GROUND = [None]      # the island's ground (IS_Ground) as a BVH, so the hill's foot meets the rolling lawn


def _ground_bvh():
    import bmesh
    import bpy
    from mathutils.bvhtree import BVHTree
    ob = bpy.data.objects.get('IS_Ground')
    if ob is None:
        return None
    bm = bmesh.new()
    bm.from_mesh(ob.data)
    bm.transform(ob.matrix_world)
    t = BVHTree.FromBMesh(bm)
    bm.free()
    return t


def final_z(xy):
    """the hill's height, never below the island's own ground (the lawn rolls a little round the foot)"""
    xy = np.atleast_2d(np.asarray(xy, dtype=float))
    z = heights(xy)
    t = _GROUND[0]
    if t is not None:
        for i, (x, y) in enumerate(xy):
            hit = t.ray_cast(Vector((x + MX, y + MY, 60.0)), Vector((0, 0, -1)))
            if hit[0] is not None:
                z[i] = max(z[i], hit[0].z + 0.03)
    return z


def slope_deg(xy, z, tris):
    out = []
    for a, b, c in tris:
        A = Vector((*xy[a], z[a])); B = Vector((*xy[b], z[b])); C = Vector((*xy[c], z[c]))
        n = (B - A).cross(C - A)
        out.append(math.degrees(math.acos(max(-1.0, min(1.0, n.normalized().z)))) if n.length > 1e-9 else 0.0)
    return np.array(out)


def forest_mask(x, y):
    """clumps of forest: > 0 inside a clump"""
    return (np.sin(x / 9.0 + 1.1) * np.cos(y / 7.5 - 0.3) + 0.6 * np.sin((x + y) / 5.2 + 2.0) * np.cos((x - y) / 6.4)
            + 0.25)


def build_terrain(coll):
    xy, tris = kit.tri_grid(FOOT, GRID)
    z = final_z(xy)
    sl = slope_deg(xy, z, tris)
    cen = np.array([[(xy[a][0] + xy[b][0] + xy[c][0]) / 3, (xy[a][1] + xy[b][1] + xy[c][1]) / 3] for a, b, c in tris])
    zc = np.array([(z[a] + z[b] + z[c]) / 3 for a, b, c in tris])
    fm = forest_mask(cen[:, 0], cen[:, 1])
    hx = (cen[:, 0] * 0.37 + cen[:, 1] * 0.61) % 7
    pts = shapely.points(cen)
    near_path = (shapely.distance(ROAD, pts) < ROAD_W / 2 + 1.2) | (shapely.distance(STAIRS, pts) < STAIR_W / 2 + 1.0)
    mats = []
    for i in range(len(tris)):
        if sl[i] > 46:
            mats.append('rock')
        elif near_path[i]:
            mats.append('grassDark')
        elif fm[i] > 0 and zc[i] > 0.8:
            mats.append('leafDark' if hx[i] < 5.2 else 'grassDark')
        else:
            mats.append('grass' if hx[i] < 5.0 else 'grassDark')
    g = kit.Mesh(3)
    V = [(x, y, zz) for (x, y), zz in zip(xy, z)]
    g.tris(V, tris, mats)
    ob = kit.make_object(g, 'IS_nui_nho', coll, smooth_angle=50, weld=True)
    return ob, (xy, z, tris, sl)


def path_points(line, prof, step, lift):
    out = []
    n = max(2, int(line.length / step))
    for i in range(n + 1):
        s = line.length * i / n
        p = line.interpolate(s)
        out.append(Vector((p.x, p.y, float(prof(s)) + lift)))
    return out


def build_road(coll):
    g = kit.Mesh(4)
    P = path_points(ROAD, road_z, 0.8, 0.0)
    g.ribbon('asphalt', P, ROAD_W / 2, -0.25, 0.02)
    for side in (1, -1):
        Q = []
        for i, p in enumerate(P):
            t = P[min(i + 1, len(P) - 1)] - P[max(i - 1, 0)]
            t.z = 0
            t.normalize()
            Q.append(p + Vector((-t.y, t.x, 0)) * side * (ROAD_W / 2 + 0.15))
        g.ribbon('curb', Q, 0.15, -0.25, 0.16)
    # a dashed centre line like the city streets
    s = 1.0
    while s + 2.0 < ROAD.length - 1.0:
        seg = LineString([ROAD.interpolate(s + k * 0.5) for k in range(5)])
        g.ribbon('roadLine', path_points(seg, lambda q, s0=s: road_z(s0 + q), 0.5, 0.025), 0.08, 0.0, 0.012, caps=False)
        s += 4.0
    return kit.make_object(g, 'IX_NuiNho_Road', coll)


def build_stairs(coll):
    g = kit.Mesh(5)
    L_ = STAIRS.length
    corners = [STAIRS.project(Point(p)) for p in STAIR_PTS[1:-1]]
    step = 0.55
    s = 0.0
    while s < L_ - 0.01:
        s1 = min(L_, s + step)
        mid = (s + s1) / 2
        landing = any(abs(mid - c) < 1.4 for c in corners)
        a, b = STAIRS.interpolate(s), STAIRS.interpolate(s1)
        dx, dy = b.x - a.x, b.y - a.y
        dl = math.hypot(dx, dy) or 1.0
        ux, uy = dx / dl, dy / dl
        nx, ny = -uy, ux
        top = float(stair_z(mid)) if not landing else float(stair_z(min(corners, key=lambda c: abs(mid - c))))
        hw = STAIR_W / 2
        quad = [(a.x + nx * hw, a.y + ny * hw), (b.x + nx * hw, b.y + ny * hw), (b.x - nx * hw, b.y - ny * hw), (a.x - nx * hw, a.y - ny * hw)]
        V = [(x, y, top - 0.9) for x, y in quad] + [(x, y, top) for x, y in quad]
        g.raw('stone', V, [(0, 3, 2, 1), (4, 5, 6, 7), (0, 1, 5, 4), (1, 2, 6, 5), (2, 3, 7, 6), (3, 0, 4, 7)])
        # low side walls
        for sgn in (1, -1):
            o0, o1 = hw, hw + 0.35
            wq = [(a.x + nx * sgn * o0, a.y + ny * sgn * o0), (b.x + nx * sgn * o0, b.y + ny * sgn * o0),
                  (b.x + nx * sgn * o1, b.y + ny * sgn * o1), (a.x + nx * sgn * o1, a.y + ny * sgn * o1)]
            V = [(x, y, top - 0.9) for x, y in wq] + [(x, y, top + 0.55) for x, y in wq]
            g.raw('concrete', V, [(0, 3, 2, 1), (4, 5, 6, 7), (0, 1, 5, 4), (1, 2, 6, 5), (2, 3, 7, 6), (3, 0, 4, 7)])
        s = s1
    return kit.make_object(g, 'LF_nui_nho_Stairs', coll)


def build_paths(coll):
    """the two summit terraces and the ridge path between them"""
    g = kit.Mesh(6)
    for (px, py), h, r, opening in ((PN, HN, TERRACE_N, ROAD.coords[-1]), (PS, HS, TERRACE_S, STAIRS.coords[-1])):
        seg = 24
        g.ring('stone', kit.Matrix.Translation((px, py, h - 0.6)), r, 0.72, seg)
        # a knee-high wall round the terrace, open towards the path that arrives and towards the ridge
        gaps = [math.atan2(opening[1] - py, opening[0] - px), math.atan2((PS if (px, py) == PN else PN)[1] - py, (PS if (px, py) == PN else PN)[0] - px)]
        for k in range(seg):
            a0, a1 = 2 * math.pi * k / seg, 2 * math.pi * (k + 1) / seg
            am = (a0 + a1) / 2
            if any(abs(math.atan2(math.sin(am - gp), math.cos(am - gp))) < 0.42 for gp in gaps):
                continue
            pts = [(px + (r - 0.35) * math.cos(a0), py + (r - 0.35) * math.sin(a0)), (px + r * math.cos(a0), py + r * math.sin(a0)),
                   (px + r * math.cos(a1), py + r * math.sin(a1)), (px + (r - 0.35) * math.cos(a1), py + (r - 0.35) * math.sin(a1))]
            V = [(x, y, h + 0.12) for x, y in pts] + [(x, y, h + 0.62) for x, y in pts]
            g.raw('concrete', V, [(0, 3, 2, 1), (4, 5, 6, 7), (0, 1, 5, 4), (1, 2, 6, 5), (2, 3, 7, 6), (3, 0, 4, 7)])
    # ridge footpath, laid on the ground
    a = Point(PN).buffer(TERRACE_N - 0.3).exterior
    b = Point(PS).buffer(TERRACE_S - 0.3).exterior
    ln = LineString([PN, PS]).difference(Point(PN).buffer(TERRACE_N - 0.3)).difference(Point(PS).buffer(TERRACE_S - 0.3))
    P = []
    n = max(2, int(ln.length / 0.8))
    for i in range(n + 1):
        p = ln.interpolate(ln.length * i / n)
        P.append(Vector((p.x, p.y, float(final_z([(p.x, p.y)])[0]) + 0.05)))
    g.ribbon('paving', P, 1.0, -0.3, 0.03)
    del a, b
    return kit.make_object(g, 'LF_NuiNho_Paths', coll)


def build_trees(coll, terrain):
    xy, z, tris, sl = terrain
    rng = np.random.default_rng(1974)
    g = kit.Mesh(7)
    minx, miny, maxx, maxy = FOOT.bounds
    keep_out = unary_union([ROAD.buffer(ROAD_W / 2 + 2.2), STAIRS.buffer(STAIR_W / 2 + 2.0), RIDGE.buffer(2.2),
                            Point(PN).buffer(TERRACE_N + 2.5), Point(PS).buffer(TERRACE_S + 2.5),
                            FOOT.exterior.buffer(3.0)])
    shapely.prepare(keep_out)
    n = 0
    tries = int(FOOT.area / 5.5)
    cand = np.c_[rng.uniform(minx, maxx, tries), rng.uniform(miny, maxy, tries)]
    cand = cand[shapely.contains_xy(FOOT, cand[:, 0], cand[:, 1]) & ~shapely.contains_xy(keep_out, cand[:, 0], cand[:, 1])]
    fm = forest_mask(cand[:, 0], cand[:, 1])
    cand = cand[(fm > 0.05) | (rng.uniform(0, 1, len(cand)) < 0.06)]
    placed = []
    for x, y in cand:
        if any((x - px) ** 2 + (y - py) ** 2 < 5.2 for px, py in placed[-60:]):
            continue
        zz = float(final_z([(x, y)])[0])
        if zz < 0.5:
            continue
        placed.append((x, y))
        s = rng.uniform(0.85, 1.3)
        h = rng.uniform(1.4, 2.4) * s
        g.ring('bark', kit.Matrix.Translation((x, y, zz - 0.3)), 0.17 * s, h + 0.3, 5, r2=0.1 * s)
        g.ico(rng.choice(['leafDark', 'leaf', 'leafDark']), (x, y, zz + h + 0.7 * s), rng.uniform(1.25, 1.65) * s,
              sub=1, scale=(1, 1, 0.82), jit=0.12, rng=rng)
        if rng.uniform() < 0.45:
            g.ico(rng.choice(['leaf', 'leafLight', 'leafDark']), (x + rng.uniform(-0.8, 0.8), y + rng.uniform(-0.8, 0.8), zz + h + 1.5 * s),
                  rng.uniform(0.8, 1.1) * s, sub=1, scale=(1, 1, 0.8), jit=0.12, rng=rng)
        n += 1
    # bougainvillea along the upper road
    for k in range(9):
        s = ROAD.length * (0.45 + 0.055 * k)
        p = ROAD.interpolate(s)
        q = ROAD.interpolate(min(ROAD.length, s + 0.5))
        dx, dy = q.x - p.x, q.y - p.y
        dl = math.hypot(dx, dy) or 1
        side = 1 if k % 2 else -1
        x, y = p.x - dy / dl * side * (ROAD_W / 2 + 0.9), p.y + dx / dl * side * (ROAD_W / 2 + 0.9)
        zz = float(road_z(s))
        g.ico('flowerPink', (x, y, zz + 0.6), 0.75, sub=1, scale=(1.3, 1.0, 0.7), jit=0.15, rng=rng)
        g.ico('leaf', (x + 0.4, y, zz + 0.35), 0.55, sub=1, scale=(1.2, 1.0, 0.7), jit=0.15, rng=rng)
    kit.make_object(g, 'CX_NuiNho_Trees', coll)
    return n


def build_rocks(coll, terrain):
    xy, z, tris, sl = terrain
    rng = np.random.default_rng(23)
    g = kit.Mesh(8)
    cen = np.array([[(xy[a][0] + xy[b][0] + xy[c][0]) / 3, (xy[a][1] + xy[b][1] + xy[c][1]) / 3] for a, b, c in tris])
    keep_out = unary_union([ROAD.buffer(ROAD_W / 2 + 1.5), STAIRS.buffer(STAIR_W / 2 + 1.5), RIDGE.buffer(1.8),
                            Point(PN).buffer(TERRACE_N + 1.0), Point(PS).buffer(TERRACE_S + 1.0)])
    steep = [i for i in np.argsort(-sl)[:400] if not keep_out.contains(Point(cen[i]))]
    rng.shuffle(steep)
    placed = []
    for i in steep:
        x, y = cen[i]
        if any(math.dist((x, y), q) < 7 for q in placed):
            continue
        placed.append((x, y))
        zz = float(final_z([(x, y)])[0])
        r = rng.uniform(0.9, 1.7)
        g.ico(rng.choice(['rock', 'rockDark', 'rock']), (x, y, zz - 0.2), r, sub=1,
              scale=(1.0, rng.uniform(0.7, 1.1), 0.62), jit=0.25, rng=rng)
        if len(placed) >= 12:
            break
    # Mũi Nghinh Phong: big granite boulders half in the sea round the southern tip
    cape = [(-34.0, -223.5, 3.2), (-27.5, -225.0, 2.6), (-21.0, -222.5, 2.9), (-40.5, -220.5, 2.2), (-15.5, -219.0, 2.0),
            (-30.5, -229.0, 1.8), (-24.0, -230.0, 1.4)]
    for x, y, r in cape:
        g.ico(rng.choice(['rock', 'rockDark']), (x, y, -0.9), r, sub=1, scale=(1.15, 0.95, 0.75), jit=0.28, rng=rng)
    kit.make_object(g, 'CX_NuiNho_Rocks', coll)
    return len(placed) + len(cape)


def run():
    kit.remove_objects(OBJECTS)
    inside = OUTLINE_WORLD.buffer(-0.5)
    shapely.prepare(inside)
    gone_trees = kit.drop_loose_parts('CX_Trees', lambda x, y: inside.contains(Point(x, y)))
    gone_rocks = kit.drop_loose_parts('IS_Boulders', lambda x, y: inside.contains(Point(x, y)))
    coll = kit.collection('SM_Mountains')
    _GROUND[0] = _ground_bvh()
    _, terrain = build_terrain(coll)
    build_road(kit.collection('SM_Streets'))
    build_stairs(coll)
    build_paths(coll)
    trees = build_trees(kit.collection('SM_Nature'), terrain)
    rocks = build_rocks(coll, terrain)
    for n in OBJECTS:
        kit.move_mesh(n, MX, MY)
    for n in ('CX_NuiNho_Trees', 'CX_NuiNho_Rocks', 'LF_nui_nho_Stairs', 'LF_NuiNho_Paths'):
        kit.add_wobble(n, 0.07)
    return {'trees': trees, 'rocks': rocks, 'old_trees_removed': gone_trees, 'old_boulders_removed': gone_rocks,
            'peak_n': HN, 'peak_s': HS, 'road_length': round(ROAD.length, 1), 'road_grade': round((ROAD_TOP - LY.ROAD_Z) / ROAD.length, 3)}


if __name__ == '__main__':
    print('NUI_NHO', run())
