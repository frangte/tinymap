# Sài Gòn Q1

A walkable miniature of District 1, Ho Chi Minh City: an island in the sea with Chợ Bến Thành as the starting point and the landmarks laid out east of it roughly as in the real city. The map is modelled in Blender (`sg.tinymap.blend`) and shown in Three.js + TypeScript. The start menu offers two ways in: **[1] map** (orbit, zoom and fly to each landmark) and **[2] play** (start in front of the south gate of Chợ Bến Thành and walk anywhere on the streets, plazas, lawns and the beach). Everything is drawn with shaders only: a toon paint shader (colour variation, street grime, rain streaks, paper grain), a screen-space ink outline and a toon sea that reads the scene depth for shallows and shore foam. Clouds, gulls, swaying trees and code-generated area sound (soft surf on the beach for now) keep it alive.

**Start with [docs/PROJECT.md](docs/PROJECT.md)**: project context, the art style, the Blender file conventions and a sheet of every landmark with renders and reference photos.

## Run

```bash
npm install
npm run dev          # http://localhost:5173
npm run build        # type-check + production build into dist/
npm run build:single # one HTML file in dist-single/saigon-noir.html (models/ still served next to it)
```

The URL hash picks the mode: `#ban-do` (map), `#choi` (play), none (menu), so the back button and shared links work. `?debug` exposes the app as `window.sgGame` (player, follow camera, map view, nav grid, `go(mode)`, `step(n)`); `?debug&manual` stops the frame loop so a test can step it.

## Controls

| | Desktop | Touch |
|---|---|---|
| Menu | press 1 (map) or 2 (play); Esc goes back to the menu | tap a card |
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
  nav.ts                walkable heightfield: floor height, walkable flag, building tops
  player.ts             movement: acceleration, turning, wall sliding, kerbs and ramps
  character.ts          low-poly pedestrian (nón lá) with procedural idle / walk / run
  followCamera.ts       third-person orbit camera with building avoidance
  input.ts              keyboard, mouse, touch joystick, pinch
  places.ts             named landmarks and streets for the HUD
  hud.ts                DOM overlay helpers
  coords.ts             Blender <-> three.js axes, small math helpers
public/models/
  saigon_q1.glb         the diorama (export of sg.tinymap.blend)
  saigon_q1_nav.png     floor height + walkable flag, 0.35 m grid
  saigon_q1_roof.png    building tops on non-walkable cells (camera collision)
  saigon_q1_nav.json    grid origin, cell size, spawn point
scripts/
  inline.mjs            inlines the single-chunk build into one HTML file
  blender/export_glb.py exports the diorama GLB from the .blend
  blender/bake_nav.py   bakes the three nav files from the .blend
  blender/nav_boxes.py  the walk-map boxes (NAV collection: buildings nobody walks into) in the .blend
docs/
  PROJECT.md            project brief: style, palette, Blender conventions, landmark sheet
  img/                  renders of each landmark from the current map
```

Coordinates: Blender is metres with Z up; three.js is Y up, so Blender (x, y, z) is three (x, z, -y). The nav files are in Blender XY with row 0 at the southern edge.

## Changing the map

1. Edit the map. The current island is `models/temp.sg.tinymap.blend` (the v4 island with Bùi Viện and Tôn Đức Thắng): the shipped GLB imported back into Blender (one scene, meshes merged by material). Both scripts run on it. `sg.tinymap.blend` keeps the original objects (scene `SaigonDiorama`) of an older layout. Frozen versions: `versions/sg.tinymap_v1.blend`, `_v2`, `_v3`.
2. Export with `scripts/blender/export_glb.py`, shrink with the `gltf-transform` chain in its header to `export/saigon_q1.glb`, and copy that to `public/models/` under the name `index.html` loads (`#stage data-model`, now `saigon_q1_v5.glb`).
3. Re-bake the nav maps (about 15 s) and copy them to the server, which checks every move against its own copy:
   ```bash
   blender models/temp.sg.tinymap.blend --background --python scripts/blender/bake_nav.py -- public/models/saigon_q1
   npm run nav:sync
   ```
   Ground (grass, sand, asphalt, kerbs, road lines, rock, water) is told apart by material; anything else at body height blocks. A building nobody should walk into (or under) needs a `NAV_Solid_` box in the file's `NAV` collection: `scripts/blender/nav_boxes.py` makes the current ones, and its header explains the `NAV_Block_` and `NAV_Open_` boxes for fixing single spots. The boxes are hidden from the export.
4. New landmark or street names go in `src/game/places.ts` (Blender XY).

## Next

- Finish phase 5: fewer meshes per avatar (draw calls in a crowd) and a check on a real mid-range Android ([docs/design/06-mvp-roadmap.md](docs/design/06-mvp-roadmap.md)). Built so far: accounts and archetypes, the phone, server-checked movement and the motorbike, NPCs and dialogue, quests and achievements, Crab Bike rides with the xu wallet, daily quests, asking the way (the co-op quest), moderation, and a 300-bot load test ([docs/design/README.md](docs/design/README.md#progress)).
- UBND building on the site kept free at the north end of Nguyễn Huệ.
