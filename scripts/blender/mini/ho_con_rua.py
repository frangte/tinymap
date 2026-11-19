"""Hồ Con Rùa (công trường Quốc Tế, 1967) for Sài Gòn mini: docs/landmarks-v9-design.md, section 1.

What makes it read: a shallow octagonal pool with a white rim in the middle of the roundabout, two white ramps that
curl round each other over the water from the rim up to a round platform, and the slender tower of three shafts on
the platform, opening at the top into a flower of thin petals with a spire.

Walking it: four zebra crossings on the diagonals bring people over the ring road from the corners onto a paved walk
round the island; two paths cut through the lawn (north and south) to two short flights of steps up to the rim, where
each ramp starts. A ramp turns 200° and climbs 1.35 m (about 9 %) to the platform, with parapets on both sides. The
pool is 0.28 m deep, over collider.ts WADE (0.25): the water keeps bodies on the rim, out from under the ramps.

Objects: LM_HoConRua (rim, ramps, platform, tower, garden, benches, lamps), LM_HoConRua_Water (the water and the jets,
painted by src/game/water.ts) and IX_RoadMarks_HoConRua (the crossings). run() replaces the model appended from
models/sg.tinymap.blend (the layout entry still names it, so a full rebuild appends it and this replaces it).
"""
import math
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
if HERE not in sys.path:
    sys.path.insert(0, HERE)
import kit  # noqa: E402
import layout as LY  # noqa: E402
from mathutils import Matrix, Vector  # noqa: E402

PI = math.pi
for _k, _v in {'turq': '#5BA9A1', 'lotusTop': '#E6D29C', 'concreteLight': '#C9C4B8', 'lampGlow': '#FFE6A8'}.items():
    kit.PAL.setdefault(_k, _v)

LM = LY.landmarks['ho_con_rua']
CX, CY = LY.tb(LM['at'])
WALK = 0.32            # the paved walk round the island
LAWN = 0.38
RIM = WALK + 0.45      # rim top, ramp landings
FLOOR = 0.44           # the basin is raised: its floor
WATER = 0.72           # 0.28 deep, 5 cm under the rim
TOP = WALK + 1.8       # the platform
OCT_OUT, OCT_IN = 7.4, 6.75          # circumradii of the rim (flat sides face N, E, S, W and the diagonals)
R_WALK0, R_WALK1 = 8.6, 10.05        # the walk; the road's own paving ring runs on to r 11
R_PLAT = 2.9
HALF = 0.8                           # half width of a ramp
GAP = 1.45                           # half the tangential width of the landing cut into the rim
RAMPS = (90.0, 270.0)                # where each ramp starts (deg, Blender XY from +X)
STEP = 2.5                           # ramp sampling (deg)


def P(r, a_deg, z):
    a = math.radians(a_deg)
    return Vector((CX + r * math.cos(a), CY + r * math.sin(a), z))


def T(x, y, z):
    return Matrix.Translation((x, y, z))


def octagon(rc):
    return [(CX + rc * math.cos(math.radians(22.5 + 45 * k)), CY + rc * math.sin(math.radians(22.5 + 45 * k))) for k in range(8)]


def prism(g, mat, pts, z0, z1, side=None):
    """a convex polygon extruded from z0 to z1 (top in mat, walls in side)"""
    n = len(pts)
    V = [(x, y, z0) for x, y in pts] + [(x, y, z1) for x, y in pts]
    g.raw(mat, V, [tuple(range(n, 2 * n))])
    g.raw(side or mat, V, [tuple(range(n - 1, -1, -1))] + [(i, (i + 1) % n, n + (i + 1) % n, n + i) for i in range(n)])


def sector(g, mat, r0, r1, a0, a1, z0, z1, seg=None, side=None):
    """part of an annulus between angles a0..a1 (deg), as a closed solid"""
    seg = seg or max(1, int(abs(a1 - a0) / 7.5))
    for k in range(seg):
        b0 = a0 + (a1 - a0) * k / seg
        b1 = a0 + (a1 - a0) * (k + 1) / seg
        pts = [P(r0, b0, 0), P(r1, b0, 0), P(r1, b1, 0), P(r0, b1, 0)]
        prism(g, mat, [(p.x, p.y) for p in pts], z0, z1, side)


def ribbon(g, mat, path, half, v0, v1, lat=None):
    """kit.Mesh.ribbon with its faces turned outwards"""
    n0 = len(g.f)
    g.ribbon(mat, path, half, v0, v1, lat=lat)
    g.f[n0:] = [tuple(reversed(f)) for f in g.f[n0:]]


