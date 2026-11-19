"""The statues of Sài Gòn mini as figures rather than blocks: Trần Hưng Đạo at Mê Linh, Đức Mẹ Hòa Bình in front of
the cathedral and the young Nguyễn Tất Thành at Bến Nhà Rồng.

- Trần Hưng Đạo (Phạm Thông, 1967; restored 2022): bronze, about 6 m, on a triangular prism of golden granite about
  10 m tall. Helmet, long beard, armour over a long robe, a cape; the right arm points out at the river (Bạch Đằng),
  the left hand rests on the hilt of a sword planted by his foot. "ĐỨC THÁNH / TRẦN HƯNG ĐẠO" in gilt letters on
  the two faces either side of the corner that points at the river; terracotta relief panels low on the faces; a
  bronze incense urn in front.
- Đức Mẹ Hòa Bình (G. Ciocchetti, 1959): white marble, standing upright, veil and mantle, the hands holding the globe
  with a cross on it, the eyes raised to the sky; on the old round pedestal of red granite.
- Nguyễn Tất Thành (Phạm Mười, 2003): bronze, a young man in a jacket and trousers with a small suitcase, looking out
  over the river; built by ben_nha_rong.py with figure_ntt() from here.

Each figure is built in a local frame facing -Y (its right hand on -X), feet at z = 0, in metres of a 1.8 m person
scaled by `s`. Low-poly but shaped: lathed robes with folds, tubes for limbs and sleeves, shells for cape, mantle
and veil.

run() replaces the statue and pedestal faces of the kept models LM_MeLinh (bronze, stone and concrete parts) and
LM_TuongDucMe_parkedv4 (marble and stone parts), keeping their gardens, and builds LM_TuongTHD and LM_TuongDucMe.
Runs inside an open models/saigon_mini.blend; running it again rebuilds both.
"""
import math
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
if HERE not in sys.path:
    sys.path.insert(0, HERE)
import bmesh  # noqa: E402
import bpy  # noqa: E402
import kit  # noqa: E402
import layout as LY  # noqa: E402
from mathutils import Matrix, Vector  # noqa: E402

PI = math.pi
for _k, _v in {'graniteGold': '#D6B47E', 'graniteRed': '#94574C', 'bronze': '#55604F', 'bronzeLight': '#8A8B66',
               'roofRed': '#A9492F', 'gold': '#C9A24A'}.items():
    kit.PAL.setdefault(_k, _v)

THD = Vector((17.0, 183.0))       # Mê Linh: the pedestal's centre (LM_MeLinh as placed), the river to the east
DM = Vector((-111.0, 127.0))      # Công xã Paris: the cathedral to the west, the statue looks east down Đồng Khởi


def T(x, y, z=0.0):
    return Matrix.Translation((x, y, z))


def Rz(a):
    return Matrix.Rotation(a, 4, 'Z')


def Rx(a):
    return Matrix.Rotation(a, 4, 'X')


