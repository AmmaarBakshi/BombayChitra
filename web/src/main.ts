import 'maplibre-gl/dist/maplibre-gl.css';
import './styles/app.css';
import { createApp } from './app.ts';

createApp();

// The service worker keeps the app and saved map areas available offline (see build/sw.js). Development builds run
// without it, so changes always show.
if (import.meta.env.PROD && 'serviceWorker' in navigator) {
  addEventListener('load', () => void navigator.serviceWorker.register('/sw.js').catch(() => {}));
}
