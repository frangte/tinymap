# Sài Gòn Q1: project brief

Đọc file này trước khi làm tiếp. Code game xem README.md, quy ước code xem CLAUDE.md.

![Toàn cảnh](img/00_overview.jpg)

## Dự án là gì

Quận 1 thu gọn dạng diorama low-poly, dựng trong Blender (`sg.tinymap.blend`). Người chơi đi bộ tự do trong Three.js, kiểu game Messenger (messenger.abeto.co). Vào game thì nhân vật đứng trước cổng nam chợ Bến Thành.

- **Đã xong:**
  - Map v2: một mạng đường liền mạch, 9 công trình bên dưới, cây, ít xe và người.
  - Menu mở đầu có 2 chế độ: [1] xem bản đồ (xoay, zoom, bay tới từng công trình) và [2] vào game đi bộ (nhân vật đội nón lá, camera góc thứ 3 tự do, tên địa điểm hiện trên HUD).
- **Để dành, chưa làm:**
  - Phố đi bộ Bùi Viện: dùng lại kiểu dãy nhà ống (`SH_01..05`, hàm `house_*` trong script 15b/15c).
  - UBND ở đầu bắc Nguyễn Huệ, chỗ ngã ba với Lý Tự Trọng, khoảng (46, -26).
- **Tiếp theo:** NPC và quest.
- **Bản cũ:** v1 đóng băng ở `versions/sg.tinymap_v1.blend`. Không sửa đè, muốn đổi lớn thì lưu version mới.
- **Cách làm việc:** plan lớn phải hỏi Khoa trước khi làm.

## Style (giữ nguyên khi vẽ thêm)

- **Low-poly, màu phẳng:** không texture, không UV. Màu chỉ lấy từ bảng `PAL` (Text `saigon_diorama_01_setup.py`). Vật liệu tên `SD_<key>`, tạo bằng `get_mat(key)`.
- **Toàn bộ là shader:**
  - Node group `SD_Paint` tạo nhiễu màu, bẩn chân tường, vệt mưa, hạt giấy và ranh sáng tối răng cưa.
  - Viền mực: trong Blender là Grease Pencil LineArt `SD_Ink`; trong game là ink pass screen-space.
  - Game dựng lại paint shader theo tên vật liệu (`src/game/paint.ts`).
- **Ánh sáng:** nắng ấm `#FFF2DC`, bóng lạnh `#717BA2`.
- **Méo tay:**
  - Modifier `SD_HandWobble` (Displace): `LM_*` là 0.045, các object khác 0.07. Chạy `art_pass_geometry()` sau khi thêm object.
  - Kính và thép hiện đại (Bitexco, cầu Ba Son) để thẳng, không méo.
- **Tỉ lệ:**
  - Đơn vị mét. Người cao 1.7 m, tầng nhà 3.2–3.6 m.
  - Lòng đường 6–10 m, vỉa hè 2.6–3 m.
  - Công trình đặt trên nền z = 0.18 (cao bằng vỉa hè).
  - Chiều cao được nén cho gọn: Bitexco chỉ 52 m.
- **Cách giản lược:** giữ silhouette và 2–3 chi tiết đặc trưng (tháp đồng hồ, tháp chuông, vòm, đỉnh hoa sen), có thể phóng to các chi tiết này 1.2–1.5 lần. Bỏ chi tiết vụn.

### Bảng màu chính (key trong `PAL`)

| Nhóm | Key và màu |
|---|---|
| Tường | cream `#EAD9AE`, creamLight `#F4EBD3`, wallGrey `#A0A7AA`, wallTeal `#5E9A96`, wallOchre `#C9A25E`, wallBeige `#CFC1A0`, wallPink `#D29A86`, opWhite `#EEE7D7`, opShade `#D8CDB8`, poYellow `#E0B45A`, brick `#B5523B`, stone `#E6DCC8`, towerPale `#DAD4C8`, concrete `#ABA69C` |
| Mái | roofRed `#A9492F` (dark `#7D3326`, light `#C0613F`), slate `#646B74`, spire `#6F7780`, tinBlue `#8AA3B6` |
| Điểm nhấn | poGreen `#3E7F5F`, clockBlue `#4E7CA6`, clockWhite `#F3EEE2`, gold `#C9A24A`, lotusTop `#E6D29C`, turq `#5BA9A1`, archDark `#3B3330`, glassBlue `#7FA0B5`, glassTeal `#4F7A7C`, finSilver `#D3DAE0`, bridgeWhite `#EEF0EE`, statueW `#F2EFE8` |
| Mặt đất | asphalt `#6F6D69`, paving `#BDB4A2`, curb `#D3CBB8`, roadLine `#ECE6D4`, grass `#86A85E`, water `#6FA9B6` |
| Cây | leaf `#6E9B4C`, leafLight `#93B85F`, leafDark `#4D7539`, bark `#6B4C36` |
| Mực | ink `#2B211C`; lampGlow `#FFE6A8` (không đổ bóng) |

