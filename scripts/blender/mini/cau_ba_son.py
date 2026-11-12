"""Cầu Ba Son for Sài Gòn mini (map v5): cable-stayed bridge with one curved two-legged pylon on the Thủ Thiêm bank.

The v2 design (versions/sg.tinymap_v2.blend, saigon_diorama_13d_ba_son.py) scaled to the v5 river: white sail-shaped
pylon leaning over the main span, two inclined planes of stay cables, humped deck with two lanes, sidewalks, white
parapets with a silver handrail and gold lamp posts. Modern steel stays straight (no SD_HandWobble).

Runs inside an open models/saigon_mini.blend (interactive or --background). It replaces the Ba Son half of the
blockout object LM_Bridges (faces north of y = 100) with the new object LM_CauBaSon; the Hàm Nghi half is left alone.
Materials are the file's existing SD_<key> paint materials.

Run it in the open file (Text editor > Run Script, or exec the file and call run()), then save. Running it again
rebuilds the bridge.

Design approved by Khoa on 2026-11-12 (docs/map-v5-tasks.md, T0).
"""
import math
import bpy
import bmesh
from mathutils import Matrix, Vector

PI = math.pi

# ---- layout (Blender XY, metres). Bridge axis runs along +x at y = Y0 (saigon_mini_layout.json "cau_ba_son").
Y0 = 165.0
XA, XB = 40.0, 87.0        # deck ends: east kerb of Tôn Đức Thắng, start of the Thủ Thiêm road
XC = 61.0                  # crown over the middle of the river (river x 48..74)
Z_ROAD = 0.07              # street asphalt level at both ends
H = 3.0                    # crown rise: deck top 3.07, underside ~2.4, ~2.8 m above the water
ROAD = 4.0                 # half width of the two-lane road
CURB = 4.2
WALK = 6.2                 # sidewalk outer edge
PARA = 6.45                # parapet outer face
GIRDER = 0.7               # deck depth under the road surface
XP = 79.0                  # pylon base on the Thủ Thiêm bank
HP = 32.0                  # pylon height
LEAN = 8.5                 # how far the top leans out over the main span (towards -x)
B0, B1 = 7.9, 0.9          # leg half-spread at the foot and at the top

PAL = {'bridgeWhite': '#EEF0EE', 'finSilver': '#D3DAE0', 'asphalt': '#6F6D69', 'paving': '#BDB4A2', 'curb': '#D3CBB8',
       'concrete': '#ABA69C', 'roadLine': '#ECE6D4', 'gold': '#C9A24A', 'lampGlow': '#FFE6A8'}


def lin(h):
    h = h.lstrip('#')
    c = [int(h[i:i + 2], 16) / 255.0 for i in (0, 2, 4)]
    return tuple(x / 12.92 if x <= 0.04045 else ((x + 0.055) / 1.055) ** 2.4 for x in c)


def get_mat(key):
    m = bpy.data.materials.get('SD_' + key)
    if m is not None:
        return m
    # same paint setup as the rest of the file: copy a sibling and recolour its SD_Paint input
    m = bpy.data.materials['SD_bridgeWhite'].copy()
    m.name = 'SD_' + key
    c = lin(PAL[key])
    for n in m.node_tree.nodes:
        if n.type == 'GROUP' and 'Color' in n.inputs:
            n.inputs['Color'].default_value = (*c, 1.0)
    m.diffuse_color = (*c, 1.0)
    return m


