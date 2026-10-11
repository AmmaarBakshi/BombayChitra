# Data & coverage

## Source

All data comes from [OpenStreetMap](https://www.openstreetmap.org), the collaborative map of the world, via the
[Geofabrik](https://download.geofabrik.de/asia/india.html) *India — Western Zone* extract, which is updated daily. The
build records the extract's timestamp, shown in the app's status bar and at `/api/v1/meta`.

## Coverage

The dataset covers the bounding box `72.70, 18.80 → 73.85, 19.82` (longitude, latitude), which includes the whole
of Mumbai, Thane district and Navi Mumbai and most of the Mumbai Metropolitan Region.

### Areas

Areas are built from OpenStreetMap administrative boundaries, so they match official limits:

| Id | Area | Defined by | Size |
|---|---|---|---|
| `mmr` | Mumbai Metropolitan Region | MMR relation (within the coverage box) | 4,333 km² |
| `mumbai-city` | Mumbai City district | District relation | 72 km² |
| `mumbai-suburban` | Mumbai Suburban district | District relation | 406 km² |
| `makabo` | **MaKaBo** — Malad, Kandivali, Borivali | BMC wards P/N, R/S and R/C | 115 km² |
| `thane-district` | Thane District | District relation | 4,254 km² |
| `thane` | Thane City | Thane Municipal Corporation | 142 km² |
| `navi-mumbai` | Navi Mumbai | Navi Mumbai Municipal Corporation | 106 km² |
| `mira-bhayandar` | Mira-Bhayandar | Municipal corporation | 90 km² |
| `kalyan-dombivli` | Kalyan-Dombivli | Municipal corporation | 117 km² |
| `vasai-virar` | Vasai-Virar | Municipal corporation | 367 km² |
| `panvel` | Panvel | Municipal corporation | 108 km² |

Every place, street and bridge in the database is tagged with the areas that contain it, so you can filter any API
query with `area=makabo` and so on.

### What is mapped (current build)

| Content | Count |
|---|---|
| Points of interest | 45,559 in 89 categories |
| Building footprints | 273,632 (28,094 with height or floor data) |
| Buildings with detail records | 76,039 |
| Named streets | 6,138 (from 20,532 named road segments) |
| Bridges | 6,048 bridge segments, 1,590 km of deck; 1,100 named bridges |
| Transit lines | 26 lines from 54 routes: 44 route relations and 10 lines assembled from track (the monorail and metro lines under construction or proposed) |
| Localities | 2,047 suburbs, neighbourhoods and villages |

OpenStreetMap is edited by volunteers, so coverage varies: central Mumbai is mapped in great detail, outer areas less
so. Everything here improves as people add to OpenStreetMap — every detail card in the app links to the feature on
openstreetmap.org.

## Downloads

| File | Format | Size | Read it with |
|---|---|---|---|
| `bombaychitra.pmtiles` | [PMTiles v3](https://github.com/protomaps/PMTiles) vector tiles, z0–z15 | ~43 MB | MapLibre, the `pmtiles` library, QGIS, GDAL |
| `bombaychitra.db` | SQLite 3 with R*Tree and FTS5 | ~64 MB | Any SQLite driver — see [Database](database.md) |

Get them from:

- a running server: `/data/bombaychitra.pmtiles` and `/data/bombaychitra.db` (range requests supported);
- the **Data (latest)** GitHub release, rebuilt weekly;
- your own build: `scripts/build-data.sh`.

You can also export subsets as GeoJSON, newline-delimited GeoJSON or CSV:

```bash
bombaychitra export --kind places --category hospital,clinic --area makabo --format csv -o makabo-health.csv
bombaychitra export --kind lines --format geojson -o transit.geojson
bombaychitra export --kind bridges --format geojson -o bridges.geojson
```

## Updating

Rebuild with `scripts/build-data.sh --refresh_sources` to fetch the latest extract, then restart the server. The
database is opened read-only and treated as immutable, so replace both files together and restart.

## Licence and attribution

The data is © OpenStreetMap contributors and available under the
[Open Database License (ODbL) 1.0](https://opendatacommons.org/licenses/odbl/). If you use it you must:

- **attribute** it, e.g. "© OpenStreetMap contributors" with a link to https://www.openstreetmap.org/copyright;
- **share alike**: if you distribute a modified database, distribute it under the ODbL.

Maps (images, tiles rendered for display) only need attribution. See [Licence](license.md).
