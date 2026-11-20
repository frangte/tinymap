# Yêu cầu của Khoa (ghi lại 2026-11-11)

Mọi yêu cầu đã nêu, kể cả phần chưa làm, để không thất lạc giữa các phiên làm việc. Mỗi mục ghi trạng thái, những gì đã tìm hiểu được và cách làm đề xuất. Khi một mục xong hoặc đổi, sửa ở đây.

Thứ tự ưu tiên: **R1 làm trước**, vì đổi map thì gameplay đổi theo. R2–R7 làm sau trên map mới.

## Quy tắc làm việc

- Lên plan trước, làm từng phần, hỏi Khoa trước khi bắt đầu một plan lớn.
- **Model mới phải được duyệt design trước khi vẽ Blender thật** (bản vẽ, phác thảo, render nháp).
- **Giữ vibe màu hiện tại:** chỉ dùng bảng `PAL` (docs/PROJECT.md). Màu mới phải được duyệt.
- **Nhà phải đúng kiểu Sài Gòn**, có tìm hiểu, không vẽ đại (map hiện tại nhìn giống Hà Nội: nhà 2 tầng mái ngói đỏ).
- Code theo CLAUDE.md: Blender là nguồn của map; va chạm lấy thẳng từ file GLB (không bake nav nữa, D21 từ 2026-11-17), server chỉ kiểm tra tốc độ.

## R1. Design lại map: "Sài Gòn mini" (ưu tiên 1, đang làm)

Bản đề xuất: [map-v5.md](map-v5.md). Design được duyệt 2026-11-11; blockout lần 1 xong cùng ngày (`models/saigon_mini.blend`, dựng bằng `scripts/blender/build_saigon_mini.py` từ `scripts/map/saigon_mini_layout.json`), chờ duyệt lần 2. Đảo nới thành 500 × 445 m cho vừa kích thước thật của các model cũ.

- Bỏ chữ "Quận 1": đây là **Sài Gòn mini**, gom nhiều nơi của Sài Gòn chứ không chỉ Quận 1.
- Sinh động hơn, nhiều thứ hơn: **nhiều đường phố hơn, nhiều block nhà hơn**.
- Nhà theo kiểu nhà ở Sài Gòn (tìm hiểu thật, xem map-v5.md phần "Nhà kiểu Sài Gòn").
- Có **sông** (sông Sài Gòn), **hai đầu sông chảy ra biển**; **cầu Ba Son** bắc qua sông.
- **Biển là biển Vũng Tàu:** Bãi Trước, Bãi Sau, Bãi Dâu.
- **Núi:** 1 núi hoặc 2 đồi, giả lập Núi Nhỏ có **Tượng Chúa Kitô Vua** (tượng Chúa dang tay).
- Chi tiết cho sinh động: **đèn giao thông** ở ngã ba, ngã tư (xem R4).
- Sau khi duyệt: dựng khối thô (blockout) trong một file version mới, render cho Khoa duyệt lần hai, rồi mới làm chi tiết.
- Khi map mới vào game: đổi tên hiển thị "Sài Gòn Q1" thành "Sài Gòn mini" (index.html, `src/game/i18n/`, docs), sửa `places.json`, chỗ đứng NPC (`npcs.json`), quest dùng địa điểm, điểm xuất phát, khung lưới nav; quyết định D10 (giữ đảo ~200 m) đổi thành quyết định mới trong `docs/design/README.md`.

## R2. Vượt vật cản, không bao giờ bị kẹt (xong 2026-11-17, cách khác với đề xuất bên dưới)

**Trạng thái:** bỏ hẳn nav (D21). Người chơi là hình con nhộng va chạm thẳng với model (`src/game/collider.ts`): bước lên được bậc 0.5 m, nhảy lên vật thấp hơn ~1.2 m, rơi khỏi gờ thì rơi thật, chỉ nước sâu hơn 0.25 m và mép map mới chặn. Không còn túi kẹt hay tường vô hình do bake. Thử bằng trình duyệt headless: 110 điểm xuất phát rải khắp đảo, đi ngẫu nhiên tổng hơn 80 phút, không lần nào kẹt trong vật, lọt tường hay rơi xuống nước sâu. Phần dưới là hướng cũ, giữ lại để tham khảo.

