"""Bến Nhà Rồng (1863, now the Hồ Chí Minh Museum) for Sài Gòn mini: docs/landmarks-v9-design.md, section 2.

What makes it read: a two-storey French house on the tip of Khánh Hội between the Bến Nghé canal and the Saigon river,
its long front facing the river: an arcade of round arches on the ground floor, a loggia of columns and balusters
above, a raised centre with a pediment, two end pavilions with flat roofs behind balustrades, a red tiled roof, and on
its ridge the two green dragons, turned outwards since 1955, with the emblem (a horse's head and an anchor) between
them. In front: a quay pushed 3 m out into the river with a railing and bollards, the statue of Nguyễn Tất Thành, pots
of bonsai and two flagpoles; moored downstream, a model of the steamer Amiral Latouche-Tréville.

Walls are wallSalmon (the coral of the photos), trim opWhite / white, roof roofRed, dragons poGreen.

The frame: the river bank of the cape runs from A along d (north-west to south-east); n points out over the river. A
point is (s metres along the bank, i metres inland). The facade stands at i = 6, from s = 10 to 32, clear of the foot
bridge that lands at s 5.

Objects: LM_NhaRong (the house), LF_NhaRong (quay, statue, pots, flags, lamps), LM_NhaRong_Ship. (The Thủ Ngữ flag
tower across the canal, LM_CotCoThuNgu, was taken out again on 2026-11-19; run() still removes it from older files.)
run() takes the old blockout model off LM_NewBuildings (built from build_saigon_mini.py before v9).
"""
import math
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
if HERE not in sys.path:
    sys.path.insert(0, HERE)
import kit  # noqa: E402
from mathutils import Matrix, Vector  # noqa: E402

PI = math.pi
for _k, _v in {'wallSalmon': '#EBA78A', 'bronze': '#7B5B3C', 'roofRedDark': '#7D3326', 'poGreen': '#3E7F5F',
               'lampGlow': '#FFE6A8',
               'railing': '#3B3330'}.items():   # railing: archDark's colour, a key the game camera looks through
    kit.PAL.setdefault(_k, _v)

A = Vector((57.0, -9.0, 0.0))
D = Vector((0.596, -0.803, 0.0))       # along the bank, downstream
N = Vector((0.803, 0.596, 0.0))        # out over the river
GROUND = 0.18                          # the paving of the cape
PZ = 0.7                               # top of the podium, the floor of the arcade
F2 = 4.9                               # first floor
EAVE = 8.65
RIDGE = 11.3
HW = 11.0                              # half the length of the house
WING = 8.0                             # where the end pavilions start
CEN = 3.2                              # half width of the raised centre
QUAY_I = -3.0                          # the new quay edge (metres inland: out over the river)


def W(s, i, z=0.0):
    p = A + D * s - N * i
    return Vector((p.x, p.y, z))


def frame(s, i, z=0.0):
    """local frame: x along the facade to the north-west, y inland, z up; the facade faces -y, out over the river"""
    o = W(s, i, z)
    x, y = -D, -N
    return Matrix(((x.x, y.x, 0, o.x), (x.y, y.y, 0, o.y), (0, 0, 1, o.z), (0, 0, 0, 1)))


MB = frame(21.0, 6.0)                  # the house: facade centre


def T(x, y, z=0.0):
    return Matrix.Translation((x, y, z))


def xz(g, mat, pts, y0, y1, M):
    """a convex polygon in the x-z plane [(x, z)], anticlockwise seen from -y, extruded from y0 to y1"""
    n = len(pts)
    V = [(x, y0, z) for x, z in pts] + [(x, y1, z) for x, z in pts]
    g.raw(mat, V, [tuple(range(n)), tuple(range(2 * n - 1, n - 1, -1))] + [(i, n + i, n + (i + 1) % n, (i + 1) % n) for i in range(n)], M)


def ccw(pts):
    a = sum(pts[i][0] * pts[(i + 1) % len(pts)][1] - pts[(i + 1) % len(pts)][0] * pts[i][1] for i in range(len(pts)))
    return pts if a > 0 else pts[::-1]


