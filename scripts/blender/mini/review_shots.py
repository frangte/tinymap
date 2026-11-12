"""Quick Workbench review renders of Sài Gòn mini (map v5), named so each task in docs/map-v5-tasks.md can point at them.

    /Applications/Blender.app/Contents/MacOS/Blender models/saigon_mini.blend --background \\
        --python scripts/blender/mini/review_shots.py -- renders/review [shot names...]

No names renders every shot. Flat colours with an outline, cavity and sun shadows: fast (about a minute a shot on a
laptop CPU) and good enough to judge shape and layout. The .blend is not modified. Final art is judged in EEVEE with
the ink (build_saigon_mini.py --render) or in the game.

Each shot looks at a point t (Blender XY, metres) from azimuth az / elevation el (degrees, az 0 = from +x, -90 = from
the south) at distance d; ortho gives an orthographic width instead of a lens.
"""
import math
import os
import sys

import bpy
from mathutils import Vector

SHOTS = {
    'top':          dict(t=(0, -10, 0), az=-90, el=89.9, d=600, ortho=560, res=(1400, 1300)),
    'overview':     dict(t=(0, -10, 0), az=-125, el=38, d=620, lens=34, res=(1400, 860)),
    'cau_ba_son':   dict(t=(62, 165, 12), az=-90, el=10, d=95, lens=40),
    'cau_ba_son_34': dict(t=(64, 165, 10), az=-140, el=24, d=85, lens=35),
    'cau_ham_nghi': dict(t=(60, 25, 4), az=-60, el=22, d=70, lens=35),
    'thu_thiem':    dict(t=(165, 40, 5), az=-150, el=35, d=260, lens=32, res=(1400, 860)),
    'nha_pho':      dict(t=(-60, 190, 4), az=-120, el=28, d=90, lens=32),
    'nha_hem':      dict(t=(-190, 170, 4), az=-60, el=35, d=90, lens=32),
    'bui_vien':     dict(t=(10, -39, 4), az=180, el=12, d=58, lens=30),
    'bui_vien_34':  dict(t=(2, -39, 3), az=-120, el=35, d=95, lens=32),
    'vung_tau':     dict(t=(150, -145, 5), az=-120, el=32, d=240, lens=30, res=(1400, 860)),
    'nui_nho_w':    dict(t=(161, -165, 8), az=-150, el=18, d=115, lens=32),
    'nui_nho_e':    dict(t=(161, -165, 6), az=20, el=22, d=120, lens=32),
    'nui_nho_city': dict(t=(159, -165, 10), az=125, el=16, d=190, lens=30, res=(1400, 800)),
    'tuong_chua':   dict(t=(163, -186, 21), az=-80, el=6, d=24, lens=40, res=(800, 900)),
    'tuong_chua_xa': dict(t=(163, -186, 20), az=-90, el=8, d=40, lens=35, res=(1000, 800)),
    'hai_dang':     dict(t=(159, -151, 19), az=150, el=12, d=32, lens=35, res=(1000, 800)),
    'mui_nghinh_phong': dict(t=(158, -222, 2), az=-60, el=25, d=60, lens=35, res=(1000, 700)),
    'bai_sau':      dict(t=(215, -100, 3), az=10, el=22, d=140, lens=32, res=(1400, 860)),
    'bai_truoc':    dict(t=(92, -152, 3), az=200, el=22, d=110, lens=32, res=(1400, 860)),
    'bo_nam':       dict(t=(-100, -105, 3), az=-80, el=30, d=200, lens=30, res=(1400, 860)),
    'ho_con_rua':   dict(t=(-111, 165, 3), az=-110, el=30, d=48, lens=35),
    # layout v8 (scripts/map/relayout_v8.py)
    'v8_trung_tam': dict(t=(-85, 85, 4), az=-60, el=34, d=230, lens=30, res=(1400, 860)),
    'v8_khuc_song': dict(t=(75, -10, 2), az=-100, el=30, d=200, lens=30, res=(1400, 860)),
    'v8_dinh_30_4': dict(t=(-111, 50, 6), az=90, el=22, d=105, lens=32, res=(1400, 860)),
    'v8_ben_nghe':  dict(t=(0, -40, 2), az=-150, el=26, d=120, lens=32, res=(1400, 860)),
    'v8_bai_truoc': dict(t=(60, -150, 3), az=-125, el=24, d=170, lens=30, res=(1400, 860)),
    'v8_thu_thiem': dict(t=(140, 60, 4), az=-160, el=32, d=230, lens=30, res=(1400, 860)),
    'v8_truc_nguyen_hue': dict(t=(70, 91, 14), az=180, el=13, d=150, lens=32, res=(1400, 800)),
    # the spots of Khoa's review of v8 (road ends, bridge landings, Chợ Lớn)
    'fix_tao_dan':   dict(t=(-141, 70, 0), az=-90, el=62, d=170, lens=32, res=(1100, 750)),
    'fix_cxp':       dict(t=(-111, 127, 0), az=-90, el=70, d=55, lens=35, res=(900, 650)),
    'fix_kenh':      dict(t=(-60, -70, 0), az=-130, el=45, d=85, lens=32, res=(1000, 700)),
    'fix_cau_di_bo': dict(t=(43, 91, 0), az=-150, el=50, d=45, lens=35, res=(900, 650)),
    'fix_tt3':       dict(t=(179, -66, 0), az=-60, el=45, d=65, lens=32, res=(1000, 700)),
    'fix_nui_duong': dict(t=(142, -118, 0), az=-120, el=50, d=50, lens=35, res=(900, 650)),
    'fix_cho_lon':   dict(t=(-178, -50, 0), az=-110, el=50, d=120, lens=32, res=(1100, 750)),
    'fix_ong_lanh':  dict(t=(-5, -40, 0), az=-150, el=42, d=80, lens=32, res=(1000, 700)),
    'thu_thiem_top': dict(t=(150, 50, 0), az=-90, el=89.9, d=400, ortho=200, res=(1000, 1300)),
    'dinh':         dict(t=(-196, 91, 7), az=8, el=10, d=62, lens=35),
    'dinh_34':      dict(t=(-200, 91, 6), az=45, el=28, d=85, lens=32),
    'bo_tay':       dict(t=(-205, 60, 0), az=-170, el=32, d=230, lens=30, res=(1400, 860)),
}


