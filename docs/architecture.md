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
| `server/` | The Go server: `cmd/bombaychitra` (CLI), `internal/pmtiles` (tile reader), `internal/store` (queries), `internal/httpapi` (HTTP), `internal/export` (GeoJSON/CSV). |
| `web/` | The web app (`src/`), the map style builder (`src/map/style/`) and build plugins for docs, styles, icons and precompression (`build/`). |
| `api/openapi.yaml` | The API contract; also renders the API reference and lets you generate clients. |
| `docs/` | This documentation, rendered into the web build. |
| `examples/` | Small programs using the API and the database from many languages. |
| `scripts/` | Data build scripts for Bash and PowerShell. |

## Why these technologies

**Planetiler** is the fastest open-source OpenStreetMap-to-vector-tiles engine. It processes the 220 MB extract on
all cores in about a minute, handles multipolygons and route relations, and lets each layer be an independent handler.
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
| Data build | ~65 s for the full MMR on 8 cores (tiles + database) |
| Tiles | 43 MB for z0–z15, largest tile 116 KB gzipped |
| Database | 64 MB with spatial and full-text indexes |
| Web app | 46 KB gzipped app code + MapLibre (286 KB gzipped, cached separately) |
| Tile request | one in-memory directory lookup + one disk read, served gzip-encoded as stored |
| Static files | brotli/gzip precompressed at build time, content-hashed and cached immutably |

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
  document it in `api/openapi.yaml`.
