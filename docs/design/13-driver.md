# Chapter 3: becoming a Crab driver

Step 4 of [10-story.md](10-story.md#14-thứ-tự-đề-xuất) (section 8, chapter 3). Written 2026-11-20 as the design the step
is built from. Khoa decided on 2026-11-19 that nobody picks the Crab driver; it takes the quests of this chapter
(decisions D9, D11, D15, D22 in the [README](README.md)). Prices are those of 10-story.md, section 11.

## The path

```
Chú Sáu kể chuyện ─┐                                                      ┌─ Má's call (newcomer)
Anh Tony's hint ───┴─> chapter.crab ─> licence ─> bike ─> Crab office ─> first trip ┴─ Chú Sáu (native)
```

| Quest | Offered when | Done by | Gives |
|---|---|---|---|
| `story.chu_sau_ke_chuyen` (newcomer), `citizen.chu_sau_ke_chuyen` (native) | newcomer: after `story.gap_chu_sau`; native: at once | the answer in Chú Sáu's "life" line | `chapter.crab` |
| `traveler.tony_goi_y` | foreigner, after the hostel quest | Tony's answer about earning money | `chapter.crab` |
| `driver.thi_bang_a1` | `chapter.crab`, no licence | theory signal, then `pass_test a1` | `vehicle.motorbike.ride` |
| `driver.mua_xe` | licence, no bike | buying `xe_so_110` | |
| `driver.dang_ky_crab` | smartphone, licence, bike, no uniform | buying `dong_phuc_crab` | |
| `driver.nhan_xe`, `driver.thuoc_duong`, `driver.chuyen_dau_tien`, the driver dailies | by capabilities (`requires_caps`), no longer the `driver` tag | as before | as before |
| `story.khoe_voi_ma` (newcomer), `citizen.khoe_voi_chu_sau` (native) | after the first trip | Má's call, or Chú Sáu | Chú Sáu's text "Giỏi! Bữa nào dạy chú xài cái app này với." |

**Foreigners.** They are not pushed, but they can do the chapter: Tony's suggestion is an optional quest after the
hostel (it only opens `chapter.crab`), and from then on the quests are the same ones. The quests need capabilities, not
a nationality, so nothing else differs. Reason: 10-story.md says the chapter is not their main goal but allowed, and a
quest the player can ignore is the least push the quest log offers.

## Capabilities (archetypes.json)

New: `chapter.crab`, `vehicle.motorbike.own` (a bike item), `exam.a1.ticket`, `exam.a1.retake`, `exam.a1.theory` (the
test's own marks). Used as they were: `vehicle.motorbike.ride`, `uniform.crab`, `driver.app`, `ridehail.drive`. No
archetype grants any of them. Say lines may now be keyed by a capability (`"ridehail.drive"`) as well as an archetype,
the archetype's own line first: the old "driver" greetings of the NPCs became the line of whoever has `ridehail.drive`.

## Fees (items.json, `fees`)

Money paid each time to an NPC for something not kept. A fee has an NPC, an `amount`, `grants` (capabilities, a ticket),
`requires_caps` and `unless_caps`. A dialogue line with `"fee": "thi_a1"` makes the game show a "pay" button; the button
is the message `pay` (`{"t":"pay","rid":1,"fee":"thi_a1"}`), never an answer: a dialogue moves no money. The server
checks the player stands next to the NPC (`far`), the fee exists (`target`), `unless_caps` (`owned`), `requires_caps`
(`locked`) and the wallet (`balance`), then in one transaction takes the amount, writes the ledger line (reason `fee`),
and stores the grants (source `fee:<id>`). It answers `wallet`, then `result`, and publishes `fee.paid` (objective
`pay_fee`; it is implemented and tested but the chapter's quests do not use it: a quest that waits for a fee is
soft-locked when the player paid before it was offered). A fee whose capabilities the player already holds is refused
(`owned`) and charges nothing, so two presses in flight charge once.

Two fees: `thi_a1` (200.000đ; not with a ticket, a retake mark or a licence) and `thi_lai_a1` (50.000đ; only with the
retake mark and no ticket). So the server, not the dialogue, decides who pays the retake price.

## The licence test (`driving.json`, `server/internal/driving`)

Thầy Mười stands on the pavement of the Ba Son bridge road in Thủ Thiêm. The attempt is a chain of capabilities that the
module adds and spends; the dialogue only offers answers by them (`requires` and the new `unless` on an answer):

| Step | Signal (sent by the dialogue) | Effect |
|---|---|---|
| fee paid | | `exam.a1.ticket` |
| the quiz starts | `thi_bat_dau`, sent on arriving at question 1 | ticket spent, `exam.a1.retake` given (any attempt makes the next fee the retake one) |
| the quiz is passed | `ly_thuyet_dau`, sent on arriving at the pass node | `exam.a1.theory` and a ticket for the practical |
| the practical starts | `thuc_hanh_bat_dau` | ticket spent, the school's bike lent |
| the route is ridden | | event `test.passed`: the quest gives `vehicle.motorbike.ride`; theory and retake marks go |

A signal is sent when a node is *arrived at* (new `emit` on a node), not by the answer that leaves it, so closing the
conversation cannot dodge a result. The ticket is spent when the quiz starts, so walking out mid-quiz costs the fee.

**Theory.** Ten questions in `thay_muoi.json`, three wrong answers fail at once (8 right of 10 pass). The tally lives
in the conversation, not in the client: an answer may add to a named mark (`"mark": "wrong"`), `goto_if` sends the
conversation on when a mark reaches a count, and a node may reset the marks (`reset_marks`), so a retake starts clean.
After each answer Thầy Mười says whether it was right and the rule. Facts (Luật Trật tự, an toàn giao thông đường bộ
2024, số 36/2024/QH15, and Nghị định 168/2024/NĐ-CP, both in force since 2025-01-01):

1. Red light: stop before the stop line; a motorbike running it is fined 4 to 6 million đồng (Nghị định 168/2024,
   Điều 7 khoản 7: [lsvn.vn](https://lsvn.vn/nhung-loi-vi-pham-giao-thong-thuong-gap-va-muc-phat-moi-ap-dung-tu-nam-2025-a152122.html),
   [pbgdpl.cantho.gov.vn](https://pbgdpl.cantho.gov.vn/dieu-khien-xe-may-vuot-den-do-di-tren-via-he-bi-phat-tu-4-6-trieu-dong)).
2. Helmet: rider and passenger, strap fastened; the rider is fined 400 to 600 thousand (same sources, and
   [thuviennhadat.vn](https://thuviennhadat.vn/phap-luat/nghi-dinh-168-doi-voi-xe-may-10-muc-phat-can-biet-632712.html)).
3. Alcohol: none at all for a driver (Luật 36/2024/QH15; [vnreview.vn](https://vnreview.vn/threads/2-diem-dang-chu-y-cua-luat-trat-tu-an-toan-giao-thong-duong-bo-2024-cam-hoan-toan-lai-xe-khi-co-nong-do-con.43684/latest),
   [luatvietnam.vn](https://luatvietnam.vn/an-ninh-trat-tu/luat-trat-tu-an-toan-giao-thong-duong-bo-2024-so-36-2024-qh15-360844-d1.html)).
4. A round sign with a red border prohibits; 5. a red-bordered yellow triangle warns of danger (the sign system, QCVN 41
   on road signs: the text carries no amounts and nothing that changed in 2025).
6. At an intersection with no lights or signs give way to the vehicle on your right (the road traffic rules of the Luật).
7. A flashing yellow light: go, slowly, give way to pedestrians.
8. No phone in hand while driving; 9. no riding on the pavement.
10. A 110 cc bike (50 to under 125 cc) needs the class A1 licence.

Only questions 1 to 3 carry amounts or a named article; 4 to 10 are the standing rules and need no number. The sources
are secondary (law-news sites); the Luật and the Nghị định themselves are on vbpl.vn and thuvienphapluat.vn, to be
checked against before the amounts change.

**Practical.** There is no driving-yard model and none is made. Thầy Mười lends the school's bike (`xe_tap_lai`, a
vehicle only a holder of `exam.a1.theory` may mount: `requires` is now checked on the server when mounting) and the
player rides about 165 m of Thủ Thiêm's real road through eight checkpoints (`driving.json`, Blender XY, each 6 m
wide, checked against the collider with a 0.5 m body) within 90 s and without getting off. The server follows the rider
every tick: the clock starts when they mount (the bike waits 90 s), the next checkpoint only counts in order, getting
off, time or never mounting ends it. It tells the game (`drive_test`: lent, run with the next checkpoint and the
seconds left, pass, fail with a reason); the game shows each checkpoint in the quest tracker. A failed practical keeps
the theory pass: the next try costs the retake fee. **The real yard** (the figure 8, the narrow straight line, not
touching the lines, no foot down, as 10-story.md describes) replaces `checkpoints` with the yard's lines once Khoa
approves its model: the module only needs "a lent bike, a clock and a pass or fail", so the yard becomes a
different test of those, with the same capabilities, fee, bike and signals. The streets route is then dropped.

## The bike and the shop (items.json)

A bike is an item of kind `bike` with a `vehicle`. `xe_so_110` (1.000.000đ) grants `vehicle.motorbike.own`. A player
who owns one has it parked beside them on every arrival (`ws.arrive` reads the items), and the world keeps it through
archetype changes; buying one mid-visit parks it beside the buyer. The archetype's starter `vehicle` is gone. The
bike stands there without a licence; mounting needs `vehicle.motorbike.ride` (the vehicle's `requires`, now enforced
on the server).

**Bargaining.** A shop may have a `haggle`: a dialogue signal and a percent. Chú Tám's `xe_chu_tam` has
`phrase_mac_qua`, the signal that Cô Ba's lesson sends and the Sổ sưu tầm collects, at 10%. The answer "Mắc quá chú!"
sends it; the shop module (it hears signals as a `world.SignalHook`) notes the bargain for that player and shop and
tells the game (`deal`) so the sheet shows the lower price. The price is computed by the server when the player
buys, from that note, and the note ends with the purchase or when the player leaves. The client never sends a price.

**Prerequisites.** An item may have `requires`: capabilities the buyer must hold, else `locked` and nothing is kept.
`dong_phuc_crab` (300.000đ, grants `uniform.crab`, `driver.app`, `ridehail.drive`) requires `phone.smart`, the licence
and a bike. Chị Vy's dialogue checks the same three and says which is missing.

## The driver archetype goes (migration 0012)

`driver` is removed from archetypes.json with `job`, the starter `vehicle` and the picker and server code for jobs. A
character stores an archetype id: one the server no longer knows plays as what its hello asks for and the store keeps
that. Migration 0012 gives every stored driver the bike item, the uniform item, `vehicle.motorbike.ride` (source
`migration:driver`), an Ifone X if it owns no phone (a driver made after migration 0009 had none), and makes it a
`citizen`. Why `citizen` and not `newcomer`: a driver has been working the streets, so Má's call, the guesthouse and
the first-day quests would be wrong for them; a native has the street quests and the neighbourhood tags, and the
driver quests they finished stay done. Their uniform look stays, because they own the uniform.

## Messages

Client to server: `pay`. Server to client: `deal`, `drive_test`; the `interaction` line gets `fee`. Result codes: `locked`
is new. See `server/README.md`.

## Cast

| NPC | Where | Why there |
|---|---|---|
| Thầy Mười | Thủ Thiêm, pavement of the Ba Son bridge road, Blender (146, 172) | the road the test rides is beside him |
| Chú Tám | Chợ Lớn, beside the bus station, (-172, -78) | Chợ Lớn is where the newcomer arrives |
| Chị Vy | Hàm Nghi, the bike stand, (18, 18.2) | where the driver stand was |

Each stands where the collider says a body fits (the game's `fits`), checked in the game.
