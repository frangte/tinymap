# Gameplay design

## Core loop

```
Explore → notice something (a landmark toast, an NPC, the phone buzzing, another player) → interact
   → progress (a quest step, a stamp, reputation) → reward (title, outfit, unlock)
   → new places, people and options → explore again, more and more often with others
```

## Why the archetypes need each other

The archetypes are designed so each one needs the others:

- **Travelers** bring purpose: they need help.
- **Citizens** bring local knowledge.
- **Crab Bike drivers** bring mobility.

Each one's progress feeds off the others. That's what makes one shared world worth more than three separate modes.

## Onboarding (the first 3 minutes)

1. The existing menu, then **"Vào phố"**, then a **"Bạn là ai?"** screen: three cards, each with one sentence and one picture of what you'll do. Then a name, then the existing dressing panel, pre-filled with the archetype's starter look:
   - Traveler: cap and backpack.
   - Citizen: áo bà ba.
   - Crab Bike driver: the **yellow Crab Bike uniform**, a yellow jacket and a yellow helmet. These are new outfit and hat ids; see [05-frontend.md](05-frontend.md#crab-bike-assets).
2. Each archetype spawns somewhere different, all on places that already exist in `src/game/places.ts`:
   - Traveler: Bùng binh Quách Thị Trang, as if just dropped off.
   - Citizen: Dãy nhà phố on Lê Lai (home).
   - Crab Bike driver: beside their motorbike on Hàm Nghi.
3. **The phone buzzes.** That one notification teaches the phone, the quest tracker and the first goal at the same time. Movement tips sit on top of the screen until the first objective is done.
4. After the first achievement, offer to link a real account so the progress isn't lost.

## The archetypes: why you'd pick each

| | **Traveler: Du khách** | **Citizen: Người Sài Gòn** | **Crab Bike driver: Tài xế Crab Bike** |
|---|---|---|---|
| Fantasy | "I'm discovering Saigon" | "This is my neighbourhood" | "I know every street" |
| Appeals to | Story and discovery players; first-timers | Social players; people who want to hang out and help | Players who want speed and mastery |
| Unique hook | A guided questline and a stamp book (Sổ tem) leading to the certificate | Local secrets (hidden spots) they can *show* to Travelers; neighbourhood reputation | The fastest movement, route mastery, and later rides with real passengers |
| Starter apps | Tourist guide (Cẩm nang) | Neighbours board (Hàng Xóm) | Crab Driver (inactive until ride-hailing) |
| Progress track | Discovery % (Khám phá) | Reputation (Uy tín khu phố) | Driving rank (Tay lái) |
| Needs others for | Directions, secrets, rides | Someone to help (that's where reputation comes from) | Passengers (later) |

Citizens are not the "plain" option:

- They are the only ones who can unlock local secrets for others.
- Their reputation shows next to their name in chat.
- They get Vietnamese-first NPC lines with real slang.

## Example questlines

These are for shaping and testing the quest engine, not final content.

### Traveler: "Officially Recognized as a Saigon Local" (6 quests)

1. **Chào Sài Gòn:** Cô Ba at Bến Thành's south gate teaches 3 phrases (a dialogue quiz).
2. **Một ly cà phê sữa đá:** order at Chú Tư's café on Lê Lai, sit on a plastic stool for 5 s, drink.
3. **Ba công trình thời Pháp:** visit Nhà thờ Đức Bà, the Post Office and the Opera House, in any order, staying 2 s at each. Each one gives a stamp.
4. **Ăn vặt Hồ Con Rùa:** buy bánh tráng trộn from the street vendor and eat it by the lake.
5. **Hỏi đường người Sài Gòn:** get directions from a Citizen *player*. If no Citizen accepts within 3 minutes, the NPC motorbike driver at the roundabout does it.
6. **Bài kiểm tra cuối:** a 5-question quiz with Cô Ba about what you saw.
   - Rewards: the title **"Người Sài Gòn chính hiệu"**, a badge, an áo bà ba outfit, and the `local.certified` capability, which unlocks a hidden spot.

### Citizen

- **Sáng ở phố** (daily): morning coffee at Chú Tư's, then greet 2 people nearby.
- **Thổ địa** (repeatable): help a Traveler. Both players progress, and the Citizen gains reputation.
- **Chuyện khu phố:** talk to 3 neighbours to learn local secrets you can share later.
- **Tập thể dục:** exercise (an emote) at Công viên 23/9.

### Crab Bike driver

- **Nhận xe:** get on your bike (the riding tutorial).
- **Thuộc đường Quận 1:** ride through checkpoints along Lê Lợi, then Đồng Khởi, then Hàm Nghi.
- **Chuyến đầu tiên** (future, with ride-hailing): complete a first trip with a passenger.

## Progression and rewards

- **No global level grind** in the MVP. Progress is quests, collections (stamps, foods, photos), achievements, titles, and each archetype's own track.
- **Rewards:**
  - Titles (shown in chat and name tags).
  - Badges.
  - Outfits (these become owned items).
  - Capabilities (hidden spots, special emotes).
  - Collectibles.
- **Achievements** are passive quests: the same engine, without an accept step. For example "Visit all 9 landmarks", "Greet 20 players", "Ride 5 km".

## Retention

- Rotating daily quests.
- Events tied to the real calendar: a Trung Thu lantern night on Nguyễn Huệ, Tết.
- A day/night cycle and the 4 p.m. rain, run by the server: cheap to build, big on atmosphere.
- Later: friends and groups, then housing and the economy as long-term goals.

## Language

- **The UI language is a per-player setting** (`vi` by default, `en` available), not tied to the archetype. A Vietnamese player may want to play a Traveler; a foreigner may want to be a Citizen.
- The archetype changes the *tone* of NPC lines, not their language. To a Traveler: "Mới tới Sài Gòn hả con?"; to a Citizen, neighbour talk. Citizen-facing lines are Vietnamese-first with slang, which covers the brief's "Vietnamese by default for Citizen interactions".

## Few players online

Every quest that needs another player has an **NPC fallback after a timeout** (3 minutes by default). Without it, "ask a Citizen for directions" stalls at 3 a.m.

## Map size and the driver (D10)

- The island is ~200 m across, so crossing it at bike speed (13–20 m/s) takes about 10–15 s.
- In the MVP, riding gives the driver speed and route mastery.
- When ride-hailing arrives, the trip itself has to be the point: chatting on the ride, a rating and a tip, scenic routes. That holds until the map grows (Bùi Viện and UBND are reserved); then saving time starts to matter.