class Fig:
    """a figure's local frame (facing -Y) and scale; every point is given in metres of a 1.8 m person"""

    def __init__(self, g, M, s):
        self.g, self.M, self.s = g, M, s

    def p(self, q, M=None):
        return (M or self.M) @ Vector((q[0] * self.s, q[1] * self.s, q[2] * self.s))

    def tube(self, mat, pts, r, seg=7, caps=True, M=None):
        r = r if isinstance(r, (list, tuple)) else [r] * len(pts)
        self.g.tube(mat, [self.p(q, M) for q in pts], [x * self.s for x in r], seg=seg, caps=caps)

    def ico(self, mat, c, r, scale=(1, 1, 1), sub=1, M=None):
        """an icosphere; the frame's rotation is applied too, so a tilted head stays tilted"""
        M = M or self.M
        bm = bmesh.new()
        bmesh.ops.create_icosphere(bm, subdivisions=sub, radius=r * self.s)
        bm.verts.index_update()
        V = [M @ Vector((c[0] * self.s + v.co.x * scale[0], c[1] * self.s + v.co.y * scale[1], c[2] * self.s + v.co.z * scale[2]))
             for v in bm.verts]
        F = [tuple(v.index for v in f.verts) for f in bm.faces]
        bm.free()
        self.g.raw(mat, [tuple(v) for v in V], F)

    def box(self, mat, lo, hi, M=None):
        s = self.s
        self.g.box(mat, lo[0] * s, lo[1] * s, lo[2] * s, hi[0] * s, hi[1] * s, hi[2] * s, M or self.M)

    def lathe(self, mat, prof, seg=12, ysc=1.0, folds=None, c=(0.0, 0.0), caps=True, M=None):
        """rings of (radius, z): an ellipse (y squashed by ysc) round (cx, cy); folds(z, a) multiplies the radius"""
        P, rings = [], []
        for r, z in prof:
            P.append(self.p((c[0], c[1], z), M))
            ring = []
            for j in range(seg):
                a = 2 * PI * j / seg
                k = folds(z, a) if folds else 1.0
                ring.append(self.p((c[0] + r * k * math.cos(a), c[1] + r * k * ysc * math.sin(a), z), M))
            rings.append(ring)
        self.g.sweep(mat, P, rings, caps)

    def shell(self, mat, prof, a0, a1, nseg, th, ysc=1.0, folds=None, c=(0.0, 0.0), M=None):
        """part of a lathe as a sheet `th` thick, from angle a0 to a1 (radians; -Y, the front, is -pi/2):
        a cape, a mantle, a veil. prof: (inner radius, z) from top to bottom"""
        O, I, A = [], [], []
        for r, z in prof:
            o_row, i_row = [], []
            for j in range(nseg + 1):
                a = a0 + (a1 - a0) * j / nseg
                k = folds(z, a) if folds else 1.0
                ri, ro = r * k, r * k + th
                i_row.append(self.p((c[0] + ri * math.cos(a), c[1] + ri * ysc * math.sin(a), z), M))
                o_row.append(self.p((c[0] + ro * math.cos(a), c[1] + ro * ysc * math.sin(a), z), M))
            O.append(o_row)
            I.append(i_row)
            A.append(self.p((c[0], c[1], z), M))
        g = self.g
        nz = len(prof)
        mid = [[(O[k][j] + I[k][j]) / 2 for j in range(nseg + 1)] for k in range(nz)]
        for k in range(nz - 1):
            ax = (A[k] + A[k + 1]) / 2
            for j in range(nseg):
                g.face_out(mat, [O[k][j], O[k][j + 1], O[k + 1][j + 1], O[k + 1][j]], ax)
                cc = (I[k][j] + I[k][j + 1] + I[k + 1][j + 1] + I[k + 1][j]) / 4
                g.face_out(mat, [I[k][j], I[k][j + 1], I[k + 1][j + 1], I[k + 1][j]], ax + (cc - ax) * 2)
            for j, jn in ((0, 1), (nseg, nseg - 1)):
                g.face_out(mat, [O[k][j], O[k + 1][j], I[k + 1][j], I[k][j]], (mid[k][jn] + mid[k + 1][jn]) / 2)
        for k, kn in ((0, 1), (nz - 1, nz - 2)):
            for j in range(nseg):
                g.face_out(mat, [O[k][j], O[k][j + 1], I[k][j + 1], I[k][j]], (mid[kn][j] + mid[kn][j + 1]) / 2)


def text(g, mat, body, M, size, depth=0.03):
    """gilt letters: Blender's built-in font (it has the Vietnamese letters), centred on the local origin, reading
    along local +X, up +Y, standing out along +Z"""
    cu = bpy.data.curves.new('sd_text', 'FONT')
    cu.body = body
    cu.size = size
    cu.align_x = 'CENTER'
    cu.align_y = 'CENTER'
    cu.extrude = depth / 2
    cu.resolution_u = 2                     # coarse curves: letters this size need no more
    ob = bpy.data.objects.new('sd_text', cu)
    bpy.context.scene.collection.objects.link(ob)
    dg = bpy.context.evaluated_depsgraph_get()
    ev = ob.evaluated_get(dg)
    me = ev.to_mesh()
    V = [tuple(M @ (v.co + Vector((0, 0, depth / 2)))) for v in me.vertices]
    F = [tuple(p.vertices) for p in me.polygons]
    g.raw(mat, V, F)
    ev.to_mesh_clear()
    bpy.data.objects.remove(ob, do_unlink=True)
    bpy.data.curves.remove(cu)


