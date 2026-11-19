# Chapter 2: odd jobs

Step 4 of [10-story.md](10-story.md#14-thứ-tự-đề-xuất) (the odd jobs of section 7), after the phone
([12-phone.md](12-phone.md)). Written 2026-11-20. Four jobs players can repeat, one story quest that opens the
coffee run for the Vietnamese origins, and Anh Tony's line for foreigners. Money targets are in
[10-story.md section 11](10-story.md#11-tiền).

## The jobs

Each is a quest in `quests.json` with `"job": true`, which is a daily (`"repeat": "daily"`, D16) with three
differences: it has no tag and no pool (every character with a phone is offered all four), it may pay up to
`XuJobRewardMax` (40.000đ) where other dailies pay up to 20.000đ, and it does not count toward the streak of dailies
(the streak is about coming back to the daily routine, not about earning). All four need `phone.basic` (any phone,
a Nokla too): chapter 2 starts after the phone of chapter 1, so the tracker is not crowded before a player has one,
and it is how a job reaches a player in the real world. Quests only suggest: the tracker follows the first quest of
the file's order, so story quests and dailies come first and a job shows only when nothing else is running.

| Job | Quest id | Given by | Objective | Pay | Time |
|---|---|---|---|---|---|
| Bưng cà phê | `job.bung_ca_phe` | Chú Tư, Lê Lai | Three coffees to customers at the Quách Thị Trang roundabout, Hàm Nghi, Lê Lợi (in that order) | 40.000đ | 120 s |
| Giao bánh tráng | `job.giao_banh_trang` | Dì Năm, Hồ Con Rùa | Công viên 30/4, then Nhà thờ Đức Bà | 30.000đ | 240 s |
| Phát tờ rơi | `job.phat_to_roi` | Chị Hạnh, Lam Sơn | A flyer to each of five people strolling on Nguyễn Huệ (any order) | 30.000đ | none |
| Khuân hàng | `job.khuan_hang` | Cô Ba, Bến Thành | Three parcels from Anh Hai's truck to Cô Ba's stall | 40.000đ | none |

"Chụp ảnh giùm" waits for a camera, and "Chỉ đường" is Thổ địa and Hỏi và đáp (D18).

### The daily limit

**Each job once a day.** A job is a daily, so a character has one record per job and day: when it is done it stays
done until midnight in Saigon, Chú Tư stops offering it (the answer that takes it is offered only while the quest
runs, `requires_quest`), and tomorrow it starts again. A failed run is not done, so it can be taken again at once.
Four jobs a day pay at most 140.000đ (a test keeps a day's total at or under 150.000đ). With the story quests
(about 1.000.000đ for chapters 0 to 2) and the quizzes, the 12 to 15 jobs section 11 asks for are three or four days
of playing a few minutes a day, and they cannot be farmed: no matter how long someone stays, a day has a ceiling
of four jobs. Each giver says so in the answer that explains the job ("mỗi người một chuyến một ngày").

### The reward cap

`XuRewardMax` (20.000đ) keeps what dailies and achievements pay "well under a day's top-up" (the lì xì refills
to 50.000đ). Jobs are the one thing that may break it, on purpose: they are real work of two to four minutes, the
section 11 table puts them at 20.000 to 40.000đ, and they are limited to four a day. So the cap stays at 20.000đ for
dailies and achievements, and `XuJobRewardMax = 40.000đ` is a second cap that applies only to quests with
`"job": true` (checked at load: a job must be a daily, with no pool and no tags). Story quests done once keep
`XuStoryRewardMax` (200.000đ).

## The errand objective

`server/internal/quest/objective/errand.go`, one generic type. Params:

```json
{ "npc": "chu_tu", "signal": "nhan_viec_bung_ca_phe", "seconds": 120, "order": "fixed",
  "stops": [{ "place": "bung_binh_quach_thi_trang" }, { "place": "duong_ham_nghi" }, { "place": "duong_le_loi" }] }
```

- The job is **taken** by the dialogue answer that sends `signal` in a conversation with `npc`. That starts the clock
  (`Progress.T`, unix milliseconds, set from the event's `At`, which the engine stamps). Stops mean nothing before it.
- A **stop** is a place (standing in it), an NPC (starting a conversation with them) or an NPC with a signal (picking
  that answer: handing over a flyer, a parcel). `order: "fixed"` (the default) wants them one after the other and may
  repeat a stop (truck, stall, truck, stall, truck, stall); `"free"` takes them in any order, each once. `count` is
  the number of stops (checked).
- **The time limit** (`seconds`, 30 to 1800, or none): the server has no timer for it. When the next thing the player
  does for the job arrives (a stop, or taking it again) and the time is over, the run is wiped (`Progress{}`: not
  taken, nothing counted; a stop reached too late counts for nothing), and the job waits to be taken again.
  Taking it again while the run is still on changes nothing (no more time for asking twice). The wire has what the
  game needs to show it: `QuestState.left` (seconds left when the progress was sent; `-1` for no limit; missing when
  not taken), and the game counts down from when it got the message. When the clock reads zero the game says "out of
  time" by itself and the next action makes it true on the server.
- The matcher also implements `objective.Runner` (`Left`), which the wire uses; other types do not need it.
- Content checks (every one also run by the engine's tests over the shipped file): the taker NPC has an answer that
  sends the signal, every stop exists, a stop's signal exists, `count` equals the stops, free order has no repeated
  stop, `seconds` is in range.

The game (`src/game/jobs.ts`) words it: not taken ("Nhận việc: nói chuyện với Chú Tư", the arrow points to the
giver), running (the objective's text, `1 / 3`, the clock), out of time. The tracker arrow points to the next stop
(fixed order) or the street (free). Taking a job makes the game follow it. The Sổ tay's quest list shows the jobs
in their own section, "Việc vặt", apart from the dailies and their streak.

## Dialogue and NPCs

Every giver has an answer "X có việc gì cho con làm không?" that is always there and explains the job and the daily
limit; the answer that takes the job (and the others the job uses) appears only while the quest runs. Chú Tư's
greeting had six answers (the limit) so "Cà phê sữa đá là sao chú?" and "Quán mở lâu chưa chú?" moved under one new
answer, "Con muốn hỏi chú vài chuyện". New NPCs: **Anh Hai** (the truck, Bến Thành) and five people strolling on
Nguyễn Huệ (Anh Khôi, Chị Lan, Cô Hoa, Bạn Minh, Chú Hùng, one shared dialogue `nguoi_di_duong`). Players can also be
handed a flyer in the future; for now it is NPCs only.

## Who may take them

Everyone, Vietnamese or foreign, with a phone: jobs are the way anybody earns in chapter 2 (D22: a foreigner may take
the Crab job too), and a foreigner who stays a few days needs money for the Sổ sưu tầm's food and the Crab. They are
offered as soon as the phone is bought (the same moment the other quests that need a phone open), never forced.

## "Xin việc quán Chú Tư" (story quest, once)

`story.xin_viec_chu_tu`: for `citizen` (both Vietnamese origins), needs a phone, pays 100.000đ and the title "Phục vụ
quán Chú Tư". An errand with a 300 s shift: take it from Chú Tư ("Dạ chú, cho con xin làm ca sáng"), carry coffee to
23/9 Park and Bùi Viện, report back to Chú Tư. His line is the one of the story: "Bưng cho vững, ly của khách quen
đó, đổ là chú trừ lương!". Chú Tư texts when it is done ("Ngày nào rảnh cứ ghé nhận một chuyến"), which is how the
coffee run opens for a player from the story's side. The coffee run itself stays open to foreigners too, and
`citizen.ca_phe_sang` and `citizen.chuyen_khu_pho` are untouched (the story quest does not require or replace them).

## Anh Tony

Once the foreigner's chapter 0 is done (`traveler.an_nhu_nguoi_sai_gon` now also grants the capability
`traveler.settled`, used only to gate this), his greeting has one more answer ("Em muốn ở Sài Gòn lâu. Anh có lời
khuyên gì không?"): "Muốn ở lâu hả? Phải kiếm tiền! Đi chạy Crab đi, Sài Gòn ai cũng chạy", and the odd jobs for
those who have no licence or bike yet. No quest. Foreigners who finished chapter 0 before this change do not have the
capability (nothing grants it afterwards).

## Not here

Chapter 3 (licence, bike, Crab office, driver archetype), NPC passengers, items and the shop, the phone code.
