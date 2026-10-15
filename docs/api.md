# API guide

The REST API exposes the whole dataset as JSON or GeoJSON. Every endpoint is a `GET` under `/api/v1`, needs no
authentication, and allows cross-origin requests, so you can call it from a browser, a mobile app or a backend in any
language. The complete, machine-readable contract is [`/openapi.json`](/openapi.json); the
[API reference](/docs/api-reference/) documents every parameter.

## Conventions

| Topic | Convention |
|---|---|
| Base URL | Your server's origin, e.g. `http://localhost:8080` |
| Coordinates | WGS84; points as `lat` + `lon` parameters, boxes as `bbox=minLon,minLat,maxLon,maxLat` |
| Ids | `osm_id × 10 + type` (1 node, 2 way, 3 relation; places from Overture Maps end in 4). Detail endpoints also accept `n123`, `w123`, `r123`. |
| Lists | `{"count": n, "items": [...]}` with `limit` (default 50, max 500) and `offset` |
| GeoJSON | add `format=geojson` to places, nearby, areas, streets, named bridges and routes |
| Errors | `{"error": {"status": 400, "code": "bad_request", "message": "…"}}` |
| Caching | `ETag` + `If-None-Match` → `304`; responses are gzip-compressed |

## Quick tour

```bash
B=http://localhost:8080

# What's in the dataset?
curl "$B/api/v1/meta"
curl "$B/api/v1/categories"
curl "$B/api/v1/areas"

# Search everything (areas, localities, lines, places, streets, bridges); bias towards a location
curl "$B/api/v1/search?q=andheri%20station&lat=19.12&lon=72.85"

# What's at this coordinate? Nearest road, locality, areas and places
curl "$B/api/v1/reverse?lat=19.0545&lon=72.8405"

# Hospitals and clinics within 1.5 km, nearest first
curl "$B/api/v1/nearby?lat=19.0178&lon=72.8478&radius=1500&category=hospital,clinic"

# All colleges in MaKaBo as GeoJSON
curl "$B/api/v1/places?area=makabo&category=college&format=geojson"

# One place in full (address, phone, website, hours, lines for stations, all OSM tags)
curl "$B/api/v1/places/311442962"

# The longest road bridges, and one of them with geometry
curl "$B/api/v1/bridges/named?kind=road&limit=10"
curl "$B/api/v1/bridges/named/1?format=geojson"

# Tallest buildings in Mumbai City
curl "$B/api/v1/buildings?area=mumbai-city&sort=height&limit=20"

# Metro lines, and one route with its stations as GeoJSON
curl "$B/api/v1/transit/lines?mode=subway"
curl "$B/api/v1/transit/routes/3808111?format=geojson"

# Statistics for an area
curl "$B/api/v1/stats?area=navi-mumbai"

# Distance by road: Andheri station to BKC (fastest by car), on foot, by two-wheeler without tolls, and as GeoJSON
curl "$B/api/v1/route?from=19.1197,72.8464&to=19.0660,72.8680"
curl "$B/api/v1/route?from=19.1197,72.8464&to=19.0660,72.8680&mode=foot"
curl "$B/api/v1/route?from=18.9985,72.8580&to=18.9310,73.0300&mode=two_wheeler&avoid=toll&steps=false"
curl "$B/api/v1/route?from=19.1197,72.8464&to=19.0660,72.8680&optimize=distance&format=geojson"

# By train, metro and monorail: Dadar to Ghatkopar, and the same trip by metro only
curl "$B/api/v1/journey?from=19.0190,72.8430&to=19.0864,72.9087"
curl "$B/api/v1/journey?from=19.0190,72.8430&to=19.0864,72.9087&modes=metro&format=geojson"

# Where can I get to from Kharghar in 10, 20 and 30 minutes on a two-wheeler?
curl "$B/api/v1/isochrone?from=19.0330,73.0700&mode=two_wheeler"

# Driving times between Andheri, Bandra and CSMT, every pair
curl -X POST "$B/api/v1/matrix" -H 'Content-Type: application/json'   -d '{"sources": [{"lat": 19.1197, "lon": 72.8464}, {"lat": 19.0544, "lon": 72.8402}, {"lat": 18.94, "lon": 72.8355}]}'

# Geocode a list of places in one request
curl -X POST "$B/api/v1/search/batch" -H 'Content-Type: application/json'   -d '{"queries": ["CST", "400050", "Powai Lake"]}'
```

## Common tasks

### Autocomplete

Call `/api/v1/search?q=<text>&limit=8` as the user types (debounce ~150 ms). The last word is matched as a prefix.
Pass the map centre as `lat`/`lon` to rank nearby results higher. Results include a `kind`, a human `detail` line,
coordinates and, for extended features (streets, bridges, lines, areas), a `bbox` to fit the map to.

