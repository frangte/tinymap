import './style.css';
import { startApp } from './game/app';

startApp().catch((err: unknown) => {
  console.error(err);
});
