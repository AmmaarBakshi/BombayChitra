# Architecture

BombayChitra has three components connected by two open file formats.

```text
                ┌───────────────────────────── pipeline/ (Java 25 + Planetiler) ─────────────────────────────┐
 OpenStreetMap  │  config/region.json ─┐                                                                   │
 extract (.pbf) │  config/categories.json ─► layer handlers ─► vector tiles ──────► bombaychitra.pmtiles     │
 ─────────────► │                         (one pass, all cores) └► records ─────► bombaychitra.db (SQLite)  │
                └───────────────────────────────────────────────────────────────────────────────────────────┘
                                                        │                               │
                ┌───────────────────────────── server/ (Go, single static binary) ─────────────────────────┐
                │  /tiles  ◄── PMTiles reader (one pread per tile)                                          │
                │  /api/v1 ◄── SQLite: R*Tree spatial index + FTS5 full-text search                        │
                │  /styles, /docs, /  ◄── embedded web build (precompressed brotli/gzip)                    │
                └───────────────────────────────────────────────────────────────────────────────────────────┘
                                                        │
                ┌───────────────────────────── web/ (TypeScript + MapLibre GL JS) ─────────────────────────┐
                │  map UI · light/dark styles · search · inspector · transit · bridges · 3D · docs site      │
                └───────────────────────────────────────────────────────────────────────────────────────────┘
```

## Repository layout

| Path | What it is |
|---|---|
| `config/` | `region.json` (coverage, areas, transit colours) and `categories.json` (place taxonomy). Shared by every component. |
| `pipeline/` | The data build. One handler class per tile layer in `pipeline/src/bombaychitra/pipeline/layers/`; the SQLite writer in `…/data/`. |
| `server/` | The Go server: `cmd/bombaychitra` (CLI), `internal/pmtiles` (tile reader), `internal/store` (queries), `internal/routing` (routes, isochrones, matrices), `internal/journey` (train and metro journeys), `internal/httpapi` (HTTP), `internal/export` (GeoJSON, CSV, MapRoulette). |
| `web/` | The web app (`src/`), the map style builder (`src/map/style/`) and build plugins for docs, styles, icons and precompression (`build/`). |
| `api/openapi.yaml` | The API contract; renders the API reference and generates the clients in `sdk/`. |
| `sdk/` | Python and JavaScript clients generated from the API contract by `scripts/generate-sdk.py`. |
| `docs/` | This documentation, rendered into the web build. |
| `examples/` | Small programs using the API and the database from many languages. |
| `scripts/` | Data build scripts for Bash and PowerShell, the pipeline test runner and the data regression check. |
| `deploy/` | Ready-made deployments: Fly.io, Docker Compose with Caddy, systemd. |

## Why these technologies

**Planetiler** is the fastest open-source OpenStreetMap-to-vector-tiles engine. It processes the 220 MB extract on
all cores in a couple of minutes, reads OpenStreetMap and GeoParquet (Overture Maps) alike, handles multipolygons and
route relations, and lets each layer be an independent handler.
The same pass also collects the records for the database, so the source is read once.

**PMTiles** stores the whole tile pyramid in a single file with a compact Hilbert-curve index. The server looks a
tile up in an in-memory directory and returns it with one positioned read — no tile database, tile server or cache
warm-up. The same file can also be read directly by browsers and CDNs over HTTP range requests.

**SQLite** is the most widely deployed database in the world and has drivers in every language. The database ships
with an R*Tree spatial index, an FTS5 full-text index and precomputed statistics, so it is useful on its own — copy it
into an Android app, a Jupyter notebook or a data warehouse.

**Go** produces a single static binary (no runtime, no cgo) that embeds the web app. It starts in milliseconds and
serves thousands of requests per second on modest hardware.

**MapLibre GL JS** renders vector tiles on the GPU. The style is generated from TypeScript, so light and dark themes
share one definition, and the same styles work in MapLibre Native on Android and iOS.

## Performance characteristics

| Stage | Result |
|---|---|
| Data build | ~2½ min for the full MMR on 8 cores (tiles, database and routing graph), 3 GB heap |
| Tiles | 96 MB for z0–z15 with 2.1 million buildings, largest tile 150 KB gzipped |
| Database | 212 MB with spatial and full-text indexes and the routing graph |
| Routing | A* over 198,000 junctions (roads, footpaths, steps, foot over-bridges): 1–25 ms per route; the graph takes ~40 MB of server memory, plus ~4 MB per travel mode used |
| Web app | 46 KB gzipped app code + MapLibre (286 KB gzipped, cached separately) |
| Tile request | one in-memory directory lookup + one disk read, served gzip-encoded as stored |
| Static files | brotli/gzip precompressed at build time, content-hashed and cached immutably |

## How routing works

The pipeline turns every way someone may use into one graph: ways are split where they share a node, each segment
keeps its shape, length, road class, car speed, flags (toll, steps, bridge, roundabout, ramp) and which of car,
two-wheeler and pedestrian may use it, and directed edges are stored in compressed sparse row form (see
[Database](database.md#routing-graph)). The server loads the graph on the first route request and, per travel mode,
derives a profile of travel times — `+Inf` where the mode may not go. A route is an A* search over those times with a
straight-line heuristic scaled by the mode's top speed, from the nearest point on a usable segment to the
destination's. The path is then walked once more to assemble the geometry, the roads used and turn-by-turn steps:
consecutive segments of one road merge into one step, the turn angle between headings names the manoeuvre, and
unnamed paths are described for walkers ("Cross the foot over-bridge").

The same graph answers the other questions. An **isochrone** runs the search without a target until the time limit,
recording every point reached with its time; points are binned into a grid (75 m on foot, 200 m by road), each cell
grows by its neighbours to cover the blocks beside the roads, and the cells within each time are traced into
polygons with holes. A **matrix** runs one search per source and stops once every destination's road is settled.

## How journeys work

`internal/journey` builds a station network from the transit lines in the database when the first journey is asked
for: stops of one mode with the same name become one station, stations within 450 m are joined by short walks for
changing lines, and each route (a line in one direction, stopping or fast) runs along its track. A ride takes the
distance at the mode's running speed plus an allowance per stop; boarding waits half the line's usual gap between
trains (`transit.headways` in `config/region.json`, recorded in the dataset), twice as long for express patterns. A
journey is a Dijkstra search from every station within walking distance of the start to the stations near the
destination, and its walks are then routed along footpaths with the foot profile. Times are estimates, not a
timetable.

## Data flow for a click on the map

1. MapLibre hit-tests the rendered vector features under the cursor.
2. The feature id from the tile (`osm_id × 10 + type`) is passed to the API, e.g. `GET /api/v1/places/311442962`.
3. The server reads one row by primary key and returns full details.
4. The inspector renders it and the selection is drawn from the returned geometry.

## Modularity

- **Add a tile layer:** write a handler in `pipeline/…/layers/`, register it in `BombayChitraProfile`, then style it
  in `web/src/map/style/layers.ts`. Build only some layers with `--only_layers=roads,pois`.
- **Add place categories:** edit `config/categories.json`. The pipeline, database, API, legend and map icons all
  update from it.
- **Change the region:** edit `config/region.json` (bounds, source extract, areas). See [Configuration](configuration.md).
- **Add an API endpoint:** add a query to `server/internal/store`, a handler to `server/internal/httpapi`, and
  document it in `api/openapi.yaml`; then run `python scripts/generate-sdk.py` to add it to the clients.
