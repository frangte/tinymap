// Toon paint shader: the same parameters as the Blender SD_Paint node group, so the hand-painted look
// (colour variation, street-level grime, rain streaks, paper grain, wobbly terminator) holds in real time.
import * as THREE from 'three';

/** variation, grime per material key, taken from the Blender SD_Paint node group */
const PAINT: Record<string, [number, number]> = {
  cream: [0.09, 0.3], creamLight: [0.09, 0.25], wallTeal: [0.09, 0.25], wallGrey: [0.09, 0.3], concrete: [0.09, 0.3],
  wallBeige: [0.09, 0.3], wallOchre: [0.09, 0.3], wallPink: [0.09, 0.3], stone: [0.09, 0.2], brick: [0.09, 0.2],
  opWhite: [0.09, 0.2], poYellow: [0.09, 0.25], concreteLight: [0.08, 0.25], towerPale: [0.06, 0.22], turq: [0.07, 0.15],
  bridgeWhite: [0.04, 0.15], glassBlue: [0.05, 0.12], white: [0.04, 0.12], roadLine: [0.03, 0.12], statueW: [0.03, 0.12],
  clockWhite: [0.02, 0.12], ink: [0, 0.12], paving: [0.09, 0.15], wallCream: [0.1, 0.12], wallMint: [0.1, 0.12],
  wallSky: [0.1, 0.12], glassDark: [0.05, 0.12],
};
const NO_GRIME = new Set(['water', 'waterDeep', 'foam', 'lampGlow', 'grass', 'grassDark', 'leaf', 'leafLight', 'leafDark', 'roadLine', 'asphalt']);
const UNLIT = new Set(['lampGlow']);
/** foliage that dissolves near the camera and in front of the player */
const SEE_THROUGH_KEYS = new Set(['leaf', 'leafLight', 'leafDark']);

/** Shared by every see-through material: the point the camera looks at (the player's head). */
export const SEE_THROUGH = { uPivot: { value: new THREE.Vector3() }, uSee: { value: 1 } };

/**
 * GLSL: `seeThrough(worldPos)` discards a dither pattern of fragments close to the camera or near the sight line
 * from the camera to the player, so tree crowns never hide the player or fill the screen.
 */
export const SEE_THROUGH_GLSL = /* glsl */ `
uniform vec3 uPivot;
uniform float uSee;
float sdBayer(vec2 p){
  ivec2 q = ivec2(mod(p, 4.0));
  int i = q.x + q.y*4;
  float m[16] = float[16](0.,8.,2.,10.,12.,4.,14.,6.,3.,11.,1.,9.,15.,7.,13.,5.);
  return (m[i] + 0.5)/16.0;
}
void seeThrough(vec3 wp){
  vec3 ca = uPivot - cameraPosition;
  float L = max(length(ca), 1e-3);
  vec3 d = ca/L;
  vec3 rel = wp - cameraPosition;
  float t = dot(rel, d);
  float r = length(rel - d*t);
  float keep = smoothstep(2.2, 5.0, length(rel));
  float inFront = step(0.0, t)*(1.0 - smoothstep(L - 1.2, L - 0.4, t));
  keep = min(keep, mix(1.0, smoothstep(1.1, 2.8, r), inFront));
  keep = mix(1.0, keep, uSee);
  if (keep < sdBayer(gl_FragCoord.xy)) discard;
}
`;

const NOISE = /* glsl */ `
float sdHash(vec3 p){ p = fract(p*0.3183099 + 0.1); p *= 17.0; return fract(p.x*p.y*p.z*(p.x+p.y+p.z)); }
float sdNoise(vec3 x){ vec3 i = floor(x); vec3 f = fract(x); f = f*f*(3.0-2.0*f);
  return mix(mix(mix(sdHash(i), sdHash(i+vec3(1,0,0)), f.x), mix(sdHash(i+vec3(0,1,0)), sdHash(i+vec3(1,1,0)), f.x), f.y),
             mix(mix(sdHash(i+vec3(0,0,1)), sdHash(i+vec3(1,0,1)), f.x), mix(sdHash(i+vec3(0,1,1)), sdHash(i+vec3(1,1,1)), f.x), f.y), f.z); }
`;

let ramp: THREE.DataTexture | null = null;

/** Two-tone toon ramp: a soft step from shadow to light. */
function toonRamp(): THREE.DataTexture {
  if (ramp) return ramp;
  ramp = new THREE.DataTexture(new Uint8Array([0, 0, 0, 0, 0, 0, 0, 0, 40, 200, 255, 255, 255, 255, 255, 255]), 16, 1, THREE.RedFormat);
  ramp.minFilter = THREE.LinearFilter;
  ramp.magFilter = THREE.LinearFilter;
  ramp.needsUpdate = true;
  return ramp;
}

