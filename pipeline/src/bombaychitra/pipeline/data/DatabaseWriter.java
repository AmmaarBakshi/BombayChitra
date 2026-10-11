package bombaychitra.pipeline.data;

import bombaychitra.pipeline.RegionConfig;
import bombaychitra.pipeline.Taxonomy;
import bombaychitra.pipeline.geo.LineSpine;
import bombaychitra.pipeline.util.Geo;
import bombaychitra.pipeline.util.GeoJson;
import bombaychitra.pipeline.util.Tags;
import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.onthegomap.planetiler.util.Parse;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.StandardCopyOption;
import java.sql.Connection;
import java.sql.DriverManager;
import java.sql.PreparedStatement;
import java.sql.SQLException;
import java.sql.Statement;
import java.sql.Types;
import java.util.ArrayList;
import java.util.Collection;
import java.util.Comparator;
import java.util.HashMap;
import java.util.HashSet;
import java.util.LinkedHashMap;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Locale;
import java.util.Map;
import java.util.Set;
import java.util.StringJoiner;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.atomic.DoubleAdder;
import java.util.concurrent.atomic.LongAdder;
import java.util.function.Function;
import java.util.stream.Collectors;
import org.locationtech.jts.geom.Envelope;
import org.locationtech.jts.geom.Geometry;
import org.locationtech.jts.geom.GeometryFactory;
import org.locationtech.jts.geom.LineString;
import org.locationtech.jts.operation.linemerge.LineMerger;
import org.locationtech.jts.operation.union.UnaryUnionOp;
import org.locationtech.jts.simplify.TopologyPreservingSimplifier;
import org.locationtech.jts.index.strtree.STRtree;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/** Writes the collected records into {@code bombaychitra.db} (see {@link Schema}). */
public final class DatabaseWriter {

  private static final Logger LOGGER = LoggerFactory.getLogger(DatabaseWriter.class);
  private static final ObjectMapper JSON = new ObjectMapper();
  private static final TypeReference<Map<String, Object>> MAP = new TypeReference<>() {};
  private static final GeometryFactory FACTORY = new GeometryFactory();
  private static final int BATCH = 10_000;

  private static final Map<String, String> STATION_MODES = Map.of(
    "metro_station", "subway,light_rail", "monorail_station", "monorail", "railway_station", "train");
  private static final List<String> ROAD_ORDER = List.of("motorway", "trunk", "primary", "secondary", "tertiary",
    "minor", "service", "pedestrian", "track", "path");
  private static final Map<String, String> ROAD_LABELS = Map.of("motorway", "Expressway", "trunk", "Highway",
    "primary", "Main road", "secondary", "Arterial road", "tertiary", "Road", "minor", "Street", "service",
    "Service road", "pedestrian", "Pedestrian street", "track", "Track", "path", "Path");
  private static final Map<String, Double> LOCALITY_WEIGHT = Map.of("city", -3d, "town", -2.5, "suburb", -2.5,
    "quarter", -2d, "neighbourhood", -1.5, "village", -1.5, "island", -1.5, "hamlet", -0.5, "locality", 0d,
    "islet", 0d);
  private static final Map<String, String> MODE_LABELS = Map.of("subway", "Metro", "light_rail", "Light rail",
    "monorail", "Monorail", "train", "Suburban railway", "tram", "Tram");

  private final RegionConfig region;
  private final Taxonomy taxonomy;
  private final DataSink sink;
  private final Map<String, String> extraMeta;
  private final Map<String, String> routingMeta = new LinkedHashMap<>();

  private PreparedStatement searchInsert;
  private PreparedStatement memberInsert;
  private int searchPending;
  private int memberPending;

  public DatabaseWriter(RegionConfig region, Taxonomy taxonomy, DataSink sink, Map<String, String> extraMeta) {
    this.region = region;
    this.taxonomy = taxonomy;
    this.sink = sink;
    this.extraMeta = extraMeta;
  }

  public void write(Path path) throws IOException, SQLException {
    Path absolute = path.toAbsolutePath();
    Files.createDirectories(absolute.getParent());
    Path tmp = absolute.resolveSibling(absolute.getFileName() + ".tmp");
    Files.deleteIfExists(tmp);
    long start = System.nanoTime();

    try (Connection db = DriverManager.getConnection("jdbc:sqlite:" + tmp)) {
      try (Statement st = db.createStatement()) {
        st.execute("PRAGMA page_size = 4096");
        st.execute("PRAGMA journal_mode = OFF");
        st.execute("PRAGMA synchronous = OFF");
        st.execute("PRAGMA temp_store = MEMORY");
        st.execute("PRAGMA cache_size = -262144");
        st.execute("PRAGMA locking_mode = EXCLUSIVE");
      }
      db.setAutoCommit(false);
      script(db, Schema.DDL);
      searchInsert = db.prepareStatement(
        "INSERT INTO search(name, alt, detail, kind, ref, category, lon, lat, weight) VALUES (?,?,?,?,?,?,?,?,?)");
      memberInsert = db.prepareStatement(
        "INSERT OR IGNORE INTO area_members(area_id, kind, member_id) VALUES (?,?,?)");

      AreaIndex areas = writeAreas(db);
      var localities = new LocalityIndex(sink.localities);
      writeCategories(db);
      List<LineStop> stops = writeTransitLines(db);
      writePlaces(db, areas, localities, stops);
      writeLocalities(db, areas);
      writeStreets(db, areas, localities);
      writeBridges(db, areas, localities);
      writeBuildings(db, areas, localities);
      RoadGraph.write(db, sink.driveWays, routingMeta);
      flushSearch();
      flushMembers();
      script(db, Schema.INDEXES);
      writeStats(db, areas);
      writeMeta(db);
      db.commit();
      db.setAutoCommit(true);
      try (Statement st = db.createStatement()) {
        st.execute("INSERT INTO search(search) VALUES ('optimize')");
        st.execute("ANALYZE");
        st.execute("PRAGMA locking_mode = NORMAL");
        st.execute("PRAGMA journal_mode = DELETE");
        st.execute("VACUUM");
      }
    }
    Files.move(tmp, absolute, StandardCopyOption.REPLACE_EXISTING);
    LOGGER.info("Database written to {} ({} MB) in {}s", absolute, Files.size(absolute) / 1_000_000,
      (System.nanoTime() - start) / 1_000_000_000);
  }

