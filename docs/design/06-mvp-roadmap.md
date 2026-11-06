# MVP scope and roadmap

## MVP scope

| Must have | Should have | Future |
|---|---|---|
| Guest identity + persistent character (SQLite) | Interest-grid snapshots (needed above ~60 players) | Ride-hailing on the Crab apps (passenger seat + ride state machine) |
| Choosing between 3 archetypes, defined as data with capabilities | Citizen↔Traveler co-op ("Thổ địa" / "Hỏi đường") | Wallet, economy, marketplace |
| Walk/run (exists), **jump**, 4 emotes, all synced | Tourist guide app + stamp book | Housing, businesses |
| Server-side movement checks (nav grid in Go) | Daily quest rotation | AI-powered NPC dialogue |
| Global + nearby chat with badges, filter, mute, report | Sitting on café stools; a scripted NPC passenger ride (tests seats early) | Friends, parties, events, seasonal content |
| Phone: Profile, Quests, Notifications, Chat, Map, Achievements | NPC wandering; day/night cycle | Several city copies + Redis + Postgres; real-account login |
| Quest engine + ~6 objective types + YAML content | Chat slow mode; moderation commands | A content admin tool (definitions in the database) |
| 4–5 NPCs with scripted dialogue: Cô Ba (Bến Thành), Chú Tư (café on Lê Lai), the Hồ Con Rùa vendor, a guide at Lam Sơn, the motorbike driver at the roundabout | Load test with 300 bot clients | More archetypes (food delivery, street vendor, photographer…) |
| The full Traveler questline (6 quests) + title, badge, outfit | | A bigger map (Bùi Viện, UBND, more districts) |
| Citizen: 3 quests. Crab Bike driver: yellow uniform, **one-person motorbike**, a route quest | | |

## Phases

### Phase 0: Foundations (1.5–2 weeks)

**Status: done 2026-11-06 (not deployed yet).**

- **Built:**
  - `hub` split into `internal/world` (state, no I/O) and `internal/ws` (connections, the account lookup).
  - SQLite in `internal/store`, with embedded migrations and a backup before each one, plus daily backups.
  - Guest accounts in `internal/identity`: `POST /api/auth/guest`, a `token` in `hello`, `error auth` for an unknown token.
  - Places moved to `server/content/places.json`, read by both the Go server and `src/game/places.ts`.
  - The systemd unit gets `StateDirectory` and `DB_PATH`.
- **Changed from the plan:**
  - The `rid` envelope and the client router wait for the first command that needs a reply (phase 3 interactions); nothing needs one yet.
  - Saving the last position waits for phase 1, where archetypes choose the spawn point.
  - One session per character (a second tab replacing the first) waits for phase 4, where quest progress makes it matter.
  - For now the profile the client sends is what gets stored; the server becomes the source of truth with character creation in phase 1.
  - Shared content is JSON, because Go can't embed files outside its module and Vite imports JSON natively.
  - SQLite runs through one connection; a read pool can come when reads grow.

The original plan:

- **Goal:** the plumbing everything else needs, with today's gameplay unchanged.
- **Features:**
  - Split `hub` into `world` + `transport`; add the `rid` envelope and the client router.
  - Set up SQLite: `modernc.org/sqlite`, embedded migrations with `user_version`, a `StateDirectory` + `DB_PATH` in the systemd unit, the nightly `VACUUM INTO` backup.
  - Guest auth and character persistence.
  - The `content/` loader; move places into shared data.
- **Depends on:** D2 (decided), D3, D6.
- **Done when:**
  - Gameplay is identical to today.
  - A reconnect (and a server restart) restores your character.
  - The database survives `npm run deploy` and `npm run deploy:rollback`.
  - `go test -race` checks every content file.
- **Risks:** refactoring a working hub. Keep `TestPlayersSeeEachOther` (`server/internal/web/router_test.go:170`) passing the whole way.

### Phase 1: Identity, archetypes, phone (2 weeks)

**Status: done 2026-11-06 (not deployed yet).**

- **Built:**
  - `server/content/archetypes.json` (capabilities, the three archetypes, restricted looks) and `apps.json`, read by both sides; `content/catalog.go` checks every reference, and a test fails if code anywhere names an archetype id.
  - The archetype is stored on the character (migration `0002_archetype.sql`) and kept once chosen; `welcome` carries it and the phone apps its capabilities give.
  - The Crab Bike uniform (`crab` outfit, `helmet` hat, yellow `#D9A21B`), wearable only with `uniform.crab`; the server swaps it off in a hello and drops a profile message wearing it.
  - Chat channels: `global` and `nearby` (20 m, not kept in the history); every line and player carries the archetype for its badge.
  - Client: the "Bạn là ai?" picker (players from before phase 1 see it once too), a spawn point and respawn label per archetype, badges in chat and dots on name tags, and the phone (round button or `P`) replacing the action menu, with Hồ sơ, Trò chuyện, Bản đồ (a drawn map with everyone on it), Thông báo, plus Cẩm nang (Traveler), Hàng Xóm (Citizen) and Crab Driver (driver, a placeholder until ride-hailing).
