// Bootstrap and frame loop. Three modes share one renderer, scene and camera:
//   menu  the map slowly circles behind the start card ([1] map, [2] play)
//   map   free orbit / zoom / pan with flights to each landmark
//   play  third-person walking (see play.ts)
// The URL hash picks the mode (#ban-do, #choi; none = menu), so the back button and shared links work.
// Around every mode: the sea, drifting clouds and gulls, swaying trees; in play, each area's own sound.
import * as THREE from 'three';
import { createRenderer, createScene, loadModel } from './world';
import { paintWorld, FOLIAGE } from './paint';
import { InkPass } from './ink';
import { NavGrid } from './nav';
import { PlayMode } from './play';
import { CameraFlight, MAP_STOPS, MENU_STOP, MapView } from './mapView';
import { Hud, el, type Mode } from './hud';
import { damp, fromBlender } from './coords';
import { Sea } from './sea';
import { Life } from './life';
import { Ambient, type Ear } from './sound/ambient';
import { DressPanel } from './dressPanel';
import { defaultProfile, loadProfile, saveProfile, type Profile } from './profile';
import { isTyping } from './input';
import { RemotePlayers } from './remotePlayers';
import { NameTags, SELF } from './nameTags';
import { ChatBox } from './chatBox';
import { ArchetypePicker } from './archetypePicker';
import { StoryCard } from './storyCard';
import { storyFor } from './stories';
import { Phone } from './phone/phone';
import { allowedLook, appsFor, archetypeById, grants, type Archetype } from './archetypes';
import { sameLook } from './outfits';
import { NetClient, socketUrl, type NetStatus } from '../net/client';
import { Vehicles, type RiderView } from './vehicles';
import { Riding } from './riding';
import { ActionPad } from './actionPad';
import { emotesFor } from './motion';
import { Npcs } from './npcs';
import { Talk } from './talk';
import { QuestLog } from './questLog';
import { QuestTracker } from './questTracker';
import { ACHIEVEMENTS, STAMPS, STREAK_STAMPS, questById, titleName } from './quests';
import type { News } from '../net/protocol';
import { loadPose, savePose } from './lastPose';
import { Rides, xu } from './rides';
import { OfferBanner } from './offerBanner';
import { CallBanner } from './callBanner';
import { Mutes } from './mutes';
import { Directions } from './directions';