  // ---------------------------------------------------------------- areas

  private AreaIndex writeAreas(Connection db) throws SQLException {
    Map<String, List<Geometry>> parts = new HashMap<>();
    for (var part : sink.areaParts) {
      parts.computeIfAbsent(part.areaId(), k -> new ArrayList<>()).add(part.polygon());
    }
    List<AreaIndex.Entry> entries = new ArrayList<>();
    try (var insert = db.prepareStatement("INSERT INTO areas VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)")) {
      int sort = 0;
      for (var area : region.areas() == null ? List.<RegionConfig.Area>of() : region.areas()) {
        var geoms = parts.get(area.id());
        if (geoms == null || geoms.isEmpty()) {
          LOGGER.warn("Area '{}' (relations {}) was not found in the source extract", area.id(), area.relations());
          continue;
        }
        Geometry union = UnaryUnionOp.union(geoms);
        if (!union.isValid()) {
          union = union.buffer(0);
        }
        double km2 = Geo.areaKm2(union);
        Envelope env = union.getEnvelopeInternal();
        var centre = union.getInteriorPoint();
        Geometry simplified = TopologyPreservingSimplifier.simplify(union, 0.0002);
        int i = 1;
        insert.setString(i++, area.id());
        insert.setString(i++, area.name());
        insert.setString(i++, area.shortName());
        insert.setString(i++, area.kind());
        insert.setString(i++, area.description());
        insert.setString(i++, json(area.relations()));
        insert.setDouble(i++, round(km2, 2));
        insert.setDouble(i++, env.getMinX());
        insert.setDouble(i++, env.getMinY());
        insert.setDouble(i++, env.getMaxX());
        insert.setDouble(i++, env.getMaxY());
        insert.setDouble(i++, Tags.round6(centre.getX()));
        insert.setDouble(i++, Tags.round6(centre.getY()));
        insert.setInt(i++, sort);
        insert.setString(i, GeoJson.write(simplified));
        insert.executeUpdate();
        entries.add(new AreaIndex.Entry(area.id(), sort, km2, union));
        search(area.name(), area.shortName(), capitalise(area.kind()), "area", area.id(), null, centre.getX(),
          centre.getY(), -4);
        sort++;
      }
    }
    LOGGER.info("Areas: {}", entries.stream().map(e -> e.id() + "=" + Math.round(e.areaKm2()) + "km²").toList());
    return new AreaIndex(entries);
  }

  // ---------------------------------------------------------------- categories

  private void writeCategories(Connection db) throws SQLException {
    Map<String, Long> counts = sink.places.stream()
      .collect(Collectors.groupingBy(Records.Place::category, Collectors.counting()));
    try (var groups = db.prepareStatement("INSERT INTO category_groups VALUES (?,?,?,?)");
      var cats = db.prepareStatement("INSERT INTO categories VALUES (?,?,?,?,?,?,?,?)")) {
      for (var group : taxonomy.groups()) {
        groups.setString(1, group.id());
        groups.setString(2, group.label());
        groups.setString(3, group.colour());
        groups.setInt(4, group.sort());
        groups.executeUpdate();
        for (var c : group.categories()) {
          cats.setString(1, c.id());
          cats.setString(2, c.group());
          cats.setString(3, c.label());
          cats.setString(4, c.icon());
          cats.setInt(5, c.rank());
          cats.setInt(6, c.minzoom());
          cats.setInt(7, c.sort());
          cats.setLong(8, counts.getOrDefault(c.id(), 0L));
          cats.executeUpdate();
        }
      }
    }
  }

  // ---------------------------------------------------------------- transit

  private record LineStop(String key, String ref, String label, String colour, String mode, String state,
    double lon, double lat) {}

  /** Stations further than this from a line's track are not inferred as its stops. */
  private static final double INFERRED_STOP_RADIUS_M = 120;