**Sửa thêm 2026-11-18** (Khoa báo không đi vào được Bến xe Chợ Lớn): đi chéo vào bó vỉa hay bệ chờ xe thì nhân vật trượt dọc mép thay vì bước lên, nặng nhất ở màn 120 Hz (đi chéo 45° qua bậc bị kẹt 22% số lần, chéo 65° bị 83%). Giờ nhân vật dò bậc theo hướng bị đẩy lại, nên lên được từ mọi hướng: trên 500 mép bậc rải khắp map, chéo tới 65° không lần nào kẹt ở 60, 120 và 144 khung hình/giây. Bỏ thêm tường vô hình dưới đầu cầu (Thủ Thiêm 3, cầu đi bộ, cầu Ông Lãnh) và quanh bệ Bitexco; thay vào đó gầm nào thấp hơn người thì chính mặt cầu chặn, không chui vào rồi kẹt được.

Yêu cầu: nhảy qua được vật thấp (chậu cây, bồn hoa, ghế). Hiện nhân vật xuất hiện trong bùng binh Quách Thị Trang thì không ra được.

Đã tìm hiểu (2026-11-11, trên `saigon_q1_nav.png` hiện tại):
- Có nhảy (`jump_speed` 4.6, `gravity` 15: cao ~0.7 m; đáp được lên chỗ cao tới ~1.2 m). Nhưng nav chỉ có ô "đi được" và ô "cấm", không có chiều cao vật cản; người không bao giờ được đứng trên ô cấm, kể cả lúc đang bay. Vì thế nhảy chỉ giúp lên chỗ cao hơn, không vượt qua được vật.
- Túi kẹt: phép loang trong `bake_nav.py` (4 ô kề, bước 0.5 m) không tính thân người, nên lọt qua khe 1 ô (0.35 m) giữa hai bồn cây mà người rộng 0.52 m không lọt. Những ô bên trong được đánh dấu "đi được" nhưng không có đường ra. Đếm được: 2 túi trong bùng binh Quách Thị Trang (~9 m² quanh (3.7, −24), ~8 m² quanh (8, −20)), 4 túi nhỏ ở Hồ Con Rùa, 3 ở Lam Sơn, 2 ở công viên 23/9, 2 ở bãi biển. Người chơi có thể rơi vào túi khi được đặt lại chỗ cũ (vị trí lưu), lúc xuống xe, hoặc khi server kéo về; lúc đó chỉ thoát được bằng nút về chỗ xuất phát.
- Cách đếm: đọc nav PNG, tính khoảng trống quanh mỗi ô như `src/game/clearance.ts`, lấy các ô đứng vừa người (khoảng trống ≥ 0.26 m), loang từ điểm xuất phát theo luật của `player.ts` (lên 0.5 m, xuống 1.1 m, nhảy lên 1.2 m), đi thuận và đi ngược; ô đứng được mà không về được điểm xuất phát là túi kẹt.

Cách làm đề xuất (chỉ đổi bake và thêm một lệnh kiểm tra, không cần model mới):
1. Bake loang theo thân người và tính cả nhảy; ô người không tới được thì đánh dấu cấm.
2. Vật thấp (cao hơn sàn quanh nó dưới ~1 m: bồn hoa, chậu cây, ghế, mép hồ) thành bậc nhảy lên được: ô có sàn bằng đỉnh của vật. Đi bộ vẫn vòng qua (cao hơn 0.5 m không bước lên được), nhảy thì đáp lên rồi bước xuống bên kia. Thân cây, cột đèn, tượng vẫn chặn; chỗ không muốn cho trèo thì đặt hộp `NAV_Block_`.
3. Thêm `npm run nav:check`: báo lỗi nếu còn túi kẹt. Chạy sau mỗi lần bake.

