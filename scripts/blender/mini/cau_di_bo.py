"""Cầu đi bộ qua sông Sài Gòn (the Nutifood footbridge, "lá dừa nước") for Sài Gòn mini, in place of its blockout deck.

The real bridge (2025-2026): 720 m long, a 187 m main span hung from a spatial steel arch drawn as a nipa palm frond
(lá dừa nước), a steel deck 6-11 m wide with a bike lane in the middle and wide walks on both sides. Here, on the
layout's path from the Bạch Đằng quay (Nguyễn Huệ) to the Thủ Thiêm park:

- the deck: a leaf in plan (6 m at the ends, 11 m over the middle of the river), a white steel box girder, a green
  bike lane down the middle with planters as soft dividers over the river, white railings (a kerb, upright bars 0.4 m
  apart and a handrail: nothing to climb on, too tight for a body);
- the frond: its rachis is the main arch, springing on both banks north of the deck and leaning over it; long
  leaflets on both sides angle towards the tip (the Bạch Đằng end), the inner ones a canopy high over the deck;
- a second, slimmer white arch on the south side, leaning in, with struts up to the frond (the spatial arch);
- hangers from both arches to the deck edges, outside the railings.

The arches, posts and rails are SD_steelWhite and the leaflets SD_etfe: the game camera looks through them
(collider.ts THIN). The hangers are SD_cable: nothing collides with them (collider.ts PASS_THROUGH), so they never
snag a body walking along the railing.

Runs inside an open models/saigon_mini.blend (interactive or --background). It removes the footbridge part of the
blockout object LM_Bridges and builds LM_CauDiBo; build_saigon_mini.py leaves the bridge out of LM_Bridges
(MODELLED_BRIDGES) and runs this file (MINI_SCRIPTS). Running it again rebuilds the bridge.
"""
import math
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
if HERE not in sys.path:
    sys.path.insert(0, HERE)
import kit  # noqa: E402
import layout as LY  # noqa: E402
from mathutils import Matrix, Vector  # noqa: E402

PI = math.pi
for _k, _v in {'steelWhite': '#F1F2EE', 'etfe': '#E3EDDC', 'cable': '#C9CED2', 'poGreen': '#3E7F5F'}.items():
    kit.PAL.setdefault(_k, _v)

# ---- layout (Blender XY, metres): saigon_mini_layout.json "cau_di_bo", M 294.5,140 L 337,140
Y0 = 91.0
XA, XB = 43.5, 86.0              # deck ends: the Bạch Đằng promenade, the Thủ Thiêm park
LEN = XB - XA
WALK = LY.WALK_Z
DECK = 2.8                       # deck top over the river
RAMP = 16.0                      # metres from walk level to the deck at each end
HW0, HW1 = 3.0, 5.5              # deck half width at the ends and over the middle
BIKE = 0.9                       # half width of the bike lane
STEP = 0.5                       # deck stations
BALUSTER = 0.4                   # railing bar spacing

# the frond: rachis from the tip (Bạch Đằng bank) to the stalk (Thủ Thiêm bank), north of the deck, leaning over it
R_TIP = Vector((45.6, Y0 + 5.6, WALK))
R_STALK = Vector((77.2, Y0 + 8.0, WALK))
R_RISE, R_LEAN = 11.2, 4.4
# the south arch
S_A = Vector((46.2, Y0 - 6.2, WALK))
S_B = Vector((76.4, Y0 - 8.0, WALK))
S_RISE, S_LEAN = 7.6, -2.4       # leans north, in over the deck
HEAD = 3.0                       # nothing of the arches lower than this over the deck (a body is ~1.8 m)


def smooth(a, b, x):
    t = min(1.0, max(0.0, (x - a) / (b - a)))
    return t * t * (3 - 2 * t)


def deck_z(x):
    s = min(max(x - XA, 0.0), LEN)
    return WALK + (DECK - WALK) * smooth(0.0, RAMP, min(s, LEN - s))


def half_w(x):
    s = min(max(x - XA, 0.0), LEN)
    return HW0 + (HW1 - HW0) * math.sin(PI * s / LEN)


def over_deck(p, margin=0.4):
    return XA - margin <= p.x <= XB + margin and abs(p.y - Y0) <= half_w(p.x) + margin


def arch(a, b, rise, lean, t):
    s = math.sin(PI * t)
    return a.lerp(b, t) + Vector((0.0, -lean * s, rise * s ** 0.8))


def rachis(t):
    return arch(R_TIP, R_STALK, R_RISE, R_LEAN, t)


def south(t):
    return arch(S_A, S_B, S_RISE, S_LEAN, t)


def tangent(f, t, h=1e-3):
    return (f(min(1.0, t + h)) - f(max(0.0, t - h))).normalized()


def clear_of_deck(p):
    return not over_deck(p) or p.z >= deck_z(p.x) + HEAD