def prism(g, mat, pts, z0, z1, side=None):
    pts = ccw(pts)
    n = len(pts)
    V = [(x, y, z0) for x, y in pts] + [(x, y, z1) for x, y in pts]
    g.raw(mat, V, [tuple(range(n, 2 * n))])
    g.raw(side or mat, V, [tuple(range(n - 1, -1, -1))] + [(i, (i + 1) % n, n + (i + 1) % n, n + i) for i in range(n)])


# ------------------------------------------------------------------------------------------------ the house
def arch_bay(g, x0, x1, y0, y1, zb, zs, zt, mat='wallSalmon', M=MB):
    """the wall of one bay with a round-arched opening: two piers, the arch filled up to zt in thin slices"""
    w = x1 - x0
    ow = w * 0.64
    xa, xb = x0 + (w - ow) / 2, x1 - (w - ow) / 2
    r, xc = ow / 2, (x0 + x1) / 2
    g.box(mat, x0, y0, zb, xa, y1, zt, M)
    g.box(mat, xb, y0, zb, x1, y1, zt, M)
    k = 8
    for j in range(k):
        u0, u1 = xa + ow * j / k, xa + ow * (j + 1) / k
        z0 = zs + math.sqrt(max(0.0, r * r - (u0 - xc) ** 2))
        z1 = zs + math.sqrt(max(0.0, r * r - (u1 - xc) ** 2))
        xz(g, mat, [(u0, z0), (u1, z1), (u1, zt), (u0, zt)], y0, y1, M)
    # white archivolt keystone and impost bands
    g.box('opWhite', xc - 0.14, y0 - 0.04, zs + r - 0.05, xc + 0.14, y0, zs + r + 0.35, M)
    for xx in (xa, xb):
        g.box('opWhite', xx - 0.12, y0 - 0.04, zs - 0.12, xx + 0.12, y0, zs, M)


def balustrade(g, x0, x1, y, z0, h=0.85, M=MB, step=0.32):
    g.box('white', x0, y - 0.14, z0, x1, y + 0.14, z0 + 0.12, M)
    g.box('white', x0, y - 0.16, z0 + h - 0.1, x1, y + 0.16, z0 + h, M)
    n = max(1, round((x1 - x0) / step))
    for k in range(n):
        x = x0 + (x1 - x0) * (k + 0.5) / n
        g.box('white', x - 0.06, y - 0.06, z0 + 0.12, x + 0.06, y + 0.06, z0 + h - 0.1, M)


def window(g, x, y, z0, z1, w=1.1, face=-1, M=MB, axis='x'):
    """a framed window on a wall facing -y (face -1) / +y, or on an end wall facing -x / +x (axis 'y')"""
    t = 0.06
    if axis == 'x':
        yf = y + face * t
        g.box('opWhite', x - w / 2 - 0.14, min(y, yf), z0 - 0.16, x + w / 2 + 0.14, max(y, yf), z1 + 0.22, M)
        g.box('glassDark', x - w / 2, min(y, yf + face * 0.02), z0, x + w / 2, max(y, yf + face * 0.02), z1, M)
    else:
        xf = x + face * t
        g.box('opWhite', min(x, xf), y - w / 2 - 0.14, z0 - 0.16, max(x, xf), y + w / 2 + 0.14, z1 + 0.22, M)
        g.box('glassDark', min(x, xf + face * 0.02), y - w / 2, z0, max(x, xf + face * 0.02), y + w / 2, z1, M)


def hip(g, mat, x0, x1, y0, y1, ze, zr, ridge_half, M=MB):
    ym = (y0 + y1) / 2
    e = [(x0, y0, ze), (x1, y0, ze), (x1, y1, ze), (x0, y1, ze)]
    ra, rb = (-ridge_half, ym, zr), (ridge_half, ym, zr)
    V = e + [ra, rb]
    g.raw(mat, V, [(0, 1, 5, 4), (2, 3, 4, 5), (3, 0, 4), (1, 2, 5), (0, 3, 2, 1)], M)


