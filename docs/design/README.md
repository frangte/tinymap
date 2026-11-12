# Saigon Virtual City: design docs

Gameplay, multiplayer and backend design for turning the Sài Gòn Q1 diorama into a shared multiplayer city. Written on 2026-11-06 as a plan for review: **nothing here is implemented yet** unless the code says so. The map and art brief stays in [../PROJECT.md](../PROJECT.md).

**Core principle:** every player shares the same world and the same core systems. The archetype (Traveler, Citizen, Crab Bike driver) only changes how they experience the city and take part in it.

| File | What's in it |
|---|---|
| [00-brief.md](00-brief.md) | The original request, condensed to its requirements, plus the answers given since |
| [01-gameplay.md](01-gameplay.md) | Core loop, onboarding, the three archetypes, questline examples, progression, retention |
| [02-architecture.md](02-architecture.md) | Module map, how modules talk, NPC sessions, quest flow, ride-hailing as a future module, runtime layout |
| [03-domain-model.md](03-domain-model.md) | Entities, Go types, YAML content examples, SQLite schema |
| [04-backend.md](04-backend.md) | Go packages, libraries, REST and WebSocket messages, transactions, validation, storage, chat |
| [05-frontend.md](05-frontend.md) | How the Three.js client uses all of it: entities, animation, phone, dialogue, quests, chat, mobile |
| [06-mvp-roadmap.md](06-mvp-roadmap.md) | MVP scope (must / should / future), phases with acceptance criteria, estimate |
| [07-tradeoffs.md](07-tradeoffs.md) | Notes on the brief, trade-offs, risks, and the existing code patterns to follow |
| [08-ride-hailing.md](08-ride-hailing.md) | Booking a Crab Bike: the flow, the trip states, the xu wallet, the server module, protocol, game, build plan |
| [09-scale-review.md](09-scale-review.md) | Review of the model at 100 players (2026-11-06): what holds, what gives way first, extensibility items to address, as a checklist |

## Decisions

Update the status when a proposed decision is confirmed or changed.