  private List<LineStop> writeTransitLines(Connection db) throws SQLException {
    Map<Long, Map<Long, Records.RouteWay>> waysByRoute = new HashMap<>();
    for (var way : sink.routeWays) {
      waysByRoute.computeIfAbsent(way.routeId(), k -> new LinkedHashMap<>()).putIfAbsent(way.wayId(), way);
    }
    STRtree stations = new STRtree();
    for (var place : sink.places) {
      if (STATION_MODES.containsKey(place.category())) {
        stations.insert(new Envelope(place.lon(), place.lon(), place.lat(), place.lat()), place);
      }
    }
    Map<Long, Map<Long, Records.RouteStop>> stopsByRoute = new HashMap<>();
    for (var stop : sink.routeStops) {
      stopsByRoute.computeIfAbsent(stop.routeId(), k -> new HashMap<>()).putIfAbsent(stop.nodeId(), stop);
    }
    List<Records.Route> routes = new ArrayList<>(sink.routes.values());
    routes.sort(Comparator.comparing(Records.Route::mode).thenComparing(r -> String.valueOf(r.network()))
      .thenComparing(r -> String.valueOf(r.ref())).thenComparing(r -> String.valueOf(r.name())));

    List<LineStop> allStops = new ArrayList<>();
    Set<String> searchedKeys = new LinkedHashSet<>();
    Set<Long> usedIds = new HashSet<>();
    routes.stream().filter(r -> !r.fromTracks()).forEach(r -> usedIds.add(r.id()));
    int written = 0;
    int assembled = 0;
    try (var insert = db.prepareStatement(
      "INSERT INTO transit_lines VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)")) {
      for (var route : routes) {
        var ways = waysByRoute.get(route.id());
        if (ways == null || ways.isEmpty()) {
          continue;
        }
        var merger = new LineMerger();
        ways.values().forEach(w -> merger.add(w.line()));
        @SuppressWarnings("unchecked")
        Collection<LineString> merged = merger.getMergedLineStrings();
        Geometry geometry = FACTORY.createMultiLineString(merged.toArray(LineString[]::new));
        Envelope env = geometry.getEnvelopeInternal();
        var spine = LineSpine.of(merged);
        String state = route.state() != null ? route.state() : lifecycle(ways.values());

        // A line assembled from tracks is identified by its longest way.
        long id = route.id();
        String osmType = "relation";
        if (route.fromTracks()) {
          osmType = "way";
          id = ways.values().stream()
            .sorted(Comparator.comparingDouble((Records.RouteWay w) -> -Geo.lengthMeters(w.line())))
            .mapToLong(Records.RouteWay::wayId).filter(usedIds::add).findFirst().orElseThrow();
          assembled++;
        }

        List<Map<String, Object>> stops = new ArrayList<>();
        var nodes = stopsByRoute.getOrDefault(route.id(), Map.of());
        String previous = null;
        for (Long nodeId : route.stopNodeIds()) {
          var stop = nodes.get(nodeId);
          if (stop == null || stop.name() != null && stop.name().equals(previous)) {
            continue;
          }
          previous = stop.name();
          var row = new LinkedHashMap<String, Object>();
          row.put("id", nodeId * 10 + 1);
          row.put("name", stop.name());
          row.put("lon", Tags.round6(stop.lon()));
          row.put("lat", Tags.round6(stop.lat()));
          stops.add(row);
        }
        if (stops.isEmpty()) {
          stops = inferStops(stations, spine, route.mode(), env);
        }
        String from = route.from();
        String to = route.to();
        if (route.fromTracks() && stops.size() >= 2) {
          from = (String) stops.getFirst().get("name");
          to = (String) stops.getLast().get("name");
        }
        String key = lineKey(route, state);
        String label = lineLabel(route);
        for (var s : stops) {
          allStops.add(new LineStop(key, route.ref(), label, route.colour(), route.mode(), state,
            (double) s.get("lon"), (double) s.get("lat")));
        }
        int i = 1;
        insert.setLong(i++, id);
        insert.setString(i++, osmType);
        insert.setString(i++, key);
        insert.setString(i++, route.mode());
        insert.setString(i++, route.network());
        insert.setString(i++, route.ref());
        insert.setString(i++, label);
        insert.setString(i++, route.name());
        insert.setString(i++, route.colour());
        insert.setString(i++, route.operator());
        insert.setString(i++, from);
        insert.setString(i++, to);
        insert.setString(i++, state);
        insert.setDouble(i++, Math.round(spine.lengthMeters()));
        insert.setInt(i++, stops.size());
        insert.setDouble(i++, env.getMinX());
        insert.setDouble(i++, env.getMinY());
        insert.setDouble(i++, env.getMaxX());
        insert.setDouble(i++, env.getMaxY());
        insert.setString(i++, json(stops));
        insert.setString(i, GeoJson.write(geometry));
        insert.executeUpdate();
        written++;

        if (searchedKeys.add(key)) {
          var centre = geometry.getInteriorPoint();
          String detail = (route.network() != null ? route.network() : MODE_LABELS.getOrDefault(route.mode(), "Line"))
            + " · " + MODE_LABELS.getOrDefault(route.mode(), route.mode())
            + ("construction".equals(state) ? " · Under construction" : "proposed".equals(state) ? " · Proposed" : "");
          search(label, joinNonNull(route.ref(), route.name()), detail, "line", id, route.mode(),
            centre.getX(), centre.getY(), "construction".equals(state) || "proposed".equals(state) ? -2 : -3);
        }
      }
    }
    LOGGER.info("Transit lines: {} routes ({} assembled from tracks), {} stops", written, assembled,
      allStops.size());
    return allStops;
  }

  /**
   * Lines not yet in service get their own key ({@code Mumbai Metro:9:construction}) so an extension under
   * construction is listed apart from the open section of the same line.
   */
  static String lineKey(Records.Route route, String state) {
    String scope = route.network() != null ? route.network() : route.mode();
    String id = route.ref() != null ? route.ref() : lineLabel(route);
    return "construction".equals(state) || "proposed".equals(state) ? scope + ":" + id + ":" + state
      : scope + ":" + id;
  }

  /** {@code construction} or {@code proposed} when no track of the line is in service, otherwise null. */
  static String lifecycle(Collection<Records.RouteWay> ways) {
    boolean construction = false;
    for (var way : ways) {
      if (way.lifecycle() == null) {
        return null;
      }
      construction |= "construction".equals(way.lifecycle());
    }
    return construction ? "construction" : "proposed";
  }

  /** Stations along a line without mapped stops, ordered from one end to the other. */
  private static List<Map<String, Object>> inferStops(STRtree stations, LineSpine spine, String mode, Envelope env) {
    Set<String> categories = new HashSet<>();
    STATION_MODES.forEach((category, modes) -> {
      if (Set.of(modes.split(",")).contains(mode)) {
        categories.add(category);
      }
    });
    Envelope probe = new Envelope(env);
    probe.expandBy(0.002);
    record Candidate(Records.Place place, double position) {}
    List<Candidate> found = new ArrayList<>();
    for (Object o : stations.query(probe)) {
      var place = (Records.Place) o;
      if (categories.contains(place.category())
        && spine.distanceMeters(place.lon(), place.lat()) <= INFERRED_STOP_RADIUS_M) {
        found.add(new Candidate(place, spine.position(place.lon(), place.lat())));
      }
    }
    found.sort(Comparator.comparingDouble(Candidate::position).thenComparingLong(c -> c.place().id()));
    List<Map<String, Object>> stops = new ArrayList<>();
    Set<String> names = new HashSet<>();
    for (var c : found) {
      var place = c.place();
      boolean duplicate = place.name() == null
        ? found.stream().anyMatch(o -> o.place().name() != null
          && Geo.haversine(o.place().lon(), o.place().lat(), place.lon(), place.lat()) < 200)
        : !names.add(place.name());
      if (duplicate) {
        continue;
      }
      var row = new LinkedHashMap<String, Object>();
      row.put("id", place.id());
      row.put("name", place.name());
      row.put("lon", Tags.round6(place.lon()));
      row.put("lat", Tags.round6(place.lat()));
      stops.add(row);
    }
    return stops;
  }