def dragon(g, s, zr, M):
    """a glazed green dragon on the ridge, its tail by the emblem, its head turned outwards (s = +1 north-west)"""
    pts = []
    for k in range(10):
        t = k / 9
        x = s * (0.95 + 3.6 * t)
        z = zr + 0.4 + 0.5 * math.sin(t * 2.6 * PI) * (0.4 + 0.6 * t) + 0.7 * t * t
        pts.append(M @ Vector((x, 0.0, z)))
    for k in range(9):
        g.rod('poGreen', pts[k], pts[k + 1], 0.14 + 0.11 * k / 8, 6)
        g.ico('poGreen', tuple(pts[k + 1]), 0.15 + 0.11 * k / 8, sub=1)
        # a gold fin on the back of every joint
        a = M.inverted() @ pts[k + 1]
        xz(g, 'gold', [(a.x - 0.16, a.z + 0.14), (a.x + 0.16, a.z + 0.14), (a.x + s * 0.03, a.z + 0.52)], -0.04, 0.04, M)
    # legs down to the ridge
    for k in (3, 7):
        a = M.inverted() @ pts[k]
        g.rod('poGreen', pts[k], M @ Vector((a.x + s * 0.25, 0.0, zr + 0.02)), 0.09, 5)
    # the head: snout, jaw, horns; and a curl of tail
    h = M.inverted() @ pts[9]
    hx = h.x
    g.box('poGreen', min(hx, hx + s * 1.05), -0.3, h.z - 0.05, max(hx, hx + s * 1.05), 0.3, h.z + 0.48, M)
    g.box('poGreen', min(hx, hx + s * 0.85), -0.24, h.z - 0.42, max(hx, hx + s * 0.85), 0.24, h.z - 0.14, M)
    for yy in (-0.15, 0.15):
        g.rod('gold', M @ Vector((hx + s * 0.15, yy, h.z + 0.42)), M @ Vector((hx - s * 0.5, yy * 2.0, h.z + 1.05)), 0.05, 4)
    g.ico('gold', tuple(M @ Vector((hx + s * 0.78, -0.31, h.z + 0.3))), 0.08, sub=0)
    g.ico('gold', tuple(M @ Vector((hx + s * 0.78, 0.31, h.z + 0.3))), 0.08, sub=0)
    a = M.inverted() @ pts[0]
    g.rod('poGreen', pts[0], M @ Vector((a.x - s * 0.15, 0.0, a.z + 0.45)), 0.08, 5)
    g.ico('poGreen', tuple(M @ Vector((a.x - s * 0.05, 0.0, a.z + 0.55))), 0.1, sub=1)


