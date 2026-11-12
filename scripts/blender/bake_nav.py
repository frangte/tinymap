"""Bake the game's nav maps from the Blender map.

Run after changing the map (and re-exporting the GLB), then copy the maps to the server (npm run nav:sync):

    blender models/saigon_mini.blend --background --python scripts/blender/bake_nav.py -- public/models/saigon_mini
    blender models/temp.sg.tinymap.blend --background --python scripts/blender/bake_nav.py -- public/models/saigon_q1

Sài Gòn mini (models/saigon_mini.blend, built by build_saigon_mini.py) is found by its SM_Houses collection and baked
with its own grid, spawn and object rules (MINI below); the older maps keep theirs.

Two kinds of map file:

- A GLB import (models/temp.sg.tinymap.blend: one scene, meshes merged by material, no original names). Ground is
  told apart by material (GROUND: grass, sand, asphalt, kerbs, road lines, rock, water); everything else is an
  obstacle where it stands at body height. The buildings nobody walks into are boxes in the NAV collection
  (scripts/blender/nav_boxes.py), which also holds hand-placed Block and Open boxes.
- A file with the original objects (scene SaigonDiorama, the old sg.tinymap.blend layouts): objects are told apart
  by name (IS_/IX_ ground, BT_/LM_/SH_0… buildings).

Writes <out>_nav.png, <out>_roof.png and <out>_nav.json on a 0.35 m grid over the island
(Blender XY metres, row 0 = south edge; three.js x = X, z = -Y):

- nav  (RGBA): R*256+G = floor height in cm + 1000, B = 255 where the player can walk.
  The floor is the lowest upward face with 2 m of headroom (tree crowns ignored; on Sài Gòn mini nothing under a
  bridge deck or the hill counts, see MINI['cover']); a cell is walkable
  if it is not water, above sea level, not under a building, has nothing at body height, and can be
  reached from the spawn without a step over 0.5 m.
- roof (RGB): R*256+G = top of the building in cm on non-walkable cells (0 = open). The camera uses it
  to stay out of buildings.
"""
import json
import math
import os
import struct
import sys
import time
import zlib
from collections import deque

import bpy
import numpy as np
from mathutils import Vector

CS = 0.35
X0, Y0, X1, Y1 = -84.0, -98.0, 140.0, 130.0    # the island and its beach
SPAWN = (8.0, -12.8)  # in front of the Ben Thanh south gate
SEA_LEVEL = -0.45     # the sea is drawn in three.js at this height; sand below it is not walkable
ISLAND_CENTER = (36.0, 8.0)
MAX_STEP = 0.5
WATER = {'SD_water', 'SD_waterDeep', 'SD_foam'}
FOLIAGE = {'SD_leaf', 'SD_leafDark', 'SD_leafLight'}

# original objects
SCENE = 'SaigonDiorama'
COLLECTIONS = ('SD_World', 'SD_NoInk')
# ground (IS_ island, IX_ streets) and water never count as obstacles
NOT_OBSTACLE_PREFIX = ('IS_', 'IX_')
NOT_OBSTACLE = {'LM_HoConRua_Water', 'CX_LamSon_Water'}
# objects whose tops occlude the camera, and the subset that are solid buildings (no walking underneath)
ROOF_PREFIX = ('BT_', 'LM_', 'SH_0', 'CX_Towers', 'CX_LamSon', 'LF_Roundabout')
SOLID_PREFIX = ('BT_Halls', 'BT_SideGates', 'BT_SouthGate', 'LM_NhaThoDucBa', 'LM_BuuDien', 'LM_NhaHatLon', 'LM_Bitexco', 'CX_Towers', 'SH_0')

