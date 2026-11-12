"""Phố đi bộ Bùi Viện (#13) for Sài Gòn mini, layout v7: the v4 street block brought back whole.

The street Khoa liked in the previous map (models/temp.sg.tinymap.blend): two rows of three- to five-storey
shophouses with bars and neon-style signs, striped awnings, balconies and air-conditioners, plastic stools and tables on
the pavement, string lights and lanterns across the street, and the gate (PHỐ ĐI BỘ BÙI VIỆN) at the east end, opening on
Tôn Đức Thắng and the river. No people, scooters or cars: they are NPCs in the game, added by code.

Source: models/bui_vien_v4.blend (made once by extract_bui_vien.py from the v4 file). The block is only moved (no turn,
no rescale): its street axis (v4 y = -24) on the layout's Bùi Viện walk, its centre at the landmark's 'at'. The walk
(15 m) paves the street; the houses stand on the ground at 0 like the v4 ones, the stools on the pavement level (0.18).

Objects: SH_BuiVien (houses, signs, stools, trees, the gate) and SH_BuiVien_Lights (string lights, lanterns). The v4
materials are matched to this file's SD_ paints by name; a colour the file does not have yet is added as a new SD_
paint with the v4 colour (the same game palette).
"""
import os
import sys

import bpy

HERE = os.path.dirname(os.path.abspath(__file__))
if HERE not in sys.path:
    sys.path.insert(0, HERE)
import kit  # noqa: E402
import layout as LY  # noqa: E402

SRC = os.path.join(LY.ROOT, 'models', 'bui_vien_v4.blend')
V4_CENTRE = (-35.25, -24.0)          # v4 Blender XY: middle of the block between its west end and the gate, on the axis
NAMES = {'BV4_Street': ('SH_BuiVien', 'SM_Houses'), 'BV4_Lights': ('SH_BuiVien_Lights', 'SM_Props')}


def _is_paint(m):
    return m is not None and m.node_tree is not None and any(n.type == 'GROUP' and 'Color' in n.inputs for n in m.node_tree.nodes)


def _paint_for(m):
    """this file's SD_ paint for an appended v4 material: the same name, or a new paint in the v4 colour"""
    base = m.name.split('.')[0]
    have = bpy.data.materials.get(base)
    if have is not None and have is not m and _is_paint(have):
        return have
    if _is_paint(m):
        return m
    col = tuple(m.diffuse_color)
    if m.node_tree:
        for n in m.node_tree.nodes:
            if n.type == 'BSDF_PRINCIPLED':
                col = tuple(n.inputs['Base Color'].default_value)
    src = next((bpy.data.materials[n] for n in ('SD_stone', 'SD_concrete', 'SD_white') if n in bpy.data.materials and _is_paint(bpy.data.materials[n])), None)
    if src is None:
        return m
    if have is m:
        m.name = base + '_v4src'
    new = src.copy()
    new.name = base
    for n in new.node_tree.nodes:
        if n.type == 'GROUP' and 'Color' in n.inputs:
            n.inputs['Color'].default_value = col
    new.diffuse_color = col
    return new


def run():
    kit.remove_objects([v[0] for v in NAMES.values()])
    with bpy.data.libraries.load(SRC, link=False) as (src, dst):
        dst.objects = [n for n in NAMES if n in src.objects]
    lm = LY.landmarks['bui_vien']
    cx, cy = LY.tb(lm['at'])
    dx, dy = cx - V4_CENTRE[0], cy - V4_CENTRE[1]
    out = {}
    appended = {m for ob in dst.objects if ob is not None for m in ob.data.materials if m is not None}
    for ob in dst.objects:
        if ob is None:
            continue
        name, coll = NAMES[ob.name]
        me = ob.data
        for i, m in enumerate(me.materials):
            if m is not None:
                me.materials[i] = _paint_for(m)
        ob.name = me.name = name
        kit.collection(coll).objects.link(ob)
        kit.move_mesh(name, dx, dy)
        kit.add_wobble(name, 0.07 if name == 'SH_BuiVien' else 0.03)
        me.calc_loop_triangles()
        out[name] = len(me.loop_triangles)
    for m in appended:
        if m.users == 0:
            bpy.data.materials.remove(m)
    out['move'] = (round(dx, 2), round(dy, 2))
    return out


if __name__ == '__main__':
    print('BUI_VIEN', run())
