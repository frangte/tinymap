"""Hải đăng Vũng Tàu for Sài Gòn mini, map v5: docs/map-v5-tasks.md, T4.

The real lighthouse is a white cylindrical tower 18 m tall and 3 m across on the higher summit of Núi Nhỏ (170 m above
the sea), rebuilt there in 1910; a spiral stair inside, a balcony round the top. Here it stands on the north terrace
of the hill (nui_nho.py): a white tower about 9 m to the gallery, a dark lantern with the lamp, a small dome, and the
keeper's house with a red tile roof beside it. The rotating beam at night is a Three.js job (map-v5.md).

Object LM_HaiDang. run() also takes the blockout lighthouse off LM_Blockout. Needs nui_nho.py (the terrace).
"""
import math
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
if HERE not in sys.path:
    sys.path.insert(0, HERE)
import kit  # noqa: E402
import nui_nho  # noqa: E402
from mathutils import Matrix, Vector  # noqa: E402

PI = math.pi
OLD_BLOCKOUT = [(-54.0, -146.0)]       # where build_new_landmarks put the plain lighthouse
TOWER_H = 8.6                          # shaft, above the plinth
R0, R1 = 1.55, 1.2                     # shaft radius at the foot and under the gallery


def build():
    g = kit.Mesh(11)
    px, py = nui_nho.PN
    z0 = nui_nho.HN + 0.12
    # the road arrives from this side: door and the path face it; the keeper's house sits opposite
    ex, ey = nui_nho.ROAD.coords[-1]
    face = math.atan2(ey - py, ex - px)
    tx, ty = px - 1.2 * math.cos(face), py - 1.2 * math.sin(face)       # tower a little back from the arrival
    T = lambda z: Matrix.Translation((tx, ty, z))  # noqa: E731
    # plinth and shaft
    g.ring('stone', T(z0 - 0.3), 2.1, 1.05, 10)
    g.lathe('opWhite', T(z0 + 0.75), [(R0, 0.0), (R0 - 0.08, 2.5), (R1 + 0.05, 6.0), (R1, TOWER_H)], seg=14)
    zt = z0 + 0.75 + TOWER_H
    # small windows climbing round the shaft with the spiral stair
    for k in range(5):
        a = face + PI + k * 1.25
        z = z0 + 2.0 + k * 1.45
        r = R0 - 0.08 - (R0 - R1) * (z - z0) / (TOWER_H + 0.75) + 0.02
        c = Vector((tx + r * math.cos(a), ty + r * math.sin(a), z))
        M = Matrix.Translation(c) @ Matrix.Rotation(a, 4, 'Z')
        g.box('archDark', -0.05, -0.22, -0.4, 0.05, 0.22, 0.4, M)
    # door towards the arriving road
    c = Vector((tx + (R0 - 0.02) * math.cos(face), ty + (R0 - 0.02) * math.sin(face), z0 + 0.75))
    g.box('archDark', -0.06, -0.5, 0.0, 0.06, 0.5, 1.9, Matrix.Translation(c) @ Matrix.Rotation(face, 4, 'Z'))
    # gallery: corbelled ring, floor, railing
    g.ring('opWhite', T(zt - 0.35), R1 + 0.05, 0.35, 14, r2=R1 + 0.75)
    g.ring('opWhite', T(zt), R1 + 0.8, 0.2, 14)
    n = 16
    for k in range(n):
        a = 2 * PI * k / n
        g.rod('archDark', (tx + (R1 + 0.7) * math.cos(a), ty + (R1 + 0.7) * math.sin(a), zt + 0.2),
              (tx + (R1 + 0.7) * math.cos(a), ty + (R1 + 0.7) * math.sin(a), zt + 1.05), 0.035, 4)
    g.ring('archDark', T(zt + 1.0), R1 + 0.74, 0.07, 16, r2=R1 + 0.74)
    # lantern: glass with dark mullions, the lamp inside, a dark dome with a ball and a vane rod
    zl = zt + 0.2
    g.ring('archDark', T(zl), 0.95, 0.35, 8)
    g.ring('glassBlue', T(zl + 0.35), 0.88, 1.25, 8)
    for k in range(8):
        a = 2 * PI * (k + 0.5) / 8
        g.rod('archDark', (tx + 0.9 * math.cos(a), ty + 0.9 * math.sin(a), zl + 0.35),
              (tx + 0.9 * math.cos(a), ty + 0.9 * math.sin(a), zl + 1.6), 0.05, 4)
    g.ico('lampGlow', (tx, ty, zl + 0.95), 0.42, sub=1)
    g.ring('archDark', T(zl + 1.6), 1.05, 0.12, 8)
    g.lathe('archDark', T(zl + 1.72), [(1.0, 0.0), (0.85, 0.35), (0.5, 0.7), (0.12, 0.85)], seg=8)
    g.ico('archDark', (tx, ty, zl + 2.65), 0.16, sub=1)
    g.rod('archDark', (tx, ty, zl + 2.5), (tx, ty, zl + 3.3), 0.03, 4)
    # keeper's house: one storey, white walls, red hip roof, green shutters, opposite the arrival
    hx, hy = px - 3.6 * math.cos(face + 0.9), py - 3.6 * math.sin(face + 0.9)
    yaw = face + 0.9 + PI / 2
    M = Matrix.Translation((hx, hy, z0)) @ Matrix.Rotation(yaw, 4, 'Z')
    w, d, h = 4.4, 3.2, 2.7
    g.box('creamLight', -w / 2, -d / 2, -0.4, w / 2, d / 2, h, M)
    g.box('stone', -w / 2 - 0.08, -d / 2 - 0.08, -0.4, w / 2 + 0.08, d / 2 + 0.08, 0.3, M)
    # hip roof as a lathe-free wedge: a pyramid with a ridge
    r = [(-w / 2 - 0.35, -d / 2 - 0.35, h), (w / 2 + 0.35, -d / 2 - 0.35, h), (w / 2 + 0.35, d / 2 + 0.35, h), (-w / 2 - 0.35, d / 2 + 0.35, h),
         (-w / 2 + 1.0, 0.0, h + 1.15), (w / 2 - 1.0, 0.0, h + 1.15)]
    g.raw('roofRed', r, [(0, 3, 2, 1), (0, 1, 5, 4), (1, 2, 5), (2, 3, 4, 5), (3, 0, 4)], M)
    for sx in (-1.2, 1.2):
        g.box('archDark', sx - 0.4, -d / 2 - 0.06, 0.9, sx + 0.4, -d / 2 + 0.02, 2.0, M)
        g.box('poGreen', sx - 0.85, -d / 2 - 0.08, 0.85, sx - 0.45, -d / 2 + 0.02, 2.05, M)
        g.box('poGreen', sx + 0.45, -d / 2 - 0.08, 0.85, sx + 0.85, -d / 2 + 0.02, 2.05, M)
    g.box('archDark', -0.45, -d / 2 - 0.06, 0.0, 0.45, -d / 2 + 0.02, 2.1, M)
    # bougainvillea over the terrace wall by the house
    rng = g.rng
    for k in range(4):
        a = face + 0.9 + PI + (k - 1.5) * 0.35
        rr = nui_nho.TERRACE_N - 0.2
        g.ico('flowerPink', (px + rr * math.cos(a), py + rr * math.sin(a), z0 + 0.9), 0.6, sub=1, scale=(1.3, 1.0, 0.75), jit=0.15, rng=rng)
    return g


def run():
    gone = kit.drop_faces_near('LM_Blockout', OLD_BLOCKOUT, 3.5)
    kit.remove_objects(['LM_HaiDang'])
    ob = kit.make_object(build(), 'LM_HaiDang', kit.collection('SM_Landmarks'), recalc=True)
    kit.move_mesh('LM_HaiDang', nui_nho.MX, nui_nho.MY)        # from the design frame to where the hill stands (layout v7)
    kit.add_wobble('LM_HaiDang', 0.045)
    ob.data.calc_loop_triangles()
    return {'object': ob.name, 'tris': len(ob.data.loop_triangles), 'blockout_faces_removed': gone}


if __name__ == '__main__':
    print('HAI_DANG', run())