  static String lineLabel(Records.Route route) {
    if (route.label() != null) {
      return route.label();
    }
    String name = route.name();
    if (name == null) {
      return route.ref() != null ? MODE_LABELS.getOrDefault(route.mode(), "Line") + " " + route.ref() : "Line";
    }
    int colon = name.indexOf(':');
    if (colon > 0) {
      name = name.substring(0, colon);
    }
    // "Line 2 (Andheri West → Dahisar (East))" → "Line 2": drop the direction part, which may nest parentheses.
    for (String arrow : List.of("→", "=>", "->", "⇒")) {
      int at = name.indexOf(arrow);
      if (at > 0) {
        int paren = name.indexOf('(');
        if (paren > 0 && paren < at) {
          name = name.substring(0, paren);
        }
        break;
      }
    }
    return name.replaceAll("\\s*\\([^()]*\\)\\s*$", "").strip();
  }

  // ---------------------------------------------------------------- places

  private void writePlaces(Connection db, AreaIndex areas, LocalityIndex localities, List<LineStop> stops)
    throws SQLException {
    STRtree stopTree = new STRtree();
    for (var stop : stops) {
      stopTree.insert(new Envelope(stop.lon(), stop.lon(), stop.lat(), stop.lat()), stop);
    }
    List<Records.Place> places = new ArrayList<>(sink.places);
    places.sort(Comparator.comparingLong(Records.Place::id));
    Map<String, Taxonomy.Category> categories = new HashMap<>();
    taxonomy.groups().forEach(g -> g.categories().forEach(c -> categories.put(c.id(), c)));

    try (var insert = db.prepareStatement(
      "INSERT OR IGNORE INTO places VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)");
      var rtree = db.prepareStatement("INSERT OR IGNORE INTO places_rtree VALUES (?,?,?,?,?)")) {
      int pending = 0;
      for (var place : places) {
        Map<String, Object> tags = parse(place.tagsJson());
        String locality = localities.nearest(place.lon(), place.lat());
        String extra = null;
        String modes = STATION_MODES.get(place.category());
        if (modes != null) {
          extra = stationLines(stopTree, place, Set.of(modes.split(",")));
        }
        int i = 1;
        insert.setLong(i++, place.id());
        insert.setString(i++, Tags.typeName(place.id()));
        insert.setLong(i++, place.id() / 10);
        insert.setString(i++, place.category());
        insert.setString(i++, place.group());
        insert.setString(i++, place.name());
        insert.setString(i++, str(tags, "name:mr", "name:hi"));
        insert.setDouble(i++, Tags.round6(place.lon()));
        insert.setDouble(i++, Tags.round6(place.lat()));
        setNullableDouble(insert, i++, place.areaM2() > 0 ? place.areaM2() : null);
        insert.setString(i++, locality);
        insert.setString(i++, Tags.address(com.onthegomap.planetiler.reader.WithTags.from(tags)));
        insert.setString(i++, str(tags, "addr:postcode", "postal_code"));
        insert.setString(i++, str(tags, "phone", "contact:phone", "contact:mobile", "mobile"));
        insert.setString(i++, str(tags, "website", "contact:website", "url"));
        insert.setString(i++, str(tags, "email", "contact:email"));
        insert.setString(i++, str(tags, "opening_hours"));
        insert.setString(i++, str(tags, "operator"));
        insert.setString(i++, str(tags, "brand"));
        insert.setString(i++, str(tags, "cuisine"));
        insert.setString(i++, str(tags, "wheelchair"));
        insert.setString(i++, str(tags, "wikidata"));
        insert.setString(i++, str(tags, "wikipedia"));
        insert.setString(i++, extra);
        insert.setString(i, place.tagsJson());
        insert.addBatch();
        rtree.setLong(1, place.id());
        rtree.setDouble(2, place.lon());
        rtree.setDouble(3, place.lon());
        rtree.setDouble(4, place.lat());
        rtree.setDouble(5, place.lat());
        rtree.addBatch();
        for (String area : areas.at(place.lon(), place.lat())) {
          member(area, "place", place.id());
        }
        if (place.name() != null) {
          var category = categories.get(place.category());
          String detail = category.label() + (locality != null ? " · " + locality : "");
          search(place.name(), joinNonNull(str(tags, "name:mr"), str(tags, "name:hi"), str(tags, "alt_name"),
              str(tags, "old_name"), str(tags, "official_name"), str(tags, "short_name"), str(tags, "brand")),
            detail, "place", place.id(), place.category(), place.lon(), place.lat(), category.rank() * 0.5 - 2);
        }
        if (++pending % BATCH == 0) {
          insert.executeBatch();
          rtree.executeBatch();
        }
      }
      insert.executeBatch();
      rtree.executeBatch();
    }
    LOGGER.info("Places: {}", places.size());
  }

  private static String stationLines(STRtree stopTree, Records.Place place, Set<String> modes) {
    Envelope probe = new Envelope(place.lon(), place.lon(), place.lat(), place.lat());
    probe.expandBy(0.004);
    Map<String, Map<String, Object>> lines = new LinkedHashMap<>();
    for (Object o : stopTree.query(probe)) {
      var stop = (LineStop) o;
      if (!modes.contains(stop.mode())
        || Geo.haversine(place.lon(), place.lat(), stop.lon(), stop.lat()) > 350) {
        continue;
      }
      lines.computeIfAbsent(stop.key(), k -> {
        var row = new LinkedHashMap<String, Object>();
        row.put("key", stop.key());
        row.put("ref", stop.ref());
        row.put("label", stop.label());
        row.put("colour", stop.colour());
        row.put("mode", stop.mode());
        if (stop.state() != null) {
          row.put("state", stop.state());
        }
        return row;
      });
    }
    if (lines.isEmpty()) {
      return null;
    }
    var sorted = new ArrayList<>(lines.values());
    sorted.sort(Comparator.comparing(m -> String.valueOf(m.get("key"))));
    return json(Map.of("lines", sorted));
  }

