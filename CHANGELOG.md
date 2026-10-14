# Changelog

All notable changes are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/) and the
project uses [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- Data pipeline (Java + Planetiler): one pass over OpenStreetMap produces a 13-layer vector tileset (PMTiles) and a
  SQLite database with R*Tree spatial indexes, FTS5 search, area membership and statistics.
- Ocean polygons built from the coastline, with no global water-polygon download.
- 89-category place taxonomy in `config/categories.json`, including schools, colleges, hospitals and pre-schools
  mapped only as buildings (`building=school` and so on); `"!*"` (tag absent) rule syntax; region and areas (Mumbai
  City, Mumbai Suburban, MaKaBo, Thane District, Thane, Navi Mumbai, Mira-Bhayandar, Kalyan-Dombivli, Vasai-Virar,
  Panvel) in `config/region.json`.
- Transit lines from OSM route relations with official colours and ordered stops; station → line lookup.
- Lines without a route relation (the monorail, metro lines under construction and proposed) assembled from their
  track via `transit.trackLines`, with stations matched along the line; `transit.routeOverrides` for corrections.
- Line status (`construction`, `proposed`) derived from track lifecycle tags; proposed metro alignments in the
  `transit` tile layer; route lengths count parallel tracks once.
- Bridges layer and named-bridge aggregation with spans; validated building heights for 3D.
- Overture Maps enrichment: ~109,000 places (confidence ≥ 0.6, mapped through `overture` lists in the taxonomy and
  deduplicated against OpenStreetMap) and ~1.8 million machine-learned building footprints, filling in Navi Mumbai,
  Panvel, Taloja and Vasai-Virar; downloaded by the build scripts (`scripts/fetch-overture.py`), optional.
- Distance by road: car routing graph built by the pipeline (`routing` table), `/api/v1/route` (A*, fastest or
  shortest, one-way aware) and a "Distance by road" panel with map picking, search, shareable links and
  "Route here" on every place card.
- Panvel Taluka and Uran Taluka areas; a dashboard hider (toolbar button and `H`) for a full-screen map.
- Go server: REST API (search, reverse geocoding, places, streets, roads, bridges, buildings, transit, areas,
  stats), PMTiles tile server, TileJSON, portable light/dark styles, raw downloads, GeoJSON/NDJSON/CSV export.
- Web app (TypeScript + MapLibre GL JS 6): light/dark themes, unified search, inspector, layer toggles, transit,
  bridge explorer, 3D buildings, measuring tool, shareable links, mobile layout.
- Documentation site and API reference generated from `api/openapi.yaml`; examples in 11 languages.
- Build scripts (Bash, PowerShell), Makefile, Dockerfiles, Docker Compose, CI, weekly data release and release
  workflows.
- `ROADMAP.md` with the plan for the next releases.
- Rate limiting for public servers: per-client token buckets (`--rate-limit`, `--rate-burst`; route searches cost
  more), `429` with `Retry-After` and `X-RateLimit-*` headers, and optional API keys (`--api-keys`, `--require-key`)
  sent in `X-API-Key`.
- `/api/v1/health` (database, tiles and routing checks, dataset age; `503` when the database is unreadable) and
  Prometheus `/metrics` (requests, latency and bytes per route pattern, route searches, build info), optionally on a
  private address (`--metrics-addr`).
- `bombaychitra download`: fetches the published dataset with checksum verification, so a server runs without Java
  or a pipeline build. The weekly data release now includes `SHA256SUMS`.
- Tagged releases attach the dataset and checksums next to the binaries and take their notes from this changelog.
- Ready-made deployments in `deploy/`: Fly.io (public demo), Docker Compose with Caddy and automatic HTTPS, and
  hardened systemd units with a weekly data refresh timer.
- Tests: pipeline unit tests with JUnit (`scripts/test-pipeline.sh`, no build tool) and web unit tests with Vitest
  (`npm test`), both run in CI.
- Routes by two-wheeler and on foot as well as by car (`mode=car|two_wheeler|foot`): the routing graph (format
  version 2) now includes footways, steps, pedestrian streets and foot over-bridges, records which modes may use each
  segment (motorways are closed to two-wheelers and pedestrians), and flags tolls, steps, bridges, tunnels,
  roundabouts and ramps. Version 1 graphs still route cars.
- Turn-by-turn steps in every route (type, modifier, road, distance, time, location and an English instruction),
  `toll_m` and `avoid=toll`; `/api/v1/meta` lists the supported travel modes; `bombaychitra route` prints a route
  from the command line.
- Web: a car / two-wheeler / walk switch, an "avoid tolls" toggle, toll notices, a directions list and dashed walking
  routes in the distance panel; travel mode and toll choice are part of shareable links.
- Quality gates for the weekly data release: a regression check against the previous build
  (`scripts/check-data.py`, fails on drops above 5 %), an API contract test that calls every documented example
  (`npm run test:contract`) and a Playwright smoke test of the map (`npm run test:e2e`).
- The 24 BMC administrative wards as areas (`/api/v1/areas?kind=ward`), with statistics for every listed area
  (`stats=true`), a ward picker and a side-by-side ward comparison in the Explore panel.
- PIN code areas derived from addresses (`/api/v1/areas?kind=pincode`, `/api/v1/areas/400050`), PIN codes in
  search and the postcode of any point in reverse geocoding.
- Curated layers maintained in `config/`: heritage sites with their status — UNESCO, precinct or monument
  (`/api/v1/heritage`) — and monsoon waterlogging hotspots (`/api/v1/hotspots`), shown as map overlays; routes warn
  about hotspots along the way during the monsoon.
- Marathi and Hindi: names in the vector tiles (`name:mr`, `name:hi`), styles labelled in either language
  (`?lang=mr|hi`) and a translated interface with a language switch.
- Spelling-insensitive search: transliteration variants ("Chhatrapati" / "Chatrapati", "Mattunga" / "Matunga",
  "Zaveri" / "Javeri", accents) find the same places, with the folding shared by the pipeline and the server.
- Station cards list the lines, operator and opening date, with a "recently opened" badge for new stations.
- Accessibility: keyboard navigation between panel tabs, skip links, accessible names for icon buttons, focus moved
  into cards and back, reduced motion honoured, and contrasting casings for line colours that fail WCAG non-text
  contrast on either theme.
- The regression check also watches wards, curated layers and indexed spellings.
- Journeys by train, metro and monorail: `/api/v1/journey` plans the quickest trip with the walk to a station, rides
  with changes of line and the walk at the end, each ride with its line, direction, stations and wait, and walks
  routed along footpaths (`format=geojson` too). Waits come from the usual gap between trains on each line
  (`transit.headways` in `config/region.json`, recorded in the dataset), with express patterns running half as often.
  Also `bombaychitra journey`, a "Train & metro" option with a journey timeline in the distance panel, and the
  journey network in `/api/v1/health`.
- An installable web app that works offline: a service worker caches the app shell and map fonts, the docs show an
  offline page, an install button appears when the browser offers it, and **offline maps** save the tiles of the area
  in view (listed with their size, shown or deleted from the Layers panel).
- "Near me": the nearest stations, hospitals and ATMs from your location (or the centre of the map), with walking
  time and directions there.
- A documentation page for the web app: journeys, near me, installing, offline maps, keyboard and shareable links.

### Fixed

- Overture places were dropped as duplicates of a different nearby business that shared only generic words with
  them ("ICICI Bank ATM" next to "HDFC Bank ATM"); names must now share a distinctive word.
- Searching for a code or former name ("CST", "VT", "Victoria Terminus") now finds the place it belongs to before
  businesses that merely contain the letters.
- `/api/v1/health` always reports the database latency, and its routing detail is always an object.
- The API reference's road, bridge and building examples used an id that did not exist.
- Addresses and locality search details end with the district, not the BMC ward.
- The "Avoid tolls" switch stayed visible when walking: the `hidden` attribute now hides rows laid out with flex.
- The API guide said the server had no rate limiting of its own.