def face_frame(c, n, z):
    """local frame on a vertical face: origin c (XY) at height z, X along the face as a reader sees it, Y up, Z out"""
    n = Vector((n[0], n[1], 0)).normalized()
    r = Vector((-n.y, n.x, 0))
    return Matrix(((r.x, 0, n.x, c[0]), (r.y, 0, n.y, c[1]), (0, 1, 0, z), (0, 0, 0, 1)))


# ---------------------------------------------------------------------------------------------------------------
# Trần Hưng Đạo
# ---------------------------------------------------------------------------------------------------------------
def figure_thd(g, M, s=1.9):
    f = Fig(g, M, s)
    B, BL = 'bronze', 'bronzeLight'
    # boots: a wide stance, the right foot a little forward
    for x, dy in ((-0.15, -0.05), (0.15, 0.03)):
        f.box(B, (x - 0.06, dy - 0.15, 0.0), (x + 0.06, dy + 0.06, 0.09))
        f.tube(B, [(x, dy, 0.08), (x * 0.9, dy * 0.5, 0.42)], [0.06, 0.07], seg=6)

    def robe_folds(z, a):
        return 1.0 + 0.07 * max(0.0, (0.9 - z) / 0.6) * math.cos(6 * a + 0.4)
    f.lathe(B, [(0.36, 0.34), (0.33, 0.48), (0.27, 0.72), (0.21, 0.96)], seg=14, ysc=0.78, folds=robe_folds)
    f.box(B, (-0.08, -0.25, 0.38), (0.08, -0.15, 0.95))                    # the front panel of the robe
    f.lathe(BL, [(0.215, 0.93), (0.22, 1.02)], seg=12, ysc=0.75)            # belt
    f.box(BL, (-0.05, -0.19, 0.93), (0.05, -0.16, 1.03))                    # buckle
    # armour over the chest, shoulder guards
    f.lathe(B, [(0.2, 1.0), (0.225, 1.16), (0.235, 1.3), (0.205, 1.41), (0.08, 1.47)], seg=12, ysc=0.72)
    f.lathe(BL, [(0.205, 1.22), (0.21, 1.25)], seg=12, ysc=0.75, caps=True)  # the armour's chest band
    for x in (-0.23, 0.23):
        f.ico(B, (x, 0.0, 1.38), 0.105, scale=(1.25, 1.1, 0.75))
        f.ico(BL, (x * 1.08, 0.0, 1.33), 0.1, scale=(1.2, 1.05, 0.35), sub=1)   # the guard's lower plate
    # cape from the shoulders, falling behind and flaring
    def cape_folds(z, a):
        return 1.0 + 0.05 * math.cos(5 * a) * (1.4 - z)
    f.shell(B, [(0.2, 1.42), (0.25, 1.15), (0.33, 0.8), (0.42, 0.45), (0.47, 0.3)], math.radians(15), math.radians(165), 8,
            0.03, ysc=0.85, folds=cape_folds, c=(0.0, 0.04))
    # head: neck, face, the long beard, a helmet with a crest
    f.tube(B, [(0, 0, 1.44), (0, -0.005, 1.53)], 0.06, seg=6)
    f.ico(B, (0, -0.01, 1.62), 0.105, scale=(0.9, 1.0, 1.12))
    f.box(B, (-0.016, -0.125, 1.585), (0.016, -0.09, 1.635))                # nose
    f.tube(B, [(0, -0.085, 1.565), (0, -0.115, 1.47), (0, -0.11, 1.36)], [0.07, 0.05, 0.012], seg=6)   # beard
    f.box(B, (-0.06, -0.115, 1.55), (0.06, -0.09, 1.57))                    # moustache
    f.lathe(B, [(0.13, 1.64), (0.132, 1.7), (0.105, 1.77), (0.045, 1.81), (0.012, 1.83)], seg=10)
    f.lathe(BL, [(0.155, 1.635), (0.155, 1.66)], seg=10)                    # helmet brim
    f.tube(BL, [(0, 0, 1.82), (0, 0.01, 1.9)], [0.02, 0.012], seg=5)        # crest spike
    f.ico(BL, (0, 0.012, 1.91), 0.025, sub=0)
    f.shell(B, [(0.125, 1.64), (0.135, 1.55)], math.radians(20), math.radians(160), 5, 0.02)   # neck guard
    # right arm: out and forward, pointing at the river
    S, E, W, H = (-0.23, 0.0, 1.37), (-0.28, -0.25, 1.27), (-0.29, -0.5, 1.13), (-0.29, -0.57, 1.09)
    f.tube(B, [S, E, W], [0.075, 0.07, 0.1], seg=7)
    f.ico(B, H, 0.045, scale=(0.9, 1.3, 0.8), sub=0)
    f.tube(B, [H, (-0.29, -0.67, 1.03)], [0.016, 0.012], seg=4)             # the pointing finger, down at the river
    # left arm: down, the hand on the sword's pommel
    S, E, W = (0.23, 0.0, 1.37), (0.31, 0.02, 1.13), (0.25, -0.1, 1.0)
    f.tube(B, [S, E, W], [0.075, 0.072, 0.098], seg=7)
    f.ico(B, (0.245, -0.12, 1.0), 0.045, sub=0)
    # the sword planted by the left foot
    f.ico(BL, (0.24, -0.13, 1.04), 0.03, sub=0)
    f.tube(BL, [(0.24, -0.13, 1.03), (0.245, -0.14, 0.9)], 0.02, seg=5)
    f.box(BL, (0.17, -0.155, 0.88), (0.32, -0.125, 0.91))
    f.tube(BL, [(0.245, -0.14, 0.88), (0.27, -0.19, 0.04)], [0.03, 0.022], seg=5)


