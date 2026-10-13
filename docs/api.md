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

### Places along a route or in view

Use `bbox` with the visible map bounds and a `category` or `group` filter. For a radius, use `/nearby`, which
sorts by distance and adds `distance_m`.

### Areas

Any list endpoint accepts `area=<id>`. Area ids are listed by `/api/v1/areas`; `/api/v1/areas/{id}` returns the
boundary geometry and statistics.

## Generating a client

Any [OpenAPI Generator](https://openapi-generator.tech/) target works:

```bash
npx @openapitools/openapi-generator-cli generate -i http://localhost:8080/openapi.json -g python -o bombaychitra-python
npx @openapitools/openapi-generator-cli generate -i http://localhost:8080/openapi.json -g kotlin -o bombaychitra-kotlin
```

TypeScript users can copy `web/src/api/client.ts`, a dependency-free typed client used by the web app itself. See
[Integrations](integrations.md) for examples in other languages.

## Rate limits and fair use

The server does not rate-limit by itself. If you expose it publicly, put it behind a reverse proxy or CDN and apply
limits there (see [Deployment](deployment.md)). For bulk use, prefer the [data downloads](data.md#downloads) over
crawling the API.
