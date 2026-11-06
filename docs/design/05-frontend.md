# Frontend architecture (Three.js + TypeScript)

The client keeps its current shape (`src/game/app.ts` wires modules together, the DOM handles the UI, three.js draws the scene). The new systems are added as modules next to the existing ones.

| Concern | Plan, building on the existing modules |
|---|---|
| Network | `src/net/router.ts`: typed handlers plus `request()` that waits for the `rid` reply, with a timeout. `src/net/api.ts`: REST calls with the guest token. `src/net/client.ts` keeps its reconnect/backoff logic. |
| State | `src/game/state/`: tiny subscribable stores (self, quests, inventory, notices). No framework, to keep the bundle small. |
| Entities | Generalize `RemotePlayers` (`src/game/remotePlayers.ts`) into `Entities` (players, NPCs, vehicles), with the same smoothing (`RENDER_DELAY` 230 ms) and per-kind avatar builders. NPCs reuse `Pedestrian`. The motorbike is a new primitive-built model (see [Crab Bike assets](#crab-bike-assets)). |
| Animation | The snapshot `s` flags drive procedural poses: jump, emotes (wave, bow, photo, exercise), sitting, riding. Same style as `src/game/character.ts`, behind an `Avatar` interface so GLB models could replace it later (D8). |
| Movement | Add jump to `src/game/player.ts` (vertical velocity over the nav floor). Mounting switches to a vehicle speed profile. On a `correct` message, ease the player back over ~150 ms instead of snapping. |
| Interaction | `src/game/interact.ts`: the nearest interactable (NPC, seat, vehicle) within reach, from the local entity list. Shows "E · Nói chuyện" on desktop, a context button on mobile. The client check is for UX only; the server checks again. |
| Phone | A DOM phone that slides up. The app grid comes from the server's list (`welcome`). Each app is a lazy-loaded module behind `PhoneApp { mount(root, ctx); unmount(); onNotify?(n) }`. Unknown app ids are hidden, so the server can roll out apps ahead of the client. The phone replaces the round action menu (`src/game/actionMenu.ts`). |
| Dialogue | A bottom sheet with big touch-friendly choice buttons. The server has already picked the archetype-specific lines. |
| Quest UI | A compact tracker (the current objective, a direction arrow and the distance, updated 5×/s) and the full log inside the phone. |
| Chat UI | Keeps the bottom-left log (`src/game/chatBox.ts`) and the speech bubbles (`src/game/nameTags.ts`). Adds channel tabs (Gần đây / Toàn server / Tin riêng), archetype badges and titles, mute/report on a long press. |
| Mobile controls | Keeps the floating joystick and orbit/pinch (`src/game/input.ts`). Adds buttons for jump, the context action, the phone and an emote wheel. 44 px touch targets and safe-area insets. |
| Mobile performance | Draw at most ~20 detailed avatars, the nearest ones (today everyone within 80 m is drawn: `VISIBLE_DIST` in `src/game/remotePlayers.ts:24`). Simplify or hide beyond ~40 m on phones. Lazy-load phone apps. Keep the adaptive pixel ratio (`src/game/app.ts`) and the no-allocation-per-frame rule. Slow the update rate when the tab is hidden. |
| Shared content | `PLACES` (`src/game/places.ts`) and NPC spots come from the same content bundle the server loads. One source of truth, still in Blender XY. |

## Project rules that apply

From `CLAUDE.md`:

- Everything added to the scene goes through `InkPass.track()`.
- Materials come from `paintMaterial()`; moving meshes (avatars, NPCs, the motorbike) pass `{ local: true }`.
- Moving things use a blob shadow, because the sun shadow map is baked once.
- Movement and the camera only read the nav grid; never raycast the city meshes per frame.
- Positions use `fromBlender()`.

## Crab Bike assets

The yellow Crab Bike uniform and the motorbike (D9). The brand is content only: code ids stay `driver` / `ridehail`. The uniform and helmet were built in phase 1 (yellow `#D9A21B`); the motorbike comes with phase 2.

| Asset | Where it goes |
|---|---|
| Outfit `crab`: a yellow jacket over dark trousers | `OUTFITS` and its style in `src/game/outfits.ts`; the allowed outfit ids in `server/internal/protocol/msg.go:48` (later, look ids should come from content so both sides read one list) |
| Hat `helmet`: a yellow motorbike helmet | `HATS` in `src/game/outfits.ts`; a new helmet part in `src/game/character.ts`, built once and toggled by `setLook` like the other hats; the allowed hat ids in `server/internal/protocol/msg.go` |
| Motorbike `xe_so_110` | A new primitive-built model in the diorama style, a yellow top box with the Crab Bike mark, `paintMaterial(…, { local: true })`, a blob shadow, `InkPass.track()`. Seats 0 (driver) and 1 (the future passenger) are attachment points. |
| Colours | Character colours live in the outfit styles. The bike's paint needs a key in `PAINT` (`src/game/paint.ts`), and in `PAL` too if it should exist in Blender (see `docs/PROJECT.md`). |
| Phone apps | "Crab Driver" (drivers) and later "Crab" (passengers), as phone app modules with yellow branding |

The uniform and helmet are unlockable looks: only characters that own them can wear them. Drivers get them as starter items.