def house():
    g = kit.Mesh(51)
    M = MB
    # podium, the central steps
    g.box('stone', -HW - 0.4, -0.4, GROUND - 0.25, HW + 0.4, 11.4, PZ, M)
    g.box('stone', -CEN - 0.4, -1.4, GROUND - 0.25, CEN + 0.4, -0.4, PZ, M)
    for k in range(3):
        g.box('stone', -CEN + 0.2, -1.4 - 0.45 * (k + 1), GROUND - 0.25, CEN - 0.2, -1.4 - 0.45 * k, PZ - 0.17 * (k + 1), M)
    # the core behind the arcade and the loggia
    g.box('wallSalmon', -WING, 2.2, PZ, WING, 11.0, EAVE, M)
    # end pavilions: two storeys, flat roof behind a balustrade
    for sx in (-1, 1):
        x0, x1 = sorted((sx * WING, sx * HW))
        g.box('wallSalmon', x0, -0.6, GROUND - 0.2, x1, 11.0, 10.0, M)
        for xq in (x0, x1):                                         # quoins
            for yq in (-0.6, 11.0):
                g.box('opWhite', xq - 0.2, yq - 0.2, GROUND - 0.2, xq + 0.2, yq + 0.2, 10.0, M)
        g.box('opWhite', x0 - 0.12, -0.75, F2 - 0.1, x1 + 0.12, 11.15, F2 + 0.2, M)
        g.box('opWhite', x0 - 0.18, -0.8, 9.6, x1 + 0.18, 11.2, 10.0, M)
        xc = (x0 + x1) / 2
        for z0, z1 in ((1.7, 3.9), (5.7, 7.9)):
            window(g, xc, -0.6, z0, z1, w=1.2)
            window(g, xc, 11.0, z0, z1, w=1.2, face=1)
            for yy in (2.4, 5.4, 8.4):
                window(g, sx * HW, yy, z0, z1, w=1.0, face=sx, axis='y')
        for a, b in (((x0, -0.7), (x1, -0.7)), ((x0, 11.1), (x1, 11.1))):
            balustrade(g, a[0] + 0.25, b[0] - 0.25, a[1], 10.0, 0.75)
        for xx in (x0 - 0.1, x1 + 0.1):
            balustrade(g, -0.45, 10.85, 0.0, 10.0, 0.75, M=M @ T(xx, 0, 0) @ Matrix.Rotation(PI / 2, 4, 'Z'))
    # ground floor: the arcade (7 round arches), its floor the podium, the first floor slab over it
    bays = [(-WING, -WING + 2.4, 0.0), (-WING + 2.4, -CEN, 0.0), (CEN, WING - 2.4, 0.0), (WING - 2.4, WING, 0.0)]
    cw = 2 * CEN / 3
    bays += [(-CEN + cw * k, -CEN + cw * (k + 1), -1.0) for k in range(3)]
    for x0, x1, yf in bays:
        arch_bay(g, x0, x1, yf - 0.3, yf, PZ, PZ + 2.25, F2)
        # a door in the core wall behind each arch
        xc = (x0 + x1) / 2
        g.box('archDark', xc - 0.55, 2.14, PZ, xc + 0.55, 2.2, PZ + 2.5, M)
        g.box('opWhite', xc - 0.7, 2.12, PZ + 2.5, xc + 0.7, 2.2, PZ + 2.7, M)
    for x0, x1 in ((-CEN - 0.3, -CEN), (CEN, CEN + 0.3)):             # side walls of the raised centre
        g.box('wallSalmon', x0, -1.3, PZ, x1, 0.0, EAVE, M)
    g.box('opWhite', -WING, -0.45, F2 - 0.1, -CEN, 2.2, F2 + 0.25, M)
    g.box('opWhite', CEN, -0.45, F2 - 0.1, WING, 2.2, F2 + 0.25, M)
    g.box('opWhite', -CEN - 0.3, -1.45, F2 - 0.1, CEN + 0.3, 2.2, F2 + 0.25, M)
    # first floor: the loggia, paired columns, balusters, windows in the wall behind, the entablature
    for x0, x1, yf in bays:
        for xx in (x0, x1):
            g.box('white', xx - 0.2, yf - 0.32, F2 + 0.25, xx + 0.2, yf - 0.02, EAVE - 0.35, M)
        balustrade(g, x0 + 0.2, x1 - 0.2, yf - 0.17, F2 + 0.25, 0.85)
        window(g, (x0 + x1) / 2, 2.2, F2 + 0.75, F2 + 2.9, w=1.0)
    g.box('opWhite', -WING, -0.45, EAVE - 0.35, -CEN, 2.2, EAVE + 0.05, M)
    g.box('opWhite', CEN, -0.45, EAVE - 0.35, WING, 2.2, EAVE + 0.05, M)
    g.box('opWhite', -CEN - 0.3, -1.45, EAVE - 0.35, CEN + 0.3, 2.2, EAVE + 0.05, M)
    # the pediment over the centre: white frame behind a coral face
    xz(g, 'opWhite', [(-CEN - 0.55, EAVE), (CEN + 0.55, EAVE), (0.0, 11.0)], -1.3, -0.8, M)
    xz(g, 'wallSalmon', [(-CEN - 0.2, EAVE + 0.12), (CEN + 0.2, EAVE + 0.12), (0.0, 10.7)], -1.38, -1.28, M)
    g.ring('gold', M @ T(0.0, -1.42, 9.45) @ Matrix.Rotation(PI / 2, 4, 'X'), 0.42, 0.06, 14)
    # back: windows on both floors, a door
    for xx in (-6.0, -3.6, -1.2, 1.2, 3.6, 6.0):
        window(g, xx, 11.0, 1.6, 3.6, w=1.0, face=1)
        window(g, xx, 11.0, 5.6, 7.7, w=1.0, face=1)
    g.box('archDark', -0.8, 11.0, PZ, 0.8, 11.06, PZ + 2.7, M)
    for k in range(3):
        g.box('stone', -1.4, 11.4 + 0.45 * k, GROUND - 0.25, 1.4, 11.4 + 0.45 * (k + 1), PZ - 0.17 * (k + 1), M)
    # the roof, the dragons and the emblem on its ridge
    hip(g, 'roofRed', -WING - 0.4, WING + 0.4, -0.7, 11.6, EAVE, RIDGE, 5.0)
    g.box('roofRedDark', -5.0, 5.35, RIDGE - 0.1, 5.0, 5.55, RIDGE + 0.12, M)
    zr = RIDGE + 0.1
    Mr = M @ T(0.0, 5.45, 0.0)
    for s in (-1, 1):
        dragon(g, s, zr, Mr)
    # the emblem between them: a gold disc with a dark anchor on a little plinth
    g.box('stone', -0.25, -0.25, zr - 0.05, 0.25, 0.25, zr + 0.3, Mr)
    g.ring('gold', Mr @ T(0.0, -0.05, zr + 0.85) @ Matrix.Rotation(PI / 2, 4, 'X'), 0.62, 0.1, 16)
    g.box('archDark', -0.05, -0.18, zr + 0.45, 0.05, -0.15, zr + 1.25, Mr)
    g.box('archDark', -0.28, -0.18, zr + 1.05, 0.28, -0.15, zr + 1.12, Mr)
    g.box('archDark', -0.32, -0.18, zr + 0.5, 0.32, -0.15, zr + 0.58, Mr)
    return g


