package bombaychitra.pipeline.data;

/**
 * SQLite schema of {@code bombaychitra.db}. The database is the language-neutral distribution format of the dataset:
 * any language with a SQLite driver can query it directly. Bump {@link #VERSION} on breaking changes.
 */
public final class Schema {

  public static final int VERSION = 2;

  private Schema() {}

  public static final String DDL = """
    CREATE TABLE meta (
      key   TEXT PRIMARY KEY,
      value TEXT NOT NULL
    ) WITHOUT ROWID;

    CREATE TABLE category_groups (
      id     TEXT PRIMARY KEY,
      label  TEXT NOT NULL,
      colour TEXT NOT NULL,
      sort   INTEGER NOT NULL
    ) WITHOUT ROWID;

    CREATE TABLE categories (
      id       TEXT PRIMARY KEY,
      group_id TEXT NOT NULL REFERENCES category_groups(id),
      label    TEXT NOT NULL,
      icon     TEXT NOT NULL,
      rank     INTEGER NOT NULL,
      minzoom  INTEGER NOT NULL,
      sort     INTEGER NOT NULL,
      count    INTEGER NOT NULL DEFAULT 0
    ) WITHOUT ROWID;

    CREATE TABLE areas (
      id          TEXT PRIMARY KEY,
      name        TEXT NOT NULL,
      short_name  TEXT,
      kind        TEXT,
      description TEXT,
      relations   TEXT NOT NULL,
      area_km2    REAL,
      min_lon REAL, min_lat REAL, max_lon REAL, max_lat REAL,
      lon REAL, lat REAL,
      sort        INTEGER NOT NULL,
      geometry    TEXT
    ) WITHOUT ROWID;

    CREATE TABLE area_members (
      area_id   TEXT NOT NULL,
      kind      TEXT NOT NULL,
      member_id INTEGER NOT NULL,
      PRIMARY KEY (kind, member_id, area_id)
    ) WITHOUT ROWID;

    CREATE TABLE area_stats (
      area_id TEXT NOT NULL,
      key     TEXT NOT NULL,
      value   REAL NOT NULL,
      PRIMARY KEY (area_id, key)
    ) WITHOUT ROWID;

    CREATE TABLE places (
      id            INTEGER PRIMARY KEY,
      osm_type      TEXT NOT NULL,
      osm_id        INTEGER NOT NULL,
      category      TEXT NOT NULL,
      group_id      TEXT NOT NULL,
      name          TEXT,
      name_local    TEXT,
      lon           REAL NOT NULL,
      lat           REAL NOT NULL,
      area_m2       REAL,
      locality      TEXT,
      address       TEXT,
      postcode      TEXT,
      phone         TEXT,
      website       TEXT,
      email         TEXT,
      opening_hours TEXT,
      operator      TEXT,
      brand         TEXT,
      cuisine       TEXT,
      wheelchair    TEXT,
      wikidata      TEXT,
      wikipedia     TEXT,
      extra         TEXT,
      tags          TEXT NOT NULL
    );
    CREATE VIRTUAL TABLE places_rtree USING rtree(id, min_lon, max_lon, min_lat, max_lat);

    CREATE TABLE localities (
      id         INTEGER PRIMARY KEY,
      name       TEXT NOT NULL,
      kind       TEXT NOT NULL,
      lon        REAL NOT NULL,
      lat        REAL NOT NULL,
      population INTEGER,
      tags       TEXT NOT NULL
    );

    CREATE TABLE streets (
      id        INTEGER PRIMARY KEY,
      name      TEXT,
      ref       TEXT,
      kind      TEXT NOT NULL,
      length_m  REAL NOT NULL,
      way_count INTEGER NOT NULL,
      locality  TEXT,
      min_lon REAL, min_lat REAL, max_lon REAL, max_lat REAL,
      lon REAL NOT NULL, lat REAL NOT NULL
    );

    CREATE TABLE roads (
      id        INTEGER PRIMARY KEY,
      street_id INTEGER NOT NULL REFERENCES streets(id),
      name      TEXT,
      ref       TEXT,
      kind      TEXT NOT NULL,
      highway   TEXT NOT NULL,
      oneway    INTEGER,
      lanes     INTEGER,
      maxspeed  TEXT,
      surface   TEXT,
      bridge    INTEGER NOT NULL DEFAULT 0,
      tunnel    INTEGER NOT NULL DEFAULT 0,
      length_m  REAL NOT NULL,
      geometry  TEXT NOT NULL
    );
    CREATE VIRTUAL TABLE roads_rtree USING rtree(id, min_lon, max_lon, min_lat, max_lat);

    CREATE TABLE bridges (
      id        INTEGER PRIMARY KEY,
      group_id  INTEGER,
      name      TEXT,
      kind      TEXT NOT NULL,
      class     TEXT,
      structure TEXT,
      layer     INTEGER,
      ref       TEXT,
      lanes     INTEGER,
      length_m  REAL NOT NULL,
      area_m2   REAL NOT NULL,
      lon       REAL NOT NULL,
      lat       REAL NOT NULL,
      geometry  TEXT NOT NULL,
      tags      TEXT NOT NULL
    );
    CREATE VIRTUAL TABLE bridges_rtree USING rtree(id, min_lon, max_lon, min_lat, max_lat);

    CREATE TABLE named_bridges (
      id             INTEGER PRIMARY KEY,
      name           TEXT NOT NULL,
      kind           TEXT NOT NULL,
      length_m       REAL NOT NULL,
      total_length_m REAL NOT NULL,
      way_count      INTEGER NOT NULL,
      locality  TEXT,
      min_lon REAL, min_lat REAL, max_lon REAL, max_lat REAL,
      lon REAL NOT NULL, lat REAL NOT NULL
    );

    CREATE TABLE buildings (
      id           INTEGER PRIMARY KEY,
      name         TEXT,
      kind         TEXT,
      levels       INTEGER,
      height_m     REAL,
      min_height_m REAL,
      area_m2      REAL NOT NULL,
      lon          REAL NOT NULL,
      lat          REAL NOT NULL,
      address      TEXT,
      locality     TEXT,
      tags         TEXT NOT NULL
    );
    CREATE VIRTUAL TABLE buildings_rtree USING rtree(id, min_lon, max_lon, min_lat, max_lat);

    CREATE TABLE transit_lines (
      id         INTEGER PRIMARY KEY,
      osm_type   TEXT NOT NULL,
      line_key   TEXT NOT NULL,
      mode       TEXT NOT NULL,
      network    TEXT,
      ref        TEXT,
      label      TEXT NOT NULL,
      name       TEXT,
      colour     TEXT,
      operator   TEXT,
      from_name  TEXT,
      to_name    TEXT,
      state      TEXT,
      length_m   REAL NOT NULL,
      stop_count INTEGER NOT NULL,
      min_lon REAL, min_lat REAL, max_lon REAL, max_lat REAL,
      stops      TEXT NOT NULL,
      geometry   TEXT NOT NULL
    );

    CREATE VIRTUAL TABLE search USING fts5(
      name, alt, detail,
      kind UNINDEXED, ref UNINDEXED, category UNINDEXED, lon UNINDEXED, lat UNINDEXED, weight UNINDEXED,
      tokenize = 'unicode61 remove_diacritics 2',
      prefix = '1 2 3'
    );
    """;

  /** Indexes are created after bulk loading, which is considerably faster than maintaining them during inserts. */
  public static final String INDEXES = """
    CREATE INDEX places_category ON places(category);
    CREATE INDEX places_group ON places(group_id);
    CREATE INDEX places_osm ON places(osm_type, osm_id);
    CREATE INDEX area_members_area ON area_members(area_id, kind, member_id);
    CREATE INDEX roads_street ON roads(street_id);
    CREATE INDEX streets_name ON streets(name COLLATE NOCASE);
    CREATE INDEX bridges_kind ON bridges(kind, length_m DESC);
    CREATE INDEX bridges_group ON bridges(group_id);
    CREATE INDEX named_bridges_length ON named_bridges(length_m DESC);
    CREATE INDEX buildings_height ON buildings(height_m DESC);
    CREATE INDEX buildings_levels ON buildings(levels DESC);
    CREATE INDEX transit_lines_key ON transit_lines(line_key);
    CREATE INDEX localities_kind ON localities(kind);
    """;
}
