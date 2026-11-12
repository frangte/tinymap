"""Dinh Thống Nhất (Dinh Độc Lập, Ngô Viết Thụ, 1962-1966) for Sài Gòn mini, map v5: #16, docs/map-v5-tasks.md, T12c.

What makes it read: a long white modernist block on a raised stone terrace, long horizontal lines (the floor slabs and
balconies), the "rèm hoa đá" screen of bamboo-node concrete fins across the two upper floors, the wide entrance canopy
on two wood-clad columns, the rooftop pavilion (Tứ phương vô sự lâu) with its thin roof, the flag on the kỳ đài at the
very top, the helipad on the roof, and the round fountain on the lawn in front. Ngô Viết Thụ composed these lines as
Chinese characters (the plan as 吉, the canopy, rooftop roof and balcony as 三, with the flagpole 王 and 主).

The facade faces east, down the lawn towards the cathedral, as on Lê Duẩn. 40 m wide, 22 m deep, about 16 m to the
pavilion roof (the real one is about 26 m: heights are squeezed on this map).

Object LM_DinhThongNhat (and its forecourt in LF_DinhThongNhat). Placement from saigon_mini_layout.json (#16,
'dinh_doc_lap': at, size as the plan extents x and y, face E/N/W/S, and 'drive': metres from the centre to where the
drive meets the street or the park walk). The model is built facing local +x and turned to the face. run() takes the
old blockout box off LM_Blockout and lawn trees off the forecourt.
"""
import math
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
if HERE not in sys.path:
    sys.path.insert(0, HERE)
import kit  # noqa: E402
import layout as LY  # noqa: E402
from mathutils import Matrix  # noqa: E402

PI = math.pi
LM = LY.landmarks['dinh_doc_lap']
CX, CY = LY.tb(LM['at'])
FACE = LM.get('face', 'E')
FACING = {'E': 0.0, 'N': PI / 2, 'W': PI, 'S': -PI / 2}[FACE]     # Blender angle of the facade (local +x)
_sx, _sy = LM['size']             # plan extents
D, W = (_sx, _sy) if FACE in ('E', 'W') else (_sy, _sx)           # depth along the facing, width of the facade
DRIVE = LM.get('drive', 43.0)     # centre to the end of the drive (v7: the pavement of the street at plan x 100)

GF, F2, F3, ROOF = 1.0, 4.6, 8.0, 11.4      # terrace top, floor 2, floor 3, roof slab
SCREEN = 15.0                                # half width of the stone screen


