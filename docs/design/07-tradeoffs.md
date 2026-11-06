# Trade-offs, risks, and patterns to follow

## Notes on the original brief

These are the places where the design deliberately differs from the brief ([00-brief.md](00-brief.md)), and why.

1. **The map is 200 m across, not a city.** Zones, rooms and Redis are premature.
   - One Go process handles hundreds of WebSocket clients easily.
   - What breaks at "hundreds" is **bandwidth**, because sending everyone to everyone grows with N², and **mobile rendering**: dozens of multi-mesh avatars, with the ink pass drawing the scene twice.
   - The fix is an interest grid plus a cap on how many avatars are drawn. To scale past one process, run copies of the city, not spatial zones.
2. **Ride-hailing on a 200 m island is a 20–30 second trip.** Until the map grows (D10), it's about the shared ride (chatting, a rating, a tip), not saving time.
3. **Brand:** a made-up brand instead of a real one. The choice is **Crab Bike** with a yellow uniform (D9). Keep the logo and app look clearly its own, and keep real brands (Grab, Zalo…) out of the game.
4. **Language belongs to the player, not the archetype.** The archetype changes the tone of NPC lines.
5. **Full server-side movement simulation isn't worth it for a social game.** The client moves and the server checks every move. The server stays fully in charge of quests, rewards, items and interactions, so cheating on movement can't fake progress.
6. **Social quests need NPC fallbacks**, or they stall when few players are online.
7. **The motorbike belongs in the MVP.** Without it, the driver is a Citizen in different clothes. It also builds the vehicle and seat system that ride-hailing needs.

## Modelling archetypes

| Approach | Pros | Cons |
|---|---|---|
| A class per archetype | Familiar | Go has no inheritance; roles can't combine; every new archetype touches every system |
| An archetype enum + `switch` | Fastest to start | The `switch` statements spread into every system |
| **Archetype as data + capabilities (chosen)** | Additive; content-driven; unlockable through progress; combinable | Needs discipline: check capabilities and tags, never the archetype id (enforced by a grep check in the tests); watch for too many capabilities |
| A full entity-component system | Maximum flexibility | Overkill for about five kinds of component |

## Other decisions

| Decision | Chosen | Alternatives and why not |
|---|---|---|
| NPCs | One shared NPC + per-player sessions | One NPC per player: simpler dialogue state, but wasteful, and players see different worlds. NPCs visible only to players on a given quest can come later as a visibility filter. |
| Quest engine | YAML + Go objective types | A scripting language (Lua, Starlark): more flexible, but security, performance and tooling costs. Definitions in the database: only once live operations need an admin tool. Hard-coded quests: never. |
| Phone | A server-filtered app list + lazy client modules | iframes / web views: heavy on mobile. A hard-coded app set: every app touches the shell. |
| Multiplayer consistency | Client movement, checked by the server | Full simulation from inputs: much more server CPU and latency handling. Only worth it for competitive racing. The weakness of the chosen approach (small speed cheats inside the tolerance) can't affect anything durable. |
| Schema | Relational core, JSON for the parts that evolve, definitions in files | Generic key/value-per-row tables: flexible on paper, hard to query and validate |
| Database (D2) | **SQLite** now | Postgres: needed for more than one process. SQLite keeps one binary and one file, with no database server to run, back up or upgrade on the VPS. The costs: a single writer, which is fine with short transactions at this scale; backups and file placement to handle ([04-backend.md](04-backend.md#storage-sqlite)); and a store-package swap when a second process arrives. |
| Characters (D8) | Procedural avatars behind an `Avatar` interface | GLB skinned characters: richer animation, but bigger downloads and a different look from the diorama |

## Bandwidth math

A snapshot row like `[7,12.34,0.18,-3.4,3.142,37]` is about 35 bytes of JSON, sent 12 times a second.

| Setup | Per client | Server total |
|---|---|---|
| Everyone to everyone, 300 players | 300 × 35 B × 12 ≈ 126 KB/s | ≈ 38 MB/s ≈ 300 Mbit/s (doesn't work) |
| Interest grid, ~40 nearby entities | 40 × 35 B × 12 ≈ 17 KB/s | ≈ 5 MB/s ≈ 40 Mbit/s (fine) |
| Plus binary encoding | ~3× less again | only if needed |

Sending only the entities that moved since the last snapshot cuts this further. Today the hub sends every placed player whenever anyone moved (`server/internal/hub/hub.go:196`).

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| Mobile performance: avatars are built from many meshes and the ink pass doubles every draw | High | Cap detailed avatars at ~20; simpler versions far away; prototype in Phase 2, not at launch |
| Too few players for social quests | High at launch | NPC fallbacks after a timeout; nearby chat over global |
| Moderation: young players, Vietnamese slang | Medium | Accent-insensitive filter, mutes, reports, posting requirements for global chat |
| Writing bilingual dialogue and quest content | Medium | YAML content with i18n keys; start with Vietnamese, add English text later |
| Losing the SQLite file to a deploy or a disk failure | Medium | `StateDirectory` outside releases; nightly `VACUUM INTO` copied off the machine |
| Rolling back to an older binary after a migration | Low | Additive-only migrations; breaking changes take two deploys |
| The Crab Bike brand being read as a copy of a real brand | Low | Its own logo, yellow (not green), no real-brand marks |
| Scope creep | High | The must / should / future split in [06-mvp-roadmap.md](06-mvp-roadmap.md); a new feature waits for its phase |

## Existing patterns to follow

| Category | Where | Pattern |
|---|---|---|
| Concurrency | `server/internal/hub/hub.go:2`, `Run` at `:93` | One goroutine owns all world state; everything goes in through one ordered channel |
| Input checks | `server/internal/protocol/msg.go:90` | One `Decode` gate; sentinel errors (`msg.go:84`); bad messages are dropped and the connection kept |
| Logging | `server/internal/hub/hub.go:163` | `slog` with key/value pairs |
| Tests | `server/internal/protocol/msg_test.go:38`, `server/internal/web/router_test.go:170` | Go table-driven tests and a real WebSocket integration test, run with `-race`. There are no TypeScript tests; verify with `npm run typecheck` and by playing |
| Data access | none yet | No database or repository code exists; `store/` is new |
| Client conventions | `CLAUDE.md` | Blender XY via `fromBlender()`; `paintMaterial()` (`{ local: true }` for moving meshes); `InkPass.track()`; blob shadows; reuse scratch objects in per-frame code |
| Deploy | `scripts/deploy.sh`, `deploy/systemd/app.service.template` | A static `CGO_ENABLED=0` binary; a locked-down unit; templates edited in the repo, never on the server |
