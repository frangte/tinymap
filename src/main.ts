// first: a play-test tab swaps the browser's storage for a copy in memory before anything reads it
import './net/playtest';
import './style.css';
import { startApp } from './game/app';
import { startInstallButton } from './install';
import { startPlaytestBadge } from './playtestBadge';
import { startPwa } from './pwa';

// before the map: Chrome may offer the install while it loads
startInstallButton();
startPlaytestBadge();
startApp()
  .then(startPwa)
  .catch((err: unknown) => {
    console.error(err);
  });
