# Sài Gòn Q1

A walkable miniature of District 1, Ho Chi Minh City: an island in the sea with Chợ Bến Thành as the starting point and the landmarks laid out east of it roughly as in the real city. The map is modelled in Blender (`sg.tinymap.blend`) and shown in Three.js + TypeScript. The start menu offers two ways in: **[1] play** (start in front of the south gate of Chợ Bến Thành and walk anywhere on the streets, plazas, lawns and the beach) and **[2] map** (orbit, zoom and fly to each landmark). Everything is drawn with shaders only: a toon paint shader (colour variation, street grime, rain streaks, paper grain), a screen-space ink outline and a toon sea that reads the scene depth for shallows and shore foam. Clouds, gulls, swaying trees and code-generated area sound (soft surf on the beach for now) keep it alive.

**Start with [docs/PROJECT.md](docs/PROJECT.md)**: project context, the art style, the Blender file conventions and a sheet of every landmark with renders and reference photos.

## Run

```bash
npm install
npm run dev          # http://localhost:5173
npm run build        # type-check + production build into dist/
npm run build:single # one HTML file in dist-single/saigon-noir.html (models/ still served next to it)
npm run i18n:check   # fails on interface text typed outside the catalogs (src/game/i18n/)
```

The game is in Vietnamese and English. The language comes from `?lang=vi|en`, then the player's last choice, then the browser's languages (English when it lists neither); the switch is on the start menu and in the tool buttons, and changes everything on screen at once. NPCs always speak Vietnamese: in English the English goes under each line.

The URL hash picks the mode: `#ban-do` (map), `#choi` (play), none (menu), so the back button and shared links work. `?debug` exposes the app as `window.sgGame` (player, follow camera, map view, the collider and `showCollider()` to draw it, `go(mode)`, `step(n)`); `?debug&manual` stops the frame loop so a test can step it.

## Controls

| | Desktop | Touch |
|---|---|---|
| Menu | press 1 (play) or 2 (map); Esc goes back to the menu | tap a card |
| Map | drag to orbit, wheel to zoom, right-drag to pan, landmark chips to fly | swipe, pinch, two-finger drag, chips |
| Walk | W A S D / arrow keys | drag on the left half (floating joystick) |
| Run | hold Shift | push the joystick to the edge |
| Game camera | drag to orbit, wheel to zoom | swipe on the right half, pinch to zoom |

The camera swings back behind the player while walking forward, pulls in instead of entering a building, and tree crowns dissolve near the camera and in front of the player.

## Structure

```
index.html              HUD markup (start menu + loader, place card, tools, map chips, toast, hints, joystick)
src/main.ts             entry
src/style.css           HUD styles (paper + ink); `data-modes` on each overlay vs `<body data-mode>`
src/game/
  app.ts                bootstrap, frame loop, mode switching (menu / map / play) with camera flights
  mapView.ts            map mode: orbit controls, landmark stops, menu idle orbit, camera flights
  play.ts               play mode: input -> player -> follow camera -> place labels
  sea.ts                toon sea: depth-based shallows, shore foam, wave streaks, horizon haze
  life.ts               drifting clouds and circling gulls
  sound/                area sound made with WebAudio (no audio files), mute toggle
    ambient.ts          per-area mixer: each area place fades its soundscape in and out by distance
    beach.ts            soft surf for bai_bien: overlapping waves over a far-off bed
    noise.ts            seamless looping brown / pink noise
  world.ts              renderer, paper sky, sun/ambient lighting, GLB loader (plain or base64 .txt)
  paint.ts              toon paint material (Blender SD_Paint parameters) + foliage wind and see-through
  ink.ts                normal/depth pass, colour pass and the ink composite
  collider.ts           what bodies bump into: the map's own triangles in one BVH (three-mesh-bvh), capsule
                        push-out, floors, water depth, the hill's skirts, the camera's sight lines
  mapInfo.ts            server/content/map.json: spawn, sea level, island bounds
  player.ts             movement: acceleration, turning, a capsule sliding along walls, steps, slopes, falls
  character.ts          low-poly pedestrian (nón lá) with procedural idle / walk / run
  followCamera.ts       third-person orbit camera that keeps out of walls (poles and crowns do not count)
  input.ts              keyboard, mouse, touch joystick, pinch
  places.ts             named landmarks and streets for the HUD
  phone/mapLand.json    the phone map's land and roads (written by scripts/map/move_game_content_v8.py)
  hud.ts                DOM overlay helpers
  storyCard.ts          the arrival card: the archetype's story and first steps, once per visit
  stories.ts            that text per archetype (written in i18n/stories.ts)
  langSwitch.ts         the language switch on the menu and in the tools
  i18n/                 every interface text, Vietnamese and English (see "Adding text")
  coords.ts             Blender <-> three.js axes, small math helpers
public/models/
  saigon_mini.glb       the map, Sài Gòn mini (export of models/saigon_mini.blend); also what bodies collide with
  saigon_q1_v5.glb      the v4 island, no longer loaded
scripts/
  inline.mjs            inlines the single-chunk build into one HTML file
  blender/export_glb.py exports the diorama GLB from the .blend
docs/
  PROJECT.md            project brief: style, palette, Blender conventions, landmark sheet
  img/                  renders of each landmark from the current map
```