# ---------------------------------------------------------------------------------------------------------------
def deck(g):
    n = int(round(LEN / STEP))
    xs = [XA + LEN * i / n for i in range(n + 1)]
    for x0, x1 in zip(xs, xs[1:]):
        z0, z1 = deck_z(x0), deck_z(x1)
        h0, h1 = half_w(x0), half_w(x1)
        # walks and the bike lane
        for (a0, b0), (a1, b1), key in (((BIKE, h0), (BIKE, h1), 'paving'), ((-h0, -BIKE), (-h1, -BIKE), 'paving'),
                                       ((-BIKE, BIKE), (-BIKE, BIKE), 'poGreen')):
            g.face_out(key, [(x0, Y0 + a0, z0), (x1, Y0 + a1, z1), (x1, Y0 + b1, z1), (x0, Y0 + b0, z0)], (x0, Y0, z0 - 1))
        # girder: fascias and soffit, deeper over the river
        d0 = 0.45 + 0.45 * math.sin(PI * (x0 - XA) / LEN)
        d1 = 0.45 + 0.45 * math.sin(PI * (x1 - XA) / LEN)
        for sgn in (1, -1):
            e0, e1 = Y0 + sgn * h0, Y0 + sgn * h1
            g.face_out('bridgeWhite', [(x0, e0, z0), (x1, e1, z1), (x1, e1 - sgn * 0.5, z1 - d1), (x0, e0 - sgn * 0.5, z0 - d0)],
                       (x0, Y0, z0 - 0.3))
        g.face_out('bridgeWhite', [(x0, Y0 + h0 - 0.5, z0 - d0), (x1, Y0 + h1 - 0.5, z1 - d1), (x1, Y0 - h1 + 0.5, z1 - d1),
                                   (x0, Y0 - h0 + 0.5, z0 - d0)], (x0, Y0, z0))
    # dashes down the bike lane
    x = XA + 1.0
    while x < XB - 1.6:
        z = deck_z(x + 0.4) + 0.012
        g.face_out('roadLine', [(x, Y0 - 0.06, z), (x + 0.8, Y0 - 0.06, z), (x + 0.8, Y0 + 0.06, z), (x, Y0 + 0.06, z)], (x, Y0, z - 1))
        x += 2.2


def railings(g):
    """kerb, upright balusters and a handrail: no rung to climb on (horizontal bars were a ladder for a body that
    steps up 0.5 m), gaps of 0.36 m that a body (r 0.26) cannot pass"""
    n = int(round(LEN / 1.0))                                       # kerb and handrail in 1 m pieces
    for sgn in (1, -1):
        P = [Vector((XA + LEN * i / n, 0, 0)) for i in range(n + 1)]
        P = [Vector((p.x, Y0 + sgn * (half_w(p.x) - 0.08), deck_z(p.x))) for p in P]
        g.strip('bridgeWhite', P, 0.05, 0.0, 0.16)                  # kerb
        g.strip('steelWhite', P, 0.05, 1.04, 1.14)                  # handrail
        k = int(LEN / BALUSTER)
        for i in range(k + 1):
            x = XA + 0.2 + (LEN - 0.4) * i / k
            y, z = Y0 + sgn * (half_w(x) - 0.08), deck_z(x)
            r = 0.045 if i % 3 == 0 else 0.018                      # a post every 1.2 m, thin bars between
            g.box('steelWhite', x - r, y - r, z + 0.1, x + r, y + r, z + 1.06)


def planters(g):
    """soft dividers between the bike lane and the walks, over the middle of the river"""
    x = XA + LEN / 2 - 9.0
    while x <= XA + LEN / 2 + 9.0:
        z = deck_z(x)
        for sgn in (1, -1):
            y = Y0 + sgn * (BIKE + 0.35)
            g.box('concreteLight', x - 0.7, y - 0.25, z, x + 0.7, y + 0.25, z + 0.42)
            g.box('earth', x - 0.62, y - 0.18, z + 0.42, x + 0.62, y + 0.18, z + 0.45)
            for dx in (-0.35, 0.35):
                g.ico('leafDark', (x + dx, y, z + 0.6), 0.3, sub=1, scale=(1.0, 0.75, 0.6), jit=0.12)
        x += 4.5


def piers(g):
    for x in (47.6, 74.6):
        z = deck_z(x) - (0.45 + 0.45 * math.sin(PI * (x - XA) / LEN))
        g.box('concrete', x - 0.6, Y0 - 2.4, LY.SEA - 1.0, x + 0.6, Y0 + 2.4, z)
        g.box('concrete', x - 0.8, Y0 - 2.6, LY.SEA - 1.0, x + 0.8, Y0 + 2.6, LY.SEA + 0.25)


