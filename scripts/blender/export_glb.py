"""Export the diorama to a raw GLB for the game (the .blend is not modified or saved).

    blender sg.tinymap.blend --background --python scripts/blender/export_glb.py -- export/saigon_q1_raw.glb

Each SD_* material is swapped for a plain Principled BSDF in its real colour (otherwise the exporter writes
identical default materials and `gltf-transform join` merges everything into one). The game rebuilds the
paint shader from the material name.

It also exports a file made by importing the shipped GLB back into Blender (new.sg.tinymap.blend: one scene,
one collection, meshes merged by material, Principled materials already in their colours): without the
SaigonDiorama scene it takes every visible mesh of the active scene. Then shrink it (see docs/PROJECT.md,
"Pipeline"):

    npx @gltf-transform/cli@4.1.1 flatten  export/saigon_q1_raw.glb /tmp/a.glb
    npx @gltf-transform/cli@4.1.1 join     /tmp/a.glb /tmp/b.glb
    npx @gltf-transform/cli@4.1.1 weld     /tmp/b.glb /tmp/c.glb
    npx @gltf-transform/cli@4.1.1 quantize /tmp/c.glb /tmp/d.glb --pattern POSITION
    npx @gltf-transform/cli@4.1.1 prune    /tmp/d.glb /tmp/e.glb
    npx @gltf-transform/cli@4.1.1 meshopt  /tmp/e.glb export/saigon_q1.glb

The last step compresses the geometry with EXT_meshopt_compression (about 5 times smaller: the Sài Gòn mini blockout
goes from 19 MB to 3.7 MB); the game's loader decodes it (src/game/world.ts, MeshoptDecoder).

Objects named PREVIEW_* (the sea plane that makes the .blend look right in Blender) are not exported: the game draws
the sea and the river water itself (src/game/sea.ts).
"""
import os
import sys

import bpy

SCENE = 'SaigonDiorama'
COLLECTIONS = ('SD_World', 'SD_NoInk')
SKIP_PREFIX = ('PREVIEW_', 'NAV_')   # Blender-only preview sea; walk-map boxes (nav_boxes.py)


def lin2srgb(c):
    return 12.92 * c if c <= 0.0031308 else 1.055 * c ** (1 / 2.4) - 0.055


def srgb2lin(c):
    return c / 12.92 if c <= 0.04045 else ((c + 0.055) / 1.055) ** 2.4


def material_colour(m):
    """sRGB 0..1 colour of an SD_* material: the SD_Paint group input, an emission colour, an imported
    Principled BSDF's base colour (and whether it glows), or the viewport colour."""
    if m.node_tree:
        for n in m.node_tree.nodes:
            if n.type == 'GROUP' and n.node_tree and n.node_tree.name.startswith('SD_Paint'):
                return [lin2srgb(x) for x in n.inputs['Color'].default_value[:3]], False
        for n in m.node_tree.nodes:
            if n.type == 'EMISSION':
                return [lin2srgb(x) for x in n.inputs['Color'].default_value[:3]], True
        for n in m.node_tree.nodes:
            if n.type == 'BSDF_PRINCIPLED' and not n.inputs['Base Color'].is_linked:
                glow = n.inputs['Emission Strength'].default_value > 0 and any(c > 0 for c in n.inputs['Emission Color'].default_value[:3])
                return [lin2srgb(x) for x in n.inputs['Base Color'].default_value[:3]], glow
    return [lin2srgb(x) for x in m.diffuse_color[:3]], False


def main():
    argv = sys.argv[sys.argv.index('--') + 1:] if '--' in sys.argv else []
    out = os.path.abspath(argv[0] if argv else bpy.path.abspath('//export/saigon_q1_raw.glb'))
    sc = bpy.data.scenes.get(SCENE)
    if sc:
        objs = [o for c in COLLECTIONS for o in bpy.data.collections[c].all_objects if o.type == 'MESH' and not o.name.startswith(SKIP_PREFIX)]
    else:
        # a re-imported GLB: everything visible in the active scene, except the walk-map boxes (nav_boxes.py)
        sc = bpy.context.scene
        objs = [o for o in sc.objects if o.type == 'MESH' and not o.hide_render and not o.hide_get() and not o.name.startswith(SKIP_PREFIX)]
    vl = sc.view_layers[0]
    for o in sc.objects:
        o.select_set(False, view_layer=vl)
    for o in objs:
        o.select_set(True, view_layer=vl)
    done = set()
    for o in objs:
        for m in o.data.materials:
            if not m or m.name in done:
                continue
            done.add(m.name)
            srgb, emit = material_colour(m)
            rgb = [srgb2lin(max(0.0, min(1.0, x))) for x in srgb]
            nt = m.node_tree
            for n in list(nt.nodes):
                nt.nodes.remove(n)
            outn = nt.nodes.new('ShaderNodeOutputMaterial')
            bs = nt.nodes.new('ShaderNodeBsdfPrincipled')
            bs.inputs['Base Color'].default_value = (*rgb, 1)
            bs.inputs['Roughness'].default_value = 1.0
            bs.inputs['Metallic'].default_value = 0.0
            if emit or m.name == 'SD_lampGlow':
                bs.inputs['Emission Color'].default_value = (*rgb, 1)
                bs.inputs['Emission Strength'].default_value = 1.0
            nt.links.new(bs.outputs[0], outn.inputs[0])
    kw = dict(filepath=out, export_format='GLB', use_selection=True, export_apply=True, export_materials='EXPORT',
              export_cameras=False, export_lights=False, export_yup=True, export_texcoords=False, export_normals=False)
    props = bpy.ops.export_scene.gltf.get_rna_type().properties.keys()
    for k, v in (('use_active_scene', True), ('export_vertex_color', 'NONE'), ('export_animations', False),
                 ('export_skins', False), ('export_morph', False), ('export_extras', True)):
        if k in props:
            kw[k] = v
    with bpy.context.temp_override(scene=sc, view_layer=vl):
        bpy.ops.export_scene.gltf(**kw)
    print('[export_glb] wrote', out, round(os.path.getsize(out) / 1e6, 2), 'MB,', len(objs), 'objects,', len(done), 'materials', flush=True)


main()