# ------------------------------------------------------------------------------------------------ the quay and the front
def rail_run(g, a, b, h=1.1):
    a, b = Vector((a.x, a.y, 0)), Vector((b.x, b.y, 0))
    L = (b - a).length
    u = (b - a) / L
    n = Vector((-u.y, u.x, 0))
    R = Matrix(((u.x, n.x, 0, a.x), (u.y, n.y, 0, a.y), (0, 0, 1, GROUND), (0, 0, 0, 1)))
    k = max(1, round(L / 1.8))
    for j in range(k + 1):
        x = L * j / k
        g.box('railing', x - 0.05, -0.05, 0.0, x + 0.05, 0.05, h, R)
    # the lower rail higher than a step (0.5 m), so nobody climbs the railing
    for z in (0.72, h - 0.05):
        g.box('railing', 0.0, -0.035, z - 0.035, L, 0.035, z + 0.035, R)
    for j in range(k * 6):
        x = L * (j + 0.5) / (k * 6)
        g.box('railing', x - 0.015, -0.015, 0.1, x + 0.015, 0.015, h - 0.05, R)


def statue(g, M):
    """the young Nguyễn Tất Thành looking out to the river, on a stone plinth (a plain bronze figure)"""
    g.box('stone', -0.8, -0.8, GROUND - 0.2, 0.8, 0.8, 1.9, M)
    g.box('stone', -0.95, -0.95, 1.9, 0.95, 0.95, 2.05, M)
    g.box('stone', -1.0, -1.0, GROUND - 0.2, 1.0, 1.0, 0.45, M)
    z = 2.05
    for xx in (-0.13, 0.13):
        g.box('bronze', xx - 0.1, -0.1, z, xx + 0.1, 0.12, z + 0.95, M)
    g.box('bronze', -0.27, -0.16, z + 0.9, 0.27, 0.16, z + 1.7, M)
    g.box('bronze', -0.3, -0.18, z + 1.45, 0.3, 0.18, z + 1.72, M)
    g.ico('bronze', tuple(M @ Vector((0.0, -0.02, z + 1.95))), 0.17, sub=1)
    g.box('bronze', 0.3, -0.08, z + 0.95, 0.42, 0.08, z + 1.65, M)          # right arm down
    g.box('bronze', -0.42, -0.08, z + 0.95, -0.3, 0.08, z + 1.65, M)
    g.box('bronze', -0.7, -0.12, z + 0.55, -0.32, 0.12, z + 0.98, M)        # the suitcase
    g.box('bronze', -0.56, -0.03, z + 0.98, -0.46, 0.03, z + 1.06, M)


def pot(g, p):
    g.ring('roofRedDark', T(p.x, p.y, GROUND), 0.42, 0.45, 10, r2=0.5)
    g.rod('bark', (p.x, p.y, GROUND + 0.4), (p.x + 0.12, p.y, GROUND + 0.95), 0.06, 5)
    g.ico('leafDark', (p.x + 0.12, p.y, GROUND + 1.1), 0.48, sub=1, scale=(1.0, 1.0, 0.55))
    g.ico('leaf', (p.x - 0.2, p.y + 0.1, GROUND + 0.85), 0.3, sub=1, scale=(1.0, 1.0, 0.5))


