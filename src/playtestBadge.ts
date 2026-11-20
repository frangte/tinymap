// A small label on a test character's tab (src/net/playtest.ts), so a play-test is never taken for a real game.
import { onLangChange } from './game/i18n';
import { L } from './game/i18n/playtest';
import { playtest } from './net/playtest';

export function startPlaytestBadge(): void {
  const session = playtest;
  if (!session) return;
  const badge = document.createElement('div');
  badge.className = 'playtest-badge';
  badge.setAttribute('role', 'status');
  const show = (): void => {
    badge.textContent = `${L.badge} · ${session.name}`;
    badge.title = L.note;
  };
  show();
  onLangChange(show);
  document.body.append(badge);
}