  // ---------------------------------------------------------------- localities

  private void writeLocalities(Connection db, AreaIndex areas) throws SQLException {
    try (var insert = db.prepareStatement("INSERT OR IGNORE INTO localities VALUES (?,?,?,?,?,?,?)")) {
      for (var l : sink.localities) {
        insert.setLong(1, l.id());
        insert.setString(2, l.name());
        insert.setString(3, l.kind());
        insert.setDouble(4, Tags.round6(l.lon()));
        insert.setDouble(5, Tags.round6(l.lat()));
        if (l.population() == null) {
          insert.setNull(6, Types.INTEGER);
        } else {
          insert.setLong(6, l.population());
        }
        insert.setString(7, l.tagsJson());
        insert.addBatch();
        List<String> in = areas.at(l.lon(), l.lat());
        in.forEach(a -> member(a, "locality", l.id()));
        String detail = capitalise(l.kind()) + (in.isEmpty() ? "" : " · " + areaName(in.getLast()));
        Map<String, Object> tags = parse(l.tagsJson());
        search(l.name(), joinNonNull(str(tags, "name:mr"), str(tags, "name:hi"), str(tags, "alt_name"),
          str(tags, "old_name")), detail, "locality", l.id(), l.kind(), l.lon(), l.lat(),
          LOCALITY_WEIGHT.getOrDefault(l.kind(), 0d));
      }
      insert.executeBatch();
    }
    LOGGER.info("Localities: {}", sink.localities.size());
  }

  // ---------------------------------------------------------------- streets & roads

  private void writeStreets(Connection db, AreaIndex areas, LocalityIndex localities) throws SQLException {
    List<Records.Road> roads = new ArrayList<>(sink.roads);
    roads.sort(Comparator.comparingLong(Records.Road::id));
    var clusters = Clusters.of(roads, r -> r.name() != null ? r.name() : r.ref(),
      r -> r.line().getEnvelopeInternal(), 0.004);
    try (var streetInsert = db.prepareStatement("INSERT INTO streets VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)");
      var roadInsert = db.prepareStatement("INSERT OR IGNORE INTO roads VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)");
      var rtree = db.prepareStatement("INSERT OR IGNORE INTO roads_rtree VALUES (?,?,?,?,?)")) {
      long streetId = 0;
      int pending = 0;
      for (var cluster : clusters) {
        streetId++;
        String name = mostCommon(cluster, Records.Road::name);
        String ref = mostCommon(cluster, Records.Road::ref);
        String kind = cluster.stream().map(Records.Road::kind)
          .min(Comparator.comparingInt(k -> ROAD_ORDER.indexOf(k) < 0 ? 99 : ROAD_ORDER.indexOf(k))).orElse("minor");
        double length = cluster.stream().mapToDouble(Records.Road::lengthM).sum();
        Envelope env = new Envelope();
        cluster.forEach(r -> env.expandToInclude(r.line().getEnvelopeInternal()));
        var longest = cluster.stream().max(Comparator.comparingDouble(Records.Road::lengthM)).orElseThrow();
        var centre = longest.line().getInteriorPoint();
        String locality = localities.nearest(centre.getX(), centre.getY());

        int i = 1;
        streetInsert.setLong(i++, streetId);
        streetInsert.setString(i++, name);
        streetInsert.setString(i++, ref);
        streetInsert.setString(i++, kind);
        streetInsert.setDouble(i++, Math.round(length));
        streetInsert.setInt(i++, cluster.size());
        streetInsert.setString(i++, locality);
        streetInsert.setDouble(i++, env.getMinX());
        streetInsert.setDouble(i++, env.getMinY());
        streetInsert.setDouble(i++, env.getMaxX());
        streetInsert.setDouble(i++, env.getMaxY());
        streetInsert.setDouble(i++, Tags.round6(centre.getX()));
        streetInsert.setDouble(i, Tags.round6(centre.getY()));
        streetInsert.addBatch();
        for (String area : areas.at(centre.getX(), centre.getY())) {
          member(area, "street", streetId);
        }
        String detail = ROAD_LABELS.getOrDefault(kind, "Road") + (locality != null ? " · " + locality : "");
        int order = ROAD_ORDER.indexOf(kind);
        search(name != null ? name : ref, name != null ? ref : null, detail, "street", streetId, kind,
          centre.getX(), centre.getY(), order < 0 ? 0 : -2 + order * 0.3);

        for (var road : cluster) {
          Map<String, Object> tags = parse(road.tagsJson());
          var withTags = com.onthegomap.planetiler.reader.WithTags.from(tags);
          Envelope e = road.line().getEnvelopeInternal();
          i = 1;
          roadInsert.setLong(i++, road.id());
          roadInsert.setLong(i++, streetId);
          roadInsert.setString(i++, road.name());
          roadInsert.setString(i++, road.ref());
          roadInsert.setString(i++, road.kind());
          roadInsert.setString(i++, road.highway());
          int direction = withTags.getDirection("oneway");
          if (direction == 0) {
            roadInsert.setNull(i++, Types.INTEGER);
          } else {
            roadInsert.setInt(i++, direction);
          }
          Integer lanes = Parse.parseIntOrNull(str(tags, "lanes"));
          if (lanes == null) {
            roadInsert.setNull(i++, Types.INTEGER);
          } else {
            roadInsert.setInt(i++, lanes);
          }
          roadInsert.setString(i++, str(tags, "maxspeed"));
          roadInsert.setString(i++, str(tags, "surface"));
          roadInsert.setInt(i++, Tags.isYes(withTags, "bridge") ? 1 : 0);
          roadInsert.setInt(i++, Tags.isYes(withTags, "tunnel") ? 1 : 0);
          roadInsert.setDouble(i++, road.lengthM());
          roadInsert.setString(i, GeoJson.write(road.line()));
          roadInsert.addBatch();
          rtree.setLong(1, road.id());
          rtree.setDouble(2, e.getMinX());
          rtree.setDouble(3, e.getMaxX());
          rtree.setDouble(4, e.getMinY());
          rtree.setDouble(5, e.getMaxY());
          rtree.addBatch();
          if (++pending % BATCH == 0) {
            streetInsert.executeBatch();
            roadInsert.executeBatch();
            rtree.executeBatch();
          }
        }
      }
      streetInsert.executeBatch();
      roadInsert.executeBatch();
      rtree.executeBatch();
      LOGGER.info("Roads: {} named ways in {} streets", roads.size(), streetId);
    }
  }

