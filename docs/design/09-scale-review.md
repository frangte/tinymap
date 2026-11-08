# Review: the game model at 100 players

Written on 2026-11-06 (branch v4, with the ride-hailing work uncommitted). A review of the server and client structure: is it sound, is it easy to extend, and what gives way first at about 100 concurrent players. Tick the boxes as items are addressed; move anything that becomes a plan into the roadmap.

**Verdict:** the server model is good and already carries 100 players on one process. The real limits at that size are on the client and in a few places where a module leaks into the core, not in the world loop.

## Measured

`go test ./...` passes in every package. 100 walking bots (`server/cmd/loadbots`) against a local build for 25 s:

| | Value |
|---|---|
| Bandwidth per client | 12.8 KB/s average, 13.6 KB/s at most |
| Frames per client | 15/s |
| Moves refused | 0 |
| Server RSS after the run | 40 MB |

The bots play as guests, so the database path (quests, pose saves, wallets) was not exercised. See item 4.

## What is solid (keep it)

- **One goroutine owns the world, no locks** (`server/internal/world/world.go`). Connections, quest workers and money jobs reach it through one ordered channel. Backpressure is right: the world never blocks on a worker, workers block on the world, the quest queue drops instead of blocking. No deadlock path between the world loop, the job workers and the quest service.
- **The interest grid with a cap of 40 nearest** (`view.go`) bounds what each client receives. A snapshot row is built once and shared by every viewer; a player standing still sends nothing.
- **Server-authoritative gameplay.** Quests progress only from events the world observed. Reward grants are idempotent by key, wallet moves are ledgered, saves are retried with backoff.
- **The module seam** (`host.go`: `Module` with Handle, Tick, Left and a `Host`) is a clean pattern. Ride-hailing added generic pieces to the core (seats, pins, jobs), not ride-hailing knowledge.
- **Content as embedded JSON, validated in tests.** Objective types are a registry (`server/internal/quest/objective/`), so a new quest kind is one file.

## At 100 players: what gives way, in order of payoff

- [ ] **1. The client renders too many draw calls in a crowd.** Each avatar is about 20 meshes and the ink pass renders the scene twice, so 40 others in view is roughly 1,500 draw calls before the city. A mid-range phone will fall below 30 fps.
  - Cull remote avatars at the server's view radius (60 m) instead of `VISIBLE_DIST` (80 m) in `src/game/remotePlayers.ts`.
  - Merge each avatar's static parts into one geometry per material, or instance body parts across avatars.
  - Build a `Pedestrian` lazily on the first snapshot row, not on join: today every join builds a full avatar even for players never in view.
- [ ] **2. One SQLite connection serialises every database call** (`store.go`: `SetMaxOpenConns(1)`). Hellos, pose saves, quest saves, wallet jobs and the daily backup queue behind each other. A few writes a second is fine; the risk is the backup: `vacuum into` holds the connection for as long as the file takes, and hellos time out after 3 s. Open a second read-only connection for lookups, or take the backup on its own connection.
- [ ] **3. Quest workers have head-of-line blocking** (`quest/service.go`). Four workers, each owning about 25 characters; a failing save retries for up to ~10 s and stalls all 25. Raise `Workers` to 16. It costs nothing.
- [ ] **4. The load test only covers guests.** Give `loadbots` a flag to take a guest token first (`POST /api/auth/guest`), so quests, pose saves and wallets are in the budget. Note the guest endpoint is rate-limited per IP (20 then one per 10 s), so the bots need a server flag or a shared token pool.
- [ ] **5. Mobile data.** 13 KB/s is about 45 MB per hour. The snapshot JSON is already compact; a binary snapshot frame would about halve it. Not urgent.

## Extensibility: what to fix before the next module

- [ ] **The module boundary leaks into the protocol and the socket layer.** Ride messages are fields on the one big `ClientMsg` struct (`protocol/msg.go`) and a named case in the rate-limit switch in `ws/conn.go`. Adding a module means editing three core files. Let a module register its message types (decoder and validation) and a rate class with the protocol and the socket, so the core stays unaware of it.
- [ ] **The event bus is a single callback** (`world.Config.OnEvent`). Only the quest service listens. A second listener (analytics, notifications, a leaderboard) needs fan-out code. Make it a small list of subscribers now, before the second one arrives.
- [ ] **Module-specific event kinds live in the core** (`events.TripCompleted`). Fine for one module; when a second appears, let modules declare their own kinds.
- [ ] **Linear scans by id and by character** (`world.byID`, `world.byCharacter`) run on every service event and module message. Fine at 100 and the comments say so. Add two maps when the cap goes past about 300.
- **Scaling past one process** follows the design doc (`02-architecture.md`): copies of the city, then Postgres and Redis together. Nothing in the code fights that; several copies in one process are already possible, since a world is a value with its own `Run`.

## Smaller notes

- `src/game/app.ts` is 732 lines of wiring. Not a problem yet; split by feature (net wiring, UI wiring, world setup) when the next feature lands there.
- The welcome carries every player and 30 chat lines. Small at 100; revisit at 300.
- Join, leave, profile, title and vehicle messages still go to everyone. Rare enough at 100; make them view-scoped only if churn becomes visible in the per-client budget.
