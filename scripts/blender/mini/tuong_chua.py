"""Tượng Chúa Kitô Vua (Christ of Vũng Tàu) for Sài Gòn mini, map v5: docs/map-v5-tasks.md, T5.

The real statue (1974) is 32 m tall with an 18.3 m arm span, on a 4 m pedestal, on the lower southern summit of Núi
Nhỏ, looking out to sea; 133 steps inside lead up to the shoulders. Here: a 12 m figure in statueW on a 4 m stone
pedestal on the south terrace of the hill (nui_nho.py), facing south over the cape.

What makes it read from far away: the long robe flaring at the hem, the arms held straight out and a little down,
and the wide sleeves hanging under them like two short wings. The face and hands are only suggested.

Object LM_TuongChua. run() also takes the blockout statue off LM_Blockout. Needs nui_nho.py (the terrace).
"""
import math
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
if HERE not in sys.path:
    sys.path.insert(0, HERE)
import kit  # noqa: E402
import nui_nho  # noqa: E402
from mathutils import Matrix  # noqa: E402

PI = math.pi
OLD_BLOCKOUT = [(-37.0, -169.0)]      # where build_new_landmarks put the plain statue
FACING = -PI / 2                      # looks towards -Y: south, over Mũi Nghinh Phong to the open sea
ARM_SPAN = 7.4                        # fingertip to fingertip (real ratio 18.3 / 32 = 0.57; a little wider here)


def build():
    g = kit.Mesh(12)
    px, py = nui_nho.PS
    z0 = nui_nho.HS + 0.12
    # local frame: x across the arms, y towards the face (+y), z up; then turned so the face looks along FACING
    W = Matrix.Translation((px, py, z0)) @ Matrix.Rotation(FACING - PI / 2, 4, 'Z')

    def at(x, y, z):
        return W @ Matrix.Translation((x, y, z))

    # pedestal: a stepped base, an octagonal drum with a band of relief, a cornice and a small plinth
    g.ring('stone', at(0, 0, -0.3), 3.7, 0.65, 8)
    g.ring('stone', at(0, 0, 0.35), 3.1, 2.8, 8, r2=2.95)
    for k in range(8):                                  # relief panels on the drum faces
        a = 2 * PI * (k + 0.5) / 8
        M = at(2.98 * math.cos(a), 2.98 * math.sin(a), 1.75) @ Matrix.Rotation(a, 4, 'Z')
        g.box('statueW', -0.08, -0.85, -0.75, 0.1, 0.85, 0.75, M)
        g.box('stone', 0.06, -0.55, -0.45, 0.16, -0.25, 0.55, M)
        g.box('stone', 0.06, 0.25, -0.45, 0.16, 0.55, 0.55, M)
    g.ring('stone', at(0, 0, 3.15), 3.35, 0.3, 8)
    g.ring('statueW', at(0, 0, 3.45), 1.55, 0.6, 8, r2=1.4)
    # the figure stands on z = 4.05 (pedestal top)
    zf = 4.05
    # the robe: a lathe with vertical folds that are deep at the hem and fade out at the chest, a little flatter
    # front to back
    robe = [(1.25, 0.0), (1.16, 0.35), (1.02, 1.4), (0.9, 3.4), (0.82, 5.4), (0.8, 6.6), (0.9, 7.8), (0.98, 8.6),
            (0.95, 9.25), (0.6, 9.75), (0.3, 9.95)]
    seg = 20
    V, F = [], []
    for r, z in robe:
        fold = 0.09 * max(0.0, 1.0 - z / 7.5)
        for i in range(seg):
            a = 2 * PI * i / seg
            rr = r * (1.0 + fold * math.cos(7 * a))
            V.append((rr * math.cos(a), 0.8 * rr * math.sin(a), zf + z))
    for k in range(len(robe) - 1):
        for i in range(seg):
            j = (i + 1) % seg
            F.append((k * seg + i, k * seg + j, (k + 1) * seg + j, (k + 1) * seg + i))
    F.append(tuple(range(seg - 1, -1, -1)))
    F.append(tuple(range((len(robe) - 1) * seg, len(robe) * seg)))
    g.raw('statueW', V, F, W)
    # neck, head, hair falling to the shoulders, a short beard
    g.ring('statueW', at(0, 0, zf + 9.85), 0.26, 0.45, 8)
    g.ico('statueW', (W @ Matrix.Translation((0, 0.05, zf + 10.8))).to_translation(), 0.6, sub=1, scale=(0.9, 0.95, 1.12))
    g.lathe('statueW', at(0, -0.12, zf + 9.95) @ Matrix.Diagonal((1.0, 0.8, 1.0, 1.0)), [(0.62, 0.0), (0.6, 0.45), (0.5, 0.85), (0.0, 1.0)], seg=10)
    g.ico('statueW', (W @ Matrix.Translation((0, 0.38, zf + 10.3))).to_translation(), 0.24, sub=1, scale=(1.0, 0.7, 1.2))
    # arms: straight out from the shoulders, a little down, hands open
    zs = zf + 9.35
    reach = ARM_SPAN / 2
    droop = math.radians(9)
    for sx in (-1, 1):
        a = W @ Matrix.Translation((sx * 0.8, 0.0, zs))
        a = a.to_translation()
        hand = (W @ Matrix.Translation((sx * reach * math.cos(droop), 0.12, zs - (reach - 0.7) * math.sin(droop)))).to_translation()
        g.rod('statueW', a, hand, 0.42, 8)
        g.ico('statueW', hand, 0.3, sub=1, scale=(1.1, 0.6, 1.0))
        # the wide sleeve hanging under the arm: a thick wedge from the side of the robe to just short of the wrist
        hx = sx * (reach - 0.55)
        hz = zs - (reach - 0.55 - 0.7) * math.sin(droop)
        pts = [(sx * 0.6, zs + 0.15), (hx, hz + 0.1), (hx * 0.98, hz - 0.85), (sx * 0.75, zs - 1.75)]
        V = [(x, y, z) for x, z in pts for y in (-0.26,)] + [(x, y, z) for x, z in pts for y in (0.26,)]
        F = [(0, 1, 2, 3), (7, 6, 5, 4), (0, 4, 5, 1), (1, 5, 6, 2), (2, 6, 7, 3), (3, 7, 4, 0)]
        if sx < 0:
            F = [tuple(reversed(f)) for f in F]
        g.raw('statueW', V, F, W @ Matrix.Translation((0, 0, 0)))
    # the sash / belt line and the hem band suggested with a slightly different white
    g.ring('stone', at(0, 0, zf + 6.55) @ Matrix.Diagonal((1.0, 0.8, 1.0, 1.0)), 0.83, 0.18, 12)
    return g


def run():
    gone = kit.drop_faces_near('LM_Blockout', OLD_BLOCKOUT, 4.5)
    kit.remove_objects(['LM_TuongChua'])
    ob = kit.make_object(build(), 'LM_TuongChua', kit.collection('SM_Landmarks'), recalc=True)
    kit.move_mesh('LM_TuongChua', nui_nho.MX, nui_nho.MY)        # from the design frame to where the hill stands (layout v7)
    kit.add_wobble('LM_TuongChua', 0.045)
    ob.data.calc_loop_triangles()
    return {'object': ob.name, 'tris': len(ob.data.loop_triangles), 'blockout_faces_removed': gone,
            'height': round(nui_nho.HS + 0.12 + 4.05 + 11.3, 1)}


if __name__ == '__main__':
    print('TUONG_CHUA', run())
