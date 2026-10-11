# Configuration

Two JSON files in `config/` define *what* BombayChitra maps. They are read by the pipeline, written into the
database, and compiled into the web app, so a change flows through the whole stack on the next build.

## `config/region.json`

| Field | Description |
|---|---|
| `id`, `name`, `description` | Identity of the dataset (shown in the app and `/api/v1/meta`). |
| `bounds` | `[minLon, minLat, maxLon, maxLat]` — tiles and records are produced inside this box. |
| `center`, `zoom` | Initial map view. |
| `maxzoom` | Highest tile zoom (10–16; default 15). Clients overzoom beyond it. |
| `source.url` | OpenStreetMap extract to download (any `.osm.pbf`, e.g. from Geofabrik). |
| `source.file` | File name under `data/sources/`. |
| `overture.release` | Overture Maps release to download (`latest`, or e.g. `2026-09-23.1`). |
| `overture.places.file`, `overture.places.minConfidence` | GeoParquet file under `data/sources/` for Overture places, and the confidence (0–1) a place needs to be added; default 0.6. |
| `overture.buildings.file` | GeoParquet file for Overture buildings (only machine-learned footprints are added). Remove the `overture` section for an OpenStreetMap-only build. |
| `areas[]` | Named areas: `id`, `name`, `short` (chip label), `kind`, `description`, and `relations` — OSM relation ids whose polygons are unioned. |
| `transit.modes` | Route relation types to treat as lines (`subway`, `light_rail`, `monorail`, `tram`, `train`). |
| `transit.trainNetworks` | `route=train` relations are included only for these networks (suburban rail, not long-distance trains). |
| `transit.colourAliases` | Hex values for named colours (`"blue"`, `"aqua"`…) used in OSM `colour` tags. |
| `transit.modeColours` | Fallback colour per mode when a line has no colour tag. |
| `transit.routeOverrides[]` | Corrections for route relations whose tags match `match`: `ref`, `label`, `network`, `colour`, `operator`. The first matching entry applies. |
| `transit.trackLines[]` | Lines assembled from track ways whose tags match `match`, for lines without a route relation: `mode`, `network`, `ref`, `label`, `colour`, `operator`. |

`match` objects use the same rule syntax as `config/categories.json`: a value, a list of values, `"*"`, `"!value"` or `"!*"`.

### Transit lines without route relations

OpenStreetMap describes most lines with route relations, which give the line's name, colour and ordered stops. Lines
that have none yet — Mumbai's monorail and most metro lines under construction — would otherwise appear only as
anonymous track. `trackLines` turns them into proper lines:

```json
"trackLines": [
  { "match": { "railway": "monorail" }, "mode": "monorail", "network": "Mumbai Monorail", "label": "Monorail" },
  { "match": { "name": "Line 7A" }, "mode": "subway", "network": "Mumbai Metro", "ref": "7A", "label": "Line 7A", "colour": "red" }
]
```

Every non-service track way (`railway=subway|light_rail|monorail|…`, or under construction or proposed) whose tags
match is collected, whether or not it already belongs to a route relation. The pipeline then:

- measures the line along a single-track spine, so both tracks of a double line are counted once;
- matches metro, monorail or railway stations within 120 m of the track as its stops, ordered along the line;
- sets the status from the track's lifecycle: `construction` when no track is open, `proposed` when all of it is
  proposed;
- identifies the line by its longest way (`osm_type = way` in the database and API).

`routeOverrides` fixes route relations without editing OpenStreetMap, for example to use the official name of a line:

```json
{ "match": { "route": "subway", "network": "Mumbai Metro", "ref": "2" }, "ref": "2A", "label": "Line 2A" }
```

Both lists are curated for the region. When OpenStreetMap gains a route relation for a line, remove its
`trackLines` entry.

### Defining an area

Areas reference OpenStreetMap relations, so they follow official boundaries. MaKaBo, for example, is the union of the
three BMC wards that cover Malad, Kandivali and Borivali:

```json
{
  "id": "makabo",
  "name": "MaKaBo",
  "short": "MaKaBo",
  "kind": "zone",
  "description": "Malad, Kandivali and Borivali — BMC wards P/N, R/S and R/C.",
  "relations": [7885384, 7885376, 7885385]
}
```

Find relation ids on [openstreetmap.org](https://www.openstreetmap.org) (search for the area and open the
boundary relation) or with an [Overpass](https://overpass-turbo.eu/) query such as
`rel["boundary"="administrative"]["name"~"Ward"](18.9,72.8,19.3,73.0); out tags;`.

## `config/categories.json`

The taxonomy of points of interest: **groups** contain **categories**, and each category has OSM tag **rules**.

```json
{
  "id": "health",
  "label": "Health",
  "colour": "#E5484D",
  "categories": [
    {
      "id": "hospital",
      "label": "Hospitals",
      "icon": "hospital",
      "rank": 1,
      "minzoom": 12,
      "match": [{ "amenity": "hospital" }, { "healthcare": "hospital" }]
    }
  ]
}
```

| Field | Description |
|---|---|
| `id` | Stable identifier used in tiles, the database and the API. |
| `label` | Display name (plural). |
| `icon` | A [Maki](https://labs.mapbox.com/maki-icons/) icon name (or `atm`, a built-in custom icon). |
| `rank` | 1 = most important (stations, hospitals) … 5 = least (small shops). Controls label priority and search ranking. |
| `minzoom` | Zoom at which the category appears in tiles (large areas appear 1–2 zooms earlier). |
| `match` | A list of rules; a feature matches if **any** rule matches. |
| `overture` | Overture Maps categories that belong here, at any level of Overture's hierarchy, e.g. `["dental_clinic"]` for `dentist`. A place takes the category of its most specific mapped Overture category; unmapped places are not added. |

### Rule syntax

A rule is an object of tag conditions that must **all** match:

| Value | Meaning | Example |
|---|---|---|
| `"x"` | tag equals `x` | `{ "amenity": "bank" }` |
| `["x", "y"]` | tag is one of the values | `{ "amenity": ["bar", "pub"] }` |
| `"*"` | tag present and not `no` | `{ "building": "*", "name": "*" }` |
| `"!x"` / `["!x", "!y"]` | tag absent or different | `{ "leisure": "swimming_pool", "access": ["!private", "!no"] }` |
| `"!*"` | tag absent | `{ "building": "school", "name": "*", "amenity": "!*" }` |

Institutions in India are often mapped only as a building (`building=school`, `building=college`,
`building=hospital`), so those categories also match named buildings that carry no `amenity` tag of their own.

**Order matters:** categories are tried from top to bottom and the first match wins. Put specific categories before
general ones — `metro_station` before `railway_station`, and catch-alls such as `shop` last in their group.

## Mapping another city

1. Pick an extract that covers the city (e.g. from [Geofabrik](https://download.geofabrik.de/)) and set
   `source.url` and `source.file`.
2. Set `bounds`, `center` and `zoom`.
3. Replace `areas` with relations for the new city.
4. Adjust `transit.trainNetworks` and colour aliases, and replace `transit.routeOverrides` and
   `transit.trackLines`.
5. Run `scripts/build-data.sh`.

Nothing else is region-specific. The coastline-based ocean, transit colours, search and statistics all derive from
the data.
