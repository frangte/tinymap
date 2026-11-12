"""Small shared toolkit for the part scripts in scripts/blender/mini/: the file's SD_ paint materials, a mesh builder
that gathers faces per material, object helpers and the terrain helpers (grid triangulation over a polygon).

Everything works on the open file (interactive or --background) and reuses the materials already in it.
"""
import math

import bmesh
import bpy
import numpy as np
from mathutils import Matrix, Vector

PI = math.pi
# fallback colours for keys a file may not have yet (docs/PROJECT.md palette)
PAL = {
    'bridgeWhite': '#EEF0EE', 'finSilver': '#D3DAE0', 'asphalt': '#6F6D69', 'paving': '#BDB4A2', 'curb': '#D3CBB8',
    'concrete': '#ABA69C', 'roadLine': '#ECE6D4', 'gold': '#C9A24A', 'lampGlow': '#FFE6A8', 'stone': '#E6DCC8',
    'rock': '#8E877A', 'rockDark': '#6C665B', 'grass': '#86A85E', 'grassDark': '#62874A', 'leaf': '#6E9B4C',
    'leafDark': '#4D7539', 'leafLight': '#93B85F', 'bark': '#6B4C36', 'flowerPink': '#E08AA6', 'flowerRed': '#D2483F',
    'opWhite': '#EEE7D7', 'archDark': '#3B3330', 'glassBlue': '#7FA0B5', 'roofRed': '#A9492F', 'roofRedDark': '#7D3326',
    'statueW': '#F2EFE8', 'creamLight': '#F4EBD3', 'cream': '#EAD9AE', 'slate': '#646B74', 'white': '#F4F1EA',
    'sand': '#CDB98F', 'wood': '#8A5E3C', 'metal': '#6B6F73', 'poGreen': '#3E7F5F', 'towerPale': '#DAD4C8',
    'glassDark': '#4F6E80', 'signRed': '#B83A2B', 'signGold': '#D9B45A', 'sigRed': '#C8553D', 'water': '#6FA9B6',
    'foam': '#F1F3EE', 'earth': '#8E6D4E', 'flowerYellow': '#EBC64C',
}


def lin(h):
    h = h.lstrip('#')
    c = [int(h[i:i + 2], 16) / 255.0 for i in (0, 2, 4)]
    return tuple(x / 12.92 if x <= 0.04045 else ((x + 0.055) / 1.055) ** 2.4 for x in c)


def get_mat(key):
    """the file's SD_<key> paint material; a missing key is made by copying a sibling and recolouring its SD_Paint"""
    m = bpy.data.materials.get('SD_' + key)
    if m is not None:
        return m
    src = next((bpy.data.materials[n] for n in ('SD_stone', 'SD_concrete', 'SD_bridgeWhite') if n in bpy.data.materials), None)
    m = src.copy() if src else bpy.data.materials.new('SD_' + key)
    m.name = 'SD_' + key
    c = lin(PAL[key])
    if m.node_tree:
        for n in m.node_tree.nodes:
            if n.type == 'GROUP' and 'Color' in n.inputs:
                n.inputs['Color'].default_value = (*c, 1.0)
    m.diffuse_color = (*c, 1.0)
    return m


def smoothstep(a, b, x):
    t = np.clip((np.asarray(x, dtype=float) - a) / (b - a), 0.0, 1.0)
    return t * t * (3 - 2 * t)