def ramp_r(t):
    """centreline radius at t degrees along a ramp (t from -10 to 205)"""
    if t <= 10:
        return 5.55
    if t <= 175:
        return 5.55 + (3.6 - 5.55) * (t - 10) / 165
    return 3.6 + (2.2 - 3.6) * (t - 175) / 30


def ramp_z(t):
    if t <= 10:
        return RIM
    if t <= 175:
        return RIM + (TOP - RIM) * (t - 10) / 165
    return TOP


def ramp(g, a0):
    ts = [-10 + STEP * k for k in range(int(215 / STEP) + 1)]

    def path(off, t0=-10, t1=205):
        return [P(ramp_r(t) + off, a0 + t, ramp_z(t)) for t in ts if t0 - 1e-6 <= t <= t1 + 1e-6]

    ribbon(g, 'concreteLight', path(0), HALF, -0.3, 0.0)
    # outer parapet (open where the steps come up at t = 0) with three turquoise grooves, a turquoise fascia under it
    ribbon(g, 'white', path(HALF - 0.08, 12, 205), 0.08, 0.0, 0.9)
    for v0 in (0.14, 0.42, 0.7):
        ribbon(g, 'turq', path(HALF + 0.01, 12, 205), 0.025, v0, v0 + 0.1)
    ribbon(g, 'turq', path(HALF + 0.01, -10, 205), 0.025, -0.28, -0.04)
    # inner parapet until the ramp runs onto the platform, and a short wall closing the low end
    ribbon(g, 'white', path(-HALF + 0.08, -10, 165), 0.08, 0.0, 0.9)
    a = math.radians(a0 - 10)
    t = Vector((-math.sin(a), math.cos(a), 0))
    q0, q1 = P(ramp_r(-10) - HALF, a0 - 10, RIM), P(ramp_r(-10) + HALF, a0 - 10, RIM)
    ribbon(g, 'white', [q0, q1], 0.08, 0.0, 0.9, lat=t)
    # square columns standing in the water
    for tc in range(20, 171, 25):
        c = P(ramp_r(tc), a0 + tc, 0)
        z = ramp_z(tc) - 0.3
        g.box('white', c.x - 0.18, c.y - 0.18, FLOOR, c.x + 0.18, c.y + 0.18, z)


def basin(g):
    out_, in_ = octagon(OCT_OUT), octagon(OCT_IN)
    for k in range(8):
        a, b = out_[k], out_[(k + 1) % 8]
        c, d = in_[(k + 1) % 8], in_[k]
        mid = 22.5 + 45 * k + 22.5          # direction this side faces
        if any(abs(((mid - r0) + 180) % 360 - 180) < 1 for r0 in RAMPS):
            # cut the landing out of the middle of this side: two short pieces of rim either side
            ab, dc = Vector((b[0] - a[0], b[1] - a[1], 0)), Vector((c[0] - d[0], c[1] - d[1], 0))
            for lo, hi in ((0.0, 0.5 - GAP / ab.length), (0.5 + GAP / ab.length, 1.0)):
                lo2 = 0.5 + (lo - 0.5) * ab.length / dc.length
                hi2 = 0.5 + (hi - 0.5) * ab.length / dc.length
                quad = [(a[0] + ab.x * lo, a[1] + ab.y * lo), (a[0] + ab.x * hi, a[1] + ab.y * hi),
                        (d[0] + dc.x * hi2, d[1] + dc.y * hi2), (d[0] + dc.x * lo2, d[1] + dc.y * lo2)]
                prism(g, 'white', quad, WALK - 0.1, RIM, 'turq')
        else:
            prism(g, 'white', [a, b, c, d], WALK - 0.1, RIM, 'turq')
    prism(g, 'turq', in_, WALK - 0.1, FLOOR)                    # the floor of the basin, light tiles
    for a0 in RAMPS:
        # the landing across the rim, and two steps down to the path through the lawn
        a = math.radians(a0)
        u, t = Vector((math.cos(a), math.sin(a), 0)), Vector((-math.sin(a), math.cos(a), 0))
        def box_rt(r0, r1, w, z1, mat):
            c0, c1 = Vector((CX, CY, 0)) + u * r0, Vector((CX, CY, 0)) + u * r1
            pts = [c0 - t * w, c1 - t * w, c1 + t * w, c0 + t * w]
            prism(g, mat, [(p.x, p.y) for p in pts], WALK - 0.1, z1, 'white')
        box_rt(5.6, OCT_OUT * math.cos(math.radians(22.5)), GAP + 0.02, RIM, 'concreteLight')
        box_rt(6.84, 7.27, 1.1, WALK + 0.3, 'concreteLight')
        box_rt(7.27, 7.7, 1.1, WALK + 0.15, 'concreteLight')
        box_rt(7.7, R_WALK0 + 0.05, 1.1, WALK, 'paving')