## R3. Đi lên cao: cầu thang Hồ Con Rùa, cầu, cầu thang nhà (xong 2026-11-17, cách khác với đề xuất bên dưới)

**Trạng thái:** làm ở client theo model, đúng như Khoa nghĩ (D21): mặt cầu, cầu thang, sân thượng, vòng đi bộ trên cao đi được nếu model có bậc ≤ 0.5 m; nhiều tầng chồng nhau không cần gì thêm vì va chạm là hình 3D thật. Server không kiểm tra độ cao nữa. Giới hạn còn lại: không đi dưới gầm đầu cầu trên bờ và gầm Núi Nhỏ (game tự dựng vách ở mép hở). Phần dưới là hướng cũ.

Khoa nghĩ phần này nên làm ở client theo model. Đã tìm hiểu:
- Mỗi ô nav chỉ có một mặt sàn; bake chọn mặt thấp nhất còn 2 m trống phía trên, nên vòng đi bộ trên cao, mặt cầu, sân thượng đều bị bỏ qua. Cầu thang Hồ Con Rùa (sàn 0.5 → 2 m quanh (43, 85)) hiện là một túi không vào được.
- Không làm riêng ở client được: server kiểm tra từng bước đi theo nav của nó (D4, `server/internal/world/move.go`: hai ô kề chênh sàn tối đa 1.2 m, độ cao người phải gần mặt sàn). Client tự cho leo theo mesh thì server từ chối và kéo người về. CLAUDE.md cũng cấm raycast mesh mỗi frame.

Cách làm đề xuất (vẫn lấy từ model, nhưng ở bước bake):
1. Hộp `NAV_Deck_<tên>` trong collection `NAV`: trong hộp, bake chọn mặt cao nhất đi được. Dùng cho cầu thang và vòng đi bộ Hồ Con Rùa, cầu Ba Son, đường bậc thang lên tượng Chúa, sân thượng có cầu thang. Không đổi định dạng nav, không đổi server.
2. Bậc thang ≤ 0.5 m mỗi ô 0.35 m (bậc thật ~0.17 m), rộng ≥ ~0.8 m.
3. Giới hạn: không đi được cả trên lẫn dưới cùng một chỗ (dưới gầm cầu). Nếu thật sự cần thì làm nav hai tầng (thêm ảnh nav tầng trên, người chơi mang số tầng, server kiểm tra theo tầng); việc lớn, để sau.

## R4. Đèn giao thông và chi tiết phố (thuộc R1, phần model phải duyệt)

- Đèn đỏ ở ngã ba, ngã tư. Cột và hộp đèn vẽ trong Blender; ba bóng là mesh riêng do Three.js đổi màu, không đổ bóng như `lampGlow`. Chu kỳ tính theo đồng hồ chung nên mọi người thấy cùng màu mà không cần gửi tin qua mạng.
- 3 màu mới `sigRed`, `sigAmber`, `sigGreen` (duyệt 2026-11-11 cùng bản design).
- Chi tiết khác (xe bánh mì, sạp báo, dù cà phê vỉa hè, trạm xe buýt, biển tên đường, thuyền trên sông): danh sách trong map-v5.md.
- Xe tự chạy theo làn và dừng đèn đỏ: việc riêng, để sau.

## R5. Sông Sài Gòn, cầu Ba Son, hai đầu sông ra biển (thuộc R1)

