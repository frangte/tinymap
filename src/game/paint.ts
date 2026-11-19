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
  wallSky: [0.1, 0.12], glassDark: [0.05, 0.12], steelWhite: [0.04, 0.12], etfe: [0.04, 0.1], cable: [0.02, 0.05],
};
const NO_GRIME = new Set(['water', 'waterDeep', 'foam', 'lampGlow', 'grass', 'grassDark', 'leaf', 'leafLight', 'leafDark', 'roadLine', 'asphalt']);
const UNLIT = new Set(['lampGlow']);
/** foliage: sways in the wind, and dissolves near the camera and in front of the player */
const FOLIAGE_KEYS = new Set(['leaf', 'leafLight', 'leafDark']);

/**
 * Shared by every foliage material (colour and ink normal pass): the point the camera looks at (the player's head),
 * whether the see-through is on (play mode) and the clock for the wind.
 */
export const FOLIAGE = { uPivot: { value: new THREE.Vector3() }, uSee: { value: 1 }, uTime: { value: 0 } };

/** how many street lamps light the ground at once (the nearest to what the camera looks at) */
export const MAX_LAMPS = 16;

/**
 * Shared by every paint material, set by the time of day (daylight/daylight.ts): how dark it is (0 by day, 1 at
 * night), the share of windows with a light on, and the pools of light under the nearest street lamps.
 */
export const NIGHT = {
  uNight: { value: 0 },
  uLit: { value: 0 },
  uLamps: { value: Array.from({ length: MAX_LAMPS }, () => new THREE.Vector3()) },
  uLampCount: { value: 0 },
};

/** Shared by every paint material, set by the weather (daylight/weather.ts): how wet the ground is (0–1). */
export const WET = { uWet: { value: 0 } };

/**
 * What lights up after dark, by material key: its glow (sRGB) and how. `always` glows whole (lanterns, traffic
 * lights, the clock face); `windows` lights some panes and not others, more in the evening than at 3 a.m.; `shops`
 * (signs, shop interiors) the same, but most are on until late.
 */
const NIGHT_GLOW: Record<string, { color: string; strength: number; kind: 'always' | 'windows' | 'shops' }> = {
  lantern: { color: '#FF5A3C', strength: 0.95, kind: 'always' },
  sigRed: { color: '#FF5034', strength: 1.1, kind: 'always' },
  sigAmber: { color: '#FFB238', strength: 0.5, kind: 'always' },
  sigGreen: { color: '#5CFF9A', strength: 1.0, kind: 'always' },
  clockWhite: { color: '#FFF1C8', strength: 0.8, kind: 'always' },
  glassTeal: { color: '#FFD08A', strength: 0.6, kind: 'windows' },
  glassBlue: { color: '#FFD9A0', strength: 0.5, kind: 'windows' },
  glassDark: { color: '#FFD08A', strength: 0.6, kind: 'windows' },
  interior: { color: '#FFB866', strength: 0.75, kind: 'shops' },
  signRed: { color: '#FF6A4A', strength: 0.5, kind: 'shops' },
  signGold: { color: '#FFD27A', strength: 0.5, kind: 'shops' },
};

/** GLSL (fragment): the light of the street lamps near `wp`, the night glow mask of a material, wet ground. */
const NIGHT_GLSL = /* glsl */ `
uniform float uNight;
uniform float uLit;
uniform vec3 uLamps[${MAX_LAMPS}];
uniform int uLampCount;
uniform float uWet;
varying vec3 vSdW;
/** how wet this fragment looks: the weather's wetness on surfaces facing up (streets, squares, roofs) */
float sdWetness = 0.0;
vec3 sdLampLight(vec3 wp){
  float k = 0.0;
  for (int i = 0; i < ${MAX_LAMPS}; i++) {
    if (i >= uLampCount) break;
    vec3 d = wp - uLamps[i];
    float r2 = dot(d, d);
    // two stepped bands, like the toon ramp: a bright pool and a softer ring round it
    k += (1.0 - smoothstep(7.0, 9.0, r2))*0.65 + (1.0 - smoothstep(22.0, 30.0, r2))*0.35;
  }
  return vec3(1.0, 0.74, 0.42)*min(k, 1.0)*0.7;
}
float sdLitShare(float bias){
  return clamp(uLit*bias, 0.0, 1.0);
}
`;

/** GLSL (vertex): `sdWind(worldPos)` offsets crowns and fronds in a slow gust; trunks below 1.2 m stay put. */
export const FOLIAGE_VERT_GLSL = /* glsl */ `
uniform float uTime;
vec3 sdWind(vec3 wp){
  float h = clamp((wp.y - 1.2)/6.0, 0.0, 1.0);
  float ph = dot(wp.xz, vec2(0.21, 0.17));
  float gust = 0.65 + 0.35*sin(uTime*0.23 + wp.x*0.013);
  float s = (sin(uTime*1.25 + ph)*0.7 + sin(uTime*2.1 + ph*1.9)*0.3)*gust;
  return vec3(s*0.12, 0.0, s*0.08)*h;
}
`;

/**
 * Replaces three's project_vertex for foliage: the wind is added in world space (the GLB's quantized positions
 * live in a scaled local frame), then projected. Leaves the world position in `sdWorld`.
 */
