"""The grounds of Dinh Độc Lập for Sài Gòn mini: docs/landmarks-v9-design.md, section 3 (approved 2026-11-18).

A fence round the grounds (white pillars every 3 m on a low wall, dark iron railings), the gates on the park 30/4 side
as on 30 April 1975: tank 390 has knocked the main gate down and stands in it, its leaves crushed and leaning, so the
main gate is shut to walkers; tank 843 is stuck in the side gate on the east. Two small gates right beside the main
gate stand open, with a sentry box each, and the back gate on the south is open: those are the ways in. Inside, the
houses and yard beds that the plan had put on the east side go; short paths join the small gates to the drive round
the fountain and the back gate to the walk round the palace.

Objects: LM_DinhGrounds (fence, gates, sentry boxes), LM_DinhTanks (the two tanks) and LF_DinhGrounds (the paths).
Run after dinh_thong_nhat.py (MINI_SCRIPTS); then clean_trees.py takes out any tree the fence or a tank now stands in.
"""
import math
import os
import sys

import bpy

HERE = os.path.dirname(os.path.abspath(__file__))
if HERE not in sys.path:
    sys.path.insert(0, HERE)
import kit  # noqa: E402
from mathutils import Matrix, Vector  # noqa: E402
from mathutils.bvhtree import BVHTree  # noqa: E402

PI = math.pi
kit.PAL.setdefault('tankGreen', '#56653F')
kit.PAL.setdefault('archDark', '#3B3330')
# iron railings and gate leaves: the colour of archDark, a key of their own so the game camera looks through them
kit.PAL.setdefault('railing', '#3B3330')

# the fence line (Blender XY): just inside the pavements on the west, south and east, across the park on the north
X0, X1, Y0, Y1 = -143.2, -87.6, 3.6, 66.5
AXIS = -111.0                          # the axis of the palace, the fountain and the park avenue
MAIN = (-114.5, -107.5)                # the main gate opening (between its two big pillars)
PED_W = (-117.4, -115.5)               # the small gates beside it
PED_E = (-106.5, -104.6)
SIDE_E = (-97.6, -93.4)                # the side gate on the east, where 843 got stuck
BACK = (-113.0, -109.0)                # the back gate on the south
BAY = 3.0
BASE, RAIL = 0.72, 2.1                 # top of the low wall (a body steps up 0.5 m at most), top rail
BAR = 0.4                              # railing bar spacing: too tight for a body (player radius 0.26)


def T(x, y, z=0.0):
    return Matrix.Translation((x, y, z))


def Rz(a):
    return Matrix.Rotation(a, 4, 'Z')


# ------------------------------------------------------------------------------------------------ fence and gates
def pillar(g, x, y, w=0.5, h=2.35, cap='stone'):
    g.box('opWhite', x - w / 2, y - w / 2, -0.2, x + w / 2, y + w / 2, h)
    g.box(cap, x - w / 2 - 0.07, y - w / 2 - 0.07, h, x + w / 2 + 0.07, y + w / 2 + 0.07, h + 0.12)


def railing(g, a, b, M=None, base=True, top=RAIL, z0=0.0):
    """a straight run of wall and railing from a to b (XY), between pillars; in a local frame when M is given"""
    a, b = Vector((a[0], a[1], 0)), Vector((b[0], b[1], 0))
    d = b - a
    L = d.length
    if L < 0.05:
        return
    u = d / L
    n = Vector((-u.y, u.x, 0))
    R = Matrix(((u.x, n.x, 0, a.x), (u.y, n.y, 0, a.y), (0, 0, 1, z0), (0, 0, 0, 1)))
    if M is not None:
        R = M @ R
    if base:
        g.box('opWhite', 0, -0.18, -0.2, L, 0.18, BASE, R)
        g.box('stone', 0, -0.22, BASE, L, 0.22, BASE + 0.08, R)
    lo = BASE + 0.08 if base else 0.0
    for z in (lo + 0.12, top - 0.05):
        g.box('railing', 0, -0.03, z - 0.03, L, 0.03, z + 0.03, R)
    k = max(1, round(L / BAR))
    for i in range(1, k):
        x = L * i / k
        g.box('railing', x - 0.025, -0.025, lo, x + 0.025, 0.025, top + 0.12, R)


