"""v4 map edits of 2026-11-07, applied to the GLB re-import (new.sg.tinymap.blend, meshes merged by material):

    blender new.sg.tinymap.blend --background --python scripts/blender/edit_v4_buivien.py -- temp.sg.tinymap.blend [stages]

Stages (default all, comma separated):
  lamps    remove the 4-globe lamp standing in the market's south gate and the one on the roundabout's east mouth
  zebra    drop the doubled / crossing zebra stripes painted on the ring road, one clean crossing on the Le Lai arm
  road     Ton Duc Thang: Ham Nghi no longer dead-ends at Bitexco, it bends round into Dong Khoi (old stub removed)
  buivien  Bui Vien bar street on Le Lai: tube houses on both sides, signs, lanterns, string lights, tables, scooters

The builders (Geo, house parts, scooter, tile roofs) are the Text blocks of the root sg.tinymap.blend, loaded from
there; the merged file has no Text blocks. New geometry goes into new objects (IX_Roads_TonDucThang,
IX_RoadMarks_TonDucThang, IX_RoadMarks_LeLaiZebra, SH_BuiVien_N/S/Life/Lights); deletions and cuts are made in
place in the merged meshes. The nav maps are NOT rebuilt by this (bake_nav.py cannot run on the merged file).
"""
import ast, math, os, random, sys
import bpy, bmesh
from mathutils import Vector, Matrix

ARGS = sys.argv[sys.argv.index('--') + 1:]
OUT = os.path.abspath(ARGS[0])
STAGES = set(ARGS[1].split(',')) if len(ARGS) > 1 else {'lamps', 'zebra', 'road', 'buivien'}
HERE = os.path.dirname(os.path.abspath(__file__))
SRC_BLEND = os.path.join(HERE, '..', '..', 'sg.tinymap.blend')
PI = math.pi
_texts = {}

def src_text(name):
    """a Text block of the original diorama file"""
    if not _texts:
        with bpy.data.libraries.load(os.path.abspath(SRC_BLEND), link=False) as (src, dst):
            dst.texts = [n for n in src.texts if n.endswith('.py')]
        for t in dst.texts:
            _texts[t.name] = t.as_string(); bpy.data.texts.remove(t)
    return _texts[name]

# ----------------------------------------------------------------------------------------------------------
# builder library from the original .blend text blocks, with the SD_ palette and plain Principled materials
# ----------------------------------------------------------------------------------------------------------
SD_PAL = {
 'cream':'#EAD9AE','creamLight':'#F4EBD3','creamShade':'#D9C394','roofRed':'#A9492F','roofRedDark':'#7D3326','roofRedLight':'#C0613F',
 'clockBlue':'#4E7CA6','clockWhite':'#F3EEE2','brownText':'#5E3F26','gold':'#C9A24A','archDark':'#3B3330','glassTeal':'#4F7A7C',
 'paving':'#BDB4A2','pavingDark':'#A69D8A','asphalt':'#6F6D69','roadLine':'#ECE6D4','curb':'#D3CBB8',
 'grass':'#86A85E','grassDark':'#62874A','leaf':'#6E9B4C','leafDark':'#4D7539','leafLight':'#93B85F','bark':'#6B4C36',
 'earth':'#8E6D4E','earthDark':'#5F4735','rock':'#8E877A','rockDark':'#6C665B','water':'#6FA9B6',
 'wallGrey':'#A0A7AA','wallTeal':'#5E9A96','wallOchre':'#C9A25E','wallBeige':'#CFC1A0','wallPink':'#D29A86',
 'tinBlue':'#8AA3B6','tinWhite':'#DDE4E7','signRed':'#B83A2B','signGold':'#D9B45A','lantern':'#C8372D','wood':'#8A5E3C',
 'concrete':'#ABA69C','metal':'#6B6F73','pipeYellow':'#C9A03D','clothBlue':'#4D6E9A','clothRed':'#B84A3A','clothYellow':'#E0B648',
 'skin':'#E7B897','hair':'#2E2622','plasticBlue':'#4E86B8','plasticRed':'#C9443A','jarPlum':'#6E4F5E','acWhite':'#E3E1D8',
 'shopDark':'#40342D','interior':'#8D6A50','flowerRed':'#D2483F','flowerPink':'#E08AA6','flowerYellow':'#EBC64C','white':'#F4F1EA',
 'lampGlow':'#FFE6A8','ink':'#2B211C',
 # landmark colours already present in the file (docs/PROJECT.md palette)
 'poYellow':'#E0B45A','poGreen':'#3E7F5F','turq':'#5BA9A1','brick':'#B5523B','stone':'#E6DCC8','opWhite':'#EEE7D7','opShade':'#D8CDB8',
}

def lin(h):
    h = h.lstrip('#'); c = [int(h[i:i + 2], 16) / 255.0 for i in (0, 2, 4)]
    return tuple(x / 12.92 if x <= 0.04045 else ((x + 0.055) / 1.055) ** 2.4 for x in c)

def get_mat(n):
    m = bpy.data.materials.get('SD_' + n)
    if m is not None:
        return m
    m = bpy.data.materials.new('SD_' + n); m.use_nodes = True
    nt = m.node_tree
    for x in list(nt.nodes): nt.nodes.remove(x)
    out = nt.nodes.new('ShaderNodeOutputMaterial'); bs = nt.nodes.new('ShaderNodeBsdfPrincipled')
    c = lin(SD_PAL[n]); bs.inputs['Base Color'].default_value = (*c, 1); bs.inputs['Roughness'].default_value = 1.0
    nt.links.new(bs.outputs[0], out.inputs[0]); m.diffuse_color = (*c, 1)
    print('[edit] new material SD_' + n)
    return m

ns = {}
exec(src_text('saigon_mini_lib.py'), ns)
ns['PAL'] = dict(SD_PAL); ns['VAR'] = {}; ns['GRIME'] = {}; ns['get_mat'] = get_mat
exec(src_text('saigon_diorama_04_street.py'), ns)

def exec_defs(fname, names):
    src = src_text(fname); tree = ast.parse(src)
    for node in tree.body:
        if isinstance(node, ast.FunctionDef) and node.name in names:
            exec(compile(ast.Module([node], []), fname, 'exec'), ns)
exec_defs('saigon_diorama_02_market.py', {'tile_slope'})
exec_defs('saigon_diorama_05_life.py', {'scooter'})

