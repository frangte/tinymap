# Sài Gòn mini v8: thiết kế lại bố cục

**Trạng thái:** Khoa duyệt tổng thể ngày 2026-11-12, sau đó sửa các chỗ Khoa khoanh (mục "Sửa sau góp ý" bên dưới).
v8 là map chính: `scripts/map/relayout_v8.py` ghi `scripts/map/saigon_mini_layout.json`, `models/saigon_mini.blend`
dựng từ đó. Bản v7 giữ ở `scripts/map/saigon_mini_layout_v7.json` và `versions/saigon_mini_v4.blend`.

![Sơ đồ v8](img/v8_plan.jpg)
![Toàn cảnh v8](img/v8_overview.jpg)

## Vì sao làm lại

Góp ý của Khoa sau v7: sông thẳng, bố cục rời rạc, có những block nhà thừa, chưa giống một map game làm kỹ. Cần:
bố cục rõ, nhìn tổng thể tự nhiên, cảm giác có thể khám phá cả thành phố lẫn bãi biển.

## Nguyên tắc rút ra từ tài liệu

1. **Người chơi định hướng bằng công trình mốc nhìn thấy từ xa** (nghiên cứu "The Image of the Open World Game", dùng 5
   yếu tố của Kevin Lynch: đường, biên, khu, nút, mốc). Đường trong game ít khi có tên, nên mốc phải cao và khác biệt;
   chỗ đứng cao (núi, cầu) giúp nhìn thấy các mốc.
2. **"Weenie" của Disneyland**: đặt một công trình cao, lạ ở cuối mỗi trục đường để kéo người chơi đi tới (lâu đài cuối
   Main Street). Trục chính đi vào một khu trung tâm, từ đó toả ra các khu, mỗi khu một chủ đề và một mốc riêng.
3. **A Short Hike**: đảo nhỏ, đích đến rõ là đỉnh núi nhìn thấy từ mọi nơi; có đường mòn dẫn lối nhưng vẫn cho đi tắt
   khám phá; thưởng nhỏ rải ở những chỗ lệch khỏi đường chính.
4. **Sable**: đầu và cuối rõ ràng, giữa thoáng; các điểm khám phá như những đảo nhỏ, mỗi chỗ tự đủ nội dung; chấp nhận
   chỗ thưa có chủ ý thay vì lấp đầy cho có.
5. **Messenger (abeto)**: thế giới bé, đi một vòng vài phút, chỗ nào cũng có chi tiết sống động, đi vài lần là thuộc.
6. **Sài Gòn thật**: Thủ Thiêm là bãi bồi nằm trong khúc sông uốn (sông ôm phía tây và phía nam); kênh Bến Nghé tách
   Quận 1 với Quận 4, có cầu Mống bắc qua; Bến Nhà Rồng ở góc kênh và sông; cầu đi bộ hình lá dừa nước nối Bạch Đằng
   (đầu Nguyễn Huệ) với quảng trường Thủ Thiêm; trục Dinh Độc Lập, công viên 30/4, công xã Paris (Nhà thờ, Bưu điện).

## Bố cục v8

**Nước là khung.**
- Sông Sài Gòn vào từ phía bắc, chảy dọc thành phố, rồi uốn một vòng lớn sang đông quanh Thủ Thiêm và ra biển, như
  sông thật.
- Kênh Bến Nghé rẽ ra ở khúc uốn, chạy chéo về tây nam ra biển.
- Sông và kênh chia đảo thành **ba khu**, mỗi khu một tính cách:

| Khu | Tính cách | Mốc nhìn từ xa |
|---|---|---|
| Trung tâm (Quận 1, Chợ Lớn) | phố Pháp, quảng trường, phố đi bộ, hẻm, chợ | Bitexco, tháp Nhà thờ Đức Bà, Chợ Bến Thành |
| Thủ Thiêm (trong khúc sông) | đô thị mới, công viên, hồ, tháp kính | tháp Thủ Thiêm ~72 m cuối trục Nguyễn Huệ |
| Khánh Hội – Vũng Tàu | cảng, phố cũ, khách sạn, hai bãi biển, núi | tượng Chúa và hải đăng trên Núi Nhỏ |

**Năm cây cầu là các nút, nối thành một vòng tham quan.**
- Cầu Ba Son ở phía bắc (đã có model).
- Cầu đi bộ lá dừa nước: nối thẳng từ phố Nguyễn Huệ sang quảng trường Thủ Thiêm; mặt cầu phình ở giữa như phiến lá.
- Cầu Thủ Thiêm 3: nối Thủ Thiêm với Vũng Tàu.
- Cầu Mống: cầu đi bộ qua kênh, cạnh Bến Nhà Rồng.
- Cầu Ông Lãnh: qua kênh ở đầu phố Đề Thám, Bùi Viện.

Đường vàng cam trên sơ đồ là tuyến tham quan gợi ý (sau này làm tuyến dẫn đường trong game): Bến Bạch Đằng → Nguyễn
Huệ → UBND → Nhà thờ, Bưu điện → công viên 30/4 → Dinh Thống Nhất → Bến Thành → Bùi Viện → cầu Ông Lãnh → Khánh Hội →
Bãi Trước → Núi Nhỏ → Bãi Sau → cầu Thủ Thiêm 3 → Thủ Thiêm → cầu đi bộ → về Bến Bạch Đằng.

