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

## Decisions

Update the status when a proposed decision is confirmed or changed.

| # | Decision | Status |
|---|---|---|
| D1 | The archetype is fixed per character; capabilities can be earned on top. The schema allows several characters per account; the MVP uses one. | accepted 2026-11-06 |
| D2 | **SQLite** (pure-Go `modernc.org/sqlite`) for now; move to Postgres when the game runs on more than one process | **decided 2026-11-06**, built in phase 0 |
| D3 | Guest accounts first (an anonymous token), real login linked later | accepted 2026-11-06, built in phase 0 |
| D4 | The client moves and the server checks every move against the nav grid; the server owns quests, rewards, items and interactions | accepted 2026-11-06 |
| D5 | One process, an interest grid, copies of the city later; no Redis until there is more than one process | accepted 2026-11-06 |
| D6 | Quests, NPCs, dialogue, items, archetypes and phone apps are data files in the repo, in `server/content/` (embedded in the binary), checked at startup and in `go test`. Files the game also reads are JSON (Vite imports them); server-only files can be YAML. | accepted 2026-11-06; places moved in phase 0; dialogue is JSON too (phase 3: one strict decoder, no new dependency) |
| D7 | NPCs are spawned at runtime as animated pedestrians; the baked figures they replace are removed in Blender | accepted 2026-11-06; **done in phase 3:** the two figures whose places NPCs took (the shopkeeper in nón lá on Lê Lai → Chú Tư, a man on Lam Sơn → Chị Hạnh) were removed from `new.sg.tinymap.blend` and the GLB re-exported. The other baked people stay as passers-by. The nav was not re-baked (see the open questions) |
| D8 | Keep procedural avatars behind an `Avatar` interface; GLB skinned models only if they become necessary | accepted 2026-11-06 |
| D9 | The driver brand is **Crab Bike**, a made-up brand with a **yellow uniform**. Code ids stay generic (`driver`, `ridehail`); the brand lives only in content (names, colours). | **decided 2026-11-06** |
| D10 | Keep the ~200 m District 1 island for now; it may grow (Bùi Viện and UBND are already reserved). The driver gets riding and route quests in the MVP. Until the map grows, ride-hailing is about the shared ride, not saving time. | **decided 2026-11-06** |
| D11 | Ride-hailing trips cost money: a simple wallet in **xu** per character (start balance, a daily top-up, fares, a platform cut, tips). See [08-ride-hailing.md](08-ride-hailing.md#money-d11). | **decided 2026-11-06** |
| D12 | A ride request goes to **every on-duty driver nearby at once; the first to accept wins**. | **decided 2026-11-06** |
| D13 | Ride-hailing is built **after phase 4**, as the roadmap orders it; the plan is [08-ride-hailing.md](08-ride-hailing.md). | **decided 2026-11-06** |

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
- **Phase 3 (interaction, NPCs, dialogue): done 2026-11-06, not deployed yet.** Five NPCs with scripted dialogue (private, public and group conversations), the dialogue engine with archetype variants, NPCs in the interest grid, the conversation sheet, and the two baked figures they replace removed from the map. See [06-mvp-roadmap.md](06-mvp-roadmap.md#phase-3-interaction-npcs-dialogue-23-weeks).

## Open questions

- **The v4 map source can export but not bake.** The v4 island is `new.sg.tinymap.blend` (Khoa, 2026-11-06; not in git yet). It is the shipped GLB imported back into Blender: one scene, 20 meshes merged by material. `export_glb.py` exports it (a round trip of the unchanged file matched the shipped GLB except duplicate and zero-area triangles, which the game's double-sided materials do not show). `bake_nav.py` cannot run on it, because it tells ground, water, buildings and roofs apart by the original object names. So the nav maps were not re-baked when the two figures were removed: the half-metre each one blocked is still blocked, right behind Chú Tư and next to Chị Hạnh. A re-bake needs the original objects back (the root `sg.tinymap.blend` has them, in an older layout), or a bake that classifies by material.
- The NPCs' spots and looks (`server/content/npcs.json`): Cô Ba by the south gate's clock tower, Chú Tư in front of the Lê Lai shophouses, Dì Năm on Hồ Con Rùa's south rim, Chị Hạnh on Lam Sơn square, Chú Sáu with his xe ôm where Lê Lai meets the roundabout. Move them in the file (Blender XY); a test checks every spot is open ground inside its place.
- The dialogue (`server/content/dialogue/`) is a first draft by the developer, to be rewritten by a native writer; the Phase 4 quests will hang off its signals (`greetings_learned`, `order_ca_phe_sua_da`, `buy_banh_trang_tron`, `tour_lam_son`, `asked_directions`).

- Crab Bike art: the yellow is `#D9A21B` (jacket, helmet, badge; `CRAB_YELLOW` in `src/game/outfits.ts`). The phase 2 motorbike has a slate-blue body and a yellow top box (`src/game/motorbike.ts`); change it there if the whole bike should be yellow.
- Bikes on pavements: there is no road layer in the nav bake, so a bike goes wherever people walk. Is that fine (it is Saigon), or should the bake mark roads?