Geo = ns['Geo']; T = ns['T']; Rm = ns['Rm']; Sc = ns['Sc']; FACE_S = ns['FACE_S']; AX_NY = ns['AX_NY']; AX_Y = ns['AX_Y']
stool = ns['stool']; lantern = ns['lantern']; awning_striped = ns['awning_striped']; tile_slope = ns['tile_slope']; scooter = ns['scooter']
COLL = bpy.data.collections['Collection']

def add_obj(g, name, wobble=0.0):
    old = bpy.data.objects.get(name)
    if old: bpy.data.objects.remove(old, do_unlink=True)
    ob = ns['make_obj'](g, name, COLL, sub=0, angle=30, recenter=True)
    if wobble:
        tex = bpy.data.textures.get('SD_Wobble') or bpy.data.textures.new('SD_Wobble', 'CLOUDS')
        tex.noise_scale = 2.6; tex.noise_depth = 1; tex.noise_basis = 'ORIGINAL_PERLIN'; tex.cloud_type = 'COLOR'
        m = ob.modifiers.new('SD_HandWobble', 'DISPLACE')
        m.texture = tex; m.texture_coords = 'GLOBAL'; m.direction = 'RGB_TO_XYZ'; m.mid_level = 0.5; m.strength = wobble * 2
    print('[edit] object', name, len(ob.data.vertices), 'verts')
    return ob

# ----------------------------------------------------------------------------------------------------------
# mesh surgery on the merged meshes (uniform scale + translation only, so local = (world - loc) / s)
# ----------------------------------------------------------------------------------------------------------
GROUND = {'SD_paving', 'SD_grass', 'SD_sand', 'SD_asphalt', 'SD_curb', 'SD_roadLine', 'SD_water', 'SD_foam', 'SD_earth'}

def to_local(ob, p):
    s = ob.scale.x; l = ob.location
    return ((p[0] - l.x) / s, (p[1] - l.y) / s)

def components(ob):
    """[(materials set, world bbox (x0,x1,y0,y1,z0,z1), face index list)] of vertex-connected parts"""
    m = ob.data; mw = ob.matrix_world; co = [mw @ v.co for v in m.vertices]
    parent = list(range(len(co)))
    def f(i):
        while parent[i] != i:
            parent[i] = parent[parent[i]]; i = parent[i]
        return i
    for p in m.polygons:
        r = f(p.vertices[0])
        for v in p.vertices[1:]:
            rv = f(v)
            if rv != r: parent[rv] = r
    g = {}
    for p in m.polygons:
        g.setdefault(f(p.vertices[0]), [set(), [], []])
    for p in m.polygons:
        e = g[f(p.vertices[0])]; e[0].add(ob.material_slots[p.material_index].material.name); e[1].append(p.index)
    out = []
    for r, (mats, faces, _) in g.items():
        vs = {v for fi in faces for v in m.polygons[fi].vertices}
        xs = [co[i].x for i in vs]; ys = [co[i].y for i in vs]; zs = [co[i].z for i in vs]
        out.append((mats, (min(xs), max(xs), min(ys), max(ys), min(zs), max(zs)), faces))
    return out

def delete_faces(ob, face_indices):
    if not face_indices: return
    bm = bmesh.new(); bm.from_mesh(ob.data); bm.faces.ensure_lookup_table()
    bmesh.ops.delete(bm, geom=[bm.faces[i] for i in face_indices], context='FACES')
    bm.to_mesh(ob.data); bm.free(); ob.data.update()

def delete_components(ob, pred, label=''):
    """pred(mats, bbox) -> bool; deletes matching vertex-connected components"""
    doomed = []; n = 0
    for mats, bb, faces in components(ob):
        if pred(mats, bb): doomed += faces; n += 1
    delete_faces(ob, doomed)
    if n: print('[edit] %s: deleted %d components (%d faces) from %s' % (label, n, len(doomed), ob.name))
    return n

def point_in_poly(x, y, poly):
    inside = False; n = len(poly); j = n - 1
    for i in range(n):
        xi, yi = poly[i]; xj, yj = poly[j]
        if (yi > y) != (yj > y) and x < (xj - xi) * (y - yi) / (yj - yi) + xi: inside = not inside
        j = i
    return inside

def carve(ob, mats, poly, label=''):
    """cut the faces of the given materials along the polygon edges, then delete the faces inside it"""
    midx = {i for i, s in enumerate(ob.material_slots) if s.material and s.material.name in mats}
    if not midx: return 0
    lp = [to_local(ob, p) for p in poly]; s = ob.scale.x; eps = 0.05 / s
    bm = bmesh.new(); bm.from_mesh(ob.data)
    n = len(lp)
    px = [p[0] for p in lp]; py = [p[1] for p in lp]
    bx0, bx1, by0, by1 = min(px) - eps, max(px) + eps, min(py) - eps, max(py) + eps
    pool = set()
    for f in bm.faces:
        if f.material_index not in midx: continue
        xs = [v.co.x for v in f.verts]; ys = [v.co.y for v in f.verts]
        if max(xs) < bx0 or min(xs) > bx1 or max(ys) < by0 or min(ys) > by1: continue
        pool.add(f)
    for i in range(n):
        a = lp[i]; b = lp[(i + 1) % n]
        ex0, ex1 = min(a[0], b[0]) - eps, max(a[0], b[0]) + eps; ey0, ey1 = min(a[1], b[1]) - eps, max(a[1], b[1]) + eps
        faces = []
        for f in pool:
            if not f.is_valid: continue
            xs = [v.co.x for v in f.verts]; ys = [v.co.y for v in f.verts]
            if max(xs) < ex0 or min(xs) > ex1 or max(ys) < ey0 or min(ys) > ey1: continue
            faces.append(f)
        if not faces: continue
        verts = {v for f in faces for v in f.verts}; edges = {e for f in faces for e in f.edges}
        dx, dy = b[0] - a[0], b[1] - a[1]; L = math.hypot(dx, dy)
        if L < 1e-9: continue
        r = bmesh.ops.bisect_plane(bm, geom=list(verts) + list(edges) + faces, dist=1e-6,
                                   plane_co=Vector((a[0], a[1], 0)), plane_no=Vector((-dy / L, dx / L, 0)))
        pool = {f for f in pool if f.is_valid} | {x for x in r['geom'] if isinstance(x, bmesh.types.BMFace) and x.is_valid}
    doomed = []
    for f in pool:
        if not f.is_valid or f.material_index not in midx: continue
        c = f.calc_center_median()
        if point_in_poly(c.x, c.y, lp): doomed.append(f)
    k = len(doomed)
    if doomed: bmesh.ops.delete(bm, geom=doomed, context='FACES')
    bm.to_mesh(ob.data); bm.free(); ob.data.update()
    print('[edit] %s: carved %d faces (%s) from %s' % (label, k, ','.join(sorted(mats)), ob.name))
    return k

