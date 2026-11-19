import './style.css';
import { startApp } from './game/app';
import { startPwa } from './pwa';

startApp()
  .then(startPwa)
  .catch((err: unknown) => {
    console.error(err);
  });
