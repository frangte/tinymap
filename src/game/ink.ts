// Screen-space ink outline: the scene is drawn once as flat normals + depth and once in colour (MSAA),
// then a full-screen shader draws wobbly ink where depth or normals break, plus paper grain and vignette.
// The depth pass runs first so see-through materials drawn in colour (the sea) can read the depth behind them.
import * as THREE from 'three';
import { FOLIAGE, FOLIAGE_FRAG_GLSL, FOLIAGE_PROJECT, FOLIAGE_VERT_GLSL } from './paint';

const VERT = 'varying vec2 vUv; void main(){ vUv = uv; gl_Position = vec4(position.xy, 0.0, 1.0); }';

const FRAG = /* glsl */ `
  uniform sampler2D tColor, tNormal, tDepth; uniform vec2 uRes; uniform float uNear, uFar, uInk, uPx; varying vec2 vUv;
  float lin(float z){ float n = z*2.0 - 1.0; return (2.0*uNear*uFar)/(uFar + uNear - n*(uFar - uNear)); }
  float h2(vec2 p){ return fract(sin(dot(p, vec2(127.1, 311.7)))*43758.5453); }
  float n2(vec2 p){ vec2 i = floor(p), f = fract(p); f = f*f*(3.0-2.0*f);
    return mix(mix(h2(i), h2(i+vec2(1,0)), f.x), mix(h2(i+vec2(0,1)), h2(i+vec2(1,1)), f.x), f.y); }
  vec3 nrm(vec2 uv){ return texture2D(tNormal, uv).xyz*2.0 - 1.0; }
  void main(){
    vec3 col = texture2D(tColor, vUv).rgb;
    if (uInk > 0.0) {
      vec2 px = uPx/uRes;
      vec2 q = gl_FragCoord.xy/64.0;
      vec2 uv = vUv + (vec2(n2(q), n2(q + 31.7)) - 0.5)*px*1.8;
      float d0 = lin(texture2D(tDepth, uv).x);
      float d1 = lin(texture2D(tDepth, uv + vec2(px.x, 0.0)).x), d2 = lin(texture2D(tDepth, uv - vec2(px.x, 0.0)).x);
      float d3 = lin(texture2D(tDepth, uv + vec2(0.0, px.y)).x), d4 = lin(texture2D(tDepth, uv - vec2(0.0, px.y)).x);
      float lap = abs(d1 + d2 + d3 + d4 - 4.0*d0)/max(d0, 1e-3);
      float ed = smoothstep(0.010, 0.028, lap);
      vec3 n0 = nrm(uv);
      float nd = max(max(length(n0 - nrm(uv + vec2(px.x, 0.0))), length(n0 - nrm(uv - vec2(px.x, 0.0)))),
                     max(length(n0 - nrm(uv + vec2(0.0, px.y))), length(n0 - nrm(uv - vec2(0.0, px.y)))));
      float en = smoothstep(0.32, 0.6, nd);
      float e = max(ed, en)*uInk;
      e *= mix(1.0, 0.5, smoothstep(160.0, 520.0, d0));
      e *= 0.82 + 0.18*n2(gl_FragCoord.xy/9.0);
      col = mix(col, vec3(0.024, 0.015, 0.011), e*0.9);
    }
    col *= 0.986 + 0.028*h2(floor(gl_FragCoord.xy/1.5));
    vec2 v = vUv - 0.5; col *= 1.0 - dot(v, v)*0.22;
    gl_FragColor = vec4(col, 1.0);
    #include <colorspace_fragment>
  }`;

export class InkPass {
  private readonly colorRT = new THREE.WebGLRenderTarget(1, 1, { type: THREE.HalfFloatType, samples: 4 });
  private readonly normalRT = new THREE.WebGLRenderTarget(1, 1, { type: THREE.UnsignedByteType });
  private readonly normalMat = new THREE.MeshNormalMaterial({ flatShading: true, side: THREE.DoubleSide });
  /** foliage sways and dissolves in the normal pass exactly as in its colour pass */
  private readonly normalFoliageMat = foliageNormalMaterial();
  /** meshes drawn in the normal pass, with the material they use there */
  private readonly tracked: Array<{ mesh: THREE.Mesh; normal: THREE.Material; color: THREE.Material | THREE.Material[] }> = [];
  /** objects left out of the normal/depth pass (the sea: it reads that depth instead) */
  private readonly skipped: THREE.Object3D[] = [];
  private readonly post: THREE.ShaderMaterial;
  private readonly quadScene = new THREE.Scene();
  private readonly quadCam = new THREE.OrthographicCamera(-1, 1, 1, -1, 0, 1);
  /** render size in device pixels, shared with materials that sample the depth texture */
  readonly resolution = new THREE.Vector2(1, 1);
  private inkOn = true;