def bbox_intersects_poly(bb, poly):
    x0, x1, y0, y1 = bb[:4]
    if any(point_in_poly(x, y, poly) for x, y in ((x0, y0), (x1, y0), (x1, y1), (x0, y1), ((x0 + x1) / 2, (y0 + y1) / 2))): return True
    return any(x0 <= px <= x1 and y0 <= py <= y1 for px, py in poly)

def bbox_in(bb, x0, x1, y0, y1):
    return x0 <= bb[0] and bb[1] <= x1 and y0 <= bb[2] and bb[3] <= y1

def centre_in(bb, x0, x1, y0, y1):
    cx = (bb[0] + bb[1]) / 2; cy = (bb[2] + bb[3]) / 2
    return x0 <= cx <= x1 and y0 <= cy <= y1

OBJ = {o.name: o for o in bpy.data.objects if o.type == 'MESH'}

def delete_trees(trunks, radius=3.3, label='trees'):
    """delete whole street trees: trunk (bark, from the ground) and every leaf/bark part above it"""
    for ob in OBJ.values():
        mats_here = {s.material.name for s in ob.material_slots if s.material}
        if not mats_here & {'SD_bark', 'SD_leaf', 'SD_leafDark', 'SD_leafLight'}: continue
        def pred(mats, bb, trunks=trunks):
            if not mats & {'SD_bark', 'SD_leaf', 'SD_leafDark', 'SD_leafLight'}: return False
            cx = (bb[0] + bb[1]) / 2; cy = (bb[2] + bb[3]) / 2
            for tx, ty in trunks:
                if math.hypot(cx - tx, cy - ty) < radius and (bb[5] > 4.0 or (bb[4] < 0.5 and math.hypot(cx - tx, cy - ty) < 0.6)): return True
            return False
        delete_components(ob, pred, label)

# ----------------------------------------------------------------------------------------------------------
# stage: lamps (image 2 + image 3)
# ----------------------------------------------------------------------------------------------------------
def stage_lamps():
    # ornate 4-globe lamp standing in the market's south gate, and the one on the roundabout's east mouth
    for (x0, x1, y0, y1) in ((7.2, 8.8, -9.6, -7.9), (22.4, 24.2, -24.9, -23.1)):
        delete_components(OBJ['BT_Arcades'], lambda m, bb: 'SD_archDark' in m and bb[5] < 5.2 and bbox_in(bb, x0, x1, y0, y1), 'lamp post')
        delete_components(OBJ['BT_SideGates'], lambda m, bb: 'SD_white' in m and 4.0 < bb[4] and bb[5] < 5.7 and bbox_in(bb, x0, x1, y0, y1), 'lamp globes')
        delete_components(OBJ['BT_Arcades'], lambda m, bb: 'SD_paving' in m and bbox_in(bb, x0 - 0.3, x1 + 0.3, y0 - 0.3, y1 + 0.3) and bb[1] - bb[0] < 2.5, 'lamp plinth')

# ----------------------------------------------------------------------------------------------------------
# stage: zebra crossings at the roundabout (image 2)
# ----------------------------------------------------------------------------------------------------------
def stripe_like(bb):
    w = bb[1] - bb[0]; h = bb[3] - bb[2]
    return max(w, h) > 1.8 and bb[5] > 0.0855

def stage_zebra():
    marks = OBJ['IX_RoadMarks_v4']
    # doubled / crossing stripes painted on the ring road itself, at the west and east mouths
    delete_components(marks, lambda m, bb: stripe_like(bb) and centre_in(bb, -3.6, 0.3, -28.6, -19.4), 'west mouth stripes')
    delete_components(marks, lambda m, bb: stripe_like(bb) and centre_in(bb, 13.4, 20.6, -33.2, -16.4), 'east mouth stripes')
    # centre dashes of Le Lai under the new crossing
    delete_components(marks, lambda m, bb: not stripe_like(bb) and centre_in(bb, -10.2, -4.8, -26.5, -21.5), 'dashes under zebra')
    # one clean crossing on the Le Lai arm, just outside the ring (asphalt y -27.75..-20.25, centre y -24)
    g = Geo(5)
    for i in range(8):
        o = (i - 3.5) * 0.85
        g.box('roadLine', -8.6, -24 + o - 0.22, 0.06, -6.4, -24 + o + 0.22, 0.09, ch=0)
    add_obj(g, 'IX_RoadMarks_LeLaiZebra')

# ----------------------------------------------------------------------------------------------------------
# stage: Ton Duc Thang, the waterfront road joining Ham Nghi to Dong Khoi (image 1)
# ----------------------------------------------------------------------------------------------------------
T_H = (62.0, -50.0); D_H = (0.894427, -0.447214)          # Ham Nghi tip and heading (slope -1/2)
A_H, A_D = 3.75, 3.5                                        # asphalt half widths of Ham Nghi / Dong Khoi
CURB, WALK = 0.25, 2.4
E_PT = (83.54, -52.0); PHI1 = math.radians(93.28)          # join onto Dong Khoi's axis, heading north
PHI0 = math.atan2(D_H[1], D_H[0])
R_ARC = 5.237
S_PT = (E_PT[0] - R_ARC * (math.sin(PHI1) - math.sin(PHI0)), E_PT[1] - R_ARC * (math.cos(PHI0) - math.cos(PHI1)))
C_ARC = (S_PT[0] - R_ARC * math.sin(PHI0), S_PT[1] + R_ARC * math.cos(PHI0))

