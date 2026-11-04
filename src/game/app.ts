// Bootstrap and frame loop. Three modes share one renderer, scene and camera:
//   menu  the map slowly circles behind the start card ([1] map, [2] play)
//   map   free orbit / zoom / pan with flights to each landmark
//   play  third-person walking (see play.ts)
// The URL hash picks the mode (#ban-do, #choi; none = menu), so the back button and shared links work.
import * as THREE from 'three';
import { createRenderer, createScene, loadModel } from './world';
import { paintWorld, SEE_THROUGH } from './paint';
import { InkPass } from './ink';
import { NavGrid } from './nav';
import { PlayMode } from './play';
import { CameraFlight, MAP_STOPS, MENU_STOP, MapView } from './mapView';
import { Hud, type Mode } from './hud';
import { damp } from './coords';

const MODEL_BYTES = 3.55e6;
const MAX_PIXEL_RATIO = 1.5;
const HASH: Record<Mode, string> = { menu: '', map: 'ban-do', play: 'choi' };
/** vertical field of view: base for landscape, widened on portrait screens to keep this half-width (degrees) */
const FOV: Record<Mode, { base: number; halfWidth: number; max: number }> = {
  menu: { base: 35, halfWidth: 21, max: 75 },
  map: { base: 35, halfWidth: 21, max: 75 },
  play: { base: 45, halfWidth: 23, max: 68 },
};

function modeFromHash(): Mode {
  if (location.hash === `#${HASH.map}`) return 'map';
  if (location.hash === `#${HASH.play}`) return 'play';
  return 'menu';
}

/** Asset URLs come from <div id="stage" data-model data-nav> so a static host can swap in a base64 model. */
function assetUrls(stage: HTMLElement): { model: string; nav: string } {
  const d = stage.dataset;
  return { model: d.model ?? 'models/saigon_q1.glb', nav: d.nav ?? 'models/saigon_q1' };
}