def tower(g):
    # the drum under the platform and the platform with its parapet (open where the ramps arrive)
    g.ring('white', T(CX, CY, FLOOR - 0.05), 1.6, TOP - 0.35 - FLOOR, 24)
    g.ring('concreteLight', T(CX, CY, TOP - 0.4), R_PLAT, 0.4, 40)
    g.ring('white', T(CX, CY, TOP - 0.42), R_PLAT + 0.04, 0.34, 40, caps=False)
    gaps = [(a0 + 160, a0 + 205) for a0 in RAMPS]
    for k in range(48):
        b0, b1 = 7.5 * k, 7.5 * (k + 1)
        mid = (b0 + b1) / 2
        if any(((mid - lo) % 360) < (hi - lo) for lo, hi in gaps):
            continue
        sector(g, 'white', R_PLAT - 0.17, R_PLAT, b0, b1, TOP, TOP + 0.9, seg=1)
    # three slender shafts, tapering, tied by two collars
    for k in range(3):
        a = math.radians(90 + 120 * k)
        u, t = Vector((math.cos(a), math.sin(a), 0)), Vector((-math.sin(a), math.cos(a), 0))
        V = []
        for z, rr, ru, rt in ((TOP - 0.02, 0.62, 0.3, 0.17), (14.0, 0.5, 0.15, 0.1)):
            c = Vector((CX, CY, z)) + u * rr
            V += [tuple(c - u * ru - t * rt), tuple(c + u * ru - t * rt), tuple(c + u * ru + t * rt), tuple(c - u * ru + t * rt)]
        g.raw('white', V, [(0, 3, 2, 1), (4, 5, 6, 7), (0, 1, 5, 4), (1, 2, 6, 5), (2, 3, 7, 6), (3, 0, 4, 7)])
    for z, r in ((6.4, 1.0), (10.4, 0.9)):
        g.ring('white', T(CX, CY, z), r, 0.22, 6)
    # the flower: a cup, 24 petals opening upwards, a spire
    g.lathe('lotusTop', T(CX, CY, 0), [(0.32, 13.3), (0.55, 13.8), (0.95, 14.25)], seg=12)
    for k in range(24):
        a0 = 15.0 * k
        lift = 0.0 if k % 2 == 0 else 0.25
        pts = []
        for r, w, z in ((0.6, 0.06, 14.0), (2.4, 0.22, 14.55 + lift), (3.3, 0.06, 14.85 + lift * 1.4)):
            a = math.radians(a0)
            u, t = Vector((math.cos(a), math.sin(a), 0)), Vector((-math.sin(a), math.cos(a), 0))
            c = Vector((CX, CY, z)) + u * r
            pts.append((c - t * w, c + t * w))
        V = []
        for lo, hi in pts:
            V += [tuple(lo), tuple(hi)]
        V += [tuple(Vector(v) + Vector((0, 0, -0.08))) for v in V]
        F = [(0, 2, 3, 1), (2, 4, 5, 3), (6, 7, 9, 8), (8, 9, 11, 10)]
        F += [(0, 6, 8, 2), (2, 8, 10, 4), (1, 3, 9, 7), (3, 5, 11, 9), (4, 10, 11, 5), (0, 1, 7, 6)]
        g.raw('lotusTop', V, F)
    g.rod('white', (CX, CY, 14.0), (CX, CY, 18.0), 0.09, 6)
    g.ico('lotusTop', (CX, CY, 18.05), 0.16, sub=1)


