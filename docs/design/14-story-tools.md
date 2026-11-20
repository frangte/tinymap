# Xưởng truyện và tình tiết bất ngờ (đề xuất 2026-11-20, chờ Khoa chốt)

## Ý của Khoa

> "các câu chuyện bây giờ vẫn rất dễ đoán, tao cần một UI hay flow story có thể design trên đó luôn. m nghĩ thử xem
> làm cách nào để dễ cho tao design story. tao muốn thêm nhiều yếu tố tình tiết kiểu ko nhặt được mồm ấy"

Hai việc: (1) engine cho phép truyện có twist, rẽ nhánh, hậu quả; (2) một giao diện để Khoa tự design truyện trên đó.

## Vì sao truyện hiện dễ đoán

- **Lựa chọn không để lại gì.** Engine chỉ nhớ ba thứ: quyền (capabilities, `archetypes.json`, cấp bởi đồ, phí và
  thưởng quest), quest đã xong, và `Marks` (điểm tạm trong một cuộc nói chuyện, `server/internal/dialogue`, mất khi
  nói xong). Một câu trả lời chỉ gửi được tín hiệu (`emit`) để xong một mục tiêu; nó không ghi lại được "người chơi đã
  chọn gì", nên truyện sau đó vẫn đi một đường.
- **Quest nào cũng thẳng một mạch:** `requires` → mục tiêu theo thứ tự → thưởng. Có mũi tên (`at`) chỉ tới đích, không
  có thất bại, không có kết cục khác.
- **NPC luôn nói thật**, nói gì thì đúng y vậy.
- **Một chương nằm rải trong 3–4 file JSON** (`quests.json`, `dialogue/*.json`, `contacts.json`, `items.json`), nên
  khó nhìn thấy cả mạch truyện.

## Phần 1: engine thêm 6 thứ để có tình tiết

1. **Cờ truyện.** Câu trả lời, nút thoại hay quest xong ghi một cờ cho nhân vật (`set: {"tra_xe_chu_sau": true}`, hoặc
   số: `than_chu_sau: +1`), lưu trong DB (bảng mới, theo nhân vật). Lời thoại, lựa chọn, quest và tin nhắn đọc cờ
   (`if`) để đổi theo. Là nền cho mọi thứ dưới đây. Câu thoại hiện đã có biến thể theo xuất phát và capability
   (`say` theo key); cờ là thêm một loại key nữa.
2. **Nhiều kết cục.** Một quest kết thúc theo một trong vài kết cục (`outcomes`); quest sau cần một kết cục cụ thể
   (`requires: ["chuong_4.chiec_xe:tra_xe"]`) thay vì chỉ cần quest đã xong.
3. **Sự kiện trễ.** "Khi X xảy ra, sau N cuốc / N việc / qua ngày mới (giờ Sài Gòn) thì: một tin nhắn, cuộc gọi từ số
   lạ, NPC xuất hiện chỗ khác." Chuyện cũ quay lại lúc người chơi đã quên. Chạy trong quest service như các mục tiêu,
   không cần đồng hồ riêng của server (như việc vặt: hết giờ thì lần làm tiếp theo mới xóa).
4. **Manh mối trong Sổ tay.** Gom những điều nghe được (một tab mới, như Sổ sưu tầm). Có lựa chọn chỉ hiện khi đang giữ
   đúng manh mối (kiểu "đưa bằng chứng ra").
5. **Biến thể riêng mỗi người.** Truyện bốc A hoặc B một lần cho mỗi nhân vật (theo id nhân vật, cố định), nên người
   chơi trước không spoil được người sau.
6. **Quest ẩn.** Không có mũi tên, tên là "???" cho tới khi phát hiện.

Đề xuất làm trước: 1, 2, 3.

### Đã làm (2026-11-20): cờ truyện và nhiều kết cục

