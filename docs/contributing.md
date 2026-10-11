# Contributing

Contributions of all sizes are welcome — bug reports, new place categories, style improvements, docs, examples and
code. There are also two ways to improve the map without writing code:

- **Fix the data on OpenStreetMap.** Every detail card has an **OSM** button that opens the feature on
  openstreetmap.org, where anyone can correct names, add building heights, phone numbers or opening hours. Changes
  appear in BombayChitra after the next data build.
- **Report issues** on [GitHub](https://github.com/AmmaarBakshi/BombayChitra/issues) with a link to the map view
  (the URL includes the position and the selected feature).

## Development setup

See [Getting started](getting-started.md). In short:

```bash
./scripts/build-data.sh                 # data (Java 21+)
cd web && npm ci && npm run build       # web app + docs (Node 22+)
cd ../server && go run ./cmd/bombaychitra serve --data ../data/build
```

For web development run `npm run dev` in `web/` (hot reload, proxies to the server on :8080).

## Checks

Run before opening a pull request:

```bash
cd server && go vet ./... && go test ./...      # unit tests + integration tests against your local data
cd web && npm test && npm run build             # unit tests (Vitest), type check, style validation, docs build
./scripts/test-pipeline.sh                      # pipeline unit tests (JUnit, no OSM data needed)
```

`make test` runs all of them. CI runs the same checks and also compiles the pipeline with `javac -Werror`.

## Code style

- **Go:** `gofmt`; small packages under `server/internal`; errors returned, not logged and ignored.
- **TypeScript:** strict mode, no framework, explicit `.ts` import extensions; small modules under `web/src`.
- **Java:** Planetiler idioms; one class per tile layer; 2-space indentation, 120 columns.
- **Comments** explain *why*, not *what*.
- **Docs:** update `docs/` and `api/openapi.yaml` with any user-visible change.

## Common contributions

| You want to… | Change |
|---|---|
| Add or refine a place category | `config/categories.json` (see [Configuration](configuration.md)) |
| Add an area | `config/region.json` → `areas` |
| Improve the map's look | `web/src/map/style/` (see [Map styles](styles.md)) |
| Add a tile layer or attribute | `pipeline/…/layers/` + style + `docs/tiles.md` |
| Add an API endpoint | `server/internal/store` + `server/internal/httpapi` + `api/openapi.yaml` |
| Add an example in a new language | `examples/<language>/` + `examples/README.md` |

## Releasing

Releases follow [Semantic Versioning](https://semver.org/); the API is versioned separately in its path (`/api/v1`).

1. Make sure CI is green on `main` and the **Data** workflow has published a recent `data-latest` (the release
   ships that dataset).
2. In `CHANGELOG.md`, rename `## [Unreleased]` to `## [0.1.0] - YYYY-MM-DD` and start a new empty `[Unreleased]`
   section above it. Check the notes read well on their own: they become the GitHub release notes
   (`scripts/changelog-section.sh 0.1.0` prints exactly what will be published).
3. Commit (`chore: release v0.1.0`), then tag and push:

   ```bash
   git tag -a v0.1.0 -m "BombayChitra v0.1.0"
   git push origin main v0.1.0
   ```

The **Release** workflow then cross-compiles the server for Linux, macOS and Windows (amd64 and arm64), attaches
the `data-latest` dataset (verified against its checksums) and a `SHA256SUMS` for everything, publishes the release
with the changelog section as notes, and pushes the `ghcr.io/ammaarbakshi/bombaychitra` image tagged with the
version, `major.minor` and `latest`. Anyone can then run the release with `bombaychitra download --release v0.1.0`.

## Commits and pull requests

- Use [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `docs:`, `perf:`, `refactor:`).
- Keep pull requests focused; include a screenshot for visual changes.
- By contributing you agree that your contributions are licensed under the project's MIT licence.
