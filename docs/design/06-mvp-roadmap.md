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

**Status: done 2026-11-06 (not deployed yet).**

- **Built:**
  - The server loads the game's nav map (`server/internal/nav`, a copy embedded from `server/content/nav/`; a test fails when it drifts from `public/models/`, `npm run nav:sync` copies it) and checks every reported move (`server/internal/world/move.go`): distance within top speed × 1.25 averaged over 1 s (a saved-up second, so bunched reports pass), the end on a walkable cell, the line between two reports crossing at most 0.2 m of blocked ground and no floor step over 1.2 m (a ledge), and the height within 0.6 m below to 1.5 m above the floor nearby, with no more than 2.5 s spent over 0.3 m up (hovering is put back on the ground; 2.5 s because a slow device runs the game behind the clock, and a jump at 8 fps takes 1.5 s). A refused move gets `correct` back to the last accepted pose; states carry the count of corrections applied (`k`), so the ones sent before the client heard are dropped instead of refused again. The one teleport is a respawn to the player's home.
  - Speeds are content: `server/content/movement.json` (walk 3.1, run 6.8, jump) and `vehicles.json` (the `xe_so_110`: 13.2 m/s riding along and 19.8 m/s with Shift since 2026-11-06, 11 m/s top before), read by the game and the server.
  - State flags in `state` and in every snapshot row: in the air, riding (set by the server), and the emote code. Emotes are content (`emotes.json`: wave, bow, photo, exercise; keys 1–4 or the smiley button); an emote needing a capability a character lacks is stripped by the server.
  - Jump (Space or the arrow button): a jump can land on something up to its height plus a step higher. Procedural poses for the jump, the four emotes and riding, blended over the walk cycle.
  - The interest grid (`server/internal/world/view.go`): positions go only to players within 60 m (out again past 68 m), the nearest 40, and only for those who moved; a snapshot lists who left your view. Join, leave, profile and vehicle messages still go to everyone. Each player's row is encoded once per tick and shared.
  - Vehicles: the driver archetype grants `vehicle.motorbike.ride` and arrives with an `xe_so_110` parked beside them (`vehicle.go`). Only the owner rides it, in seat 0, within 2.5 m to get on (E or the "Lên xe" button); moves are then checked at bike speed, and the bike leaves with its owner. The motorbike model is built from primitives (slate-blue body, yellow Crab Bike top box) with spinning wheels, steering and lean. Offline, a driver gets a local bike.
  - Arrivals at home are scattered by the server (up to 1.5 m), and the bike parks beside that spot; the game eases onto the pose the welcome gives.
  - The last position: saved with the character (migration `0003_last_pose.sql`) every 30 s and when the player leaves, and the next visit starts there; `welcome` carries the pose the server chose. The browser also remembers it, so the player spawns there before the server answers and the welcome rarely moves them.
  - The same character joining again (a reconnect after the network dropped, or a second tab) replaces the earlier connection and starts where it stood; the replaced tab shows a message and stops reconnecting.
  - `MAX_PLAYERS` defaults to 150, and `server/cmd/loadbots` connects walking bots and reports what each receives.