def frond(g):
    # the rachis, thicker at the stalk
    N = 56
    P = [rachis(i / N) for i in range(N + 1)]
    g.tube('steelWhite', P, [0.2 + 0.24 * (i / N) ** 1.4 for i in range(N + 1)], seg=8)
    for p in (P[0], P[-1]):
        g.ring('bridgeWhite', Matrix.Translation((p.x, p.y, LY.SEA)), 0.75, WALK + 0.3 - LY.SEA, 8, r2=0.5)
    # leaflets: long narrow blades angled to the tip, drooping; the inner ones over the deck stay HEAD clear of it
    K = 60
    for i in range(K):
        t = 0.035 + 0.93 * i / (K - 1)
        p = rachis(t)
        T = tangent(rachis, t)
        n = Vector((-T.y, T.x, 0.0)).normalized()                 # north, away from the deck
        fn = T.cross(n).normalized()
        if fn.z < 0:
            fn = -fn
        s = math.sin(PI * t)
        for side, length, droop in ((1, 0.7 + 4.4 * s ** 0.7, 0.42), (-1, 0.6 + 4.2 * s ** 0.7, 0.12)):
            d = (-0.55 * T + 0.84 * n * side - Vector((0, 0, droop))).normalized()
            pts = [p + d * length * u - Vector((0, 0, 0.5 * length * u * u * droop)) for u in (0.0, 0.34, 0.67, 1.0)]
            # shorten a blade that would come down into the walk, or into the river
            keep = len(pts)
            for k, q in enumerate(pts):
                if not clear_of_deck(q) or q.z < WALK + 0.8:
                    keep = k
                    break
            if keep < 2:
                continue
            pts = pts[:keep]
            w = Vector(fn.cross(d)).normalized()
            rings = []
            for k, q in enumerate(pts):
                u = k / 3
                hw = 0.24 * (1 - u) + 0.04
                rings.append([q - w * hw - fn * 0.035, q + w * hw - fn * 0.035, q + w * hw + fn * 0.035, q - w * hw + fn * 0.035])
            g.sweep('etfe', pts, rings)
    return P


def south_arch(g):
    N = 48
    P = [south(i / N) for i in range(N + 1)]
    g.tube('steelWhite', P, 0.17, seg=6)
    for p in (P[0], P[-1]):
        g.ring('bridgeWhite', Matrix.Translation((p.x, p.y, LY.SEA)), 0.5, WALK + 0.25 - LY.SEA, 8, r2=0.32)
    # struts from the south arch up to the frond: the arch is one spatial frame
    for t in (0.3, 0.4, 0.5, 0.6, 0.7):
        a, b = south(t), rachis(t)
        g.tube('steelWhite', [a, a.lerp(b, 0.5), b], 0.07, seg=5, caps=False)


def hangers(g):
    """from both arches down to the deck edges, outside the railings; SD_cable, nothing collides with them"""
    for f, sgn, ts in ((rachis, 1, [0.14 + 0.06 * i for i in range(13)]), (south, -1, [0.16 + 0.06 * i for i in range(12)])):
        for t in ts:
            a = f(t)
            x = a.x
            b = Vector((x, Y0 + sgn * (half_w(x) + 0.08), deck_z(x) - 0.1))
            if a.z - b.z < 1.5:
                continue
            g.tube('cable', [a, b], 0.03, seg=4, caps=False)
            g.box('bridgeWhite', x - 0.12, b.y - 0.12, b.z - 0.15, x + 0.12, b.y + 0.12, b.z + 0.1)


def check():
    """the arches clear the walk: over the walk (inside the railings, less a body's radius) nothing lower than HEAD
    above the deck, over the railings nothing lower than 1.25 m"""
    bad = []
    for f, name, r in ((rachis, 'frond', 0.45), (south, 'south', 0.2)):
        for i in range(401):
            p = f(i / 400)
            if not XA <= p.x <= XB:
                continue
            lat, bot = abs(p.y - Y0) - r, p.z - r - deck_z(p.x)
            if (lat < half_w(p.x) - 0.34 and bot < HEAD) or (lat < half_w(p.x) + 0.1 and bot < 1.25):
                bad.append((name, round(p.x, 1), round(p.y, 1), round(bot, 2)))
    return bad


def run():
    kit.remove_objects(['LM_CauDiBo'])
    gone = kit.drop_faces_near('LM_Bridges', [(x, Y0) for x in [XA - 1 + 0.5 * k for k in range(int((LEN + 2) / 0.5) + 1)]], 6.5)
    g = kit.Mesh(seed=7)
    deck(g)
    railings(g)
    planters(g)
    piers(g)
    frond(g)
    south_arch(g)
    hangers(g)
    ob = kit.make_object(g, 'LM_CauDiBo', kit.collection('SM_Landmarks'))
    ob.data.calc_loop_triangles()
    return {'object': ob.name, 'tris': len(ob.data.loop_triangles), 'blockout_faces_removed': gone, 'clearance': check()}


if __name__ == '__main__':
    print('CAU_DI_BO', run())
    # alone on the map: blender models/saigon_mini.blend --background --python scripts/blender/mini/cau_di_bo.py -- --save
    if '--save' in (sys.argv[sys.argv.index('--') + 1:] if '--' in sys.argv else []):
        import bpy
        bpy.ops.wm.save_mainfile(compress=True)