def fence_run(g, a, b, gaps=()):
    """pillars every BAY metres from a to b, railing between, leaving the gaps [(s0, s1) metres along] open"""
    a, b = Vector((a[0], a[1], 0)), Vector((b[0], b[1], 0))
    L = (b - a).length
    u = (b - a) / L
    cuts = [0.0]
    for s0, s1 in sorted(gaps):
        cuts += [s0, s1]
    cuts.append(L)
    for k in range(0, len(cuts), 2):
        s0, s1 = cuts[k], cuts[k + 1]
        if s1 - s0 < 0.3:
            continue
        n = max(1, round((s1 - s0) / BAY))
        for i in range(n + 1):
            s = s0 + (s1 - s0) * i / n
            p = a + u * s
            pillar(g, p.x, p.y)
            if i < n:
                q = a + u * (s0 + (s1 - s0) * (i + 1) / n)
                railing(g, (p + u * 0.25).xy, (q - u * 0.25).xy)


def leaf(g, M, w, h=2.0):
    """a gate leaf in its own frame: hinge on local z, leaf along +x (w long), standing h tall"""
    g.box('railing', 0, -0.05, 0.0, w, 0.05, 0.1, M)
    g.box('railing', 0, -0.05, h - 0.1, w, 0.05, h, M)
    g.box('railing', 0, -0.05, 0.0, 0.1, 0.05, h, M)
    g.box('railing', w - 0.1, -0.05, 0.0, w, 0.05, h, M)
    g.box('railing', 0, -0.04, h * 0.45, w, 0.04, h * 0.45 + 0.08, M)
    k = max(2, round(w / 0.22))
    for i in range(1, k):
        x = w * i / k
        g.box('railing', x - 0.022, -0.022, 0.0, x + 0.022, 0.022, h + 0.14, M)
    # the round emblem in the middle of the old gates
    g.ring('signGold', M @ T(w / 2, 0.0, h * 0.62) @ Matrix.Rotation(PI / 2, 4, 'X'), 0.28, 0.06, 12)


def sentry(g, x, y, face):
    """a small sentry box, its door towards `face` (rad)"""
    M = T(x, y, 0) @ Rz(face)
    g.box('opWhite', -0.75, -0.75, -0.1, 0.75, 0.75, 2.4, M)
    g.box('stone', -0.95, -0.95, 2.4, 0.95, 0.95, 2.6, M)
    g.box('archDark', 0.75, -0.35, 0.0, 0.78, 0.35, 2.0, M)
    g.box('glassDark', -0.4, 0.75, 1.2, 0.4, 0.78, 1.9, M)
    g.box('glassDark', -0.4, -0.78, 1.2, 0.4, -0.75, 1.9, M)