**Trục nào cũng kết thúc ở một công trình.**
- Nguyễn Huệ: từ UBND, qua cầu đi bộ, qua quảng trường và hai hồ phản chiếu, tới tháp Thủ Thiêm. Đây là trục chính của
  cả map, nhìn từ UBND thấy thẳng tháp bên kia sông.
- Trục công viên 30/4: từ Dinh Thống Nhất lên công xã Paris (Nhà thờ, Bưu điện), tới Hồ Con Rùa.
- Lê Duẩn: từ Hồ Con Rùa ra cầu Ba Son.
- Lê Lợi: nối Nhà hát với Bến Thành.
- Đồng Khởi: từ Nhà thờ ra sông.

![Trục Nguyễn Huệ nhìn sang Thủ Thiêm](img/v8_truc_nguyen_hue.jpg)

**Phân cấp đường.**
- Đại lộ có hàng cây: Lê Duẩn, Đồng Khởi, Lê Lợi, Mai Chí Thọ, Quang Trung.
- Phố thường, phố đi bộ (Nguyễn Huệ, Bùi Viện), lối dạo trong công viên.
- Đường ven biển vòng quanh từng khu. Bờ sông và bờ kênh có lối đi dạo liên tục.

**Ít block hơn nhưng to hơn**, mọi block là một lô ghi rõ trong layout (không còn lưới tự sinh): 310 căn, trước 505.
Chỗ trống thành công viên có tên ở các nút: 30/4, Tao Đàn (có lối đi chéo), 23/9, Bến Bình Đông, rừng dừa nước,
công viên Bãi Trước. Không còn bãi cỏ thừa.

**Bãi biển.**
- Bãi Trước quay về tây nam, có cầu tàu cánh ngầm (tàu từ Bạch Đằng theo kênh ra), dù, ghe.
- Bãi Sau dọc bờ đông, có khách sạn trên Thùy Vân.
- Núi Nhỏ ở mũi đông nam, có tượng Chúa và hải đăng; đỉnh núi là chỗ ngắm toàn map.

![Khúc sông và Thủ Thiêm](img/v8_khuc_song.jpg)
![Bãi Trước và Núi Nhỏ](img/v8_bai_truoc.jpg)
![Dinh Thống Nhất và công viên 30/4](img/v8_dinh_30_4.jpg)
![Thủ Thiêm](img/v8_thu_thiem.jpg)

## Những gì giữ nguyên và những gì đổi

- **Giữ nguyên hình (chỉ dời hoặc xoay):** cầu Ba Son, Núi Nhỏ cùng tượng và hải đăng (vẫn chỗ cũ của v7), khối phố
  Bùi Viện v4 (dời sang tây), Dinh Thống Nhất (quay mặt về bắc nhìn lên công viên 30/4), các công trình cũ của map.
- **Đổi:**
  - Sông, thêm kênh Bến Nghé.
  - Đảo nhỏ hơn: 144 nghìn m² (v7: 156 nghìn m²).
  - Thủ Thiêm có trục chính và tháp cao ở cuối trục; ba tháp thấp ở phía bắc.
  - Khánh Hội thành khu mới giữa kênh và sông.
  - Chợ Lớn và bến xe dời về góc tây nam.
  - Nhà thờ Đức Bà quay mặt ra công xã Paris.
- **Script dựng đã hỗ trợ v8 và vẫn dựng v7 y như cũ:** kênh (`canals`), cầu đi bộ (`walk`, `wMid`), cầu tàu
  (`piers`), tuyến tham quan (`tour`, chỉ vẽ trên sơ đồ), chọn layout bằng `SAIGON_LAYOUT`.

## Sửa sau góp ý (2026-11-12)

Khoa khoanh các chỗ: đường không liền mạch, cầu đáp xuống giữa lòng đường, đường đâm vào nhà, nhà đứng chơ vơ.

- **Đầu đường:** `relayout_v8.py` có bước dọn cuối, áp cho mọi đầu đường:
  - đầu đường trong bùng binh thì kéo vào tâm (hết khấc ở công xã Paris, Lê Lai vào Quách Thị Trang);
  - đường chạy lố qua đường khác thì cắt ở ngã ba (Lê Lai ở mép biển, Đề Thám ở cửa kênh, Quốc lộ 51, Quang Trung vào chân núi);
  - đường dừng hụt thì kéo thẳng tới đường phía trước (các phố ra đường ven biển);
  - đường lệch vài mét thì dời đầu lên tim đường kia.
- **Chỗ nối:** đầu đường trong bản dựng bo tròn, nên hai đường nối đầu vào nhau ở một góc (đường ven biển và Võ Văn
  Kiệt, Quốc lộ 51 và Hạ Long) không còn khấc.
