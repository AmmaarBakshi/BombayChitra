# bombaychitra (JavaScript)

A small, dependency-free client for the [BombayChitra](https://github.com/AmmaarBakshi/BombayChitra) API — the open
map of Mumbai, Thane, Navi Mumbai and the wider MMR — for browsers and Node 18+, with TypeScript declarations. It is
generated from `api/openapi.yaml`, so every endpoint is a method named after its operation.

```bash
npm install bombaychitra
```

```js
import { BombayChitra, BombayChitraError } from 'bombaychitra';

const api = new BombayChitra('http://localhost:8080'); // { apiKey: '…' } for servers that require one

const { items } = await api.search('CST', { limit: 1 });
console.log(items[0].name, items[0].lat, items[0].lon);

const route = await api.route('19.1197,72.8464', '18.9400,72.8355', { mode: 'two_wheeler' });
console.log(route.distance_m, route.duration_s);

const journey = await api.journey('19.0190,72.8430', '19.0864,72.9087', { modes: ['train', 'metro'] });
for (const leg of journey.legs) console.log(leg.type, leg.line?.label ?? '', leg.duration_s);

const areas = await api.isochrone('19.0330,73.0700', { mode: 'foot', minutes: [10, 20] }); // GeoJSON

try {
  await api.route('19.1197,72.8464', '18.5,72.0');
} catch (err) {
  if (err instanceof BombayChitraError) console.log(err.status, err.code, err.apiMessage);
}
```

Required parameters are positional (path ids, `q`, `from` and `to`), the rest go in an options object; lists can be
arrays. POST endpoints (`matrix`, `searchBatch`) take the JSON body. Pass `{ fetch }` to use another fetch
implementation. See the [API guide](https://github.com/AmmaarBakshi/BombayChitra/blob/main/docs/api.md) for every
field.
