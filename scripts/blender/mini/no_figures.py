"""No people or parked vehicles on the map (layout v7, Khoa 2026-11-12): pedestrians, riders, parked scooters and cars
are NPCs in the game, added by code. The map's own builders no longer place any (build_saigon_mini.py build_props, the
Bùi Viện block); this takes out the ones that come baked into the landmark models appended from models/sg.tinymap.blend
(the people sitting round Hồ Con Rùa). Anchored boats and beach umbrellas stay: they are scenery.

Runs after the other part scripts (MINI_SCRIPTS). kit.strip_figures finds a person by a small skin-coloured head and the
parts round it, and a scooter by a pair of wheels.
"""
import os
import sys

import bpy

HERE = os.path.dirname(os.path.abspath(__file__))
if HERE not in sys.path:
    sys.path.insert(0, HERE)
import kit  # noqa: E402
import layout as LY  # noqa: E402


def run():
    names = [n for lm in LY.L['landmarks'] for n in lm.get('models', [])]
    out = {}
    for n in names:
        ob = bpy.data.objects.get(n)
        if ob is None or ob.type != 'MESH':
            continue
        if not any(m is not None and m.name.split('.')[0] in ('SD_skin', 'SD_hair') for m in ob.data.materials):
            continue
        r = kit.strip_figures(n)
        if r.get('parts_removed'):
            out[n] = r
    return out


if __name__ == '__main__':
    print('NO_FIGURES', run())