  // ---------------------------------------------------------------- bridges

  private void writeBridges(Connection db, AreaIndex areas, LocalityIndex localities) throws SQLException {
    List<Records.Bridge> bridges = new ArrayList<>(sink.bridges);
    bridges.sort(Comparator.comparingLong(Records.Bridge::id));
    var named = Clusters.of(bridges.stream().filter(b -> b.name() != null && !"outline".equals(b.kind())).toList(),
      Records.Bridge::name, b -> b.geometry().getEnvelopeInternal(), 0.003);
    Map<Long, Long> groupOf = new HashMap<>();
    try (var insert = db.prepareStatement("INSERT INTO named_bridges VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)")) {
      long groupId = 0;
      for (var cluster : named) {
        groupId++;
        for (var b : cluster) {
          groupOf.put(b.id(), groupId);
        }
        // Dual carriageways and parallel tracks are separate ways: the span is the longest connected run, while the
        // total counts every deck.
        double total = cluster.stream().mapToDouble(Records.Bridge::lengthM).sum();
        var merger = new LineMerger();
        cluster.forEach(b -> merger.add(b.geometry()));
        @SuppressWarnings("unchecked")
        Collection<LineString> runs = merger.getMergedLineStrings();
        double length = Math.min(total, runs.stream().mapToDouble(Geo::lengthMeters).max().orElse(total));
        Envelope env = new Envelope();
        cluster.forEach(b -> env.expandToInclude(b.geometry().getEnvelopeInternal()));
        var longest = cluster.stream().max(Comparator.comparingDouble(Records.Bridge::lengthM)).orElseThrow();
        var centre = longest.geometry().getInteriorPoint();
        String locality = localities.nearest(centre.getX(), centre.getY());
        String kind = mostCommon(cluster, Records.Bridge::kind);
        String name = mostCommon(cluster, Records.Bridge::name);
        int i = 1;
        insert.setLong(i++, groupId);
        insert.setString(i++, name);
        insert.setString(i++, kind);
        insert.setDouble(i++, Math.round(length));
        insert.setDouble(i++, Math.round(total));
        insert.setInt(i++, cluster.size());
        insert.setString(i++, locality);
        insert.setDouble(i++, env.getMinX());
        insert.setDouble(i++, env.getMinY());
        insert.setDouble(i++, env.getMaxX());
        insert.setDouble(i++, env.getMaxY());
        insert.setDouble(i++, Tags.round6(centre.getX()));
        insert.setDouble(i, Tags.round6(centre.getY()));
        insert.addBatch();
        String detail = "Bridge · " + formatLength(length) + (locality != null ? " · " + locality : "");
        search(name, null, detail, "bridge", groupId, kind, centre.getX(), centre.getY(),
          length > 1000 ? -2.5 : length > 200 ? -1.5 : -0.5);
      }
      insert.executeBatch();
    }

    try (var insert = db.prepareStatement("INSERT OR IGNORE INTO bridges VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)");
      var rtree = db.prepareStatement("INSERT OR IGNORE INTO bridges_rtree VALUES (?,?,?,?,?)")) {
      for (var b : bridges) {
        Map<String, Object> tags = parse(b.tagsJson());
        var centre = b.geometry().getInteriorPoint();
        Envelope env = b.geometry().getEnvelopeInternal();
        String structure = str(tags, "bridge");
        if ("yes".equals(structure)) {
          structure = str(tags, "bridge:structure");
        }
        int i = 1;
        insert.setLong(i++, b.id());
        Long group = groupOf.get(b.id());
        if (group == null) {
          insert.setNull(i++, Types.INTEGER);
        } else {
          insert.setLong(i++, group);
        }
        insert.setString(i++, b.name());
        insert.setString(i++, b.kind());
        insert.setString(i++, b.cls());
        insert.setString(i++, structure);
        Integer layer = Parse.parseIntOrNull(str(tags, "layer"));
        if (layer == null) {
          insert.setNull(i++, Types.INTEGER);
        } else {
          insert.setInt(i++, layer);
        }
        insert.setString(i++, str(tags, "ref"));
        Integer lanes = Parse.parseIntOrNull(str(tags, "lanes"));
        if (lanes == null) {
          insert.setNull(i++, Types.INTEGER);
        } else {
          insert.setInt(i++, lanes);
        }
        insert.setDouble(i++, b.lengthM());
        insert.setDouble(i++, b.areaM2());
        insert.setDouble(i++, Tags.round6(centre.getX()));
        insert.setDouble(i++, Tags.round6(centre.getY()));
        insert.setString(i++, GeoJson.write(b.geometry()));
        insert.setString(i, b.tagsJson());
        insert.addBatch();
        rtree.setLong(1, b.id());
        rtree.setDouble(2, env.getMinX());
        rtree.setDouble(3, env.getMaxX());
        rtree.setDouble(4, env.getMinY());
        rtree.setDouble(5, env.getMaxY());
        rtree.addBatch();
        for (String area : areas.at(centre.getX(), centre.getY())) {
          member(area, "bridge", b.id());
        }
      }
      insert.executeBatch();
      rtree.executeBatch();
    }
    LOGGER.info("Bridges: {} ({} named groups)", bridges.size(), named.size());
  }