## Adding text

Interface text never goes straight into the code or `index.html`; `npm run i18n:check` fails if it does.

- **In a module:** add a key to its catalog in `src/game/i18n/<module>.ts` (`const vi = {…}; const en: typeof vi = {…}`; tsc fails until the English is there), then read it as `L.key`, or `L.key(arg)` for text with values in it.
- **In `index.html`:** give the element `data-i18n="key"` (its text), `data-i18n-title`, `data-i18n-aria` or `data-i18n-placeholder`, and add the key to `src/game/i18n/page.ts`.
- **Game content** (quests, dialogue, places, apps, archetypes in `server/content/`) is a `{ "vi": …, "en": … }` pair: show it with `tx()`, place names with `placeTitle()` / `placeNote()`. `go test` fails on a text without its English.
- **Text that stays on screen** redraws itself on `onLangChange()`; text built each time something opens needs nothing more.

Coordinates: Blender is metres with Z up; three.js is Y up, so Blender (x, y, z) is three (x, z, -y). Places, NPC spots and `server/content/map.json` are in Blender XY.

## Changing the map

1. Edit the map. The current one is Sài Gòn mini, `models/saigon_mini.blend`, built by
   `scripts/blender/build_saigon_mini.py` from the layout `scripts/map/saigon_mini_layout.json` (made by
   `scripts/map/relayout_v8.py`, checked by `scripts/map/check_layout.py`; see `docs/map-v8-design.md`). The v4 island
   (`models/temp.sg.tinymap.blend`, `saigon_q1_*`) is kept but no longer loaded.
   Trees are scattered before the landmarks and houses round them were modelled: after rebuilding or adding a
   building, run `blender models/saigon_mini.blend --background --python scripts/blender/mini/clean_trees.py`
   (`-- --dry-run` to only list them) to remove the trees that grow through buildings, monuments, stairs or the hill
   road, stand inside or under a building, in a lake, float, or have a wall through a crown.
2. Export: `npm run map:export` writes `export/saigon_mini.glb`; copy it to `public/models/saigon_mini.glb` (the name
   `index.html` loads, `#stage data-model`). That is all collision needs: the game builds it from the GLB as it
   loads (`src/game/collider.ts`, about half a second). Whatever is in the model stops a body (walls, kerbs,
   trunks, railings; tree crowns too, below head height), except the lamp glows and lanterns; water stops it where
   it is deeper than 0.25 m. Open in `?debug` and call `sgGame.showCollider()` to see it (orange: the skirts under
   the hill's open rim; bridges and other built things get none, the space under them is open). To let players through something, leave it out of the
   model or give it a pass-through material (`PASS_THROUGH` in `collider.ts`); to stop them, model a wall.
   The spawn, sea level and island bounds are in `server/content/map.json`.
3. Places, NPCs and spawns are in `server/content/` (Blender XY). After moving things in the layout, run
   `python3 scripts/map/move_game_content_v8.py` (needs shapely): it puts them back on their streets and squares and
   rewrites the phone map.

## Next

- Finish phase 5: fewer meshes per avatar (draw calls in a crowd) and a check on a real mid-range Android ([docs/design/06-mvp-roadmap.md](docs/design/06-mvp-roadmap.md)). Built so far: accounts and archetypes, the phone, server-checked movement and the motorbike, NPCs and dialogue, quests and achievements, Crab Bike rides with the xu wallet, daily quests, asking the way (the co-op quest), moderation, and a 300-bot load test ([docs/design/README.md](docs/design/README.md#progress)).
- UBND building on the site kept free at the north end of Nguyễn Huệ.