Thêm màu mới thì thêm key vào `PAL`. Nếu cần chỉnh variation hay grime riêng thì thêm key đó vào bảng `PAINT` trong `src/game/paint.ts`.

## File Blender

- **Scene và collection:**
  - Scene `SaigonDiorama`.
  - `SD_World` chứa mọi thứ có viền mực; `SD_NoInk` chứa nước và dây điện.
  - Object nằm ngoài hai collection này thì không được export.
- **Tiền tố tên object:**
  - `IS_` đế đảo; `IX_` đường, sông, bờ kè.
  - `BT_` chợ Bến Thành; `LM_` công trình lớn; `SH_` nhà ống.
  - `CX_` phần lấp đầy thành phố (cây, xe, người, công viên, tháp Thủ Thiêm); `LF_` cây và đồ trang trí cũ quanh chợ.
- **Script nằm trong Text datablock của file `.blend`:**
  - `saigon_mini_lib.py`: bộ dựng `Geo` (`box`, `bc`, `cyl`, `cone`, `ico`, `lathe`, `extrude`, `rod`, `pyramid`, `hip`, `arch`, `text`) và `make_obj`.
  - `saigon_diorama_01_setup.py`: `PAL`, `get_mat`, `SD_Paint`, ink.
  - `saigon_diorama_02…18_*.py`: mỗi bước một công trình.
  - `20/21/22_v2_*.py`: layout, đường và build của v2. Hai script này cần `shapely` và `mapbox_earcut`.
  - `00_ns_loader.py`: `load_ns()` nạp lại toàn bộ hàm mà không chạy các bước build.
  - `saigon_diorama_TODO.md`: ghi chú.
- **Toạ độ:**
  - Blender XY, trục Z hướng lên. Đảo rộng khoảng X −69…158, Y −137…46.
  - Lưới phố nghiêng theo trục `U=(0.6606, 0.7507)` (từ sông lên nhà thờ) và `VL=(0.7507, −0.6606)`.
  - Hàm `lm(dv, du) = LAMSON + VL·dv + U·du`, với `LAMSON=(49.29, −60.33)`.
  - Công trình theo lưới phố quay `yaw = −2.2924`.
- **Điểm mốc (Blender XY):**
  - Bùng binh Bến Thành (8, −24); Lam Sơn (49.3, −54.3); Công xã Paris (79.7, −25.8); Hồ Con Rùa (91.5, 22.4).
  - Nguyễn Huệ chạy từ (−5.8, −85.1) qua (30.5, −43.8) đến (46.4, −25.8).

Mẫu thêm một công trình (tạo Text mới `saigon_diorama_23_<ten>.py`):

```python
exec(bpy.data.texts['saigon_diorama_00_ns_loader.py'].as_string()); ns = load_ns()
g = ns['Geo'](2026)
g.box('cream', -6, -4, 0, 6, 4, 9)              # tên vật liệu = key trong PAL; ch=0.06 là vát cạnh
g.hip('roofRed', -6.4, -4.4, 6.4, 4.4, 9, 3.2)  # mái 4 mái
ns['place_landmark'](g, 'LM_TenCongTrinh', x, y, yaw, z=0.18)
ns['art_pass_geometry']()
```

## Công trình

Ảnh "model" là render từ map hiện tại, dùng để giữ design khi sửa lại. Ảnh "thật" là thư mục ảnh trên Wikimedia Commons, có nhiều góc để tham chiếu.