const MODEL_BYTES = 2.01e6;
/** distance from the island centre to the beach (m) */
const COAST_R = 92;
const MAX_PIXEL_RATIO = 1.5;
const HASH: Record<Mode, string> = { menu: '', map: 'ban-do', play: 'choi' };
/** a welcome placing us further than this from where we stand moves us there (m); closer, we stay put */
const WELCOME_MOVE = 0.3;
/** up to this far the move eases (the avatar slides over); further, the player and camera cut there (m) */
const WELCOME_EASE = 6;
/** how often the player's spot is kept in this browser for the next visit (ms) */
const POSE_SAVE_EVERY = 5000;
/** how often trip countdowns and the offer banner move on (ms) */
const RIDE_TICK = 200;
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
  const tags = new NameTags(el('tags'));
  const map = new MapView(camera, renderer.domElement);
  const flight = new CameraFlight(camera);
  map.flyTo(MENU_STOP, true);
  map.idle = true;
  FOLIAGE.uSee.value = 0;

  let pixelRatio = Math.min(window.devicePixelRatio || 1, MAX_PIXEL_RATIO);
  const resize = (): void => {
    const w = hud.stage.clientWidth;
    const h = hud.stage.clientHeight;
    renderer.setPixelRatio(pixelRatio);
    renderer.setSize(w, h, false);
    camera.aspect = w / Math.max(1, h);
    camera.updateProjectionMatrix();
    tags.setSize(w, h);
    ink.setSize(Math.floor(w * pixelRatio), Math.floor(h * pixelRatio), pixelRatio);
  };
  window.addEventListener('resize', resize);
  resize();

  let mode: Mode = 'menu';
  /** the ink outline per mode: on behind the menu, off over the map (its button turns it on), never in play */
  const inkWanted: Record<Mode, boolean> = { menu: true, map: false, play: false };
  const applyInk = (): void => {
    ink.enabled = mode !== 'play' && inkWanted[mode];
    hud.inkButton.setAttribute('aria-pressed', String(ink.enabled));
  };
  let play: PlayMode | null = null;
  let dress: DressPanel | null = null;
  let chat: ChatBox | null = null;
  let phone: Phone | null = null;
  /** closes the archetype picker when leaving play (replaced once the map has loaded) */
  let closePicker = (): void => {};
  /** closes the arrival card when leaving play (replaced once the map has loaded) */
  let closeStory = (): void => {};
  /** walks out of a conversation when leaving play (replaced once the map has loaded) */
  let closeTalk = (): void => {};
  /** what happens when play starts: pick an archetype, dress, or go straight online (replaced once loaded) */
  let arrive = (): void => {};
  /** replaced once the map has loaded and the panel exists */
  let openDress = (): void => {};
  // multiplayer: created with the world; joins once the player has a character
  let remotes: RemotePlayers | null = null;
  let npcs: Npcs | null = null;
  let net: NetClient | null = null;
  let vehicles: Vehicles | null = null;
  /** where each vehicle's rider is drawn this frame (replaced once the map has loaded) */
  let riderView = (_id: number): RiderView | null => null;
  /** draws a passenger where its rider is drawn this frame (replaced once the map has loaded) */
  let seatPassenger = (_id: number, _at: RiderView): void => {};
  /** the label of the action button, refreshed every frame (replaced once loaded) */
  let actionLabel = (): string | null => null;
  /** riding bookkeeping every frame (replaced once loaded) */
  let updateRiding = (_now: number): void => {};
  let pad: ActionPad | null = null;
  /** the quest tracker under the place card (replaced once the map has loaded) */
  let tracker: QuestTracker | null = null;
  const forward = new THREE.Vector3();
  let poseSavedAt = 0;
  /** replaced once the map has loaded */
  let goOnline = (): void => {};
  // the saved character, or a fresh one that the dressing panel asks about on the first "Vào game"
  const saved = loadProfile();
  let profile: Profile = saved ?? defaultProfile();
  let dressed = saved !== null;
  /** what the player's archetype grants (apps, uniforms) */
  let caps = grants(profile.archetype);
  let sea: Sea | null = null;
  let life: Life | null = null;
  const ambient = new Ambient();
  /** where the player hears from (Blender XY) */
  const ear: Ear = { x: 0, y: 0 };
  const islandCenter = new THREE.Vector3();
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
    applyInk();
    hud.setMode(next);
    map.interactive = false;
    map.idle = false;
    if (prev === 'play') {
      dress?.dismiss();
      chat?.close(false);
      phone?.close();
      closePicker();
      closeStory();
      closeTalk();
      play.exit();
    }
    // where the camera is looking now, whoever was driving it
    const looking = flight.active ? flight.lookAt.clone() : prev === 'play' ? play.target.clone() : map.controls.target.clone();
    flight.cancel();

    if (next === 'play') {
      const p = play;
      p.entryPose(toPos, toTarget);
      flight.start(looking, toPos, toTarget, () => {
        if (mode !== 'play') return;
        p.enter();
        arrive();
      }, 1600);
      return;
    }
    FOLIAGE.uSee.value = 0;
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
    FOLIAGE.uTime.value = clock;
    flight.update(now);
    play?.update(dt, now, clock, mode === 'play' && !flight.active);
    if (mode !== 'play' && !flight.active) map.update(now);
    // others: hidden beyond walking distance in play mode, all of them over the map
    remotes?.update(dt, now, clock, mode === 'play' ? camera : null);
    npcs?.update(dt, now, clock, mode === 'play' ? camera.position : null);
    vehicles?.update(dt, riderView, mode === 'play' ? camera.position : null, seatPassenger);
    if (draw && remotes && npcs && play && mode === 'play') tags.update(camera, now, remotes, npcs, play.player.position);
    if (play && mode === 'play') {
      const pl = play.player;
      const pp = pl.position;
      net?.sendState(pp.x, pp.y, pp.z, pl.heading, pl.flags, now);
      pad?.setUse(play.controlling ? actionLabel() : null);
      updateRiding(now);
      if (tracker) {
        play.follow.forward(forward);
        tracker.update(now, pl.position, forward);
      }
      if (now - poseSavedAt > POSE_SAVE_EVERY) {
        poseSavedAt = now;
        savePose(pp.x, pp.z, pl.heading);
      }
    }
    sea?.update(clock, camera);
    life?.update(clock, dt);
    // each area sounds of itself (only the beach for now), heard where the player is; quiet over the map
    if (play && mode === 'play') {
      ear.x = play.player.position.x;
      ear.y = -play.player.position.z;
      ambient.update(ear);
    } else {
      ambient.update(null);
    }
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
  p.player.avatar.setLook(profile.look);
  tags.setSelfName(profile.name);
  // the archetype's start (the market gate without one), its tool button label, badge and apps
  const applyArchetype = (): void => {
    const a = archetypeById(profile.archetype);
    p.setHome(a?.spawn ?? null);
    hud.respawnButton.textContent = a?.spawn.button.vi ?? 'Về chợ';
    hud.respawnButton.title = a ? 'Quay lại chỗ xuất phát' : 'Quay lại cổng chợ Bến Thành';
    tags.setSelfArch(profile.archetype);
    phone?.setApps(serverApps ?? appsFor(caps));
    if (pad) setEmotes();
  };
  /** the phone apps the server sent; null until it has (offline, an older server): then the archetype decides */
  let serverApps: string[] | null = null;
  applyArchetype();
  // back where we left off, if this browser remembers and the spot is still walkable (the server may move us)
  const lastSpot = profile.archetype ? loadPose() : null;
  if (lastSpot && nav.isWalkable(lastSpot.x, lastSpot.z)) p.player.spawn(lastSpot.x, lastSpot.z, lastSpot.h);
  else p.spawnAtHome();
  scene.add(p.player.avatar.root);
  ink.track(p.player.avatar.root);
  window.addEventListener('pagehide', () => savePose(p.player.position.x, p.player.position.z, p.player.heading));

  // ---- dressing: live preview on the player, saved on "Vào phố", reverted on "Huỷ"
  const closeDress = (): void => {
    p.setDressing(false);
    renderer.domElement.focus({ preventScroll: true });
  };
  const panel = new DressPanel({
    preview: (look) => p.player.avatar.setLook(look),
    done: (next) => {
      profile = next;
      dressed = true;
      saveProfile(next);
      p.player.avatar.setLook(next.look);
      tags.setSelfName(next.name);
      closeDress();
      welcome();
      netClient.setProfile(next);
    },
    cancel: () => {
      p.player.avatar.setLook(profile.look);
      closeDress();
      // a returning player who just picked an archetype has not gone online yet
      welcome();
    },
  });
  dress = panel;
  openDress = (): void => {
    if (mode !== 'play' || flight.active || panel.isOpen) return;
    chatBox.close(false);
    // the dressing panel takes over the screen: the conversation ends
    talk.close();
    p.setDressing(true);
    panel.open(profile, !dressed, caps);
  };
  hud.dressButton.addEventListener('click', () => openDress());

  // ---- chat: the log bottom left, bubbles over heads (tags); lines show when the server sends them back
  const backToGame = (): void => renderer.domElement.focus({ preventScroll: true });
  // players this one chose not to hear: their lines and bubbles are hidden here
  const mutes = new Mutes();
  const chatBox: ChatBox = new ChatBox({
    send: (text, ch) => netClient.sendChat(text, ch),
    opened: () => p.input.release(),
    closed: backToGame,
    muted: (id, name) => mutes.has(id, name),
    mute: (id, name) => mutes.add(id, name),
    report: (id) => netClient.sendReport(id),
  });
  chat = chatBox;
  const openChat = (): void => {
    // while talking the chat log makes room for the conversation sheet
    if (mode !== 'play' || flight.active || panel.isOpen || story.isOpen || !dressed || talk.isOpen) return;
    chatBox.open();
  };

  // ---- phones: Đổi đồ / Nam·Nữ / Chat behind the round button bottom right
  const switchBody = (): void => {
    if (mode !== 'play' || flight.active || panel.isOpen || !dressed) return;
    const body = profile.look.body === 'man' ? 'woman' : 'man';
    profile = { ...profile, look: { ...profile.look, body } };
    saveProfile(profile);
    p.player.avatar.setLook(profile.look);
    netClient.setProfile(profile);
  };

  // ---- "Bạn là ai?": once, before the first dressing (and once for players from before archetypes)
  const choose = (a: Archetype): void => {
    caps = grants(a.id);
    riding.leave();
    // a new player starts in the archetype's clothes; a returning one keeps theirs, minus what it may not wear
    const look = dressed ? allowedLook(profile.look, caps) : { body: profile.look.body, outfit: a.look.outfit, hat: a.look.hat };
    profile = { ...profile, archetype: a.id, look };
    if (dressed) saveProfile(profile);
    p.player.avatar.setLook(look);
    applyArchetype();
    p.spawnAtHome();
    p.follow.snapBehind(p.player.position, p.player.heading);
    riding.start();
    p.setDressing(false);
    // the dressing panel next, so a returning player also sees what the archetype adds
    openDress();
  };
  const picker = new ArchetypePicker({
    preview: (a) => {
      if (!dressed) p.player.avatar.setLook({ body: profile.look.body, outfit: a.look.outfit, hat: a.look.hat });
    },
    pick: choose,
  });
  closePicker = (): void => {
    if (!picker.isOpen) return;
    picker.close();
    p.setDressing(false);
    p.player.avatar.setLook(profile.look);
  };
  // ---- the arrival card: who you are and what to do, once per visit, then online
  const story = new StoryCard({
    done: () => {
      p.setDressing(false);
      renderer.domElement.focus({ preventScroll: true });
      goOnline();
    },
  });
  closeStory = (): void => {
    if (!story.isOpen) return;
    story.close();
    p.setDressing(false);
  };
  let storyTold = false;
  const welcome = (): void => {
    const a = archetypeById(profile.archetype);
    if (storyTold || !a || mode !== 'play') {
      goOnline();
      return;
    }
    storyTold = true;
    chatBox.close(false);
    talk.close();
    p.setDressing(true);
    story.open(a, storyFor(a.id));
  };
  arrive = (): void => {
    if (!profile.archetype) {
      chatBox.close(false);
      p.setDressing(true);
      picker.open();
    } else if (!dressed) openDress();
    else welcome();
  };

  // ---- multiplayer: everyone else on the island, and our own pose streamed out (tick)
  const others = new RemotePlayers(scene, ink, (x, z) => nav.smoothFloor(x, z, 0));
  remotes = others;
  // ---- quests: the server's word on them, shown by the tracker, the phone and toasts
  const log = new QuestLog();
  log.onChange(() => tags.setSelfTitle(log.title));
  /** what the progress says just happened: a toast for the biggest thing, a phone notice for each */
  const celebrate = (news: readonly News[], opened: readonly string[]): void => {
    let toast: { name: string; note: string } | null = null;
    for (const n of news) {
      if (n.kind === 'quest') {
        const q = questById(n.id);
        if (!q) continue;
        const paid = q.xu > 0 ? ` · +${xu(q.xu)}` : '';
        ph.notify(q.daily ? 'Xong nhiệm vụ hằng ngày' : 'Hoàn thành nhiệm vụ', q.title.vi + paid);
        if (!toast || toast.note !== 'Thành tích mới') toast = { name: q.title.vi, note: (q.daily ? 'Nhiệm vụ hằng ngày' : 'Hoàn thành nhiệm vụ') + paid };
      } else if (n.kind === 'achievement') {
        const a = ACHIEVEMENTS.find((x) => x.id === n.id);
        if (!a) continue;
        ph.notify('Thành tích mới', a.title.vi);
        toast = { name: a.title.vi, note: 'Thành tích mới' };
      } else if (n.kind === 'title') {
        const t = titleName(n.id);
        if (t) ph.notify('Danh hiệu mới', `${t}. Đổi danh hiệu trong app Hồ sơ.`);
      } else {
        const s = STAMPS.find((x) => x.id === n.id);
        if (!s) continue;
        const where = STREAK_STAMPS.includes(s) ? 'mục Hằng ngày (app Nhiệm vụ)' : 'Sổ tem (app Cẩm nang)';
        ph.notify('Nhận tem', `${s.name.vi}: xem trong ${where}.`);
      }
    }
    for (const id of opened) {
      const q = questById(id);
      if (q) ph.notify('Nhiệm vụ mới', q.title.vi);
    }
    if (toast && mode === 'play') hud.toast(toast);
  };
  // ---- NPCs: drawn at their spots from the start; online the server's take their place
  const cast = new Npcs(scene, ink, (x, z) => nav.smoothFloor(x, z, 0));
  npcs = cast;
  let netStatus: NetStatus = 'connecting';
  const showOnline = (): void => hud.setOnline(netStatus, others.count + 1);
  const netClient: NetClient = new NetClient(socketUrl(hud.stage), {
    status: (s) => {
      netStatus = s;
      chatBox.setOnline(s === 'online');
      showOnline();
    },
    welcome: (players, lines, arch, apps, pose, list, npcList) => {
      others.clear();
      talk.reset();
      rides.reset();
      cast.reset(npcList);
      const now = performance.now();
      for (const info of players) others.add(info, now);
      chatBox.history(lines, netClient.selfId);
      adoptArchetype(arch);
      serverApps = apps;
      applyArchetype();
      riding.welcome(list);
      // the server says where we are: where we left off last time, or the start
      if (pose) {
        const pp = p.player.position;
        const off = Math.hypot(pose[0] - pp.x, pose[2] - pp.z);
        if (off > WELCOME_EASE) {
          p.player.spawn(pose[0], pose[2], pose[3]);
          p.follow.snapBehind(p.player.position, p.player.heading);
        } else if (off > WELCOME_MOVE) {
          p.player.correct(pose[0], pose[1], pose[2]);
        }
      }
      if (!welcomed) {
        welcomed = true;
        const a = archetypeById(profile.archetype);
        ph.notify('Chào mừng tới Quận 1', a ? `Bạn là ${a.name.vi}. ${a.pitch.vi}` : 'Đi dạo, trò chuyện, ghé các công trình.');
      }
      showOnline();
    },
    join: (info) => {
      others.add(info, performance.now());
      chatBox.system(`${info.name} vào phố`);
      showOnline();
    },
    leave: (id) => {
      const gone = others.get(id);
      if (gone) chatBox.system(`${gone.name} rời phố`);
      others.remove(id);
      showOnline();
    },
    profile: (id, name, look, title) => {
      const before = others.get(id)?.name;
      if (before !== undefined && before !== name) chatBox.system(`${before} đổi tên thành ${name}`);
      others.setProfile(id, name, look, title);
    },
    progress: (pr) => {
      // quests that started just now (not on arriving: those were already there)
      const had = log.known ? new Set(log.active().map((q) => q.def.id)) : null;
      log.set(pr);
      const opened = had ? log.active().map((q) => q.def.id).filter((id) => !had.has(id)) : [];
      // the first progress of a visit says where things stand: nothing in it is news
      celebrate(had ? pr.news : [], opened);
    },
    snap: (rows, receivedAt, out) => {
      others.applySnap(rows, receivedAt, netClient.selfId, out);
      cast.applySnap(rows, receivedAt);
    },
    correct: (pose) => p.player.correct(pose[0], pose[1], pose[2]),
    vehicle: (v) => riding.vehicle(v),
    vehicleGone: (id) => riding.gone(id),
    mountRefused: () => riding.refused(),
    chat: (line) => {
      const self = line.id === netClient.selfId;
      chatBox.line(line, netClient.selfId);
      if (!self && !line.npc && mutes.has(line.id, line.name)) return;
      tags.say(self ? SELF : line.id, line.text, performance.now());
    },
    chatRefused: (code) => chatBox.refused(code),
    notice: (text) => chatBox.system(text),
    interaction: (i) => talk.interaction(i),
    interactionEnd: (sid, reason) => talk.ended(sid, reason),
    rateLimited: () => chatBox.rateLimited(),
    ride: (m) => rides.handle(m),
    way: (m) => directions.handle(m),
    reset: () => {
      others.clear();
      log.clear();
      talk.reset();
      rides.reset();
      directions.reset();
      cast.local();
      riding.offline();
      showOnline();
    },
  });
  net = netClient;

  // ---- vehicles: parked bikes, and bikes under their riders; the action pad and E get on and off
  const bikes = new Vehicles(scene, ink);
  vehicles = bikes;
  const riding: Riding = new Riding({
    player: p.player,
    vehicles: bikes,
    net: netClient,
    nav,
    starter: () => archetypeById(profile.archetype)?.vehicle ?? '',
    notify: (text) => chatBox.system(text),
  });
  const view: RiderView = { position: new THREE.Vector3(), heading: 0, speed: 0, lean: 0 };
  riderView = (id) => {
    if (id === riding.self) {
      view.position.copy(p.player.drawn);
      view.heading = p.player.heading;
      view.speed = p.player.speed;
      view.lean = p.player.lean;
      return view;
    }
    const r = others.get(id);
    if (!r || !r.inView || !r.avatar.root.visible) return null;
    view.position.copy(r.position);
    view.heading = r.heading;
    view.speed = r.speed;
    view.lean = r.lean;
    return view;
  };
  seatPassenger = (id, at) => {
    if (id !== riding.self) others.get(id)?.sitBehind(at);
    else if (p.player.seated !== 0) p.player.carry(at.position, at.heading, at.lean, at.speed);
  };
  // ---- talking to NPCs: next to one on foot, the action button (E) says hello; the sheet shows the conversation
  const talk: Talk = new Talk({ net: netClient, npcs: cast, player: p.player, notify: (text) => chatBox.system(text), done: backToGame });
  closeTalk = (): void => talk.close();
  // ---- Crab Bike trips: booked and driven from the phone; the action button takes the trip's next step
  const rides: Rides = new Rides({
    net: netClient,
    player: p.player,
    other: (id) => {
      const r = others.get(id);
      return r?.inView ? r : null;
    },
    say: (text) => chatBox.system(text),
    notify: (title, text) => ph.notify(title, text),
    openApp: (id) => {
      if (mode === 'play' && !panel.isOpen) ph.open(id);
    },
  });
  const offerBanner = new OfferBanner(rides, () => p.player.position);
  // ---- asking the way (D18): a Traveler asks from the Cẩm nang; a Citizen gets a call and answers with the button
  const directions: Directions = new Directions({
    net: netClient,
    other: (id) => {
      const r = others.get(id);
      return r?.inView ? r : null;
    },
    self: () => p.player.position,
    say: (text) => chatBox.system(text),
    notify: (title, text) => ph.notify(title, text),
  });
  const callBanner = new CallBanner(directions, () => netClient.serverNow());
  let rideTickAt = 0;
  // a trip's step first, then answering an asker; on the bike the button gets off; on foot an NPC in reach comes
  // before the bike
  actionLabel = () => rides.label() ?? directions.label() ?? (p.player.riding ? riding.label() : talk.label() ?? riding.label());
  updateRiding = (now) => {
    riding.update(now);
    if (now - rideTickAt < RIDE_TICK) return;
    rideTickAt = now;
    rides.update();
    offerBanner.update();
    directions.update();
    callBanner.update();
  };
  p.onUse = () => {
    if (rides.use()) return;
    if (directions.use()) return;
    if (!p.player.riding && talk.near()) talk.use();
    else riding.use();
  };
  p.pressDigit = (n) => talk.press(n);
  pad = new ActionPad({
    jump: () => p.jump(),
    use: () => p.use(),
    emote: (code) => p.emote(code),
    done: () => renderer.domElement.focus({ preventScroll: true }),
  });
  const setEmotes = (): void => {
    const list = emotesFor(caps);
    pad?.setEmotes(list);
    p.emoteKeys = list.map((e) => e.code);
  };
  setEmotes();
  // a returning driver finds their bike beside them (the server's replaces it once online)
  riding.start();
  goOnline = (): void => {
    if (dressed) netClient.start(profile);
  };
  let welcomed = false;
  /** the server keeps the archetype a character picked first: if it differs from ours, the server's wins */
  const adoptArchetype = (arch: string): void => {
    if (!arch || arch === profile.archetype || !archetypeById(arch)) return;
    caps = grants(arch);
    const look = allowedLook(profile.look, caps);
    if (!sameLook(look, profile.look)) p.player.avatar.setLook(look);
    profile = { ...profile, archetype: arch, look };
    saveProfile(profile);
    netClient.setProfile(profile);
  };

  // ---- the phone, bottom right: the apps the server sent (or that the archetype gives, offline)
  const ph = new Phone({
    profile: () => profile,
    online: () => netStatus === 'online',
    self: () => ({ position: p.player.position, heading: p.player.heading }),
    others: () => others.values(),
    landmarksSeen: () => p.landmarksSeen,
    channel: () => chatBox.channel,
    setChannel: (ch) => chatBox.setChannel(ch),
    openChat: () => openChat(),
    dress: () => openDress(),
    switchBody: () => {
      switchBody();
      ph.rerender();
    },
    quests: () => log,
    chooseTitle: (id) => netClient.sendTitle(id),
    rides: () => rides,
    directions: () => directions,
    mutes: () => mutes,
  });
  const questTracker = new QuestTracker(log, () => {
    if (mode !== 'play' || panel.isOpen) return;
    // while guiding, the tracker opens the app of what it guides to: a trip's, or the Cẩm nang for an ask
    if (!questTracker.guiding) ph.open('quests');
    else if (rides.guide()) ph.open(rides.ride?.role === 'rider' ? 'crab_driver' : 'crab');
    else ph.open(directions.calls.size > 0 ? 'quests' : 'guide');
  });
  tracker = questTracker;
  const guide = (): void => questTracker.guideTo(rides.guide() ?? directions.guide());
  rides.onChange(guide);
  directions.onChange(guide);
  phone = ph;
  applyArchetype();
  p.onLandmark = (place) => ph.notify(`Đã ghé ${place.name}`, place.note);

  const [icx, icy] = nav.meta.island_center ?? [36, 8];
  fromBlender(icx, icy, 0, islandCenter);
  sea = new Sea(ink, camera, nav.meta.sea_level ?? -0.45, islandCenter);
  scene.add(sea.mesh);
  life = new Life(islandCenter, COAST_R);
  scene.add(life.root);
  ink.track(life.root);

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
    // the one jump the server allows: home, and the bike comes along
    riding.leave();
    p.spawnAtHome();
    p.follow.snapBehind(p.player.position, p.player.heading);
    const pp = p.player.position;
    netClient.sendRespawn(pp.x, pp.y, pp.z, p.player.heading);
    riding.bringBike();
    renderer.domElement.focus({ preventScroll: true });
  });
  // sound starts with the first click (browsers block audio until then)
  hud.soundButton.setAttribute('aria-pressed', String(ambient.enabled));
  window.addEventListener('pointerdown', () => {
    if (ambient.enabled) ambient.start();
  });
  window.addEventListener('keydown', () => {
    if (ambient.enabled) ambient.start();
  }, { once: true });
  hud.soundButton.addEventListener('click', () => {
    hud.soundButton.setAttribute('aria-pressed', String(ambient.toggle()));
  });
  hud.inkButton.addEventListener('click', () => {
    inkWanted[mode] = !inkWanted[mode];
    applyInk();
  });
  window.addEventListener('keydown', (e) => {
    if (e.metaKey || e.ctrlKey || e.altKey) return;
    // Esc closes the dressing panel first (also from inside its name field); other keys typed into a field stay there
    if (panel.isOpen && e.key === 'Escape') {
      e.preventDefault();
      panel.dismiss();
      return;
    }
    if (story.isOpen && e.key === 'Escape') {
      e.preventDefault();
      story.dismiss();
      return;
    }
    if (isTyping(e)) return;
    if (mode === 'play' && talk.isOpen && e.key === 'Escape') {
      e.preventDefault();
      talk.close();
      return;
    }
    if (mode === 'play' && ph.isOpen && e.key === 'Escape') {
      e.preventDefault();
      ph.close();
      return;
    }
    // P: the phone
    if (mode === 'play' && e.code === 'KeyP' && !panel.isOpen && !picker.isOpen && !story.isOpen) {
      e.preventDefault();
      if (ph.isOpen) ph.close();
      else ph.open();
      return;
    }
    // Enter / T: chat (Enter on a focused button presses the button instead)
    if (mode === 'play' && (e.key === 'Enter' || e.code === 'KeyT') && !(e.target instanceof HTMLButtonElement)) {
      // keep the T out of the field that is about to take focus
      e.preventDefault();
      openChat();
      return;
    }
    if (mode === 'menu' && (e.key === '1' || e.key === '2')) request(e.key === '1' ? 'play' : 'map');
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
    Object.assign(window, { sgGame: { play: p, player: p.player, follow: p.follow, map, nav, scene, camera, ink, renderer, remotes: others, npcs: cast, talk, quests: log, rides, net: netClient, chat: chatBox, phone: ph, tags, vehicles: bikes, riding, go: request, step, mode: () => mode } });
    if (manual) tick(0, simNow, true);
  }
}