  constructor(camera: THREE.PerspectiveCamera) {
    this.normalRT.depthTexture = new THREE.DepthTexture(1, 1);
    this.post = new THREE.ShaderMaterial({
      uniforms: {
        tColor: { value: this.colorRT.texture },
        tNormal: { value: this.normalRT.texture },
        tDepth: { value: this.normalRT.depthTexture },
        uRes: { value: this.resolution },
        uNear: { value: camera.near },
        uFar: { value: camera.far },
        uInk: { value: 1.0 },
        uPx: { value: 1.0 },
      },
      vertexShader: VERT,
      fragmentShader: FRAG,
      depthTest: false,
      depthWrite: false,
    });
    this.quadScene.add(new THREE.Mesh(new THREE.PlaneGeometry(2, 2), this.post));
  }

  get enabled(): boolean {
    return this.inkOn;
  }

  set enabled(on: boolean) {
    this.inkOn = on;
    this.post.uniforms.uInk.value = on ? 1 : 0;
  }

  /** Scene depth of the current frame (valid during the colour pass). */
  get depthTexture(): THREE.DepthTexture {
    return this.normalRT.depthTexture;
  }

  /** Draw every mesh under `root` in the normal pass (foliage where `userData.foliage`). */
  track(root: THREE.Object3D): void {
    root.traverse((o) => {
      if (o instanceof THREE.Mesh) {
        this.tracked.push({ mesh: o, normal: o.userData.foliage === true ? this.normalFoliageMat : this.normalMat, color: o.material });
      }
    });
  }

  /** Leave `o` out of the normal/depth pass. */
  skip(o: THREE.Object3D): void {
    this.skipped.push(o);
  }

  /** Size in device pixels. */
  setSize(width: number, height: number, pixelRatio: number): void {
    this.colorRT.setSize(width, height);
    this.normalRT.setSize(width, height);
    this.resolution.set(width, height);
    this.post.uniforms.uPx.value = pixelRatio > 1.4 ? 1.35 : 1.0;
  }

  render(renderer: THREE.WebGLRenderer, scene: THREE.Scene, camera: THREE.PerspectiveCamera): void {
    this.post.uniforms.uNear.value = camera.near;
    this.post.uniforms.uFar.value = camera.far;
    // 1. flat normals + depth: always drawn, the sea needs the depth even with the ink turned off
    const bg = scene.background;
    scene.background = null;
    for (const t of this.tracked) {
      t.color = t.mesh.material;
      t.mesh.material = t.normal;
    }
    for (const o of this.skipped) o.visible = false;
    renderer.setClearColor(0x8080ff, 1);
    renderer.setRenderTarget(this.normalRT);
    renderer.clear();
    renderer.render(scene, camera);
    for (const o of this.skipped) o.visible = true;
    for (const t of this.tracked) t.mesh.material = t.color;
    scene.background = bg;
    // 2. colour
    renderer.setRenderTarget(this.colorRT);
    renderer.render(scene, camera);
    // 3. ink + paper
    renderer.setRenderTarget(null);
    renderer.render(this.quadScene, this.quadCam);
  }
}

function foliageNormalMaterial(): THREE.MeshNormalMaterial {
  const m = new THREE.MeshNormalMaterial({ flatShading: true, side: THREE.DoubleSide });
  m.onBeforeCompile = (sh) => {
    Object.assign(sh.uniforms, FOLIAGE);
    sh.vertexShader = sh.vertexShader
      .replace('#include <common>', `#include <common>\nvarying vec3 vWPos;\n${FOLIAGE_VERT_GLSL}`)
      .replace('#include <project_vertex>', `${FOLIAGE_PROJECT}\nvWPos = sdWorld.xyz;`);
    sh.fragmentShader = sh.fragmentShader
      .replace('#include <clipping_planes_pars_fragment>', `#include <clipping_planes_pars_fragment>\nvarying vec3 vWPos;\n${FOLIAGE_FRAG_GLSL}`)
      .replace('#include <clipping_planes_fragment>', '#include <clipping_planes_fragment>\nseeThrough(vWPos);');
  };
  m.customProgramCacheKey = () => 'ink-normal-foliage';
  return m;
}