| # | Decision | Status |
|---|---|---|
| D1 | A character picks an archetype on its first visit and **can change it in game** (Đổi vai in the phone's Hồ sơ app): at most once a minute, and not during a Crab Bike trip, an ask for the way or a conversation. It keeps its quests, titles, stamps, wallet and earned capabilities, wears the new archetype's starter clothes, and gets one archetype's dailies a day (the first it plays that day). Capabilities can be earned on top. The schema allows several characters per account; the MVP uses one. | accepted 2026-11-06 (fixed per character); **changed 2026-11-08** (Khoa: players can change archetype in game) |
| D2 | **SQLite** (pure-Go `modernc.org/sqlite`) for now; move to Postgres when the game runs on more than one process | **decided 2026-11-06**, built in phase 0 |
| D3 | Guest accounts first (an anonymous token), real login linked later | accepted 2026-11-06, built in phase 0 |
| D4 | The client moves and the server checks every move against the nav grid; the server owns quests, rewards, items and interactions | accepted 2026-11-06 |
| D5 | One process, an interest grid, copies of the city later; no Redis until there is more than one process | accepted 2026-11-06 |
| D6 | Quests, NPCs, dialogue, items, archetypes and phone apps are data files in the repo, in `server/content/` (embedded in the binary), checked at startup and in `go test`. Files the game also reads are JSON (Vite imports them); server-only files can be YAML. | accepted 2026-11-06; places moved in phase 0; dialogue is JSON too (phase 3: one strict decoder, no new dependency) |
| D7 | NPCs are spawned at runtime as animated pedestrians; the baked figures they replace are removed in Blender | accepted 2026-11-06; **done in phase 3:** the two figures whose places NPCs took (the shopkeeper in nón lá on Lê Lai → Chú Tư, a man on Lam Sơn → Chị Hạnh) were removed from `new.sg.tinymap.blend` and the GLB re-exported. The other baked people stay as passers-by. The nav was not re-baked (see the open questions) |
| D8 | Keep procedural avatars behind an `Avatar` interface; GLB skinned models only if they become necessary | accepted 2026-11-06 |
| D9 | The driver brand is **Crab Bike**, a made-up brand with a **yellow uniform**. Code ids stay generic (`driver`, `ridehail`); the brand lives only in content (names, colours). | **decided 2026-11-06** |
| D10 | (Superseded by D20 once the new map ships.) Keep the ~200 m District 1 island for now; it may grow (Bùi Viện and UBND are already reserved). The driver gets riding and route quests in the MVP. Until the map grows, ride-hailing is about the shared ride, not saving time. | **decided 2026-11-06** |
| D11 | Ride-hailing trips cost money: a simple wallet in **xu** per character (start balance, a daily top-up, fares, a platform cut, tips). See [08-ride-hailing.md](08-ride-hailing.md#money-d11). | **decided 2026-11-06** |
| D12 | A ride request goes to **every on-duty driver nearby at once; the first to accept wins**. | **decided 2026-11-06** |
| D13 | Ride-hailing is built **after phase 4**, as the roadmap orders it; the plan is [08-ride-hailing.md](08-ride-hailing.md). | **decided 2026-11-06** |
| D15 | Crab Bike rules as built: a driver is **on duty only while riding their own bike**; **no cancellation fee** in v1; getting off early (or leaving the seat by a respawn) charges the **whole fare**, a dropped connection the way covered; a restart refunds open trips. See [08-ride-hailing.md](08-ride-hailing.md#as-built). | proposed 2026-11-06 (built), to confirm |
| D16 | **Daily quests:** a quest with `"repeat": "daily"` starts over at midnight in Saigon. Dailies in a pool take turns (one a day, picked by the date, the same for everyone); one daily is on for everyone every day. A daily pays **xu** (through the ledger, once per quest and day) and counts toward a **streak** of days in a row, with stamps at 3 and 7 days. No accept step. | **decided 2026-11-06** (Khoa: xu and streak stamps), built in phase 5 |
| D17 | **Moderation is done in the game:** characters marked as moderators (by a server command) mute, ban and read reports with chat commands. | **decided 2026-11-06**, built in phase 5 |
| D18 | **The co-op quest is "ask and answer":** a Traveler asks the way, Citizens online get a call with a pin, the first to walk over and accept answers, and both get credit. After 3 minutes with no Citizen, Chú Sáu can answer instead. | **decided 2026-11-06**, built in phase 5 |
| D19 | **Vietnamese and English for everything.** Every interface text, place, quest, dialogue line and server notice has both; the player picks the language (start menu or tool button, kept on the device). The default follows the browser (`vi*` → Vietnamese, else English), not "Vietnamese by default", because the Traveler map is meant as a quick first look at Saigon for outsiders. NPCs speak Vietnamese in both, with the English under it in English mode; place names show in English with the Vietnamese under them. Content without its English fails `go test`. See [01-gameplay.md](01-gameplay.md#language). | proposed 2026-11-08 (built), to confirm |
| D20 | **The map becomes "Sài Gòn mini"** (replaces D10's ~200 m District 1 island): a 500 × 445 m island with the city, the Saigon river and Ba Son bridge, Thủ Thiêm, and Vũng Tàu's beaches with Núi Nhỏ and the Christ the King statue. Design in [../map-v5.md](../map-v5.md); every open request in [../requirements.md](../requirements.md). Places, NPC spots, quests that name places and the nav bake move with it when it ships. | **decided 2026-11-11** (Khoa approved the proposal; blockout round 1 awaiting review) |
| D14 | Quests **start by themselves** once available (no accept step, no NPC giving them out); achievements are passive quests on the same engine. Rewards are titles (the first earned is carried at once), stamps and capabilities. An accept step can come with daily quests (phase 5). | proposed 2026-11-06 (built in phase 4), to confirm |

## Where the code is today (2026-11-06, branch v4)

| Area | What exists | What it means for the design |
|---|---|---|
| World server | One Go process (gin + gorilla/websocket). One goroutine owns the whole world, with no locks (`server/internal/hub/hub.go:2`, `Run` at `:93`). Snapshots go out at 12 Hz to every player (`broadcastSnap`, `:196`). Cap of 40 players. | Keep it: it becomes the world loop. Sending everything to everyone has to change before hundreds of players. |
| Protocol | Flat JSON messages `{"t":…}`. All client input goes through `protocol.Decode` (`server/internal/protocol/msg.go:90`). Names and chat are cleaned; rate limits per connection (`server/internal/hub/client.go:21`). | Keep the format; add a request id so commands can get replies. |
| Movement | Client-side, on the nav grid baked in Blender (`public/models/saigon_q1_nav.png`: R·256+G = floor height in cm + 1000, B = walkable, 0.35 m cells). The server only checks world bounds. Walk 3.1 m/s, run 6.8 m/s (`src/game/player.ts:23`). No jump yet. | Go can load the same PNG, so the server can check movement with no physics engine. |
| World content | The island is ~200×200 m. 19 places in Blender XY are defined in TypeScript (`src/game/places.ts:25`). No NPCs yet, but the GLB has baked static people (vendors at Hồ Con Rùa and Lam Sơn, shopkeepers on Lê Lai). | Places move into shared data both sides read. The baked figures mark good NPC spots. |
| Characters | Procedural low-poly pedestrians built from primitives and animated from speed (`src/game/character.ts`). A look is body/outfit/hat; one outfit is already `xeom`. | Not GLB skeletal animation. Emotes, riding and the Crab Bike uniform need new procedural parts and poses. |
| Identity | Name and look in `localStorage` (`src/game/profile.ts:10`). No accounts, no database; ids are per connection. | Persistent identity is the first real dependency. |
| UI | DOM HUD, chat log and speech bubbles, a mobile action menu, the dressing panel, the map view. All Vietnamese. | The phone grows out of the mobile action menu; UI stays in the DOM. |
| Ops | One VPS, nginx + systemd, a static binary cross-compiled with `CGO_ENABLED=0` (`scripts/deploy.sh:116`). The unit is locked down (`ProtectSystem=strict`, `ProtectHome=read-only`); releases are pruned to the last 5. | SQLite needs the pure-Go driver and a systemd `StateDirectory` for the database file (see [04-backend.md](04-backend.md#storage-sqlite)). |

## Progress

- **Phase 0 (foundations): done 2026-11-06, not deployed yet.** What was built, and what moved to later phases, is in [06-mvp-roadmap.md](06-mvp-roadmap.md#phase-0-foundations-152-weeks). The table above describes the code as it was before phase 0.
- **Phase 1 (identity, archetypes, phone): done 2026-11-06, not deployed yet.** See [06-mvp-roadmap.md](06-mvp-roadmap.md#phase-1-identity-archetypes-phone-2-weeks).
- **Phase 2 (movement and presence): done 2026-11-06, not deployed yet.** Server-checked movement, jump, emotes, the interest grid, the driver's motorbike, and the last position kept between visits. See [06-mvp-roadmap.md](06-mvp-roadmap.md#phase-2-movement-and-presence-23-weeks).
- **Phase 4 (quests and achievements): done 2026-11-06, not deployed yet.** The quest engine on world events, the Traveler's questline to the certificate, Citizen and driver starter quests, achievements, titles and stamps, the tracker and the phone apps. See [06-mvp-roadmap.md](06-mvp-roadmap.md#phase-4-quests-and-achievements-3-weeks).
- **Ride-hailing (D13): done 2026-11-06, not deployed yet.** The xu wallet with its ledger, booking a Crab Bike, offers to every driver on duty with the first accept winning, pickup, the passenger carried in seat 1, the fare, the cut and tips, ratings, the Crab and Crab Driver apps, and a quest for each side. See [08-ride-hailing.md](08-ride-hailing.md#as-built).
- **Phase 5 (social and launch hardening): started 2026-11-06.** Daily quests (D16), the co-op quest (D18) and moderation (D17) are built; 300 bots with accounts stay within budget; the mobile pass took a crowd from 40 to 60 fps with the CPU slowed 4×. Left: fewer meshes per avatar (draw calls in a crowd), and checking on a real mid-range Android. See [06-mvp-roadmap.md](06-mvp-roadmap.md#phase-5-social-and-launch-hardening-2-weeks).
- **Phase 3 (interaction, NPCs, dialogue): done 2026-11-06, not deployed yet.** Five NPCs with scripted dialogue (private, public and group conversations), the dialogue engine with archetype variants, NPCs in the interest grid, the conversation sheet, and the two baked figures they replace removed from the map. See [06-mvp-roadmap.md](06-mvp-roadmap.md#phase-3-interaction-npcs-dialogue-23-weeks).

## Open questions

- **The map file bakes (resolved 2026-11-07).** The island is now `models/temp.sg.tinymap.blend` (the GLB import with Bùi Viện and Tôn Đức Thắng). `bake_nav.py` tells ground from obstacles by material on it, and the buildings nobody walks into are `NAV_Solid_` boxes in its `NAV` collection (`scripts/blender/nav_boxes.py`). On the unchanged v4 file this reproduced the old nav except the two removed figures' spots, which are now open. The first bake fixed walking through the Bùi Viện houses and the invisible walls where the edit removed lamps and planters; the game now steps down up to 1.1 m, so no ledge traps a player.
- The NPCs' spots and looks (`server/content/npcs.json`): Cô Ba by the south gate's clock tower, Chú Tư in front of the Lê Lai shophouses, Dì Năm on Hồ Con Rùa's south rim, Chị Hạnh on Lam Sơn square, Chú Sáu with his xe ôm where Lê Lai meets the roundabout. Move them in the file (Blender XY); a test checks every spot is open ground inside its place.
- The dialogue (`server/content/dialogue/`) is a first draft by the developer, to be rewritten by a native writer; the Phase 4 quests will hang off its signals (`greetings_learned`, `order_ca_phe_sua_da`, `buy_banh_trang_tron`, `tour_lam_son`, `asked_directions`).

- Crab Bike art: the yellow is `#D9A21B` (jacket, helmet, badge; `CRAB_YELLOW` in `src/game/outfits.ts`). The phase 2 motorbike has a slate-blue body and a yellow top box (`src/game/motorbike.ts`); change it there if the whole bike should be yellow.
- Bikes on pavements: there is no road layer in the nav bake, so a bike goes wherever people walk. Is that fine (it is Saigon), or should the bake mark roads?