def gates(g):
    # main gate: two big pillars with lamps; tank 390 stands in the opening (tank()), the leaves lean crushed beside it
    for x in (MAIN[0] - 0.5, MAIN[1] + 0.5):
        g.box('opWhite', x - 0.5, Y1 - 0.5, -0.2, x + 0.5, Y1 + 0.5, 2.9)
        g.box('stone', x - 0.62, Y1 - 0.62, 2.9, x + 0.62, Y1 + 0.62, 3.1)
        g.ring('archDark', T(x, Y1, 3.1), 0.08, 0.5, 6)
        g.ico('lampGlow', (x, Y1, 3.85), 0.28, sub=1)
    # west leaf: torn round on its hinge and pushed over, leaning in against the tank
    M = T(MAIN[0], Y1 - 0.1, 0) @ Rz(-0.42) @ Matrix.Rotation(0.5, 4, 'X')
    leaf(g, M, 2.3)
    # east leaf: crumpled, leaning on the east pillar and the tank
    M = T(MAIN[1], Y1 - 0.1, 0) @ Rz(PI + 0.38) @ Matrix.Rotation(-0.5, 4, 'X')
    leaf(g, M, 2.2)
    # a third piece flat under the tank's tracks
    M = T(-112.8, 62.6, 0.08) @ Rz(0.3) @ Matrix.Rotation(PI / 2, 4, 'X')
    leaf(g, M, 3.0, 1.9)
    # small gates beside the main gate: open, leaves swung in against the fence
    for (a, b), sgn in ((PED_W, -1), (PED_E, 1)):
        far = a if sgn < 0 else b
        pillar(g, far + sgn * 0.3, Y1, w=0.6, h=2.6)
        hinge = far
        M = T(hinge, Y1 - 0.25, 0) @ Rz(-PI / 2 - (0.12 if sgn < 0 else -0.12))
        leaf(g, M, 1.0, 1.9)
    sentry(g, PED_W[0] - 3.6, Y1 - 2.0, 0.0)
    sentry(g, PED_E[1] + 3.6, Y1 - 2.0, PI)
    # east side gate with 843 stuck in it: pillars, one leaf bent over the tank's front, one hanging open
    for x in (SIDE_E[0] - 0.3, SIDE_E[1] + 0.3):
        pillar(g, x, Y1, w=0.6, h=2.6)
    M = T(SIDE_E[1], Y1 - 0.1, 0) @ Rz(PI + 0.55) @ Matrix.Rotation(-0.6, 4, 'X')
    leaf(g, M, 1.9)
    # back gate: open, both leaves swung in
    for x in (BACK[0] - 0.3, BACK[1] + 0.3):
        pillar(g, x, Y0, w=0.6, h=2.6)
    leaf(g, T(BACK[0], Y0 + 0.25, 0) @ Rz(PI / 2 - 0.1), 2.0)
    leaf(g, T(BACK[1], Y0 + 0.25, 0) @ Rz(PI / 2 + 0.1), 2.0)


def fence(g):
    north_gaps = [(MAIN[0] - 1.0 - X0, MAIN[1] + 1.0 - X0), (PED_W[0] - 0.85 - X0, PED_W[1] - X0), (PED_E[0] - X0, PED_E[1] + 0.85 - X0),
                  (SIDE_E[0] - 0.85 - X0, SIDE_E[1] + 0.85 - X0)]
    # the main gate's big pillars close the small gates on their inner side: merge the overlapping gaps
    north_gaps = merge(north_gaps)
    fence_run(g, (X0, Y1), (X1, Y1), north_gaps)
    fence_run(g, (X0, Y0), (X1, Y0), [(BACK[0] - 0.85 - X0, BACK[1] + 0.85 - X0)])
    fence_run(g, (X0, Y0), (X0, Y1))
    fence_run(g, (X1, Y0), (X1, Y1))


def merge(gaps):
    out = []
    for a, b in sorted(gaps):
        if out and a <= out[-1][1] + 0.05:
            out[-1] = (out[-1][0], max(out[-1][1], b))
        else:
            out.append((a, b))
    return out


# ------------------------------------------------------------------------------------------------ the tanks
def _profile(g, mat, pts, y0, y1, M):
    """a side profile [(x, z)] extruded across y0..y1"""
    n = len(pts)
    V = [(x, y0, z) for x, z in pts] + [(x, y1, z) for x, z in pts]
    # profile points go anticlockwise seen from -y
    g.raw(mat, V, [tuple(range(n)), tuple(range(2 * n - 1, n - 1, -1))] + [(i, n + i, n + (i + 1) % n, (i + 1) % n) for i in range(n)], M)


DIGITS = {'0': 'abcdef', '1': 'bc', '2': 'abged', '3': 'abgcd', '4': 'fgbc', '5': 'afgcd', '6': 'afgedc', '7': 'abc', '8': 'abcdefg', '9': 'abcdfg'}


def number(g, M, text, x0, z0, h=0.34, side=1):
    """seven-segment white digits on a plane at local y = side * 1.18 (the turret side), reading along x"""
    w, t = h * 0.55, 0.05
    y = side * 1.19
    for k, ch in enumerate(text):
        cx = x0 + (k * (w + 0.1)) * (-side)
        segs = {'a': (0, h, w, h), 'b': (w, h / 2, w, h), 'c': (w, 0, w, h / 2), 'd': (0, 0, w, 0), 'e': (0, 0, 0, h / 2),
                'f': (0, h / 2, 0, h), 'g': (0, h / 2, w, h / 2)}
        for s in DIGITS[ch]:
            xa, za, xb, zb = segs[s]
            xa, xb = cx - side * xa, cx - side * xb
            g.box('white', min(xa, xb) - t / 2, y - 0.01, z0 + min(za, zb) - t / 2, max(xa, xb) + t / 2, y + 0.01, z0 + max(za, zb) + t / 2, M)


