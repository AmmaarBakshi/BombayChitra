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
| `areas[]` | Named areas: `id`, `name`, `short` (chip label), `kind`, `description`, and `relations` — OSM relation ids whose polygons are unioned. |
| `transit.modes` | Route relation types to treat as lines (`subway`, `light_rail`, `monorail`, `tram`, `train`). |
| `transit.trainNetworks` | `route=train` relations are included only for these networks (suburban rail, not long-distance trains). |
| `transit.colourAliases` | Hex values for named colours (`"blue"`, `"aqua"`…) used in OSM `colour` tags. |
| `transit.modeColours` | Fallback colour per mode when a line has no colour tag. |

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

### Rule syntax

A rule is an object of tag conditions that must **all** match:

| Value | Meaning | Example |
|---|---|---|
| `"x"` | tag equals `x` | `{ "amenity": "bank" }` |
| `["x", "y"]` | tag is one of the values | `{ "amenity": ["bar", "pub"] }` |
| `"*"` | tag present and not `no` | `{ "building": "*", "name": "*" }` |
| `"!x"` / `["!x", "!y"]` | tag absent or different | `{ "leisure": "swimming_pool", "access": ["!private", "!no"] }` |

**Order matters:** categories are tried from top to bottom and the first match wins. Put specific categories before
general ones — `metro_station` before `railway_station`, and catch-alls such as `shop` last in their group.

## Mapping another city

1. Pick an extract that covers the city (e.g. from [Geofabrik](https://download.geofabrik.de/)) and set
   `source.url` and `source.file`.
2. Set `bounds`, `center` and `zoom`.
3. Replace `areas` with relations for the new city.
4. Adjust `transit.trainNetworks` and colour aliases.
5. Run `scripts/build-data.sh`.

Nothing else is region-specific. The coastline-based ocean, transit colours, search and statistics all derive from
the data.