| Công trình | Object · tâm (Blender XY) · cao | Đặc trưng cần giữ | Ảnh |
|---|---|---|---|
| Chợ Bến Thành (1914) | `BT_*` · (8, 8) · tháp 24.9 m | Tháp đồng hồ ở giữa cổng nam, mặt đồng hồ xanh, chóp mái ngói đỏ. Các dãy chợ mái ngói đỏ, tường xám kem. Cổng vòm ở 4 mặt, hàng quán dưới mái hiên. Bùng binh Quách Thị Trang phía trước (`LF_Roundabout`: cây tạo hình tầng, 4 cây cọ, bồn hoa) | [model](img/01_cho_ben_thanh.jpg) · [thật](https://commons.wikimedia.org/wiki/Category:Ben_Thanh_Market) |
| Nhà hát Thành phố (1900) | `LM_NhaHatLon` · (65.4, −68.5) · 14.6 m | Tường kem trắng (opWhite, opShade). Vòm lớn giữa mặt tiền có kính hình quạt, hai cánh hai bên, mái xám. Đã có mặt sau và hai mặt bên. Đài phun nước Lam Sơn phía trước | [model](img/02_nha_hat.jpg) · [thật](https://commons.wikimedia.org/wiki/Category:Municipal_Theatre_of_Ho_Chi_Minh_City) |
| Nhà thờ Đức Bà (1880) | `LM_NhaThoDucBa` · (98, −5) · 26.6 m | Gạch đỏ (brick), hai tháp chuông chóp xám nhọn, cửa sổ tròn và đồng hồ giữa hai tháp | [model](img/03_nha_tho_duc_ba.jpg) · [thật](https://commons.wikimedia.org/wiki/Category:Notre-Dame_Cathedral_Basilica_of_Saigon) |
| Bưu điện Thành phố (1891) | `LM_BuuDien` · (94.9, −41.8), yaw −2.2924 · 15.7 m | Tường vàng (poYellow), cửa vòm xanh lá (poGreen), đồng hồ trên vòm cổng chính, hai khối đầu hồi, mái slate. Có sân trước nối ra Công xã Paris; đã có mặt sau | [model](img/04_buu_dien.jpg) · [thật](https://commons.wikimedia.org/wiki/Category:Saigon_Central_Post_Office) |
| Công xã Paris | `LM_TuongDucMe` · (79.7, −25.8) | Bùng binh có tượng Đức Mẹ trắng (statueW) trên bệ, nằm trước nhà thờ | [model](img/08_cong_xa_paris.jpg) · [thật](https://commons.wikimedia.org/wiki/Category:Paris_Commune_Square_(Saigon)) |
| Hồ Con Rùa (1967) | `LM_HoConRua` (+`_Water`) · (91.5, 22.4) · 15.1 m | Hồ tròn có vòi phun, trụ cao với đỉnh hình hoa sen (lotusTop), cầu thang lên vòng đi bộ trên cao, tường xanh ngọc (turq). Sân vòng ngoài có ghế và bồn hoa | [model](img/05_ho_con_rua.jpg) · [thật](https://commons.wikimedia.org/wiki/Category:H%E1%BB%93_Con_R%C3%B9a) |
| Bitexco (2010) | `LM_Bitexco` · (−15.5, −53.5) · 52.4 m | Tháp kính xanh hình búp sen thuôn dần, đỉnh cắt xiên. Sân đỗ trực thăng nhô ra ở khoảng 2/3 chiều cao, khối đế bên dưới. Không méo tay | [model](img/06_bitexco.jpg) · [thật](https://commons.wikimedia.org/wiki/Category:Bitexco_Financial_Tower) |
| Cầu Ba Son (2022) | `LM_CauBaSon` (+`LF_CauBaSon_Traffic`) · trục từ (95, −88) theo (−0.578, 0.816) · trụ 24.8 m | Cầu dây văng một trụ trắng hình cánh buồm ở bờ Thủ Thiêm, dây xoè hình quạt, cột đèn vàng. Mặt cầu đi bộ được. Không méo tay | [model](img/07_cau_ba_son.jpg) · [thật](https://commons.wikimedia.org/wiki/Category:Ba_Son_Bridge) |
| Quảng trường Lam Sơn | `CX_LamSon` (+`_Water`) · (49.3, −54.3) | Đài phun nước tròn nhiều tầng giữa bồn hoa, thu nhỏ 0.72 lần | [model](img/09_lam_son.jpg) · [thật](https://commons.wikimedia.org/wiki/Category:Lam_Son_Square) |
| Dãy nhà ống | `SH_01..05`, `SH_StreetLife`, `SH_Wires` · x −45…−19, y −17…−7 · 8.8–13.9 m | Mỗi căn một mặt tiền (nhà cầu thang, tiệm sư tử, tiệm lu…). Có ban công, mái tôn, biển hiệu, máy lạnh, dây điện. Đây là mẫu style cho Bùi Viện | [model](img/10_nha_ong.jpg) |
| Thủ Thiêm | `CX_Towers_ThuThiem` · x 65…151, y −130…−69 · 25.7 m | Các tháp nhạt màu (towerPale) có dải kính teal, nằm bên kia sông | [model](img/11_thu_thiem.jpg) |

## Pipeline: từ Blender ra game

1. Sửa `sg.tinymap.blend`.
2. Export GLB, file `.blend` không bị đổi:
   ```bash
   blender sg.tinymap.blend --background --python scripts/blender/export_glb.py -- export/saigon_q1_raw.glb
   ```
3. Thu nhỏ file bằng 5 lệnh `gltf-transform` ghi trong đầu `export_glb.py` (`flatten → join → weld → quantize --pattern POSITION → prune`). Ra `export/saigon_q1.glb` (khoảng 3.5 MB), rồi copy sang `public/models/saigon_q1.glb`.
4. Bake lại bản đồ đi lại, mất khoảng 30 giây:
   ```bash
   blender sg.tinymap.blend --background --python scripts/blender/bake_nav.py -- public/models/saigon_q1
   ```
   - Nhà mới mà camera không được chui vào: thêm tiền tố tên vào `ROOF_PREFIX`.
   - Nếu không ai đi được bên dưới nhà đó: thêm vào cả `SOLID_PREFIX`.
5. Thêm tên địa điểm mới vào `src/game/places.ts`, toạ độ Blender XY.

![Trong game](img/12_gameplay.jpg)