# Sài Gòn mini (layout v8): a bigger island, objects named by build_saigon_mini.py and the part scripts
MINI = dict(
    bounds=(-246.0, -230.0, 246.0, 228.0),
    spawn=(-25.0, 25.0),                     # on the square in front of Chợ Bến Thành (plan 226, 206)
    island_center=(0.0, 0.0),
    # ground and walkable surfaces: the island, streets, forecourts, the hill's stairs and terraces, water, the sea
    not_obstacle_prefix=('IS_Ground', 'IS_nui_nho', 'IX_', 'LF_', 'PREVIEW_'),
    not_obstacle={'LM_HoConRua_Water', 'CX_Lakes', 'SH_BuiVien_Lights'},
    # camera occluders, and the buildings nobody walks under (houses are closed boxes: their walls keep the player out)
    roof_prefix=('BT_', 'LM_', 'SH_'),
    roof_skip={'SH_BuiVien_Lights', 'LM_Bridges', 'LM_CauBaSon', 'LM_HoConRua_Water'},
    # covers: nothing under them can be the floor, only what is on or above their lowest face. The bridge decks
    # (the player crosses them; nobody walks under a bridge in this 2.5D map) and the hill, an open shell over the
    # flat ground. The water under a bridge never counts, and the hill's bushes (leaf materials) are ground, not
    # tree crowns.
    cover={'LM_Bridges', 'LM_CauBaSon', 'IS_nui_nho'},
    # closed parts built inside out are turned round on these: the covers, and the streets, paths and stairs on them
    # (the hill's road and paths are slabs that straddle its ground; their tops face down)
    turn_prefix=('LM_Bridges', 'LM_CauBaSon', 'IS_nui_nho', 'IX_', 'LF_'),
    solid_prefix=('BT_Halls', 'BT_SideGates', 'BT_SouthGate', 'LM_NhaThoDucBa', 'LM_BuuDien', 'LM_NhaHatLon', 'LM_Bitexco',
                  'LM_DinhThongNhat', 'LM_NewBuildings', 'LM_Blockout', 'LM_HaiDang', 'LM_TuongChua', 'LM_MeLinh_Pier'),
)

# GLB imports
GROUND = {'SD_grass', 'SD_grassDark', 'SD_sand', 'SD_earthDark', 'SD_rock', 'SD_rockDark', 'SD_asphalt', 'SD_curb', 'SD_roadLine'} | WATER
NAV_PREFIX = 'NAV_'
# camera occluders smaller than this many cells (lamp posts, trunks, statues) are dropped: the camera would jump
MIN_OCCLUDER = 12


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


def drop_small(mask, min_cells):
    """`mask` without its 4-connected patches of fewer than `min_cells` cells."""
    seen = np.zeros_like(mask)
    out = mask.copy()
    ny, nx = mask.shape
    for j, i in zip(*np.nonzero(mask)):
        if seen[j, i]:
            continue
        seen[j, i] = True
        queue, cells = deque([(j, i)]), []
        while queue:
            cj, ci = queue.popleft()
            cells.append((cj, ci))
            for dj, di in ((1, 0), (-1, 0), (0, 1), (0, -1)):
                nj, ni = cj + dj, ci + di
                if 0 <= nj < ny and 0 <= ni < nx and mask[nj, ni] and not seen[nj, ni]:
                    seen[nj, ni] = True
                    queue.append((nj, ni))
        if len(cells) < min_cells:
            for c in cells:
                out[c] = False
    return out


def sample_triangles(o, dg, cap=160):
    """(points, material name) for each material of `o`: points spread over its triangles at under the cell size,
    in world space."""
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
    MI = np.empty(nt, np.int32)
    me.loop_triangles.foreach_get('material_index', MI)
    ev.to_mesh_clear()
    names = [s.material.name if s.material else '' for s in o.material_slots]
    for mi in np.unique(MI):
        mat = names[mi] if mi < len(names) else ''
        tri = T[MI == mi]
        A, B, C = V[tri[:, 0]], V[tri[:, 1]], V[tri[:, 2]]
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
            yield (A[sel, None, :] * w[None, :, None] + B[sel, None, :] * u[None, :, None] + C[sel, None, :] * v[None, :, None]).reshape(-1, 3), mat


def inside_out(o, dg):
    """For each polygon of `o` (evaluated): True where it belongs to a closed loose part built inside out (negative
    volume). Open parts (a railing, a deck strip) have no inside: they are left as they are."""
    me = o.evaluated_get(dg).to_mesh()
    me.calc_loop_triangles()
    V = np.empty(len(me.vertices) * 3)
    me.vertices.foreach_get('co', V)
    V = V.reshape(-1, 3)
    nt = len(me.loop_triangles)
    T = np.empty(nt * 3, np.int64)
    me.loop_triangles.foreach_get('vertices', T)
    T = T.reshape(-1, 3)
    TP = np.empty(nt, np.int64)
    me.loop_triangles.foreach_get('polygon_index', TP)
    uses = {}
    for pg in me.polygons:
        for ek in pg.edge_keys:
            uses[ek] = uses.get(ek, 0) + 1
    npoly = len(me.polygons)
    o.evaluated_get(dg).to_mesh_clear()
    par = np.arange(len(V))

    def root(x):
        while par[x] != x:
            par[x] = par[par[x]]
            x = par[x]
        return x
    for a, b, c in T:
        ra = root(a)
        for v in (b, c):
            rv = root(v)
            if rv != ra:
                par[rv] = ra
    part = np.array([root(v) for v in range(len(V))])
    vol = np.einsum('ij,ij->i', V[T[:, 0]], np.cross(V[T[:, 1]], V[T[:, 2]]))
    pv = {}
    for pt, v in zip(part[T[:, 0]], vol):
        pv[pt] = pv.get(pt, 0.0) + v
    open_parts = {part[a] for (a, _b), n in uses.items() if n != 2}
    flip = np.zeros(npoly, bool)
    flip[TP] = [pv[pt] < 0 and pt not in open_parts for pt in part[T[:, 0]]]
    return flip


