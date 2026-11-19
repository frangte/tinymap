"""Remove the trees that grow through things, from the Sài Gòn mini map (review 2026-11-18: trees through the stairs
and the tower of Hồ Con Rùa).

The trees are scattered by build_saigon_mini.py (CX_Trees: streets, parks, medians, palms; CX_YardTrees) and the hill's
script (CX_NuiNho_Trees), each object one merged mesh, before the landmarks and the houses round them were modelled. So
some stand where a landmark, a building or the hill road went later. This finds every tree among the loose parts of
those objects (a bark trunk standing up, with the branches and crowns nearest above it) and removes one that:

  - has its trunk through a building, a landmark, a stall, a prop, a boulder or the hill road;
  - has anything over its trunk (it stands inside or under a building, a monument or the hill);
  - stands on a building or in water (a pond, or over the river bed where a bank was rounded off), or floats more
    than 0.5 m over a yard or a street;
  - has a building's wall through the middle of a crown (closer to its centre than 3% of its radius);
  - stands on the island of a roundabout (Khoa 2026-11-19: no trees in the roundabouts; their low garden stays).

A crown that only brushes a balcony stays, and so does a street tree right next to a house; a landmark's own trees
(bark inside an LM_ object) are judged against everything but their landmark. Crowns left floating by a removed tree go
too, and the search runs again until nothing changes (a removed tree can uncover a small one that was taken for its
branch).

    blender models/saigon_mini.blend --background --python scripts/blender/mini/clean_trees.py -- [--dry-run] [--report out.json]

Without --dry-run it saves the file in place. Then `npm run map:export` and copy the GLB (README, "Changing the map").
The first run (2026-11-18) removed 59 of 984 trees and one crown left over.
"""
import json
import math
import re
import sys

import os

import bmesh
import bpy
import numpy as np
from mathutils import Vector
from mathutils.bvhtree import BVHTree

HERE = os.path.dirname(os.path.abspath(__file__))
if HERE not in sys.path:
    sys.path.insert(0, HERE)
import layout as LY  # noqa: E402

# the roundabout islands: (x, y, radius)
ROUNDABOUTS = [(*LY.tb(ra['c']), ra['r']) for ra in LY.L['roundabouts']]

TREE_OBJECTS = ('CX_Trees', 'CX_YardTrees', 'CX_NuiNho_Trees')
LEAF = {'leaf', 'leafDark', 'leafLight'}
FLOWER = {'flowerRed', 'flowerYellow', 'flowerPink'}
WATER = {'water', 'waterDeep', 'foam'}
# what a tree must not grow through or stand on
BUILDINGS = ('SH_np', 'SH_hem', 'SH_cl', 'SH_vt', 'SH_ks', 'SH_tt', 'SH_BuiVien', 'LM_', 'BT_')
SOLID = BUILDINGS + ('CX_Props', 'CX_StreetLife', 'CX_NuiNho_Rocks', 'IX_NuiNho_Road', 'CX_Flowers', 'CX_Lakes')
# what a tree may stand on
GROUND = ('IS_', 'IX_Roads', 'SH_Yards', 'IX_RoadMarks', 'LF_')
TRUNK_MIN_H = 0.9        # a bark part this tall or more, and this narrow or less, is a trunk (m)
TRUNK_MAX_W = 1.3
BRANCH_NEAR = 2.2        # ...unless it starts up inside the height of a taller trunk this close: a branch
CROWN_REACH = 3.5        # crowns and branches belong to the nearest trunk below them this close (m, in plan)
FOOT_CLEAR = 0.4         # the trunk is checked from this far over its foot (a kerb beside the foot is fine)
CROWN_CORE = 0.03        # a wall closer to a crown's centre than this share of its radius cuts through it
FLOAT_MAX = 0.5          # a foot this far over the yard or street under it floats (m)
OVERHEAD = 60.0          # how far up something over a trunk counts (m)


def material_key(m):
    return re.sub(r'^SD_|\.\d+$', '', m.name) if m else ''


def part(name):
    """'object|material' -> (object, material)"""
    o, _, m = (name or '|').partition('|')
    return o, m


def exported_meshes():
    """The meshes export_glb.py writes to the GLB: every mesh in its collections, hidden or not."""
    cols = [bpy.data.collections[c] for c in ('SD_World', 'SD_NoInk') if c in bpy.data.collections]
    objs = [o for c in cols for o in c.all_objects] if cols else list(bpy.context.scene.objects)
    seen = set()
    return [o for o in objs if o.type == 'MESH' and not o.name.startswith(('PREVIEW_', 'NAV_')) and not (o.name in seen or seen.add(o.name))]