class Geo:
    def __init__(self):
        self.v, self.f, self.fm, self.mats = [], [], [], []

    def raw(self, mat, V, F):
        if mat not in self.mats:
            self.mats.append(mat)
        k = self.mats.index(mat)
        b = len(self.v)
        self.v.extend(tuple(p) for p in V)
        for f in F:
            self.f.append(tuple(b + i for i in f))
            self.fm.append(k)

    def box(self, mat, x0, y0, z0, x1, y1, z1):
        x0, x1 = sorted((x0, x1)); y0, y1 = sorted((y0, y1)); z0, z1 = sorted((z0, z1))
        V = [(x0, y0, z0), (x1, y0, z0), (x1, y1, z0), (x0, y1, z0), (x0, y0, z1), (x1, y0, z1), (x1, y1, z1), (x0, y1, z1)]
        self.raw(mat, V, [(0, 3, 2, 1), (4, 5, 6, 7), (0, 1, 5, 4), (1, 2, 6, 5), (2, 3, 7, 6), (3, 0, 4, 7)])

    def ring(self, mat, M, r, h, seg=12, r2=None, caps=True):
        r2 = r if r2 is None else r2
        V = [M @ Vector((r * math.cos(2 * PI * i / seg), r * math.sin(2 * PI * i / seg), 0)) for i in range(seg)]
        V += [M @ Vector((r2 * math.cos(2 * PI * i / seg), r2 * math.sin(2 * PI * i / seg), h)) for i in range(seg)]
        F = [(i, (i + 1) % seg, seg + (i + 1) % seg, seg + i) for i in range(seg)]
        if caps:
            F += [tuple(range(seg - 1, -1, -1)), tuple(range(seg, 2 * seg))]
        self.raw(mat, V, F)

    def rod(self, mat, a, b, r=0.05, seg=6):
        a, b = Vector(a), Vector(b)
        d = b - a
        if d.length < 1e-6:
            return
        q = Vector((0, 0, 1)).rotation_difference(d)
        self.ring(mat, Matrix.Translation(a) @ q.to_matrix().to_4x4(), r, d.length, seg)

    def ico(self, mat, c, r):
        bm = bmesh.new()
        bmesh.ops.create_icosphere(bm, subdivisions=1, radius=r)
        bm.verts.index_update()
        V = [Vector(c) + v.co for v in bm.verts]
        F = [tuple(v.index for v in f.verts) for f in bm.faces]
        bm.free()
        self.raw(mat, V, F)

    def sweep(self, mat, P, u0, u1, v0, v1, caps=True):
        """box section, lateral u0..u1 (along +y) and vertical v0..v1, swept along the deck path P"""
        V = []
        for p in P:
            for (u, v) in ((u0, v0), (u1, v0), (u1, v1), (u0, v1)):
                V.append((p.x, p.y + u, p.z + v))
        n = len(P)
        F = []
        for i in range(n - 1):
            a, b = 4 * i, 4 * (i + 1)
            for k in range(4):
                k2 = (k + 1) % 4
                F.append((a + k, b + k, b + k2, a + k2))
        if caps:
            F.append((0, 1, 2, 3))
            m = 4 * (n - 1)
            F.append((m + 3, m + 2, m + 1, m))
        self.raw(mat, V, F)

    def tube(self, mat, P, W, D, ref=(0, 0, 1)):
        """rectangular tube along a 3D path; W lateral (along ref x tangent), D the other way"""
        n = len(P)
        V = []
        for i in range(n):
            t = (P[min(i + 1, n - 1)] - P[max(i - 1, 0)]).normalized()
            a = Vector(ref).cross(t)
            if a.length < 1e-4:
                a = Vector((1, 0, 0)).cross(t)
            a.normalize()
            b = t.cross(a)
            for (u, v) in ((-1, -1), (1, -1), (1, 1), (-1, 1)):
                V.append(P[i] + a * (u * W[i] / 2) + b * (v * D[i] / 2))
        F = []
        for i in range(n - 1):
            a0, b0 = 4 * i, 4 * (i + 1)
            for k in range(4):
                k2 = (k + 1) % 4
                F.append((a0 + k, a0 + k2, b0 + k2, b0 + k))
        F.append((3, 2, 1, 0))
        m = 4 * (n - 1)
        F.append((m, m + 1, m + 2, m + 3))
        self.raw(mat, V, F)


def zdeck(x):
    """road surface height: flat into the streets at both ends, a smooth hump with its crown over the river"""
    x = min(max(x, XA), XB)
    s = (x - XA) / (XC - XA) if x <= XC else (XB - x) / (XB - XC)
    return Z_ROAD + H * (1 - math.cos(PI * s)) / 2


def deck_path(y=Y0, step=0.5):
    n = int(round((XB - XA) / step))
    return [Vector((XA + i * step, y, zdeck(XA + i * step))) for i in range(n + 1)]


def leg(sy, t):
    return Vector((XP - LEAN * t ** 1.6, Y0 + sy * (B0 * (1 - t) + B1 * t), -0.3 + HP * t))


