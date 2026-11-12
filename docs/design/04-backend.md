# Backend architecture (Go)

## Packages

The packages evolve from the original `hub`, `protocol` and `web`. Phase 0 built `ws`, `web`, `world`, `identity`, `store`, `ids` and `server/content/`; the rest are the plan:

```
server/content/      data files shared with the game (places.json), embedded in the binary; checked by its tests
server/internal/
  protocol/          wire types + Decode, the one place client input is validated (exists, grows)
  ws/                connection loops, the hello's account, ping, rate limits   (from hub/client.go)
  web/               gin: the account API, REST, static files
  ids/               ULIDs for stored ids
  world/             the loop: entities, interest grid, snapshots, mounts, seats  (from hub/hub.go)
  world/movement/    speed / walkability / height checks
  nav/               loads saigon_mini_nav.png, the same file the client uses
  content/           loads YAML, cross-checks every id, fails fast with file:line
  identity/          accounts, tokens, characters
  archetype/  capability/  inventory/  achievement/
  quest/             engine and registry
  quest/objectives/  one file per objective type
  npc/  interaction/  dialogue/  chat/  phone/  notify/  events/
  store/             SQLite repositories behind interfaces, embedded migrations
  ridehail/          future leaf module; nothing in the core imports it
```

**Dependency rules:**

- Domain packages never import the transport packages (`ws`, `web`).
- `world` never imports `quest`; it only publishes events.
- Gameplay rules are plain Go, testable without a socket or a database file.

## Library choices

| Option | Verdict |
|---|---|
| Keep gorilla/websocket and the custom world loop | **Yes.** It's small and correct, and it fits the problem. |
| Nakama (a full Go game server) | The one serious alternative: auth, storage, chat, friends, groups and notifications come built in. But the world loop would have to be rewritten as a Nakama match handler, plugins must be built with the exact same Go version, it's heavy, and it expects Postgres or CockroachDB. Worth a 2-day trial only if friends and groups become a near-term priority. |
| Pitaya, Leaf | No. Pitaya is a cluster/microservice framework (NATS, etcd), which is premature; Leaf is dated. |
| Centrifugo | Maybe later, for chat and presence across processes. |
| `modernc.org/sqlite` | **Yes** (D2). Pure Go, so the `CGO_ENABLED=0` cross-compile in `scripts/deploy.sh:116` keeps working. `mattn/go-sqlite3` needs cgo. |

## REST

REST covers things that are cacheable or not real-time.

| Endpoint | Purpose |
|---|---|
| `POST /api/auth/guest` | returns `{token}` |
| `GET /api/me` | the account and its characters |
| `POST /api/characters {name, archetype, look}` | creates a character |
| `GET /api/content/<hash>.json` | the public content bundle (places, NPC names, quest text), cached forever |
| `GET /api/me/quests`, `/api/me/achievements`, `/api/me/inventory` | data for the phone apps |
| `GET /api/profiles/:id` | a public profile card |
| `POST /api/reports` | files a chat report |

The token travels in the `Authorization` header for REST, and in the first WebSocket frame (`hello`), since browsers can't set headers on a WebSocket.

## WebSocket messages

The flat `{"t":…}` style stays. Commands that need an answer carry a `rid` and get a `result` back with the same `rid`.

| Client → server | Payload | Owner |
|---|---|---|
| `hello` | `token, character` | identity → world |
| `state` | `x,y,z,h,s` (`s` = flags: running, jumping, emote id, mounted) | world, checked |
| `emote` | `id` | world (unlockable emotes require a capability) |
| `chat` | `ch: global\|nearby\|whisper, text, to?` | chat |
| `interact` / `choose` / `interact_leave` | `target` / `sid, choice` / `sid` (+`rid`) | interaction, dialogue |
| `quest_accept` / `quest_track` | `qid` (+`rid`) | quest |
| `item_use` | `item` (+`rid`) | inventory |
| `mount` / `dismount` | `vehicle, seat` | world |
| *future* `ride_request` / `ride_accept` / `ride_cancel` | `dest` / `ride` | ridehail |

| Server → client | Payload |
|---|---|
| `welcome` | self (character, capabilities, phone apps, active quests), entities nearby, chat history, content hash |
| `spawn` / `despawn` | an entity enters or leaves your area of interest (player, NPC, vehicle) with look, name, archetype badge, title |
| `snap` | `[[id,x,y,z,h,s,age]…]`, only entities **near you that moved** |
| `chat` | `ch, from{id,name,archetype,title}, text, at` |
| `interaction` / `interaction_end` | `sid, npc, mode, node{say, choices}` / `sid, reason` |
| `quest` / `reward` / `achievement` / `inventory` | state changes |
| `notice` | `app, kind, payload` (the phone routes it to that app) |
| `correct` | the server-approved pose after a rejected move |
| `result` / `error` | `rid, ok, code` / `code` |

## What happens synchronously vs through events

| Synchronous (request → reply) | Event-driven (published → handled in the background) |
|---|---|
| Login, character creation, starting an interaction, dialogue choices, accepting or claiming a quest, using an item, mounting | Quest progress from world events, achievements, notifications, chat logging for moderation, NPCs reacting to nearby players, analytics |

## Concurrency

- **World loop:** one goroutine per city copy. It owns positions, the interest grid, NPC behaviour, interaction sessions, mounts and seats, and the connection send queues. It never does I/O.
  - NPC and interest updates run at ~20 Hz.
  - Snapshots stay at 12 Hz, as today.