def road_samples():
    """(x, y, tx, ty, half width) along the new centreline"""
    out = []
    L = math.hypot(S_PT[0] - T_H[0], S_PT[1] - T_H[1]); n = max(2, int(L / 2.0))
    for i in range(n + 1):
        u = i / n
        out.append((T_H[0] + D_H[0] * L * u, T_H[1] + D_H[1] * L * u, D_H[0], D_H[1], A_H + (A_D - A_H) * u))
    m = max(6, int(abs(PHI1 - PHI0) / math.radians(8)))
    for i in range(1, m + 1):
        ph = PHI0 + (PHI1 - PHI0) * i / m
        out.append((C_ARC[0] + R_ARC * math.sin(ph), C_ARC[1] - R_ARC * math.cos(ph), math.cos(ph), math.sin(ph), A_D))
    return out

def offsets(samples, dist_fn):
    """left and right offset polylines; dist_fn(half_width) -> offset distance; mitred at the joints"""
    L = []; Rr = []
    n = len(samples)
    for i, (x, y, tx, ty, a) in enumerate(samples):
        if 0 < i < n - 1:
            px, py = samples[i - 1][2], samples[i - 1][3]; qx, qy = samples[i + 1][2], samples[i + 1][3]
            # tangent at a joint: mean of the two segment directions
            ax, ay = samples[i][0] - samples[i - 1][0], samples[i][1] - samples[i - 1][1]
            bx, by = samples[i + 1][0] - samples[i][0], samples[i + 1][1] - samples[i][1]
            la = math.hypot(ax, ay) or 1; lb = math.hypot(bx, by) or 1
            ax, ay, bx, by = ax / la, ay / la, bx / lb, by / lb
            mx, my = ax + bx, ay + by; lm = math.hypot(mx, my) or 1; tx, ty = mx / lm, my / lm
            cosh = max(0.5, tx * ax + ty * ay); k = 1 / cosh
        else:
            k = 1.0
        nx, ny = -ty, tx; d = dist_fn(a) * k
        dl = d
        if math.hypot(x - C_ARC[0], y - C_ARC[1]) < R_ARC + 0.05:          # on the arc: the inside cannot pass the centre
            dl = min(d, R_ARC - 0.03)
        L.append((x + nx * dl, y + ny * dl)); Rr.append((x - nx * d, y - ny * d))
    return L, Rr

def strip(g, mat, A, B, z, flip=False):
    """quads between two polylines at height z, facing up"""
    for i in range(len(A) - 1):
        a0, a1, b1, b0 = A[i], A[i + 1], B[i + 1], B[i]
        V = [(a0[0], a0[1], z), (b0[0], b0[1], z), (b1[0], b1[1], z), (a1[0], a1[1], z)]
        if flip: V.reverse()
        g.raw(mat, V, [(0, 1, 2, 3)])

def wall(g, mat, A, z0, z1):
    for i in range(len(A) - 1):
        a, b = A[i], A[i + 1]
        g.raw(mat, [(a[0], a[1], z0), (b[0], b[1], z0), (b[0], b[1], z1), (a[0], a[1], z1)], [(0, 1, 2, 3)])

def outline_poly(samples, extra):
    Lf, Rf = offsets(samples, lambda a: a + extra)
    return Lf + Rf[::-1]

