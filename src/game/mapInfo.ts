// The island as a whole: server/content/map.json, which the Go server embeds too (and whose tests keep it valid).
import mapData from '../../server/content/map.json';

export interface MapInfo {
  /** where a player without an archetype starts, Blender XY */
  spawn: readonly [number, number];
  /** the height (Blender Z = three.js y) the sea is drawn at; a body cannot wade deeper than collider.ts WADE into it */
  seaLevel: number;
  /** the middle of the island, Blender XY: the gulls and clouds circle it */
  islandCenter: readonly [number, number];
  /** the island and its beach, Blender XY [x0, y0, x1, y1] */
  bounds: readonly [number, number, number, number];
}

function pair(v: readonly number[]): [number, number] {
  return [v[0], v[1]];
}

export const MAP: Readonly<MapInfo> = {
  spawn: pair(mapData.spawn),
  seaLevel: mapData.sea_level,
  islandCenter: pair(mapData.island_center),
  bounds: [mapData.bounds[0], mapData.bounds[1], mapData.bounds[2], mapData.bounds[3]],
};
