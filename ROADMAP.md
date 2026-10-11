# BombayChitra — Roadmap

What to build next, based on the current state of the repo (pipeline, Go server, MapLibre web app, 27 API
endpoints, examples in 11 languages). Ordered by impact: the first section is what makes the project *usable by
strangers*, the rest is what makes it *better than other maps of Mumbai*.

Items marked with a pull-request-sized scope are good first issues; open an issue before starting on the larger ones
so the design can be agreed first.

---

## 0. Ship it

The code is far ahead of the release process. `CHANGELOG.md` is still all `[Unreleased]`.

- [ ] **Tag `v0.1.0`** and let `release.yml` publish the binary + `bombaychitra.pmtiles` + `bombaychitra.db`, so
      people can run it without Java/Planetiler and a 3-minute build.
- [ ] **Host a public demo** (a small VPS or Fly.io/Railway is enough — it's one 16 MB binary + ~300 MB data) and
      put the link at the top of the README. A map project without a "Try it" link loses most visitors.
- [ ] **Short demo GIF** in the README (search → route → 3D buildings → dark mode).
- [ ] **`ROADMAP.md` + GitHub Issues** from this list, labelled `good first issue` where they fit, so contributors
      know where to start.

## 1. Testing & quality gaps

Only the Go server has tests (`routing`, `geo`, `pmtiles`, `httpapi`). CI just *compiles* the pipeline and
type-checks the web app.

- [ ] **Pipeline tests (JUnit)** — taxonomy matching (`Taxonomy.java`, the `"!*"` rule), Overture ↔ OSM
      de-duplication, bridge span clustering, route-relation → ordered stations.
- [ ] **Web tests** — Vitest for `format.ts`, `taxonomy.ts`, URL/shareable-link parsing; one Playwright smoke test
      (load map → search "CST" → open card → "Route here").
- [ ] **Data regression check** in `data.yml`: compare the weekly build against the previous release (place count
      per category, bridge count, line count, route length of Line 3) and fail/flag on a > 5 % drop — catches
      upstream OSM vandalism or Overture schema changes.
- [ ] **API contract test**: validate live responses against `api/openapi.yaml` so docs and server can't drift.

## 2. Routing — the biggest feature gap

`/api/v1/route` is car-only. Most Mumbaikars walk + train.

- [ ] **Walking profile** — the road graph already has footpaths and foot-over-bridges; add a `mode=foot` cost
      model (ignore one-ways, allow `highway=footway|steps|pedestrian`, use FOBs).
- [ ] **Two-wheeler profile** — allow roads cars can, forbid sea link / freeways where two-wheelers are banned
      (Bandra–Worli Sea Link, Atal Setu, Eastern Freeway).
- [ ] **Train + metro journey planner** — a station graph with interchanges (Dadar, Ghatkopar, Andheri, Thane…)
      and average inter-station times → "Andheri to CST: Western to Dadar, change to Central, ~55 min". Later:
      import GTFS if MMRDA/BEST/railways publish one.
- [ ] **Isochrones** — "where can I reach in 30 min from Kharghar" (`/api/v1/isochrone`), drawn as a polygon.
- [ ] **Turn-by-turn steps** in the route response (street names + manoeuvres), not just geometry.
- [ ] **Toll awareness** — flag Atal Setu, Sea Link, expressway tolls; `avoid=toll` option.

## 3. Mumbai-specific layers (what Google Maps doesn't do well)

- [ ] **Monsoon waterlogging hotspots** — a toggleable layer of flood-prone spots + a warning on routes passing
      through them during June–September.
- [ ] **BMC wards (A–T)** and **PIN codes** as areas — `/api/v1/areas?type=ward|pincode` and per-ward stats
      (hospitals, schools, buildings per ward).
- [ ] **Heritage layer** — Grade I/II/III heritage structures and precincts (Fort, Kala Ghoda, Ballard Estate).
- [ ] **Local-train extras** — platform side, first/last train, ladies/AC coach info at stations where OSM has it.
- [ ] **Navi Mumbai International Airport** & new metro stations — a "recently opened" badge from OSM
      `start_date`.

## 4. Language & accessibility

- [ ] **Marathi / Hindi UI and labels** — a language switch that changes map labels and UI strings.
- [ ] **Transliterated search** — "chhatrapati" vs "chatrapati", "vile parle" vs "vileparle": a normalised search
      column in the FTS5 index.
- [ ] **Accessibility pass** — keyboard navigation for panels, focus states, screen-reader labels for buttons,
      colour contrast on both themes (metro line colours on the dark map especially).

## 5. Offline & mobile

- [ ] **Installable PWA** with a service worker caching the app shell, styles and fonts.
- [ ] **Offline area download** — cache the tiles of a chosen area (e.g. "my neighbourhood") in the Cache API.
      Useful on trains with no signal.
- [ ] **"Locate me"** and "nearest station / hospital / ATM to me" shortcuts.

## 6. API & developer experience

- [ ] **Rate limiting** (per-IP token bucket) and an optional API-key mode before the public demo goes live —
      routing loads a ~100 MB graph and is the obvious thing to abuse.
- [ ] **Generated SDKs** — `bombaychitra` clients for npm and PyPI generated from `openapi.yaml`.
- [ ] **Batch geocoding** — `POST /api/v1/search/batch` with a list of addresses.
- [ ] **Distance matrix** — `POST /api/v1/matrix` (N origins × M destinations) built on the same search.
- [ ] **`/api/v1/health` + Prometheus `/metrics`** for anyone self-hosting.

## 7. Community & data improvement

- [ ] **"Report a problem" on every card** that opens an OpenStreetMap note at that location.
- [ ] **Coverage map** — a layer showing where data is thin (few places/buildings per km²) so mappers know where
      to contribute.
- [ ] **Overture → OSM import candidates** — export Overture-only places as a MapRoulette challenge.

---

## Suggested order

| Phase | Items | Why |
|---|---|---|
| **Now** | 0 (release, demo, GIF), rate limiting | Nothing else matters if nobody can try it |
| **Next** | Walking routing, Marathi labels, BMC wards/PIN codes, pipeline tests | High value, builds on data we already have |
| **Then** | Train journey planner, waterlogging layer, PWA/offline | Signature features — what makes it *the* Mumbai map |
| **Later** | Isochrones, matrix/batch APIs, SDKs, coverage map | Developer and community growth |
