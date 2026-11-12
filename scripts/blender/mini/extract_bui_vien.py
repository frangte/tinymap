"""Take the Bùi Viện street block out of the v4 map (models/temp.sg.tinymap.blend, the previous map Khoa liked) into a
small source file for Sài Gòn mini: models/bui_vien_v4.blend. Run once (or again if the v4 file changes):

    Blender models/temp.sg.tinymap.blend --background --python scripts/blender/mini/extract_bui_vien.py -- models/bui_vien_v4.blend

What is kept: both rows of shophouses with their bars, signs, awnings, balconies and air-conditioners, the stools and
tables on the pavement, the trees, the string lights and lanterns across the street and the gate (PHỐ ĐI BỘ BÙI VIỆN)
at the east end. The v4 file is a re-imported GLB, so the block is spread over several joined objects: every loose
part whose centre lies in the block's box is taken, whatever object it is in. Left out: the road, pavements and ground
(the v7 walk paves the street), and the people, parked scooters and the van in the street (kit.strip_figures: they
come back as NPCs in the game, by code).

Out: two objects at their v4 coordinates (the street axis is y = -24, the gate at x = -10.8): BV4_Street and
BV4_Lights, with the v4 SD_ materials. scripts/blender/mini/bui_vien.py places them on the v7 map.
"""
import collections
import os
import sys

import bpy

HERE = os.path.dirname(os.path.abspath(__file__))
if HERE not in sys.path:
    sys.path.insert(0, HERE)
import kit  # noqa: E402

REG = (-62.5, -7.5, -43.5, -4.5)               # x0, x1, y0, y1 (v4 Blender XY): both rows, the street and the gate
SKIP = ('NAV_', 'IS_', 'IX_', 'CX_LamSon')     # navigation solids, ground, roads
VAN = (-24.2, -22.6, -19.2, -19.8, 2.2)        # the white van parked in the north lane


def gather(names):
    """one mesh of every loose part (centre in REG) of the given objects, in world coordinates"""
    V, F, FM, mats = [], [], [], []
    for o in [bpy.data.objects[n] for n in names]:
        me, mw = o.data, o.matrix_world
        slot = []
        for m in me.materials:
            if m not in mats:
                mats.append(m)
            slot.append(mats.index(m))
        for fl, c, bb, _ in kit._loose_parts(o):
            if not (REG[0] < c.x < REG[1] and REG[2] < c.y < REG[3]):
                continue
            remap = {}
            for i in fl:
                p = me.polygons[i]
                face = []
                for v in p.vertices:
                    if v not in remap:
                        remap[v] = len(V)
                        V.append(tuple(mw @ me.vertices[v].co))
                    face.append(remap[v])
                F.append(face)
                FM.append(slot[p.material_index] if slot else 0)
    return V, F, FM, mats


def make(name, data):
    V, F, FM, mats = data
    me = bpy.data.meshes.new(name)
    me.from_pydata(V, [], F)
    for m in mats:
        me.materials.append(m)
    me.polygons.foreach_set('material_index', FM)
    me.validate()
    me.update()
    ob = bpy.data.objects.new(name, me)
    bpy.context.scene.collection.objects.link(ob)
    return ob


def main():
    out = sys.argv[sys.argv.index('--') + 1] if '--' in sys.argv else os.path.join(HERE, '..', '..', '..', 'models', 'bui_vien_v4.blend')
    meshes = [o.name for o in bpy.data.objects if o.type == 'MESH' and not o.name.startswith(SKIP)]
    lights = make('BV4_Lights', gather(['SH_BuiVien_Lights']))
    street = make('BV4_Street', gather([n for n in meshes if n != 'SH_BuiVien_Lights']))
    gone = kit.strip_figures('BV4_Street', boxes=[VAN])
    for o in list(bpy.data.objects):
        if o not in (street, lights):
            bpy.data.objects.remove(o, do_unlink=True)
    bpy.ops.outliner.orphans_purge(do_recursive=True)
    for o in (street, lights):
        o.data.calc_loop_triangles()
    print('[extract_bui_vien]', 'removed', gone, 'tris street', len(street.data.loop_triangles),
          'lights', len(lights.data.loop_triangles), 'materials', len(bpy.data.materials))
    bpy.ops.wm.save_as_mainfile(filepath=os.path.abspath(out), compress=True)
    print('[extract_bui_vien] saved', os.path.abspath(out))


main()
