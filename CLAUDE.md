# Sài Gòn Q1

Walkable low-poly District 1 diorama: map modelled in Blender (`sg.tinymap.blend`), game in Three.js 0.160 + TypeScript (strict), built with Vite 5. README.md has the full structure and the map pipeline.

Read `docs/PROJECT.md` first for any map or modelling work: project status, art style rules, palette, Blender file conventions, and a landmark sheet with renders (`docs/img/`) and reference photos. Keep it up to date when a landmark changes or a new one is added (re-render its image). Ask Khoa before starting a big plan.

For gameplay, multiplayer and backend work (archetypes, quests, NPCs, phone, chat, persistence), read `docs/design/README.md` first: the agreed design, the decision log (which decisions are confirmed, which are only proposed), and the roadmap. Update the decision log when a decision changes.

## Commands

- `npm run dev` — dev server at http://localhost:5173 (`?debug` exposes `window.sgGame`, `?debug&manual` stops the loop for stepping)
- `npm run typecheck` — `tsc --noEmit`
- `npm run build` — type-check + production build into `dist/`
- `npm run build:single` — one HTML file at `dist-single/saigon-noir.html` (serve `models/` next to it)
- `blender sg.tinymap.blend --background --python scripts/blender/export_glb.py -- export/saigon_q1_raw.glb` — export the GLB (then the gltf-transform chain in that file's header)
- `blender sg.tinymap.blend --background --python scripts/blender/bake_nav.py -- public/models/saigon_q1` — re-bake the nav maps after a map change, then `npm run nav:sync` (the server checks moves against its own embedded copy; `go test` fails until they match)

No test runner and no linter. Verify changes with `npm run typecheck`, then play in the browser.

## Conventions

- Blender is the source of truth for the map. Never edit `public/models/*` by hand: re-export the GLB and re-bake the nav maps.
- Axes: Blender (x, y, z-up) = three (x, z, -y). Use `fromBlender()` from `src/game/coords.ts`. Places and the nav metadata are in Blender XY.
- All world materials come from `paintMaterial()` (`src/game/paint.ts`), keyed by the Blender material name without `SD_`. Moving meshes pass `{ local: true }` so the paint noise doesn't swim.
- Every mesh added to the scene must be registered with `InkPass.track()`, otherwise the normal pass draws it with its colour material.
- The sun shadow map is baked once (`shadowMap.autoUpdate = false`); moving things use a blob shadow.
- Movement and the camera only read the nav grid; never raycast the city meshes per frame.

## Overrides for the ECC rules in `.claude/rules/ecc/`

- In-place mutation is intended in Three.js code. Reuse and mutate `Vector3` / `Matrix4` / `Quaternion` / `Color` scratch objects, buffers and instance matrices rather than allocating, especially in per-frame loops. This takes precedence over the "Immutability" sections of `common/coding-style.md` and `typescript/coding-style.md`.
- The TDD and 80% coverage requirements (`common/testing.md`, `common/development-workflow.md`) don't apply until a test runner is added.