def build():
    g = kit.Mesh(16)
    M = Matrix.Translation((CX, CY, 0.0)) @ Matrix.Rotation(FACING, 4, 'Z')
    hd, hw = D / 2, W / 2
    fx = hd - 1.4                            # facade line of the upper floors

    def box(mat, x0, y0, z0, x1, y1, z1):
        g.box(mat, x0, y0, z0, x1, y1, z1, M)

    # raised stone terrace and the wide front steps
    box('stone', -hd - 1.0, -hw - 1.0, -0.3, hd + 1.5, hw + 1.0, GF)
    for k in range(4):
        box('stone', hd + 1.5 + k * 0.7, -9.0 + k * 0.4, -0.3, hd + 2.2 + k * 0.7, 9.0 - k * 0.4, GF - 0.25 * (k + 1))
    # ground floor: dark glass set back under the upper floors, white columns along the front
    box('glassDark', -hd + 1.0, -hw + 1.0, GF, fx - 1.2, hw - 1.0, F2)
    y = -hw + 2.0
    while y <= hw - 1.9:
        box('opWhite', fx - 0.9, y - 0.3, GF, fx - 0.3, y + 0.3, F2)
        y += 3.6
    # upper floors: the white mass, dark glass behind the screen, solid end bays with a few windows
    box('opWhite', -hd, -hw, F2, fx - 0.6, hw, ROOF)
    box('glassDark', fx - 0.6, -SCREEN, F2 + 0.2, fx - 0.45, SCREEN, ROOF - 0.15)
    for sy in (-1, 1):
        box('opWhite', fx - 0.6, sy * SCREEN, F2, fx, sy * hw, ROOF)
        for z in (F2 + 1.1, F3 + 1.1):
            for yy in (17.0, 18.6):
                box('glassDark', fx, sy * yy - 0.45, z, fx + 0.05, sy * yy + 0.45, z + 1.4)
    # long horizontal lines: the balcony slabs at floors 2 and 3, and the roof slab with its cornice
    box('opWhite', fx - 0.6, -hw - 0.6, F2 - 0.35, fx + 1.9, hw + 0.6, F2)
    box('opWhite', fx - 0.6, -SCREEN, F3 - 0.2, fx + 1.2, SCREEN, F3)
    box('opWhite', -hd - 0.5, -hw - 0.7, ROOF, fx + 1.9, hw + 0.7, ROOF + 0.45)
    box('opWhite', -hd - 0.2, -hw - 0.4, ROOF + 0.45, fx + 1.6, -hw + 0.0, ROOF + 1.0)       # low parapets
    box('opWhite', -hd - 0.2, hw - 0.0, ROOF + 0.45, fx + 1.6, hw + 0.4, ROOF + 1.0)
    # the stone screen: vertical fins with bamboo nodes, standing on the floor-2 balcony up to the roof slab
    n = int(2 * SCREEN / 0.9) + 1
    for k in range(n):
        y = -SCREEN + k * (2 * SCREEN / (n - 1))
        x0, x1 = fx + 0.9, fx + 1.3
        box('opWhite', x0, y - 0.11, F2, x1, y + 0.11, ROOF)
        z = F2 + 0.55
        while z < ROOF - 0.3:
            box('stone', x0 - 0.05, y - 0.17, z, x1 + 0.05, y + 0.17, z + 0.12)
            z += 1.15
    # entrance canopy (mái hiên tiền sảnh) on two wood-clad columns, over the steps
    box('opWhite', fx + 1.9, -7.5, F2 - 0.55, hd + 6.5, 7.5, F2 - 0.05)
    box('opWhite', hd + 6.3, -7.5, F2 - 0.85, hd + 6.5, 7.5, F2 - 0.05)
    for sy in (-1, 1):
        g.ring('wood', M @ Matrix.Translation((hd + 5.4, sy * 5.2, GF)), 0.45, F2 - 0.55 - GF, 10)
    # rooftop pavilion: glass between slim white posts under a thin wide roof (the third line of 三)
    px0, px1, py = -4.0, 5.0, 7.0
    box('glassBlue', px0 + 0.3, -py + 0.3, ROOF + 0.45, px1 - 0.3, py - 0.3, ROOF + 3.4)
    for xx in (px0, px1):
        for yy in (-py, -py / 3, py / 3, py):
            box('opWhite', xx - 0.2, yy - 0.2, ROOF + 0.45, xx + 0.2, yy + 0.2, ROOF + 3.4)
    box('opWhite', px0 - 1.2, -py - 1.4, ROOF + 3.4, px1 + 1.6, py + 1.4, ROOF + 3.75)
    # kỳ đài and the flag at the very top
    box('stone', 0.0, -1.0, ROOF + 3.75, 2.0, 1.0, ROOF + 4.4)
    g.rod('metal', M @ Matrix.Translation((1.0, 0.0, ROOF + 4.4)).to_translation(), (M @ Matrix.Translation((1.0, 0.0, ROOF + 11.0))).to_translation(), 0.07, 6)
    box('signRed', 0.98, 0.1, ROOF + 9.4, 1.02, 2.7, ROOF + 11.0)
    box('signGold', 0.96, 1.15, ROOF + 9.95, 1.04, 1.65, ROOF + 10.45)
    # helipad at the back of the roof: a pad, a red ring and a white H
    hx, hy = -hd + 4.2, hw - 6.0
    box('concrete', hx - 3.2, hy - 3.2, ROOF + 0.45, hx + 3.2, hy + 3.2, ROOF + 0.55)
    ring = M @ Matrix.Translation((hx, hy, ROOF + 0.55))
    g.ring('sigRed', ring, 2.7, 0.03, 20, caps=True)
    g.ring('concrete', M @ Matrix.Translation((hx, hy, ROOF + 0.56)), 2.3, 0.03, 20, caps=True)
    box('white', hx - 1.0, hy - 0.9, ROOF + 0.59, hx + 1.0, hy - 0.55, ROOF + 0.61)
    box('white', hx - 1.0, hy + 0.55, ROOF + 0.59, hx + 1.0, hy + 0.9, ROOF + 0.61)
    box('white', hx - 0.17, hy - 0.9, ROOF + 0.59, hx + 0.17, hy + 0.9, ROOF + 0.61)
    return g