def read_map():
    """Every visible mesh in world space: tree parts (loose parts of the tree objects, bark and leaves of the landmarks)
    and a BVH of everything else, with an 'object|material' name per triangle."""
    dg = bpy.context.evaluated_depsgraph_get()
    parts, verts, tris, names = [], [], [], []
    base = 0
    for o in exported_meshes():
        ev = o.evaluated_get(dg)
        me = ev.to_mesh()
        co = np.empty(len(me.vertices) * 3)
        me.vertices.foreach_get('co', co)
        M = np.array(o.matrix_world)
        co = co.reshape(-1, 3) @ M[:3, :3].T + M[:3, 3]
        me.calc_loop_triangles()
        n = len(me.loop_triangles)
        tri = np.empty(n * 3, dtype=np.int64)
        me.loop_triangles.foreach_get('vertices', tri)
        tri = tri.reshape(-1, 3)
        mi = np.empty(n, dtype=np.int64)
        me.loop_triangles.foreach_get('material_index', mi)
        poly = np.empty(n, dtype=np.int64)
        me.loop_triangles.foreach_get('polygon_index', poly)    # a Displace keeps the mesh's own polygons
        ev.to_mesh_clear()
        if n == 0:
            continue
        mats = [material_key(s.material) for s in o.material_slots] or ['']
        tmat = np.array([mats[i] if i < len(mats) else '' for i in mi])
        solid = np.ones(n, bool)
        is_tree_obj = o.name in TREE_OBJECTS
        if is_tree_obj or (o.name.startswith(('LM_', 'BT_')) and (tmat == 'bark').any()):
            roots = loose_parts(co, tri)
            for r in np.unique(roots):
                sel = np.nonzero(roots == r)[0]
                ms = set(tmat[sel])
                if not is_tree_obj and not ms <= LEAF | {'bark'}:
                    continue                       # a landmark's own part that is not a tree
                kind = 'bark' if 'bark' in ms else 'flower' if ms & FLOWER else 'leaf' if ms & LEAF else 'other'
                v = co[np.unique(tri[sel])]
                parts.append({'obj': o.name, 'kind': kind, 'polys': sorted(set(poly[sel].tolist())),
                              'lo': v.min(0), 'hi': v.max(0), 'c': v.mean(0)})
                solid[sel] = False
        verts.append(co)
        tris.append(tri[solid] + base)
        names.extend(f'{o.name}|{m}' for m in tmat[solid])
        base += len(co)
    V = np.vstack(verts)
    T = np.vstack(tris)
    bvh = BVHTree.FromPolygons([Vector(p) for p in V], [tuple(t) for t in T], epsilon=0.0)
    return parts, bvh, names


def loose_parts(co, tri):
    """A part id per triangle: triangles joined by a vertex (welded to the millimetre) are one part."""
    _, weld = np.unique(np.round(co * 1000).astype(np.int64), axis=0, return_inverse=True)
    weld = weld.ravel()
    parent = np.arange(weld.max() + 1)

    def find(a):
        while parent[a] != a:
            parent[a] = parent[parent[a]]
            a = parent[a]
        return a

    for t in tri:
        a = find(weld[t[0]])
        for k in (1, 2):
            b = find(weld[t[k]])
            if b != a:
                parent[b] = a
    return np.array([find(weld[t[0]]) for t in tri])


def axis(p):
    return (p['lo'][0] + p['hi'][0]) / 2, (p['lo'][1] + p['hi'][1]) / 2


def find_trees(parts):
    """Trunks, and every other bark or leaf part given to the nearest trunk below it; parts left over (bushes, a
    landmark's hedges, a crown with no trunk near) are returned apart."""
    cand = [p for p in parts if p['kind'] == 'bark' and p['hi'][2] - p['lo'][2] > TRUNK_MIN_H
            and max(p['hi'][0] - p['lo'][0], p['hi'][1] - p['lo'][1]) < TRUNK_MAX_W]
    cand.sort(key=lambda p: -(p['hi'][2] - p['lo'][2]))
    trunks = []
    for p in cand:
        x, y = axis(p)
        if any(math.hypot(x - axis(t)[0], y - axis(t)[1]) < BRANCH_NEAR and t['lo'][2] + 0.5 < p['lo'][2] < t['hi'][2] + 0.5
               for t in trunks):
            continue
        trunks.append(p)
    trees = [{'trunk': t, 'parts': []} for t in trunks]
    loose = []
    if not trunks:
        return trees, [p for p in parts if p['kind'] in ('bark', 'leaf')]
    xy = np.array([axis(t) for t in trunks])
    lo = np.array([t['lo'][2] for t in trunks])
    for p in parts:
        if p['kind'] not in ('bark', 'leaf') or any(p is t for t in trunks):
            continue
        d = np.where(lo < p['c'][2], np.hypot(xy[:, 0] - p['c'][0], xy[:, 1] - p['c'][1]), 1e9)
        j = int(np.argmin(d))
        (trees[j]['parts'] if d[j] < CROWN_REACH else loose).append(p)
    return trees, loose