- **Verified:**
  - Go tests with `-race`: honest walking, running and riding are never refused; speeding, teleporting, walking into a building, flying, sinking and hovering are corrected while a slow jump passes; stale states are ignored; respawn only goes home, and gets the player off the bike even when it is dropped; 11 m/s is fine on the bike and refused on foot; only the owner can mount; positions only reach players nearby, and leaving view shows up in `out`; the last pose survives a restart; a rejoin replaces the earlier connection.
  - Bandwidth: 150 players at 12 Hz in the world tests, 12.1 KB/s at most spread over the island and 12.0 KB/s all in one square (the view's cap of 40 holds it). Over real sockets with `loadbots`, 150 bots for 20 s: 13.0 KB/s on average, 14.3 KB/s at most, no move refused.
  - Two independent code reviews (server and client) before sign-off; their findings were fixed: the ledge and hover checks, a respawn that the server dropped leaving it thinking the player still rode (now it always dismounts, and the game resends a dismount if the two ever disagree), a mount that was never answered, keys acting while a button had focus, a slow client removed in the middle of a snapshot, the last save waiting for the world to drop the player, and shutdown waiting for those saves.
  - Two headless browsers (a driver and a traveller): the bike parked beside the driver, getting on and off with E, the traveller seeing the driver ride and the bike under them, the bike reaching 11 m/s without a correction, jump and wave seen by the other player, a 40 m tampered move corrected back to the exact spot, respawn accepted with the bike brought home, and the second visit starting where the first ended; the only move the server refused was the tampered one; no console errors.
- **Changed from the plan:**
  - There is no drivable mask yet: a bike may go wherever a pedestrian may (pavements included). The nav bake would need a road layer for it.
  - Seat 1 (a passenger) is in the model but not usable: it comes with ride-hailing, where the passenger's own client has to follow the driver's bike.
  - Guests without an account start at home on every visit; only characters keep their position.
  - The world view radius (60 m) is smaller than the island, so the phone's map and Hàng Xóm only place the players near you (the others show as "ở xa"), and the map view only draws those.
  - Mobile draw calls with many avatars were not prototyped: the server's cap of 40 visible players bounds what a phone draws; merged avatar meshes wait for phase 5's performance pass.

The original plan:

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

**Status: done 2026-11-06 (not deployed yet).**

- **Built:**
  - Five NPCs (`server/content/npcs.json`, shared with the game): Cô Ba by Bến Thành's south gate, Chú Tư at his café on Lê Lai, Dì Năm selling bánh tráng trộn by Hồ Con Rùa, Chị Hạnh guiding tours on Lam Sơn square, and Chú Sáu with his xe ôm (and a parked bike) at the roundabout. Each has a name, a role shown under the name tag, a look, a spot and a reach (3 m, 4 m for the guide).
  - NPCs are entities of the world (`server/internal/world/npc.go`), in the interest grid like players: one shared figure each, its id before the players'. It turns toward whoever spoke last, plays the line's emote (a wave to greet, a bow to thank), turns back 5 s after its last conversation, and NPCs with calls (`barks`: Dì Năm, Chú Sáu) call one out every 35–55 s while someone is within 14 m. The game draws them as pedestrians (`src/game/npcs.ts`), offline too, and the phone's map marks them.
  - Conversations (`server/internal/world/talk.go`): `interact` / `choose` / `interact_leave`, answered by `interaction` / `interaction_end` and a `result` for the request id (`rid`, the first commands that need an answer). Three modes: **private** (each player their own conversation: any number talk to Cô Ba at once), **public** (private, but the NPC's lines are also heard nearby as chat, at most one every 3 s), **group** (the guide: one conversation everyone who walks up joins, up to 8; its longest-standing member picks the answers, the others see them greyed out). One conversation at a time; it ends on close, on walking further than reach + 2 m, on starting another, or on leaving. Every line carries a step, and an answer to a line already gone is refused.
  - The dialogue engine (`server/internal/dialogue`): a `Provider` interface, the scripted `Script` first. Graphs are content (`server/content/dialogue/<id>.json`), one per NPC: each node a line with a variant per archetype (`default` otherwise) and `{name}`, an optional NPC emote, and answers that lead to a node or the end, may require capabilities, and may send a signal (`greetings_learned`, `order_ca_phe_sua_da`, `buy_banh_trang_tron`, `tour_lam_son`, `asked_directions`) for the quests of phase 4. Signals go to `world.Config.OnSignal`, which only logs them for now; dialogue never grants anything.
  - Content checks (`server/content/cast.go`): every reference (place, look, vehicle, emote, capability, archetype variant), every goto, every node reachable from the start, an end reachable from every node, no placeholder but `{name}`, no control or invisible characters, and a public NPC's lines short enough for chat. A test checks each NPC stands on open ground inside its place.
  - The game: the action button and E offer "Nói chuyện · Cô Ba" next to an NPC (on the bike it still gets off first); the conversation sheet (`src/game/dialogueSheet.ts`) sits bottom left where the chat steps aside (a bottom sheet left of the action buttons on phones), with the line, its English under it, big answer buttons and keys 1–9 (which play emotes again once the sheet closes), × or Esc to leave. NPC lines in the chat log and as bubbles over the NPC.
- **Verified:**
  - Go tests with `-race`: three players talking to Cô Ba at once in three conversations, each moving on its own, and walking away ending only that one; reach, unknown targets, stale and wrong answers refused; one conversation at a time; a group shared, led, joined, full, and handed to the next leader when the first leaves; signals sent to everyone in the conversation; public lines heard within 14 m and paced; the NPC turning to the speaker and back, waving as it greets; calls to passers-by; every conversation graph able to end; the content rejections; and the same over real sockets (walk up to Cô Ba, talk, answer, walk away).
  - Two independent code reviews (server and client); their findings were fixed: a player dropped (full queue) while switching conversations left a session behind, a line that fails to render could read an emptied session, a public NPC could be used to flood nearby chat, answers indexed after a malformed one would shift, an answer crossing the next line on the wire answered the wrong line (now the step), a closed conversation reopening from a line already on its way, a stuck "busy" after a conversation ended, Enter opening the hidden chat, the dressing panel opening over a conversation, and an unknown NPC's conversation kept open.
  - Three headless browsers (a traveller, a citizen, a driver) walking up to Cô Ba: each greeted in its archetype's words, three conversations, answers moving each on its own (digit keys answering rather than waving), the traveller walking away and only theirs ending (with "Bạn đã đi xa Cô Ba."), Esc closing the driver's; then the citizen and the driver joining Chị Hạnh's tour as one group, the driver seeing the citizen's choices greyed out and the line moving for both; Chú Sáu calling out to the traveller; no move refused, no console errors. Screenshots at desktop and phone widths.
- **Changed from the plan:**
  - The baked figures (D7): only the two whose places NPCs took were removed (the shopkeeper in nón lá at a Lê Lai shop counter, now Chú Tư's spot, and a man on Lam Sơn, now Chị Hạnh's), from `new.sg.tinymap.blend`, the v4 source. The other 14 baked people (people sitting round Hồ Con Rùa, two on Lam Sơn, the Lê Lai shopkeepers) stay as passers-by; none of them was a vendor Dì Năm could take over. The GLB was re-exported through the usual chain (only the five clothing, skin and hat materials changed). The nav maps were not re-baked: that file is the GLB imported back, and the bake needs the original objects (see the open questions in the README).
  - NPCs stand; they do not wander yet (that is in the "later" column above). Their poses already go through snapshots, so wandering needs no protocol change.
  - Dialogue is JSON rather than YAML (D6), with the same strict loading as the rest of the content.
  - The AI dialogue provider stays future work: `dialogue.Provider` is synchronous today, and an AI one would need an asynchronous path into the world loop.
  - Reach is measured on the ground (no height), and a player may keep talking while riding; the game only offers to start a conversation on foot.

The original plan:

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

**Status: done 2026-11-06 (not deployed yet).**

- **Built:**
  - **Events** (`server/internal/events`, published by the world in `server/internal/world/observe.go` and the conversation and vehicle code):
    - a place entered (after 2 s, from accepted moves only) and time spent there (5 s steps)
    - a conversation started with an NPC, and an answer's signal (only for whoever picked it)
    - an emote, and a greeting (two characters within 5 m playing the same emote at each other within 20 s)
    - getting on a vehicle, and distance ridden (100 m steps)
    - The world publishes through a non-blocking hook; no client message is ever an event.
  - **The quest engine** (`server/internal/quest`), compiled from `server/content/quests.json`:
    - Quests and achievements share one engine (an achievement is a passive quest).
    - Quests start by themselves once available: an archetype tag in `tags_any`, every quest in `requires` done (D14).
    - Objectives go in `sequence`, `all` or `any`.
    - A completion can set off achievements (`quest_done`), and opens the quests that require it.
  - **Objective types** (`server/internal/quest/objective`), one file each, the extension point:
    - `reach_place`, `stay_place`, `talk_npc`, `dialogue_signal`, `emote_at`, `greet`, `mount`, `ride_through`, `ride_distance`, `quest_done`
    - Each checks its params against the game data when the engine starts; an unknown place, an NPC signal that does not exist, or more distinct things than there are fails the tests.
    - Distinct things count once; an objective that does not count distinct things must have a count of 1.
  - **The quest service:**
    - Workers run the characters, each always on the same one, so its events apply in order.
    - Each event changes a copy of the character's progress, which is saved in one transaction (retried twice) and only then kept and shown.
    - Rewards are keyed in `reward_grants` in that same transaction, so a retry never gives one twice.
    - Migration `0004_quests.sql`: quest progress, reward grants, earned titles and stamps, earned capabilities, the carried title.
  - **Rewards:**
    - titles: the first one earned is carried at once; others can be picked in Hồ sơ with the `title` command, and others see them under the name and in chat
    - stamps, for the Traveler's stamp book
    - capabilities: `local.certified`, the certificate, applies at once and on every later visit
  - **Content:**
    - The Traveler's questline "Người Sài Gòn chính hiệu" in six quests:
      - Cô Ba's greeting lesson
      - a coffee at Chú Tư's and 20 s at the café
      - the Cathedral, Post Office and Opera House
      - bánh tráng from Dì Năm and 15 s by the lake
      - directions from Chú Sáu
      - Cô Ba's five-question final test, now in her dialogue, offered only during that quest (`requires_quest`)
    - Four Citizen and two driver starter quests.
    - Five achievements, eight titles and six stamps.
  - **The game:**
    - the quest tracker under the place card (the next objective, an arrow relative to the camera and the distance; tap for the quests app)
    - the Nhiệm vụ and Thành tích apps, the stamp book in Cẩm nang and the title picker in Hồ sơ
    - a toast and a phone notice for each quest done, achievement, title and stamp, and for each quest that opens
- **Verified:**
  - **Go tests with `-race`:**
    - the whole Traveler questline on events alone (order rules, all-of-three, the final test opening only after the four middle quests, the certificate given once)
    - wrong events changing nothing, distinct things counting once, a driver's route only counting while riding and in order
    - every objective type's param checks
    - the store keeping progress and giving a grant once even when committed twice
    - the service: a join showing the progress, a failed save keeping nothing, a replaced visit's late leave not dropping the new one, only earned titles carried, the certificate's capability given once
    - the world: a place counting only after 2 s and never for a refused teleport, no events for guests, a greeting needing to be returned in time, mounting and riding published
    - the first quest over real sockets
  - **Two independent code reviews** (server and client); their findings were fixed:
    - a group's answer credited to everyone in the group
    - objectives with a count that could be farmed by repeating one action
    - greetings counted from standing in a crowd
    - one-off events lost to a busy database
    - a capability missed after an ambiguous save
    - shutdown able to block a closing connection
    - the arrow spinning the long way round
    - the quests app collapsing on every update
    - the title picker not following the server
    - unbounded or prototype-unsafe parsing of `progress`
    - per-frame allocations in the tracker
  - **A headless browser played the whole Traveler questline with real, server-checked moves** (about 600 m of walking):
    - the tracker pointed the way at every step
    - a wrong quiz answer had to be redone
    - six quests, six stamps, the title carried and seen by a second player, the certificate achievement
    - no move refused, no server error, no console error
    - A Citizen arriving got the Citizen quests.
- **Changed from the plan:**
  - No accept step: quests start themselves (D14, to confirm). There are no café stools or items yet, so the coffee quest asks for time at the café after ordering, and the bánh tráng quest for time by the lake.
  - "Ask a Citizen for directions" is asked of Chú Sáu for now; the Citizen-player version with its NPC fallback is phase 5's co-op quest.
  - Titles and stamps live in an `earned` table rather than counters; achievements keep their progress in the quest table.
  - Guests without an account have no quests (nowhere to keep them).
  - A capability earned while playing updates what the character may do at once, but the phone's apps only on the next visit; none of today's earned capabilities unlocks an app.

The original plan:

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

- **Ride-hailing, next after phase 4 (D13): built 2026-11-06, not deployed yet** ([08-ride-hailing.md](08-ride-hailing.md#as-built)); the load run with bots is still to do. Planned at about 2 weeks. It needs phase 2's seats and phase 4's character workers and events, and brings the xu wallet (D11).
- **Done when (ride-hailing):** a traveller books, a driver picks them up and drops them off, and the fare moves between their wallets. Three drivers accepting at once gives one winner, and every cancel path refunds. The ride logic lives in the `ridehail` package and the Crab apps; the core gains only generic pieces (seat 1, carried players, view pins, a module hook, the wallet) with no ride-specific code. (The first plan said "without editing any core package"; the seat, the view and money turned out to be core.)
- The rest of the economy, city copies with Postgres + Redis, AI NPCs, a bigger map.

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