class Mesh:
    """faces gathered per material; every primitive takes world coordinates (or a Matrix M)"""

    def __init__(self, seed=0):
        self.v, self.f, self.fm, self.mats = [], [], [], []
        self.rng = np.random.default_rng(seed)

    def raw(self, mat, V, F, M=None):
        if mat not in self.mats:
            self.mats.append(mat)
        k = self.mats.index(mat)
        b = len(self.v)
        self.v.extend(tuple(M @ Vector(p)) if M is not None else tuple(p) for p in V)
        for f in F:
            self.f.append(tuple(b + i for i in f))
            self.fm.append(k)

    def box(self, mat, x0, y0, z0, x1, y1, z1, M=None):
        x0, x1 = sorted((x0, x1)); y0, y1 = sorted((y0, y1)); z0, z1 = sorted((z0, z1))
        V = [(x0, y0, z0), (x1, y0, z0), (x1, y1, z0), (x0, y1, z0), (x0, y0, z1), (x1, y0, z1), (x1, y1, z1), (x0, y1, z1)]
        self.raw(mat, V, [(0, 3, 2, 1), (4, 5, 6, 7), (0, 1, 5, 4), (1, 2, 6, 5), (2, 3, 7, 6), (3, 0, 4, 7)], M)

    def ring(self, mat, M, r, h, seg=12, r2=None, caps=True):
        """cylinder / frustum along local +Z of M"""
        r2 = r if r2 is None else r2
        V = [(r * math.cos(2 * PI * i / seg), r * math.sin(2 * PI * i / seg), 0) for i in range(seg)]
        V += [(r2 * math.cos(2 * PI * i / seg), r2 * math.sin(2 * PI * i / seg), h) for i in range(seg)]
        F = [(i, (i + 1) % seg, seg + (i + 1) % seg, seg + i) for i in range(seg)]
        if caps:
            F += [tuple(range(seg - 1, -1, -1)), tuple(range(seg, 2 * seg))]
        self.raw(mat, V, F, M)

    def lathe(self, mat, M, prof, seg=12, cap_top=True, cap_bottom=True):
        """surface of revolution of (radius, z) pairs about local +Z"""
        n = len(prof)
        V = [(r * math.cos(2 * PI * i / seg), r * math.sin(2 * PI * i / seg), z) for r, z in prof for i in range(seg)]
        F = []
        for k in range(n - 1):
            for i in range(seg):
                j = (i + 1) % seg
                F.append((k * seg + i, k * seg + j, (k + 1) * seg + j, (k + 1) * seg + i))
        if cap_bottom:
            F.append(tuple(range(seg - 1, -1, -1)))
        if cap_top:
            F.append(tuple(range((n - 1) * seg, n * seg)))
        self.raw(mat, V, F, M)

    def rod(self, mat, a, b, r=0.05, seg=6):
        a, b = Vector(a), Vector(b)
        d = b - a
        if d.length < 1e-6:
            return
        q = Vector((0, 0, 1)).rotation_difference(d)
        self.ring(mat, Matrix.Translation(a) @ q.to_matrix().to_4x4(), r, d.length, seg)

    def ico(self, mat, c, r, sub=1, scale=(1, 1, 1), jit=0.0, rng=None):
        bm = bmesh.new()
        bmesh.ops.create_icosphere(bm, subdivisions=sub, radius=r)
        bm.verts.index_update()
        rng = rng or self.rng
        V = []
        for v in bm.verts:
            k = 1.0 + (rng.uniform(-jit, jit) if jit else 0.0)
            V.append((c[0] + v.co.x * k * scale[0], c[1] + v.co.y * k * scale[1], c[2] + v.co.z * k * scale[2]))
        F = [tuple(v.index for v in f.verts) for f in bm.faces]
        bm.free()
        self.raw(mat, V, F)

    def ribbon(self, mat, P, half, v0, v1, caps=True, lat=None):
        """box section swept along a 3D path P (list of Vector): lateral -half..half, vertical v0..v1"""
        n = len(P)
        V = []
        for i in range(n):
            t = P[min(i + 1, n - 1)] - P[max(i - 1, 0)]
            t.z = 0
            t.normalize()
            s = Vector((-t.y, t.x, 0)) if lat is None else lat
            for (u, v) in ((-half, v0), (half, v0), (half, v1), (-half, v1)):
                V.append(tuple(P[i] + s * u + Vector((0, 0, v))))
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

    def tris(self, V, T, mats):
        """a triangle soup: shared vertex list V, triangles T, one material key per triangle"""
        for t, mk in zip(T, mats):
            self.raw(mk, [V[t[0]], V[t[1]], V[t[2]]], [(0, 1, 2)])