def pedestal_thd(g, c, z0):
    """a triangular prism of golden granite on two steps, one corner at the river (+X); returns its top"""
    TOP = z0 + 5.6
    R = 2.05                               # circumradius
    ch = 0.22                              # the corners are cut off this much

    def tri(r, z):
        corners = [Vector((math.cos(2 * PI * k / 3), math.sin(2 * PI * k / 3), 0)) * r for k in range(3)]
        pts = []
        for k in range(3):
            p, prv, nxt = corners[k], corners[k - 1], corners[(k + 1) % 3]
            pts += [p + (prv - p).normalized() * ch, p + (nxt - p).normalized() * ch]
        return [Vector((c.x + q.x, c.y + q.y, z)) for q in pts]

    def prism(r, za, zb):
        lo, hi = tri(r, za), tri(r, zb)
        ctr = Vector((c.x, c.y, (za + zb) / 2))
        for i in range(6):
            j = (i + 1) % 6
            g.face_out('graniteGold', [lo[i], lo[j], hi[j], hi[i]], ctr)
        g.face_out('graniteGold', hi, ctr)
        g.face_out('graniteGold', lo, ctr)

    prism(R + 1.0, LY.WALK_Z - 0.2, z0 - 0.2)              # two steps
    prism(R + 0.5, z0 - 0.2, z0)
    prism(R, z0, TOP)
    # the two faces either side of the river corner: the inscription high up, two relief panels low down
    for a in (PI / 3, -PI / 3):
        n = Vector((math.cos(a), math.sin(a), 0))
        fc = Vector((c.x, c.y, 0)) + n * (R / 2 + 0.004)    # R / 2: the inradius
        text(g, 'gold', 'ĐỨC THÁNH', face_frame(fc, n, z0 + 4.75), 0.24, 0.03)
        text(g, 'gold', 'TRẦN HƯNG ĐẠO', face_frame(fc, n, z0 + 4.3), 0.34, 0.03)
        for dx in (-0.72, 0.72):
            M = face_frame(fc, n, z0 + 1.5) @ T(dx, 0, 0)
            g.box('roofRedDark', -0.52, -0.62, 0.0, 0.52, 0.62, 0.03, M)          # frame
            g.box('roofRed', -0.46, -0.56, 0.03, 0.46, 0.56, 0.05, M)
            for k, (x0, x1, y0, y1) in enumerate(((-0.36, -0.22, -0.4, 0.05), (-0.12, 0.0, -0.4, 0.12), (0.1, 0.22, -0.4, 0.0),
                                                  (-0.4, 0.38, 0.18, 0.28), (0.26, 0.38, -0.4, 0.1))):
                g.box('roofRed', x0, y0, 0.05, x1, y1, 0.075 + 0.01 * (k % 2), M)  # the battle scene in relief
    return TOP