- **Character workers:** N goroutines, with each character hashed to one of them. Quest, inventory and achievement work runs here, in order per character, and so do the database transactions.
- **Outbound messages:** services post `deliver(characterID, msg)` to the world loop, which keeps the queue ownership rule of `server/internal/hub/hub.go:2`.

## Transactions

| Operation | Boundary |
|---|---|
| Position | Memory only; save `last_pose` every 30 s and on leave (best effort) |
| Objective done → quest done → rewards → achievements | **One transaction per (character, event)**; the `reward_grants` primary key makes retries safe |
| Using an item | One transaction (decrease the stack), then publish `ItemUsed` after commit |
| Character creation | One transaction: character + archetype grants + starter items |
| Future ride accept | `UPDATE rides SET state='accepted', driver_id=? WHERE id=? AND state='offered'`; whoever changes 1 row wins (works the same on Postgres) |
| Future ride completion (two players) | One transaction: change the ride state, pay the fare, update both players' counters |
| Chat | Not transactional; appended in the background to a moderation log |

## What the server enforces

- **Movement:**
  - Distance over a sliding 1 s window must stay within the allowed speed × 1.25 (walk 3.1 / run 6.8 m/s from `src/game/player.ts`, 19.8 m/s on a motorbike).
  - The target cell must be walkable (or drivable when mounted).
  - Height must stay between the floor and floor + 1.3 m (room for a jump).
  - No teleports except server-issued respawns.
  - Anything else gets a `correct` message, and repeated offences are counted.
- **Interactions:** within reach (3 m for an NPC, 1.5 m for a seat) when starting, and ended once the player is more than reach + 2 m away.
- **Quests:** progress only comes from events the server itself observed. Social objectives require distinct partners and a daily cap per pair.
- **Appearance and emotes:** unlockable outfits and emotes require owning the item or the capability.
- **Chat:** rate limits per account (today's limits are per connection), length, filters, mutes.

## Storage: SQLite

- **Driver:** `modernc.org/sqlite` (pure Go, see the library table above).
- **Settings at open:** `journal_mode=WAL`, `synchronous=NORMAL`, `foreign_keys=ON`, `busy_timeout=5000`.
- **Connections:** one write connection (a dedicated `*sql.DB` with `SetMaxOpenConns(1)`) and a small read pool. Start write transactions with `BEGIN IMMEDIATE` so two writers never deadlock upgrading a read lock. The character workers keep write transactions short, so one writer is plenty at this scale.
- **Where the file lives:** **not** inside `releases/`, which `deploy/remote.sh` swaps on every deploy and prunes to the last 5.
  - Add `StateDirectory=sg-tinymap` to `deploy/systemd/app.service.template`. systemd then creates `/var/lib/sg-tinymap`, writable by the service even under the unit's `ProtectSystem=strict` and `ProtectHome=read-only`.
  - Pass `DB_PATH=/var/lib/sg-tinymap/game.db`.
  - Update the unit's comment "the server writes nothing to disk".
- **Migrations:** `.sql` files embedded in the binary and applied in order, tracked with `PRAGMA user_version`; no extra dependency.
  - Migrations only **add** (new tables, new nullable columns), so `npm run deploy:rollback` to an older binary still runs against a newer database.
  - A breaking change takes two deploys.
- **Backups:** a nightly `VACUUM INTO '/var/lib/sg-tinymap/backup/game-YYYYMMDD.db'`, keeping 7 days, copied off the machine. Add Litestream later if continuous backup is wanted.
- **Tests:** a temporary database file per test, with `go test -race`, like the existing tests.
- **Repositories behind interfaces in `store/`:** the move to Postgres (the trigger is a second game process, at the same time as Redis) only replaces that package.

## What lives where

| Store | Holds |
|---|---|
| SQLite | Durable player state only ([03-domain-model.md](03-domain-model.md#sqlite-schema-player-state-only)) |
| YAML content | All definitions; loaded at startup, checked in `go test` |
| Memory | Positions, sessions, NPC behaviour, recent chat |
| Redis (none in the MVP) | Later, with several processes: presence (character → process), global chat and DMs across processes, shared rate limits |

## Chat at hundreds of players

- **Nearby (20 m) is the main channel**, shown as speech bubbles over heads, as the name tags already do. The world loop picks the recipients from the interest grid.
- **Global chat:**
  - *Slow mode* that kicks in when traffic is high.
  - A short time in the game first (e.g. one finished quest) before posting.
  - Identical repeated messages are suppressed.
  - Links are stripped.
- **Moderation:**
  - A word filter that ignores accents, catching Vietnamese with and without diacritics plus common abbreviations.
  - Personal mute and block lists.
  - Reports that capture the last 20 lines.
  - Server-side mutes and bans (`accounts.banned_until`).
- **Display:** name, archetype badge, title (e.g. "Lan · Du khách · Người Sài Gòn chính hiệu").
- **Server-side cleaning stays as today** (`server/internal/protocol/msg.go`): control and invisible characters are removed, and the length is capped.

## Future AI NPC dialogue

- It sits behind a `dialogue.Provider` interface; scripted dialogue is the first provider.
- The model writes text in character, and can only *suggest* actions from an allowed list (`give_hint`, `suggest_quest`). The dialogue engine checks each suggestion against the game rules.
- It never calls inventory, quest or reward services.
- It needs per-player rate limits, moderation of its output and a cost budget.
