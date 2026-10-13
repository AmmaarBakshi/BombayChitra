# Data & coverage

## Sources

| Source | What it provides | Licence |
|---|---|---|
| [OpenStreetMap](https://www.openstreetmap.org), via the [Geofabrik](https://download.geofabrik.de/asia/india.html) *India — Western Zone* extract (updated daily) | Everything: roads, bridges, buildings, transit, boundaries, water, land use and ~45,000 places | ODbL 1.0 |
| [Overture Maps](https://overturemaps.org) places | ~109,000 shops, clinics, schools, offices, restaurants and services that OpenStreetMap does not have yet | CDLA-Permissive-2.0 |
| [Overture Maps](https://overturemaps.org) buildings | ~1.8 million machine-learned footprints (Google Open Buildings, Microsoft ML Buildings) where OpenStreetMap has none | ODbL 1.0 |

OpenStreetMap is the base and always wins. Overture fills the gaps that are largest in fast-growing areas such as
Kharghar, Kalamboli, Taloja, Panvel, Ulwe and Vasai-Virar:

- **Places** are kept when Overture's confidence is at least 0.6 and their category maps to the taxonomy, and dropped
  when OpenStreetMap already has the same category within 25 m or a similarly named place of the same group within 120
  m; repeated Overture records of the same place are merged (8,005 dropped in all). Each is marked `source: overture`
  in the API and on its card, with a link to add it to OpenStreetMap.
- **Buildings** come only from Overture's machine-learned sources; Overture has already removed those that overlap
  OpenStreetMap buildings.

The build records the OpenStreetMap extract timestamp and the Overture release (`/api/v1/meta`). Building without
Python, or with `SKIP_OVERTURE=1`, produces an OpenStreetMap-only dataset.

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
| `panvel-taluka` | Panvel Taluka — Kharghar, Kamothe, Kalamboli, Taloja, New Panvel | Sub-district relation | 590 km² |
| `uran` | Uran Taluka — Ulwe, Dronagiri, JNPA, Navi Mumbai airport | Sub-district relation | 204 km² |

Every place, street and bridge in the database is tagged with the areas that contain it, so you can filter any API
query with `area=makabo` and so on.

### BMC wards

The Brihanmumbai Municipal Corporation's 24 administrative wards are areas too (`kind: ward`), from their
OpenStreetMap boundaries — so hospitals, schools, buildings or bridges can be counted ward by ward
(`/api/v1/stats?area=ward-kw`) and the map can highlight one:

| Id | Ward | Covers | Id | Ward | Covers |
|---|---|---|---|---|---|
| `ward-a` | A | Colaba, Fort, Churchgate | `ward-kw` | K/W | Andheri West, Juhu, Versova |
| `ward-b` | B | Dongri, Masjid Bunder | `ward-l` | L | Kurla, Chandivali, Saki Naka |
| `ward-c` | C | Kalbadevi, Marine Lines | `ward-me` | M/E | Govandi, Mankhurd, Trombay |
| `ward-d` | D | Malabar Hill, Grant Road, Tardeo | `ward-mw` | M/W | Chembur, Tilak Nagar |
| `ward-e` | E | Byculla, Mazgaon | `ward-n` | N | Ghatkopar, Vikhroli |
| `ward-fn` | F/N | Matunga, Sion, Wadala | `ward-pn` | P/N | Malad |
| `ward-fs` | F/S | Parel, Sewri, Lalbaug | `ward-ps` | P/S | Goregaon, Aarey |
| `ward-gn` | G/N | Dadar, Mahim, Dharavi | `ward-rc` | R/C | Borivali, Gorai |
| `ward-gs` | G/S | Worli, Prabhadevi, Lower Parel | `ward-rn` | R/N | Dahisar |
| `ward-he` | H/E | Bandra East, Santacruz East, BKC | `ward-rs` | R/S | Kandivali, Charkop |
| `ward-hw` | H/W | Bandra West, Khar West, Santacruz West | `ward-s` | S | Bhandup, Powai, Kanjurmarg |
| `ward-ke` | K/E | Andheri East, Vile Parle East, airport | `ward-t` | T | Mulund |

### PIN codes

OpenStreetMap has no postal boundaries for the region, but about 112,000 places carry a PIN in their address. The
server turns every PIN with at least eight such places into an area (`/api/v1/areas?kind=pincode`): its median
position, an outline around its places (outliers — such as a branch carrying its head office's PIN — dropped), its
size, the localities it covers and place counts by category. PIN areas can be used as `area=400050` in place queries
and statistics, are found by search, and reverse geocoding reports the PIN a point falls in.

They are approximations of where the addressed places are, not the postal department's delivery boundaries, which
are not published as open data; neighbouring outlines can overlap a little.

### What is mapped (current build)

| Content | Count |
|---|---|
| Points of interest | 154,607 in 89 categories (45,559 from OpenStreetMap, 109,048 from Overture Maps) |
| Building footprints | 2,096,690 (273,632 from OpenStreetMap, 28,094 with height or floor data) |
| Road network for routing | 170,184 junctions, 211,887 road segments a car can use |
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
| `bombaychitra.pmtiles` | [PMTiles v3](https://github.com/protomaps/PMTiles) vector tiles, z0–z15 | ~96 MB | MapLibre, the `pmtiles` library, QGIS, GDAL |
| `bombaychitra.db` | SQLite 3 with R*Tree, FTS5 and the routing graph | ~209 MB | Any SQLite driver — see [Database](database.md) |

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

## Quality checks

OpenStreetMap is edited by anyone, and Overture Maps changes its schema between releases, so every weekly build is
checked before it is published. `scripts/check-data.py` compares the new database with the current `data-latest`
release:

- rows per table, places per category and per source (OpenStreetMap or Overture Maps);
- bridge kilometres and every building footprint;
- transit lines and stops, and the length of Metro Line 3 (a fully mapped line, so a good canary);
- the size of the routing graph.

If anything with at least 50 items shrinks by more than 5 %, the workflow fails and the old release stays online.
The report (also shown in the run's summary) lists every drop, so a maintainer can find the cause — usually a
vandalised relation or a failed download — or, for a genuine change, re-run the workflow with `allow_regressions`.
Run it locally against any two builds:

```bash
python3 scripts/check-data.py data/build/bombaychitra.db --baseline old/bombaychitra.db
```

## Licence and attribution

The data is © OpenStreetMap contributors and available under the
[Open Database License (ODbL) 1.0](https://opendatacommons.org/licenses/odbl/); places and buildings added from
Overture Maps are © Overture Maps Foundation (see [Licence](license.md)). If you use it you must:

- **attribute** it, e.g. "© OpenStreetMap contributors" with a link to https://www.openstreetmap.org/copyright;
- **share alike**: if you distribute a modified database, distribute it under the ODbL.

Maps (images, tiles rendered for display) only need attribution. See [Licence](license.md).