def local(x, y):
    """world XY -> the model's frame (x along the facing)"""
    dx, dy = x - CX, y - CY
    c, s_ = math.cos(-FACING), math.sin(-FACING)
    return dx * c - dy * s_, dx * s_ + dy * c


def forecourt():
    """the round fountain on the front lawn and the drive from the steps round it to the street"""
    g = kit.Mesh(17)
    hd = D / 2
    M = Matrix.Translation((CX, CY, 0.0)) @ Matrix.Rotation(FACING, 4, 'Z')
    fxc = hd + 17.0                                   # fountain centre, in front of the steps
    # paved ring round the fountain and the straight drive in from the street and up to the steps
    seg = 40
    r0, r1 = 8.0, 10.5
    for k in range(seg):
        a0, a1 = 2 * PI * k / seg, 2 * PI * (k + 1) / seg
        pts = [(fxc + r0 * math.cos(a0), r0 * math.sin(a0)), (fxc + r1 * math.cos(a0), r1 * math.sin(a0)),
               (fxc + r1 * math.cos(a1), r1 * math.sin(a1)), (fxc + r0 * math.cos(a1), r0 * math.sin(a1))]
        V = [(x, y, -0.1) for x, y in pts] + [(x, y, 0.06) for x, y in pts]
        g.raw('paving', V, [(0, 3, 2, 1), (4, 5, 6, 7), (0, 1, 5, 4), (1, 2, 6, 5), (2, 3, 7, 6), (3, 0, 4, 7)], M)
    g.box('paving', hd + 4.0, -3.5, -0.1, fxc - r0 - 0.5 + 2.6, 3.5, 0.06, M)
    g.box('paving', fxc + r1 - 0.5, -3.5, -0.1, DRIVE, 3.5, 0.06, M)              # to the street (or park walk) in front
    # the fountain: stone rim, water, a tiered jet
    g.ring('stone', M @ Matrix.Translation((fxc, 0, -0.1)), 5.2, 0.7, 24)
    g.ring('water', M @ Matrix.Translation((fxc, 0, 0.0)), 4.7, 0.62, 24)
    g.ring('stone', M @ Matrix.Translation((fxc, 0, 0.0)), 1.2, 1.4, 12, r2=0.9)
    g.ring('foam', M @ Matrix.Translation((fxc, 0, 1.4)), 0.5, 1.8, 8, r2=0.08)
    g.ring('foam', M @ Matrix.Translation((fxc, 0, 0.62)), 2.2, 0.08, 16, r2=1.0)
    fx_, fy_ = CX + fxc * math.cos(FACING), CY + fxc * math.sin(FACING)
    return g, (fx_, fy_), r1


def run():
    gone = kit.drop_faces_near('LM_Blockout', [(CX, CY)], max(D, W) / 2 + 1.0)
    kit.remove_objects(['LM_DinhThongNhat', 'LF_DinhThongNhat'])
    coll = kit.collection('SM_Landmarks')
    ob = kit.make_object(build(), 'LM_DinhThongNhat', coll, recalc=True)
    fg, (fx, fy), fr = forecourt()
    kit.make_object(fg, 'LF_DinhThongNhat', coll, recalc=True)
    hd = D / 2
    trees = kit.drop_loose_parts('CX_Trees', lambda x, y: (x - fx) ** 2 + (y - fy) ** 2 < (fr + 2.5) ** 2 or
                                 (hd < local(x, y)[0] < DRIVE and abs(local(x, y)[1]) < 5.5))
    kit.add_wobble('LM_DinhThongNhat', 0.03)
    kit.add_wobble('LF_DinhThongNhat', 0.03)
    ob.data.calc_loop_triangles()
    return {'object': ob.name, 'tris': len(ob.data.loop_triangles), 'blockout_faces_removed': gone, 'lawn_tree_parts_removed': trees,
            'centre': (round(CX, 1), round(CY, 1))}


if __name__ == '__main__':
    print('DINH_THONG_NHAT', run())