def make_object(g, name, coll, smooth_angle=30, weld=False, recalc=False):
    """weld merges coincident vertices (terrain); recalc points every face of closed parts outwards (models)"""
    me = bpy.data.meshes.new(name)
    me.from_pydata([tuple(v) for v in g.v], [], g.f)
    me.polygons.foreach_set('material_index', g.fm)
    for k in g.mats:
        me.materials.append(get_mat(k))
    me.validate()
    if weld or recalc:
        bm = bmesh.new()
        bm.from_mesh(me)
        if weld:
            bmesh.ops.remove_doubles(bm, verts=bm.verts, dist=1e-4)
        if recalc:
            bmesh.ops.recalc_face_normals(bm, faces=bm.faces)
        bm.to_mesh(me)
        bm.free()
    try:
        me.shade_smooth()
        me.set_sharp_from_angle(angle=math.radians(smooth_angle))
    except Exception:
        pass
    me.update()
    ob = bpy.data.objects.new(name, me)
    coll.objects.link(ob)
    return ob


def remove_objects(names):
    for n in names:
        ob = bpy.data.objects.get(n)
        if ob is not None:
            me = ob.data
            bpy.data.objects.remove(ob, do_unlink=True)
            if me is not None and me.users == 0:
                bpy.data.meshes.remove(me)


def collection(name):
    c = bpy.data.collections.get(name)
    if c is None:
        c = bpy.data.collections.new(name)
        (bpy.data.collections.get('SD_World') or bpy.context.scene.collection).children.link(c)
    return c


def drop_loose_parts(obname, inside):
    """delete the loose parts of an object whose centre satisfies inside(x, y) (world XY); returns how many went"""
    ob = bpy.data.objects.get(obname)
    if ob is None:
        return 0
    bm = bmesh.new()
    bm.from_mesh(ob.data)
    bm.verts.ensure_lookup_table()
    mw = ob.matrix_world
    parent = list(range(len(bm.verts)))

    def find(a):
        while parent[a] != a:
            parent[a] = parent[parent[a]]
            a = parent[a]
        return a
    for e in bm.edges:
        a, b = find(e.verts[0].index), find(e.verts[1].index)
        if a != b:
            parent[a] = b
    sums = {}
    for v in bm.verts:
        r = find(v.index)
        p = mw @ v.co
        s = sums.setdefault(r, [0.0, 0.0, 0])
        s[0] += p.x
        s[1] += p.y
        s[2] += 1
    kill_roots = {r for r, (sx, sy, n) in sums.items() if inside(sx / n, sy / n)}
    kill = [v for v in bm.verts if find(v.index) in kill_roots]
    bmesh.ops.delete(bm, geom=kill, context='VERTS')
    bm.to_mesh(ob.data)
    bm.free()
    ob.data.update()
    return len(kill_roots)


def tri_grid(poly, step):
    """points on a grid inside poly plus its outline, Delaunay triangles kept inside -> (xy array, triangles)"""
    import shapely
    from shapely.geometry import LineString
    minx, miny, maxx, maxy = poly.bounds
    gx, gy = np.meshgrid(np.arange(minx + step / 2, maxx, step), np.arange(miny + step / 2, maxy, step))
    pts = np.c_[gx.ravel(), gy.ravel()]
    # a little jitter so the facets do not read as a grid
    pts = pts + np.random.default_rng(5).uniform(-0.22, 0.22, pts.shape) * step
    pts = pts[shapely.contains_xy(poly.buffer(-step * 0.35), pts[:, 0], pts[:, 1])]
    ring = []
    for p in getattr(poly, 'geoms', [poly]):
        for r in [p.exterior, *p.interiors]:
            ln = LineString(r.coords)
            n = max(8, int(ln.length / (step * 0.8)))
            ring += [ln.interpolate(i / n, normalized=True).coords[0] for i in range(n)]
    allp = np.vstack([pts, np.asarray(ring)])
    tg = shapely.delaunay_triangles(shapely.multipoints(allp))
    tri = shapely.get_coordinates(tg).reshape(-1, 4, 2)[:, :3, :]
    cen = tri.mean(axis=1)
    keep = shapely.contains_xy(poly, cen[:, 0], cen[:, 1])
    index = {(round(x, 4), round(y, 4)): i for i, (x, y) in enumerate(allp)}
    out = []
    for t in tri[keep]:
        a, b, c = (index[(round(x, 4), round(y, 4))] for x, y in t)
        (ax, ay), (bx, by), (cx, cy) = allp[a], allp[b], allp[c]
        out.append((a, b, c) if (bx - ax) * (cy - ay) - (by - ay) * (cx - ax) >= 0 else (a, c, b))
    return allp, out