def mini_source():
    """Sài Gòn mini: the original objects, told apart by the names build_saigon_mini.py gives them"""
    sc = bpy.data.scenes[SCENE]
    vl = sc.view_layers[0]
    objs = [o for c in COLLECTIONS for o in bpy.data.collections[c].all_objects if o.type == 'MESH']

    def kind(o, mat):
        n = o.name
        obstacle = n not in MINI['not_obstacle'] and not n.startswith(MINI['not_obstacle_prefix'])
        roof = n.startswith(MINI['roof_prefix']) and n not in MINI['roof_skip']
        return obstacle, roof, n.startswith(MINI['solid_prefix'])
    return sc, vl, objs, [], kind


def original_source():
    """A file with the original objects: what to bake, and how each object counts (by name)."""
    sc = bpy.data.scenes[SCENE]
    vl = sc.view_layers[0]
    if bpy.context.window:  # None when Blender runs with --background
        bpy.context.window.scene = sc
    objs = [o for c in COLLECTIONS for o in bpy.data.collections[c].all_objects if o.type == 'MESH']
    for o in sc.objects:
        if o.type == 'MESH' and o not in objs:
            o.hide_set(True, view_layer=vl)

    def kind(o, mat):
        obstacle = o.name not in NOT_OBSTACLE and not o.name.startswith(NOT_OBSTACLE_PREFIX)
        roof = o.name.startswith(ROOF_PREFIX) and o.name not in NOT_OBSTACLE
        return obstacle, roof, o.name.startswith(SOLID_PREFIX)
    return sc, vl, objs, [], kind


def merged_source():
    """A GLB import: what to bake, the NAV boxes, and how each material counts. Solid buildings come from the boxes."""
    sc = bpy.context.scene
    vl = bpy.context.view_layer
    boxes = [o for o in sc.objects if o.name.startswith(NAV_PREFIX)]
    for o in boxes:
        o.hide_set(True, view_layer=vl)  # out of the rays
    objs = [o for o in sc.objects if o.type == 'MESH' and o not in boxes and not o.hide_render and not o.hide_get(view_layer=vl)]
    log('GLB import:', len(objs), 'meshes,', len(boxes), 'NAV boxes')

    def kind(o, mat):
        return mat not in GROUND, mat not in GROUND and mat not in FOLIAGE, False
    return sc, vl, objs, boxes, kind


def box_mask(boxes, rule, nx, ny):
    """Cells whose centre is inside one of the NAV_<rule>_ boxes (only the box outline in Blender XY counts)."""
    gx, gy = np.meshgrid(X0 + (np.arange(nx) + 0.5) * CS, Y0 + (np.arange(ny) + 0.5) * CS)
    m = np.zeros((ny, nx), bool)
    for o in boxes:
        if not o.name.startswith(NAV_PREFIX + rule + '_') or o.type != 'MESH':
            continue
        inv = np.array(o.matrix_world.inverted())
        zc = o.matrix_world.translation.z
        lx = inv[0, 0] * gx + inv[0, 1] * gy + inv[0, 2] * zc + inv[0, 3]
        ly = inv[1, 0] * gx + inv[1, 1] * gy + inv[1, 2] * zc + inv[1, 3]
        bx = [c[0] for c in o.bound_box]
        by = [c[1] for c in o.bound_box]
        m |= (lx >= min(bx)) & (lx <= max(bx)) & (ly >= min(by)) & (ly <= max(by))
        log('box', o.name)
    return m