### "Pin point" a location

`/api/v1/reverse` snaps a coordinate to the nearest named road (`road.snapped`, `road.distance_m`) and returns a
one-line `label` such as "Swami Vivekananda Road, Bandra West, Mumbai Suburban". Footpaths and service roads only win
when they are much closer than a real road.

### Distance by road

`/api/v1/route?from=lat,lon&to=lat,lon` returns the route between two points by car (default), two-wheeler or on
foot:

```json
{
  "mode": "car", "optimize": "time", "avoid": [],
  "distance_m": 10836, "duration_s": 950, "straight_m": 6388, "toll_m": 0,
  "from": { "lat": 19.1197, "lon": 72.8464, "snapped": { "lat": 19.119641, "lon": 72.846216, "road": "Madhavdas Amarshi Marg", "offset_m": 20 } },
  "to":   { "lat": 19.066, "lon": 72.868, "snapped": { "lat": 19.066015, "lon": 72.867986, "road": "Avenue 3", "offset_m": 2 } },
  "roads": [{ "name": "Madhavdas Amarshi Marg", "distance_m": 74 }, { "name": "Swami Vivekanand Road", "distance_m": 436 }, "…"],
  "steps": [
    { "type": "depart", "name": "Madhavdas Amarshi Marg", "distance_m": 74, "duration_s": 10, "location": [72.846216, 19.119641], "bearing": 312, "instruction": "Head north-west on Madhavdas Amarshi Marg" },
    { "type": "turn", "modifier": "left", "name": "Swami Vivekanand Road", "distance_m": 436, "duration_s": 45, "location": [72.845589, 19.119921], "bearing": 200, "instruction": "Turn left onto Swami Vivekanand Road" },
    "…",
    { "type": "arrive", "distance_m": 0, "duration_s": 0, "location": [72.867986, 19.066015], "bearing": 0, "instruction": "Arrive at your destination" }
  ],
  "bbox": [72.843633, 19.066015, 72.872476, 19.119921],
  "geometry": { "type": "LineString", "coordinates": [[72.846216, 19.119641], "…"] }
}
```

| Parameter | Values |
|---|---|
| `mode` | `car` (default), `two_wheeler`, `foot` — `drive`, `bike`, `scooter` and `walk` work too |
| `optimize` | `time` (default, fastest) or `distance` (shortest) |
| `avoid` | `toll` keeps off toll roads |
| `steps` | `false` leaves out the turn-by-turn steps |
| `format` | `geojson` for a FeatureCollection of the line and its ends |

**Travel modes.**

| Mode | Uses | Speed |
|---|---|---|
| `car` | Roads cars may use, following one-way rules and access tags | Typical urban speeds per road class: 65 km/h on expressways down to 18 km/h on residential streets |
| `two_wheeler` | The same roads except motorways: expressways, Atal Setu, the Bandra-Worli Sea Link, the Eastern Freeway and the Coastal Road are closed to two-wheelers | A fifth faster than cars through traffic, up to 60 km/h |
| `foot` | Roads (except motorways), footpaths, pedestrian streets, steps, subways and foot over-bridges, in either direction | 4.8 km/h, 2.4 km/h on steps |

**Tolls.** `toll_m` is the distance on toll roads (Atal Setu, the sea link, expressways — wherever OpenStreetMap
has `toll=yes`). With `avoid=toll` the route keeps off them; if that is impossible the answer is `422 no_route`.

**Steps.** Each step is one manoeuvre followed by a stretch of travel: `type` is `depart`, `turn`, `continue`,
`ramp`, `roundabout`, `exit_roundabout` or `arrive`; `modifier` is `straight`, `slight_left`, `left`,
`sharp_left`, `slight_right`, `right`, `sharp_right` or `uturn`; `distance_m` and `duration_s` cover the stretch
after the manoeuvre; `location` is where it happens and `bearing` the heading afterwards. `instruction` is
ready-made English ("Turn left onto Swami Vivekanand Road", "Cross the foot over-bridge"); apps that show another
language can build their own text from `type`, `modifier` and `name`. Consecutive segments of the same road are
merged into one step.

Notes:

- Each end is snapped to the nearest road or path the mode may use (within 1.5 km); `snapped.offset_m` is how far.
- Durations are off-peak estimates without live traffic. One-way streets, access restrictions and roundabouts are
  respected; turn restrictions are not.
- Errors: `422` with code `no_road` (nothing usable nearby) or `no_route` (not connected for that mode); `503
  mode_unavailable` from datasets built before travel modes existed (cars only).