Cơ chế 1 và 2 đã có engine (chưa có truyện nào dùng, test dùng fixture `server/internal/quest/testdata/branching_story.json`);
chi tiết, định dạng và cách viết một nhánh: [15-flags-outcomes.md](15-flags-outcomes.md) (tiếng Anh). Tóm tắt: quest có
`outcomes` (2 đến 6 kết cục, mỗi cái là một mục tiêu có `title`, `flags`, `rewards`); khi xong các mục tiêu thì mọi kết cục
cùng mở, cái nào xong trước thì kết thúc quest và đặt cờ `<id quest>:<id kết cục>`. Quest có `requires_flags` và `unless_flags`;
câu trả lời có `if_flags` và `unless_flags`; câu thoại có biến thể `"flag:<cờ>"`. Cờ lưu trong bảng `story_flags` cùng giao dịch
với việc hoàn thành quest. Chưa làm: cờ có giá trị (số), sự kiện trễ, manh mối, biến thể riêng, quest ẩn.

### Ví dụ twist dùng nhân vật có sẵn (mẫu, Khoa là tác giả)

- **Chiếc xe của Chú Tám.** Chú Sáu nhìn xe mình mới mua rồi im lặng: đó là chiếc xe ông bị mất trộm mấy năm trước.
  Câu "xe zin, chục năm chưa hư" và "Đừng nói với ai nghen!" của Chú Tám (`dialogue/chu_tam.json`) đã có sẵn từ đầu,
  giờ mới hiểu nghĩa. Lựa chọn: trả xe, giữ xe, hay đi hỏi Chú Tám; mỗi kết cục mở một chương 5 khác. Dùng 1, 2, 4.
- **Tiền của Má.** Má nhắn nhầm vào máy mình: "Anh Sáu ơi, tháng này em chưa gửi tiền cho thằng nhỏ được, anh ứng giùm
  em…". Hóa ra 100.000đ lúc đầu (`story.goi_ve_cho_ma`) là tiền của Chú Sáu. Dùng 3.
- **Vị khách nửa đêm.** Khách NPC đi Bùi Viện lúc khuya, bỏ quên gói đồ trên xe. Mở ra, trả lại, hay báo công an? Gói
  đồ vô hại (quà sinh nhật) hay mờ ám là do mục 5 bốc cho từng người. Dùng 1, 2, 5.

## Phần 2: Xưởng truyện, trang design trong repo

Chạy cùng `npm run dev` (một trang riêng, chỉ trên máy dev, không vào bản production), đọc và ghi thẳng vào
`server/content/*.json`, nên thay đổi xem được bằng git diff và server đọc như bây giờ.

- **Bản đồ truyện:** chương là cột, quest là thẻ, mũi tên là "cần quest nào xong" hoặc "kết cục nào dẫn đi đâu", màu theo
  xuất phát. Nhìn một lần thấy đoạn nào thẳng tuột, đoạn nào rẽ nhánh.
- **Bấm vào quest hay NPC** thì thấy lời thoại dạng sơ đồ (ô lời nói, mũi tên lựa chọn). Sửa chữ vi/en ngay trên ô, kéo
  mũi tên để nối lại, gắn cờ, manh mối, kết cục bằng menu.
- **Bảng cờ và manh mối:** mỗi cờ bật ở đâu, đọc ở đâu.
- **Kiểm lỗi:** đánh dấu đỏ chỗ nối hỏng, cờ được đọc mà không ai bật, nút không tới được, câu thiếu tiếng Anh (cùng
  luật với kiểm tra nội dung của server, `server/content/*_test.go`).
- **Chơi thử từ đây:** chọn một điểm trong truyện, bật sẵn các cờ, mở game với một nhân vật test ở đúng trạng thái đó.

Không dùng Twine, Ink hay articy: chúng không biết NPC, địa điểm, đồ trong game, phải chuyển qua lại và mất thông tin.

## Thứ tự đề xuất

