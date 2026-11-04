// The sea round the island: one big disc at sea level with a toon water shader. It reads the depth of the
// normal pass (drawn first, without the sea) to know how much water lies over the sand: shallow turquoise near
// the beach, a foam line where water meets sand, foam bands rolling in, slow wave streaks, haze to the horizon.
import * as THREE from 'three';
import type { InkPass } from './ink';

const VERT = /* glsl */ `
varying vec3 vW;
void main(){
  vec4 w = modelMatrix * vec4(position, 1.0);
  vW = w.xyz;
  gl_Position = projectionMatrix * viewMatrix * w;
}`;

const FRAG = /* glsl */ `
uniform sampler2D tDepth; uniform vec2 uRes; uniform float uNear, uFar, uTime;
uniform vec3 uDeep, uShallow, uFoam, uHaze;
varying vec3 vW;
float lin(float z){ float n = z*2.0 - 1.0; return (2.0*uNear*uFar)/(uFar + uNear - n*(uFar - uNear)); }
float h2(vec2 p){ return fract(sin(dot(p, vec2(127.1, 311.7)))*43758.5453); }
float n2(vec2 p){ vec2 i = floor(p), f = fract(p); f = f*f*(3.0-2.0*f);
  return mix(mix(h2(i), h2(i+vec2(1,0)), f.x), mix(h2(i+vec2(0,1)), h2(i+vec2(1,1)), f.x), f.y); }
void main(){
  vec3 view = vW - cameraPosition;
  float dist = length(view);
  // metres of water along the view ray, turned into depth below the surface
  float behind = lin(texture2D(tDepth, gl_FragCoord.xy/uRes).x);
  float ahead = lin(gl_FragCoord.z);
  float depth = max(0.0, behind - ahead)*abs(view.y/dist);
  vec2 p = vW.xz;
  float wob = n2(p*0.35 + uTime*0.15) - 0.5;
  float shallow = exp(-(depth + wob*0.15)*1.1);
  vec3 col = mix(uDeep, uShallow, shallow);
  // wave streaks drifting across the open water
  float w1 = n2(p*vec2(0.05, 0.11) + vec2(uTime*0.035, uTime*0.02));
  float w2 = n2(p*vec2(0.13, 0.07) - vec2(uTime*0.05, -uTime*0.03));
  float crest = smoothstep(0.64, 0.68, w1*0.55 + w2*0.45);
  col = mix(col, mix(uShallow, uFoam, 0.35), crest*0.45*(1.0 - smoothstep(60.0, 260.0, dist)));
  // foam: a line where water meets sand, and bands rolling in over the shallows
  float line = 1.0 - smoothstep(0.03, 0.12 + wob*0.05, depth);
  float roll = fract(depth*2.2 - uTime*0.22 + wob*0.6);
  float bands = smoothstep(0.86, 0.9, roll)*(1.0 - smoothstep(0.15, 0.85, depth));
  float foam = max(line, bands*0.8);
  col = mix(col, uFoam, foam);
  // paper haze toward the horizon
  col = mix(col, uHaze, smoothstep(120.0, 700.0, dist));
  float alpha = mix(0.58, 0.97, 1.0 - shallow);
  gl_FragColor = vec4(col, max(alpha, foam));
  #include <colorspace_fragment>
}`;

export class Sea {
  readonly mesh: THREE.Mesh;
  private readonly mat: THREE.ShaderMaterial;

  constructor(ink: InkPass, camera: THREE.PerspectiveCamera, level: number, center: THREE.Vector3) {
    this.mat = new THREE.ShaderMaterial({
      uniforms: {
        tDepth: { value: ink.depthTexture },
        uRes: { value: ink.resolution },
        uNear: { value: camera.near },
        uFar: { value: camera.far },
        uTime: { value: 0 },
        uDeep: { value: new THREE.Color('#3F8399') },
        uShallow: { value: new THREE.Color('#86CBC6') },
        uFoam: { value: new THREE.Color('#F3F1E7') },
        uHaze: { value: new THREE.Color('#DCDCD0') },
      },
      vertexShader: VERT,
      fragmentShader: FRAG,
      transparent: true,
      depthWrite: false,
    });
    const geo = new THREE.CircleGeometry(1400, 96);
    geo.rotateX(-Math.PI / 2);
    this.mesh = new THREE.Mesh(geo, this.mat);
    this.mesh.name = 'Sea';
    this.mesh.position.set(center.x, level, center.z);
    this.mesh.renderOrder = 2;
    this.mesh.matrixAutoUpdate = false;
    this.mesh.updateMatrix();
    ink.skip(this.mesh);
  }

  update(time: number, camera: THREE.PerspectiveCamera): void {
    this.mat.uniforms.uTime.value = time;
    this.mat.uniforms.uNear.value = camera.near;
    this.mat.uniforms.uFar.value = camera.far;
  }
}