  // ---------------------------------------------------------------- buildings

  private void writeBuildings(Connection db, AreaIndex areas, LocalityIndex localities) throws SQLException {
    List<Records.Building> buildings = new ArrayList<>(sink.buildings);
    buildings.sort(Comparator.comparingLong(Records.Building::id));
    try (var insert = db.prepareStatement("INSERT OR IGNORE INTO buildings VALUES (?,?,?,?,?,?,?,?,?,?,?,?)");
      var rtree = db.prepareStatement("INSERT OR IGNORE INTO buildings_rtree VALUES (?,?,?,?,?)")) {
      int pending = 0;
      for (var b : buildings) {
        Map<String, Object> tags = parse(b.tagsJson());
        int i = 1;
        insert.setLong(i++, b.id());
        insert.setString(i++, b.name());
        insert.setString(i++, b.kind());
        if (b.levels() == null) {
          insert.setNull(i++, Types.INTEGER);
        } else {
          insert.setInt(i++, b.levels());
        }
        setNullableDouble(insert, i++, b.heightM());
        setNullableDouble(insert, i++, b.minHeightM());
        insert.setDouble(i++, b.areaM2());
        insert.setDouble(i++, Tags.round6(b.lon()));
        insert.setDouble(i++, Tags.round6(b.lat()));
        insert.setString(i++, Tags.address(com.onthegomap.planetiler.reader.WithTags.from(tags)));
        insert.setString(i++, b.name() != null || b.heightM() != null ? localities.nearest(b.lon(), b.lat()) : null);
        insert.setString(i, b.tagsJson());
        insert.addBatch();
        rtree.setLong(1, b.id());
        rtree.setDouble(2, b.lon());
        rtree.setDouble(3, b.lon());
        rtree.setDouble(4, b.lat());
        rtree.setDouble(5, b.lat());
        rtree.addBatch();
        if (++pending % BATCH == 0) {
          insert.executeBatch();
          rtree.executeBatch();
        }
      }
      insert.executeBatch();
      rtree.executeBatch();
    }
    LOGGER.info("Buildings: {} detailed records", buildings.size());
  }

  // ---------------------------------------------------------------- stats & meta

  private void writeStats(Connection db, AreaIndex areas) throws SQLException {
    try (Statement st = db.createStatement()) {
      st.execute("""
        INSERT INTO area_stats(area_id, key, value)
        SELECT am.area_id, 'places:' || p.category, COUNT(*) FROM area_members am
        JOIN places p ON p.id = am.member_id WHERE am.kind = 'place' GROUP BY am.area_id, p.category""");
      st.execute("""
        INSERT INTO area_stats(area_id, key, value)
        SELECT '*', 'places:' || category, COUNT(*) FROM places GROUP BY category""");
      st.execute("""
        INSERT INTO area_stats(area_id, key, value)
        SELECT area_id, 'places', COUNT(*) FROM area_members WHERE kind = 'place' GROUP BY area_id
        UNION ALL SELECT '*', 'places', COUNT(*) FROM places""");
      st.execute("""
        INSERT INTO area_stats(area_id, key, value)
        SELECT am.area_id, 'bridges', COUNT(*) FROM area_members am JOIN bridges b ON b.id = am.member_id
        WHERE am.kind = 'bridge' AND b.kind <> 'outline' GROUP BY am.area_id
        UNION ALL SELECT am.area_id, 'bridges_km', ROUND(SUM(b.length_m) / 1000.0, 2) FROM area_members am
        JOIN bridges b ON b.id = am.member_id WHERE am.kind = 'bridge' GROUP BY am.area_id
        UNION ALL SELECT '*', 'bridges', COUNT(*) FROM bridges WHERE kind <> 'outline'
        UNION ALL SELECT '*', 'bridges_km', ROUND(SUM(length_m) / 1000.0, 2) FROM bridges""");
      st.execute("""
        INSERT INTO area_stats(area_id, key, value)
        SELECT area_id, 'streets', COUNT(*) FROM area_members WHERE kind = 'street' GROUP BY area_id
        UNION ALL SELECT '*', 'streets', COUNT(*) FROM streets""");
    }

    // Building statistics cover every footprint, not just the detailed rows.
    record Acc(LongAdder count, LongAdder withHeight, DoubleAdder footprint, DoubleAdder heightSum,
      java.util.concurrent.atomic.AtomicReference<Double> max) {
      Acc() {
        this(new LongAdder(), new LongAdder(), new DoubleAdder(), new DoubleAdder(),
          new java.util.concurrent.atomic.AtomicReference<>(0d));
      }

      void add(double height, double area) {
        count.increment();
        footprint.add(area);
        if (!Double.isNaN(height)) {
          withHeight.increment();
          heightSum.add(height);
          max.accumulateAndGet(height, Math::max);
        }
      }
    }
    Map<String, Acc> stats = new ConcurrentHashMap<>();
    stats.put("*", new Acc());
    sink.buildingStats().parallelStream().forEach(chunk -> {
      for (int i = 0; i + 3 < chunk.length; i += 4) {
        double height = chunk[i + 2];
        double area = chunk[i + 3];
        stats.get("*").add(height, area);
        for (String id : areas.at(chunk[i], chunk[i + 1])) {
          stats.computeIfAbsent(id, k -> new Acc()).add(height, area);
        }
      }
    });
    try (var insert = db.prepareStatement("INSERT INTO area_stats(area_id, key, value) VALUES (?,?,?)")) {
      for (var e : stats.entrySet()) {
        Acc a = e.getValue();
        long withHeight = a.withHeight().sum();
        put(insert, e.getKey(), "buildings", a.count().sum());
        put(insert, e.getKey(), "buildings_with_height", withHeight);
        put(insert, e.getKey(), "building_footprint_km2", round(a.footprint().sum() / 1e6, 3));
        put(insert, e.getKey(), "building_avg_height_m", withHeight == 0 ? 0 : round(a.heightSum().sum() / withHeight, 1));
        put(insert, e.getKey(), "building_max_height_m", a.max().get());
      }
      insert.executeBatch();
    }
  }