def add_wobble(obname, k):
    """the hand-drawn wobble of the art pass (saigon_diorama_17_art_pass.py): LM_ 0.045, others 0.07; modern steel,
    glass, roads and the island stay straight. build_saigon_mini.py runs the art pass after the part scripts, so this
    only matters when a part script runs alone on an open file."""
    ob = bpy.data.objects.get(obname)
    tex = bpy.data.textures.get('SD_Wobble')
    if ob is None or tex is None:
        return
    m = ob.modifiers.get('SD_HandWobble') or ob.modifiers.new('SD_HandWobble', 'DISPLACE')
    m.texture = tex
    m.texture_coords = 'GLOBAL'
    m.direction = 'RGB_TO_XYZ'
    m.mid_level = 0.5
    m.strength = k * 2


def move_mesh(obname, dx, dy, dz=0.0):
    """move an object's mesh in place (the origin stays at the world origin, like the other map objects)"""
    ob = bpy.data.objects.get(obname)
    if ob is not None:
        ob.data.transform(Matrix.Translation((dx, dy, dz)))
        ob.data.update()
    return ob


def _loose_parts(ob):
    """[(face indices, world centre, (x0, x1, y0, y1, z0, z1), {material key: faces})] of an object's loose parts"""
    import collections
    me = ob.data
    mw = ob.matrix_world
    par = list(range(len(me.vertices)))

    def find(a):
        while par[a] != a:
            par[a] = par[par[a]]
            a = par[a]
        return a
    for p in me.polygons:
        r0 = find(p.vertices[0])
        for v in p.vertices[1:]:
            r = find(v)
            if r != r0:
                par[r] = r0
    comps = collections.defaultdict(list)
    for p in me.polygons:
        comps[find(p.vertices[0])].append(p.index)
    co = [mw @ v.co for v in me.vertices]
    keys = []
    for m in me.materials:
        n = m.name if m else ''
        n = n[3:] if n.startswith('SD_') else n
        keys.append(n.split('.')[0])
    out = []
    for fl in comps.values():
        vids = {v for i in fl for v in me.polygons[i].vertices}
        xs = [co[v].x for v in vids]
        ys = [co[v].y for v in vids]
        zs = [co[v].z for v in vids]
        mats = collections.Counter(keys[me.polygons[i].material_index] if keys else '' for i in fl)
        out.append((fl, Vector((sum(xs) / len(xs), sum(ys) / len(ys), sum(zs) / len(zs))),
                    (min(xs), max(xs), min(ys), max(ys), min(zs), max(zs)), mats))
    return out


PEOPLE_KEEP = {'plasticRed', 'plasticBlue', 'metal', 'stone', 'paving', 'concrete', 'water', 'foam', 'towerPale',
               'concreteLight', 'lampGlow', 'gold', 'lantern'}