export const FOLIAGE_PROJECT = `
vec4 sdWorld = modelMatrix * vec4(transformed, 1.0);
sdWorld.xyz += sdWind(sdWorld.xyz);
vec4 mvPosition = viewMatrix * sdWorld;
gl_Position = projectionMatrix * mvPosition;`;

/**
 * GLSL (fragment): `seeThrough(worldPos)` discards a dither pattern of fragments close to the camera or near the sight
 * line from the camera to the player, so tree crowns never hide the player or fill the screen.
 */
export const FOLIAGE_FRAG_GLSL = /* glsl */ `
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
  /** sway in the wind and dissolve near the camera / in front of the player (foliage) */
  foliage?: boolean;
}

export function paintMaterial(color: THREE.ColorRepresentation, key: string, opts: PaintOptions = {}): THREE.MeshToonMaterial {
  const [baseVar, baseGrime] = PAINT[key] ?? [0.09, 0.12];
  const variation = opts.variation ?? baseVar;
  const grime = opts.grime ?? (NO_GRIME.has(key) ? 0 : baseGrime);
  const local = opts.local === true;
  const see = opts.foliage ?? FOLIAGE_KEYS.has(key);
  const glow = NIGHT_GLOW[key];
  const m = new THREE.MeshToonMaterial({ color: new THREE.Color(color), gradientMap: toonRamp(), side: THREE.DoubleSide });
  m.name = key;
  m.defines = { FLAT_SHADED: '' }; // faceted normals from screen derivatives: the GLB ships positions only
  const pos = local ? 'transformed' : '(modelMatrix * vec4(transformed, 1.0)).xyz';
  const world = see && !local ? 'sdWorld.xyz' : '(modelMatrix * vec4(transformed, 1.0)).xyz';
  // lit panes and shop fronts: one cell of a few metres is on or off, picked by its place (the same every night)
  const glowMask = !glow
    ? ''
    : glow.kind === 'always'
      ? '1.0'
      : glow.kind === 'windows'
        ? 'step(1.0 - sdLitShare(1.0), sdHash(floor(vSdW/vec3(2.4, 3.2, 2.4)) + 3.7))'
        : 'step(1.0 - sdLitShare(1.35), sdHash(floor(vSdW/vec3(4.0, 3.0, 4.0)) + 9.1))';
  m.onBeforeCompile = (sh) => {
    sh.uniforms.uVar = { value: variation };
    sh.uniforms.uGrime = { value: grime };
    if (see) Object.assign(sh.uniforms, FOLIAGE);
    Object.assign(sh.uniforms, NIGHT, WET);
    if (glow) sh.uniforms.uGlow = { value: new THREE.Color(glow.color).multiplyScalar(glow.strength) };
    sh.vertexShader = sh.vertexShader
      .replace('#include <common>', `#include <common>\nvarying vec3 vWPos;\nvarying vec3 vSdW;\n${see ? FOLIAGE_VERT_GLSL : ''}`)
      .replace(
        '#include <project_vertex>',
        `${see && !local ? `${FOLIAGE_PROJECT}\nvWPos = sdWorld.xyz;` : `#include <project_vertex>\nvWPos = ${pos};`}\nvSdW = ${world};`,
      );
    sh.fragmentShader = sh.fragmentShader
      .replace(
        '#include <common>',
        `#include <common>\nvarying vec3 vWPos;\nuniform float uVar;\nuniform float uGrime;\n${glow ? 'uniform vec3 uGlow;\n' : ''}float gTermN = 0.0;\n${NOISE}${NIGHT_GLSL}${see ? FOLIAGE_FRAG_GLSL : ''}`,
      )
      .replace('#include <clipping_planes_fragment>', `#include <clipping_planes_fragment>\n${see ? 'seeThrough(vWPos);' : ''}`)
      .replace(
        '#include <normal_fragment_maps>',
        `#include <normal_fragment_maps>
        // wet ground is darker, and shines back the lamps at night (below)
        if (uWet > 0.01) {
          vec3 sdUp = normalize((viewMatrix*vec4(0.0, 1.0, 0.0, 0.0)).xyz);
          sdWetness = uWet*smoothstep(0.55, 0.85, dot(normal, sdUp));
          diffuseColor.rgb *= 1.0 - 0.32*sdWetness;
        }`,
      )
      .replace(
        '#include <emissivemap_fragment>',
        `#include <emissivemap_fragment>\n${glow ? `totalEmissiveRadiance += uGlow*(${glowMask})*uNight;` : ''}`,
      )
      .replace(
        '#include <lights_fragment_end>',
        '#include <lights_fragment_end>\nif (uNight > 0.01) reflectedLight.directDiffuse += diffuseColor.rgb*sdLampLight(vSdW)*uNight*(1.0 + 0.9*sdWetness);',
      )
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
  m.customProgramCacheKey = () => `sd-paint-${local ? 'l' : 'w'}${see ? 's' : ''}${glow ? `-g${glow.kind}` : ''}`;
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
    o.userData.foliage = FOLIAGE_KEYS.has(key);
    o.userData.paintKey = key;
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
