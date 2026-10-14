// BombayChitra service worker: keeps the app working without a connection.
//
// - The app shell (page, scripts, styles, icons and map fonts) is cached at install, so the app opens offline after one
//   visit; a new deployment installs a new shell and the old one is removed.
// - Pages are fetched from the network first and fall back to the cached app (or the offline page for the docs).
// - Map tiles come from the areas saved for offline use (caches named bc-area-…) and otherwise from the network;
//   when both fail an empty tile keeps the map quiet instead of erroring.
// - The dataset's metadata, categories and TileJSON are fetched from the network first and kept for offline use.
//
// The build fills in VERSION and PRECACHE (see build/sw.ts).

const VERSION = '__BC_VERSION__';
const PRECACHE = __BC_PRECACHE__;
const SHELL = `bc-shell-${VERSION}`;
const RUNTIME = 'bc-runtime';
const NETWORK_FIRST = ['/api/v1/meta', '/api/v1/categories', '/tiles/bombaychitra.json', '/styles/'];

self.addEventListener('install', (event) => {
  event.waitUntil(caches.open(SHELL).then((cache) => cache.addAll(PRECACHE)).then(() => self.skipWaiting()));
});

self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys()
      .then((keys) => Promise.all(keys.filter((k) => k.startsWith('bc-shell-') && k !== SHELL).map((k) => caches.delete(k))))
      .then(() => self.clients.claim()),
  );
});

self.addEventListener('fetch', (event) => {
  const request = event.request;
  if (request.method !== 'GET') return;
  const url = new URL(request.url);
  if (url.origin !== self.location.origin) return;
  if (request.mode === 'navigate') {
    event.respondWith(page(request, url));
  } else if (url.pathname.startsWith('/tiles/') && url.pathname.endsWith('.mvt')) {
    event.respondWith(tile(request));
  } else if (PRECACHE.includes(url.pathname)) {
    event.respondWith(caches.match(url.pathname).then((hit) => hit ?? fetch(request)));
  } else if (url.pathname.startsWith('/fonts/') || NETWORK_FIRST.some((p) => url.pathname.startsWith(p))) {
    event.respondWith(networkFirst(request));
  }
});

/** A page: the network, else the cached app (or the offline page for the documentation). */
async function page(request, url) {
  try {
    return await fetch(request);
  } catch {
    const fallback = url.pathname.startsWith('/docs') ? '/offline.html' : '/';
    return (await caches.match(fallback)) ?? (await caches.match('/offline.html')) ?? Response.error();
  }
}

/** A map tile: a saved area's copy, else the network, else an empty tile. */
async function tile(request) {
  const saved = await caches.match(request, { ignoreSearch: true });
  if (saved) return saved;
  try {
    return await fetch(request);
  } catch {
    return new Response(null, { status: 204 });
  }
}

/** Fresh from the network when online, the last copy when not. */
async function networkFirst(request) {
  try {
    const response = await fetch(request);
    if (response.ok) {
      const cache = await caches.open(RUNTIME);
      await cache.put(request, response.clone());
    }
    return response;
  } catch {
    return (await caches.match(request)) ?? Response.error();
  }
}
