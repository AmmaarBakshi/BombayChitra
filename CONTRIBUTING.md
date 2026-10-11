# Contributing to BombayChitra

Thanks for helping map Mumbai! The full guide is in [docs/contributing.md](docs/contributing.md); the essentials:

1. **Data fixes go to OpenStreetMap.** Wrong name, missing building height, new café? Edit it on
   [openstreetmap.org](https://www.openstreetmap.org) — every detail card in the app links there.
2. **Set up:** Java 21+, Go 1.26+, Node 22+, then `./scripts/build-data.sh`, `cd web && npm ci && npm run build`,
   `cd server && go run ./cmd/bombaychitra serve --data ../data/build`.
3. **Check before a PR:** `make test`, or `cd server && go vet ./... && go test ./...`,
   `cd web && npm run typecheck && npm run build` and `./scripts/test-pipeline.sh`.
4. **Commits:** [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `docs:`…), focused
   pull requests, screenshots for visual changes, docs and `api/openapi.yaml` updated with user-visible changes.

Please follow the [Code of Conduct](CODE_OF_CONDUCT.md). By contributing you agree your contributions are licensed
under the MIT licence.
