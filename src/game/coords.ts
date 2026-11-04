import * as THREE from 'three';

/**
 * The diorama is modelled in Blender: metres, Z up, X east, Y north.
 * three.js is Y up, so Blender (x, y, z) maps to three (x, z, -y).
 */
export function fromBlender(x: number, y: number, z = 0, out: THREE.Vector3 = new THREE.Vector3()): THREE.Vector3 {
  return out.set(x, z, -y);
}

/** Wrap an angle into (-PI, PI]. */
export function wrapAngle(a: number): number {
  return Math.atan2(Math.sin(a), Math.cos(a));
}

/** Frame-rate independent smoothing factor for `value += (goal - value) * damp(rate, dt)`. */
export function damp(rate: number, dt: number): number {
  return 1 - Math.exp(-rate * dt);
}

export function clamp(v: number, lo: number, hi: number): number {
  return v < lo ? lo : v > hi ? hi : v;
}

export function smoothstep(e0: number, e1: number, v: number): number {
  const t = clamp((v - e0) / (e1 - e0), 0, 1);
  return t * t * (3 - 2 * t);
}
