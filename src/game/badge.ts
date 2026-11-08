// An archetype's badge next to a name (chat lines, the phone): its short label on its colour.
import { archetypeById } from './archetypes';

/** the badge element, or null for a player without an archetype */
export function badge(arch: string): HTMLElement | null {
  const a = archetypeById(arch);
  if (!a) return null;
  const b = document.createElement('span');
  b.className = 'badge';
  b.textContent = a.badge.short;
  b.style.background = a.badge.color;
  b.style.color = a.badge.text;
  return b;
}
