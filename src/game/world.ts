// Renderer, paper sky, two-tone sun/ambient lighting and the diorama model loader.
import * as THREE from 'three';
import { GLTFLoader } from 'three/examples/jsm/loaders/GLTFLoader.js';
import { MeshoptDecoder } from 'three/examples/jsm/libs/meshopt_decoder.module.js';
import { fromBlender } from './coords';

export function createRenderer(container: HTMLElement): THREE.WebGLRenderer {
  const renderer = new THREE.WebGLRenderer({ antialias: false, powerPreference: 'high-performance' });
  renderer.outputColorSpace = THREE.SRGBColorSpace;
  renderer.toneMapping = THREE.NoToneMapping;
  renderer.shadowMap.enabled = true;
  renderer.shadowMap.type = THREE.PCFSoftShadowMap;
  renderer.shadowMap.autoUpdate = false; // the city never moves: bake the sun shadow once
  renderer.domElement.tabIndex = 0;
  container.appendChild(renderer.domElement);
  return renderer;
}

function paperSky(): THREE.Texture {
  // cool blue-grey at the top, warm paper at the horizon
  const c = document.createElement('canvas');
  c.width = 4;
  c.height = 256;
  const g = c.getContext('2d');
  if (g) {
    const gr = g.createLinearGradient(0, 0, 0, 256);
    gr.addColorStop(0, '#A9BCC6');
    gr.addColorStop(0.55, '#D9D6CB');
    gr.addColorStop(1, '#EFE4D0');
    g.fillStyle = gr;
    g.fillRect(0, 0, 4, 256);
  }
  const tex = new THREE.CanvasTexture(c);
  tex.colorSpace = THREE.SRGBColorSpace;
  return tex;
}

const SUN_DIR = new THREE.Vector3(-0.5506, 0.7708, -0.3204).normalize();

/** Scene with the paper sky and the Blender LIGHT_TINT / SHADOW_TINT two-tone lighting; aim the sun with aimSun. */
export function createScene(): { scene: THREE.Scene; sun: THREE.DirectionalLight } {
  const scene = new THREE.Scene();
  scene.background = paperSky();
  const LIGHT_TINT = new THREE.Color('#FFF2DC');
  const SHADOW_TINT = new THREE.Color('#717BA2');
  scene.add(new THREE.AmbientLight(SHADOW_TINT, Math.PI));
  const sun = new THREE.DirectionalLight(LIGHT_TINT.clone().sub(SHADOW_TINT), Math.PI);
  sun.castShadow = true;
  const res = Math.min(window.innerWidth, window.innerHeight) < 700 ? 2048 : 4096;
  sun.shadow.mapSize.set(res, res);
  scene.add(sun, sun.target);
  aimSun(sun, fromBlender(0, 0, 0), 160);
  return { scene, sun };
}

/**
 * Point the sun's shadow at a square `half` metres each way round `center` (three.js). The map is too big for one
 * shadow map at street detail, so the app moves it with the view: tight round the player, the whole map from
 * high up. Re-bake the shadow map after (renderer.shadowMap.needsUpdate).
 */
export function aimSun(sun: THREE.DirectionalLight, center: THREE.Vector3, half: number): void {
  const dist = half + 300;
  sun.target.position.copy(center);
  sun.position.copy(center).addScaledVector(SUN_DIR, dist);
  sun.target.updateMatrixWorld();
  sun.updateMatrixWorld();
  const cam = sun.shadow.camera;
  Object.assign(cam, { left: -half, right: half, top: half, bottom: -half, near: 1, far: dist + 2 * half + 100 });
  cam.updateProjectionMatrix();
  // tuned at half 125 (far 560, 6 cm texels): keep the same bias in metres and one texel of normal offset
  sun.shadow.bias = -0.0004 * (560 / cam.far);
  sun.shadow.normalBias = Math.max(0.06, (2 * half) / sun.shadow.mapSize.x);
}

export type Progress = (loaded: number, total: number) => void;

/** Read a response body while reporting progress. */
async function readBody(res: Response, totalHint: number, onProgress: Progress): Promise<Uint8Array> {
  const total = Number(res.headers.get('content-length')) || totalHint;
  if (!res.body) {
    const buf = new Uint8Array(await res.arrayBuffer());
    onProgress(buf.byteLength, buf.byteLength);
    return buf;
  }
  const reader = res.body.getReader();
  const chunks: Uint8Array[] = [];
  let loaded = 0;
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    chunks.push(value);
    loaded += value.byteLength;
    onProgress(loaded, Math.max(total, loaded));
  }
  const out = new Uint8Array(loaded);
  let o = 0;
  for (const c of chunks) {
    out.set(c, o);
    o += c.byteLength;
  }
  return out;
}

function base64ToBytes(b64: Uint8Array): ArrayBuffer {
  const bin = atob(new TextDecoder().decode(b64).trim());
  const out = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
  return out.buffer;
}

/**
 * Load the diorama GLB. A URL ending in `.txt` is read as base64 text
 * (for hosts that do not serve model/gltf-binary). Geometry may be meshopt-compressed
 * (EXT_meshopt_compression, the last step of the export pipeline in scripts/blender/export_glb.py).
 */
export async function loadModel(url: string, totalHint: number, onProgress: Progress): Promise<THREE.Group> {
  const res = await fetch(url);
  if (!res.ok) throw new Error(`${url}: HTTP ${res.status}`);
  const body = await readBody(res, totalHint, onProgress);
  // readBody always returns a view over its own, exactly sized ArrayBuffer
  const buffer = url.endsWith('.txt') ? base64ToBytes(body) : (body.buffer as ArrayBuffer);
  const gltf = await new GLTFLoader().setMeshoptDecoder(MeshoptDecoder).parseAsync(buffer, '');
  return gltf.scene;
}
