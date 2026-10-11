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

# Distance by road: Andheri station to BKC (fastest), and the shortest route as GeoJSON
curl "$B/api/v1/route?from=19.1197,72.8464&to=19.0660,72.8680"
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

`/api/v1/route?from=lat,lon&to=lat,lon` returns the driving route between two points:

```json
{
  "optimize": "time",
  "distance_m": 10812, "duration_s": 975, "straight_m": 6405,
  "from": { "lat": 19.1197, "lon": 72.8464, "snapped": { "lat": 19.119702, "lon": 72.846381, "road": "Andheri - Kurla Road", "offset_m": 2 } },
  "to":   { "lat": 19.066, "lon": 72.868, "snapped": { "lat": 19.065965, "lon": 72.867996, "road": "G Block Road", "offset_m": 4 } },
  "roads": [{ "name": "Andheri - Kurla Road", "distance_m": 1830 }, { "name": "Western Express Highway", "distance_m": 4120 }],
  "bbox": [72.8464, 19.066, 72.868, 19.1197],
  "geometry": { "type": "LineString", "coordinates": [[72.846381, 19.119702], "…"] }
}
```

- Each end is snapped to the nearest road a car can use (within 1.5 km); `snapped.offset_m` is how far that is.
- `optimize=time` (default) gives the fastest route, `optimize=distance` the shortest.
- Durations use typical urban speeds per road class (65 km/h on expressways down to 18 km/h on residential
  streets), not live traffic, so treat them as off-peak estimates. One-way streets, access restrictions and
  roundabouts are respected; turn restrictions are not.
- Errors: `422` with code `no_road` (nothing drivable nearby) or `no_route` (not connected).
- Routes are computed with A* on a 170,000-junction graph in a few milliseconds; the graph is loaded on the first
  request.

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
