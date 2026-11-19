// The still water of the map: Hồ Con Rùa, the fountain in front of Dinh Độc Lập, the lake. The map paints it flat
// in Blender (material SD_water); here it gets a toon pond shader instead. Like the sea (sea.ts) it reads the depth of
// the normal pass, drawn without the water, to know how much water lies over the floor: the tiles show through where
// it is shallow, a pale line rims the water where it meets a wall or a column. Small ripples drift over it, the sky
// shows on it at a low angle and the sun glints on the ripple tops.
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
uniform vec3 uDeep, uShallow, uRim, uSky;
varying vec3 vW;
float lin(float z){ float n = z*2.0 - 1.0; return (2.0*uNear*uFar)/(uFar + uNear - n*(uFar - uNear)); }
float h2(vec2 p){ return fract(sin(dot(p, vec2(127.1, 311.7)))*43758.5453); }
float n2(vec2 p){ vec2 i = floor(p), f = fract(p); f = f*f*(3.0-2.0*f);
  return mix(mix(h2(i), h2(i+vec2(1,0)), f.x), mix(h2(i+vec2(0,1)), h2(i+vec2(1,1)), f.x), f.y); }
void main(){
  vec3 view = vW - cameraPosition;
  float dist = length(view);
  float down = abs(view.y)/dist;
  // depth of water under this point, from the floor drawn in the normal pass
  float behind = lin(texture2D(tDepth, gl_FragCoord.xy/uRes).x);
  float ahead = lin(gl_FragCoord.z);
  float depth = max(0.0, behind - ahead)*down;
  vec2 p = vW.xz;
  // two layers of small ripples drifting different ways
  float r1 = n2(p*1.7 + vec2(uTime*0.35, uTime*0.22));
  float r2 = n2(p*2.9 - vec2(uTime*0.27, -uTime*0.31));
  float rip = r1*0.6 + r2*0.4;
  float shallow = exp(-depth*5.0);
  vec3 col = mix(uDeep, uShallow, shallow*0.85);
  // ripple bands, in two flat tones (toon)
  col = mix(col, uShallow, smoothstep(0.62, 0.66, rip)*0.45);
  col = mix(col, uDeep, (1.0 - smoothstep(0.30, 0.34, rip))*0.25);
  // the sky on the water at a low angle
  float fres = pow(1.0 - down, 3.0);
  col = mix(col, uSky, fres*0.55);
  // glints on the ripple tops
  float g = n2(p*6.0 + vec2(uTime*0.9, -uTime*0.6))*rip;
  // (pale as the rim: white by day, moonlit at night)
  col = mix(col, min(uRim*1.1, vec3(1.0)), smoothstep(0.52, 0.58, g)*0.8);
  // a pale line where the water meets the rim, the columns, the jets
  float line = 1.0 - smoothstep(0.012, 0.05 + (r1 - 0.5)*0.02, depth);
  col = mix(col, uRim, line*0.85);
  float alpha = mix(0.62, 0.95, 1.0 - shallow);
  gl_FragColor = vec4(col, max(alpha, line*0.9));
  #include <colorspace_fragment>
}`;

/** The pond water: every mesh of the map painted `water` (paintWorld marks them in `userData.paintKey`). */
export class Ponds {
  private readonly mat: THREE.ShaderMaterial;
  readonly meshes: THREE.Mesh[] = [];
  /** the shader's colours, tinted by the time of day (daylight/daylight.ts): edge is the rim, far the sky on it */
  readonly colours: { deep: THREE.Color; shallow: THREE.Color; edge: THREE.Color; far: THREE.Color };

  constructor(world: THREE.Object3D, ink: InkPass, camera: THREE.PerspectiveCamera) {
    this.mat = new THREE.ShaderMaterial({
      uniforms: {
        tDepth: { value: ink.depthTexture },
        uRes: { value: ink.resolution },
        uNear: { value: camera.near },
        uFar: { value: camera.far },
        uTime: { value: 0 },
        uDeep: { value: new THREE.Color('#3E8E9C') },
        uShallow: { value: new THREE.Color('#8FD0C8') },
        uRim: { value: new THREE.Color('#EEF4EC') },
        uSky: { value: new THREE.Color('#D8E9E6') },
      },
      vertexShader: VERT,
      fragmentShader: FRAG,
      transparent: true,
      depthWrite: false,
    });
    const u = this.mat.uniforms;
    this.colours = { deep: u.uDeep.value, shallow: u.uShallow.value, edge: u.uRim.value, far: u.uSky.value };
    world.traverse((o) => {
      if (o instanceof THREE.Mesh && o.userData.paintKey === 'water') this.meshes.push(o);
    });
    for (const m of this.meshes) {
      m.material = this.mat;
      m.renderOrder = 2;
      m.castShadow = false;
      // the depth under the water comes from the normal pass: leave the water out of it
      ink.skip(m);
    }
  }

  update(time: number, camera: THREE.PerspectiveCamera): void {
    this.mat.uniforms.uTime.value = time;
    this.mat.uniforms.uNear.value = camera.near;
    this.mat.uniforms.uFar.value = camera.far;
  }
}
