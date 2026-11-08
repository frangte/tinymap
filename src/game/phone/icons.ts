// App icons for the phone: a few ink strokes on a 24 × 24 grid, by the icon names in server/content/apps.json.
const PATHS: Record<string, string> = {
  profile: 'M12 12a4 4 0 1 0 0-8 4 4 0 0 0 0 8zM4 21c1.2-4 4.3-6 8-6s6.8 2 8 6',
  chat: 'M4 5h16v11H9l-5 4zM8 9.5h8M8 12.5h5',
  map: 'M3 6l6-2 6 2 6-2v14l-6 2-6-2-6 2zM9 4v14M15 6v14',
  bell: 'M6 17V11a6 6 0 0 1 12 0v6l2 2H4zM10 21h4',
  book: 'M4 5c3-1 6-1 8 1 2-2 5-2 8-1v14c-3-1-6-1-8 1-2-2-5-2-8-1zM12 6v14',
  people: 'M9 11a3.2 3.2 0 1 0 0-6.4A3.2 3.2 0 0 0 9 11zM3 20c.8-3.4 3-5 6-5s5.2 1.6 6 5M16 5.2a3 3 0 0 1 0 5.6M18 15c1.5.6 2.5 2.2 3 5',
  bike: 'M5.5 18a3 3 0 1 0 0-6 3 3 0 0 0 0 6zM18.5 18a3 3 0 1 0 0-6 3 3 0 0 0 0 6zM5.5 15l4-6h5l4 6M9.5 9 8 6H6M14.5 9l2-3h2',
  flag: 'M6 21V4M6 4h11l-2.5 4L17 12H6',
  pin: 'M12 21s-6.5-6.2-6.5-11.2a6.5 6.5 0 0 1 13 0C18.5 14.8 12 21 12 21zM12 12.3a2.5 2.5 0 1 0 0-5 2.5 2.5 0 0 0 0 5z',
  trophy: 'M8 4h8v5a4 4 0 0 1-8 0zM8 6H5a3 3 0 0 0 3 4M16 6h3a3 3 0 0 1-3 4M12 13v4M8.5 20h7M10 17h4',
};
const FALLBACK = 'M5 5h14v14H5z';

export function icon(name: string): SVGSVGElement {
  const ns = 'http://www.w3.org/2000/svg';
  const svg = document.createElementNS(ns, 'svg');
  svg.setAttribute('viewBox', '0 0 24 24');
  svg.setAttribute('aria-hidden', 'true');
  const path = document.createElementNS(ns, 'path');
  path.setAttribute('d', PATHS[name] ?? FALLBACK);
  svg.appendChild(path);
  return svg;
}
