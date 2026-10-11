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
| `--rate-limit` | `BC_RATE_LIMIT` | `0` (off) | API requests per minute per client |
| `--rate-burst` | `BC_RATE_BURST` | rate ÷ 10 (min. 10) | API requests a client may make at once |
| `--api-keys` | `BC_API_KEYS` | none | Comma-separated API keys, or `@file` with one key per line (`#` comments allowed) |
| `--require-key` | `BC_REQUIRE_KEY` | `false` | Reject API requests without a valid key |

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

### Rate limiting and API keys

A public server should limit how much one client can ask for: a route search walks a road graph of ~200,000
segments, and nothing stops a script from asking for thousands. Rate limiting is off by default (a server on your
own machine needs none) and is turned on with `--rate-limit`:

```bash
bombaychitra serve --rate-limit 300 --rate-burst 40 --trust-proxy
```

- Each client gets a **token bucket** of `--rate-burst` tokens that refills at `--rate-limit` tokens per minute.
  Most requests cost one token; `/api/v1/route` costs five.
- Clients are identified by **IP address** — IPv6 clients by their `/64` network. Behind a reverse proxy, pass
  `--trust-proxy` so the address the proxy saw (`X-Real-IP`, or the right-most `X-Forwarded-For` entry) is used;
  without it every request appears to come from the proxy.
- Only `/api/` is limited. Tiles, styles, fonts and the web app are static and cache well, so they are not.
- Every limited response carries `X-RateLimit-Limit` and `X-RateLimit-Remaining`. When the bucket is empty the
  server answers `429 Too Many Requests` with `Retry-After` (seconds) and
  `{"error":{"status":429,"code":"rate_limited",…}}`.

**API keys** let known users (your own apps, partners) skip the limit. Give the server a list of keys and clients
send theirs in the `X-API-Key` header (or the `key` query parameter, for tools that cannot set headers):

```bash
openssl rand -hex 24 >> keys.txt                       # one key per line, at least 16 characters
bombaychitra serve --rate-limit 300 --api-keys @keys.txt
curl -H "X-API-Key: $(head -1 keys.txt)" "http://localhost:8080/api/v1/route?from=19.1197,72.8464&to=19.0660,72.8680"
```

An unknown key is rejected with `401` (`invalid_key`), so a typo is noticed rather than silently rate-limited. With
`--require-key` the API is private: requests without a key get `401` (`missing_key`), while the map's tiles and
styles stay public.

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
- The routing graph is loaded on the first `/api/v1/route` request (~0.3 s) and then uses about 30 MB of memory,
  plus ~3 MB per route being computed; at most four routes are computed at once.
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
