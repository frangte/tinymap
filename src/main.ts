import './style.css';
import { startApp } from './game/app';
import { startInstallButton } from './install';
import { startPwa } from './pwa';

// before the map: Chrome may offer the install while it loads
startInstallButton();
startApp()
  .then(startPwa)
  .catch((err: unknown) => {
    console.error(err);
  });
