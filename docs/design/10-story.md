# Kịch bản game (bản nháp 2026-11-19)

Viết theo hướng Khoa đưa ra ngày 2026-11-19 ([R10](../requirements.md#r10-design-lại-game-từ-tay-trắng-tới-tài-xế-crab-2026-11-19-bản-nháp-chờ-duyệt)). Đây là bản nháp để Khoa duyệt, **chưa làm gì trong code**. Nếu được duyệt thì quyết định D22 thay cho D1 (chọn vai lúc vào game), xem [README.md](README.md#decisions). Các câu cần Khoa chốt ở [cuối file](#15-cần-khoa-chốt).

## 1. Ý của Khoa

- Vào game là một người bình thường, mặc đồ thường: chọn nam hay nữ, người Việt hay người nước ngoài; có sẵn một ít tiền.
- Dùng tiền mua điện thoại (nhiều đời máy, giá khác nhau: Ifone X, 11…), rồi đặt Crab.
- Làm nhiệm vụ cho đủ tiền mua xe, tới chỗ một NPC đăng ký chạy Crab (đăng ký thì trừ tiền), thành tài xế.
- Sau đó thêm nhiều nhiệm vụ nữa.
- Người chỉ muốn đi dạo: tới một chỗ thì NPC nói chuyện, hoặc hiện tooltip, bấm vào đọc lịch sử, thông tin, mẹo du lịch Sài Gòn. **Đây là mục đích chính của trang web.**
- Điện thoại là điện thoại thật, có các ứng dụng ẩn. Thông tin tách ra một menu riêng, đọc được mà không cần vào điện thoại (thêm sau, cùng ngày).

## 2. Nhận xét

Ý này hợp, và hay hơn cách chọn vai hiện tại ở ba chỗ:

- Có một câu chuyện với mục tiêu rõ (từ tay trắng tới tài xế Crab) thay vì chọn một tấm thẻ lúc đầu. Thành tài xế là phần thưởng phải kiếm, nên nó có giá trị.
- "Lên Sài Gòn lập nghiệp rồi chạy xe công nghệ" là chuyện thật của rất nhiều người. Người Sài Gòn chơi sẽ thấy quen, người nước ngoài chơi sẽ hiểu thêm về thành phố.
- Phần lớn thứ đã làm vẫn dùng được: ví xu, Crab Bike, quest engine, quyền (capability) kiếm thêm được, 5 NPC, quest Citizen và tài xế. Xem [mục 13](#13-so-với-design-hiện-tại).

Bốn chỗ phải tính thêm thì ý này mới chạy được:

1. **Không được chặn người đi dạo.** Mục đích chính là thông tin du lịch, còn chuỗi "mua điện thoại, kiếm tiền, mua xe" là lớp chơi thêm. Nếu phải mua điện thoại mới đọc được thông tin, người vào từ Google tìm "Sài Gòn có gì chơi" sẽ thoát ngay. Đề xuất: thẻ thông tin nằm ngay ngoài phố, không cần điện thoại hay tiền, nên ai vào game cũng đọc được từ phút đầu ([mục 3](#3-một-cách-vào-ai-cũng-đi-dạo-được)).
2. **Ít người online là kẹt.** Đặt Crab cần một tài xế thật đang chạy; làm tài xế cần một khách thật. Lúc 3 giờ sáng không có ai. Cần khách NPC (dễ: NPC chỉ ngồi sau xe người chơi) và chỗ thay thế khi không có tài xế: xe ôm Chú Sáu, rồi tài xế NPC (khó hơn: NPC phải tự chạy theo đường). Xem [mục 8](#8-chương-3-thành-tài-xế-crab) và [mục 13](#13-so-với-design-hiện-tại).
3. **Máy đắt phải có lý do để mua, nhưng không được mua sức mạnh.** Đề xuất: cục gạch lên smartphone thì mở thêm app; đời cao hơn chỉ để khoe và chụp ảnh đẹp hơn ([mục 6](#chương-1-cái-điện-thoại-đầu-tiên)).
4. **Phải tính tiền trước cho khỏi cày.** Lì xì mỗi ngày như bây giờ (nạp lại tới 50.000) thì không cần làm gì; giá xe thật (vài chục triệu) thì cày cả tuần. Đặt mục tiêu theo thời gian chơi rồi mới ra giá ([mục 11](#11-tiền)).

Bổ sung thêm, chi tiết ở các mục dưới:

- Thi bằng lái A1 với vòng số 8, trước khi mua xe.
- Chú Sáu xe ôm là bạn cũ của ba người chơi, là người khuyên đi chạy Crab, và về sau được người chơi dạy xài app.
- Mẹo du lịch hiện đúng lúc: qua đường, trả giá, tiền polymer, giật điện thoại, mưa chiều.
- Giá đồ ăn, đồ uống, cuốc xe trong game là giá thật ngoài đời, nên chơi cũng là học giá.
- Đố vui ở mỗi công trình cũng ra tiền, để người đi dạo cũng kiếm được.

## 3. Một cách vào, ai cũng đi dạo được

Khoa chốt (2026-11-19): **không làm nút "Dạo phố" riêng.** Ai cũng vào game cùng một cách (tạo nhân vật, [mục 4](#4-tạo-nhân-vật)), và từ phút đầu ai cũng làm được những việc cơ bản, không cần điện thoại hay tiền:

- đi khắp phố, nói chuyện với NPC, chat với người đứng gần;
- đọc thẻ thông tin ở mỗi công trình và mọi trang trong Sổ tay ([mục 6](#6-điện-thoại-và-sổ-tay));
- xem bản đồ, và nhờ dẫn đường tới một công trình.

Cốt truyện (mua điện thoại, kiếm tiền, chạy Crab) là phần chơi thêm: nhiệm vụ chỉ gợi ý, không chặn ai. Người chỉ muốn dạo phố, tìm hiểu Sài Gòn thì cứ đi, không phải làm nhiệm vụ nào. Mọi người cùng một thế giới, cùng chat, thấy nhau như bây giờ (nguyên tắc cốt lõi của design không đổi).

## 4. Tạo nhân vật

- **Nam hay nữ:** đã có (`man`, `woman` trong `src/game/outfits.ts`).
- **Xuất phát:** Người Việt (từ quê lên) hoặc Người nước ngoài (du khách ba lô). Chọn một lần.
- Tên, rồi bảng thay đồ đang có, chỉ với đồ thường (áo thun, sơ mi). Đồng phục Crab chỉ có khi đăng ký.
- Người nước ngoài cần thêm màu da và màu tóc (vàng, nâu, đỏ). Hiện chỉ có một màu da `#C68D66`; màu mới phải được duyệt theo bảng PAL.
- Ngôn ngữ giao diện vẫn là cài đặt riêng (D19), không theo xuất phát: người Việt chơi nhân vật nước ngoài được, và ngược lại.
- **Xưng hô:** NPC lớn tuổi gọi người chơi là "con", không theo giới; người ngang tuổi gọi "anh" hay "chị" theo giới. Lời thoại cần thêm một biến xưng hô (ví dụ `{anh_chi}`), như `{name}` đang có.

## 5. Chương 0: tới Sài Gòn

### Người Việt: "Lên Sài Gòn lập nghiệp"

Xuất hiện ở **Bến xe Chợ Lớn**, ba lô trên vai, trong túi 500.000đ. Điện thoại cũ hư từ dưới quê; má dặn lên tới nơi thì mua cái mới gọi về.

1. **Tờ giấy ghi địa chỉ.** Tracker là tờ giấy má viết: "Nhà trọ bà Chín, dãy nhà phố Lê Lai". Đi bộ tới đó; lần đầu bước xuống lòng đường thì hiện mẹo qua đường.
2. **Nhà trọ bà Chín.** Nhận phòng: đây là chỗ "Về nhà" (thay chỗ về nhà của Citizen bây giờ).
   - Bà Chín: "Con của con Hai dưới Bến Tre phải hôn? Phòng trên lầu, tiền phòng tháng này bà cho thiếu, kiếm được việc rồi đưa."
   - Bà chỉ tiệm điện thoại của anh Bảy trên Lê Lợi.
3. **Mua điện thoại** ([chương 1](#chương-1-cái-điện-thoại-đầu-tiên)).
4. **Gọi về cho má.** Cuộc gọi đầu tiên. Má gửi lên 100.000đ.
   - Má: "Tới nơi rồi hả con? Ăn uống đàng hoàng nghe. Có gì khó thì kiếm chú Sáu, bạn ba hồi xưa, chạy xe ôm ở bùng binh Quách Thị Trang."
   - Mở quest đi gặp Chú Sáu: quest đặt Crab đầu tiên.

### Người nước ngoài: "Ba lô tới Bùi Viện"

Xuất hiện ở **Phố đi bộ Bùi Viện**, vừa xuống xe buýt từ sân bay, trong túi 20 đô. Điện thoại rớt vỡ màn hình lúc xuống xe (hay lý do khác, xem [câu hỏi 2](#15-cần-khoa-chốt)).

1. **Hostel của anh Tony.** Nhận giường: đây là chỗ "Về nhà".
   - Tony nói tiếng Việt pha tiếng Anh: "Welcome, welcome! Phòng sạch, wifi mạnh, bia rẻ! Đổi tiền hả? Đừng đổi ở sân bay, ra tiệm vàng gần chợ Bến Thành, giá better!"
2. **Đổi tiền ở tiệm vàng của cô Kim** (đường Lê Thánh Tôn). 20 đô thành khoảng 500.000đ (tỷ giá trong game cố định, ghi trong content).
   - Mini game: cô Kim đếm tiền, người chơi chỉ ra tờ nào là 20.000, tờ nào là 500.000 (cùng màu xanh, dễ nhầm). Đúng thì được tem "Biết tiền Việt".
3. **Mua điện thoại và SIM** ([chương 1](#chương-1-cái-điện-thoại-đầu-tiên)).
4. Chuỗi nhiệm vụ riêng cho người nước ngoài: **thiết kế sau.** Chuỗi "Người Sài Gòn chính hiệu" cũ (Cô Ba dạy chào, cà phê, ba công trình, bánh tráng, hỏi đường, bài kiểm tra cuối) đã gỡ khỏi game ngày 2026-11-19 theo ý Khoa; nó chỉ còn làm dữ liệu mẫu cho test của quest engine (`server/internal/quest/testdata/traveler_questline.json`).

## 6. Điện thoại và Sổ tay

Khoa chốt (2026-11-19): **điện thoại là một chiếc điện thoại thật trong game, có ứng dụng ẩn; thông tin tách ra một menu riêng, đọc được mà không cần điện thoại.**

- **Sổ tay** là menu của game: một nút luôn có trên màn hình, từ phút đầu, không cần điện thoại hay tiền, cả lúc đang chơi lẫn lúc xem bản đồ. Mọi thông tin để đọc nằm ở đây.
- **Điện thoại** phải mua. Nó giống máy thật: màn hình chính, giờ Sài Gòn trên thanh trạng thái, thông báo, danh bạ, và những ứng dụng chỉ hiện khi mở được. Trong máy chỉ có những việc ngoài đời người ta làm bằng điện thoại.

Cách chia: **đọc, xem, tra cứu thì ở Sổ tay; gọi, nhắn, đặt xe, chụp ảnh thì ở điện thoại.**

### Sổ tay (menu)

| Mục | Có gì | Bây giờ nằm ở |
|---|---|---|
| Bản đồ | Cả đảo, các công trình, vị trí của mình; bấm một công trình thì mở thẻ thông tin, hoặc chọn làm điểm đến cho tracker | App Bản đồ, chế độ xem bản đồ |
| Cẩm nang | Các công trình với thẻ thông tin (đã đọc, chưa đọc), phần trăm khám phá, tour đi bộ, đố vui ([mục 10](#10-đi-dạo-và-tìm-hiểu-sài-gòn-phần-chính)) | App Cẩm nang (chỉ Du khách có) |
| Sổ mẹo | Các mẹo đã gặp | Mới |
| Nhiệm vụ | Nhiệm vụ đang làm và đã xong | App Nhiệm vụ |
| Tem và thành tích | Sổ tem, thành tích, danh hiệu | App Thành tích, sổ tem trong app Cẩm nang |
| Nhân vật | Tên, thay đồ, xuất phát, tiền đang có, đồ đang có (máy, xe, bằng lái) | App Hồ sơ, nút Thay đồ |
| Cài đặt | Ngôn ngữ, âm thanh, nét mực | Các nút công cụ |

Các màn hình app bây giờ (`src/game/phone/apps.ts`) vẽ vào một khung bất kỳ, nên chuyển chúng sang Sổ tay chủ yếu là đổi chỗ gắn, không phải viết lại.

### Điện thoại

| Ứng dụng | Làm gì | Máy |
|---|---|---|
| Điện thoại, Danh bạ | Gọi NPC đã gặp (gặp ai thì có số người đó): gọi Má, gọi Chú Sáu tới chở, gọi hỏi gợi ý khi kẹt nhiệm vụ | Mọi máy |
| Tin nhắn | NPC nhắn để mở nhiệm vụ mới ("Chú Tư: mai con ghé quán sớm nha"), tổng đài Crab nhắn | Mọi máy |
| Thông báo | Như bây giờ | Mọi máy |
| Crab | Đặt xe | Smartphone |
| Chỉ đường | Vẽ đường tới chỗ muốn đến, theo dõi tài xế Crab đang tới | Smartphone |
| Trò chuyện | Chat cả thành phố, nhắn riêng người chơi; hỏi đường người Sài Gòn (D18) | Smartphone |
| Camera, Album | Chụp ảnh (nhiệm vụ chụp ảnh, check-in ở công trình), quét mã QR, xem lại ảnh | Smartphone |

Nói chuyện với người đứng cạnh (chat gần) không cần điện thoại, như ngoài đời.

### Ứng dụng ẩn

Không hiện trên màn hình chính cho tới khi mở được. Server đã có sẵn cách làm: một app trong `apps.json` có `requires` thì chỉ hiện khi nhân vật có đủ các quyền đó, và quyền là phần thưởng của quest hoặc của món đồ.

| Ứng dụng | Hiện khi |
|---|---|
| Crab Driver | Đăng ký xong ở văn phòng Crab |
| Hàng Xóm (ai đang ở ngoài phố) | Quen đủ 3 hàng xóm (quest `citizen.chuyen_khu_pho` đang có) |
| Rắn săn mồi | Chỉ có trên Nokla |
| App tìm được ngoài phố (ý tưởng) | Quét mã QR dán trên tường bằng camera. Ví dụ: mã ở Bùi Viện cài app "Đêm Bùi Viện" (sự kiện buổi tối); mã ở sạp Cô Ba cài app đặt hàng của chợ |
| Crab Food, ví điện tử… | Các chương sau |

### Chương 1: cái điện thoại đầu tiên

Chung cho cả hai. Tiệm **"Dế Yêu"** của anh Bảy trên đường Lê Lợi ("dế" là tiếng lóng của điện thoại).

| Máy | Giá | Có gì |
|---|---|---|
| **Nokla 1280** (cục gạch) | 150.000đ | Điện thoại, Tin nhắn, Thông báo, Rắn săn mồi. Không có Crab, không có camera |
| **Ifone X** (máy cũ, trầy) | 400.000đ | Smartphone: mọi ứng dụng |
| **Ifone 11** | 1.200.000đ | Như Ifone X, thêm màu vỏ, hình nền và bộ lọc ảnh |
| **Ifone 15 Pro** | 6.000.000đ | Để khoe: tên máy hiện trong hồ sơ và khi cầm máy, khung ảnh riêng |
| Samsun, Oppa… | sau | Cùng kiểu: nhại tên thật như Crab, không dùng tên thật |

- Máy đắt không giúp kiếm tiền nhanh hơn, không chạy nhanh hơn. Crab Driver chạy trên mọi smartphone, như ngoài đời.
- Chưa có điện thoại vẫn làm được: mọi thứ trong Sổ tay, đi, nói chuyện với NPC, chat gần, gọi xe ôm khi đứng cạnh Chú Sáu.
- Anh Bảy: "Vô coi đi em, máy zin, bao test bảy ngày!" Mua cục gạch: "Cục gạch này rớt lầu ba hổng bể nha!" Với người nước ngoài: "SIM du lịch nè, 4G xài thả ga!"

**Quest: cuốc Crab đầu tiên** (dựa trên `traveler.di_crab` đang có):

- Người Việt đặt Crab tới bùng binh gặp Chú Sáu. Chú Sáu: "Trời đất, đi Crab tới kiếm xe ôm, thời buổi gì kỳ vậy con!"
- Người nước ngoài đặt Crab tới chợ Bến Thành gặp Cô Ba (chuỗi nhiệm vụ sau đó thiết kế sau).
- Lên xe thì hiện mẹo: kiểm tra biển số trên app, đội mũ bảo hiểm tài xế đưa.
- Không có tài xế nào nhận trong 60 giây: lần đầu thì Chú Sáu chở (xe ôm, giá thoả thuận trước); sau này là tài xế NPC.

## 7. Chương 2: kiếm tiền

**Việc vặt** làm lại được (có giới hạn mỗi ngày), mỗi việc 2–4 phút, và mỗi việc đưa người chơi tới một chỗ mới để thấy thẻ thông tin ở đó:

| Việc | Ai giao | Làm gì | Trả |
|---|---|---|---|
| Bưng cà phê | Chú Tư (Lê Lai) | Mang 3 ly tới khách ở các góc phố trước khi đá tan (có giờ) | 40.000đ |
| Giao bánh tráng | Dì Năm (Hồ Con Rùa) | Giao túi bánh tới Công viên 30/4, Nhà thờ Đức Bà | 30.000đ |
| Phát tờ rơi | Chị Hạnh (Lam Sơn) | Phát cho 5 người (NPC qua đường hoặc người chơi) trên phố đi bộ Nguyễn Huệ | 30.000đ |
| Khuân hàng | Cô Ba (Bến Thành) | Chuyển 3 kiện từ xe tải vào sạp | 40.000đ |
| Chụp ảnh giùm | du khách NPC | Chụp đúng góc ở một công trình (cần camera) | 20.000đ + tip |
| Chỉ đường | người chơi nước ngoài | "Thổ địa" và "Hỏi và đáp" (D18) đang có | 30.000đ |

**Quest cốt truyện** (một lần, trả nhiều hơn):

- Người Việt, **"Xin việc quán Chú Tư":** chạy bàn ca sáng. Chú Tư: "Bưng cho vững, ly của khách quen đó, đổ là chú trừ lương!" Dùng lại `citizen.ca_phe_sang`, `citizen.chuyen_khu_pho`.
- Người Việt, **"Chú Sáu kể chuyện":** hồi xưa Chú Sáu chạy xe ôm nuôi cả nhà, giờ khách đặt app hết. "Con còn trẻ, đi chạy Crab đi. Có bằng, có xe, có điện thoại là chạy được." Mở chương 3.
- Người nước ngoài: chuỗi nhiệm vụ thiết kế sau (chuỗi "Người Sài Gòn chính hiệu" cũ đã gỡ). Cuối chuỗi, anh Tony: "Muốn ở lâu hả? Phải kiếm tiền! Đi chạy Crab đi, Sài Gòn ai cũng chạy." Mở chương 3 ([câu hỏi 5](#15-cần-khoa-chốt)).

## 8. Chương 3: thành tài xế Crab

Bốn bước, theo thủ tục ngoài đời nhưng đơn giản hơn:

1. **Thi bằng lái A1** ở sân tập lái Thủ Thiêm (Thủ Thiêm đang trống). Thầy Mười. Lệ phí 200.000đ.
   - Lý thuyết: 10 câu về biển báo và luật (đèn đỏ, mũ bảo hiểm, rượu bia). Đúng 8 câu là đậu. Du khách đọc cũng có ích.
   - Thực hành: xe của trường; đi vòng số 8 và đường thẳng hẹp, không chạm vạch, không chống chân. Bài thi A1 ngoài đời có thêm đường quanh co và đường gồ ghề; game làm hai hoặc cả bốn.
   - Rớt thì thi lại, 50.000đ. Thầy Mười: "Nhìn xa phía trước, đừng ngó bánh xe! Ga đều tay!"
   - Đậu thì có quyền `vehicle.motorbike.ride`: lái được xe của mình, và xe công cộng khi có R6.
   - Cần model sân tập: phải duyệt design trước khi vẽ.
2. **Mua xe** ở tiệm xe cũ chú Tám (Chợ Lớn). Xe số 110 cũ, 1.000.000đ (giá game). Trả giá được: nói "Mắc quá chú!" (câu Cô Ba dạy) thì bớt 10%. Chú Tám: "Xe zin, máy êm ru, chạy chục năm chưa hư!"
3. **Đăng ký ở văn phòng Crab** trên đường Hàm Nghi (chỗ bãi xe tài xế bây giờ). Chị Vy kiểm tra smartphone, bằng lái, xe. Phí 300.000đ là bộ đồng phục (áo vàng, mũ vàng). Được quyền `uniform.crab`, `driver.app`, `ridehail.drive`.
   - Chị Vy: "Chào mừng {anh_chi} gia nhập đại gia đình Crab! Áo đây, mũ đây. Nhớ nha: năm sao là cơm, một sao là cháo!"
4. **Cuốc đầu tiên** (`driver.chuyen_dau_tien` đang có): chở một khách. Không có người chơi nào đặt xe thì một khách NPC đặt.
   - Người Việt gọi về khoe với má: hết chương. Má: "Con má giỏi quá! Chạy từ từ thôi nghe con."
   - Chú Sáu: "Giỏi! Bữa nào dạy chú xài cái app này với."

**Khách NPC:** đứng ở một chỗ và đặt xe như người chơi; tài xế tới, khách lên ngồi sau, tới nơi trả tiền và chấm sao. Khách chấm theo cách chạy: có tông gì không, có đi đúng chỗ không, có để khách chờ lâu không. Chỉ có khách NPC khi không có khách thật gần đó, mỗi tài xế tối đa một khách NPC một lúc. Khách NPC trả đúng giá nhưng không tip; chỉ khách thật mới tip, nên chở người thật vẫn có lời hơn.

## 9. Sau chương 3 (ý tưởng, chưa viết)

- **Dạy Chú Sáu xài app:** chở Chú Sáu tới văn phòng Crab, chỉ ông từng bước trên điện thoại; cuối cùng ông mặc áo vàng. Một mẩu chuyện về Sài Gòn đổi thay, từ xe ôm truyền thống sang xe công nghệ.
- **Hạng tài xế:** Đồng, Bạc, Vàng theo số cuốc và số sao; mua xe tay ga, đổi màu xe.
- **Crab Food:** lấy món ở quán NPC (bánh mì, cơm tấm), giao tới khách trước khi nguội.
- **Mưa chiều:** lúc mưa thì giá Crab tăng và khách NPC nhiều hơn, như ngoài đời.
- **Cuốc đi Vũng Tàu:** cuốc xa qua cầu, ra tới biển.
- **Nghề khác:** bán bánh mì xe đẩy, chụp ảnh dạo, hướng dẫn viên (dẫn người chơi khác đi tour, R7), lái buýt đường sông.
- **Nhà:** từ phòng trọ lên căn hộ ở Chung cư 42 Nguyễn Huệ, mục tiêu dài.
- **Người nước ngoài:** học tiếng Việt (mỗi câu học được là một tem), làm hướng dẫn viên tiếng Anh.

## 10. Đi dạo và tìm hiểu Sài Gòn (phần chính)

Ba lớp thông tin, lớp nào cũng không cần điện thoại hay tiền:

1. **Nhãn trên phố:** tới gần một công trình (trong bán kính `r` của nó trong `places.json`) thì hiện tên và dấu "i" phía trên.
2. **Thẻ nhỏ** dưới màn hình: tên (tiếng Anh và tiếng Việt như D19), một câu giới thiệu, ba thông tin nhanh (năm xây, giờ mở cửa, vé), và các nút **Đọc thêm**, **Hỏi người ở đây** (khi có NPC gần), **Chụp ảnh** (khi có camera).
3. **Trang chi tiết:**
   - Lịch sử (150–300 chữ).
   - Có gì để xem.
   - Mẹo: giờ đẹp, ăn mặc, giá, chỗ hay bị nói thách.
   - Ăn gì gần đây (bấm vào thì chỉ tới chỗ đó trong game).
   - Tới đó ngoài đời: Grab, xe buýt số mấy, metro, đi bộ từ đâu.
   - Ảnh thật, nút "Mở Google Maps".
   - "Kiểm tra lần cuối: ngày …": giờ mở cửa và giá vé đổi theo thời gian, nên phải ghi ngày.

Đọc xong thì tính vào Cẩm nang trong Sổ tay (phần trăm khám phá, danh sách công trình đã thấy, như app Cẩm nang bây giờ) và được tem.

**NPC kể chuyện.** NPC ở công trình có thêm nhánh hỏi chuyện, như hỏi một người thật: "Chợ này có từ hồi nào vậy cô?", "Quanh đây có gì ngon?", "Mua đồ ở đây có bị nói thách không?". Cùng nội dung với trang chi tiết nhưng là lời nói, ngắn hơn và có tính cách. 5 NPC đang có ở chợ Bến Thành, Lê Lai, Hồ Con Rùa, Lam Sơn, bùng binh; các công trình khác (Dinh Độc Lập, Bến Nhà Rồng, Nhà thờ Đức Bà, Vũng Tàu) cần thêm NPC: bác bảo vệ, cô bán vé, ông câu cá…

**Mẹo theo tình huống.** Một dòng hiện lên đúng lúc, rồi lưu vào Sổ mẹo trong Sổ tay:

| Lúc nào | Mẹo |
|---|---|
| Lần đầu bước xuống lòng đường | Qua đường: đi chậm và đều, đừng chạy, đừng lùi bất ngờ; xe sẽ lách bạn |
| Cầm điện thoại đứng sát lề | Đứng lùi vào trong, cầm điện thoại phía xa lòng đường: giật điện thoại là chuyện có thật |
| Lần đầu mua đồ ở chợ | Hỏi giá trước, trả giá vui vẻ. "Mắc quá!" (miền Nam) là "đắt quá" |
| Lần đầu trả tiền | Tờ 20.000 và tờ 500.000 cùng màu xanh, coi kỹ trước khi đưa |
| Lần đầu lên Crab | Kiểm tra biển số trên app, đội mũ bảo hiểm tài xế đưa |
| Lên xe ôm Chú Sáu | Xe ôm truyền thống: hỏi giá trước khi lên xe |
| Trời mưa | Mùa mưa hay có mưa chiều, mưa nhanh rồi tạnh; mang áo mưa; Crab lúc mưa đắt hơn |
| Vào nhà thờ, Dinh Độc Lập | Ăn mặc kín vai, kín gối |

**Tour đi bộ** trong Cẩm nang: một tuyến 4–6 điểm theo thứ tự, mỗi điểm mở một thẻ. Ví dụ "Sài Gòn thời Pháp" (Chợ Bến Thành, Nhà hát, Nhà thờ Đức Bà, Bưu điện), "Ăn vặt Sài Gòn", "Bờ sông" (Bến Bạch Đằng, Bến Nhà Rồng, cầu Ba Son). Làm được ngay vì không cần NPC biết đi; Chị Hạnh tự đi dẫn đoàn là việc sau.

**Đố vui:** đọc xong thì có 2–3 câu hỏi. Đúng thì được tem và 20.000đ, mỗi công trình một lần. Người đi dạo cũng có tiền, người theo cốt truyện có thêm lý do để đọc.

### Mẫu thẻ: Chợ Bến Thành

Dòng có ⚠ phải kiểm tra lại trước khi đưa vào game.

- **Một câu:** ngôi chợ biểu tượng của Sài Gòn, với tháp đồng hồ ở cửa Nam.
- **Nhanh:** khánh thành năm 1914 · mở cửa ⚠ khoảng 6:00–18:00 · vào cửa miễn phí.
- **Lịch sử:** chợ Bến Thành đầu tiên nằm gần bờ sông, cạnh thành Gia Định ⚠. Chợ bây giờ được xây từ năm 1912 tới năm 1914 trên khu đất lấp ao (ao Bồ Rệt) ⚠, khánh thành tháng 3 năm 1914. Tháp đồng hồ ở cửa Nam là hình ảnh quen thuộc nhất của Sài Gòn.
- **Có gì để xem:** bốn cửa Bắc, Nam, Đông, Tây; hàng vải, đồ lưu niệm, cà phê, khu ăn uống bên trong chợ.
- **Mẹo:** hỏi giá trước, trả giá ở sạp quần áo và đồ lưu niệm; giá ở đây thường cao hơn chợ trong khu dân cư; chỗ đông thì đeo túi phía trước.
- **Tới đó ngoài đời:** ⚠ ga ngầm Bến Thành của metro số 1 (Bến Thành – Suối Tiên) ở ngay trước chợ; đi bộ khoảng 10 phút tới Nhà thờ Đức Bà và Bưu điện.
- **Ăn gần đây:** trong game là bánh tráng trộn của Dì Năm ở Hồ Con Rùa; quán thật nào thì người viết chọn ⚠.

**Nội dung:** 20 công trình trong `places.json`, mỗi cái đủ tiếng Việt và tiếng Anh (D19). Phần này tốn công nhất: cần người viết, có nguồn cho từng dữ kiện, và ngày kiểm tra. Ở Dinh Độc Lập và Bến Nhà Rồng thì dùng đúng tên gọi và cách viết lịch sử chính thức ở Việt Nam.

## 11. Tiền

**Đơn vị:** hiện là "xu". Đề xuất đổi tên hiển thị thành "đồng" (đ) và dùng giá thật cho đồ nhỏ, để người chơi biết giá ngoài đời; đồ lớn (điện thoại, xe) là giá game và ghi rõ như vậy. Id trong code vẫn là xu ([câu hỏi 3](#15-cần-khoa-chốt)).

**Mục tiêu thời gian**, chơi theo cốt truyện, không cày:

| Mốc | Sau khoảng |
|---|---|
| Có điện thoại | 3 phút |
| Cuốc Crab đầu tiên (làm khách) | 6–8 phút |
| Thi đậu bằng lái | 25 phút |
| Mua xe, đăng ký, cuốc đầu làm tài xế | 45–60 phút (một hai buổi chơi) |

**Giá đề xuất**, đều nằm trong content nên sửa được:

| Khoản | Giá | Ghi chú |
|---|---|---|
| Tiền mang theo | 500.000đ (người Việt), 20 đô ≈ 500.000đ (người nước ngoài) | Hai bên bằng nhau |
| Cà phê sữa đá, bánh mì, bánh tráng trộn | 20.000–30.000đ | Giá thật ⚠ |
| Xe ôm Chú Sáu | 30.000đ, trả giá được | |
| Crab (làm khách) | 10.000đ + 60đ mỗi mét, như bây giờ: 15.000–40.000đ trên map mới | `ridehail.json` |
| Điện thoại | 150.000đ – 6.000.000đ | [Mục 6](#chương-1-cái-điện-thoại-đầu-tiên) |
| Thi bằng | 200.000đ, thi lại 50.000đ | |
| Xe số cũ | 1.000.000đ | Giá game |
| Đăng ký Crab (đồng phục) | 300.000đ | |
| Việc vặt | 20.000–40.000đ, khoảng 3 phút, có giới hạn mỗi ngày | |
| Quest cốt truyện | 50.000–200.000đ mỗi quest, khoảng 1.000.000đ cả chương 0–2 | |
| Đố vui | 20.000đ mỗi công trình, một lần | |
| Làm tài xế | 80% giá cuốc, cộng tip của khách thật | Như bây giờ |

Tính thử: mua Ifone X còn 100.000đ. Chương 3 cần 1.500.000đ (bằng 200.000, xe 1.000.000, đăng ký 300.000). Quest cốt truyện trả khoảng 1.000.000đ, đố vui khoảng 100.000đ, còn lại là 12–15 việc vặt.

- **Lì xì mỗi ngày:** vẫn chỉ nạp lại tới 50.000đ (đủ đi vài cuốc Crab), như bây giờ; không đủ mua đồ lớn, nên không phá mục tiêu.
- **Để sau:** mua xe trả góp (đặt cọc 30%, trả dần 20% mỗi cuốc). Đúng chuyện ngoài đời, nhưng là thêm một hệ thống nợ.

## 12. Nhân vật NPC

Tên đặt theo thứ trong nhà kiểu miền Nam (Ba, Tư, Năm, Sáu, Bảy, Tám, Chín, Mười), như 5 NPC đang có.

**Đang có**, thêm vai trong cốt truyện:

| NPC | Ở đâu | Vai |
|---|---|---|
| Cô Ba | Chợ Bến Thành | Dạy chào hỏi và trả giá; giao việc khuân hàng; kể chuyện chợ |
| Chú Tư | Quán cà phê, Lê Lai | Chỗ làm đầu tiên của người Việt; việc bưng cà phê |
| Dì Năm | Hồ Con Rùa | Bán bánh tráng trộn; việc giao bánh; mẹo ăn vặt |
| Chị Hạnh | Quảng trường Lam Sơn | Hướng dẫn viên: tour đi bộ, đố vui, việc phát tờ rơi |
| Chú Sáu | Bùng binh Quách Thị Trang | Xe ôm, bạn cũ của ba người chơi (người Việt); khuyên đi chạy Crab; về sau được dạy xài app |

**Mới:**

| NPC | Ở đâu | Vai |
|---|---|---|
| Bà Chín | Dãy nhà phố Lê Lai | Chủ nhà trọ, chỗ về nhà của người Việt |
| Anh Tony | Phố đi bộ Bùi Viện | Chủ hostel, nói tiếng Việt pha tiếng Anh; chỗ về nhà của người nước ngoài |
| Cô Kim | Tiệm vàng, đường Lê Thánh Tôn | Đổi tiền, dạy nhận biết tiền polymer |
| Anh Bảy | Tiệm điện thoại "Dế Yêu", đường Lê Lợi | Bán điện thoại, SIM |
| Thầy Mười | Sân tập lái, Thủ Thiêm | Thi bằng A1 |
| Chú Tám | Tiệm xe cũ, Chợ Lớn | Bán xe, cho trả giá |
| Chị Vy | Văn phòng Crab, đường Hàm Nghi | Đăng ký tài xế, phát đồng phục |
| Má | Chỉ qua điện thoại | Cuộc gọi về quê của người Việt |
| Khách NPC | Khắp map | Đặt Crab khi không có khách thật |

## 13. So với design hiện tại

**Giữ:**

- Ví xu và sổ cái, công thức giá Crab, phần chia cho nền tảng, tip, chấm sao (D11, D12, D15).
- Quest engine: quest tự bắt đầu (D14); thưởng xu, tem, danh hiệu, quyền. D1 đã cho quyền "kiếm thêm được", nên thành tài xế chỉ là phần thưởng của một quest.
- Quest Citizen cho người Việt, quest tài xế (nhận xe, thuộc đường, cuốc đầu) cho chương 3, quest "Đi Crab" cho người nước ngoài. Chuỗi Du khách "Người Sài Gòn chính hiệu" đã gỡ (2026-11-19), chuỗi mới cho người nước ngoài thiết kế sau.
- Hỏi và đáp (D18): người nước ngoài hỏi đường, người Việt trả lời.
- Tag `traveler` và `citizen` vẫn dùng để chọn quest, chỉ đổi nghĩa thành "xuất phát".

**Đổi:**

- **D1:** không chọn vai lúc vào game. `driver` không còn là vai chọn được mà là một nghề: quyền `vehicle.motorbike.ride` (có bằng lái), `uniform.crab`, `driver.app`, `ridehail.drive` (đăng ký xong). Bỏ "Đổi vai" ([câu hỏi 4](#15-cần-khoa-chốt)).
- **Điện thoại thành một món đồ.** Bây giờ ai cũng có điện thoại từ đầu. Mua máy thì có quyền `phone.basic` hoặc `phone.smart`; app trong `apps.json` vẫn dùng `requires` như bây giờ, và ứng dụng ẩn cũng là `requires`.
- **Thông tin ra khỏi điện thoại:** Hồ sơ, Nhiệm vụ, Thành tích, Cẩm nang, Bản đồ chuyển sang Sổ tay; quyền `traveler.guide_app` bỏ đi vì ai cũng có Cẩm nang. Trong máy còn Crab, Crab Driver, Trò chuyện, Thông báo, Hàng Xóm, thêm Điện thoại, Tin nhắn, Chỉ đường, Camera.
- Tiền mang theo và lì xì đổi số (`ridehail.json`).
- Nhân vật đang có: tài xế giữ xe, bằng và đồng phục; ai cũng được một smartphone.

**Mới phải làm:**

| Phần | Làm gì | Độ khó |
|---|---|---|
| Sổ tay | Menu ngoài điện thoại; gắn lại các màn hình app đang có, thêm Sổ mẹo | Nhỏ |
| Thẻ thông tin công trình | Nhãn, thẻ, trang chi tiết; nội dung trong `server/content/` dạng `{vi, en}` | Code nhỏ, nội dung lớn |
| Điện thoại thật | Danh bạ và cuộc gọi NPC, tin nhắn mở nhiệm vụ, chỉ đường, camera và quét QR, giao diện theo đời máy | Vừa |
| Mẹo theo tình huống | `tips.json` với điều kiện (bước xuống lòng đường, lần đầu mua đồ…) | Nhỏ |
| Đồ và cửa hàng | Danh mục đồ (`items.json`: điện thoại, xe, đồ ăn), túi đồ, mua qua lời thoại NPC (server trừ tiền qua sổ cái rồi cấp đồ) | Vừa |
| Tạo nhân vật mới | Xuất phát, màu da, màu tóc, hai chỗ xuất hiện | Vừa |
| Loại objective mới | Mua đồ, giao đồ, đọc thẻ, trả lời đố, chạy qua bài thi | Vừa |
| Khách NPC | NPC đặt xe, ngồi sau, trả tiền, chấm sao | Vừa |
| Tài xế NPC | NPC tự chạy theo đường | Lớn: cần đồ thị đường, lấy từ các con đường trong `scripts/map/saigon_mini_layout.json`; dùng lại được cho xe tự chạy (R4) |
| Sân thi bằng | Model sân tập ở Thủ Thiêm (duyệt design trước) và kiểm tra chạm vạch | Vừa |

## 14. Thứ tự đề xuất

1. **Sổ tay, thẻ thông tin, nội dung 5 công trình đầu, mẹo theo tình huống.** Đây là phần chính của trang web, không phụ thuộc phần nào khác, và làm được trên map hiện tại.
   - **Làm xong 2026-11-19 (chưa deploy):** Sổ tay (nút trên cùng bên phải, phím B; mở được cả ở chế độ xem bản đồ) với 5 mục Cẩm nang, Bản đồ, Nhiệm vụ, Thành tích, Nhân vật; Hồ sơ, Nhiệm vụ, Thành tích, Bản đồ đã rời khỏi điện thoại, app Cẩm nang trong máy chỉ còn phần "Hỏi đường". Thẻ thông tin dưới khung tên địa điểm (phím I mở trang), trang chi tiết có nút "Dẫn đường tới đây" (mũi tên của tracker) hoặc "Bay tới đây" khi đang xem bản đồ, và link Google Maps. Nội dung 5 trang (Chợ Bến Thành, Nhà thờ Đức Bà, Bưu điện, Nhà hát, Dinh Độc Lập) trong `server/content/guide.json`, tra cứu ngày 2026-11-19 và ghi nguồn; chỗ các nguồn nói khác nhau (giá vé Dinh Độc Lập, giờ mở cửa Nhà hát) ghi khoảng hoặc chỉ tới trang chính thức. Trang đã đọc chỉ lưu trên trình duyệt.
   - Còn lại của bước này: mẹo theo tình huống và Sổ mẹo; trang cho 15 công trình còn lại. Cài đặt (ngôn ngữ, âm thanh) vẫn ở hàng nút công cụ.
2. Tạo nhân vật mới, hai chương mở đầu, đồ và cửa hàng, điện thoại thành món đồ thật (danh bạ, tin nhắn, ứng dụng ẩn).
3. Đặt Crab có chỗ thay thế khi không có tài xế: Chú Sáu xe ôm trước, tài xế NPC sau.
4. Việc vặt, thi bằng, mua xe, đăng ký Crab, khách NPC.
5. Các chương sau ([mục 9](#9-sau-chương-3-ý-tưởng-chưa-viết)).

## 15. Cần Khoa chốt

1. ~~Có tách "Dạo phố" và "Sống ở Sài Gòn" ở màn đầu không?~~ Khoa chốt 2026-11-19: không, ai cũng đi dạo được ([mục 3](#3-một-cách-vào-ai-cũng-đi-dạo-được)).
2. Vì sao người nước ngoài không có điện thoại? Rớt vỡ màn hình (đề xuất, nhẹ nhàng); bị giật (nhớ lâu và dạy được mẹo an toàn, nhưng ấn tượng đầu tiên về Sài Gòn lại xấu); hay không cần lý do?
3. Tiền hiển thị là "đồng" với giá thật cho đồ nhỏ (đề xuất), hay giữ "xu"?
4. Xuất phát cố định (đề xuất, bỏ "Đổi vai"), hay vẫn cho đổi?
5. Người nước ngoài có được chạy Crab không? Đề xuất: được, vì đây là game; nghề riêng cho người nước ngoài (hướng dẫn viên, dạy tiếng Anh) để sau.
6. Tên điện thoại Nokla, Ifone (sau đó Samsun, Oppa) được chưa?
7. Làm theo thứ tự ở [mục 14](#14-thứ-tự-đề-xuất), Sổ tay và thẻ thông tin trước?
8. Ứng dụng ẩn: ngoài những app mở theo tiến trình (Crab Driver, Hàng Xóm), có làm app tìm được bằng cách quét mã QR ngoài phố không ([mục 6](#ứng-dụng-ẩn))?
