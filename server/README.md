# Sài Gòn Q1 server

Go server for multiplayer: [gin](https://github.com/gin-gonic/gin) for HTTP, [gorilla/websocket](https://github.com/gorilla/websocket) for the socket, SQLite ([modernc.org/sqlite](https://gitlab.com/cznic/sqlite), pure Go) for what lasts between visits. One shared world, capped at 150 players. Everyone hears who is on the island and the chat; positions go to the players nearby. The server checks every move against the game's own nav map. Each browser gets a guest account the first time, so the server knows the same player next time, and where they stood.

The design this grows into (archetypes, quests, NPCs, the phone) is in [../docs/design/](../docs/design/README.md).

```bash
npm run server          # from the repo root: go run ./cmd/server on :8787
npm run server:test     # go vet + go test -race
npm run nav:sync        # after re-baking the nav maps: copy them into content/nav/ (a test fails until you do)
cd server && go run ./cmd/loadbots -n 150 -seconds 30   # walking bots against a running server: KB/s each
cd server && go run ./cmd/loadbots -n 300 -seconds 120 -accounts -chat 1   # with accounts and chat (server: GUEST_BURST=400 MAX_PLAYERS=320)
```

In dev, run `npm run server` next to `npm run dev`. Vite forwards `/ws` and `/api` to `localhost:8787`, so the game connects to `ws://<page host>/ws` both in dev and in production. The dev database is `server/data/game.db` (git-ignored); delete the folder to start over.

Production is one process: build the game and let the server serve it.

```bash
npm run build
cd server && go build -o bin/server ./cmd/server
STATIC_DIR=../dist ./bin/server
```

Deploying to the server (nginx + systemd, `npm run deploy`): see [DEPLOY.md](DEPLOY.md).

## Configuration (environment)

| Variable | Default | |
|---|---|---|
| `PORT` | `8787` | listen port |
| `STATIC_DIR` | (none) | serve this directory (the Vite build) for every path other than `/ws` and `/healthz` |
| `ALLOWED_ORIGINS` | (none) | extra page origins allowed to open the socket, comma-separated, exact `scheme://host[:port]` (`https://tinymap.live,https://www.tinymap.live`). Use it when the page is served from another domain. A page on the server's own host is always allowed. |
| `TRUSTED_PROXIES` | none | reverse proxy IPs/CIDRs whose `X-Forwarded-For` gin trusts |
| `MAX_PLAYERS` | `150` | player cap; the next one gets `{"t":"error","code":"full"}` |
| `GUEST_BURST` | `20` | guest accounts one address may take at once (then one per 10 s); raise it only for a load test from one machine (`cmd/loadbots -accounts`) |
| `DB_PATH` | `data/game.db` | the SQLite database, relative to the working directory; its folder is created. The server stops at start if it cannot open it. |
| `BACKUP_DIR` | `<database folder>/backup` | daily backups (`game-<time>.db`, the last 7 kept) and a copy taken before every schema migration (`pre-v<N>-<time>.db`) |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |

`GET /healthz` returns `{"ok":true,"players":3}`.

## HTTP API

| Request | Answer |
|---|---|
| `POST /api/auth/guest` (no body) | `{"token":"<43 characters>"}`: a new guest account. Only the token's SHA-256 is stored. 20 per client address at once, then one per 10 s; past that `429 {"error":"rate"}` (the game then plays without an account and tries again on a later visit). `503 {"error":"unavailable"}` if the database fails. |

`/api` answers the same page origins as `/ws` (its own host and `ALLOWED_ORIGINS`, with `Access-Control-Allow-Origin` for the latter); other origins get 403. Answers are `Cache-Control: no-store`.

## Layout

```
cmd/server/main.go               config from env, wiring, graceful shutdown, daily backups
content/*.json                   game data shared with the game: places, archetypes (capabilities, spawns, badges,
                                 restricted looks, starter vehicle), phone apps, movement speeds, vehicles, emotes;
                                 catalog.go checks every reference, its tests keep archetype ids out of code
content/npcs.json                the NPCs (shared with the game, which draws them): name, role, look, spot, mode, reach
content/dialogue/*.json          what each NPC says: one graph per NPC, server only; cast.go checks every line,
                                 reference and node (reachable, no unknown archetype, emote or capability)
content/quests.json              quests, achievements, titles and stamps (shared with the game); quests.go checks
                                 every reference, the quest engine checks each objective's params
content/nav/                     a copy of the game's nav map (public/models/saigon_q1_nav.*), embedded
cmd/loadbots/                    load test: walking (and chatting) bots, with accounts or as guests; KB/s, welcome time, errors
internal/capability/             a set of capability ids: what systems check instead of the archetype
internal/web/router.go           gin routes, WebSocket upgrade, origin check, the account API, static files
internal/ws/conn.go              per-connection read/write loops, the hello and its account, ping/pong, rate limits
internal/world/world.go          the world, owned by one goroutine (no locks, no I/O): joins, messages, chat
internal/world/move.go           checks every reported move; corrections; respawn; where a player starts
internal/world/view.go           who sees whom (the interest grid) and the snapshots, 12 Hz
internal/world/vehicle.go        vehicles: parking, mounting, getting off; seat 1 (a passenger carried with the rider)
internal/world/host.go           the module hook: modules in the world loop, seats, view pins, jobs off the loop
internal/ridehail/               Crab Bike trips, a world module: booking, offers, pickup, the trip, its money
internal/directions/             asking the way, a world module: a Traveler asks, Citizens are called, one answers
internal/moderation/             reports, mutes, bans and the moderators' chat commands, a world module with a chat hook
internal/chatfilter/             the word filter for chat lines and names (content/chat_filter.json)
internal/world/chat.go           the chat rules: repeats, the new-player wait for global, slow mode
cmd/server/mod.go                `sg-server mod list|add|remove`: who moderates
internal/store/wallets.go        xu wallets, the ledger and the rides table (ridehail.Bank)
internal/world/npc.go            NPCs: placed at start, turn to whoever speaks, play the line's emote, call out
internal/world/talk.go           conversations with NPCs: private, public, group; walking away ends yours
internal/dialogue/dialogue.go    the dialogue engine (a Provider; Script runs the graphs in content/dialogue)
internal/world/observe.go        what players do, published as events for quests: places, time there, emotes, rides
internal/events/                 the event types the world publishes (never sent by a client)
internal/quest/engine.go         the quest rules, compiled from quests.json: progress, completion, rewards, dailies
internal/gameday/                the game's calendar: days run midnight to midnight in Saigon (dailies, top-ups)
internal/quest/objective/        the objective types, one file each (the extension point for new kinds of quests)
internal/quest/service.go        runs quests for the characters playing, on worker goroutines; stores each event's
                                 changes in one transaction, then tells the world
internal/nav/nav.go              the nav grid: walkable cells, floor heights, line checks
internal/identity/identity.go    guest accounts, tokens and characters
internal/store/store.go          SQLite: one connection, migrations (migrations/*.sql), backups (on a read-only connection of their own), the queries
internal/ids/ids.go              ULIDs for stored ids
internal/protocol/msg.go         message types and validation (the rules below)
```

Schema changes are new numbered files in `internal/store/migrations/`. They only add (tables, nullable columns), so `npm run deploy:rollback` to an older server still works on the newer database.

## Protocol

JSON text frames, one message per frame, the type in `t`. The client mirrors this in `src/net/protocol.ts`. Positions are three.js metres (Y up) and the heading is radians (0 = facing +Z), which matches `Player.position` and `Player.heading` in the game.

`look` is `{"body": "man"|"woman", "outfit": "thun"|"aodai"|"baba"|"somi"|"xeom"|"hoa"|"bikini"|"crab", "hat": "nonla"|"cap"|"bucket"|"helmet"|"bun"|"none"}` (`src/game/outfits.ts`). `crab` and `helmet` are the Crab Bike uniform: only a character whose archetype grants `uniform.crab` may wear them (`content/archetypes.json`). In a hello the server swaps them for `thun` / `none`; a profile message wearing them is dropped.

Archetype ids (`arch`) are the ids in `content/archetypes.json` (`traveler`, `citizen`, `driver`). A character keeps the first archetype it picks; its capabilities decide its phone apps (`content/apps.json`). An archetype with a `vehicle` (the driver's `xe_so_110`) arrives with it parked beside them.

`s`, the state flags, is a number: bit 0 in the air, bit 1 riding, bit 2 a passenger in seat 1 behind a rider (both set by the server; what a client says is ignored), bits 4–7 the emote code from `content/emotes.json` (0 for none).

### Client to server

| Message | When | Rules |
|---|---|---|
| `{"t":"hello","token":"…","arch":"driver","name":"Lan","look":{…}}` | first frame, within 10 s | `arch` (optional) is the archetype the player picked: stored on a character that has none yet, ignored for one that has one, used for this visit by a guest without an account; an unknown id plays without an archetype. an invalid look, a token over 64 characters or a different first frame → `error bad_hello`, then the socket closes. An empty name becomes `Khách <id>`. `token` (from `POST /api/auth/guest`) is optional: without it the player is a guest without an account, as before. A token no account has → `error auth`, then the socket closes. With a valid token, the name and look are stored on the account's character (created on its first visit). If the database fails, the player still joins, without the account. |
| `{"t":"state","x":1.2,"y":0.18,"z":-3.4,"h":3.14,"s":0,"k":0}` | about 10 Hz while playing, and when the flags change | \|x\|, \|z\| ≤ 400, −20 ≤ y ≤ 120, all finite, `s` only known bits; anything else is dropped. Up to 20/s; extra frames are dropped silently. `k` is the number of `correct` messages the client has applied on this connection: a state with another `k` was sent before the latest one arrived and is ignored. Every move is checked (below); a refused one gets `correct`. An emote the character may not play is stripped. |
| `{"t":"respawn","x":8,"y":0.1,"z":19,"h":0,"k":0}` | the respawn button | the one teleport: it must land within 4 m of the player's home (the archetype's spawn, or the market gate) on a walkable cell, else `correct`. Gets the player off a vehicle and brings their own vehicle along. At most 4/s with mount and dismount. |
| `{"t":"mount","v":12}` | next to their vehicle | only the owner, within 2.5 m, not riding already; then `vehicle` to everyone with `rider` set, and moves are checked at the vehicle's speed. Otherwise `error mount`. |
| `{"t":"dismount"}` | | the vehicle stays parked where the rider was; `vehicle` to everyone |
| `{"t":"profile","name":"Lan","look":{…}}` | after the dressing panel | same rules as hello; at most 4/s. Stored on the character when the hello had a token (an empty name keeps the stored one). |
| `{"t":"interact","n":3,"rid":5}` | E next to an NPC | starts a conversation with NPC `n` (or joins its group conversation); within the NPC's reach (3 m, 4 m for the guide). Answered with `interaction`, then `result`. Refused: `result` with `far`, `target` (no such NPC), `full` (the group has 8), `mute` (no dialogue). Talking to another NPC ends the current conversation (`interaction_end` `other`); talking to the same one shows the line again. |
| `{"t":"choose","sid":12,"c":1,"rid":6}` | an answer button, or keys 1–9 | `c` indexes the choices of the line shown. Answered with `interaction` (the next line, to everyone in the conversation) or `interaction_end` `done`, then `result`. Refused: `stale` (not your conversation, or over), `lead` (in a group only the leader picks), `choice` (not on offer). |
| `{"t":"title","title":"nguoi_trong_xom"}` | the title picker in the phone's Hồ sơ | carry an earned title (`""` puts it away); one not earned is ignored. At most 4/s with profile. |
| `{"t":"interact_leave","sid":12}` | ×, goodbye, Esc | `interaction_end` `left`; a group carries on, led by the next player. interact, choose and interact_leave together: about 7/s with a burst of 6; extra ones are dropped (without a result: the game gives up waiting after 3 s). |
| `{"t":"duty","on":true}` | Crab Driver app | needs the `ridehail.drive` capability, an account, and riding one's own bike (else `ride_error`). Answered with `duty`; getting off the bike ends duty. |
| `{"t":"ride_request","dest":"nha_hat_thanh_pho"}` | Crab app | `dest` a place id. The fare is taken from the wallet at once (`wallet`, then `ride` `requested`) and offered to the drivers on duty nearby. Refused (`ride_error`): `no_account`, `busy` (a trip already), `riding` (on a vehicle), `unknown_place`, `here` (standing in it), `too_soon` (one per 10 s), `balance`. |
| `{"t":"ride_cancel"}` | either side, before pickup | the passenger: the fare comes back. The driver: the trip is offered to the others again. |
| `{"t":"ride_accept","offer":"01J…"}` | the offer banner, Crab Driver | the first accept wins; the others get `ride_offer_gone` `taken`, a late one `ride_error` `gone`. The two players then see each other from anywhere until the trip ends. |
| `{"t":"ride_board"}` | the passenger, next to the bike | within 3 m of the driver's bike, the bike stopped (`too_far`, `not_stopped`, `here`): the passenger sits in seat 1 and is carried; their own states are ignored until they get off. |
| `{"t":"ride_alight"}` | the passenger, on the trip | gets off early: the whole fare is charged (at the destination it is the trip completed) |
| `{"t":"ride_complete"}` | the driver, at the destination | within the place's reach, stopped (`not_there`, `not_stopped`): the passenger gets off, the driver is paid the fare minus the cut |
| `{"t":"ride_rate","stars":5,"tip":5000}` | the passenger, within 2 min of a completed trip | stars 1–5, `tip` one of `content/ridehail.json` `tips`; once per trip (`gone`, `tip`, `balance`). Ride messages together: about 4/s with a burst of 4; extra ones are dropped. |
| `{"t":"way_ask","dest":"nha_hat_thanh_pho"}` | the Cẩm nang (a Traveler) | needs `directions.ask` and an account; `dest` a landmark. Every player online with `directions.give` gets `way_call`, and keeps the asker in view. Answered with `way` `asking`, or at once `unanswered` (`why` `nobody`) when no one can be called. Refused (`way_error`): `no_account`, `not_allowed`, `busy` (asking already), `unknown_place`, `too_soon` (one per `ask_gap_seconds`). |
| `{"t":"way_cancel"}` | the asker | stops asking: the calls get `way_gone` `cancelled`, the asker `way` `cancelled`. |
| `{"t":"way_answer","ask":"01J…"}` | a Citizen called, standing within `reach` (4 m) of the asker | the first answer wins: both get credit (the events below), the asker `way` `answered` with the `helper`, the helper `way_done`, the other calls `way_gone` `taken`. Refused: `gone`, `not_allowed`, `too_far`, `already_helped` (a pair is credited once a day). Asking messages share the ride messages' rate. |
| `{"t":"chat","ch":"nearby","text":"xin chào"}` | | `ch` is `global` (also when missing) or `nearby`: only players within 20 m (on the ground) hear a nearby line, and it is not kept in the history; any other channel is dropped. Text cleaned (below), 1–140 characters; about one per 0.7 s with a burst of 3, otherwise `error rate` and the line is dropped |

Names and chat are cleaned on the server: control characters and invisible format characters (zero-width, bidi overrides) are removed, runs of whitespace (newlines too) become one space, and the result is trimmed and capped (16 characters for names, 140 for chat). Frames over 1 KB close the connection. Malformed or unknown messages are dropped; the connection stays open.

### Server to client

| Message | When |
|---|---|
| `{"t":"welcome","id":7,"name":"Lan","arch":"driver","apps":["profile",…],"max":150,"pose":[x,y,z,h],"players":[player…],"vehicles":[vehicle…],"npcs":[npc…],"chat":[line…],"wallet":100000}` | reply to hello. `wallet` is the character's balance in xu (missing for a guest without an account): opened with `start_balance` on its first visit, topped up to `topup_to` on its first visit of a Saigon day. `pose` is where the server placed the player: where the character stood when it last left (if that spot is still walkable), else its home. `players` is everyone else, `vehicles` every vehicle, `npcs` every NPC, `chat` the last 30 global lines, oldest first. `name` is the cleaned name. `arch` is the archetype as the server keeps it (missing for none); `apps` the phone apps its capabilities give, in phone order. |
| `{"t":"join","p":player}` | someone else joined |
| `{"t":"leave","id":7}` | someone left, timed out, or was dropped |
| `{"t":"profile","id":7,"name":"Lan","look":{…}}` | someone else changed name or look |
| `{"t":"snap","at":1730790000000,"p":[[id,x,y,z,h,s,age],…],"out":[id…]}` | at most 12 Hz, and only with something in it. A row for each player in the receiver's view (within 60 m, out again past 68 m, the nearest 40) that moved or just came into view; never the receiver. Rounded to cm and milliradians. `age` is how many ms before this snapshot the server received that pose: place it at (receive time − age) to follow the sender's own timing rather than the snapshot tick. `out` (missing when empty) lists the players that left the view: hide them until they come back. |
| `{"t":"correct","k":3,"pose":[x,y,z,h]}` | a move was refused | go back to `pose` and send `k` from now on |
| `{"t":"vehicle",…vehicle}` | a vehicle arrived (with its owner) or someone got on or off | |
| `{"t":"vehicle_gone","id":12}` | its owner left | |
| `{"t":"chat","id":7,"name":"Lan","arch":"driver","ch":"global","text":"xin chào","at":1730790000000}` | a chat line, sent to everyone (`nearby`: to those within 20 m) including its author; `arch` is the sender's archetype. `"npc":true` marks an NPC's line (a call to passers-by, or a line of a public conversation), heard within 14 m of it: `id` is the NPC's. |
| `{"t":"interaction","sid":12,"n":3,"mode":"private","lead":true,"size":1,"say":{"vi":"…","en":"…"},"choices":[{"vi":"…","en":"…"},…]}` | where a conversation is: when it starts, after every answer, and in a group whenever someone joins or leaves. `say` is the line for this player's archetype (`en` missing when there is none); `choices` are the leader's answers (only the leader, `lead`, may pick); none means only goodbye is left. `mode`: `private`, `public` (the NPC's lines are also heard nearby as chat), `group` (`size` players share it). |
| `{"t":"interaction_end","sid":12,"reason":"far"}` | the conversation is over for this player: `done` (it reached its end), `left` (closed), `far` (walked further than the NPC's reach + 2 m), `other` (started another) |
| `{"t":"progress","quests":[{"id":"traveler.kien_truc_phap","n":{"nha_tho":1,"buu_dien":0,"nha_hat":0}},{"id":"traveler.chao_sai_gon","done":true}],"achievements":[{"id":"kham_pha_quan_1","n":3}],"titles":[…],"stamps":[…],"title":"","streak":2,"news":[{"kind":"quest","id":"traveler.chao_sai_gon"},{"kind":"stamp","id":"chao_sai_gon"}]}` | a character's quests (only to one with an account): on joining, and whenever something changed. `n` is how far each objective got (seconds for a stay, metres for a ride); only today's dailies are in it. `streak` is the days in a row with a daily done (0 once a day went by without one). `news` is what just happened (`quest`, `achievement`, `title`, `stamp`), for the game to celebrate. The text is in `content/quests.json`, which the game has. A reward in xu sends `wallet` first. |
| `{"t":"wallet","balance":84000}` | the balance after money moved (a hold, a settlement, a tip) |
| `{"t":"duty","on":true}` | on or off duty |
| `{"t":"ride","id":"01J…","state":"accepted","role":"passenger","dest":"nha_hat_thanh_pho","fare":16000,"passenger":{"id":7,"name":"Lan"},"driver":{"id":9,"name":"Tài","stars":4.8,"trips":31},"pickup":[x,z],"deadline":1730790060000}` | to both sides of a trip on every change. `role` is `passenger` or `rider`; `state` `requested`, `accepted`, `on_trip`, then how it ended: `completed`, `cancelled`, `expired`, `ended`, with `end` (`completed`, `alighted`, `cancelled_by_passenger`, `cancelled_by_driver`, `no_driver`, `pickup_timeout`, `passenger_left`, `driver_left`, `driver_got_off`, `passenger_got_off`, `trip_timeout`, `hold_failed`) and `paid` (what the driver got). `deadline` is when the state times out, or when the rating window closes. |
| `{"t":"ride_offer","offer":"01J…","dest":"…","fare":16000,"passenger":{"id":7,"name":"Lan"},"pickup":[x,z],"expires":1730790015000}` | a request near a driver on duty, free, on their bike |
| `{"t":"ride_offer_gone","offer":"01J…","why":"taken"}` | `taken`, `cancelled`, `expired`, `off_duty`, `busy` (the driver took another trip) |
| `{"t":"ride_rated","id":"01J…"}` | the passenger's rating (and tip) were stored |
| `{"t":"way","id":"01J…","state":"asking","dest":"nha_hat_thanh_pho","until":1762400000000}` | where the asker's ask stands: `asking` (until when), `answered` (with `helper` `{id,name}`), `unanswered` (`why`: `nobody` online, or `no_answer` in `wait_seconds`), `cancelled` |
| `{"t":"way_call","id":"01J…","from":{"id":7,"name":"Lan"},"dest":"bitexco","until":…}` | to each Citizen online (and to those who come online while it waits): someone asks the way |
| `{"t":"way_gone","id":"01J…","why":"taken"}` / `{"t":"way_done","id":"01J…","to":{…},"dest":"…"}` / `{"t":"way_error","code":"too_far"}` | a call withdrawn (`taken`, `cancelled`, `expired`, `left`); thanks to the one who answered; a refused asking message |
| `{"t":"ride_error","code":"too_far"}` | a ride message refused (codes above) |
| `{"t":"result","rid":5,"ok":false,"code":"far"}` | the answer to a command that carried a `rid` (interact, choose); `code` only when it failed |
| `{"t":"error","code":"full"\|"bad_hello"\|"auth"\|"rate"\|"mount"\|"replaced"\|"slow"\|"repeat"\|"new_player"\|"muted"\|"banned"\|"kicked"}` | `slow`, `repeat`, `new_player` and `muted` say why the last chat line was not sent (see Chat rules and moderation). `full`, `bad_hello`, `auth`, `replaced`, `banned` and `kicked` are followed by the server closing the socket; after `banned` or `kicked` the client stops reconnecting. On `auth` the client forgets its token and gets a new one (the database was reset). `replaced`: the same character joined from another connection (a reconnect after the network dropped, or a second tab), which starts where this one stood; the client should not reconnect by itself. |

`npc` is `{"id":3,"npc":"co_ba","pose":[x,y,z,h]}`: `npc` is its id in `content/npcs.json`, which the game also has (name, role, look). NPC ids come before players' and never leave; snapshots carry an NPC's row like a player's when it turns or gestures.

`player` is `{"id":7,"name":"Lan","arch":"driver","title":"nguoi_trong_xom","look":{…}}` (`title` missing for none; profile messages and chat lines carry it too) (`arch` missing for a player without one); where they are comes in snapshots, to the players near them. `vehicle` is `{"id":12,"kind":"xe_so_110","owner":7,"rider":7,"passenger":8,"pose":[x,y,z,h]}`: `rider` is 0 while parked; while ridden the vehicle moves with its rider, and `pose` is where it was last parked. `passenger` (missing for none) sits in seat 1 behind the rider and moves with them.

### Conversations

An NPC is one shared figure (`content/npcs.json`), and any number of players talk to it at once (`internal/world/talk.go`). Each private or public conversation keeps its own place in the NPC's dialogue graph (`content/dialogue/<id>.json`); a group NPC (the guide) has one conversation that everyone who walks up joins, and its longest-standing member picks the answers. The server renders every line for its reader: the variant for their archetype (`default` otherwise), `{name}` filled in, the answers their capabilities allow. A player is in one conversation at a time; it ends when they close it, walk further than the NPC's reach + 2 m, start another one, or leave.

The NPC turns toward whoever spoke last (4 rad/s) and plays the line's emote; 5 s after its last conversation it turns back. NPCs with `barks` call one out every 60–90 s while someone is within 14 m and nobody is talking to them.

An answer can send a signal (`emit`, e.g. `greetings_learned` when Cô Ba's lesson is done). Dialogue never grants anything: signals go to `world.Config.OnSignal`, which only logs them until the quest engine listens (phase 4).

### Quests

Quests and achievements are content (`content/quests.json`): adding one needs no Go change. A quest starts by itself once it is available (one of its `tags_any` among the character's archetype tags, every quest in `requires` done); objectives go in a `sequence`, `all` at once, or `any`. Achievements are passive quests with one objective.

Progress only comes from events the world observed (`internal/events`), never from a client message: entering a place (after standing in it 2 s, from moves the server accepted), time spent there (in 5 s steps), starting a conversation with an NPC, an answer's signal (only for whoever picked the answer, in a group too), an emote, a greeting (two characters within 5 m playing the same emote at each other within 20 s: waving at a crowd that does not wave back greets nobody), getting on a vehicle, distance ridden (in 100 m steps). Distinct things (places, NPCs, players greeted) count once each; an objective that does not count distinct things must have a count of 1, since doing the same thing again is not progress.

The quest service runs each character on one of a few worker goroutines, so its events apply in order. For each event it changes a copy of the character's progress, stores it in one transaction (tried three times, the database being busy), and only then keeps it and tells the game (`progress`); a save that still fails keeps nothing. A reward has a key (`quest:<id>:` or `achievement:<id>`) stored in `reward_grants` in that same transaction, so a retry never gives it twice. Rewards are titles (the first one earned is carried at once), stamps, capabilities (`local.certified`, the Traveler's certificate, applies at once and on later visits) and xu (into the wallet, with a `quest_reward` line in the ledger).

A quest with `"repeat": "daily"` starts over at midnight in Saigon (`internal/gameday`); each day is its own row (`cycle`) and its own reward key (`quest:<id>:<day>`). Dailies with the same `pool` take turns, one a day picked by the date; a daily without a pool runs every day. Dailies give xu and stamps, never titles or capabilities, and no quest may require one. Doing a daily on consecutive days builds a streak (`characters.streak_days`), which `daily_streak` objectives count. The service looks at the date every 30 s, so a player online at midnight gets the new day's dailies. Guests without an account have no quests. The objective types: `reach_place`, `stay_place`, `talk_npc`, `dialogue_signal`, `emote_at`, `greet`, `mount`, `ride_through`, `ride_distance`, `quest_done`, `trip` (Crab Bike trips completed as `rider` or `passenger`, distinct partners), `daily_streak` (days in a row with a daily done; the best streak counts), `ask_way` (a Citizen player answered, or after an unanswered ask the NPC's signal), `give_way` (asks answered, distinct askers).

### Crab Bike trips

Ride-hailing (`internal/ridehail`, `docs/design/08-ride-hailing.md`) is a world module: the world hands it the ride messages, ticks it and tells it who left, and it runs on the world goroutine, so the first `ride_accept` the world reads wins with no lock. The core only gained generic pieces for it (`internal/world/host.go`): seat 1, view pins that keep two players in each other's view, and jobs that run off the loop and answer back in it.

Money is xu (`content/ridehail.json`): a fare is `base` + `per_metre` × the straight line from the pickup to the destination, rounded to `round`; the driver gets it minus `cut_percent`. Every move of money is one transaction in `internal/store/wallets.go`, and a row in `ledger` whose key (ride, reason, character) makes a retried step move nothing. Booking takes the fare at once (`ride_hold`); the trip's end gives it back (`ride_refund`), pays the driver (`ride_earn`), or both in part. The store settles only a ride still open and never more than its fare. A settlement that fails is tried again from the tick (2 s, doubling to a minute), and both sides hear how the trip ended once it is written. Trips live in memory: at startup the server refunds every ride a stopped server left open.

### Chat rules and moderation

Every chat line goes through the world's chat path (`internal/world/chat.go`): modules with a chat hook see it first (moderation); then the same line again within 30 s is refused (`repeat`), global lines wait until a character is 3 minutes old and are closed to guests without an account (`new_player`), and more than 12 global lines in 10 s starts slow mode, one line per 6 s for each player (`slow`). The word filter (`internal/chatfilter`, `content/chat_filter.json`) masks listed words with asterisks in lines and names: a word with Vietnamese marks is matched as written, a word without marks against a short plain list, so unmarked look-alikes of swear words stay readable.

Moderation (`internal/moderation`, D17) is a world module. It keeps the last 300 lines; `{"t":"report","n":7}` stores the last 20 lines of the reporter and the reported (`chat_reports`, once per pair per 10 minutes) and answers `{"t":"reported","n":7}`. A line starting with `/` is a command and goes nowhere else; only characters with `moderation.act` (a moderator account, `sg-server mod add <name or id>`) run them: `/mute <name or #id> [30m|2h|1d]`, `/unmute`, `/ban [1d]`, `/unban <name>`, `/kick`, `/reports`, `/done <n>`. Answers come as `{"t":"mod_notice","text":"…"}`. Mutes and bans are stored on the account (`accounts.muted_until`, `banned_until`), every action in `moderation_log`. A muted player's lines get `error` `muted`; a banned account's hello gets `error` `banned` and the socket closes, as does a kick (`kicked`).

### Asking the way

The co-op quest (`internal/directions`, D18) is a world module like ride-hailing. A Traveler asks the way to a landmark; every Citizen online is called and keeps the asker in view (a view pin) while the call stands; the first to stand within `reach` and answer gives the way. The module publishes `directions.answered` for the asker and `directions.given` for the helper (each with the other as partner), so `ask_way` and `give_way` objectives count them; a pair is credited once a Saigon day. When nobody answers within `wait_seconds` (`content/directions.json`), or nobody could be called, it publishes `directions.unanswered`: the `ask_way` objective then also takes the NPC's signal (Chú Sáu's `asked_directions`), and stays open to it. To find the Citizens online the module walks the world's players (`Host.Each`, a generic piece). Nothing about an ask is stored: an ask ends with the server or the asker leaving.

### Movement checks

The game moves its own player and reports where it got to; the server keeps the last position it accepted (`internal/world/move.go`). A move is refused, and answered with `correct`, when:

- it goes further than the top speed (running 6.8 m/s from `content/movement.json`, or the vehicle's from `content/vehicles.json`) × 1.25 allows, averaged over one second: unused movement saves up for a second, so reports bunched up by the network pass;
- it ends off the walkable cells of the nav map, or the line from the last position crosses more than 0.2 m of blocked cells (a wall);
- its height is more than 0.6 m below or 1.5 m above the floor nearby (a jump reaches about 0.7 m), or it has been over 0.3 m above the floor for more than 2.5 s (a jump lasts 0.6 s, longer on a slow device whose game runs behind the clock): then it is also put back on the ground;
- the floor along the way changes by more than 1.2 m between neighbouring cells (a ledge; a jump climbs about 1.15 m).

Every 50th refused move of a player is logged. Where a character stands is saved every 30 s and when it leaves.

The server pings every 54 s and drops a connection that hasn't answered within 60 s. A client whose send queue falls 64 messages behind is dropped too (`leave` for everyone else).
