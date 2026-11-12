"""Small helpers for the plan scripts in scripts/map/ (relayout_v8.py and later): SVG path strings in plan
coordinates (x east, y south, metres) made from points, sampled back into points, and moved.
"""
import math
import re


def catmull_rom_path(pts, closed=True, tension=0.5):
    """an SVG path of cubic Béziers through every point"""
    n = len(pts)
    seg = []
    for i in (range(n) if closed else range(n - 1)):
        p0, p1, p2, p3 = pts[(i - 1) % n], pts[i], pts[(i + 1) % n], pts[(i + 2) % n]
        if not closed:
            p0, p3 = pts[max(i - 1, 0)], pts[min(i + 2, n - 1)]
        c1 = (p1[0] + (p2[0] - p0[0]) * tension / 3, p1[1] + (p2[1] - p0[1]) * tension / 3)
        c2 = (p2[0] - (p3[0] - p1[0]) * tension / 3, p2[1] - (p3[1] - p1[1]) * tension / 3)
        seg.append('C %.1f,%.1f %.1f,%.1f %.1f,%.1f' % (*c1, *c2, *p2))
    return 'M %.1f,%.1f ' % tuple(pts[0]) + ' '.join(seg) + (' Z' if closed else '')


def sample(d, step=1.0):
    """plan points of a path of absolute M L Q C S Z commands (one subpath)"""
    toks = re.findall(r'[MLQCSZ]|-?\d+(?:\.\d+)?', d)
    out, i, cur, cmd, c2prev = [], 0, None, None, None

    def bez(ctrl):
        n = max(2, int(sum(math.dist(ctrl[k], ctrl[k + 1]) for k in range(len(ctrl) - 1)) / step))
        res = []
        for k in range(1, n + 1):
            t = k / n
            q = list(ctrl)
            while len(q) > 1:
                q = [((1 - t) * a[0] + t * b[0], (1 - t) * a[1] + t * b[1]) for a, b in zip(q, q[1:])]
            res.append(q[0])
        return res

    def num(k):
        return [(float(toks[i + 2 * j]), float(toks[i + 2 * j + 1])) for j in range(k)]
    while i < len(toks):
        if toks[i] in 'MLQCSZ':
            cmd = toks[i]
            i += 1
            if cmd == 'Z':
                continue
        if cmd == 'M':
            (cur,) = num(1)
            out.append(cur)
            i += 2
            cmd, c2prev = 'L', None
        elif cmd == 'L':
            (p,) = num(1)
            out += bez([cur, p])
            cur, c2prev, i = p, None, i + 2
        elif cmd == 'Q':
            c, p = num(2)
            out += bez([cur, c, p])
            cur, c2prev, i = p, None, i + 4
        elif cmd == 'C':
            c1, c2, p = num(3)
            out += bez([cur, c1, c2, p])
            cur, c2prev, i = p, c2, i + 6
        elif cmd == 'S':
            c2, p = num(2)
            c1 = (2 * cur[0] - c2prev[0], 2 * cur[1] - c2prev[1]) if c2prev else cur
            out += bez([cur, c1, c2, p])
            cur, c2prev, i = p, c2, i + 4
    return out


def poly_path(coords, closed=True, nd=1):
    pts = list(coords)
    if closed and len(pts) > 1 and tuple(pts[0]) == tuple(pts[-1]):
        pts = pts[:-1]
    fmt = '%.' + str(nd) + 'f,%.' + str(nd) + 'f'
    return 'M ' + ' L '.join(fmt % tuple(p) for p in pts) + (' Z' if closed else '')


def move_path(d, dx, dy):
    """translate every coordinate pair of a path"""
    toks = re.findall(r'[MLQCSZ]|-?\d+(?:\.\d+)?', d)
    out, nums = [], []
    for t in toks:
        if t in 'MLQCSZ':
            out.append(t)
        else:
            nums.append(float(t))
            if len(nums) == 2:
                out.append('%g,%g' % (nums[0] + dx, nums[1] + dy))
                nums = []
    return ' '.join(out)
