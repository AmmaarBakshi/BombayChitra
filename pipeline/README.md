# pipeline

Turns an OpenStreetMap extract into `bombaychitra.pmtiles` (vector tiles) and `bombaychitra.db` (SQLite) in a single
pass, using [Planetiler](https://github.com/onthegomap/planetiler). Requires a JDK 21+.

```bash
../scripts/build-data.sh            # or ..\scripts\build-data.ps1 on Windows
```

| Path | Contents |
|---|---|
| `src/bombaychitra/pipeline/Main.java` | Entry point: loads `config/`, runs Planetiler, writes the database |
| `…/BombayChitraProfile.java` | Registers the layer handlers |
| `…/layers/` | One handler per tile layer (roads, bridges, transit, pois, buildings, water…) |
| `…/data/` | Records, SQLite schema and writer, area index, clustering |
| `…/geo/Coastline.java` | Ocean polygons from coastline ways |
| `Dockerfile` | Container image for the pipeline |

See [docs/pipeline.md](../docs/pipeline.md) for options, internals and how to add a layer.