def stage_road():
    S = road_samples()
    asph = outline_poly(S, 0.0); curb_out = outline_poly(S, CURB); walk_out = outline_poly(S, CURB + WALK)
    # a little further back past both tips so the old curb caps and the dash at the Dong Khoi cap go too
    back = [(T_H[0] - D_H[0] * 0.3, T_H[1] - D_H[1] * 0.3)]
    ycap = lambda x: E_PT[1] + (x - E_PT[0]) * math.tan(PHI1 - PI / 2)
    stub = [(77.0, ycap(77.0)), (90.6, ycap(90.6)), (90.6, -63.8), (77.0, -63.8)]    # old Dong Khoi dead end
    # objects in the way: planters and lamps whose base touches the new road or the old stub, with their tops
    LEAFY = {'SD_bark', 'SD_leaf', 'SD_leafDark', 'SD_leafLight'}
    bases = []
    for ob in OBJ.values():
        if {s_.material.name for s_ in ob.material_slots if s_.material} <= GROUND: continue
        for mats, bb, faces in components(ob):
            if (mats & GROUND) or (mats & LEAFY): continue
            if bb[4] < 0.6 and bb[5] < 9 and (bbox_intersects_poly(bb, curb_out) or bbox_intersects_poly(bb, stub)):
                bases.append(((bb[0] + bb[1]) / 2, (bb[2] + bb[3]) / 2))
    print('[edit] road obstacle bases', [(round(x, 1), round(y, 1)) for x, y in bases])
    def near_base(bb):
        cx = (bb[0] + bb[1]) / 2; cy = (bb[2] + bb[3]) / 2
        return any(math.hypot(cx - bx, cy - by) < 1.6 for bx, by in bases)
    for ob in OBJ.values():
        if {s_.material.name for s_ in ob.material_slots if s_.material} <= GROUND: continue
        delete_components(ob, lambda m, bb: not (m & GROUND) and bb[5] < 9 and not ((m & LEAFY) and bb[5] > 4) and near_base(bb), 'road obstacles')
    trunks = []
    for mats, bb, faces in components(OBJ['LF_Roundabout']):
        if 'SD_bark' in mats and bb[4] < 0.5 and bbox_intersects_poly(bb, curb_out): trunks.append(((bb[0] + bb[1]) / 2, (bb[2] + bb[3]) / 2))
    if trunks: delete_trees(trunks, label='road trees')
    # ground: sidewalks and the old stub, then the old caps / Dong Khoi stub in the road mesh, then its dashes
    carve(OBJ['BT_Arcades'], {'SD_paving'}, walk_out, 'sidewalk outline')
    carve(OBJ['BT_Arcades'], {'SD_paving'}, stub, 'old stub sidewalk')
    carve(OBJ['IX_Roads_v4'], {'SD_asphalt', 'SD_curb'}, outline_poly(S, CURB + 0.02), 'road caps')
    carve(OBJ['IX_Roads_v4'], {'SD_asphalt', 'SD_curb'}, stub, 'old stub road')
    # Ham Nghi's curb cap (just behind the tip): a small box around the tip across the full curb width
    nx, ny = -D_H[1], D_H[0]; w = A_H + CURB + 0.02
    cap = [(T_H[0] - D_H[0] * 0.35 + nx * w, T_H[1] - D_H[1] * 0.35 + ny * w), (T_H[0] + D_H[0] * 0.3 + nx * w, T_H[1] + D_H[1] * 0.3 + ny * w),
           (T_H[0] + D_H[0] * 0.3 - nx * w, T_H[1] + D_H[1] * 0.3 - ny * w), (T_H[0] - D_H[0] * 0.35 - nx * w, T_H[1] - D_H[1] * 0.35 - ny * w)]
    carve(OBJ['IX_Roads_v4'], {'SD_curb', 'SD_asphalt'}, cap, 'Ham Nghi cap')
    delete_components(OBJ['IX_RoadMarks_v4'], lambda m, bb: bbox_intersects_poly(bb, stub) or bbox_intersects_poly(bb, curb_out), 'old dashes')
    # new geometry
    g = Geo(9)
    La, Ra = offsets(S, lambda a: a); Lc, Rc = offsets(S, lambda a: a + CURB); Lw, Rw = offsets(S, lambda a: a + CURB + WALK)
    # extend the curb strips 0.3 m back over Ham Nghi's removed cap
    def ext(P, k=0.35):
        return [(P[0][0] - D_H[0] * k, P[0][1] - D_H[1] * k)] + P
    strip(g, 'asphalt', ext(La, 0.35), ext(Ra, 0.35), 0.07)
    wall(g, 'asphalt', ext(La, 0.35), -0.15, 0.07); wall(g, 'asphalt', ext(Ra, 0.35), -0.15, 0.07)
    for side in ((Lc, La, 1), (Ra, Rc, 1)):
        o, i_, _ = side
        strip(g, 'curb', ext(o), ext(i_), 0.24)
        wall(g, 'curb', ext(i_), 0.07, 0.24); wall(g, 'curb', ext(o), -0.15, 0.24)
    strip(g, 'paving', Lw, Lc, 0.18); strip(g, 'paving', Rc, Rw, 0.18)
    n_straight = max(2, int(math.hypot(S_PT[0] - T_H[0], S_PT[1] - T_H[1]) / 2.0)) + 1
    wall(g, 'paving', Lw[:n_straight], -0.15, 0.18); wall(g, 'paving', Rw, -0.15, 0.18)
    # the sidewalk band's own cap walls (hidden under the old sidewalks, harmless)
    road = add_obj(g, 'IX_Roads_TonDucThang')
    # centre dashes: 1.6 m every 3.6 m, continuing Ham Nghi's rhythm (its last dash is 3.3 m before the tip)
    gm = Geo(10)
    acc = [0.0]
    for i in range(1, len(S)): acc.append(acc[-1] + math.hypot(S[i][0] - S[i - 1][0], S[i][1] - S[i - 1][1]))
    total = acc[-1]
    def at(s):
        for i in range(1, len(S)):
            if s <= acc[i]:
                u = (s - acc[i - 1]) / (acc[i] - acc[i - 1]); p, q = S[i - 1], S[i]
                return (p[0] + (q[0] - p[0]) * u, p[1] + (q[1] - p[1]) * u, math.atan2(q[1] - p[1], q[0] - p[0]))
        return (S[-1][0], S[-1][1], math.atan2(S[-1][3], S[-1][2]))
    s = 0.3
    while s + 0.8 < total - 1.4:
        x, y, yaw = at(s + 0.8)
        gm.box('roadLine', -0.8, -0.08, 0.06, 0.8, 0.08, 0.085, ch=0, M=T(x, y, 0) @ Rm(0, 0, yaw))
        s += 3.6
    add_obj(gm, 'IX_RoadMarks_TonDucThang')

# ----------------------------------------------------------------------------------------------------------
# stage: Bui Vien, the bar street on Le Lai (image 4)
# ----------------------------------------------------------------------------------------------------------
Y_N, Y_S = -16.3, -31.6            # house front lines: north row faces south, south row faces north
BARS = [('BIA SÀI GÒN', 'signRed', 'signGold'), ('KARAOKE', 'clockBlue', 'white'), ('BUFFALO BAR', 'signGold', 'signRed'),
        ('GO 2 BAR', 'lantern', 'white'), ('LẨU DÊ 404', 'wallTeal', 'signGold'), ('BÁNH MÌ', 'signGold', 'signRed'),
        ('HOSTEL', 'clothBlue', 'white'), ('SPA MASSAGE', 'flowerPink', 'white'), ('TATTOO', 'archDark', 'signGold'),
        ('CƠM TẤM', 'clothYellow', 'signRed'), ('THE VIEW', 'turq', 'white'), ('CÀ PHÊ SỮA ĐÁ', 'wood', 'signGold'),
        ('BIA HƠI', 'clothBlue', 'clothYellow'), ('PUB 86', 'signRed', 'white'), ('PHỞ 24H', 'poGreen', 'white'),
        ('SKY BAR', 'clockBlue', 'signGold'), ('CRAB HOUSE', 'plasticRed', 'white'), ('BÚN BÒ', 'poYellow', 'signRed')]
PAINTS = ['wallTeal', 'wallPink', 'wallBeige', 'wallOchre', 'wallGrey', 'cream', 'creamLight', 'poYellow', 'turq', 'opWhite', 'brick']

def bar_front(g, w, GF, rng, wall, depth=1.4):
    hw = w / 2
    g.box(wall, -hw, 0, 0, -hw + 0.3, depth, GF, ch=0); g.box(wall, hw - 0.3, 0, 0, hw, depth, GF, ch=0)
    g.box(wall, -hw, 0, GF - 0.5, hw, depth, GF, ch=0)
    g.box('shopDark', -hw + 0.3, depth - 0.1, 0, hw - 0.3, depth, GF - 0.5, ch=0)
    g.box('concrete', -hw, -0.1, 0, hw, depth, 0.12, ch=0)
    g.box('wood', -hw + 0.45, depth - 0.95, 0.12, hw - 0.45, depth - 0.35, 1.05, ch=0)                 # bar counter
    g.box('wood', -hw + 0.45, depth - 0.24, 1.75, hw - 0.45, depth - 0.1, 1.82, ch=0)                  # bottle shelf
    x = -hw + 0.6
    while x < hw - 0.6:
        g.cyl(rng.choice(['glassTeal', 'lantern', 'signGold', 'white', 'clothBlue']), 0.05, 0.3, 6, T(x, depth - 0.17, 1.82))
        x += 0.22
    g.box(rng.choice(['clockBlue', 'flowerPink', 'turq', 'signRed']), -0.6, depth - 0.12, 2.2, 0.6, depth - 0.09, 2.9, ch=0)   # neon panel
    for k in range(int((w - 1.6) / 0.8)):
        stool(g, T(-hw + 0.9 + k * 0.8, depth - 1.35, 0.12), rng.choice(['plasticRed', 'plasticBlue']))

