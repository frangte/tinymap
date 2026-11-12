"""Triangle budget of Sài Gòn mini (docs/map-v5-tasks.md, T1): what the game would draw, per category and per object.

    /Applications/Blender.app/Contents/MacOS/Blender models/saigon_mini.blend --background \\
        --python scripts/blender/mini/budget.py [-- --strict]

Counts the triangles of everything export_glb.py exports (collections SD_World and SD_NoInk, modifiers applied,
PREVIEW_* and NAV_* left out), sorted into the budget categories. With --strict it exits with status 1 when a
category or the total is over budget, so it can gate an export. The .blend is not modified.
"""
import sys
from collections import Counter

import bpy

COLLECTIONS = ('SD_World', 'SD_NoInk')
SKIP_PREFIX = ('PREVIEW_', 'NAV_')
# category: (budget in triangles, test on the object name), checked in this order
CATEGORIES = [
    ('houses', 180_000, lambda n: n.startswith('SH_')),
    ('trees', 60_000, lambda n: n.startswith('CX_') and 'Tree' in n),
    ('props', 40_000, lambda n: n.startswith('CX_')),
    ('ground', 70_000, lambda n: n.startswith(('IS_', 'IX_'))),
    ('landmarks', 150_000, lambda n: n.startswith(('LM_', 'BT_', 'LF_'))),
    ('other', 0, lambda n: True),
]
TOTAL_BUDGET = 500_000
MATERIAL_BUDGET = 100


def category(name):
    return next(c for c, _, test in CATEGORIES if test(name))


def main():
    strict = '--strict' in (sys.argv[sys.argv.index('--') + 1:] if '--' in sys.argv else [])
    dg = bpy.context.evaluated_depsgraph_get()
    seen, rows, mats = set(), [], set()
    for cname in COLLECTIONS:
        coll = bpy.data.collections.get(cname)
        if coll is None:
            continue
        for o in coll.all_objects:
            if o.type != 'MESH' or o.name in seen or o.name.startswith(SKIP_PREFIX):
                continue
            seen.add(o.name)
            oe = o.evaluated_get(dg)
            me = oe.to_mesh()
            me.calc_loop_triangles()
            rows.append((len(me.loop_triangles), o.name, category(o.name)))
            mats.update(m.name for m in o.data.materials if m)
            oe.to_mesh_clear()
    per = Counter()
    for n, _, c in rows:
        per[c] += n
    total = sum(per.values())
    over = []
    print('\n[budget] category      triangles     budget')
    for c, budget, _ in CATEGORIES:
        if c == 'other' and not per[c]:
            continue
        flag = '  OVER' if per[c] > budget else ''
        if flag:
            over.append(c)
        print(f'[budget] {c:12s} {per[c]:10,d} {budget:10,d}{flag}')
    flag = '  OVER' if total > TOTAL_BUDGET else ''
    if flag:
        over.append('total')
    print(f'[budget] {"total":12s} {total:10,d} {TOTAL_BUDGET:10,d}{flag}')
    flag = '  OVER' if len(mats) > MATERIAL_BUDGET else ''
    if flag:
        over.append('materials')
    print(f'[budget] {"materials":12s} {len(mats):10,d} {MATERIAL_BUDGET:10,d}{flag}')
    print('\n[budget] biggest objects')
    for n, name, c in sorted(rows, reverse=True)[:15]:
        print(f'[budget] {n:10,d}  {name}  ({c})')
    if strict and over:
        print('[budget] over budget:', ', '.join(over))
        sys.exit(1)


main()