- Bố cục trong map-v5.md. Lòng sông thấp hơn mực nước biển thì shader biển hiện có (`src/game/sea.ts`, màu theo độ sâu) tự vẽ nước và bọt dọc bờ.
- Cầu Ba Son: bản v2 (`versions/sg.tinymap_v2.blend`, render `renders/q1_bason_opera.png`) đã có cầu dây văng một trụ cong; lấy lại khung đó, chỉnh tỉ lệ và màu.
- **2026-11-12: design cầu Ba Son mới được duyệt** ("đẹp rồi"): khung v2 chỉnh theo sông v5, script `scripts/blender/mini/cau_ba_son.py`, bản nháp `models/saigon_mini_draft_cau_ba_son.blend`. Việc còn lại là T0 trong [map-v5-tasks.md](map-v5-tasks.md). Cầu thứ hai ở Hàm Nghi sẽ là cầu Mống (T7).

## R6. Xe trên map: ai cũng leo lên lái được, không cần là Crab (chờ)

Hiện tại: mỗi xe thuộc người mang nó vào, chỉ chủ được lái (`server/internal/world/vehicle.go`), và phải có quyền `vehicle.motorbike.ride` (chỉ vai tài xế có). Xe đậu trong `CX_Traffic_v4` chỉ là mesh tĩnh.

Cách làm đề xuất:
1. Xe công cộng do server quản lý, khai báo trong `server/content/` (loại xe, chỗ đậu gốc theo Blender XY). Không có chủ; ai bấm E trước thì lên ghế lái (server quyết). Xuống xe thì xe nằm lại đó; bỏ không N phút thì về chỗ đậu gốc. Mỗi người lái một xe một lúc.
2. Vai nào cũng lái được xe công cộng. Chuyến Crab Bike vẫn chỉ chạy bằng xe riêng của tài xế (D15 không đổi).
3. Xe đậu tĩnh nào thành xe thật thì xoá khỏi Blender (như D7 với NPC), rồi bake lại.
4. Thứ tự: xe máy (dùng lại `src/game/motorbike.ts`) → xe đạp, xích lô (model mới, duyệt trước) → ô tô, taxi (xe dài 4 m cần cách kiểm tra va chạm khác vòng tròn bán kính 0.42 m, ở cả client và server). Có thể thêm buýt đường sông hoặc tàu cao tốc từ bến Bạch Đằng ra Vũng Tàu khi có sông.
5. Ghi quyết định D21 vào `docs/design/README.md` khi được duyệt (D20 là map mới).

## R7. Game co-op (chờ: viết design trước, `docs/design/11-coop-games.md`)

Dựa trên những gì đã có: quest engine, kiểu "hỏi và đáp" của D18, ghế sau của Crab Bike, NPC, xe công cộng (R6). Ý tưởng để chọn:
1. **Giao hàng đôi:** một người lái, một người ngồi sau ôm hàng, giao N món trước khi hết giờ.
2. **Hướng dẫn viên:** một Citizen dẫn nhóm Traveler qua 3 công trình, mỗi điểm trả lời đúng một câu thì cả nhóm được tem (mở rộng D18).
3. **Tìm đồ thất lạc:** mỗi vai thấy một phần manh mối (Citizen thấy gợi ý, Traveler thấy món đồ, tài xế chở nhóm), phải phối hợp.
4. **Tiếp sức:** chạy hoặc lái qua các trạm, trao lượt cho đồng đội; đường chạy qua cầu Ba Son, kết thúc trên đỉnh Núi Nhỏ.
5. **Săn ảnh:** giải câu đố để biết các điểm, cả nhóm phải có người đứng ở mọi điểm cùng lúc.
6. Map mới gợi thêm: leo bậc thang lên tượng Chúa theo nhóm, đường ra Hòn Bà chỉ hiện lúc nước ròng (theo giờ), đua thuyền trên sông.

## R8. Rà lại toàn bộ map v5 và vẽ lại những chỗ chưa hợp lý (2026-11-12, chờ duyệt thứ tự)

