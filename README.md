# Sài Gòn Q1

A walkable miniature of District 1, Ho Chi Minh City. The map is modelled in Blender (`sg.tinymap.blend`) and shown in Three.js + TypeScript. The start menu offers two ways in: **[1] map** (orbit, zoom and fly to each landmark) and **[2] play** (start in front of the south gate of Chợ Bến Thành and walk anywhere on the streets, plazas, the Ba Son bridge and the Hồ Con Rùa plaza). Everything is drawn with shaders only: a toon paint shader (colour variation, street grime, rain streaks, paper grain) and a screen-space ink outline.

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
  world.ts              renderer, paper sky, sun/ambient lighting, GLB loader (plain or base64 .txt)
  paint.ts              toon paint material (Blender SD_Paint parameters) + see-through foliage
  ink.ts                colour + normal/depth passes and the ink composite
  nav.ts                walkable heightfield: floor height, walkable flag, building tops
  player.ts             movement: acceleration, turning, wall sliding, kerbs and ramps
  character.ts          low-poly pedestrian rig, dressed by a style, with procedural idle / walk / run
  characters.ts         the playable cast (colours + build switches) and the remembered pick
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
docs/
  PROJECT.md            project brief: style, palette, Blender conventions, landmark sheet
  img/                  renders of each landmark from the current map
```

Coordinates: Blender is metres with Z up; three.js is Y up, so Blender (x, y, z) is three (x, z, -y). The nav files are in Blender XY with row 0 at the southern edge.

## Changing the map

1. Edit `sg.tinymap.blend` (scene `SaigonDiorama`; collections `SD_World` and `SD_NoInk`). Frozen v1: `versions/sg.tinymap_v1.blend`.
2. Export with `scripts/blender/export_glb.py`, shrink with the `gltf-transform` chain in its header to `export/saigon_q1.glb`, and copy that to `public/models/saigon_q1.glb`.
3. Re-bake the nav maps:
   ```bash
   blender sg.tinymap.blend --background --python scripts/blender/bake_nav.py -- public/models/saigon_q1
   ```
   New buildings that the camera should not enter need their object name prefix in `ROOF_PREFIX` (and `SOLID_PREFIX` if nobody walks underneath).
4. New landmark or street names go in `src/game/places.ts` (Blender XY).

## Next

- NPCs and quests.
- Bùi Viện walking street (tube-house rows are reserved for it).
- UBND building at the north end of Nguyễn Huệ.