def balcony(g, w, z0, rng, rail='metal'):
    hw = w / 2
    g.box('creamLight', -hw + 0.12, -0.95, z0, hw - 0.12, 0.0, z0 + 0.16, ch=0)
    for x in [(-hw + 0.25) + k * 0.2 for k in range(int((w - 0.5) / 0.2) + 1)]:
        g.rod(rail, (x, -0.9, z0 + 0.16), (x, -0.9, z0 + 1.05), 0.018, 4)
    g.box(rail, -hw + 0.12, -0.95, z0 + 1.02, hw - 0.12, -0.85, z0 + 1.1, ch=0)
    g.box('wood', -0.85, -0.08, z0 + 0.16, 0.85, 0.0, z0 + 2.3, ch=0)
    for k in range(2):
        if rng.random() < 0.7: g.ico(rng.choice(['leaf', 'flowerPink', 'leafLight']), 0.3, T(-hw + 0.55 + k * (w - 1.1), -0.6, z0 + 0.55), sub=1, jit=0.12)
    if rng.random() < 0.5:
        g.rod('metal', (-hw + 0.3, -0.55, z0 + 2.35), (hw - 0.3, -0.55, z0 + 2.35), 0.012, 4)
        for k in range(rng.randint(2, 4)):
            x = -hw + 0.4 + k * 0.5
            g.box(rng.choice(['clothBlue', 'white', 'clothRed', 'clothYellow']), x, -0.58, z0 + 1.55, x + 0.4, -0.54, z0 + 2.33, ch=0)

def windows(g, w, z0, rng, shut='wallTeal'):
    for cx in (-w / 4, w / 4):
        g.box('creamLight', cx - 0.55, -0.12, z0 + 0.7, cx + 0.55, 0.0, z0 + 2.2, ch=0)
        g.box('glassTeal', cx - 0.42, -0.14, z0 + 0.85, cx + 0.42, -0.1, z0 + 2.05, ch=0)
        g.box(shut, cx - 0.57 - 0.4, -0.08, z0 + 0.85, cx - 0.57, 0.0, z0 + 2.05, ch=0)
        g.box(shut, cx + 0.57, -0.08, z0 + 0.85, cx + 0.57 + 0.4, 0.0, z0 + 2.05, ch=0)

def vtext(g, body, size, mat, M, max_h=None):
    g.text('\n'.join(body), size, mat, M, extrude=0.02, max_w=0.42)

def house_bar(g, w, D, rng, floors, paint, bar, vsign=True, roof='flat'):
    name, c0, c1 = bar
    hw = w / 2; GF = 3.6; F = 3.0; Ht = GF + F * floors
    g.box(paint, -hw, 1.4, 0, hw, D, GF, ch=0)
    bar_front(g, w, GF, rng, paint)
    g.box(paint, -hw, 0, GF, hw, D, Ht, ch=0)
    for i in range(floors):
        z0 = GF + i * F
        g.box('creamLight', -hw - 0.05, -0.18, z0, hw + 0.05, 0.0, z0 + 0.2, ch=0)
        if (i + (1 if rng.random() < 0.3 else 0)) % 2 == 0: balcony(g, w, z0, rng)
        else: windows(g, w, z0, rng, shut=rng.choice(['wallTeal', 'wood', 'clothBlue', 'signRed']))
    # ground-floor sign band with a row of bulbs under it
    g.box(c0, -hw + 0.25, -0.34, GF - 0.78, hw - 0.25, -0.02, GF - 0.04, ch=0)
    g.text(name, 0.34, c1, T(0, -0.36, GF - 0.41) @ FACE_S, extrude=0.02, max_w=w - 0.9)
    x = -hw + 0.4
    while x < hw - 0.3:
        g.ico('lampGlow', 0.06, T(x, -0.4, GF - 0.86), sub=0); x += 0.45
    # canopy: striped awning or a flat tin canopy with a lit fascia
    if rng.random() < 0.5:
        a, b = rng.choice([('white', 'signRed'), ('white', 'clothBlue'), ('clothYellow', 'signRed'), ('white', 'poGreen')])
        awning_striped(g, -hw + 0.2, hw - 0.2, GF - 0.95, 1.3, 0.5, a, b)
    else:
        g.box('metal', -hw + 0.1, -1.45, GF - 1.02, hw - 0.1, 0.0, GF - 0.94, ch=0)
        g.box(c0, -hw + 0.1, -1.5, GF - 1.3, hw - 0.1, -1.4, GF - 0.9, ch=0)
        for x_ in (-hw + 0.25, hw - 0.25): g.rod('metal', (x_, -1.35, GF - 1.0), (x_, -0.05, GF - 0.4), 0.025, 5)
        x = -hw + 0.35
        while x < hw - 0.25:
            g.ico('lampGlow', 0.06, T(x, -1.46, GF - 1.36), sub=0); x += 0.5
    if rng.random() < 0.45:
        for x_ in (-hw + 0.6, hw - 0.6): lantern(g, T(x_, -0.9, GF - 1.0), 0.75)
    # vertical neon sign hanging off one corner of the upper floors
    if vsign:
        sx = hw - 0.55 if rng.random() < 0.5 else -hw + 0.55
        ztop = min(Ht - 0.2, GF + 3.3); zbot = max(GF + 0.4, ztop - 0.36 * len(name.split()[0]) - 0.5)
        g.box(c0, sx - 0.28, -0.62, zbot, sx + 0.28, -0.08, ztop, ch=0)
        g.box('metal', sx - 0.02, -0.08, zbot + 0.3, sx + 0.02, 0.0, ztop - 0.3, ch=0)
        vtext(g, name.split()[0], 0.3, c1, T(sx, -0.64, (zbot + ztop) / 2) @ FACE_S)
    # AC units and pipes
    for k in range(rng.randint(1, 3)):
        z = GF + 0.4 + rng.uniform(0, F * floors - 1.5); x_ = rng.choice([-hw + 0.5, hw - 0.5])
        g.box('acWhite', x_ - 0.35, -0.55, z, x_ + 0.35, -0.05, z + 0.55, ch=0)
        g.cyl('metal', 0.2, 0.04, 12, T(x_, -0.55, z + 0.28) @ AX_NY)
    g.rod('pipeYellow', (hw - 0.12, -0.1, 0), (hw - 0.12, -0.1, Ht + 0.2), 0.05, 6)
    # roof
    if roof == 'tile':
        g.box(paint, -hw, 0, Ht, hw, D, Ht + 0.3, ch=0)
        tile_slope(g, 'roofRed', -hw - 0.4, Ht + 0.2, 0.05, Ht + 1.8, D / 2, D + 0.8)
        tile_slope(g, 'roofRed', hw + 0.4, Ht + 0.2, -0.05, Ht + 1.8, D / 2, D + 0.8)
        g.extrude(paint, [(-hw, Ht + 0.3), (hw, Ht + 0.3), (0, Ht + 1.7)], 0.0, 0.25)
        g.box('roofRedDark', -0.18, -0.4, Ht + 1.75, 0.18, D + 0.4, Ht + 1.98, ch=0)
        return Ht + 1.9
    g.box(paint, -hw, -0.05, Ht, hw, 0.3, Ht + 0.7, ch=0)
    g.box('creamLight', -hw - 0.05, -0.15, Ht + 0.7, hw + 0.05, 0.4, Ht + 0.85, ch=0)
    g.box('concrete', -hw, 0.3, Ht, hw, D, Ht + 0.06, ch=0)
    for x_ in (-hw + 0.2, hw - 0.2):
        for y_ in (0.5, D - 0.4): g.rod('metal', (x_, y_, Ht), (x_, y_, Ht + 1.1), 0.03, 5)
    g.rod('metal', (-hw + 0.2, 0.5, Ht + 1.1), (hw - 0.2, 0.5, Ht + 1.1), 0.02, 4)
    g.cyl('white', 0.45, 1.1, 14, T(rng.uniform(-hw + 0.8, hw - 0.8), D * 0.7, Ht + 0.06))
    if rng.random() < 0.5:   # rooftop frame with a billboard
        g.box(c1 if c1 != 'white' else 'white', -hw + 0.4, D - 0.35, Ht + 1.2, hw - 0.4, D - 0.25, Ht + 2.6, ch=0)
        g.text(name.split()[0], 0.5, c0, T(0, D - 0.37, Ht + 1.9) @ FACE_S, extrude=0.02, max_w=w - 1.2)
        for x_ in (-hw + 0.5, hw - 0.5): g.rod('metal', (x_, D - 0.3, Ht), (x_, D - 0.3, Ht + 1.25), 0.03, 5)
    return Ht + 0.85