def strip_figures(obname, people=True, scooters=True, boxes=()):
    """Take people and parked scooters out of an object (they come back as NPCs in the game, by code):
    - a person: a small skin-coloured head above the ground, with the parts round it and below it (body, arms, legs,
      hair, hat), but not the stools, tables or lamps they sit at (PEOPLE_KEEP materials);
    - a scooter: a pair of wheels about 1.2 m apart, with every small part between them (body, seat, bars, rider);
    - boxes: extra world boxes (x0, y0, x1, y1, zmax) whose parts all go (a van parked in the street).
    Returns how many of each went."""
    ob = bpy.data.objects.get(obname)
    if ob is None or ob.type != 'MESH':
        return {}
    parts = _loose_parts(ob)

    def dims(p):
        b = p[2]
        return (b[1] - b[0], b[3] - b[2], b[5] - b[4])
    kill = set()
    n_people = n_scooters = n_box = 0
    if people:
        heads = [p for p in parts if p[3].most_common(1)[0][0] == 'skin' and max(dims(p)) < 0.45 and p[2][4] > 0.3]
        for h in heads:
            n_people += 1
            for k, p in enumerate(parts):
                if p is h:
                    kill.add(k)
                    continue
                if (math.dist(p[1][:2], h[1][:2]) < 0.55 and p[2][5] <= h[2][5] + 0.35 and p[2][4] >= h[2][4] - 2.0
                        and max(dims(p)) < 1.2 and not (set(p[3]) & PEOPLE_KEEP)):
                    kill.add(k)
    if scooters:
        wheels = []
        for k, p in enumerate(parts):
            dx, dy, dz = dims(p)
            thin, wide = sorted((dx, dy))
            if 0.45 < dz < 0.7 and 0.04 < thin < 0.22 and 0.45 < wide < 0.7:
                wheels.append(k)
        used = set()
        for i in wheels:
            if i in used:
                continue
            best = None
            for j in wheels:
                if j == i or j in used:
                    continue
                d = math.dist(parts[i][1][:2], parts[j][1][:2])
                if 1.0 < d < 1.45 and abs(parts[i][2][4] - parts[j][2][4]) < 0.1 and (best is None or abs(d - 1.2) < best[0]):
                    best = (abs(d - 1.2), j)
            if best is None:
                continue
            j = best[1]
            used |= {i, j}
            n_scooters += 1
            a, b = parts[i][1], parts[j][1]
            m = (a + b) / 2
            u = Vector((b.x - a.x, b.y - a.y))
            u.normalize()
            ground = min(parts[i][2][4], parts[j][2][4])
            for k, p in enumerate(parts):
                d = p[1] - m
                along, across = d.x * u.x + d.y * u.y, -d.x * u.y + d.y * u.x
                if abs(along) < 1.15 and abs(across) < 0.5 and p[2][5] < ground + 1.9 and max(dims(p)) < 1.3:
                    kill.add(k)
    for x0, y0, x1, y1, zmax in boxes:
        for k, p in enumerate(parts):
            if x0 < p[1].x < x1 and y0 < p[1].y < y1 and p[2][5] < zmax:
                kill.add(k)
                n_box += 1
    if kill:
        faces = {i for k in kill for i in parts[k][0]}
        bm = bmesh.new()
        bm.from_mesh(ob.data)
        bm.faces.ensure_lookup_table()
        bmesh.ops.delete(bm, geom=[bm.faces[i] for i in faces], context='FACES')
        loose = [v for v in bm.verts if not v.link_faces]
        bmesh.ops.delete(bm, geom=loose, context='VERTS')
        bm.to_mesh(ob.data)
        bm.free()
        ob.data.update()
    return {'people': n_people, 'scooters': n_scooters, 'box_parts': n_box, 'parts_removed': len(kill)}


def drop_faces_near(obname, points, radius):
    """delete the faces of an object whose centre lies within radius (XY) of any of the points"""
    ob = bpy.data.objects.get(obname)
    if ob is None:
        return 0
    bm = bmesh.new()
    bm.from_mesh(ob.data)
    mw = ob.matrix_world
    kill = []
    for f in bm.faces:
        c = mw @ f.calc_center_median()
        if any((c.x - px) ** 2 + (c.y - py) ** 2 < radius * radius for px, py in points):
            kill.append(f)
    bmesh.ops.delete(bm, geom=kill, context='FACES')
    bm.to_mesh(ob.data)
    bm.free()
    ob.data.update()
    return len(kill)
