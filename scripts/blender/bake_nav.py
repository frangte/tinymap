"""Bake the game's nav maps from the Blender diorama.

Run after changing the map in sg.tinymap.blend (and re-exporting the GLB):

    blender sg.tinymap.blend --background --python scripts/blender/bake_nav.py -- public/models/saigon_q1

Writes <out>_nav.png, <out>_roof.png and <out>_nav.json on a 0.35 m grid over the island
(Blender XY metres, row 0 = south edge; three.js x = X, z = -Y):

- nav  (RGBA): R*256+G = floor height in cm + 1000, B = 255 where the player can walk.
  The floor is the lowest upward face with 2 m of headroom (tree crowns ignored); a cell is walkable
  if it is not water, not under a building, has nothing at body height, and can be reached from the
  spawn without a step over 0.5 m.
- roof (RGB): R*256+G = top of the building in cm on non-walkable cells (0 = open). The camera uses it
  to stay out of buildings.
"""
import json
import math
import struct
import sys
import time
import zlib

import bpy
import numpy as np
from mathutils import Vector

CS = 0.35
X0, Y0, X1, Y1 = -72.0, -142.0, 164.0, 52.0
SPAWN = (8.0, -12.8)  # in front of the Ben Thanh south gate
SCENE = 'SaigonDiorama'
COLLECTIONS = ('SD_World', 'SD_NoInk')
WATER = {'SD_water', 'SD_waterDeep', 'SD_foam'}
FOLIAGE = {'SD_leaf', 'SD_leafDark', 'SD_leafLight'}
# ground and water never count as obstacles
NOT_OBSTACLE = {'IS_Base', 'IX_Roads_v2', 'IX_RoadMarks_v2', 'IX_River', 'IX_River.001', 'LM_HoConRua_Water', 'CX_LamSon_Water'}
# objects whose tops occlude the camera, and the subset that are solid buildings (no walking underneath)
ROOF_PREFIX = ('BT_', 'LM_', 'SH_0', 'CX_Towers', 'CX_LamSon', 'LF_Roundabout')
SOLID_PREFIX = ('BT_Halls', 'BT_SideGates', 'BT_SouthGate', 'LM_NhaThoDucBa', 'LM_BuuDien', 'LM_NhaHatLon', 'LM_Bitexco', 'CX_Towers', 'SH_0')
MAX_STEP = 0.5


def log(*a):
    print('[bake_nav]', *a, flush=True)


def out_base():
    argv = sys.argv[sys.argv.index('--') + 1:] if '--' in sys.argv else []
    return argv[0] if argv else bpy.path.abspath('//public/models/saigon_q1')


def write_png(path, pixels):
    """Minimal 8-bit PNG writer (no colour profile, so the numbers stay raw). pixels: (h, w, c) uint8."""
    h, w, c = pixels.shape
    raw = b''.join(b'\x00' + pixels[y].tobytes() for y in range(h))
    def chunk(tag, data):
        return struct.pack('>I', len(data)) + tag + data + struct.pack('>I', zlib.crc32(tag + data) & 0xffffffff)
    ctype = {3: 2, 4: 6}[c]
    png = b'\x89PNG\r\n\x1a\n' + chunk(b'IHDR', struct.pack('>IIBBBBB', w, h, 8, ctype, 0, 0, 0))
    png += chunk(b'IDAT', zlib.compress(raw, 9)) + chunk(b'IEND', b'')
    with open(path, 'wb') as f:
        f.write(png)


def shift(a, dj, di, fill):
    out = np.full_like(a, fill)
    ys = slice(max(dj, 0), a.shape[0] + min(dj, 0))
    yd = slice(max(-dj, 0), a.shape[0] + min(-dj, 0))
    xs = slice(max(di, 0), a.shape[1] + min(di, 0))
    xd = slice(max(-di, 0), a.shape[1] + min(-di, 0))
    out[ys, xs] = a[yd, xd]
    return out


def dilate(m, n):
    for _ in range(n):
        m = m | shift(m, 1, 0, False) | shift(m, -1, 0, False) | shift(m, 0, 1, False) | shift(m, 0, -1, False)
    return m