def flagpole(g, p, h=9.0):
    g.ring('stone', T(p.x, p.y, GROUND), 0.35, 0.3, 8)
    g.ring('metal', T(p.x, p.y, GROUND), 0.06, h, 6)
    g.ico('gold', (p.x, p.y, GROUND + h + 0.08), 0.1, sub=0)
    # the flag streams along the river, downstream
    a = Vector((p.x, p.y, GROUND + h - 1.3))
    u = D
    V = [a, a + u * 1.9, a + u * 1.9 + Vector((0, 0, 1.25)), a + Vector((0, 0, 1.25))]
    g.raw('signRed', [tuple(v + N * 0.02) for v in V], [(0, 1, 2, 3)])
    g.raw('signRed', [tuple(v - N * 0.02) for v in V], [(3, 2, 1, 0)])
    c = a + u * 0.95 + Vector((0, 0, 0.62))
    for sgn in (1, -1):
        star = [c + (u * math.cos(PI / 2 + k * 4 * PI / 5) + Vector((0, 0, math.sin(PI / 2 + k * 4 * PI / 5)))) * 0.32 for k in range(5)]
        g.raw('signGold', [tuple(v + N * 0.03 * sgn) for v in [c] + star], [(0, 1, 3) if sgn > 0 else (0, 3, 1), (0, 2, 4) if sgn > 0 else (0, 4, 2),
                                                                             (0, 3, 5) if sgn > 0 else (0, 5, 3), (0, 4, 1) if sgn > 0 else (0, 1, 4),
                                                                             (0, 5, 2) if sgn > 0 else (0, 2, 5)])


def lamp(g, p):
    g.ring('archDark', T(p.x, p.y, GROUND), 0.07, 3.4, 6)
    g.ico('lampGlow', (p.x, p.y, GROUND + 3.65), 0.26, sub=1)


def front():
    g = kit.Mesh(52)
    # the quay: 3 m over the river, paved, stone walls down to the river bed
    quay = [W(3.0, QUAY_I), W(40.0, QUAY_I), W(40.5, 1.4), W(3.0, 1.4)]
    prism(g, 'paving', [(p.x, p.y) for p in quay], -2.6, GROUND, 'stone')
    # the railing stands 0.6 m back from the edge; the bollards are on the strip outside it, out of reach
    rail_run(g, W(3.0, QUAY_I + 0.6), W(40.0, QUAY_I + 0.6))
    rail_run(g, W(3.0, QUAY_I + 0.6), W(3.0, 1.2))
    rail_run(g, W(40.0, QUAY_I + 0.6), W(40.0, QUAY_I + 0.05))      # closes the strip outside the railing
    for s in range(6, 40, 6):
        p = W(s, QUAY_I + 0.2)
        g.ring('archDark', T(p.x, p.y, GROUND), 0.13, 0.4, 8)
        g.ring('archDark', T(p.x, p.y, GROUND + 0.4), 0.17, 0.07, 8)
    for s in (9, 17, 25, 33):
        lamp(g, W(s, QUAY_I + 1.2))
    # the statue on the axis of the house, two flagpoles, bonsai in pots along the podium
    statue(g, frame(21.0, 1.8))
    for s in (17.2, 24.8):
        flagpole(g, W(s, -0.6))
    for x in (4.6, 6.6, 8.6, 10.4):
        for sx in (-1, 1):
            pot(g, W(21.0 - sx * x, 4.0))
    return g