def string_lights(g, p0, p1, sag, bulbs=True, lanterns=False, flags=False, rng=None):
    a = Vector(p0); b = Vector(p1); n = 18
    pts = [a.lerp(b, t / n) - Vector((0, 0, sag * 4 * (t / n) * (1 - t / n))) for t in range(n + 1)]
    for i in range(n): g.rod('ink', pts[i], pts[i + 1], 0.02, 4)
    for i in range(1, n):
        p = pts[i]
        if bulbs and i % 2 == 0: g.ico('lampGlow', 0.09, T(p.x, p.y, p.z - 0.12), sub=0)
        if lanterns and i % 3 == 0: lantern(g, T(p.x, p.y, p.z), 0.55)
        if flags:
            c = rng.choice(['clothRed', 'clothYellow', 'clothBlue', 'white', 'lantern'])
            d = (pts[i + 1] - pts[i - 1]).normalized() * 0.18
            g.raw(c, [(p.x - d.x, p.y - d.y, p.z), (p.x + d.x, p.y + d.y, p.z), (p.x, p.y, p.z - 0.3)], [(0, 1, 2)])

def table_set(g, M, rng):
    g.push(M)
    c = rng.choice(['plasticRed', 'plasticBlue', 'white'])
    g.cyl(c, 0.3, 0.05, 8, T(0, 0, 0.45)); g.cyl(c, 0.04, 0.45, 6)
    for k in range(rng.randint(2, 4)):
        g.cyl(rng.choice(['signGold', 'glassTeal', 'white']), 0.03, 0.2, 6, T(rng.uniform(-0.18, 0.18), rng.uniform(-0.18, 0.18), 0.5))
    for k in range(rng.randint(2, 4)):
        a = k * PI / 2 + rng.uniform(-0.3, 0.3)
        stool(g, T(0.62 * math.cos(a), 0.62 * math.sin(a), 0) @ Rm(0, 0, a), rng.choice(['plasticRed', 'plasticBlue']))
    g.pop()

def cooler(g, M, rng):
    g.push(M)
    c = rng.choice(['plasticBlue', 'plasticRed', 'white'])
    g.box(c, -0.35, -0.22, 0, 0.35, 0.22, 0.42, ch=0.02); g.box('white', -0.36, -0.23, 0.42, 0.36, 0.23, 0.5, ch=0.02)
    g.pop()

def crates(g, M, rng):
    g.push(M)
    for k in range(rng.randint(2, 4)):
        g.box(rng.choice(['clothYellow', 'signRed', 'plasticBlue']), -0.25, -0.18, k * 0.3, 0.25, 0.18, k * 0.3 + 0.28, ch=0.02)
    g.pop()

def aboard(g, M, rng):
    g.push(M)
    for s in (-1, 1):
        g.bc('white', s * 0.1, 0, 0.45, 0.04, 0.5, 0.85, ry=s * 0.2, ch=0)
        g.bc(rng.choice(['signRed', 'clockBlue', 'poGreen']), s * 0.13, 0, 0.6, 0.02, 0.4, 0.3, ry=s * 0.2, ch=0)
    g.pop()