export async function startApp(): Promise<void> {
  const hud = new Hud();
  hud.setMode('menu');
  if (!document.createElement('canvas').getContext('webgl2')) {
    hud.fail('Trình duyệt này không hỗ trợ WebGL 2, nên không mở được bản đồ.');
    return;
  }
  const renderer = createRenderer(hud.stage);
  renderer.domElement.setAttribute('aria-label', 'Sài Gòn Q1: Quận 1 thu nhỏ');
  renderer.domElement.addEventListener('webglcontextlost', (e) => {
    e.preventDefault();
    hud.fail('Mất kết nối với card đồ họa. Tải lại trang để xem tiếp.');
  });
  const scene = createScene();
  const camera = new THREE.PerspectiveCamera(35, 1, 0.25, 1500);
  const ink = new InkPass(camera);
  const map = new MapView(camera, renderer.domElement);
  const flight = new CameraFlight(camera);
  map.flyTo(MENU_STOP, true);
  map.idle = true;
  SEE_THROUGH.uSee.value = 0;

  let pixelRatio = Math.min(window.devicePixelRatio || 1, MAX_PIXEL_RATIO);
  const resize = (): void => {
    const w = hud.stage.clientWidth;
    const h = hud.stage.clientHeight;
    renderer.setPixelRatio(pixelRatio);
    renderer.setSize(w, h, false);
    camera.aspect = w / Math.max(1, h);
    camera.updateProjectionMatrix();
    ink.setSize(Math.floor(w * pixelRatio), Math.floor(h * pixelRatio), pixelRatio);
  };
  window.addEventListener('resize', resize);
  resize();

  let mode: Mode = 'menu';
  let play: PlayMode | null = null;
  let stop = 0;
  let clock = 0;
  const toPos = new THREE.Vector3();
  const toTarget = new THREE.Vector3();
  const fovGoal = (): number => {
    const f = FOV[mode];
    const keep = THREE.MathUtils.radToDeg(2 * Math.atan(Math.tan(THREE.MathUtils.degToRad(f.halfWidth)) / camera.aspect));
    return THREE.MathUtils.clamp(keep, f.base, f.max);
  };
  camera.fov = fovGoal();
  camera.updateProjectionMatrix();

  /** Switch mode with a camera flight; the HUD follows at once. */
  const go = (next: Mode): void => {
    if (!play || next === mode) return;
    const prev = mode;
    mode = next;
    hud.setMode(next);
    map.interactive = false;
    map.idle = false;
    if (prev === 'play') play.exit();
    // where the camera is looking now, whoever was driving it
    const looking = flight.active ? flight.lookAt.clone() : prev === 'play' ? play.target.clone() : map.controls.target.clone();
    flight.cancel();

    if (next === 'play') {
      const p = play;
      p.entryPose(toPos, toTarget);
      flight.start(looking, toPos, toTarget, () => {
        if (mode === 'play') p.enter();
      }, 1600);
      return;
    }
    SEE_THROUGH.uSee.value = 0;
    map.adopt(looking);
    if (next === 'map') {
      map.interactive = true;
      hud.selectStop(stop, MAP_STOPS[stop]);
      map.flyTo(MAP_STOPS[stop]);
    } else {
      map.flyTo(MENU_STOP, false, () => {
        if (mode === 'menu') map.idle = true;
      });
    }
  };
  const request = (next: Mode): void => {
    if (modeFromHash() === next) go(next);
    else location.hash = HASH[next];
  };
  window.addEventListener('hashchange', () => go(modeFromHash()));

  const tick = (dt: number, now: number, draw: boolean): void => {
    clock += dt;
    flight.update(now);
    play?.update(dt, now, clock, mode === 'play' && !flight.active);
    if (mode !== 'play' && !flight.active) map.update(now);
    const fov = fovGoal();
    if (Math.abs(camera.fov - fov) > 0.01) {
      camera.fov += (fov - camera.fov) * damp(4, dt);
      camera.updateProjectionMatrix();
    }
    if (draw) ink.render(renderer, scene, camera);
  };

  let last = performance.now();
  let frames = 0;
  let slowFrames = 0;
  let running = true;
  const frame = (now: number): void => {
    if (!running) return;
    requestAnimationFrame(frame);
    const dt = Math.min(0.05, Math.max(0, (now - last) / 1000));
    last = now;
    tick(dt, now, true);
    // drop resolution once if the GPU cannot keep up (two scene passes per frame)
    frames++;
    if (dt > 1 / 36) slowFrames++;
    if (frames === 120) {
      if (slowFrames > 60 && pixelRatio > 1) {
        pixelRatio = Math.max(1, pixelRatio - 0.25);
        resize();
      }
      frames = slowFrames = 0;
    }
  };
  const params = new URLSearchParams(location.search);
  // ?debug&manual: no frame loop, a test steps the simulation itself
  const manual = params.has('debug') && params.has('manual');
  if (!manual) requestAnimationFrame(frame);

  // ---- load the map while the menu shows its progress bar
  const urls = assetUrls(hud.stage);
  let world: THREE.Group;
  let nav: NavGrid;
  try {
    [world, nav] = await Promise.all([loadModel(urls.model, MODEL_BYTES, (l, t) => hud.progress(l, t)), NavGrid.load(urls.nav)]);
  } catch (err) {
    console.error(err);
    hud.fail(`Không tải được bản đồ (${err instanceof Error ? err.message : String(err)}). Tải lại trang để thử lại.`);
    running = false;
    return;
  }
  paintWorld(world);
  scene.add(world);
  ink.track(world);
  renderer.shadowMap.needsUpdate = true;

  const p = new PlayMode(nav, camera, hud, renderer.domElement);
  play = p;
  scene.add(p.player.avatar.root);
  ink.track(p.player.avatar.root);

  // ---- controls
  hud.buildStops(MAP_STOPS, (i) => {
    stop = i;
    hud.selectStop(i, MAP_STOPS[i]);
    map.flyTo(MAP_STOPS[i]);
  });
  map.onUserMove = () => hud.selectStop(-1, null);
  hud.modeMapButton.addEventListener('click', () => request('map'));
  hud.modePlayButton.addEventListener('click', () => request('play'));
  hud.toMapButton.addEventListener('click', () => request('map'));
  hud.toPlayButton.addEventListener('click', () => request('play'));
  hud.respawnButton.addEventListener('click', () => {
    p.spawnAtMarket();
    p.follow.snapBehind(p.player.position, p.player.heading);
    renderer.domElement.focus({ preventScroll: true });
  });
  hud.inkButton.addEventListener('click', () => {
    ink.enabled = !ink.enabled;
    hud.inkButton.setAttribute('aria-pressed', String(ink.enabled));
  });
  window.addEventListener('keydown', (e) => {
    if (e.metaKey || e.ctrlKey || e.altKey) return;
    if (mode === 'menu' && (e.key === '1' || e.key === '2')) request(e.key === '1' ? 'map' : 'play');
    else if (mode !== 'menu' && e.key === 'Escape') request('menu');
  });

  hud.ready();
  go(modeFromHash());

  if (params.has('debug')) {
    let simNow = performance.now();
    const step = (n: number, dtMs = 1000 / 60): void => {
      for (let i = 0; i < n; i++) {
        simNow += dtMs;
        tick(dtMs / 1000, simNow, i === n - 1);
      }
    };
    Object.assign(window, { sgGame: { play: p, player: p.player, follow: p.follow, map, nav, go: request, step, mode: () => mode } });
    if (manual) tick(0, simNow, true);
  }
}