- **Verified:** Go tests with `-race` (catalog, archetype kept once, apps per archetype, uniform gating, nearby chat reaching 10 m and not 100 m), type-check and build, and a headless-browser run of each archetype through picker, dressing, going online, every phone app and both chat channels, with no console errors.
- **Changed from the plan:**
  - Titles have no data yet (they are quest rewards, phase 4); only the archetype badge shows next to names.
  - Restoring the last position moves to phase 2: the client spawns before it connects, so a server-sent position would make the player jump after the camera flight. Each visit starts at the archetype's spawn point.
  - Name and look are still what the client sends (checked by the server); the archetype and what it may wear are the server's. A fully server-owned profile comes with real login.
  - The default chat channel stays global, as before; nearby is one tap away (the button left of the chat field, or the Trò chuyện app).

The original plan:

- **Goal:** three archetypes that feel different from the first minute.
- **Features:**
  - Archetype selection and capability resolution.
  - The Crab Bike uniform (`crab` outfit, `helmet` hat).
  - The phone shell with Profile, Chat, Map and Notifications.
  - Chat channels, badges and titles; nearby chat.
- **Depends on:** Phase 0.
- **Done when:**
  - The three archetypes spawn in different places, see different apps, and appear differently in chat.
  - A grep check in the tests finds no comparison against archetype ids outside `archetype/`.
- **Risks:** phone UX on small screens.

### Phase 2: Movement and presence (2–3 weeks)

- **Goal:** richer movement that the server checks, and a world that scales past 40 players.
- **Features:**
  - Jump, emotes and the `s` state flags.
  - Server movement checks against the nav grid.
  - The interest grid.
  - Vehicle and seat entities; motorbike riding.
- **Depends on:** Phase 1.
- **Done when:**
  - A tampered client can't speed or teleport.
  - The driver rides at bike speed.
  - 150 bots at 12 Hz stay under ~25 KB/s per client.
- **Risks:**
  - **Mobile draw calls with many avatars:** prototype merged or simplified avatar meshes early.
  - How riding feels on the nav grid.

### Phase 3: Interaction, NPCs, dialogue (2–3 weeks)

- **Goal:** a shared world with people in it.
- **Features:**
  - NPC entities and behaviours.
  - Interaction sessions (private, public, group).
  - The dialogue engine with archetype variants.
  - 5 NPCs.
  - Remove the baked figures the NPCs replace in Blender (D7), then re-export the GLB and re-bake the nav.
- **Depends on:** Phase 2.
- **Done when:** three players talk to Cô Ba at once in separate sessions, and walking away ends your session.
- **Risks:** the effort of writing dialogue; the Blender re-export pipeline.

### Phase 4: Quests and achievements (3 weeks)

- **Goal:** progression, with the server in charge.
- **Features:**
  - The engine and the objective registry.
  - Rewards granted in one transaction; achievements and counters.
  - The Traveler questline; Citizen and Crab Bike starter quests.
  - The quest tracker and achievement apps.
- **Depends on:** Phase 3.
- **Done when:**
  - The Saigon Local certificate is reachable only by actually playing; crafted client messages can't get it.
  - Editing content needs no Go change.
- **Risks:** content versioning; objectives are keyed by stable ids for this reason.

### Phase 5: Social and launch hardening (2 weeks)

- **Goal:** ready for real players.
- **Features:**
  - The co-op quest with its NPC fallback.
  - Daily quests.
  - Moderation tooling.
  - A 300-bot load test; a mobile performance pass.
- **Depends on:** Phase 4.
- **Done when:** a load test at the target number of players stays within budget, and a mid-range Android phone holds 30 fps.
- **Risks:** too few players for social quests; the moderation workload.

### Future

- Ride-hailing (needs Phase 2 seats and Phase 4 events), the economy, city copies with Postgres + Redis, AI NPCs, a bigger map.
- **Done when (ride-hailing):** it adds the `ridehail` package, the Crab and Crab Driver apps, and new quest objectives, **without editing any core package**.

## Estimate

| Phase | Rough effort (one developer) |
|---|---|
| 0. Foundations | 1.5–2 weeks |
| 1. Identity, archetypes, phone | 2 weeks |
| 2. Movement and presence | 2–3 weeks |
| 3. Interaction, NPCs, dialogue | 2–3 weeks |
| 4. Quests and achievements | 3 weeks |
| 5. Social and launch hardening | 2 weeks |
| **MVP total** | **~12–16 weeks** |

The biggest unknowns are writing the dialogue and quest content, and avatar performance on mobile.