Khoa yêu cầu sau khi duyệt cầu Ba Son:
- Cầu còn lại thành cầu Mống.
- Núi thoải hơn; tượng Chúa dang tay đẹp hơn; hải đăng phải ở trên núi (đúng vị trí thật).
- Thủ Thiêm còn trống.
- Nhà kề nhau giống nhau, không giống ngoài đời.
- GLB phải nhẹ để load lên web.
- Bỏ khối Cổng Trời ở mũi nam.
- Chia thành task nhỏ nhưng giữ bối cảnh.

Đã rà map (ảnh `docs/img/v5r_*.jpg`, đã xóa 2026-11-20, xem git), đo GLB (1.39 triệu tam giác, 19 MB qua pipeline hiện tại, 3.7 MB khi nén meshopt) và chia thành T0–T14 trong [map-v5-tasks.md](map-v5-tasks.md). Mỗi task ghi hiện trạng, tư liệu thật, design đề xuất, cách dựng và điều kiện xong. Chờ Khoa duyệt thứ tự và 5 câu hỏi cuối file đó.

## R9. Vẽ lại chi tiết Hồ Con Rùa, Bến Nhà Rồng, Dinh Độc Lập (2026-11-18, đã dựng, chờ Khoa xem)

- Hồ Con Rùa: cầu thang lên chưa đúng, các lối chưa nối với nhau, hồ chưa có nước (nước làm bằng three.js).
- Bến Nhà Rồng: còn thô, chi tiết hơn, quay đúng hướng ra sông, cảnh xung quanh giống ngoài đời.
- Dinh Độc Lập: khuôn viên rối, có nhà dân; làm lại cho đúng, có hàng rào, cổng chính bị xe tăng húc đổ, cổng nhỏ bên
  cạnh để người chơi đi vào.

Design, sơ đồ, ảnh tham khảo và kết quả: [landmarks-v9-design.md](landmarks-v9-design.md). Khoa duyệt design và chốt:
hai xe tăng (390, 843), tường Nhà Rồng hồng cam, shader nước cho mọi hồ. Đã dựng cả ba, map xuất lại, thử vật lý xong.

Sửa sau góp ý (2026-11-18, 19): xoá dãy nhà bị hàng rào Dinh cắt ngang ở góc đông bắc; camera nhìn xuyên thân cây
và song sắt, lùi ra chậm sau khi bị chắn, khi zoom xa thì ngẩng lên thay vì lao vào; bỏ cột cờ Thủ Ngữ; bỏ cây trong
bùng binh (giữ vườn thấp); bo tròn các mũi đất nhọn ở bờ sông, kênh (`TIP_R` trong `build_saigon_mini.py`).

Thêm (2026-11-19): vẽ lại cầu đi bộ cho giống thật (vòm lá dừa nước, mặt cầu hình lá, làn xe đạp, lan can song đứng)
và ba bức tượng chi tiết hơn (Trần Hưng Đạo trên bệ lăng trụ tam giác có chữ, Đức Mẹ Hòa Bình cầm quả địa cầu, Bác Hồ
ở Bến Nhà Rồng mặc vest xách va li). Đã dựng, xuất map, thử vật lý: xem [landmarks-v9-design.md](landmarks-v9-design.md).

## R10. Design lại game: từ tay trắng tới tài xế Crab (2026-11-19, đã duyệt)

Khoa quyết định đổi cách vào game:
- Vào game là người bình thường, mặc đồ thường: chọn nam/nữ, người Việt hay người nước ngoài; có sẵn một ít tiền.
- Dùng tiền mua điện thoại (nhiều đời, giá khác nhau: Ifone X, 11…), đặt Crab.
- Làm nhiệm vụ cho đủ tiền mua xe, tới chỗ NPC đăng ký chạy Crab (trừ phí), thành tài xế. Sau đó thêm nhiều nhiệm vụ.
- Người chỉ muốn đi dạo: tới một chỗ thì NPC nói chuyện hoặc hiện tooltip, bấm vào đọc lịch sử, thông tin, mẹo du lịch.
  **Đây là mục đích chính của trang web.**
