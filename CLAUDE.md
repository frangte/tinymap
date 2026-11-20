# Sài Gòn Q1

Walkable low-poly District 1 diorama: map modelled in Blender (the current map is Sài Gòn mini, `models/saigon_mini.blend`, built by `scripts/blender/build_saigon_mini.py`; the v4 island `models/temp.sg.tinymap.blend` and the older `sg.tinymap.blend` are kept but no longer loaded, and the build still takes the older landmark models from `sg.tinymap.blend`), game in Three.js 0.160 + TypeScript (strict), built with Vite 5. README.md has the full structure and the map pipeline.

Read `docs/PROJECT.md` first for any map or modelling work: project status, art style rules, palette, Blender file conventions, and a landmark sheet with renders (`docs/img/`) and reference photos. Keep it up to date when a landmark changes or a new one is added (re-render its image). Ask Khoa before starting a big plan.

Open requests from Khoa and their status are in `docs/requirements.md`; the map is "Sài Gòn mini" (`docs/map-v5.md`; layout v8 in `docs/map-v8-design.md`, open map tasks in `docs/map-v5-tasks.md`); the house kit and new landmarks still need their own design approval before real modelling.

For gameplay, multiplayer and backend work (archetypes, quests, NPCs, phone, chat, persistence), read `docs/design/README.md` first: the agreed design, the decision log (which decisions are confirmed, which are only proposed), and the roadmap. Update the decision log when a decision changes.

## Commands

- `npm run dev` — dev server at http://localhost:5173 (`?debug` exposes `window.sgGame`, `?debug&manual` stops the loop for stepping)
- `npm run story` — the Xưởng truyện, the story editor at http://localhost:5173/tools/story/: draws and edits `server/content/` (quests, dialogue) and saves back to the JSON (dev only, localhost only; `?fixture` is a read-only sample with outcomes and flags; docs/design/14-story-tools.md). `npm run story:format-check` checks its JSON writer reproduces the content files exactly
- `npm run typecheck` — `tsc --noEmit`
- `npm run i18n:check` — fails on interface text typed outside the catalogs in `src/game/i18n/`
- `npm run build` — type-check + production build into `dist/`
- `npm run build:single` — one HTML file at `dist-single/saigon-noir.html` (serve `models/` next to it)
- `npm run map:export` — export the map: `models/saigon_mini.blend` → `export/saigon_mini.glb` (`export_glb.py`, then the gltf-transform chain); copy it to `public/models/saigon_mini.glb`
- `blender models/saigon_mini.blend --background --python scripts/blender/mini/clean_trees.py -- [--dry-run]` — remove the trees that grow through buildings, landmarks, stairs or roads (run after rebuilding the map or adding a building, before `npm run map:export`)
- `node scripts/map/layout_svg.mjs` and `/Applications/Blender.app/Contents/MacOS/Blender --background --factory-startup --python scripts/blender/build_saigon_mini.py -- models/saigon_mini.blend --render renders/blockout` — redraw the Sài Gòn mini plan and rebuild its blockout from `scripts/map/saigon_mini_layout.json` (needs `.blender-site/`, see docs/map-v5.md)

No test runner and no linter. Verify changes with `npm run typecheck` and `npm run i18n:check`, then play in the browser (in both languages: `?lang=vi`, `?lang=en`).

## Conventions

- Blender is the source of truth for the map. Never edit `public/models/*` by hand: re-export the GLB. What players bump into is the GLB itself (no bake): what is in the model stops them, what is not lets them through.
- Axes: Blender (x, y, z-up) = three (x, z, -y). Use `fromBlender()` from `src/game/coords.ts`. Places and `server/content/map.json` (spawn, sea level, island bounds) are in Blender XY.
- All world materials come from `paintMaterial()` (`src/game/paint.ts`), keyed by the Blender material name without `SD_`. Moving meshes pass `{ local: true }` so the paint noise doesn't swim.
- Every mesh added to the scene must be registered with `InkPass.track()`, otherwise the normal pass draws it with its colour material.
- The sun shadow map is baked once (`shadowMap.autoUpdate = false`); moving things use a blob shadow.
- Movement, the camera and every floor lookup go through `CityCollider` (`src/game/collider.ts`): the map's triangles in one three-mesh-bvh BVH, built once at load, a capsule body for the player. Never raycast the scene's meshes (`THREE.Raycaster` over the city) per frame; ask the collider. Water stops a body only where it is deeper than `WADE`; only the hill's open rim (terrain materials) gets collision-only skirts. A body steps up 0.5 m from any angle (it looks for the step past whatever pushed it back, so a 120 Hz screen climbs kerbs as 60 Hz does) and never goes where something over its head pushes it down. `?debug` has `sgGame.showCollider()`.
- The server checks speeds only (`server/internal/world/move.go`), not walls or heights: the game keeps its player out of walls, and the server takes its word (decision D21 in docs/design/README.md).
- The game is in Vietnamese and English. Interface text lives in the catalogs in `src/game/i18n/` (`en: typeof vi`), `index.html` text in `i18n/page.ts` via `data-i18n` keys, and content text in `server/content/` as `{vi, en}` pairs shown with `tx()`. Views that keep text on screen redraw on `onLangChange()`. README.md, "Adding text".

## Overrides for the ECC rules in `.claude/rules/ecc/`

- In-place mutation is intended in Three.js code. Reuse and mutate `Vector3` / `Matrix4` / `Quaternion` / `Color` scratch objects, buffers and instance matrices rather than allocating, especially in per-frame loops. This takes precedence over the "Immutability" sections of `common/coding-style.md` and `typescript/coding-style.md`.
- The TDD and 80% coverage requirements (`common/testing.md`, `common/development-workflow.md`) don't apply until a test runner is added.