def judge(tree, bvh, names):
    """Why the tree has to go (an empty list if it can stay)."""
    t = tree['trunk']
    own = t['obj']
    x, y = axis(t)
    z0, z1 = t['lo'][2], t['hi'][2]
    r = max(t['hi'][0] - t['lo'][0], t['hi'][1] - t['lo'][1]) / 2
    name = lambda i: names[i] if i is not None else ''   # noqa: E731
    mine = lambda n: part(n)[0] == own or part(n)[1] in LEAF | {'bark'}   # noqa: E731
    why = []
    through = set()
    for z in np.arange(z0 + FOOT_CLEAR, z1, 0.25):
        loc, _, i, _ = bvh.find_nearest(Vector((x, y, z)), r * 0.85)
        if i is not None and part(name(i))[0].startswith(SOLID) and not mine(name(i)):
            through.add(name(i))
    if through:
        why.append('trunk through ' + ', '.join(sorted(through)))
    loc, _, i, _ = bvh.ray_cast(Vector((x, y, z0 + 0.6)), Vector((0, 0, 1)), OVERHEAD)
    if loc is not None and not mine(name(i)) and not part(name(i))[0].startswith('CX_StreetLife') \
            and part(name(i))[1] not in ('lantern', 'lampGlow'):
        why.append('under ' + name(i))           # an awning or a wire of a stall over it is fine
    loc, _, i, _ = bvh.ray_cast(Vector((x, y, z0 + 0.35)), Vector((0, 0, -1)), 30)
    if loc is not None:
        under = name(i)
        gap = z0 - loc.z
        if part(under)[1] in WATER or (part(under)[0].startswith('IS_') and loc.z < LY.SEA):
            why.append('stands in water')
        elif not part(under)[0].startswith(GROUND) and part(under)[0] != own:
            why.append('stands on ' + under)
        if gap > FLOAT_MAX and not part(under)[0].startswith('IS_'):
            why.append(f'floats {gap:.2f} m over {under}')
    if own in TREE_OBJECTS:
        for cx, cy, rr in ROUNDABOUTS:
            if math.hypot(x - cx, y - cy) < rr + 0.3:
                why.append('stands in a roundabout')
    for p in tree['parts']:
        if p['kind'] != 'leaf':
            continue
        R = min(p['hi'][0] - p['lo'][0], p['hi'][1] - p['lo'][1]) / 2
        loc, _, i, dist = bvh.find_nearest(Vector(p['c']), R * CROWN_CORE)
        if i is not None and part(name(i))[0].startswith(BUILDINGS) and part(name(i))[0] != own:
            why.append('crown cut by ' + name(i))
            break
    return why


def remove(polys_by_obj):
    for o_name, polys in polys_by_obj.items():
        me = bpy.data.objects[o_name].data
        bm = bmesh.new()
        bm.from_mesh(me)
        bm.faces.ensure_lookup_table()
        bmesh.ops.delete(bm, geom=[bm.faces[i] for i in sorted(polys)], context='FACES')
        bm.to_mesh(me)
        bm.free()
        me.update()


def main():
    argv = sys.argv[sys.argv.index('--') + 1:] if '--' in sys.argv else []
    dry = '--dry-run' in argv
    report_path = argv[argv.index('--report') + 1] if '--report' in argv else None
    removed = []
    loose_before = None
    for rnd in range(4):
        parts, bvh, names = read_map()
        trees, loose = find_trees(parts)
        if loose_before is None:
            loose_before = {(p['obj'], tuple(np.round(p['c'], 2))) for p in loose}
            total = len(trees)
        kill = {}
        for tr in trees:
            why = judge(tr, bvh, names)
            if not why:
                continue
            t = tr['trunk']
            removed.append({'object': t['obj'], 'xy': [round(axis(t)[0], 2), round(axis(t)[1], 2)], 'z': round(float(t['lo'][2]), 2),
                            'height': round(float(t['hi'][2] - t['lo'][2]), 2), 'why': why})
            # the trunk, its branches and crowns in its own object (a landmark's own bushes next to it stay)
            for p in [t] + [p for p in tr['parts'] if p['obj'] == t['obj']]:
                kill.setdefault(p['obj'], set()).update(p['polys'])
        # crowns of the tree objects that lost their trunk with an earlier pass
        for p in loose:
            if p['obj'] in TREE_OBJECTS and (p['obj'], tuple(np.round(p['c'], 2))) not in loose_before:
                removed.append({'object': p['obj'], 'xy': [round(float(p['c'][0]), 2), round(float(p['c'][1]), 2)],
                                'z': round(float(p['c'][2]), 2), 'why': ['crown left without its trunk']})
                kill.setdefault(p['obj'], set()).update(p['polys'])
        print(f'[clean_trees] pass {rnd + 1}: {len(trees)} trees, {sum(1 for r in removed if "height" in r)} to remove so far')
        if not kill or dry:
            break
        remove(kill)
    trees_removed = [r for r in removed if 'height' in r]
    print(f'[clean_trees] {len(trees_removed)} of {total} trees removed'
          f'{" (dry run, nothing changed)" if dry else ""}, {len(removed) - len(trees_removed)} stray crowns')
    for r in removed:
        print('[clean_trees]  ', r['object'], r['xy'], '; '.join(r['why']))
    if report_path:
        with open(report_path, 'w') as f:
            json.dump(removed, f, ensure_ascii=False, indent=1)
    if not dry and removed:
        bpy.ops.wm.save_mainfile(compress=True)
        print('[clean_trees] saved', bpy.data.filepath)


main()
