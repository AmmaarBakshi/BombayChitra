// Search and reverse-geocode with the BombayChitra API (Node 18+, or any browser).
//   node search.mjs "gateway of india"
const BASE = `${process.env.BOMBAYCHITRA_URL ?? 'http://localhost:8080'}/api/v1`;

async function get(path, params = {}) {
  const res = await fetch(`${BASE}${path}?${new URLSearchParams(params)}`);
  if (!res.ok) throw new Error(`${res.status}: ${(await res.json()).error?.message}`);
  return res.json();
}

const query = process.argv[2] ?? 'gateway of india';
const { items } = await get('/search', { q: query, limit: 5 });
for (const r of items) {
  console.log(`[${r.kind}] ${r.name} — ${r.detail ?? ''} (${r.lat.toFixed(5)}, ${r.lon.toFixed(5)})`);
}

if (items[0]) {
  const here = await get('/reverse', { lat: items[0].lat, lon: items[0].lon });
  console.log(`\nTop result is at: ${here.label}`);
  if (here.road) console.log(`Nearest road: ${here.road.name ?? here.road.ref} (${here.road.distance_m} m away)`);
}