def urn(g, c, z):
    """the bronze incense urn in front of the statue"""
    g.ring('graniteGold', T(c.x, c.y, z), 0.42, 0.45, 8, r2=0.38)
    g.lathe('bronze', T(c.x, c.y, z + 0.45), [(0.08, 0.0), (0.24, 0.05), (0.36, 0.2), (0.38, 0.36), (0.33, 0.48), (0.36, 0.52)],
            seg=12, cap_top=True)
    for a in (0.0, PI):
        g.ring('bronze', T(c.x + 0.4 * math.cos(a), c.y + 0.4 * math.sin(a), z + 0.88), 0.06, 0.12, 6)


def build_thd():
    g = kit.Mesh(seed=11)
    z0 = LY.WALK_Z + 0.42
    top = pedestal_thd(g, THD, z0)
    figure_thd(g, T(THD.x + 0.1, THD.y, top) @ Rz(PI / 2), s=1.9)        # faces +X, the river
    urn(g, Vector((THD.x + 3.75, THD.y)), LY.WALK_Z)
    return g


# ---------------------------------------------------------------------------------------------------------------
# Đức Mẹ Hòa Bình
# ---------------------------------------------------------------------------------------------------------------
def figure_ducme(g, M, s=1.72):
    f = Fig(g, M, s)
    W = 'statueW'
    f.lathe(W, [(0.3, 0.0), (0.3, 0.07)], seg=14)                           # the statue's own round base

    def robe_folds(z, a):
        return 1.0 + 0.08 * max(0.0, (1.0 - z)) * math.cos(8 * a + 1.0)
    f.lathe(W, [(0.27, 0.07), (0.26, 0.2), (0.22, 0.55), (0.185, 0.85), (0.17, 1.0), (0.18, 1.16), (0.19, 1.3),
                (0.17, 1.41), (0.07, 1.47)], seg=16, ysc=0.8, folds=robe_folds)
    for x in (-0.07, 0.07):                                                 # toes under the hem
        f.ico(W, (x, -0.22, 0.09), 0.05, scale=(0.8, 1.3, 0.6), sub=0)
    f.lathe(W, [(0.172, 0.98), (0.176, 1.02)], seg=14, ysc=0.82)            # sash

    # the mantle: over the shoulders, open at the front, down to the hem behind
    def mantle_folds(z, a):
        return 1.0 + 0.06 * math.cos(6 * a) * (1.5 - z)
    f.shell(W, [(0.2, 1.43), (0.215, 1.3), (0.22, 1.0), (0.26, 0.6), (0.3, 0.25), (0.31, 0.12)],
            math.radians(-40), math.radians(220), 12, 0.035, ysc=0.82, folds=mantle_folds)
    # head tilted back, the eyes to the sky; the veil over it and down onto the shoulders
    H = M @ T(0, 0, 1.5 * s) @ Rx(math.radians(-16))
    f.tube(W, [(0, 0, 1.42), (0, 0, 1.52)], 0.055, seg=6)
    f.ico(W, (0, -0.012, 0.11), 0.1, scale=(0.88, 1.0, 1.12), M=H)
    f.box(W, (-0.013, -0.125, 0.085), (0.013, -0.095, 0.13), M=H)           # nose
    f.shell(W, [(0.025, 0.235), (0.1, 0.205), (0.118, 0.13), (0.125, 0.04), (0.16, -0.06)],
            math.radians(-35), math.radians(215), 10, 0.022, M=H)
    # arms bent, the hands holding the globe in front of the chest; wide sleeves
    for sx in (-1, 1):
        f.tube(W, [(sx * 0.19, 0.0, 1.37), (sx * 0.21, -0.05, 1.13), (sx * 0.12, -0.2, 1.1)], [0.06, 0.068, 0.095], seg=7)
        f.ico(W, (sx * 0.1, -0.23, 1.11), 0.04, scale=(0.8, 1.2, 0.9), sub=0)
    f.ico(W, (0, -0.24, 1.15), 0.1, sub=2)
    f.box(W, (-0.012, -0.252, 1.24), (0.012, -0.228, 1.42))                 # the cross on the globe
    f.box(W, (-0.055, -0.252, 1.335), (0.055, -0.228, 1.36))