- Routes are computed with A* on a graph of about 200,000 junctions in a few milliseconds; the graph is loaded on
  the first request and each mode's costs are prepared on its first use. Route requests cost five rate-limit tokens.

### Train, metro and monorail journeys

`/api/v1/journey` plans the quickest trip on the suburban railway, the metro lines and the monorail between two
points: a walk to a station, rides with any changes, and a walk to the destination.

```bash
curl "$B/api/v1/journey?from=19.0190,72.8430&to=19.0864,72.9087"   # Dadar to Ghatkopar
```

```json
{
  "duration_s": 1533, "distance_m": 11069, "walk_m": 694, "rides": 1, "changes": 0,
  "legs": [
    { "type": "walk", "to": { "name": "Dadar (Central)", "place": 26300750031, … }, "distance_m": 525, "duration_s": 418, … },
    { "type": "ride", "line": { "key": "Mumbai Suburban Railway:C", "label": "Central Line", "colour": "#ff0000", … },
      "headsign": "Kasara", "from": { "name": "Dadar (Central)", … }, "to": { "name": "Ghatkopar", … },
      "stops": 2, "via": [{ "name": "Kurla", … }], "wait_s": 120, "duration_s": 829, "distance_m": 10375, … },
    { "type": "walk", "from": { "name": "Ghatkopar", … }, "distance_m": 169, "duration_s": 165, … }
  ],
  "note": "Estimated from typical gaps between trains and running times, not from a timetable."
}
```

Every leg has a `geometry` to draw: rides follow the line's track, walks follow streets, footpaths and foot
over-bridges. Stations carry their `place` id, so `/api/v1/places/{id}` gives their platforms, lines and opening
date. With `format=geojson` the journey is a FeatureCollection ready for a map (see the [reference](/docs/api-reference/)).

- **How times are estimated.** Each boarding waits half the line's usual daytime gap between trains — 4 minutes
  on the Western and Central lines, 7 on the Harbour line, 5 to 10 on the Mumbai Metro lines (`transit.headways` in
  `config/region.json`), and twice as long for express patterns. Trains run at a typical speed for their mode with an
  allowance for every stop, and a change adds a minute and a half for platforms plus any walk between stations
  (for example Marol Naka on Lines 1 and 3). There is no timetable in OpenStreetMap, so peak crowding, late trains
  and the last train of the night are not known.
- `modes` picks what to ride (`train`, `metro`, `monorail`); `max_walk` (200–3000 m, default 1500) limits the walk
  at either end, as the crow flies. A destination within walking distance is reached on foot when that is quicker.
- Errors: `422` with code `no_station` (no station within `max_walk` of a point) or `no_journey`.
- The station network (about 230 stations on 15 lines) is built on the first request; journeys take a few
  milliseconds plus the walks. Journey requests cost five rate-limit tokens.

### Isochrones: how far in a given time

`/api/v1/isochrone` returns the areas you can reach from a point within up to four times, by car, two-wheeler or on
foot — the same speeds and one-way rules as routes — as GeoJSON ready to draw:

```bash
curl "$B/api/v1/isochrone?from=19.0330,73.0700&mode=foot&minutes=10,20"
```

Each time is a `MultiPolygon` feature (`role: isochrone`) with `minutes`, `mode` and `area_km2`, largest first, so
drawing them in order leaves the shorter times on top; the last feature is the start snapped to the network
(`role: from`). The areas follow every road and path reached and the land beside them, on a 75 m grid on foot and
200 m by road. Times run from 1 to 60 minutes; an isochrone costs ten rate-limit tokens. In the app, **Reach** in the
distance panel draws 10, 20 and 30 minutes from the start.

### Distance matrices

`POST /api/v1/matrix` gives the travel time and distance from every source to every destination — for delivery
planning, "which branch is nearest by road", or choosing a meeting point. Up to 10 sources and 50 destinations; leave
out `destinations` for every pair of the sources:

```bash
curl -X POST "$B/api/v1/matrix" -H 'Content-Type: application/json' -d '{
  "sources": [{"lat": 19.1197, "lon": 72.8464}, {"lat": 19.0544, "lon": 72.8402}],
  "destinations": [{"lat": 19.066, "lon": 72.868}, {"lat": 18.94, "lon": 72.8355}],
  "mode": "two_wheeler"
}'
```

`durations[i][j]` (seconds) and `distances[i][j]` (metres) go from source `i` to destination `j` along the fastest
route (or the shortest with `"optimize": "distance"`), exactly as `/api/v1/route` would find it; `null` means the
destination cannot be reached, and a point with no road within 1.5 km has `"snapped": null`. One search runs per
source, so a 10 × 50 matrix takes about as long as ten routes. A matrix costs ten rate-limit tokens.

