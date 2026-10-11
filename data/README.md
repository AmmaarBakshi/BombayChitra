# data

Working directory for the data build (not committed):

- `sources/` — the OpenStreetMap extract (downloaded on first build)
- `tools/` — Planetiler (downloaded and checksum-verified by the build scripts)
- `build/` — outputs: `bombaychitra.pmtiles` and `bombaychitra.db`

Build with `scripts/build-data.sh` (or `.ps1`), or download both files from the **Data (latest)** release into
`build/`.