def tank(g, M, num):
    """a T-54 / Type 59: 6.45 m long, 3.27 m wide, 2.4 m to the turret top. Local +x forward."""
    for s in (-1, 1):
        y0, y1 = sorted((s * 1.0, s * 1.635))
        _profile(g, 'archDark', [(-3.2, 0.25), (-2.9, 0.0), (2.9, 0.0), (3.25, 0.4), (3.0, 0.85), (-3.15, 0.85)], y0, y1, M)
        for x in (-2.3, -1.15, 0.0, 1.15, 2.3):
            g.ring('tankGreen', M @ T(x, s * 1.66, 0.42) @ Matrix.Rotation(-s * PI / 2, 4, 'X'), 0.4, 0.06, 10)
        # fender with stowage boxes
        g.box('tankGreen', -3.1, s * 1.0, 0.85, 2.6, s * 1.64, 0.95, M)
        for x0, x1 in ((-2.6, -1.4), (-1.1, 0.1), (0.5, 1.6)):
            g.box('tankGreen', x0, s * 1.2, 0.95, x1, s * 1.6, 1.3, M)
    g.box('tankGreen', -3.0, -1.0, 0.75, 2.4, 1.0, 1.42, M)
    _profile(g, 'tankGreen', [(2.4, 0.75), (3.25, 0.75), (3.25, 0.85), (2.4, 1.42)], -1.0, 1.0, M)
    # turret, cupola, gun with the bore evacuator, two drums on the back
    g.lathe('tankGreen', M @ T(0.35, 0, 0), [(1.15, 1.42), (1.12, 1.72), (0.9, 2.05), (0.45, 2.27), (0.05, 2.32)], seg=16)
    g.ring('tankGreen', M @ T(0.0, 0.45, 2.2), 0.32, 0.18, 8)
    g.ring('tankGreen', M @ T(1.35, 0, 1.86) @ Matrix.Rotation(PI / 2, 4, 'Y'), 0.26, 0.35, 8)
    a, b = M @ Vector((1.6, 0, 1.88)), M @ Vector((5.0, 0, 1.94))
    g.rod('tankGreen', a, b, 0.085, 8)
    g.ring('tankGreen', M @ T(4.25, 0, 1.925) @ Matrix.Rotation(PI / 2, 4, 'Y'), 0.13, 0.4, 8)
    for s in (-1, 1):
        g.ring('tankGreen', M @ T(-3.15, s * 0.9 - 0.45, 1.05) @ Matrix.Rotation(-PI / 2, 4, 'X'), 0.3, 0.9, 10)
        number(g, M, num, 0.75 if s > 0 else -0.05, 1.42, side=s)


def tanks():
    g = kit.Mesh(41)
    # 390 in the main gate, swung a little as it drove over the leaves, heading for the palace
    tank(g, T(-110.6, 64.5, 0.06) @ Rz(-PI / 2 + 0.21), '390')
    # 843 stuck in the east side gate
    tank(g, T(-95.4, 66.2, 0.0) @ Rz(-PI / 2 + 0.35), '843')
    return g


# ------------------------------------------------------------------------------------------------ paths
def ground_bvh():
    dg = bpy.context.evaluated_depsgraph_get()
    V, F = [], []
    for name in ('IS_Ground', 'IX_Roads', 'LF_DinhThongNhat'):
        ob = bpy.data.objects.get(name)
        if ob is None:
            continue
        me = ob.evaluated_get(dg).to_mesh()
        b = len(V)
        V += [ob.matrix_world @ v.co for v in me.vertices]
        F += [tuple(b + i for i in p.vertices) for p in me.polygons]
        ob.evaluated_get(dg).to_mesh_clear()
    return BVHTree.FromPolygons(V, F)