  private static void put(PreparedStatement insert, String area, String key, double value) throws SQLException {
    insert.setString(1, area);
    insert.setString(2, key);
    insert.setDouble(3, value);
    insert.addBatch();
  }

  private void writeMeta(Connection db) throws SQLException {
    Map<String, String> meta = new LinkedHashMap<>();
    meta.put("schema_version", String.valueOf(Schema.VERSION));
    meta.put("region_id", region.id());
    meta.put("region_name", region.name());
    meta.put("description", region.description());
    meta.put("bounds", json(region.bounds()));
    meta.put("center", json(region.center()));
    meta.put("zoom", String.valueOf(region.zoom()));
    meta.put("maxzoom", String.valueOf(region.maxzoom()));
    meta.put("source_name", region.source().name());
    meta.put("source_url", region.source().url());
    boolean overture = extraMeta.containsKey("overture_release");
    meta.put("attribution", "© OpenStreetMap contributors (ODbL)" + (overture ? "; © Overture Maps Foundation" : ""));
    meta.put("license", "ODbL-1.0");
    meta.putAll(extraMeta);
    meta.putAll(routingMeta);
    try (Statement st = db.createStatement()) {
      for (String table : List.of("places", "localities", "streets", "roads", "bridges", "named_bridges", "buildings",
        "transit_lines")) {
        var rs = st.executeQuery("SELECT COUNT(*) FROM " + table);
        rs.next();
        meta.put("count_" + table, String.valueOf(rs.getLong(1)));
      }
    }
    try (var insert = db.prepareStatement("INSERT INTO meta VALUES (?,?)")) {
      for (var e : meta.entrySet()) {
        if (e.getValue() != null) {
          insert.setString(1, e.getKey());
          insert.setString(2, e.getValue());
          insert.addBatch();
        }
      }
      insert.executeBatch();
    }
  }

  // ---------------------------------------------------------------- helpers

  private void search(String name, String alt, String detail, String kind, Object ref, String category, double lon,
    double lat, double weight) throws SQLException {
    if (name == null) {
      return;
    }
    searchInsert.setString(1, name);
    searchInsert.setString(2, alt);
    searchInsert.setString(3, detail);
    searchInsert.setString(4, kind);
    searchInsert.setObject(5, ref);
    searchInsert.setString(6, category);
    searchInsert.setDouble(7, Tags.round6(lon));
    searchInsert.setDouble(8, Tags.round6(lat));
    searchInsert.setDouble(9, weight);
    searchInsert.addBatch();
    if (++searchPending % BATCH == 0) {
      flushSearch();
    }
  }

  private void flushSearch() throws SQLException {
    searchInsert.executeBatch();
  }

  private void member(String area, String kind, long id) {
    try {
      memberInsert.setString(1, area);
      memberInsert.setString(2, kind);
      memberInsert.setLong(3, id);
      memberInsert.addBatch();
      if (++memberPending % BATCH == 0) {
        flushMembers();
      }
    } catch (SQLException e) {
      throw new IllegalStateException(e);
    }
  }

  private void flushMembers() throws SQLException {
    memberInsert.executeBatch();
  }

  private String areaName(String id) {
    return region.areas().stream().filter(a -> a.id().equals(id)).map(RegionConfig.Area::name).findFirst().orElse(id);
  }

  private static void script(Connection db, String sql) throws SQLException {
    try (Statement st = db.createStatement()) {
      for (String statement : sql.split(";\\s*\\n")) {
        if (!statement.isBlank()) {
          st.execute(statement);
        }
      }
    }
  }

  private static Map<String, Object> parse(String json) {
    try {
      return JSON.readValue(json, MAP);
    } catch (JsonProcessingException e) {
      return Map.of();
    }
  }

  private static String json(Object value) {
    try {
      return JSON.writeValueAsString(value);
    } catch (JsonProcessingException e) {
      throw new IllegalStateException(e);
    }
  }

  private static String str(Map<String, Object> tags, String... keys) {
    for (String key : keys) {
      Object value = tags.get(key);
      if (value != null) {
        String s = value.toString().strip();
        if (!s.isEmpty()) {
          return s;
        }
      }
    }
    return null;
  }

  private static String joinNonNull(String... values) {
    var joiner = new StringJoiner(" ");
    for (String v : values) {
      if (v != null && !v.isBlank()) {
        joiner.add(v);
      }
    }
    String result = joiner.toString();
    return result.isEmpty() ? null : result;
  }

  private static <T> String mostCommon(List<T> items, Function<T, String> getter) {
    Map<String, Integer> counts = new HashMap<>();
    for (T item : items) {
      String value = getter.apply(item);
      if (value != null) {
        counts.merge(value, 1, Integer::sum);
      }
    }
    return counts.entrySet().stream().max(Map.Entry.<String, Integer>comparingByValue()
      .thenComparing(Map.Entry.comparingByKey(Comparator.reverseOrder()))).map(Map.Entry::getKey).orElse(null);
  }

  private static void setNullableDouble(PreparedStatement st, int index, Double value) throws SQLException {
    if (value == null) {
      st.setNull(index, Types.REAL);
    } else {
      st.setDouble(index, value);
    }
  }

  private static double round(double value, int decimals) {
    double scale = Math.pow(10, decimals);
    return Math.round(value * scale) / scale;
  }

  private static String capitalise(String value) {
    if (value == null || value.isEmpty()) {
      return value;
    }
    return value.substring(0, 1).toUpperCase(Locale.ROOT) + value.substring(1).replace('_', ' ');
  }

  private static String formatLength(double metres) {
    return metres >= 1000 ? String.format(Locale.ROOT, "%.1f km", metres / 1000) : Math.round(metres) + " m";
  }
}
