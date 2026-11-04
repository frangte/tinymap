// Screen-space ink outline: the scene is drawn once in colour (MSAA) and once as flat normals + depth,
// then a full-screen shader draws wobbly ink where depth or normals break, plus paper grain and vignette.
import * as THREE from 'three';
import { SEE_THROUGH, SEE_THROUGH_GLSL } from './paint';

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
  /** the normal pass of see-through foliage dissolves the same way as its colour pass */
  private readonly normalSeeMat = seeThroughNormalMaterial();
  /** meshes drawn in the normal pass, with the material they use there */
  private readonly tracked: Array<{ mesh: THREE.Mesh; normal: THREE.Material; color: THREE.Material | THREE.Material[] }> = [];
  private readonly post: THREE.ShaderMaterial;
  private readonly quadScene = new THREE.Scene();
  private readonly quadCam = new THREE.OrthographicCamera(-1, 1, 1, -1, 0, 1);
  private inkOn = true;

  constructor(camera: THREE.PerspectiveCamera) {
    this.normalRT.depthTexture = new THREE.DepthTexture(1, 1);
    this.post = new THREE.ShaderMaterial({
      uniforms: {
        tColor: { value: this.colorRT.texture },
        tNormal: { value: this.normalRT.texture },
        tDepth: { value: this.normalRT.depthTexture },
        uRes: { value: new THREE.Vector2(1, 1) },
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

  /** Draw every mesh under `root` in the normal pass (with see-through foliage where `userData.seeThrough`). */
  track(root: THREE.Object3D): void {
    root.traverse((o) => {
      if (o instanceof THREE.Mesh) {
        this.tracked.push({ mesh: o, normal: o.userData.seeThrough === true ? this.normalSeeMat : this.normalMat, color: o.material });
      }
    });
  }

  /** Size in device pixels. */
  setSize(width: number, height: number, pixelRatio: number): void {
    this.colorRT.setSize(width, height);
    this.normalRT.setSize(width, height);
    (this.post.uniforms.uRes.value as THREE.Vector2).set(width, height);
    this.post.uniforms.uPx.value = pixelRatio > 1.4 ? 1.35 : 1.0;
  }

  render(renderer: THREE.WebGLRenderer, scene: THREE.Scene, camera: THREE.PerspectiveCamera): void {
    this.post.uniforms.uNear.value = camera.near;
    this.post.uniforms.uFar.value = camera.far;
    renderer.setRenderTarget(this.colorRT);
    renderer.render(scene, camera);
    if (this.inkOn) {
      const bg = scene.background;
      scene.background = null;
      for (const t of this.tracked) {
        t.color = t.mesh.material;
        t.mesh.material = t.normal;
      }
      renderer.setClearColor(0x8080ff, 1);
      renderer.setRenderTarget(this.normalRT);
      renderer.clear();
      renderer.render(scene, camera);
      for (const t of this.tracked) t.mesh.material = t.color;
      scene.background = bg;
    }
    renderer.setRenderTarget(null);
    renderer.render(this.quadScene, this.quadCam);
  }
}

function seeThroughNormalMaterial(): THREE.MeshNormalMaterial {
  const m = new THREE.MeshNormalMaterial({ flatShading: true, side: THREE.DoubleSide });
  m.onBeforeCompile = (sh) => {
    Object.assign(sh.uniforms, SEE_THROUGH);
    sh.vertexShader = sh.vertexShader
      .replace('#include <common>', '#include <common>\nvarying vec3 vWPos;')
      .replace('#include <project_vertex>', '#include <project_vertex>\nvWPos = (modelMatrix * vec4(transformed, 1.0)).xyz;');
    sh.fragmentShader = sh.fragmentShader
      .replace('#include <clipping_planes_pars_fragment>', `#include <clipping_planes_pars_fragment>\nvarying vec3 vWPos;\n${SEE_THROUGH_GLSL}`)
      .replace('#include <clipping_planes_fragment>', '#include <clipping_planes_fragment>\nseeThrough(vWPos);');
  };
  m.customProgramCacheKey = () => 'ink-normal-see';
  return m;
}