def erode(m, n):
    for _ in range(n):
        m = m & shift(m, 1, 0, True) & shift(m, -1, 0, True) & shift(m, 0, 1, True) & shift(m, 0, -1, True)
    return m


def sample_triangles(o, dg, cap=120):
    """Points spread over every triangle of `o` at under the cell size, in world space."""
    ev = o.evaluated_get(dg)
    me = ev.to_mesh()
    me.calc_loop_triangles()
    nt = len(me.loop_triangles)
    if nt == 0:
        ev.to_mesh_clear()
        return
    V = np.empty(len(me.vertices) * 3, np.float32)
    me.vertices.foreach_get('co', V)
    V = V.reshape(-1, 3)
    M = np.array(o.matrix_world, dtype=np.float32)
    V = V @ M[:3, :3].T + M[:3, 3]
    T = np.empty(nt * 3, np.int32)
    me.loop_triangles.foreach_get('vertices', T)
    T = T.reshape(-1, 3)
    ev.to_mesh_clear()
    A, B, C = V[T[:, 0]], V[T[:, 1]], V[T[:, 2]]
    L = np.maximum(np.maximum(np.linalg.norm(B - A, axis=1), np.linalg.norm(C - B, axis=1)), np.linalg.norm(A - C, axis=1))
    n = np.clip(np.ceil(L / (CS * 0.6)).astype(int), 1, cap)
    for nv in np.unique(n):
        sel = np.where(n == nv)[0]
        u, v = [], []
        for a in range(nv + 1):
            for b in range(nv + 1 - a):
                u.append(a / nv)
                v.append(b / nv)
        u = np.array(u, np.float32)
        v = np.array(v, np.float32)
        w = 1 - u - v
        yield (A[sel, None, :] * w[None, :, None] + B[sel, None, :] * u[None, :, None] + C[sel, None, :] * v[None, :, None]).reshape(-1, 3)


