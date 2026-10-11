# BombayChitra

**BombayChitra** (बॉम्बे चित्र, "picture of Bombay") is an open, detailed and fast map of the **Mumbai
Metropolitan Region**: Mumbai City, Mumbai Suburban, Thane district, Navi Mumbai, **MaKaBo** (Malad · Kandivali ·
Borivali) and the wider MMR. It is built entirely from [OpenStreetMap](https://www.openstreetmap.org) and is designed
to be used from **any programming language, platform or product**.

> BombayChitra is a complete stack: a data pipeline that turns OpenStreetMap into vector tiles and a queryable
> database, a single-binary server with a REST API, and a web map with light and dark themes.

## What's inside

| | |
|---|---|
| **Roads** | Every road, street, flyover, service road and footpath, with names, route numbers, lanes, speed limits, surface, one-way arrows, bridges and tunnels. Named roads are searchable and power reverse geocoding ("pin point"). |
| **Bridges** | A dedicated bridge layer: sea links (Atal Setu, Bandra–Worli), freeways, flyovers, rail bridges, metro viaducts and foot over-bridges, each with its measured span. |
| **Buildings** | 270,000+ footprints with heights and floor counts for 3D, plus searchable records for named towers and housing societies. |
| **Transit** | Suburban railway (Western, Central, Harbour, Trans-Harbour, Uran…), all metro lines (including lines under construction) and monorail in their official colours, with ordered stations. |
| **Places** | ~45,000 points of interest in 89 categories: hospitals, clinics, schools, colleges, universities, banks, ATMs, railway/metro/monorail stations, bus depots and stops, grounds, sports clubs and gymkhanas, cafés, restaurants, hotels, temples, mosques, churches, police, fire stations and more. |
| **Areas** | Mumbai City, Mumbai Suburban, MaKaBo, Thane District, Thane, Navi Mumbai, Mira-Bhayandar, Kalyan-Dombivli, Vasai-Virar and Panvel — with boundaries, statistics and filters. |

## Use it

- **The map** — open the server in a browser: search, filter layers, explore transit, bridges and the tallest
  buildings in 3D, right-click anywhere for "What's here?", and switch between light and dark themes.
- **The REST API** — JSON/GeoJSON for search, reverse geocoding, places, roads, bridges, buildings and transit. See
  the [API guide](api.md) and the [API reference](/docs/api-reference/).
- **Vector tiles and styles** — use the tiles with MapLibre on the web, Android, iOS, Flutter or React Native, or in
  QGIS. See [Integrations](integrations.md) and [Map styles](styles.md).
- **The data files** — a single PMTiles archive and a SQLite database that any language can read directly. See
  [Data](data.md) and [Database](database.md).

## Design principles

1. **Open data, open code.** Code is MIT-licensed; data is © OpenStreetMap contributors under the ODbL.
2. **Fast by construction.** One pass over OpenStreetMap builds everything in about a minute. Tiles are served from
   a single file with one disk read each; the database is indexed for spatial and full-text queries.
3. **No lock-in.** Every output uses an open standard: Mapbox Vector Tiles, PMTiles, MapLibre styles, GeoJSON,
   SQLite and OpenAPI.
4. **Config, not code.** The region and the place taxonomy are JSON files, so the same stack can map another city.

Start with [Getting started](getting-started.md).
