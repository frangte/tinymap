"""Walk-map rules kept in the map file: boxes in the NAV collection, read by bake_nav.py, skipped by export_glb.py.

Each box is an upright cube; only its outline in Blender XY counts (rotate it about Z freely, its height is just
for seeing it in the viewport):

  NAV_Solid_*  a building nobody walks into: inside the box, every cell with geometry more than 2 m over its floor
               is blocked (a roof, an upper floor, a canopy). The box can be loose: ground with nothing overhead
               stays walkable, so keep it off arcades and pavements under awnings that people should walk through.
  NAV_Block_*  every cell inside is blocked.
  NAV_Open_*   nothing inside counts as an obstacle at body height (water, drops and steps still do).

This script makes or moves the boxes listed below, by name; boxes added by hand are left alone. Either

    blender models/temp.sg.tinymap.blend --background --python scripts/blender/nav_boxes.py [-- out.blend]

(saves over the file, or into out.blend), or open it in Blender's Text editor, Run Script, and save the file.
Then re-bake the nav maps (bake_nav.py) and copy them to the server (npm run nav:sync).
"""
import sys

import bpy

COLLECTION = 'NAV'
HEIGHT = 4.0   # drawn height of a box (m); the bake ignores it

# name: (x0, x1, y0, y1) in Blender XY metres, drawn from the floor (z 0)
BOXES = {
    # the v4 buildings: their blocked footprints in the nav baked from the original objects (2026-11-05), kept off
    # the walkways under the market's eaves and the old houses' awnings
    'NAV_Solid_ChoBenThanh': (-6.30, 22.40, -6.30, 21.40),
    'NAV_Solid_ChoBenThanh_SouthGate': (2.80, 12.60, -10.50, -6.30),
    'NAV_Solid_NhaHat': (96.25, 115.50, -15.75, 7.70),
    'NAV_Solid_NhaTho': (68.95, 87.15, 64.05, 99.75),
    'NAV_Solid_BuuDien': (93.80, 106.05, 39.55, 68.60),
    'NAV_Solid_Bitexco': (32.20, 55.65, -72.45, -51.80),
    'NAV_Solid_DayNhaPho': (-44.80, -18.90, -17.25, -6.65),
    # Bùi Viện (scripts/blender/edit_v4_buivien.py): from the house fronts (y -16.3 north, -31.6 south, give or take
    # a 0.25 m jut) to the back, so the pavement under the balconies and sign bands stays walkable
    'NAV_Solid_BuiVien_N_West': (-60.05, -42.95, -16.05, -7.28),
    'NAV_Solid_BuiVien_N_East': (-19.05, -9.49, -16.05, -6.68),
    'NAV_Solid_BuiVien_S_West': (-59.01, -30.01, -36.51, -31.85),
    'NAV_Solid_BuiVien_S_East': (-29.93, -9.95, -40.92, -31.85),
}


def box_mesh(name):
    me = bpy.data.meshes.get(name) or bpy.data.meshes.new(name)
    me.clear_geometry()
    v = [(x, y, z) for z in (0.0, HEIGHT) for x, y in ((-0.5, -0.5), (0.5, -0.5), (0.5, 0.5), (-0.5, 0.5))]
    f = [(0, 3, 2, 1), (4, 5, 6, 7), (0, 1, 5, 4), (1, 2, 6, 5), (2, 3, 7, 6), (3, 0, 4, 7)]
    me.from_pydata(v, [], f)
    me.update()
    return me


def collection(scene):
    col = bpy.data.collections.get(COLLECTION)
    if col is None:
        col = bpy.data.collections.new(COLLECTION)
    if col.name not in scene.collection.children:
        scene.collection.children.link(col)
    col.hide_render = True
    col.color_tag = 'COLOR_05'
    return col


def main():
    scene = bpy.context.scene
    col = collection(scene)
    for name, (x0, x1, y0, y1) in BOXES.items():
        ob = bpy.data.objects.get(name)
        if ob is None:
            ob = bpy.data.objects.new(name, box_mesh(name + '_box'))
        if ob.name not in col.objects:
            col.objects.link(ob)
        for other in list(ob.users_collection):
            if other != col:
                other.objects.unlink(ob)
        ob.location = ((x0 + x1) / 2, (y0 + y1) / 2, 0.0)
        ob.rotation_euler = (0.0, 0.0, 0.0)
        ob.scale = (x1 - x0, y1 - y0, 1.0)
        ob.display_type = 'WIRE'
        ob.show_name = True
        ob.hide_render = True
    print('[nav_boxes]', len(BOXES), 'boxes in', COLLECTION, flush=True)
    if bpy.app.background:
        argv = sys.argv[sys.argv.index('--') + 1:] if '--' in sys.argv else []
        if argv:
            bpy.ops.wm.save_as_mainfile(filepath=bpy.path.abspath(argv[0]), copy=True)
        else:
            bpy.ops.wm.save_mainfile()
        print('[nav_boxes] saved', argv[0] if argv else bpy.data.filepath, flush=True)


main()
