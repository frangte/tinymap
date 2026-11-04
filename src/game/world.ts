// Renderer, paper sky, two-tone sun/ambient lighting and the diorama model loader.
import * as THREE from 'three';
import { GLTFLoader } from 'three/examples/jsm/loaders/GLTFLoader.js';
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

/** Scene with the paper sky and the Blender LIGHT_TINT / SHADOW_TINT two-tone lighting. */
export function createScene(): THREE.Scene {
  const scene = new THREE.Scene();
  scene.background = paperSky();
  const LIGHT_TINT = new THREE.Color('#FFF2DC');
  const SHADOW_TINT = new THREE.Color('#717BA2');
  scene.add(new THREE.AmbientLight(SHADOW_TINT, Math.PI));
  const sun = new THREE.DirectionalLight(LIGHT_TINT.clone().sub(SHADOW_TINT), Math.PI);
  const sunDir = new THREE.Vector3(-0.5506, 0.7708, -0.3204).normalize();
  const center = fromBlender(45, -45, 0);
  sun.position.copy(center).addScaledVector(sunDir, 260);
  sun.target.position.copy(center);
  sun.castShadow = true;
  const res = Math.min(window.innerWidth, window.innerHeight) < 700 ? 2048 : 4096;
  sun.shadow.mapSize.set(res, res);
  Object.assign(sun.shadow.camera, { left: -150, right: 150, top: 150, bottom: -150, near: 20, far: 560 });
  sun.shadow.bias = -0.0004;
  sun.shadow.normalBias = 0.06;
  scene.add(sun, sun.target);
  return scene;
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
 * (for hosts that do not serve model/gltf-binary).
 */
export async function loadModel(url: string, totalHint: number, onProgress: Progress): Promise<THREE.Group> {
  const res = await fetch(url);
  if (!res.ok) throw new Error(`${url}: HTTP ${res.status}`);
  const body = await readBody(res, totalHint, onProgress);
  // readBody always returns a view over its own, exactly sized ArrayBuffer
  const buffer = url.endsWith('.txt') ? base64ToBytes(body) : (body.buffer as ArrayBuffer);
  const gltf = await new GLTFLoader().parseAsync(buffer, '');
  return gltf.scene;
}
