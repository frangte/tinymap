"""The Sài Gòn mini plan (scripts/map/saigon_mini_layout.json) as shapely shapes in Blender XY, for the part scripts in
scripts/blender/mini/. Same reading as build_saigon_mini.py (plan x east, y south; Blender XY = (x - 251, 231 - y)),
without building anything, so a part script can find its roads, coast and outlines on its own.

Needs shapely (Blender's Python: pip install --target .blender-site shapely, see docs/map-v5.md).
"""
import json
import math
import os
import re
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.abspath(os.path.join(HERE, '..', '..', '..'))
_site = os.path.join(ROOT, '.blender-site')
if os.path.isdir(_site) and _site not in sys.path:
    sys.path.insert(0, _site)

from shapely.geometry import LineString, Point, Polygon  # noqa: E402
from shapely.ops import unary_union  # noqa: E402

# SAIGON_LAYOUT=<file.json> reads another layout (a proposal), as build_saigon_mini.py does
LAYOUT_FILE = os.path.abspath(os.environ.get('SAIGON_LAYOUT') or os.path.join(ROOT, 'scripts', 'map', 'saigon_mini_layout.json'))
L = json.load(open(LAYOUT_FILE, encoding='utf-8'))
TX, TY = L['toBlender']['x'], L['toBlender']['y']
SEA, GROUND_Z, ROAD_Z, WALK_Z = -0.45, 0.0, 0.07, 0.18


def tb(p):
    """plan point -> Blender XY"""
    return (p[0] - TX, TY - p[1])


def _bez(ctrl, step):
    n = max(2, int(math.ceil(sum(math.dist(ctrl[i], ctrl[i + 1]) for i in range(len(ctrl) - 1)) / step)))
    out = []
    for k in range(n + 1):
        t = k / n
        pts = list(ctrl)
        while len(pts) > 1:
            pts = [((1 - t) * a[0] + t * b[0], (1 - t) * a[1] + t * b[1]) for a, b in zip(pts, pts[1:])]
        out.append(pts[0])
    return out


def path_pts(d, step=1.0):
    """sample an SVG path (absolute M L Q C S Z, one subpath) into Blender XY points"""
    toks = re.findall(r'[MLQCSZ]|-?\d+(?:\.\d+)?', d)
    pts, i, cmd, cur, c2prev = [], 0, None, None, None

    def nums(n):
        nonlocal i
        v = [float(t) for t in toks[i:i + n]]
        i += n
        return [(v[k], v[k + 1]) for k in range(0, n, 2)]

    while i < len(toks):
        if toks[i] in 'MLQCSZ':
            cmd = toks[i]
            i += 1
            if cmd == 'Z':
                if pts and pts[0] != pts[-1]:
                    pts.append(pts[0])
                continue
        if cmd == 'M':
            (p,) = nums(2)
            pts.append(p)
            cur, c2prev, cmd = p, None, 'L'
        elif cmd == 'L':
            (p,) = nums(2)
            pts += _bez([cur, p], step)[1:]
            cur, c2prev = p, None
        elif cmd == 'Q':
            c, p = nums(4)
            pts += _bez([cur, c, p], step)[1:]
            cur, c2prev = p, None
        elif cmd == 'C':
            c1, c2, p = nums(6)
            pts += _bez([cur, c1, c2, p], step)[1:]
            cur, c2prev = p, c2
        elif cmd == 'S':
            c2, p = nums(4)
            c1 = (2 * cur[0] - c2prev[0], 2 * cur[1] - c2prev[1]) if c2prev else cur
            pts += _bez([cur, c1, c2, p], step)[1:]
            cur, c2prev = p, c2
    return [tb(p) for p in pts]


coast = Polygon(path_pts(L['coast'], 1.0)).buffer(0)
road_lines = {r.get('name', 'road_%d' % k): (LineString(path_pts(r['path'], 0.8)), r['w']) for k, r in enumerate(L['roads'])}
roads = unary_union([ln.buffer(w / 2, cap_style='flat', join_style='round') for ln, w in road_lines.values()])
mountains = {m['id']: (m, Polygon(path_pts(m['outline'], 1.0)).buffer(0)) for m in L['mountains']}
beaches = {b['id']: LineString(path_pts(b['path'], 0.8)).buffer(b['width'] / 2, cap_style='flat') for b in L['beaches']}
landmarks = {lm['id']: lm for lm in L['landmarks']}
