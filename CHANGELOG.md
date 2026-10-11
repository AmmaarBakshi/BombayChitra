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
