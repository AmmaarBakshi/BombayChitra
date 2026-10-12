# Server

`bombaychitra` is a single static binary (Go, no cgo) that serves everything: the REST API, vector tiles, map
styles, raw data downloads, the web app and these docs.

## Commands

```text
bombaychitra serve    [flags]   run the HTTP server
bombaychitra export   [flags]   export data as GeoJSON, NDJSON or CSV
bombaychitra info     [flags]   print dataset information
bombaychitra download [flags]   download the published dataset
bombaychitra route    [flags]   print a route and its directions
bombaychitra version            print the version
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
| `--metrics` | `BC_METRICS` | `true` | Expose Prometheus metrics at `/metrics` |
| `--metrics-addr` | `BC_METRICS_ADDR` | main address | Serve `/metrics` on a separate address instead, e.g. `127.0.0.1:9090` |

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
| `/api/v1/health` | Readiness and status: database, tiles and routing checks, dataset age; `503` when the database is unreadable |
| `/metrics` | Prometheus metrics (see [Monitoring](#monitoring)) |

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

### Monitoring

`/healthz` answers as long as the process runs — use it as the liveness probe. `/api/v1/health` checks the
components and is the better readiness probe:

```json
{
  "status": "ok",
  "version": "v0.1.0",
  "uptime_s": 86400,
  "dataset": { "built_at": "2026-10-11T14:33:58Z", "source_timestamp": "2026-10-10T20:21:02Z", "schema_version": "2", "age_days": 1.2 },
  "checks": {
    "database": { "status": "ok", "latency_ms": 0.08 },
    "tiles": { "status": "ok", "detail": { "tiles": 31744, "minzoom": 0, "maxzoom": 15 } },
    "routing": { "status": "ok", "detail": { "vertices": 170184, "segments": 211887, "edges": 376888 } }
  }
}
```

The routing check reports `not_loaded` until the first route request (the graph is loaded lazily), which is not an
error.

`/metrics` serves [Prometheus](https://prometheus.io) metrics in the text format, without any third-party
library:

| Metric | Type | Labels |
|---|---|---|
| `bombaychitra_http_requests_total` | counter | `route` (the matched pattern, e.g. `/api/v1/places/{id}`), `method`, `code` |
| `bombaychitra_http_request_duration_seconds` | histogram | `route` |
| `bombaychitra_http_response_bytes_total` | counter | `route` |
| `bombaychitra_http_rate_limited_total` | counter | `route` |
| `bombaychitra_http_in_flight_requests` | gauge | |
| `bombaychitra_route_searches_total` | counter | `mode`, `outcome` (`ok`, `no_road`, `no_route`, `unavailable`, `error`) |
| `bombaychitra_route_search_duration_seconds` | histogram | `mode` |
| `bombaychitra_route_settled_vertices` | histogram | `mode` |
| `bombaychitra_routing_graph_vertices` | gauge | |
| `bombaychitra_rate_limit_clients` | gauge | |
| `bombaychitra_build_info` | gauge | `version`, `dataset`, `schema` |
| `bombaychitra_uptime_seconds`, `bombaychitra_dataset_age_seconds` | gauge | |
| `go_goroutines`, `go_memstats_heap_alloc_bytes` | gauge | |

A scrape configuration:

```yaml
scrape_configs:
  - job_name: bombaychitra
    scrape_interval: 30s
    static_configs:
      - targets: ['localhost:8080']
```

On a public server keep the metrics private: either serve them on a separate address that only your monitoring
can reach (`--metrics-addr 127.0.0.1:9090`) or turn them off (`--metrics=false`). Useful alerts are
`bombaychitra_dataset_age_seconds > 14 * 86400` (the weekly data refresh stopped) and a rising share of `5xx`
codes in `bombaychitra_http_requests_total`.

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

## `route`

Routes straight from the database, without a server — useful to check the routing graph after a data build:

```text
$ bombaychitra route --from 19.1197,72.8464 --to 19.1209,72.8481 --mode foot
0.5 km, 6 min by foot
  1. Head north along the foot over-bridge                            29 m
  2. Turn right and cross the foot over-bridge                       263 m
  3. Turn sharp right                                                 68 m
  4. Turn right onto Andheri Station Road                            109 m
  5. Arrive at your destination
```

| Flag | Description |
|---|---|
| `--from`, `--to` | Ends as `lat,lon` |
| `--mode` | `car` (default), `two_wheeler` or `foot` |
| `--shortest` | Minimise distance instead of time |
| `--avoid-tolls` | Keep off toll roads |
| `--json` | Print the full result as JSON |
| `--data` | Data directory (`BC_DATA`, default `data/build`) |

## `download`

Fetches `bombaychitra.pmtiles` and `bombaychitra.db` from a GitHub release, so a server can run without building
the data:

```text
bombaychitra download                          # data-latest → data/build
bombaychitra download --release v0.1.0         # the dataset a version was released with
bombaychitra download --data /srv/bombaychitra --force
```

| Flag | Env | Default | Description |
|---|---|---|---|
| `--data` | `BC_DATA` | `data/build` | Directory to download into |
| `--release` | `BC_DATA_RELEASE` | `data-latest` | Release tag |
| `--repo` | `BC_DATA_REPO` | `AmmaarBakshi/BombayChitra` | Repository publishing the data (for forks) |
| `--url` | | | Download from this base URL instead (a mirror or your own bucket) |
| `--force` | | `false` | Download even when the local files match the published checksums |

Files are checked against the release's `SHA256SUMS`, downloaded beside the destination (`*.part`) and renamed into
place together only when both are intact. Files that already match are skipped, so running it on a schedule costs
one small request when nothing changed. Restart the server afterwards to serve new data.
