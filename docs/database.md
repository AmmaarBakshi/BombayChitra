# Database

`bombaychitra.db` is a SQLite 3 database. It is the most portable form of the dataset: every language has a SQLite
driver, the file works offline, and it carries its own spatial and full-text indexes. The server reads it; you can
also query it directly.

## Conventions

- **Ids** of places, roads, bridges, buildings and localities are `osm_id × 10 + type` (`1` node, `2` way,
  `3` relation), the same as the vector tile feature ids. `osm_type` and `osm_id` are stored separately too.
  Places from Overture Maps have `osm_type = 'overture'` and ids ending in `4` (a hash of the Overture id, which is
  in `tags ->> '$."overture:id"'` with the confidence, category and original sources).
- **Coordinates** are WGS84 `lon`/`lat` (6 decimals ≈ 0.1 m).
- **Geometries** are GeoJSON text (`geometry` columns), ready to return or parse.
- **Tags** are the complete OpenStreetMap tags as a JSON object (`tags` columns).
- The schema version is in `meta.schema_version` (currently `2`).

## Tables

| Table | Rows | Contents |
|---|---|---|
| `meta` | key/value | Region, bounds, source and build timestamps, counts |
| `category_groups`, `categories` | 13 / 89 | The taxonomy with labels, icons, ranks and place counts |
| `areas` | 11 | Configured areas: name, kind, size, bbox, centre, simplified GeoJSON boundary |
| `area_members` | | Which areas contain each place, street, bridge and locality (`kind`, `member_id`, `area_id`) |
| `area_stats` | | Per-area statistics; `area_id = '*'` is the whole dataset |
| `places` | 154k | Points of interest with address, phone, website, hours, operator, brand, cuisine, accessibility, station lines (`extra`) and all tags; 109k from Overture Maps (`osm_type = 'overture'`) |
| `routing` | 12 | The car road network as compressed arrays — see [Routing graph](#routing-graph) |
| `localities` | 2k | Suburbs, neighbourhoods, villages with population |
| `streets` | 6k | Named roads merged from connected ways: class, length, bbox, centre, nearest locality |
| `roads` | 20k | Named road segments: lanes, speed limit, surface, one-way, bridge/tunnel flags, GeoJSON geometry |
| `bridges` | 6k | Bridge ways and outlines: kind, structure, length, GeoJSON geometry, tags |
| `named_bridges` | 1k | Named bridges: span (`length_m`), all decks (`total_length_m`), segment count |
| `buildings` | 76k | Buildings with name/height/levels/address/type: floors, height, footprint area, address |
| `transit_lines` | 54 | Routes: line key and label, mode, network, colour, status, from/to, length, ordered `stops` (JSON) and geometry |
| `search` | FTS5 | Unified full-text index over areas, localities, lines, places, streets and named bridges |

In `transit_lines`, `id` and `osm_type` identify the OSM source: a route relation, or, for a line assembled from
track (see [Configuration](configuration.md#transit-lines-without-route-relations)), its longest way. A line has one
row per route relation, usually one per direction, and every row of a line shares its `line_key`. `state` is
`construction` or `proposed` for lines not yet in service, and their key ends with the state, e.g.
`Mumbai Metro:9:construction` for the extension under construction next to the open `Mumbai Metro:9`. `length_m` is
the route length with parallel tracks counted once.

Spatial indexes: `places_rtree`, `roads_rtree`, `bridges_rtree`, `buildings_rtree` (SQLite R*Tree with
`min_lon`, `max_lon`, `min_lat`, `max_lat`).

## Example queries

Hospitals within a bounding box:

```sql
SELECT p.name, p.lat, p.lon, p.phone
FROM places_rtree r JOIN places p ON p.id = r.id
WHERE r.min_lon >= 72.82 AND r.max_lon <= 72.86
  AND r.min_lat >= 19.04 AND r.max_lat <= 19.08
  AND p.category = 'hospital';
```

Every college in MaKaBo:

```sql
SELECT p.name, p.locality
FROM places p JOIN area_members a ON a.member_id = p.id AND a.kind = 'place'
WHERE a.area_id = 'makabo' AND p.category = 'college' AND p.name IS NOT NULL
ORDER BY p.name;
```

Full-text search with prefix matching (what the search box does):

```sql
SELECT kind, ref, name, detail
FROM search
WHERE search MATCH '"lilavati" "hosp"*'
ORDER BY bm25(search, 10.0, 4.0, 1.0) * (1 - 0.15 * weight)  -- weight < 0 boosts important entries
LIMIT 10;
```

The ten longest named road bridges:

```sql
SELECT name, ROUND(length_m / 1000.0, 2) AS km, locality
FROM named_bridges WHERE kind = 'road'
ORDER BY length_m DESC LIMIT 10;
```

Metro stations with the lines serving them:

```sql
SELECT name, json_extract(extra, '$.lines') AS lines
FROM places WHERE category = 'metro_station' AND extra IS NOT NULL;
```

Stops of a line, in order:

```sql
SELECT t.label, s.value ->> '$.name' AS stop
FROM transit_lines t, json_each(t.stops) s
WHERE t.line_key = 'Mumbai Metro:1' AND t.from_name = 'Versova';
```

Tallest buildings per area:

```sql
SELECT a.name, s.value AS tallest_m
FROM area_stats s JOIN areas a ON a.id = s.area_id
WHERE s.key = 'building_max_height_m'
ORDER BY tallest_m DESC;
```

## Using it from code

```python
import sqlite3
db = sqlite3.connect("file:bombaychitra.db?mode=ro", uri=True)
for name, phone in db.execute("SELECT name, phone FROM places WHERE category = 'hospital' AND phone IS NOT NULL LIMIT 5"):
    print(name, phone)
```

More languages are in [`examples/`](../examples/README.md) and on the [Integrations](integrations.md) page.

## Routing graph

The `routing` table holds the car road network behind `/api/v1/route` as named blobs: little-endian arrays,
each gzip-compressed (about 10 MB in total). Vertices are the points where drivable ways meet or end; a segment is
the road between two vertices with its full shape.

| Blob | Type | Content |
|---|---|---|
| `nodes` | int32 × 2 per vertex | lon, lat × 10⁷ |
| `offsets` | int32 × (vertices + 1) | Edges leaving vertex *v* are `offsets[v] … offsets[v+1] − 1` |
| `edge_to` | int32 per edge | Destination vertex |
| `edge_seg` | int32 per edge | Segment *s*, or `~s` (bitwise not) when the edge runs against the segment's shape |
| `edge_time` | float32 per edge | Travel time in seconds |
| `seg_len` | float32 per segment | Length in metres |
| `seg_shape` | int32 × (segments + 1) | Offsets into `shape`, in points |
| `shape` | int32 × 2 per point | Segment shapes: first point absolute (lon, lat × 10⁷), the rest as deltas |
| `seg_class` | uint8 per segment | 1 motorway, 2 trunk, 3 primary, 4 secondary, 5 tertiary, 6 minor, 7 service |
| `seg_dir` | uint8 per segment | Bit 1: drivable along the shape; bit 2: against it |
| `seg_name` | int32 per segment | Index into `names`, or −1 |
| `names` | UTF-8 text | Road names, one per line |

`meta` records `routing_version`, `routing_profile` (`car`) and the vertex, edge and segment counts. Travel times
use typical urban speeds per road class (see `DRIVE_SPEED` in `pipeline/…/layers/Roads.java`).

## Notes

- Open the file read-only (`mode=ro`); the server additionally sets `immutable=1`.
- R*Tree coordinates are 32-bit floats, rounded outwards — fine for filtering; use `lon`/`lat` for exact distances.
- FTS5 uses the `unicode61` tokenizer with diacritics removed and prefix indexes for 1–3 characters.