def stage_buivien():
    rng = random.Random(1975)
    # south-side park trees along the street make room for the second row of houses
    trunks = []
    for mats, bb, faces in components(OBJ['LF_Roundabout']):
        if 'SD_bark' in mats and bb[4] < 0.5 and centre_in(bb, -58, -9.5, -36.5, -28.3): trunks.append(((bb[0] + bb[1]) / 2, (bb[2] + bb[3]) / 2))
    print('[edit] south trunks', [(round(x, 1), round(y, 1)) for x, y in trunks])
    delete_trees(trunks, label='Le Lai south trees')
    # the two rows
    gN = Geo(31); gS = Geo(32); fronts = {'N': [], 'S': []}
    bars = list(BARS); rng.shuffle(bars)
    WALK_N = [(-16, -65), (-14, -64), (-12, -59), (-10, -54), (-9, -52), (-8, -51), (-7, -51), (-6, -50), (-4, -49), (-2, -48), (0, -45)]
    def depth_n(x1):
        # deepest back line whose east walkway edge still clears x1
        best = 0
        for y, xe in WALK_N:
            if xe - 0.4 >= x1: best = y
        return max(0.0, min(9.0, best - Y_N))
    def row(g, x0, x1, yf, yaw, side):
        x = x0; k = 0
        while x1 - x > 3.4:
            w = min(rng.uniform(4.0, 5.4), x1 - x)
            if x1 - x - w < 3.4: w = x1 - x
            cx = x + w / 2
            D = depth_n(x + w) if side == 'N' else (9.0 if x >= -25.5 else 4.6)
            if D < 4.0:
                x += w; continue
            floors = rng.choice([2, 3, 3, 4]); paint = rng.choice(PAINTS); bar = bars[(len(fronts['N']) + len(fronts['S'])) % len(bars)]
            jut = rng.uniform(-0.25, 0.25)
            M = T(cx, yf + (jut if side == 'N' else -jut), 0.18) @ Rm(0, 0, yaw)
            g.push(M)
            g.box('pavingDark', -w / 2 - 0.02, -1.3, -0.17, w / 2 + 0.02, D, 0.01, ch=0)
            top = house_bar(g, w, D, rng, floors, paint, bar, vsign=rng.random() < 0.75, roof=rng.choice(['flat', 'flat', 'tile']))
            g.pop()
            fronts[side].append((cx, w, top)); x += w; k += 1
    row(gN, -60.0, -43.0, Y_N, 0.0, 'N')
    row(gN, -19.0, -10.0, Y_N, 0.0, 'N')
    row(gS, -58.5, -10.0, Y_S, PI, 'S')
    add_obj(gN, 'SH_BuiVien_N', wobble=0.07); add_obj(gS, 'SH_BuiVien_S', wobble=0.07)
    # street dressing: string lights, lanterns and bunting across the street, tables, coolers, scooters, menu boards
    gl = Geo(33); gf = Geo(34)
    for i, x in enumerate(range(-57, -10, 6)):
        kind = i % 3
        string_lights(gl, (x, Y_N - 0.3, 6.3), (x + 2.5, Y_S + 0.3, 6.1), 0.9, bulbs=kind != 1, lanterns=kind == 1, flags=kind == 2, rng=rng)
    for x in range(-56, -12, 11):
        string_lights(gl, (x, Y_N - 0.3, 7.4), (x - 3, Y_S + 0.3, 7.2), 0.7, bulbs=True)
    # a gantry sign at the roundabout end
    for y in (-18.9, -29.1):
        gf.cyl('signRed', 0.14, 6.6, 10, T(-10.5, y, 0.18)); gf.cyl('signGold', 0.2, 0.3, 10, T(-10.5, y, 0.18))
    gf.box('signRed', -10.75, -29.1, 6.5, -10.25, -18.9, 7.5, ch=0.02)
    gf.box('signGold', -10.78, -27.5, 6.6, -10.22, -20.5, 7.4, ch=0)
    for s, yaw in ((1, PI / 2), (-1, -PI / 2)):
        gf.text('PHỐ ĐI BỘ BÙI VIỆN', 0.5, 'signRed', T(-10.5 + s * 0.3, -24.0, 7.0) @ Rm(0, 0, yaw) @ FACE_S, extrude=0.02, max_w=6.6)
    for k in range(17): gf.ico('lampGlow', 0.07, T(-10.5, -28.0 + k * 0.5, 6.42), sub=0)
    # sidewalk life in front of the new houses (north: the old houses already have theirs)
    def life(x0, x1, y_front, y_curb, yaw_curb):
        x = x0 + 1.0
        while x < x1 - 1.0:
            if any(abs(x - lx) < 1.3 for lx in (-40.9, -18.9, -29.2, -19.1)) and abs(y_front + 24) > 5:
                x += 1.0; continue
            r = rng.random()
            if r < 0.3: table_set(gf, T(x, y_front, 0.18) @ Rm(0, 0, rng.uniform(0, PI)), rng); x += 1.9
            elif r < 0.42: cooler(gf, T(x, y_front, 0.18) @ Rm(0, 0, rng.uniform(-0.3, 0.3)), rng); x += 1.0
            elif r < 0.52: crates(gf, T(x, y_front, 0.18), rng); x += 0.9
            elif r < 0.64: aboard(gf, T(x, y_front + (0.3 if y_front < -24 else -0.3), 0.18) @ Rm(0, 0, PI / 2), rng); x += 1.0
            else: x += 1.2
            if rng.random() < 0.3 and x < x1 - 3.5:
                for k in range(rng.randint(2, 4)):
                    scooter(gf, T(x + k * 0.75, y_curb, 0.18) @ Rm(0, 0, yaw_curb + rng.uniform(-0.15, 0.15)), rng, rider=False)
                x += 0.75 * 4
    life(-60, -43, Y_N - 1.4, Y_N - 3.0, PI / 2)
    life(-19, -10, Y_N - 1.4, Y_N - 3.0, PI / 2)
    life(-58.5, -10, Y_S + 1.4, Y_S + 3.0, -PI / 2)
    add_obj(gl, 'SH_BuiVien_Lights', wobble=0.0); add_obj(gf, 'SH_BuiVien_Life', wobble=0.07)

# ----------------------------------------------------------------------------------------------------------
if 'lamps' in STAGES: stage_lamps()
if 'zebra' in STAGES: stage_zebra()
if 'road' in STAGES: stage_road()
if 'buivien' in STAGES: stage_buivien()
for ob in bpy.data.objects:
    if ob.name.startswith('sm_txt'): bpy.data.objects.remove(ob, do_unlink=True)
bpy.ops.wm.save_as_mainfile(filepath=OUT, compress=True)
print('[edit] saved', OUT)
