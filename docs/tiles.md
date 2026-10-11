# Vector tiles

The tileset is a single PMTiles archive of [Mapbox Vector Tiles](https://github.com/mapbox/vector-tile-spec), zoom
levels 0–15 (overzoom beyond 15 for street-level detail). The server exposes it as:

- **TileJSON:** `/tiles/bombaychitra.json`
- **Tiles:** `/tiles/{z}/{x}/{y}.mvt` (gzip-encoded when the client accepts it; `204` for empty tiles)
- **Raw archive:** `/data/bombaychitra.pmtiles` (HTTP range requests)

Every feature that comes from an OpenStreetMap element has a numeric **feature id** of `osm_id × 10 + type`
(`1` node, `2` way, `3` relation). Use it with the detail endpoints of the API.

Attributes marked *z14+* appear only from that zoom so lower zooms stay small.

## Layers

### `water`

Polygons: the ocean (built from the coastline), lakes, reservoirs, rivers and docks.

| Attribute | Values |
|---|---|
| `kind` | `ocean`, `lake`, `reservoir`, `river`, `pond`, `basin`, `lagoon`, `dock` |

### `waterway`

Lines: rivers, creeks, canals, streams and drains (nallahs).

| Attribute | Values |
|---|---|
| `kind` | `river`, `canal`, `tidal_channel`, `stream`, `drain`, `ditch` |
| `name` | z12+ |
| `tunnel` | `1` for culverts and covered drains, z12+ |
| `intermittent` | `1` for seasonal streams, z13+ |

### `landuse`

Polygons of land cover and land use.

| Attribute | Values |
|---|---|
| `kind` | natural: `protected`, `forest`, `mangrove`, `wetland`, `salt_pan`, `grass`, `farmland`, `beach`, `rock`, `quarry`, `aquaculture` · urban: `residential`, `commercial`, `industrial`, `port`, `railway`, `military`, `construction`, `landfill`, `airport`, `parking`, `religious` · amenities: `park`, `golf`, `zoo`, `sports`, `playground`, `cemetery`, `education`, `hospital` |

### `buildings`

Polygons from z13. At z13 nearby buildings are merged into blocks.

| Attribute | Description |
|---|---|
| `height` | Height in metres (validated `height`/`building:height`, else floors × 3.2 m), z14+ |
| `min_height` | Base height for building parts, z14+ |
| `levels` | Floor count (`building:levels`), z14+ |
| `kind` | Building type when specific (`apartments`, `commercial`, `office`, `roof`, `part`…), z14+ |

### `roads`

Lines for the whole road and path network.

| Attribute | Description |
|---|---|
| `kind` | `motorway`, `trunk`, `primary`, `secondary`, `tertiary`, `minor`, `service`, `pedestrian`, `track`, `path`, `construction` |
| `link` | `1` for slip roads |
| `highway` | Raw OSM `highway` value, z13+ |
| `name` | z11+ for major roads, z13+ otherwise |
| `ref` | Route number (e.g. `NH48`), z9+ |
| `brunnel` | `bridge` or `tunnel`, z11+ |
| `layer` | Vertical order, z11+ |
| `oneway` | `1` (with the line) or `-1` (against), z14+ |
| `service`, `access` | z14+ |
| `lanes`, `maxspeed`, `surface`, `toll` | z15 |
| `construction` | Target class for roads under construction |

### `bridges`

Every bridge as its own layer for analysis and highlighting: lines for bridge ways and polygons for bridge outlines.

| Attribute | Description |
|---|---|
| `kind` | `road`, `rail`, `metro`, `foot`, `construction`, `aqueduct`, `outline` |
| `class` | What it carries (`primary`, `rail`, `subway`, `footway`…) |
| `length` | Length of this segment in metres |
| `name` | Bridge name (`bridge:name`, else `name`) |
| `structure` | `viaduct`, `aqueduct`, `boardwalk`, `beam`, `suspension`… z13+ |
| `layer`, `ref` | z12+ |

Long bridges appear early: ≥ 2 km from z8, ≥ 500 m from z10, ≥ 100 m from z12.

### `transit`

Railway, metro, monorail and tram lines plus platform outlines.

| Attribute | Description |
|---|---|
| `kind` | `rail`, `subway`, `light_rail`, `monorail`, `tram`, `funicular`, `construction`, `platform` |
| `colour` | Line colour (`#rrggbb`) from the OSM route relation, e.g. Western line yellow, Central red |
| `ref` | Line refs served by this track, comma-separated (e.g. `W`, `C,H`, `1`) |
| `network` | e.g. `Mumbai Metro`, `Mumbai Suburban Railway`, z11+ |
| `construction` | Target mode for lines under construction (e.g. `subway`) |
| `service`, `usage` | Track usage (`siding`, `yard`…; `main`, `branch`) |
| `name`, `brunnel`, `layer` | |

### `pois`

Points of interest, classified by `config/categories.json`. Polygons are reduced to a label point.

| Attribute | Description |
|---|---|
| `category` | Category id, e.g. `hospital`, `metro_station`, `cafe` — see `/api/v1/categories` |
| `group` | Group id: `transport`, `health`, `education`, `finance`, `food`, `lodging`, `sports`, `leisure`, `shopping`, `civic`, `worship`, `business`, `buildings` |
| `rank` | Importance 1 (stations, hospitals) – 5 (small shops) |
| `name` | Name (English where tagged) |

Below z15, each 24 px grid cell keeps at most 8 POIs, ordered by rank, to keep tiles small; z15 tiles contain
everything.

### `places`

Label points: `kind` = `city`, `town`, `suburb`, `quarter`, `neighbourhood`, `village`, `hamlet`, `locality`,
`island`, `islet` or `water` (named lakes), with `name`, `rank` and `population`.

### `boundaries`

Administrative boundary lines with `admin_level` (4 state, 5 district, 6 taluka, 8 municipal corporation,
9 BMC zone, 10 ward) and `maritime`.

### `areas`

The configured areas as dissolved polygons with `id`, `name` and `kind` — useful for highlighting a selected area.

### `aeroway`

Runways and taxiways (lines) and aprons and helipads (polygons) with `kind` and runway `ref`.

### `addresses`

House numbers (`housenumber`) at z15.

## Reading the archive directly

The PMTiles file can be used without the server:

```js
import * as maplibregl from 'maplibre-gl';
import { Protocol } from 'pmtiles';

const protocol = new Protocol();
maplibregl.addProtocol('pmtiles', protocol.tile);
// In the style's source: "url": "pmtiles://https://example.com/bombaychitra.pmtiles"
```

This is how to host the map on static storage (S3, R2, GitHub Pages) with no server at all. The REST API still needs
the server or direct use of the SQLite database.
