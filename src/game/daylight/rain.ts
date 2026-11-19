// Falling rain: thin streaks in a box round the camera. Each drop has a fixed place in the world, wrapped into the
// box, so the rain does not slide when the camera moves; the shader moves them down and slants them with the wind.
// Left out of the ink's normal pass (no outlines round raindrops). How many fall follows the rain's strength.
import * as THREE from 'three';
import type { InkPass } from '../ink';

/** drops at full strength (a downpour) */
const MAX_DROPS = 6000;
/** the box round the camera, metres */
const BOX = new THREE.Vector3(56, 34, 56);
/** metres a second */
const FALL_SPEED = 14;
/** a streak's length (m) */
const STREAK = 0.85;
/** the slant: metres sideways per metre down */
const WIND = new THREE.Vector2(0.18, 0.08);
/** seconds */
const TIME_WRAP = 3600;

const VERT = /* glsl */ `
uniform float uTime, uFall, uStreak;
uniform vec3 uCam, uBox;
uniform vec2 uWind;
attribute float aEnd;
varying float vFade;
void main(){
  vec3 p = position;
  // falling, then wrapped into the box round the camera (the drop keeps its place in the world)
  vec3 w;
  w.y = uCam.y + mod(p.y - uTime*uFall - uCam.y, uBox.y) - uBox.y*0.5;
  vec2 drift = uWind*uTime*uFall;
  w.xz = uCam.xz + mod(p.xz + drift - uCam.xz, uBox.xz) - uBox.xz*0.5;
  // the streak trails up and back against the wind, where the drop came from
  w += vec3(-uWind.x, 1.0, -uWind.y)*uStreak*aEnd;
  // thin out toward the box's sides, so its edges never show
  vec3 d = abs(w - uCam)/(uBox*0.5);
  vFade = 1.0 - smoothstep(0.6, 1.0, max(max(d.x, d.z), d.y));
  gl_Position = projectionMatrix*viewMatrix*vec4(w, 1.0);
}`;

const FRAG = /* glsl */ `
uniform vec3 uColor;
uniform float uAlpha;
varying float vFade;
void main(){
  gl_FragColor = vec4(uColor, uAlpha*vFade);
  #include <colorspace_fragment>
}`;

export class Rain {
  readonly lines: THREE.LineSegments;
  private readonly mat: THREE.ShaderMaterial;

  constructor(ink: InkPass) {
    const pos = new Float32Array(MAX_DROPS * 6);
    const end = new Float32Array(MAX_DROPS * 2);
    for (let i = 0; i < MAX_DROPS; i++) {
      const x = Math.random() * BOX.x * 4;
      const y = Math.random() * BOX.y;
      const z = Math.random() * BOX.z * 4;
      pos.set([x, y, z, x, y, z], i * 6);
      end[i * 2 + 1] = 1;
    }
    const geo = new THREE.BufferGeometry();
    geo.setAttribute('position', new THREE.BufferAttribute(pos, 3));
    geo.setAttribute('aEnd', new THREE.BufferAttribute(end, 1));
    this.mat = new THREE.ShaderMaterial({
      uniforms: {
        uTime: { value: 0 },
        uFall: { value: FALL_SPEED },
        uStreak: { value: STREAK },
        uCam: { value: new THREE.Vector3() },
        uBox: { value: BOX },
        uWind: { value: WIND },
        uColor: { value: new THREE.Color('#DDE4EA') },
        uAlpha: { value: 0.5 },
      },
      vertexShader: VERT,
      fragmentShader: FRAG,
      transparent: true,
      depthWrite: false,
    });
    this.lines = new THREE.LineSegments(geo, this.mat);
    this.lines.name = 'Rain';
    this.lines.frustumCulled = false;
    this.lines.renderOrder = 3;
    this.lines.visible = false;
    ink.skip(this.lines);
  }

  /** `amount` is the rain's strength (0–1); `colour` the streaks' tint (paler than the sky). */
  update(time: number, camera: THREE.Camera, amount: number, colour: THREE.Color): void {
    const drops = Math.round(MAX_DROPS * amount);
    this.lines.visible = drops > 0;
    if (!this.lines.visible) return;
    this.lines.geometry.setDrawRange(0, drops * 2);
    const u = this.mat.uniforms;
    // wrapped every hour so the shader's floats keep their precision (a jump of the drops nobody sees in the rain)
    u.uTime.value = time % TIME_WRAP;
    u.uCam.value.copy(camera.position);
    u.uColor.value.copy(colour);
    u.uAlpha.value = 0.3 + 0.3 * amount;
  }
}