def lamp(g, x, sy):
    """gold lamp post on the kerb, arm reaching over the road; it stays inside the cable planes"""
    y = Y0 + sy * (ROAD + 0.1)
    z = zdeck(x) + 0.2
    g.ring('gold', Matrix.Translation((x, y, z)), 0.2, 0.45, 10)
    g.ring('gold', Matrix.Translation((x, y, z)), 0.11, 5.4, 10, r2=0.075)
    pts = [Vector((x, y, z + 5.4))]
    for k in range(1, 7):
        a = PI / 2 * k / 6
        pts.append(Vector((x, y - sy * 1.1 * math.sin(a), z + 5.4 + 0.5 * math.sin(a))))
    for a, b in zip(pts, pts[1:]):
        g.rod('gold', a, b, 0.065)
    e = pts[-1]
    g.ring('gold', Matrix.Translation((e.x, e.y - sy * 0.1, e.z - 0.05)) @ Matrix.Rotation(PI, 4, 'X'), 0.34, 0.36, 10, r2=0.06)
    g.ico('lampGlow', (e.x, e.y - sy * 0.1, e.z - 0.48), 0.2)


def build():
    g = Geo()
    P = deck_path()
    # deck: road, curbs, sidewalks, parapets, girder and its white fascia
    g.sweep('asphalt', P, -ROAD, ROAD, -0.12, 0.0)
    g.sweep('bridgeWhite', P, -PARA + 0.6, PARA - 0.6, -GIRDER, -0.12)
    for sy in (1, -1):
        def sw(mat, a, b, v0, v1):
            u0, u1 = sorted((sy * a, sy * b))
            g.sweep(mat, P, u0, u1, v0, v1)
        sw('concrete', ROAD, CURB, -0.12, 0.2)                       # kerb
        sw('paving', CURB, WALK, -0.12, 0.12)                        # sidewalk
        sw('bridgeWhite', WALK, PARA, -GIRDER - 0.1, 0.5)            # parapet wall + fascia, one white band
        sw('bridgeWhite', PARA - 0.75, PARA, -GIRDER - 0.18, -GIRDER + 0.05)   # drip edge under the fascia
        # handrail on posts
        sw('finSilver', WALK + 0.05, WALK + 0.2, 1.0, 1.1)
        k = 0
        x = XA + 0.6
        while x < XB - 0.4:
            yy = Y0 + sy * (WALK + 0.125)
            z = zdeck(x)
            g.box('finSilver', x - 0.05, yy - 0.05, z + 0.5, x + 0.05, yy + 0.05, z + 1.0)
            x += 1.2
            k += 1
    # centre line: 2 m dashes on a 4 m pitch, like the streets' IX_RoadMarks
    x = XA + 1.0
    while x + 2.0 < XB - 1.0:
        seg = [Vector((x + d, Y0, zdeck(x + d))) for d in (0.0, 0.5, 1.0, 1.5, 2.0)]
        g.sweep('roadLine', seg, -0.09, 0.09, 0.0, 0.012, caps=False)
        x += 4.0
    # bank walls under the deck where it leaves each bank of the river (x 48 and 74)
    for xw in (48.0, 74.0):
        top = zdeck(xw) - GIRDER
        g.box('bridgeWhite', xw - 0.6, Y0 - PARA + 0.3, -2.5, xw + 0.6, Y0 + PARA - 0.3, top)
        g.box('concrete', xw - 0.9, Y0 - PARA, -2.5, xw + 0.9, Y0 + PARA, -2.1)
    # pylon: two tapering legs straddling the deck, curving out over the main span and closing in a rounded crown
    ts = [i / 28 for i in range(29)]
    for sy in (1, -1):
        Lp = [leg(sy, t) for t in ts]
        g.tube('bridgeWhite', Lp, [1.7 - 0.8 * t for t in ts], [2.4 - 1.25 * t for t in ts], ref=(1, 0, 0))
        f = leg(sy, 0.0)
        g.box('concrete', f.x - 1.6, f.y - 1.4, -0.45, f.x + 1.6, f.y + 1.4, 0.25)   # pile cap
    top_r, top_l = leg(1, 1.0), leg(-1, 1.0)
    c = (top_r + top_l) / 2
    tdir = (leg(1, 1.0) - leg(1, 0.97)).normalized()
    tdir = (tdir - Vector((0, tdir.y, 0))).normalized()
    arc = [c + Vector((0, B1 * math.cos(PI * k / 14), 0)) + tdir * (B1 * 1.15 * math.sin(PI * k / 14)) for k in range(15)]
    g.tube('bridgeWhite', arc, [0.9] * 15, [1.15] * 15, ref=(1, 0, 0))
    # tie beam between the legs just above the deck traffic
    tb = 0.62
    a, b = leg(1, tb), leg(-1, tb)
    g.tube('bridgeWhite', [a + (b - a) * (k / 6) for k in range(7)], [0.9] * 7, [1.2] * 7, ref=(1, 0, 0))
    # stay cables: one inclined plane per leg, fanning over the main span to the parapet line; back-stays to the east end
    n_main = 10
    for sy in (1, -1):
        for i in range(n_main):
            t = 0.4 + 0.56 * i / (n_main - 1)
            xd = 73.0 - (73.0 - 45.0) * i / (n_main - 1)
            A = leg(sy, t)
            D = Vector((xd, Y0 + sy * (PARA - 0.2), zdeck(xd) + 0.5))
            g.rod('finSilver', A, D, 0.06)
            g.box('bridgeWhite', D.x - 0.22, D.y - 0.2, D.z - 0.1, D.x + 0.22, D.y + 0.2, D.z + 0.22)
        for i in range(4):
            t = 0.5 + 0.13 * i
            xd = 82.0 + 1.5 * i
            A = leg(sy, t)
            D = Vector((xd, Y0 + sy * (PARA - 0.2), zdeck(xd) + 0.5))
            g.rod('finSilver', A, D, 0.07)
            g.box('bridgeWhite', D.x - 0.25, D.y - 0.22, D.z - 0.1, D.x + 0.25, D.y + 0.22, D.z + 0.25)
    # lamps
    for x in (43.0, 51.0, 59.0, 67.0, 85.0):
        for sy in (1, -1):
            lamp(g, x, sy)
    # white newel posts with gold caps at the four corners
    for x in (XA + 0.35, XB - 0.35):
        for sy in (1, -1):
            y = Y0 + sy * (PARA - 0.35)
            z = zdeck(x)
            g.box('bridgeWhite', x - 0.38, y - 0.38, z - 0.2, x + 0.38, y + 0.38, z + 1.35)
            g.box('gold', x - 0.3, y - 0.3, z + 1.35, x + 0.3, y + 0.3, z + 1.5)
    return g