def build_ducme():
    g = kit.Mesh(seed=12)
    c, z = DM, LY.WALK_Z
    # the old round pedestal of red granite: a plinth, the drum, a moulded cap
    g.ring('stone', T(c.x, c.y, z - 0.2), 1.25, 0.5, 16)
    g.lathe('graniteRed', T(c.x, c.y, z + 0.3), [(0.95, 0.0), (0.95, 0.18), (0.82, 0.3), (0.74, 0.34), (0.72, 1.7), (0.8, 1.76),
                                               (0.88, 1.9), (0.88, 2.02), (0.66, 2.1)], seg=16)
    g.lathe('stone', T(c.x, c.y, z + 2.38), [(0.68, 0.0), (0.6, 0.14), (0.42, 0.18)], seg=16)
    figure_ducme(g, T(c.x, c.y, z + 2.56) @ Rz(PI / 2), s=1.72)             # faces +X, away from the cathedral
    return g


# ---------------------------------------------------------------------------------------------------------------
# Nguyễn Tất Thành (used by ben_nha_rong.py)
# ---------------------------------------------------------------------------------------------------------------
def figure_ntt(g, M, s=1.35):
    f = Fig(g, M, s)
    B, BL = 'bronze', 'bronzeLight'
    # shoes and trousers, the right foot a step forward
    for x, dy in ((-0.1, -0.06), (0.1, 0.03)):
        f.box(B, (x - 0.055, dy - 0.16, 0.0), (x + 0.055, dy + 0.06, 0.08))
        f.tube(B, [(x, dy, 0.07), (x * 1.02, dy * 0.6 - 0.01, 0.48), (x * 0.9, 0.0, 0.86)], [0.06, 0.068, 0.085], seg=7)
    # the jacket: hem below the hips, buttoned, lapels
    f.lathe(B, [(0.2, 0.7), (0.19, 0.82), (0.175, 0.96), (0.19, 1.14), (0.205, 1.3), (0.19, 1.41), (0.075, 1.47)],
            seg=12, ysc=0.68)
    for z in (0.8, 0.95, 1.1):
        f.box(BL, (-0.018, -0.15, z), (0.018, -0.125, z + 0.035))
    for sx in (-1, 1):
        f.g.face_out(B, [f.p(q) for q in ((sx * 0.02, -0.142, 1.18), (sx * 0.1, -0.14, 1.42), (sx * 0.06, -0.13, 1.43),
                                           (sx * 0.005, -0.15, 1.2))], f.p((0, 0, 1.3)))         # lapels
        f.box(B, (sx * 0.09 - 0.05, -0.15, 0.82), (sx * 0.09 + 0.05, -0.128, 0.84))   # pocket flaps
    for x in (-0.2, 0.2):
        f.ico(B, (x, 0.0, 1.37), 0.08, scale=(1.1, 1.0, 0.7), sub=1)
    # head: neck, face, the short hair parted on the side
    f.tube(B, [(0, 0, 1.44), (0, -0.005, 1.53)], 0.052, seg=6)
    f.ico(B, (0, -0.008, 1.62), 0.098, scale=(0.88, 1.0, 1.12))
    f.ico(B, (0.01, 0.01, 1.66), 0.1, scale=(0.95, 1.0, 0.82))
    f.box(B, (-0.014, -0.118, 1.585), (0.014, -0.088, 1.63))                # nose
    # right arm down by the side, a little forward
    f.tube(B, [(-0.2, 0.0, 1.37), (-0.24, -0.03, 1.1), (-0.22, -0.1, 0.88)], [0.062, 0.058, 0.055], seg=7)
    f.ico(B, (-0.22, -0.11, 0.84), 0.042, scale=(0.8, 1.0, 1.2), sub=0)
    # left arm down, carrying the small suitcase
    f.tube(B, [(0.2, 0.0, 1.37), (0.245, 0.02, 1.1), (0.26, 0.0, 0.86)], [0.062, 0.058, 0.055], seg=7)
    f.ico(B, (0.265, 0.0, 0.82), 0.042, sub=0)
    f.box(B, (0.255, -0.22, 0.44), (0.375, 0.2, 0.74))
    f.box(BL, (0.25, -0.225, 0.58), (0.38, 0.205, 0.6))                     # strap
    f.tube(B, [(0.31, -0.06, 0.74), (0.3, -0.04, 0.8), (0.3, 0.04, 0.8), (0.31, 0.06, 0.74)], 0.012, seg=4)