- **Cầu:**
  - Cầu đi bộ lá dừa nước xuống trên vỉa hè và lối dạo bờ sông, không còn đáp xuống lòng Tôn Đức Thắng.
  - Cầu xe (Thủ Thiêm 3, Ông Lãnh): đầu cầu nằm trên tim đường, phẳng bằng mặt đường suốt ngã ba và vỉa hè (`flat`),
    rồi mới lên dốc (`ramp`) trên mặt nước. Lan can chỉ có trên đoạn qua nước.
  - Thùy Vân bắt đầu ngay chân cầu Thủ Thiêm 3.
  - Cầu Mống đi từ vỉa hè bờ kênh bên này sang vỉa hè bên kia.
- **Không còn đường cụt:**
  - Lê Thánh Tôn chạy tiếp ngang trước mặt UBND như đường thật, tới phố mới Lý Tự Trọng sau Bến Thành. Con đường sau
    lưng UBND bắt đầu từ đó, không còn đâm vào lưng UBND.
  - Bỏ đường cụt ở phía bắc Thủ Thiêm, chỗ đó thành vườn.
  - Đường lên hải đăng nối vào Quốc lộ 51 bằng một đoạn ngắn.
- **Chợ Lớn:**
  - Bỏ con đường chạy xuyên chùa Bà Thiên Hậu và bến xe.
  - Chùa nhìn ra một quảng trường nhỏ, phố Bùi Viện chạy thẳng vào quảng trường đó.
  - Bến xe quay ra đường ven biển, dãy phố người Hoa bao quanh.
- **Bỏ block thừa:** dải nhà 8 m sát biển phía bắc, dải nhà sau Chung cư 42, khách sạn bị Thùy Vân đè.
- **Kiểm tra tự động:** `scripts/map/check_layout.py` bắt các lỗi trên: đầu đường lơ lửng, cầu đáp vào lòng đường
  hoặc lên dốc giữa ngã ba, đường đâm vào công trình, lối dạo cụt, lô nhà quá nhỏ hoặc không có đường trước mặt, công
  trình không có lối vào. v8 có 0 lỗi (v7 có 12).

![Công xã Paris: các nhánh vào bùng binh liền](img/v8_fix_cxp.jpg)
![Cầu đi bộ xuống trên vỉa hè bờ sông](img/v8_fix_cau_di_bo.jpg)
![Cầu Thủ Thiêm 3 xuống ngang mặt đường](img/v8_fix_tt3.jpg)
![Đường lên hải đăng nối Quốc lộ 51](img/v8_fix_nui_duong.jpg)
![Chợ Lớn: quảng trường chùa Bà Thiên Hậu, bến xe](img/v8_fix_cho_lon.jpg)
![Cầu Ông Lãnh, cầu Mống qua kênh Bến Nghé](img/v8_fix_ong_lanh.jpg)
![Công viên Tao Đàn, 30/4, UBND](img/v8_fix_tao_dan.jpg)
![Cửa kênh Bến Nghé](img/v8_fix_kenh.jpg)

Dựng lại map chính từ v8: 315 căn nhà, 1,04 triệu tam giác, GLB 3,0 MB (gzip 1,0 MB), đã thử nạp bằng three.js.

## Chưa làm

- Model thật cho các cầu mới, tháp Thủ Thiêm, Bến Nhà Rồng, Khánh Hội: hiện là khối thô, làm sau khi duyệt bố cục,
  mỗi cái gửi bản nháp trước.
- Bộ nhà (T9, T10): bố cục v8 đã chừa đúng chỗ cho các block, bộ nhà sẽ lấp vào.

## Nguồn

- [The Image of the Open World Game (Wellesley)](https://cs.wellesley.edu/~pmwh/mvmap/papers/open_world_elements/open_world_elements.html)
- [Level Layout, Disneyland Style: wienies](https://www.above-the-garage.com/wienies/)
- [What Mario learned from Mickey Mouse, phần 3: weenies (Game Developer)](https://gamedeveloper.com/design/what-mario-learned-from-mickey-mouse---part-3-decision-making-and-weenies)
- [Crafting a Tiny Open World: A Short Hike postmortem (GDC Vault)](https://gdcvault.com/play/1026613/Independent-Games-Summit-Crafting-A), [tóm tắt trên 80.lv](https://80.lv/articles/gdc-a-short-hike-postmortem)
- [Sable: cấu trúc thế giới hình quả chanh (Game Developer)](https://www.gamedeveloper.com/design/using-a-lemon-shaped-structure-to-create-the-sanguine-open-world-of-i-sable-i-)
- [Messenger, game trình duyệt của abeto (Aftermath)](https://aftermath.site/messenger-browser-game-abeto)
- [Thủ Thiêm, bãi bồi trong khúc sông Sài Gòn (Wikipedia)](https://www.wikiwand.com/en/articles/Thu_Thiem_New_Urban_Area)
- [Cầu đi bộ qua sông Sài Gòn hình lá dừa nước (SGGP)](https://www.sggp.org.vn/cau-di-bo-qua-song-sai-gon-hinh-la-dua-nuoc-post709248.html)
- [Tứ giác vàng di sản Sài Gòn xưa (Doanh Nhân Plus)](https://doanhnhanplus.vn/tu-giac-vang-di-san-sai-gon-xua-482016.html)
