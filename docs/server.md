# Server

`bombaychitra` is a single static binary (Go, no cgo) that serves everything: the REST API, vector tiles, map
styles, raw data downloads, the web app and these docs.

## Commands

```text
bombaychitra serve   [flags]   run the HTTP server
bombaychitra export  [flags]   export data as GeoJSON, NDJSON or CSV
bombaychitra info    [flags]   print dataset information
bombaychitra version           print the version
```

## `serve`

Every flag has an environment variable, which is convenient in containers.

| Flag | Env | Default | Description |
|---|---|---|---|
| `--addr` | `BC_ADDR` | `:8080` | Listen address |
| `--data` | `BC_DATA` | `data/build` | Directory with `bombaychitra.pmtiles` and `bombaychitra.db` |
| `--tiles` | `BC_TILES` | `<data>/bombaychitra.pmtiles` | Tile archive path |
| `--db` | `BC_DB` | `<data>/bombaychitra.db` | Database path |
| `--public-url` | `BC_PUBLIC_URL` | derived from the request | Absolute base URL written into TileJSON and styles, e.g. `https://maps.example.com` |
| `--trust-proxy` | `BC_TRUST_PROXY` | `false` | Honour `X-Forwarded-Proto` / `X-Forwarded-Host` |
| `--cors` | `BC_CORS` | `*` | `Access-Control-Allow-Origin` for data endpoints (empty disables CORS) |
| `--web` | `BC_WEB` | embedded | Serve the web app from a directory instead |
| `--downloads` | `BC_DOWNLOADS` | `true` | Expose `/data/bombaychitra.pmtiles` and `/data/bombaychitra.db` |
| `--log-format` | `BC_LOG_FORMAT` | `text` | `text` or `json` |
| `--log-level` | `BC_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |

### Endpoints

| Path | Description |
|---|---|
| `/` | The map |
| `/docs/` | Documentation; `/docs/api-reference/` is generated from the OpenAPI spec |
| `/api/v1/…` | REST API — see the [API guide](api.md) |
| `/tiles/bombaychitra.json`, `/tiles/{z}/{x}/{y}.mvt` | TileJSON and vector tiles |
| `/styles/light.json`, `/styles/dark.json` | Portable MapLibre styles pointing at this server |
| `/fonts/{fontstack}/{range}.pbf` | Glyphs used by the styles |
| `/data/bombaychitra.pmtiles`, `/data/bombaychitra.db` | Raw downloads with range support |
| `/openapi.json`, `/openapi.yaml` | API specification |
| `/healthz` | Liveness probe: `{"status":"ok","version":"…","dataset":"…"}` |

### Caching and compression

- **API:** JSON is gzip-compressed above 1 KB and carries a content-hash `ETag`; `If-None-Match` returns
  `304 Not Modified`. `Cache-Control: public, max-age=300, stale-while-revalidate=3600`.
- **Tiles:** stored gzip-compressed and sent as-is to clients that accept gzip (decompressed otherwise), with
  `Cache-Control: public, max-age=86400`, an `ETag` per tile and dataset, and `204` for empty tiles.
- **Static files:** brotli and gzip variants are precomputed at build time. Content-hashed assets under `/assets/`
  are cached for a year (`immutable`); HTML is revalidated with an `ETag`.

This makes BombayChitra well suited to running behind a CDN.

### Operational notes

- The database is opened read-only and treated as immutable. To publish new data, replace both files and restart.
- Shutdown is graceful on `SIGINT`/`SIGTERM` (in-flight requests get 10 seconds).
- Logs are structured (`log/slog`). Tile and asset requests log at `debug` level.
- Timeouts: 10 s to read headers, 30 s to read the request, 120 s to write (downloads are exempt).

## `export`

```text
bombaychitra export --kind places --category hospital --area makabo --format geojson -o hospitals.geojson
```

| Flag | Description |
|---|---|
| `--kind` | `places`, `buildings`, `bridges` (named), `streets` or `lines` |
| `--format` | `geojson` (FeatureCollection), `ndjson` (one feature per line) or `csv` |
| `--category`, `--group` | Comma-separated filters (places only) |
| `--area` | Restrict to an area id |
| `--data` / `--db` | Where the database is |
| `-o` | Output file (`-` for stdout) |

## `info`

Prints dataset metadata (build time, source timestamp, counts) and tile archive statistics.
