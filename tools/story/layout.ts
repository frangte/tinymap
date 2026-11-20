/** A layered left-to-right layout for a directed graph with cycles (a dialogue). No dependencies. */

export interface LayoutEdge {
  from: string;
  to: string;
}

export interface Placed {
  layer: number;
  /** top edge, in the same unit as the node heights */
  y: number;
}

const SWEEPS = 4;
const PLACE_PASSES = 3;

/** Depth-first from the roots; an edge to a node still on the stack closes a cycle and is left out of the layering. */
function forwardEdges(ids: string[], edges: LayoutEdge[], roots: string[]): { forward: LayoutEdge[]; order: string[] } {
  const out = new Map<string, LayoutEdge[]>();
  for (const e of edges) out.set(e.from, [...(out.get(e.from) ?? []), e]);
  const state = new Map<string, 'open' | 'done'>();
  const discovered: string[] = [];
  const back = new Set<LayoutEdge>();
  const visit = (id: string) => {
    state.set(id, 'open');
    discovered.push(id);
    for (const e of out.get(id) ?? []) {
      const s = state.get(e.to);
      if (s === 'open') back.add(e);
      else if (!s) visit(e.to);
    }
    state.set(id, 'done');
  };
  for (const id of [...roots, ...ids]) if (!state.has(id)) visit(id);
  return { forward: edges.filter((e) => !back.has(e) && e.from !== e.to), order: discovered };
}

/** Longest path from the roots: a node sits one layer right of the deepest node that points at it. */
function assignLayers(ids: string[], forward: LayoutEdge[], roots: string[]): Map<string, number> {
  const layer = new Map<string, number>(ids.map((id) => [id, 0]));
  const incoming = new Map<string, number>(ids.map((id) => [id, 0]));
  for (const e of forward) incoming.set(e.to, incoming.get(e.to)! + 1);
  const queue = ids.filter((id) => incoming.get(id) === 0);
  const rootSet = new Set(roots);
  queue.sort((a, b) => Number(rootSet.has(b)) - Number(rootSet.has(a)));
  while (queue.length > 0) {
    const id = queue.shift()!;
    for (const e of forward.filter((x) => x.from === id)) {
      layer.set(e.to, Math.max(layer.get(e.to)!, layer.get(id)! + 1));
      incoming.set(e.to, incoming.get(e.to)! - 1);
      if (incoming.get(e.to) === 0) queue.push(e.to);
    }
  }
  return layer;
}

function mean(values: number[], fallback: number): number {
  return values.length ? values.reduce((a, b) => a + b, 0) / values.length : fallback;
}

/** Orders each layer by the average position of its neighbours, sweeping both ways. */
function orderLayers(layers: string[][], forward: LayoutEdge[]): void {
  const position = new Map<string, number>();
  const refresh = () => layers.forEach((l) => l.forEach((id, i) => position.set(id, i)));
  refresh();
  for (let s = 0; s < SWEEPS; s++) {
    const down = s % 2 === 0;
    const range = down ? layers.map((_, i) => i).slice(1) : layers.map((_, i) => i).slice(0, -1).reverse();
    for (const li of range) {
      const bary = new Map<string, number>();
      for (const id of layers[li]) {
        const neighbours = forward.filter((e) => (down ? e.to === id : e.from === id)).map((e) => position.get(down ? e.from : e.to)!);
        bary.set(id, mean(neighbours, position.get(id)!));
      }
      layers[li].sort((a, b) => bary.get(a)! - bary.get(b)!);
      refresh();
    }
  }
}

/** Stacks one layer's nodes, each as near its wanted y as the others allow. */
function stack(layer: string[], wanted: Map<string, number>, heights: Map<string, number>, gap: number, y: Map<string, number>): void {
  let floor = 0;
  for (const id of layer) {
    const h = heights.get(id)!;
    const top = Math.max(floor, (wanted.get(id) ?? floor + h / 2) - h / 2);
    y.set(id, top);
    floor = top + h + gap;
  }
}

/**
 * Places every node: its layer (left to right, from the roots) and its top y.
 * Back edges are ignored for layering; the view draws them as loops.
 */
export function layoutLayered(ids: string[], edges: LayoutEdge[], roots: string[], heights: Map<string, number>, gap: number): Map<string, Placed> {
  const { forward, order } = forwardEdges(ids, edges, roots);
  const layerOf = assignLayers(ids, forward, roots);
  const count = Math.max(0, ...layerOf.values()) + 1;
  const layers: string[][] = Array.from({ length: count }, () => []);
  for (const id of order) layers[layerOf.get(id)!].push(id);
  orderLayers(layers, forward);
  const y = new Map<string, number>();
  layers.forEach((l) => stack(l, new Map(), heights, gap, y));
  const centre = (id: string) => y.get(id)! + heights.get(id)! / 2;
  for (let pass = 0; pass < PLACE_PASSES; pass++) {
    for (const li of layers.map((_, i) => i).slice(1)) {
      const wanted = new Map(layers[li].map((id) => [id, mean(forward.filter((e) => e.to === id).map((e) => centre(e.from)), NaN)]));
      stack(layers[li], new Map([...wanted].filter(([, v]) => !Number.isNaN(v))), heights, gap, y);
    }
    for (const li of layers.map((_, i) => i).slice(0, -1).reverse()) {
      const wanted = new Map(layers[li].map((id) => [id, mean(forward.filter((e) => e.from === id).map((e) => centre(e.to)), NaN)]));
      stack(layers[li], new Map([...wanted].filter(([, v]) => !Number.isNaN(v))), heights, gap, y);
    }
  }
  const top = Math.min(...y.values());
  return new Map(ids.map((id) => [id, { layer: layerOf.get(id)!, y: y.get(id)! - top }]));
}
