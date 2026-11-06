# Sài Gòn Q1 server

Go server for multiplayer: [gin](https://github.com/gin-gonic/gin) for HTTP, [gorilla/websocket](https://github.com/gorilla/websocket) for the socket, SQLite ([modernc.org/sqlite](https://gitlab.com/cznic/sqlite), pure Go) for what lasts between visits. One shared world, capped at 150 players. Everyone hears who is on the island and the chat; positions go to the players nearby. The server checks every move against the game's own nav map. Each browser gets a guest account the first time, so the server knows the same player next time, and where they stood.

The design this grows into (archetypes, quests, NPCs, the phone) is in [../docs/design/](../docs/design/README.md).

```bash
npm run server          # from the repo root: go run ./cmd/server on :8787
npm run server:test     # go vet + go test -race
npm run nav:sync        # after re-baking the nav maps: copy them into content/nav/ (a test fails until you do)
cd server && go run ./cmd/loadbots -n 150 -seconds 30   # walking bots against a running server: KB/s each
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
content/nav/                     a copy of the game's nav map (public/models/saigon_q1_nav.*), embedded
cmd/loadbots/main.go             load test: walking bots, KB/s received per bot
internal/capability/             a set of capability ids: what systems check instead of the archetype
internal/web/router.go           gin routes, WebSocket upgrade, origin check, the account API, static files
internal/ws/conn.go              per-connection read/write loops, the hello and its account, ping/pong, rate limits
internal/world/world.go          the world, owned by one goroutine (no locks, no I/O): joins, messages, chat
internal/world/move.go           checks every reported move; corrections; respawn; where a player starts
internal/world/view.go           who sees whom (the interest grid) and the snapshots, 12 Hz
internal/world/vehicle.go        vehicles: parking, mounting, getting off
internal/world/npc.go            NPCs: placed at start, turn to whoever speaks, play the line's emote, call out
internal/world/talk.go           conversations with NPCs: private, public, group; walking away ends yours
internal/dialogue/dialogue.go    the dialogue engine (a Provider; Script runs the graphs in content/dialogue)
internal/nav/nav.go              the nav grid: walkable cells, floor heights, line checks
internal/identity/identity.go    guest accounts, tokens and characters
internal/store/store.go          SQLite: one connection, migrations (migrations/*.sql), backups, the queries
internal/ids/ids.go              ULIDs for stored ids
internal/protocol/msg.go         message types and validation (the rules below)
```

Schema changes are new numbered files in `internal/store/migrations/`. They only add (tables, nullable columns), so `npm run deploy:rollback` to an older server still works on the newer database.

## Protocol

JSON text frames, one message per frame, the type in `t`. The client mirrors this in `src/net/protocol.ts`. Positions are three.js metres (Y up) and the heading is radians (0 = facing +Z), which matches `Player.position` and `Player.heading` in the game.

`look` is `{"body": "man"|"woman", "outfit": "thun"|"aodai"|"baba"|"somi"|"xeom"|"bikini"|"crab", "hat": "nonla"|"cap"|"helmet"|"none"}` (`src/game/outfits.ts`). `crab` and `helmet` are the Crab Bike uniform: only a character whose archetype grants `uniform.crab` may wear them (`content/archetypes.json`). In a hello the server swaps them for `thun` / `none`; a profile message wearing them is dropped.

Archetype ids (`arch`) are the ids in `content/archetypes.json` (`traveler`, `citizen`, `driver`). A character keeps the first archetype it picks; its capabilities decide its phone apps (`content/apps.json`). An archetype with a `vehicle` (the driver's `xe_so_110`) arrives with it parked beside them.

`s`, the state flags, is a number: bit 0 in the air, bit 1 riding (set by the server; what a client says is ignored), bits 4–7 the emote code from `content/emotes.json` (0 for none).

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
| `{"t":"interact_leave","sid":12}` | ×, goodbye, Esc | `interaction_end` `left`; a group carries on, led by the next player. interact, choose and interact_leave together: about 7/s with a burst of 6; extra ones are dropped (without a result: the game gives up waiting after 3 s). |
| `{"t":"chat","ch":"nearby","text":"xin chào"}` | | `ch` is `global` (also when missing) or `nearby`: only players within 20 m (on the ground) hear a nearby line, and it is not kept in the history; any other channel is dropped. Text cleaned (below), 1–140 characters; about one per 0.7 s with a burst of 3, otherwise `error rate` and the line is dropped |

Names and chat are cleaned on the server: control characters and invisible format characters (zero-width, bidi overrides) are removed, runs of whitespace (newlines too) become one space, and the result is trimmed and capped (16 characters for names, 140 for chat). Frames over 1 KB close the connection. Malformed or unknown messages are dropped; the connection stays open.

### Server to client

| Message | When |
|---|---|
| `{"t":"welcome","id":7,"name":"Lan","arch":"driver","apps":["profile",…],"max":150,"pose":[x,y,z,h],"players":[player…],"vehicles":[vehicle…],"npcs":[npc…],"chat":[line…]}` | reply to hello. `pose` is where the server placed the player: where the character stood when it last left (if that spot is still walkable), else its home. `players` is everyone else, `vehicles` every vehicle, `npcs` every NPC, `chat` the last 30 global lines, oldest first. `name` is the cleaned name. `arch` is the archetype as the server keeps it (missing for none); `apps` the phone apps its capabilities give, in phone order. |
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
| `{"t":"result","rid":5,"ok":false,"code":"far"}` | the answer to a command that carried a `rid` (interact, choose); `code` only when it failed |
| `{"t":"error","code":"full"\|"bad_hello"\|"auth"\|"rate"\|"mount"\|"replaced"}` | `full`, `bad_hello`, `auth` and `replaced` are followed by the server closing the socket. On `auth` the client forgets its token and gets a new one (the database was reset). `replaced`: the same character joined from another connection (a reconnect after the network dropped, or a second tab), which starts where this one stood; the client should not reconnect by itself. |

`npc` is `{"id":3,"npc":"co_ba","pose":[x,y,z,h]}`: `npc` is its id in `content/npcs.json`, which the game also has (name, role, look). NPC ids come before players' and never leave; snapshots carry an NPC's row like a player's when it turns or gestures.

`player` is `{"id":7,"name":"Lan","arch":"driver","look":{…}}` (`arch` missing for a player without one); where they are comes in snapshots, to the players near them. `vehicle` is `{"id":12,"kind":"xe_so_110","owner":7,"rider":7,"pose":[x,y,z,h]}`: `rider` is 0 while parked; while ridden the vehicle moves with its rider, and `pose` is where it was last parked.

### Conversations

An NPC is one shared figure (`content/npcs.json`), and any number of players talk to it at once (`internal/world/talk.go`). Each private or public conversation keeps its own place in the NPC's dialogue graph (`content/dialogue/<id>.json`); a group NPC (the guide) has one conversation that everyone who walks up joins, and its longest-standing member picks the answers. The server renders every line for its reader: the variant for their archetype (`default` otherwise), `{name}` filled in, the answers their capabilities allow. A player is in one conversation at a time; it ends when they close it, walk further than the NPC's reach + 2 m, start another one, or leave.

The NPC turns toward whoever spoke last (4 rad/s) and plays the line's emote; 5 s after its last conversation it turns back. NPCs with `barks` call one out every 35–55 s while someone is within 14 m and nobody is talking to them.

An answer can send a signal (`emit`, e.g. `greetings_learned` when Cô Ba's lesson is done). Dialogue never grants anything: signals go to `world.Config.OnSignal`, which only logs them until the quest engine listens (phase 4).

### Movement checks

The game moves its own player and reports where it got to; the server keeps the last position it accepted (`internal/world/move.go`). A move is refused, and answered with `correct`, when:

- it goes further than the top speed (running 6.8 m/s from `content/movement.json`, or the vehicle's from `content/vehicles.json`) × 1.25 allows, averaged over one second: unused movement saves up for a second, so reports bunched up by the network pass;
- it ends off the walkable cells of the nav map, or the line from the last position crosses more than 0.2 m of blocked cells (a wall);
- its height is more than 0.6 m below or 1.5 m above the floor nearby (a jump reaches about 0.7 m), or it has been over 0.3 m above the floor for more than 2.5 s (a jump lasts 0.6 s, longer on a slow device whose game runs behind the clock): then it is also put back on the ground;
- the floor along the way changes by more than 1.2 m between neighbouring cells (a ledge; a jump climbs about 1.15 m).

Every 50th refused move of a player is logged. Where a character stands is saved every 30 s and when it leaves.

The server pings every 54 s and drops a connection that hasn't answered within 60 s. A client whose send queue falls 64 messages behind is dropped too (`leave` for everyone else).