### Batch geocoding

`POST /api/v1/search/batch` runs up to 100 searches in one request and answers each as `/api/v1/search` would, in
order — handy for turning a spreadsheet of addresses or station names into coordinates:

```bash
curl -X POST "$B/api/v1/search/batch" -H 'Content-Type: application/json'   -d '{"queries": ["CST", "400050", "Powai Lake"], "near": {"lat": 19.07, "lon": 72.87}}'
```

Each result has the `query`, a `count` and `items` — the best match by default, up to 10 with `limit`. `kind` limits
every query to kinds of result (for example `["place"]`). A batch costs ten rate-limit tokens. From JavaScript,
the bundled client has `matrix()` and `searchBatch()`.

### Wards and PIN codes

```bash
curl "$B/api/v1/areas?kind=ward"                           # the 24 BMC wards
curl "$B/api/v1/stats?area=ward-kw"                        # what is in K/W ward (Andheri West, Juhu)
curl "$B/api/v1/places?area=ward-a&category=hospital"      # hospitals in A ward
curl "$B/api/v1/areas?kind=pincode"                        # PIN code areas, derived from addresses
curl "$B/api/v1/areas/400050?format=geojson"               # one PIN's outline and statistics
curl "$B/api/v1/places?area=400050&category=school"        # schools addressed with PIN 400050
```

`/api/v1/reverse` returns the PIN (`postcode`) of any point, and searching for a six-digit PIN finds its area.

### Heritage and the monsoon

```bash
curl "$B/api/v1/heritage?status=unesco"                    # the three World Heritage Sites
curl "$B/api/v1/heritage?format=geojson"                    # all curated heritage sites, for a map layer
curl "$B/api/v1/hotspots"                                   # waterlogging hotspots and whether it is the monsoon
```

A place that represents a heritage site carries it in its details as `heritage` (status, year of inscription,
description, Wikipedia article). Every route lists the waterlogging `hotspots` it passes and says whether it is
`monsoon` season (June to September, India Standard Time); warn your users when it is.

### Search

Search is forgiving of how Indian names are spelled in Latin script: queries and names are compared with accents
dropped, aspiration (`bh`, `dh`, `kh`…) ignored, `chh`/`ch`, `ph`/`f`, `w`/`v`, `z`/`j`, `ee`/`i` and `oo`/`u`
treated alike and doubled letters single, and without spaces — "bandup", "gatkopar", "vorli" and "vileparle"
find Bhandup, Ghatkopar, Worli and Vile Parle. Station codes and former names count as names ("CST", "VT",
"Victoria Terminus"), and a six-digit PIN finds its area.

### Places along a route or in view

Use `bbox` with the visible map bounds and a `category` or `group` filter. For a radius, use `/nearby`, which
sorts by distance and adds `distance_m`.

### Areas

Any list endpoint accepts `area=<id>`. Area ids are listed by `/api/v1/areas`; `/api/v1/areas/{id}` returns the
boundary geometry and statistics.

## Client libraries

Official clients for Python and JavaScript/TypeScript are generated from `api/openapi.yaml`, so every endpoint is a
method named after its operation (`search`, `route`, `journey`, `isochrone`, `matrix`, `list_places` /
`listPlaces`…). Both have no dependencies, send an API key when given one and raise an error carrying the API's
status, code and message.

```bash
pip install bombaychitra       # sdk/python
npm install bombaychitra       # sdk/js
```

```python
from bombaychitra import BombayChitra
api = BombayChitra("http://localhost:8080")
api.route("19.1197,72.8464", "18.9400,72.8355", mode="two_wheeler")["duration_s"]
```

```js
import { BombayChitra } from 'bombaychitra';
const api = new BombayChitra('http://localhost:8080');
const journey = await api.journey('19.0190,72.8430', '19.0864,72.9087', { modes: ['metro'] });
```

After changing the spec, run `python scripts/generate-sdk.py` (CI fails while the clients are out of date). For other
languages any [OpenAPI Generator](https://openapi-generator.tech/) target works:

```bash
npx @openapitools/openapi-generator-cli generate -i http://localhost:8080/openapi.json -g kotlin -o bombaychitra-kotlin
```

See [Integrations](integrations.md) for examples in other languages.

## Rate limits and fair use

Public servers may limit each client (see [Server](server.md#rate-limiting-and-api-keys)). Over the limit the API
answers `429 Too Many Requests` with `Retry-After`; every limited response carries `X-RateLimit-Limit` and
`X-RateLimit-Remaining`. Most requests cost one token; routes and journeys cost five. An API key, sent in the
`X-API-Key` header, lifts the limit. For bulk use, prefer the [data downloads](data.md#downloads) over crawling the
API.