def main():
    t0 = time.time()
    base = out_base()
    sc = bpy.data.scenes[SCENE]
    vl = sc.view_layers[0]
    if bpy.context.window:  # None when Blender runs with --background
        bpy.context.window.scene = sc
    objs = [o for c in COLLECTIONS for o in bpy.data.collections[c].all_objects if o.type == 'MESH']
    for o in sc.objects:
        if o.type == 'MESH' and o not in objs:
            o.hide_set(True, view_layer=vl)
    dg = vl.depsgraph
    dg.update()
    NX, NY = int((X1 - X0) / CS), int((Y1 - Y0) / CS)

    # 1. floor: multi-hit rays from above, skipping foliage
    H = np.full((NY, NX), np.nan, np.float32)
    WM = np.zeros((NY, NX), bool)
    mats = {}
    down = Vector((0, 0, -1))
    for j in range(NY):
        y = Y0 + (j + 0.5) * CS
        for i in range(NX):
            x = X0 + (i + 0.5) * CS
            z = 80.0
            hits = []
            for _ in range(18):
                ok, loc, nrm, idx, ob, _m = sc.ray_cast(dg, Vector((x, y, z)), down, distance=200.0)
                if not ok:
                    break
                m = mats.get((ob.name, idx))
                if m is None:
                    poly_ok = ob.material_slots and idx < len(ob.data.polygons)
                    m = ob.material_slots[ob.data.polygons[idx].material_index].material.name if poly_ok else ''
                    mats[(ob.name, idx)] = m
                z = loc.z - 0.002
                if m in FOLIAGE:
                    continue
                if loc.z < -1.0:
                    break
                hits.append((loc.z, nrm.z, m))
            for k in range(len(hits) - 1, -1, -1):
                hz, nz, m = hits[k]
                if nz <= 0.6 or hz < -0.9:
                    continue
                head = (hits[k - 1][0] - hz) if k > 0 else 99.0
                if head >= 2.0:
                    H[j, i] = hz
                    WM[j, i] = m in WATER
                    break
        if j % 100 == 0:
            log('rays row', j, '/', NY, round(time.time() - t0), 's')
    Hf = np.nan_to_num(H, nan=-50.0)

    # 2. obstacles at body height, building tops
    block = np.zeros((NY, NX), bool)
    roof = np.full((NY, NX), -50.0, np.float32)
    solid = np.full((NY, NX), -50.0, np.float32)
    for o in objs:
        is_obstacle = o.name not in NOT_OBSTACLE
        is_roof = o.name.startswith(ROOF_PREFIX) and o.name not in NOT_OBSTACLE
        is_solid = o.name.startswith(SOLID_PREFIX)
        if not (is_obstacle or is_roof):
            continue
        for P in sample_triangles(o, dg, 160 if is_roof else 120):
            ii = ((P[:, 0] - X0) / CS).astype(int)
            jj = ((P[:, 1] - Y0) / CS).astype(int)
            ok = (ii >= 0) & (ii < NX) & (jj >= 0) & (jj < NY)
            ii, jj, pz = ii[ok], jj[ok], P[ok, 2]
            if is_obstacle:
                g = Hf[jj, ii]
                hit = (pz > g + 0.35) & (pz < g + 1.8)
                block[jj[hit], ii[hit]] = True
            if is_roof:
                np.maximum.at(roof, (jj, ii), pz)
            if is_solid:
                np.maximum.at(solid, (jj, ii), pz)
    log('obstacles', round(time.time() - t0), 's')

    # 3. walkable = floor, not water, not under a building, nothing at body height, reachable from the spawn
    walk = (~np.isnan(H)) & (~WM) & (H > -0.4) & (~block)
    covered = solid > Hf + 2.0
    covered |= erode(dilate(covered, 2), 2)
    w2 = walk & ~covered
    si, sj = int((SPAWN[0] - X0) / CS), int((SPAWN[1] - Y0) / CS)
    if not w2[sj, si]:
        raise RuntimeError('spawn is not walkable; move SPAWN')
    reach = np.zeros_like(w2)
    reach[sj, si] = True
    Hs = np.where(w2, H, np.nan)
    steps = []
    for dj, di in ((1, 0), (-1, 0), (0, 1), (0, -1)):
        nb = shift(Hs, dj, di, np.nan)
        steps.append((dj, di, np.nan_to_num(np.abs(Hs - nb), nan=9.0) <= MAX_STEP))
    while True:
        before = int(reach.sum())
        for _ in range(25):
            for dj, di, ok in steps:
                reach |= shift(reach, dj, di, False) & ok & w2
        if int(reach.sum()) == before:
            break
    occ = (~reach) & (roof > Hf + 2.0)
    log('walkable', int(reach.sum()), 'occluders', int(occ.sum()))

    # 4. write
    hc = np.clip(np.nan_to_num(H, nan=-10.0) * 100 + 1000, 0, 65535).astype(np.uint16)
    nav = np.zeros((NY, NX, 4), np.uint8)
    nav[..., 0] = hc >> 8
    nav[..., 1] = hc & 255
    nav[..., 2] = np.where(reach, 255, 0)
    nav[..., 3] = 255
    write_png(base + '_nav.png', nav)
    rc = np.where(occ, np.clip(roof * 100, 1, 65535), 0).astype(np.uint16)
    top = np.zeros((NY, NX, 3), np.uint8)
    top[..., 0] = rc >> 8
    top[..., 1] = rc & 255
    top[..., 2] = np.where(occ, 255, 0)
    write_png(base + '_roof.png', top)
    with open(base + '_nav.json', 'w') as f:
        json.dump({
            'x0': X0, 'y0': Y0, 'cell': CS, 'nx': NX, 'ny': NY, 'spawn': list(SPAWN),
            'nav': 'saigon_q1_nav.png: R*256+G = floor height cm + 1000, B=255 walkable',
            'roof': 'saigon_q1_roof.png: R*256+G = building top cm (0 = open), camera occluder cells only',
            'axes': 'row 0 = y0 (south); Blender XY metres; three.js x = X, z = -Y',
        }, f, indent=1)
    log('wrote', base + '_nav.png / _roof.png / _nav.json in', round(time.time() - t0), 's')


main()