def lin(h):
    h = h.lstrip('#')
    c = [int(h[i:i + 2], 16) / 255.0 for i in (0, 2, 4)]
    return tuple(x / 12.92 if x <= 0.04045 else ((x + 0.055) / 1.055) ** 2.4 for x in c)


def setup(sc):
    sc.render.engine = 'BLENDER_WORKBENCH'
    sh = sc.display.shading
    sh.light = 'STUDIO'
    sh.color_type = 'MATERIAL'
    sh.show_object_outline = True
    sh.object_outline_color = lin('#2B211C')
    sh.show_cavity = True
    sh.cavity_type = 'WORLD'
    sh.show_shadows = True
    sh.shadow_intensity = 0.35
    sc.display.light_direction = (-0.45, -0.35, 0.82)
    sc.world = sc.world or bpy.data.worlds.new('World')
    sc.world.color = lin('#DCDCD0')
    sc.render.film_transparent = False
    sc.render.image_settings.file_format = 'PNG'
    sc.view_settings.view_transform = 'Standard'
    sc.view_settings.look = 'None'
    sc.view_settings.exposure = 0.25


def main():
    argv = sys.argv[sys.argv.index('--') + 1:] if '--' in sys.argv else []
    out = os.path.abspath(argv[0] if argv else 'renders/review')
    names = argv[1:] or list(SHOTS)
    os.makedirs(out, exist_ok=True)
    sc = bpy.context.scene
    setup(sc)
    cd = bpy.data.cameras.new('REVIEW_Cam')
    cam = bpy.data.objects.new('REVIEW_Cam', cd)
    sc.collection.objects.link(cam)
    sc.camera = cam
    for name in names:
        s = SHOTS[name]
        t = Vector(s['t'])
        az, el = math.radians(s['az']), math.radians(s['el'])
        cam.location = t + Vector((math.cos(el) * math.cos(az), math.cos(el) * math.sin(az), math.sin(el))) * s['d']
        cam.rotation_euler = (t - cam.location).to_track_quat('-Z', 'Y').to_euler()
        if s.get('ortho'):
            cd.type, cd.ortho_scale = 'ORTHO', s['ortho']
        else:
            cd.type, cd.lens = 'PERSP', s.get('lens', 35)
        cd.clip_start, cd.clip_end = 0.5, 5000
        sc.render.resolution_x, sc.render.resolution_y = s.get('res', (1200, 800))
        sc.render.resolution_percentage = 100
        sc.render.filepath = os.path.join(out, name + '.png')
        bpy.ops.render.render(write_still=True)
        print('[review_shots]', sc.render.filepath)


main()