def make_object(g, name, coll):
    me = bpy.data.meshes.new(name)
    me.from_pydata([tuple(v) for v in g.v], [], g.f)
    me.polygons.foreach_set('material_index', g.fm)
    for k in g.mats:
        me.materials.append(get_mat(k))
    me.validate()
    try:
        me.shade_smooth()
        me.set_sharp_from_angle(angle=math.radians(30))
    except Exception:
        pass
    me.update()
    ob = bpy.data.objects.new(name, me)
    coll.objects.link(ob)
    return ob


def drop_blockout_ba_son():
    """remove the Ba Son half of the blockout LM_Bridges (everything north of y = 100); the Hàm Nghi half stays"""
    ob = bpy.data.objects.get('LM_Bridges')
    if ob is None:
        return 0
    bm = bmesh.new()
    bm.from_mesh(ob.data)
    mw = ob.matrix_world
    kill = [f for f in bm.faces if (mw @ f.calc_center_median()).y > 100.0]
    bmesh.ops.delete(bm, geom=kill, context='FACES')
    bm.to_mesh(ob.data)
    bm.free()
    ob.data.update()
    return len(kill)


def run():
    old = bpy.data.objects.get('LM_CauBaSon')
    if old is not None:
        bpy.data.objects.remove(old, do_unlink=True)
    removed = drop_blockout_ba_son()
    coll = bpy.data.collections.get('SM_Landmarks') or bpy.context.scene.collection
    ob = make_object(build(), 'LM_CauBaSon', coll)
    return {'object': ob.name, 'verts': len(ob.data.vertices), 'faces': len(ob.data.polygons), 'blockout_faces_removed': removed}


if __name__ == '__main__':
    print('CAU_BA_SON', run())