export interface PaintOptions {
  /** colour noise strength (default from the material key) */
  variation?: number;
  /** darkening near the ground and rain streaks (default from the material key) */
  grime?: number;
  /** sample the noise in object space, so it does not swim over moving meshes such as the player */
  local?: boolean;
  /** dissolve near the camera and in front of the player (foliage) */
  seeThrough?: boolean;
}

export function paintMaterial(color: THREE.ColorRepresentation, key: string, opts: PaintOptions = {}): THREE.MeshToonMaterial {
  const [baseVar, baseGrime] = PAINT[key] ?? [0.09, 0.12];
  const variation = opts.variation ?? baseVar;
  const grime = opts.grime ?? (NO_GRIME.has(key) ? 0 : baseGrime);
  const local = opts.local === true;
  const see = opts.seeThrough ?? SEE_THROUGH_KEYS.has(key);
  const m = new THREE.MeshToonMaterial({ color: new THREE.Color(color), gradientMap: toonRamp(), side: THREE.DoubleSide });
  m.name = key;
  m.defines = { FLAT_SHADED: '' }; // faceted normals from screen derivatives: the GLB ships positions only
  const pos = local ? 'transformed' : '(modelMatrix * vec4(transformed, 1.0)).xyz';
  m.onBeforeCompile = (sh) => {
    sh.uniforms.uVar = { value: variation };
    sh.uniforms.uGrime = { value: grime };
    if (see) Object.assign(sh.uniforms, SEE_THROUGH);
    sh.vertexShader = sh.vertexShader
      .replace('#include <common>', '#include <common>\nvarying vec3 vWPos;')
      .replace('#include <project_vertex>', `#include <project_vertex>\nvWPos = ${pos};`);
    sh.fragmentShader = sh.fragmentShader
      .replace('#include <common>', `#include <common>\nvarying vec3 vWPos;\nuniform float uVar;\nuniform float uGrime;\nfloat gTermN = 0.0;\n${NOISE}${see ? SEE_THROUGH_GLSL : ''}`)
      .replace('#include <clipping_planes_fragment>', `#include <clipping_planes_fragment>\n${see ? 'seeThrough(vWPos);' : ''}`)
      .replace('#include <gradientmap_pars_fragment>', `
        uniform sampler2D gradientMap;
        vec3 getGradientIrradiance(vec3 normal, vec3 lightDirection) {
          float dotNL = dot(normal, lightDirection) + gTermN;
          return vec3(texture2D(gradientMap, vec2(dotNL*0.5 + 0.5, 0.0)).r);
        }`)
      .replace('#include <color_fragment>', `#include <color_fragment>
        vec3 sp = vWPos;
        float nv = sdNoise(sp*0.42)*0.6 + sdNoise(sp*0.07)*0.4 - 0.5;
        diffuseColor.rgb *= 1.0 + nv*2.0*uVar;
        diffuseColor.rgb *= mix(1.0 - uGrime*0.55, 1.0, smoothstep(0.05, 1.9, sp.y));
        float st = smoothstep(0.52, 0.72, sdNoise(vec3(sp.x*2.6, sp.y*0.22, sp.z*2.6)*2.2));
        diffuseColor.rgb *= 1.0 - st*uGrime*0.55;
        diffuseColor.rgb *= mix(0.955, 1.045, sdNoise(sp*22.0));
        gTermN = (sdNoise(sp*0.9) - 0.5)*0.16;`);
  };
  m.customProgramCacheKey = () => `sd-paint-${local ? 'l' : 'w'}${see ? 's' : ''}`;
  return m;
}

export interface PaintStats {
  triangles: number;
  materials: number;
}

/** Swap the exported Principled materials of the diorama for paint materials (one per key). */
export function paintWorld(root: THREE.Object3D): PaintStats {
  const cache = new Map<string, THREE.Material>();
  let triangles = 0;
  root.traverse((o) => {
    if (!(o instanceof THREE.Mesh)) return;
    const src = o.material as THREE.MeshStandardMaterial;
    const key = (src.name || '').replace(/^SD_/, '').replace(/\.\d+$/, '');
    let mat = cache.get(key);
    if (!mat) {
      mat = UNLIT.has(key) ? new THREE.MeshBasicMaterial({ color: src.color.clone() }) : paintMaterial(src.color, key);
      cache.set(key, mat);
    }
    o.material = mat;
    o.userData.seeThrough = SEE_THROUGH_KEYS.has(key);
    const water = key.startsWith('water') || key === 'foam';
    o.castShadow = !water;
    o.receiveShadow = true;
    o.matrixAutoUpdate = false;
    o.updateMatrix();
    const geo = o.geometry as THREE.BufferGeometry;
    triangles += (geo.index ? geo.index.count : geo.attributes.position.count) / 3;
  });
  return { triangles, materials: cache.size };
}