def garden(g):
    paths = [a0 for a0 in RAMPS]

    def on_path(a, half_deg):
        return any(abs(((a - p) + 180) % 360 - 180) < half_deg for p in paths)

    # the walk round the island and the lawn between it and the pool
    sector(g, 'paving', R_WALK0, R_WALK1, 0, 360, WALK - 0.12, WALK, seg=48, side='curb')
    lawn_in = OCT_OUT * math.cos(math.radians(22.5)) - 0.05
    for k in range(48):
        b0, b1 = 7.5 * k, 7.5 * (k + 1)
        if on_path((b0 + b1) / 2, 8.5):
            continue
        sector(g, 'grassDark', lawn_in - 0.6, R_WALK0, b0, b1, WALK - 0.1, LAWN, seg=1)
    # a clipped box hedge along the walk, and a bed of flowers in arcs of colour at the foot of the rim
    cols = ['flowerRed', 'leaf', 'flowerPink', 'leaf', 'flowerYellow', 'leaf', 'white', 'leaf']
    for k in range(32):
        b0, b1 = 11.25 * k, 11.25 * (k + 1)
        mid = (b0 + b1) / 2
        if on_path(mid, 10):
            continue
        sector(g, 'leafDark', R_WALK0 - 0.35, R_WALK0 - 0.05, b0 + 0.6, b1 - 0.6, LAWN, LAWN + 0.36, seg=1)
        sector(g, cols[k % 8], 7.35, 7.95, b0 + 0.4, b1 - 0.4, LAWN, LAWN + 0.2, seg=1)
    # benches on the walk facing the monument, lamps at the paths
    for a in (12, 68, 112, 168, 192, 248, 292, 348):
        rad = math.radians(a)
        M = T(CX + 9.55 * math.cos(rad), CY + 9.55 * math.sin(rad), WALK) @ Matrix.Rotation(rad, 4, 'Z')
        g.box('wood', -0.22, -0.8, 0.42, 0.22, 0.8, 0.48, M)
        g.box('wood', 0.2, -0.8, 0.48, 0.26, 0.8, 0.85, M)
        for s in (-0.65, 0.65):
            g.box('archDark', -0.18, s - 0.05, 0.0, 0.24, s + 0.05, 0.42, M)
    for a in (78, 102, 258, 282, 0, 180):
        c = P(9.6, a, WALK)
        g.ring('archDark', T(c.x, c.y, WALK), 0.07, 3.4, 6)
        g.ico('lampGlow', (c.x, c.y, WALK + 3.65), 0.26, sub=1)


def water():
    g = kit.Mesh(31)
    pts = octagon(OCT_IN - 0.01)
    g.raw('water', [(x, y, WATER) for x, y in pts], [tuple(range(8))])
    # low jets round the drum: a thin column and a ring of splash each
    for k in range(8):
        c = P(2.25, 22.5 + 45 * k, WATER)
        g.ring('foam', T(c.x, c.y, WATER - 0.02), 0.1, 0.75, 6, r2=0.03)
        g.ring('foam', T(c.x, c.y, WATER - 0.02), 0.32, 0.05, 8, r2=0.24)
    return g


def zebras():
    g = kit.Mesh(32)
    for a in (45, 135, 225, 315):
        rad = math.radians(a)
        M0 = Matrix.Rotation(rad, 4, 'Z')
        for r in (11.6, 12.5, 13.4, 14.3, 15.2):
            M = T(CX + r * math.cos(rad), CY + r * math.sin(rad), LY.ROAD_Z) @ M0
            g.box('roadLine', -0.225, -1.4, 0.0, 0.225, 1.4, 0.05, M)
    return g


def build():
    g = kit.Mesh(30)
    basin(g)
    for a0 in RAMPS:
        ramp(g, a0)
    tower(g)
    garden(g)
    return g


def run():
    kit.remove_objects(['LM_HoConRua', 'LM_HoConRua_Water', 'IX_RoadMarks_HoConRua'])
    coll = kit.collection('SM_Landmarks')
    ob = kit.make_object(build(), 'LM_HoConRua', coll)
    kit.make_object(water(), 'LM_HoConRua_Water', coll)
    streets = bpy_coll('SM_Streets') or coll
    kit.make_object(zebras(), 'IX_RoadMarks_HoConRua', streets)
    kit.add_wobble('LM_HoConRua', 0.02)
    trees = kit.drop_loose_parts('CX_Trees', lambda x, y: (x - CX) ** 2 + (y - CY) ** 2 < R_WALK1 ** 2)
    ob.data.calc_loop_triangles()
    return {'object': ob.name, 'tris': len(ob.data.loop_triangles), 'tree_parts_removed': trees, 'centre': (round(CX, 1), round(CY, 1))}


def bpy_coll(name):
    import bpy
    return bpy.data.collections.get(name)


if __name__ == '__main__':
    print('HO_CON_RUA', run())
    # alone on the map: blender models/saigon_mini.blend --background --python scripts/blender/mini/ho_con_rua.py -- --save
    if '--save' in (sys.argv[sys.argv.index('--') + 1:] if '--' in sys.argv else []):
        import bpy
        bpy.ops.wm.save_mainfile(compress=True)