1. **Bản đồ truyện chỉ xem**, vẽ từ JSON hiện có. Nhỏ, làm nhanh, thấy ngay chỗ truyện thẳng.
   - **Đã làm** (`tools/story/`, chạy bằng `npm run story`, mở http://localhost:5173/tools/story/): bản đồ các chương (cột là chương, thẻ là quest, mũi tên `requires`, quyền, cờ và kết cục), lọc "Xem như" theo xuất phát, tìm quest, bật vi/en; bấm quest ra bảng chi tiết (mục tiêu, và ô thoại nào gửi tín hiệu mà mục tiêu đang chờ); sơ đồ lời thoại từng NPC (kéo, phóng to thu nhỏ); bảng Vấn đề; bảng Cờ. Chỉ xem, chưa ghi vào JSON. Thêm `?fixture` để xem mẫu có kết cục và cờ (`tools/story/fixture.json`). Tầng mô hình (`model.ts`, `dialogues.ts`, `problems.ts`) tách khỏi phần vẽ để bước 3 dùng lại khi sửa.
2. **Engine:** cờ, kết cục, sự kiện trễ, manh mối, có test Go. Làm song song với bước 1.
3. **Xưởng truyện sửa được**, có kiểm lỗi và chơi thử. Phần lớn nhất.
   - **Đã làm (sửa, lưu, kiểm lỗi; chưa có chơi thử):** `npm run story` giờ sửa được. Sơ đồ lời thoại sửa ngay trên ô: câu thoại vi/en và các biến thể (xuất phát, quyền, `flag:<cờ>`), thêm/xóa ô (xóa thì báo những câu trả lời đang nối tới), đặt ô bắt đầu và ô gọi điện, thêm/xóa/đổi thứ tự câu trả lời, kéo dấu ● của câu trả lời tới một ô (hay vào khung "kết thúc") để nối, và các ô điều kiện (tín hiệu, quyền, quest, cờ, điểm). Quest sửa trong bảng bên phải: tên, mô tả, điều kiện (requires, cờ, tags, quyền), mục tiêu (loại và tham số theo từng loại; việc vặt `errand` sửa bằng JSON), kết cục, phần thưởng, tin nhắn; thêm quest vào một chương (ô nhập id cuối cột) và xóa quest (liệt kê nơi nhắc tới trước). Hoàn tác/làm lại (Ctrl/Cmd+Z, Shift+Z), Lưu (Ctrl/Cmd+S), dấu "chưa lưu", cảnh báo khi rời trang.
   - **Ghi thẳng `server/content/*.json`** qua plugin Vite chỉ chạy lúc dev (`tools/story/vite-plugin.ts`, `/__story/load|save|check`): chỉ máy này gọi được (từ chối địa chỉ khác, Origin khác, thiếu header), chỉ ghi `server/content/*.json` và `dialogue/<id>.json`, và từ chối ghi đè file đã đổi trên đĩa từ lúc tải (có nút tải lại). Bộ ghi JSON (`tools/story/format.ts`) nhớ file nào gõ gọn một dòng, file nào xuống dòng, nên lưu không sửa gì thì git diff rỗng và sửa nhỏ thì diff nhỏ; `npm run story:format-check` kiểm tra điều đó. Không có trường nào mà server không biết được ghi ra (bố cục tự động, không lưu vị trí ô).
   - **Kiểm lỗi:** `problems.ts` + `rules.ts` (luật của `quests.go`, `flags.go`, `cast.go` mà trang tự kiểm được) chạy lại sau mỗi sửa, thẻ quest và ô thoại sai thì viền đỏ. Ngoài ra bản nháp được đưa cho chính server kiểm (`server/cmd/contentcheck`, dùng `content.LoadDir` và `quest.NewEngine`, trên bản sao của `server/content` có bản nháp phủ lên); lỗi hiện ở thanh trên và ở tab Vấn đề, gắn vào quest hay ô thoại khi lời lỗi nói rõ. Lưu khi server từ chối thì hỏi lại ("server sẽ không khởi động được").
   - **Chưa làm:** chơi thử (agent khác làm), file lời thoại mới cho NPC mới, kéo từ thẻ quest này sang thẻ khác để thêm `requires`, sửa danh hiệu/tem/achievement, sửa tên ô, lưu vị trí ô.
4. Viết thử chương "Chiếc xe của Chú Tám" bằng Xưởng truyện để thử công cụ; sau đó Khoa viết tiếp.

## Cần Khoa chốt

1. **Kiểu design:** kéo thả sơ đồ (đề xuất), gõ kịch bản dạng chữ rồi trang tự vẽ sơ đồ, hay cả hai?
2. **Cơ chế:** làm hết 6, hay chọn vài cái trước (đề xuất 1, 2, 3)?
3. **Tông truyện:** ba twist mẫu có hợp không, hay tông khác (hài, cảm động, noir)?
4. Có muốn một mockup Xưởng truyện bấm thử được trước khi code thật không?

## Chơi thử (đã làm)

Chọn một quest trong Xưởng truyện, bấm "Chơi thử", game mở ở tab mới với một nhân vật thử đã ở đúng trạng thái trước quest đó, khỏi chơi lại hàng giờ.

**Chạy:** `SG_DEV_TOOLS=1 npm run server` (có thể thêm `DB_PATH=/tmp/test.db` để khỏi đụng DB dev) và `npm run story`; mở tab "Chơi thử" (hoặc link "Chơi thử từ đây" ở bảng chi tiết của quest), chọn quest và bấm nút. Không đặt `SG_DEV_TOOLS` thì route không tồn tại (404).

**Trang tự điền:** xuất phát; các quest phải xong (`requires`, cộng quest đặt các cờ trong `requires_flags`), mỗi quest có kết cục thì chọn được kết cục (mặc định là cái thỏa điều kiện của quest đã chọn); đồ cần để có quyền mà các quest đọc (`requires_caps`: điện thoại, xe, đồng phục); cờ thêm; tiền thêm; chỗ đứng (đích đầu tiên của quest, thường là NPC giao quest).

**Server (`server/internal/playtest`, route `server/internal/web/playtest.go`):** `POST /api/dev/playtest` tạo một tài khoản mới và nhân vật tên `Thử: …`, không bao giờ đụng tài khoản có sẵn. Quest "xong" đi qua `quest.Engine.Complete`: cùng đường code với lúc chơi thật (bản ghi và kết cục, cờ `<quest>:<kết cục>`, `rewards.flags`, thưởng xu/danh hiệu/tem/quyền, tin nhắn, và quest hay thành tích mở ra sau đó), nên chơi thử không lệch luật; test Go so sánh với chơi thật bằng sự kiện. Đồ và tiền ghi bằng đúng các dòng mua đồ sẽ ghi (giá 0). Quest chưa mở được (thiếu quyền, cờ) thì thử lại sau các quest khác, hết lượt mà vẫn không mở thì từ chối và nói thiếu gì. Từ chối: id lạ, kết cục không thuộc quest, quest lặp hằng ngày, quest thiếu `requires`.

**Cổng an toàn:** tắt mặc định; chỉ nhận kết nối từ loopback và không có header `X-Forwarded-*` (nginx cùng máy cũng bị chặn); Vite dev (nghe cả LAN) chặn `/api/dev/` với máy khác bằng middleware chỉ chạy khi dev (`vite.config.ts`); Origin lạ bị chặn như các API khác.

**Client:** `/?playtest=<token>&ptarch=…&ptname=…&ptlook=…` (giữ `lang`, `debug`). Tab đó thay `localStorage` bằng bộ nhớ (token và hồ sơ test), nên nhân vật đã lưu của người chơi không bị đọc hay ghi đè; có nhãn "NHÂN VẬT THỬ" (`src/game/i18n/playtest.ts`).

**Chưa làm:** quyền `exam.a1.*` (cấp bởi phí thi, không có item) không tự điền được; tin nhắn của quest xong nằm chờ trong Tin nhắn như lúc chơi thật; quest mà server chưa có (ví dụ trong `?fixture`) bị server từ chối.