# ---------------------------------------------------------------------------------------------------------------
def drop_faces(obname, keep):
    """delete the faces of an object for which keep(material name, world centre) is false"""
    ob = bpy.data.objects.get(obname)
    if ob is None:
        return 0
    bm = bmesh.new()
    bm.from_mesh(ob.data)
    mw = ob.matrix_world
    names = [m.name if m else '' for m in ob.data.materials]
    kill = [f for f in bm.faces if not keep(names[f.material_index] if f.material_index < len(names) else '', mw @ f.calc_center_median())]
    bmesh.ops.delete(bm, geom=kill, context='FACES')
    loose = [v for v in bm.verts if not v.link_faces]
    bmesh.ops.delete(bm, geom=loose, context='VERTS')
    bm.to_mesh(ob.data)
    bm.free()
    ob.data.update()
    return len(kill)


def run():
    kit.remove_objects(['LM_TuongTHD', 'LM_TuongDucMe'])
    coll = kit.collection('SM_Landmarks')
    gone = {
        # the old pedestal (stone, concrete) and figure (bronze) inside the flower ring; the benches' metal legs stay
        'LM_MeLinh': drop_faces('LM_MeLinh', lambda m, c: not (m in ('SD_stone', 'SD_concreteLight', 'SD_bronze', 'SD_bronzeLight')
                                                                and (c.x - THD.x) ** 2 + (c.y - THD.y) ** 2 < 3.4 ** 2)),
        'LM_TuongDucMe_parkedv4': drop_faces('LM_TuongDucMe_parkedv4', lambda m, c: m not in ('SD_stone', 'SD_statueW')),
    }
    out = {}
    for name, g in (('LM_TuongTHD', build_thd()), ('LM_TuongDucMe', build_ducme())):
        ob = kit.make_object(g, name, coll, smooth_angle=40)
        ob.data.calc_loop_triangles()
        out[name] = len(ob.data.loop_triangles)
    return {'tris': out, 'removed': gone}


if __name__ == '__main__':
    print('TUONG', run())
    # alone on the map: blender models/saigon_mini.blend --background --python scripts/blender/mini/tuong.py -- --save
    if '--save' in (sys.argv[sys.argv.index('--') + 1:] if '--' in sys.argv else []):
        bpy.ops.wm.save_mainfile(compress=True)
