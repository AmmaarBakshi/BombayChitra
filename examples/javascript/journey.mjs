// A train and metro journey, leg by leg, with the walks to and from the stations (Node 18+, no dependencies).
//
//   node journey.mjs                                   # Dadar to Ghatkopar
//   node journey.mjs 19.2290,72.8566 19.1860,72.9750   # Borivali to Thane
//   node journey.mjs 19.0190,72.8430 19.0864,72.9087 metro
const base = (process.env.BOMBAYCHITRA_URL ?? 'http://localhost:8080') + '/api/v1';
const [from = '19.0190,72.8430', to = '19.0864,72.9087', modes] = process.argv.slice(2);

const query = new URLSearchParams({ from, to, ...(modes ? { modes } : {}) });
const res = await fetch(`${base}/journey?${query}`);
const journey = await res.json();
if (!res.ok) {
  console.error(journey.error?.message ?? res.statusText);
  process.exit(1);
}

const min = (s) => `${Math.max(1, Math.round(s / 60))} min`;
console.log(`${min(journey.duration_s)} · ${journey.rides} train${journey.rides === 1 ? '' : 's'} · ${journey.changes} change${journey.changes === 1 ? '' : 's'}`);
for (const leg of journey.legs) {
  if (leg.type === 'walk') {
    console.log(`  Walk ${Math.round(leg.distance_m)} m to ${leg.to.name ?? 'your destination'} (${min(leg.duration_s)})`);
  } else {
    const via = leg.via.length ? `, via ${leg.via.map((s) => s.name).join(', ')}` : '';
    console.log(`  ${leg.line.label}${leg.express ? ' (fast)' : ''} towards ${leg.headsign}: ${leg.from.name} → ${leg.to.name}, ${leg.stops} stop${leg.stops === 1 ? '' : 's'}${via} (wait ~${min(leg.wait_s)}, ride ${min(leg.duration_s)})`);
  }
}
console.log(journey.note);