- Soạn kịch bản trước rồi mới quyết định.
- Điện thoại là điện thoại thật, có ứng dụng ẩn. Thông tin tách ra một menu riêng (Sổ tay), đọc được mà không cần vào
  điện thoại (Khoa chốt cùng ngày).
- Không làm nút "Dạo phố" riêng: ai vào game cũng đi dạo, đọc thông tin được, cốt truyện chỉ là phần chơi thêm (Khoa
  chốt cùng ngày).
- Chạy Crab không phải mục tiêu chung: người Việt lập nghiệp thành tài xế; người nước ngoài làm được nếu muốn, nhưng
  mục tiêu của họ là nhiệm vụ ngầm "Sổ sưu tầm": chuyện ngoài lề về địa điểm, món ăn (Khoa đưa nội dung sau), học
  xin chào, cảm ơn bằng tiếng Việt (Khoa, cùng ngày).
- Người Việt có cả hai kiểu: người từ quê lên lập nghiệp và người Sài Gòn gốc. Sổ sưu tầm cho mọi người (người nước
  ngoài: mục tiêu chính), phần Tiếng Việt (xin chào, cảm ơn, bao nhiêu tiền, mắc quá, ngon quá) đứng đầu (Khoa, cùng
  ngày). Khoa duyệt kịch bản: "ok ổn rồi".
- Bỏ chuỗi nhiệm vụ "Người Sài Gòn chính hiệu", thiết kế lại sau (Khoa, cùng ngày). Đã gỡ khỏi game: 6 quest, tem, danh
  hiệu, thành tích, bài kiểm tra của Cô Ba; chuỗi chỉ còn làm dữ liệu mẫu cho test.

Bản nháp kịch bản: [design/10-story.md](design/10-story.md) (hai chương mở đầu, điện thoại, kiếm tiền, thi bằng, đăng
ký Crab, thẻ thông tin công trình, mẹo theo tình huống, bảng giá, NPC mới, thứ tự làm). Quyết định D22 (thay D1) ghi
"proposed" trong `docs/design/README.md`. Còn 7 câu hỏi cuối file đó chờ Khoa (câu 1 về "Dạo phố" đã chốt).

**2026-11-19, Khoa: "làm Sổ tay với thẻ thông tin trước".** Đã làm xong (chưa deploy): Sổ tay (nút "Sổ tay", phím B),
thẻ thông tin dưới khung tên địa điểm (phím I), trang chi tiết cho 5 công trình đầu, dẫn đường tới đó; Hồ sơ, Nhiệm vụ,
Thành tích, Bản đồ ra khỏi điện thoại. Chi tiết ở mục 14 của [design/10-story.md](design/10-story.md).

**2026-11-19, phần còn lại của bước 1 (làm song song bằng 5 agent, đã review và gộp):** mẹo theo tình huống và Sổ mẹo
(9 mẹo), Sổ sưu tầm phần Tiếng Việt (9 câu học qua lời thoại Cô Ba, Chú Tư, Dì Năm, Chú Sáu; tab Sưu tầm trong Sổ tay),
trang Cẩm nang cho cả 20 công trình. Còn chờ: nội dung chuyện ngoài lề về địa điểm và món ăn của Khoa.

**2026-11-19, Khoa chốt cho bước 2:** vẫn cho đổi xuất phát ("Đổi vai"); tiền hiện là "đồng", giá thật cho đồ nhỏ; ai
vào game cũng chưa có điện thoại, phải mua từ đầu, có thông báo cho người chơi biết đó là nhiệm vụ đầu tiên; tên máy
Nokla, Ifone dùng được. Bước 2 (tạo nhân vật, đồ và cửa hàng, điện thoại thành món đồ) đang làm, cách làm ở
[design/11-items.md](design/11-items.md).