def path(g, bvh, a, b, w, lift=0.05):
    """a paved path from a to b, w wide, draped over the ground every 0.5 m"""
    a, b = Vector((a[0], a[1], 0)), Vector((b[0], b[1], 0))
    L = (b - a).length
    u = (b - a) / L
    n = Vector((-u.y, u.x, 0))
    k = max(1, int(L / 0.5))

    def gz(p):
        hit = bvh.ray_cast(Vector((p.x, p.y, 20)), Vector((0, 0, -1)))
        return hit[0].z if hit[0] is not None else 0.0
    for i in range(k):
        p0, p1 = a + u * (L * i / k), a + u * (L * (i + 1) / k)
        z = max(gz(p0 + n * s) for s in (-w / 2, 0, w / 2)) + lift
        z = max(z, max(gz(p1 + n * s) for s in (-w / 2, 0, w / 2)) + lift)
        q = [p0 - n * w / 2, p1 - n * w / 2, p1 + n * w / 2, p0 + n * w / 2]
        V = [(v.x, v.y, z - 0.25) for v in q] + [(v.x, v.y, z) for v in q]
        g.raw('paving', V, [(0, 3, 2, 1), (4, 5, 6, 7), (0, 1, 5, 4), (1, 2, 6, 5), (2, 3, 7, 6), (3, 0, 4, 7)])


def paths():
    g = kit.Mesh(42)
    bvh = ground_bvh()
    fx, fy, fr = AXIS, 54.0, 10.3          # the drive round the fountain (dinh_thong_nhat.forecourt)
    for a, b in (PED_W, PED_E):
        cx = (a + b) / 2
        d = Vector((cx - fx, Y1 - fy))
        end = Vector((fx, fy)) + d.normalized() * (fr - 0.6)
        path(g, bvh, (cx, Y1 + 2.4), (end.x, end.y), 1.9)
    path(g, bvh, (AXIS, Y0 - 1.0), (AXIS, 12.6), 3.0)
    # outside the main gate: a paved apron joining the park avenue to the small gates
    path(g, bvh, (PED_W[0] - 0.6, Y1 + 1.4), (PED_E[1] + 0.6, Y1 + 1.4), 2.2, lift=0.03)
    return g


def run():
    inside = lambda x, y: X0 < x < X1 and Y0 < y < Y1   # noqa: E731
    # the houses the plan put in the north-east corner run on past the fence line to the road (x -83.2): all of them go
    ne_corner = lambda x, y: X1 <= x < -83.0 and 44.0 < y < Y1 + 1.0   # noqa: E731
    gone = {n: kit.drop_loose_parts(n, lambda x, y: inside(x, y) or ne_corner(x, y)) for n in ('SH_np', 'SH_Yards', 'CX_YardTrees')}
    kit.remove_objects(['LM_DinhGrounds', 'LM_DinhTanks', 'LF_DinhGrounds'])
    coll = kit.collection('SM_Landmarks')
    g = kit.Mesh(40)
    fence(g)
    gates(g)
    ob = kit.make_object(g, 'LM_DinhGrounds', coll)
    kit.make_object(tanks(), 'LM_DinhTanks', coll)
    kit.make_object(paths(), 'LF_DinhGrounds', coll)
    # trees standing on the fence line or in a gate
    near_fence = lambda x, y: ((abs(x - X0) < 1.2 or abs(x - X1) < 1.2) and Y0 - 1 < y < Y1 + 1) or \
        ((abs(y - Y0) < 1.2 or abs(y - Y1) < 1.2) and X0 - 1 < x < X1 + 1) or \
        (MAIN[0] - 6 < x < MAIN[1] + 6 and Y1 - 8 < y < Y1 + 4) or (SIDE_E[0] - 2 < x < SIDE_E[1] + 2 and Y1 - 5 < y < Y1 + 6)  # noqa: E731
    gone['CX_Trees'] = kit.drop_loose_parts('CX_Trees', near_fence)
    kit.add_wobble('LM_DinhGrounds', 0.02)
    ob.data.calc_loop_triangles()
    return {'object': ob.name, 'tris': len(ob.data.loop_triangles), 'removed': gone}


if __name__ == '__main__':
    print('DINH_GROUNDS', run())
    # alone on the map: blender models/saigon_mini.blend --background --python scripts/blender/mini/dinh_grounds.py -- --save
    if '--save' in (sys.argv[sys.argv.index('--') + 1:] if '--' in sys.argv else []):
        bpy.ops.wm.save_mainfile(compress=True)
