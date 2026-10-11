# server

The BombayChitra server: one static Go binary that serves the REST API, vector tiles, styles, data downloads, the web
app and the docs.

```bash
go build -o ../bin/bombaychitra ./cmd/bombaychitra
../bin/bombaychitra serve --data ../data/build      # http://localhost:8080
go test ./...                                       # integration tests run against ../data/build when present
```

| Package | Responsibility |
|---|---|
| `cmd/bombaychitra` | CLI: `serve`, `export`, `info`, `version` |
| `internal/config` | Flags and `BC_*` environment variables |
| `internal/pmtiles` | PMTiles v3 reader (Hilbert tile ids, cached directories, one read per tile) |
| `internal/store` | Read-only SQLite queries: places, search, reverse geocoding, streets, bridges, buildings, transit, areas |
| `internal/httpapi` | Routing, handlers, caching, compression, CORS, static files |
| `internal/export` | GeoJSON, NDJSON and CSV exports |
| `internal/geo` | Distances, bounding boxes, snapping, point-in-polygon |
| `internal/webui` | Embedded web build (`npm run build` in `../web` fills `dist/`) |

See [docs/server.md](../docs/server.md) and [docs/api.md](../docs/api.md).