def main():
    global X0, Y0, X1, Y1, SPAWN, ISLAND_CENTER
    t0 = time.time()
    cover = set()
    base = out_base()
    merged = SCENE not in bpy.data.scenes
    mini = not merged and bpy.data.collections.get('SM_Houses') is not None
    if mini:
        X0, Y0, X1, Y1 = MINI['bounds']
        SPAWN, ISLAND_CENTER = MINI['spawn'], MINI['island_center']
        cover = MINI['cover']
        log('Sài Gòn mini')
    sc, vl, objs, boxes, kind = merged_source() if merged else (mini_source() if mini else original_source())
    dg = vl.depsgraph
    dg.update()
    flips = {o.name: inside_out(o, dg) for o in objs if mini and o.name.startswith(MINI['turn_prefix'])}
    for n, f in flips.items():
        if f.any():
            log(n, int(f.sum()), 'of', len(f), 'faces inside out, turned round')
    NX, NY = int((X1 - X0) / CS), int((Y1 - Y0) / CS)

    # 1. floor: multi-hit rays from above, skipping foliage
    H = np.full((NY, NX), np.nan, np.float32)
    G = np.full((NY, NX), np.nan, np.float32)  # the lowest upward face, headroom or not: the ground under a building
    WM = np.zeros((NY, NX), bool)
    OVER = np.zeros((NY, NX), np.int32)  # what hangs over the floor (2 m up or more), for the log
    over_ids = {}
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
                if m in FOLIAGE and ob.name not in cover:
                    continue                # tree crowns (the hill's bushes are painted on its ground: they count)
                if loc.z < -1.0:
                    break
                nz = -nrm.z if ob.name in flips and flips[ob.name][idx] else nrm.z
                hits.append((loc.z, nz, m, ob.name))
            # the lowest face of a cover: nothing under it can be the floor, nor water over it (where its own face
            # there is too steep, the cell has no floor)
            kc = max((k for k, h in enumerate(hits) if h[3] in cover), default=-1)
            under = kc >= 0
            for k in range(kc if under else len(hits) - 1, -1, -1):
                hz, nz, m, name = hits[k]
                if nz <= 0.6 or hz < -0.9 or (under and m in WATER):
                    continue
                if np.isnan(G[j, i]):
                    G[j, i] = hz
                head = (hits[k - 1][0] - hz) if k > 0 else 99.0
                if head >= 2.0:
                    H[j, i] = hz
                    WM[j, i] = m in WATER
                    if k > 0:
                        OVER[j, i] = over_ids.setdefault(hits[k - 1][3], len(over_ids) + 1)
                    break
        if j % 100 == 0:
            log('rays row', j, '/', NY, round(time.time() - t0), 's')
    Hf = np.nan_to_num(H, nan=-50.0)

    # 2. obstacles at body height, building tops
    block = np.zeros((NY, NX), bool)
    roof = np.full((NY, NX), -50.0, np.float32)
    solid = np.full((NY, NX), -50.0, np.float32)
    for o in objs:
        for P, mat in sample_triangles(o, dg):
            is_obstacle, is_roof, is_solid = kind(o, mat)
            if not (is_obstacle or is_roof):
                continue
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
    if merged:
        solid = np.where(box_mask(boxes, 'Solid', NX, NY), roof, -50.0)
        block &= ~box_mask(boxes, 'Open', NX, NY)
        block |= box_mask(boxes, 'Block', NX, NY)
    log('obstacles', round(time.time() - t0), 's')

    # 3. walkable = floor, not water, not under a building, nothing at body height, reachable from the spawn
    walk = (~np.isnan(H)) & (~WM) & (H > SEA_LEVEL + 0.08) & (~block)
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
    if merged:
        # a GLB import can read a building's top as its floor (no 2 m of headroom inside): measure from the ground
        occ = drop_small((~reach) & (roof > np.nan_to_num(G, nan=-50.0) + 2.0), MIN_OCCLUDER)
    log('walkable', int(reach.sum()), 'occluders', int(occ.sum()))
    names = {v: k for k, v in over_ids.items()}
    under = np.bincount(OVER[reach], minlength=len(names) + 1)
    log('walkable cells under something:', ', '.join('%s %d' % (names[k], under[k]) for k in np.argsort(-under) if k and under[k]))

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
            'sea_level': SEA_LEVEL, 'island_center': list(ISLAND_CENTER),
            'nav': os.path.basename(base) + '_nav.png: R*256+G = floor height cm + 1000, B=255 walkable',
            'roof': os.path.basename(base) + '_roof.png: R*256+G = building top cm (0 = open), camera occluder cells only',
            'axes': 'row 0 = y0 (south); Blender XY metres; three.js x = X, z = -Y',
        }, f, indent=1)
    log('wrote', base + '_nav.png / _roof.png / _nav.json in', round(time.time() - t0), 's')


main()