**2026-11-19, đợt 1 của bước 2 làm xong (chưa deploy):** ba xuất phát (thêm Người từ quê lên ở Bến xe Chợ Lớn), ai
cũng mặc đồ thường và chưa có điện thoại; thẻ "Nhiệm vụ đầu tiên"; anh Bảy bán 4 đời máy ở tiệm Dế Yêu trên Lê Lợi;
Nokla chỉ có Thông báo, Ifone có mọi ứng dụng; tiền hiện bằng đồng, mang theo 500.000đ. Đã thử mua máy từ đầu tới cuối
trên trình duyệt (tiếng Việt và tiếng Anh, máy tính và điện thoại). Còn lại: chương 0 của từng xuất phát, danh bạ, tin
nhắn, cô Kim đổi tiền.

**2026-11-19, đợt 2 của bước 2 làm xong (chưa deploy):** chương 0 cho người từ quê lên (tờ giấy ghi địa chỉ, nhà trọ
bà Chín) và người nước ngoài (hostel anh Tony học xin chào, cảm ơn; cô Kim dạy phân biệt tờ 20.000 và 500.000; trả giá
với Cô Ba; qua đường với Chú Sáu; hỏi giá cà phê, bánh tráng trộn); điện thoại thật: Danh bạ (gặp ai thì có số người
đó), gọi điện cho NPC ở đâu cũng được, Tin nhắn từ NPC, cuộc gọi về cho Má (Má gửi 100.000đ, dặn đi gặp Chú Sáu). Quest
mua điện thoại vẫn đứng đầu khung nhiệm vụ. Chưa làm: "Về nhà" ở nhà trọ bà Chín và hostel anh Tony, tour đi bộ của người
nước ngoài, đổi đô ở tiệm cô Kim (bỏ: ai cũng có sẵn tiền đồng).

**2026-11-20, bước 3–4 làm xong (chưa deploy, PR #11):** Khoa: không ai chọn làm tài xế Crab lúc đầu, phải làm nhiệm
vụ mới lên được. Đổi vai chỉ còn ba xuất phát. Việc vặt chương 2 (bưng cà phê, giao bánh tráng, phát tờ rơi, khuân
hàng; mỗi việc một lần một ngày, Khoa duyệt "ok ổn"); chương 3: thi bằng A1 với Thầy Mười (lệ phí, lý thuyết 10 câu,
thực hành bằng xe của trường trên đường Thủ Thiêm vì chưa có model sân tập), mua xe cũ ở tiệm Chú Tám (trả giá bớt
10%), đăng ký ở văn phòng Crab với Chị Vy; vai tài xế bỏ, tài xế cũ giữ xe, đồng phục, bằng. Khách NPC đặt xe khi không
có khách thật. Chưa làm: tài xế NPC và Chú Sáu chở xe ôm khi không ai nhận cuốc (cần đồ thị đường, cùng R4), sân tập
lái (chờ duyệt model).

## R11. Truyện có tình tiết bất ngờ và Xưởng truyện để Khoa tự design (2026-11-20, chờ chốt)

Khoa: truyện hiện quá dễ đoán; cần một UI hay flow để design truyện trên đó, và thêm nhiều tình tiết "không nhặt được
mồm". Đề xuất (cờ truyện, nhiều kết cục, sự kiện trễ, manh mối, biến thể riêng mỗi người, quest ẩn; trang Xưởng truyện
chạy cùng `npm run dev`) và các câu cần chốt: [design/14-story-tools.md](design/14-story-tools.md). Đã làm engine cờ truyện và nhiều kết cục quest (cơ chế 1 và 2, chưa có nội dung truyện, PR chờ duyệt): [design/15-flags-outcomes.md](design/15-flags-outcomes.md). Chưa làm: sự kiện trễ, manh mối, biến thể riêng, quest ẩn, Xưởng truyện.

## Thứ tự đề xuất

R1 (duyệt design → blockout → duyệt → chi tiết, gồm R4, R5) → R2 và R3 khi bake map mới → R6 → R7 (design viết song song, game 1 và 4 cần R6).