# ------------------------------------------------------------------------------------------------ the ship, the flag tower
def ship():
    """a model of a steamer of 1911 (Amiral Latouche-Tréville), 15 m on this map, moored bow downstream"""
    g = kit.Mesh(53)
    o = W(38.5, -6.7)
    x, y = D, N
    M = Matrix(((x.x, y.x, 0, o.x), (x.y, y.y, 0, o.y), (0, 0, 1, 0), (0, 0, 0, 1)))
    L2, B2 = 7.5, 1.8

    def outline(shrink=0.0):
        pts = [(L2 - shrink * 1.6, 0.0), (L2 - 3.2, B2 - shrink), (-L2 + 1.8, B2 - shrink)]
        for k in range(1, 6):
            a = PI / 2 + PI * k / 6
            pts.append((-L2 + 1.8 + (1.8 - shrink) * math.cos(a), (B2 - shrink) * math.sin(a)))
        pts += [(-L2 + 1.8, -B2 + shrink), (L2 - 3.2, -B2 + shrink)]
        return [tuple(M @ Vector((px, py, 0)))[:2] for px, py in pts]
    prism(g, 'archDark', outline(), -1.2, 1.15)
    prism(g, 'roofRedDark', outline(-0.03), -0.7, -0.25)
    prism(g, 'white', outline(-0.03), 1.0, 1.3)
    prism(g, 'wood', outline(0.2), 1.15, 1.36)
    # deckhouse, bridge, funnel, masts with yards and stays
    g.box('white', -3.2, -1.2, 1.35, 2.4, 1.2, 2.9, M)
    for xx in (-2.6, -1.6, -0.6, 0.4, 1.4):
        for yy in (-1.21, 1.21):
            g.box('glassDark', xx - 0.25, yy - 0.02, 2.0, xx + 0.25, yy + 0.02, 2.45, M)
    g.box('white', 0.6, -1.0, 2.9, 2.4, 1.0, 3.9, M)
    g.box('glassDark', 2.38, -0.8, 3.3, 2.42, 0.8, 3.7, M)
    g.box('wood', 0.4, -1.3, 3.9, 2.6, 1.3, 4.0, M)
    g.ring('archDark', M @ T(-1.0, 0, 2.9), 0.5, 2.8, 12)
    g.ring('signRed', M @ T(-1.0, 0, 5.1), 0.52, 0.35, 12)
    for xm, hm in ((4.2, 8.5), (-5.0, 7.8)):
        top = M @ Vector((xm, 0, hm))
        g.rod('wood', M @ Vector((xm, 0, 1.36)), top, 0.08, 6)
        g.rod('wood', M @ Vector((xm, -1.2, hm - 1.6)), M @ Vector((xm, 1.2, hm - 1.6)), 0.04, 5)
        for end in ((L2 - 0.2, 0, 1.3), (-L2 + 0.3, 0, 1.3)):
            g.rod('archDark', top, M @ Vector(end), 0.015, 3)
    # mooring lines to the bollards
    for s, xx in ((30.0, -6.5), (36.0, 6.5)):
        b = W(s, QUAY_I + 0.2, GROUND + 0.4)
        g.rod('archDark', M @ Vector((xx, -B2, 1.2)), b, 0.025, 4)
    return g


def run():
    old = kit.drop_loose_parts('LM_NewBuildings', lambda x, y: 46 < x < 71 and -41 < y < -21)
    # LM_CotCoThuNgu: the Thủ Ngữ flag tower across the canal, taken out again (Khoa 2026-11-19)
    kit.remove_objects(['LM_NhaRong', 'LF_NhaRong', 'LM_NhaRong_Ship', 'LM_CotCoThuNgu'])
    coll = kit.collection('SM_Landmarks')
    ob = kit.make_object(house(), 'LM_NhaRong', coll)
    kit.make_object(front(), 'LF_NhaRong', coll)
    kit.make_object(ship(), 'LM_NhaRong_Ship', coll)

    def si(x, y):
        p = Vector((x, y, 0)) - A
        return p.dot(D), -p.dot(N)

    def on_site(x, y):
        s, i = si(x, y)
        return (9.0 < s < 33.0 and 4.5 < i < 18.5) or (2.5 < s < 41.0 and -3.5 < i < 4.5)
    gone = {n: kit.drop_loose_parts(n, on_site) for n in ('CX_StreetLife', 'CX_Trees')}
    gone['old_model_parts'] = old
    kit.add_wobble('LM_NhaRong', 0.02)
    ob.data.calc_loop_triangles()
    return {'object': ob.name, 'tris': len(ob.data.loop_triangles), 'removed': gone}


if __name__ == '__main__':
    print('BEN_NHA_RONG', run())
    # alone on the map: blender models/saigon_mini.blend --background --python scripts/blender/mini/ben_nha_rong.py -- --save
    if '--save' in (sys.argv[sys.argv.index('--') + 1:] if '--' in sys.argv else []):
        import bpy
        bpy.ops.wm.save_mainfile(compress=True)
