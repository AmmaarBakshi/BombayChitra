<div align="center">

<img src="web/public/favicon.svg" width="72" height="72" alt="BombayChitra logo">

# BombayChitra

**An open, detailed and fast map of the Mumbai Metropolitan Region — usable from any language, platform or product.**

Mumbai City · Mumbai Suburban · Thane District · Navi Mumbai · Panvel · Vasai-Virar · MaKaBo (Malad–Kandivali–Borivali) · the wider MMR

[![CI](https://github.com/AmmaarBakshi/BombayChitra/actions/workflows/ci.yml/badge.svg)](https://github.com/AmmaarBakshi/BombayChitra/actions/workflows/ci.yml)
[![Code: MIT](https://img.shields.io/badge/code-MIT-blue.svg)](LICENSE)
[![Data: ODbL](https://img.shields.io/badge/data-ODbL%20%C2%A9%20OpenStreetMap-7ebc6f.svg)](https://www.openstreetmap.org/copyright)

[Getting started](docs/getting-started.md) · [Documentation](docs/index.md) · [API](docs/api.md) · [Integrations](docs/integrations.md) · [Examples](examples/README.md)

<img src="docs/images/light-overview.jpg" alt="BombayChitra light theme: Bandra, Kurla and the airport with metro and suburban lines" width="100%">

</div>

## Features

- **Every road, pin-pointed** — expressways to footpaths with names, route numbers, lanes, speed limits, surface and
  one-way arrows. Click anywhere for the nearest named road, locality and area ("What's here?").
- **Advanced bridge mapping** — a dedicated layer for all 6,000+ bridge segments: sea links, freeways, flyovers,
  rail bridges, metro viaducts and foot over-bridges, with a bridge explorer and measured spans (Atal Setu 18.2 km,
  Eastern Freeway 9.2 km, Bandra–Worli Sea Link 4.0 km…).
- **Advanced building data** — 2.1 million footprints (OpenStreetMap plus Google and Microsoft machine-learned
  outlines via Overture Maps), 3D extrusion from validated heights and floor counts, the tallest towers, footprint
  area and per-area statistics.
- **Distance by road** — driving distance, time and route between any two places (fastest or shortest), following
  one-way streets; from the toolbar, any place card ("Route here") or the API.
- **All of transport** — Western, Central, Harbour, Trans-Harbour and Uran suburban lines; every metro line in
  service (1, 2A, 2B, 3, 7, 9, Navi Mumbai Line 1), under construction (2B, 4, 4A, 5, 6, 7A, 9 extensions, Navi
  Mumbai Line 1 Stage 2) and proposed (10, 11, Thane Ring Metro), and the monorail, in their official colours with
  ordered stations; railway, metro and monorail stations, bus depots and 3,000+ bus stops.
- **Everything else** — ~154,000 places in 89 categories (OpenStreetMap plus 109,000 from Overture Maps, which fills
  in Navi Mumbai, Panvel, Taloja and Vasai-Virar): hospitals, clinics, pharmacies, schools, colleges,
  universities, banks, ATMs, grounds and maidans, sports clubs and gymkhanas, cafés, restaurants, hotels, temples,
  mosques, churches, gurudwaras, agiaries, police, fire stations, post offices, malls, markets, offices and more.
- **Areas** — Mumbai City, Mumbai Suburban, MaKaBo, Thane District, Thane, Navi Mumbai, Mira-Bhayandar,
  Kalyan-Dombivli, Vasai-Virar, Panvel, Panvel Taluka (Kharghar, Kalamboli, Taloja) and Uran (Ulwe, airport) from
  official boundaries, for filtering and statistics.
- **Light and dark themes**, a dashboard you can hide (`H`) for a full-screen map, unified search with autocomplete,
  shareable links, a measuring tool and a mobile layout.
- **Use it anywhere** — REST API with an OpenAPI 3.1 spec, MapLibre styles for web, Android, iOS, Flutter and
  React Native, vector tiles for QGIS, a SQLite database any language can read, and GeoJSON/CSV exports.

<table>
  <tr>
    <td><img src="docs/images/dark-fort.jpg" alt="Dark theme in Fort, South Mumbai"></td>
    <td><img src="docs/images/3d-buildings.jpg" alt="3D buildings in Lower Parel"></td>
  </tr>
  <tr>
    <td><img src="docs/images/transit.jpg" alt="Metro Line 3 with its 27 stations"></td>
    <td><img src="docs/images/bridges.jpg" alt="Bridge explorer showing Atal Setu"></td>
  </tr>
</table>

## Quick start

Requirements: Java 21+, Go 1.26+, Node.js 22+ and, optionally, Python 3 (for the Overture Maps places and buildings).

```bash
git clone https://github.com/AmmaarBakshi/BombayChitra.git && cd BombayChitra
./scripts/build-data.sh                          # OpenStreetMap + Overture → tiles + database (~3 min)
(cd web && npm ci && npm run build)              # web app + docs, embedded into the server
(cd server && go build -o ../bin/bombaychitra ./cmd/bombaychitra)
./bin/bombaychitra serve                         # → http://localhost:8080
```

No Java? Build only the server and fetch the published dataset instead (`./bin/bombaychitra download`), or grab a
[release](https://github.com/AmmaarBakshi/BombayChitra/releases) binary and run `bombaychitra download` then
`bombaychitra serve`. On Windows use `.\scripts\build-data.ps1`. With Docker: `docker compose run --rm pipeline && docker compose up -d server`.
See [Getting started](docs/getting-started.md) for details and prebuilt releases.

## Use it from your code

```bash
curl "http://localhost:8080/api/v1/search?q=lilavati%20hospital"
curl "http://localhost:8080/api/v1/nearby?lat=19.0178&lon=72.8478&radius=1000&category=atm,bank"
curl "http://localhost:8080/api/v1/reverse?lat=19.0545&lon=72.8405"
curl "http://localhost:8080/api/v1/places?area=makabo&category=college&format=geojson"
curl "http://localhost:8080/api/v1/transit/lines?mode=subway"
curl "http://localhost:8080/api/v1/route?from=19.1197,72.8464&to=19.0660,72.8680"   # distance by road
```

```js
// Any web page — the same styles work in MapLibre Native on Android and iOS
new maplibregl.Map({ container: 'map', style: 'http://localhost:8080/styles/dark.json' });
```

```python
import sqlite3  # or skip the server entirely and query the database
db = sqlite3.connect("file:data/build/bombaychitra.db?mode=ro", uri=True)
db.execute("SELECT name, phone FROM places WHERE category = 'hospital' LIMIT 5").fetchall()
```

Runnable examples for Python, JavaScript, Go, Java, C#, PHP, Ruby, Rust, R and SQL are in [`examples/`](examples/README.md).

## How it works

```text
OpenStreetMap ─┬► pipeline/ (Java + Planetiler, one pass) ──► bombaychitra.pmtiles  (vector tiles, 96 MB)
Overture Maps ─┘                                         └──► bombaychitra.db       (SQLite: R*Tree, FTS5, routing graph, 209 MB)
                                                                       │
                  server/ (Go, single static binary) ◄─────────────────┘
                  REST API · tiles · styles · web app · docs
                                │
                  web/ (TypeScript + MapLibre GL JS): map UI, light/dark styles, docs site
```

| Component | Stack | Why |
|---|---|---|
| [`pipeline/`](pipeline/README.md) | Java 25, [Planetiler](https://github.com/onthegomap/planetiler) | Fastest OSM → vector tile engine; builds tiles, database and routing graph in ~2½ min |
| [`server/`](server/README.md) | Go 1.26, pure-Go SQLite | One ~16 MB binary, no runtime dependencies, one disk read per tile |
| [`web/`](web/README.md) | TypeScript, MapLibre GL JS 6, Vite | 46 KB gzipped app code, GPU rendering, styles generated from one definition |
| `config/` | JSON | Region and place taxonomy shared by every component |
| `api/openapi.yaml` | OpenAPI 3.1 | Generate a typed client in any language |

Read more in [Architecture](docs/architecture.md).

## Documentation

The docs are served by the app at `/docs/` and are readable on GitHub:

| | |
|---|---|
| **Introduction** | [Overview](docs/index.md) · [Getting started](docs/getting-started.md) · [Architecture](docs/architecture.md) |
| **Data** | [Data & coverage](docs/data.md) · [Vector tiles](docs/tiles.md) · [Database](docs/database.md) · [Configuration](docs/configuration.md) |
| **Build & run** | [Pipeline](docs/pipeline.md) · [Server](docs/server.md) · [Deployment](docs/deployment.md) |
| **Use it** | [API guide](docs/api.md) · [API reference](api/openapi.yaml) · [Integrations](docs/integrations.md) · [Map styles](docs/styles.md) |
| **Project** | [Contributing](CONTRIBUTING.md) · [Licence](docs/license.md) |

## Contributing

Contributions are welcome — see [CONTRIBUTING.md](CONTRIBUTING.md). The easiest way to improve the map is to improve
[OpenStreetMap](https://www.openstreetmap.org): every detail card links to the feature so you can fix or add data.

## Licence

Code is [MIT](LICENSE). Map data © [OpenStreetMap contributors](https://www.openstreetmap.org/copyright), available
under the [ODbL](https://opendatacommons.org/licenses/odbl/); places and building footprints added from
[Overture Maps](https://overturemaps.org) © Overture Maps Foundation (CDLA-Permissive-2.0, ODbL). Fonts: Noto Sans
(OFL). Icons: Maki (CC0).
