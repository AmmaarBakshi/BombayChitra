# Data pipeline

The pipeline turns an OpenStreetMap extract into `bombaychitra.pmtiles` and `bombaychitra.db` in a single pass. It
is written in Java on top of [Planetiler](https://github.com/onthegomap/planetiler) and lives in `pipeline/`.

## Running

```bash
./scripts/build-data.sh [options]        # Bash
.\scripts\build-data.ps1 [options]       # PowerShell
docker compose run --rm pipeline         # Docker
```

The scripts download Planetiler (checksum-verified), compile the pipeline with `javac`, and run it with `--download`,
which fetches the OSM extract on first use.

### Options

Pipeline options:

| Option | Default | Description |
|---|---|---|
| `--config_dir=` | `config` | Directory containing `region.json` and `categories.json` |
| `--data_dir=` | `data` | Working directory (`sources/`, `build/`) |
| `--tiles=` | `data/build/bombaychitra.pmtiles` | Tile output |
| `--database=` | `data/build/bombaychitra.db` | Database output |
| `--skip_database` | `false` | Only build tiles |

Useful Planetiler options:

| Option | Description |
|---|---|
| `--refresh_sources` | Re-download the extract if it changed upstream |
| `--threads=N` | Limit CPU threads |
| `--only_layers=roads,pois` / `--exclude_layers=addresses` | Build a subset of tile layers |
| `--maxzoom=14` | Override the maximum zoom |
| `--bounds=72.8,18.9,73.0,19.1` | Override the bounding box |

Memory is set with `JAVA_OPTS` (default `-Xmx4g`).

## How it works

```text
pass 1   nodes → node locations          relations → route / admin metadata, stop node ids
pass 2   nodes, ways, relations ─► layer handlers ─► tile features ─► sort ─► PMTiles archive
                                    └────────────► records (places, roads, bridges, buildings, transit, areas)
overture places (GeoParquet) ─► taxonomy + dedupe against OSM ─► pois  ──► records
overture buildings (GeoParquet) ─► machine-learned footprints ─► buildings
finish   coastline ─► ocean polygons                                   └────────────► SQLite writer + routing graph
```

The Overture extracts are read after OpenStreetMap, so every OpenStreetMap place is known when deduplicating.

### Layer handlers

Each tile layer is an independent class in `pipeline/src/bombaychitra/pipeline/layers/`:

| Handler | Layer | Also writes |
|---|---|---|
| `Water` | `water` (+ water labels in `places`) | — |
| `Waterways` | `waterway` | — |
| `Landuse` | `landuse` | — |
| `Buildings` | `buildings` | `buildings`, building statistics |
| `Roads` | `roads` | `roads` → `streets` |
| `Bridges` | `bridges` | `bridges` → `named_bridges` |
| `Transit` | `transit` | `transit_lines` (geometry + ordered stops) |
| `Pois` | `pois` | `places` |
| `Places` | `places` | `localities` |
| `Boundaries` | `boundaries` | — |
| `Areas` | `areas` | `areas`, `area_members` |
| `Aeroways` | `aeroway` | — |
| `Addresses` | `addresses` | — |

Each handler declares the tags it needs (`filter()`), so Planetiler only dispatches matching features to it.
Handlers can also post-process tiles: roads, waterways and transit lines are merged at low zooms, buildings are
merged into blocks at z13, and overlapping land-use polygons are dissolved.

### Notable techniques

- **Ocean from the coastline.** Instead of downloading the ~1 GB global water-polygon file, the `Coastline` class
  nodes the coastline ways with the region's bounding box, polygonises the result, and classifies each face as sea
  or land by the coastline's direction (OSM draws land on the left).
- **Validated building heights.** `height` is cross-checked against `building:levels`; implausible values
  (over 350 m, or under 1.8 m / over 8 m per floor) fall back to floors × 3.2 m.
- **Transit colours from route relations.** Track segments inherit the colour and refs of the route relations they
  belong to; stop nodes are indexed during the relation pass because Planetiler only tracks way membership.
- **Streets and named bridges.** OSM splits a road into many ways. Ways sharing a name are clustered by proximity
  into streets, so "Station Road" in Andheri and in Thane stay separate. For bridges, the span is the longest
  connected run, so dual carriageways are not double-counted.
- **Station lines.** Each railway, metro and monorail station gets the lines whose stops are within 350 m.
- **Area membership** is computed with prepared-geometry point-in-polygon tests against the configured areas.
- **Overture Maps enrichment.** `OverturePlaces` maps Overture's category hierarchy to the taxonomy, keeps places
  with confidence ≥ `minConfidence`, and skips any that duplicate an OpenStreetMap place (same category within
  25 m, or a similar name in the same group within 120 m) or another Overture record. `OvertureBuildings` adds only
  the machine-learned footprints. Both are skipped when their GeoParquet file is missing.
- **Routing graph.** The roads handler records every way a car can use, with one-way rules and a typical speed;
  `RoadGraph` splits them at shared nodes into vertices and segments and stores the network in compact form (see
  [Database](database.md#routing-graph)).
- **Lines without route relations** are assembled from their track (`transit.trackLines`), measured along a
  single-track spine and given the stations along them.

### Database build

After the tiles are written, `DatabaseWriter` bulk-loads every table in one transaction with journaling disabled,
then creates indexes, builds the FTS5 search index, runs `ANALYZE` and `VACUUM`, and atomically moves the file into
place. See [Database](database.md) for the schema.

## Extending

**Add a tile layer:**

1. Create `layers/MyLayer.java` extending `Layer`, with `name()`, `filter()` and `processFeature()`.
2. Register it in `BombayChitraProfile`.
3. Add style layers in `web/src/map/style/layers.ts` and document it in `docs/tiles.md`.

**Add database content:** add a record type in `data/Records.java`, collect it in a handler via `ctx.sink()`, add the
table to `data/Schema.java` (bump `Schema.VERSION` for breaking changes), and write it in `DatabaseWriter`.

## Testing

The pipeline has unit tests under `pipeline/test`, written with JUnit and run without a build tool:

```bash
./scripts/test-pipeline.sh                                              # all tests
./scripts/test-pipeline.sh --select-class=bombaychitra.pipeline.TaxonomyTest
```

The script compiles `pipeline/src` and `pipeline/test` with `javac` and runs them with the JUnit console launcher,
downloading Planetiler and JUnit into `data/tools/` on first use (both checksum-verified). Tests run in seconds and
need no OpenStreetMap extract: they cover the rule syntax and the shipped `categories.json`, `region.json`
validation, Overture ↔ OpenStreetMap de-duplication, street and bridge clustering, transit line labels and
station ordering, building heights and addresses, and the routing graph encoding. Tests live in the same package as
the code they test, so package-private helpers can be tested directly.

## Performance

On an 8-core laptop the full build takes about 2½ minutes: about 8 s for pass 1, 42 s for pass 2, 8 s for the
Overture places, 38 s for the 2.1 million Overture buildings, 15 s to write tiles and 25 s to write the database
(including the routing graph). It runs in a 3 GB heap. The first run also downloads the OpenStreetMap extract
(~220 MB) and the Overture extracts (~345 MB).
