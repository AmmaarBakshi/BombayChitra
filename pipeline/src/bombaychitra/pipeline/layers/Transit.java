package bombaychitra.pipeline.layers;

import bombaychitra.pipeline.RegionConfig;
import bombaychitra.pipeline.data.Records;
import bombaychitra.pipeline.util.TagRule;
import bombaychitra.pipeline.util.Tags;
import com.onthegomap.planetiler.FeatureCollector;
import com.onthegomap.planetiler.FeatureMerge;
import com.onthegomap.planetiler.ForwardingProfile;
import com.onthegomap.planetiler.VectorTile;
import com.onthegomap.planetiler.expression.Expression;
import com.onthegomap.planetiler.geo.GeometryException;
import com.onthegomap.planetiler.reader.SourceFeature;
import com.onthegomap.planetiler.reader.osm.OsmElement;
import com.onthegomap.planetiler.reader.osm.OsmReader;
import com.onthegomap.planetiler.reader.osm.OsmRelationInfo;
import java.util.ArrayList;
import java.util.Comparator;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.CopyOnWriteArrayList;

/**
 * {@code transit} — suburban rail, metro, monorail and tram lines coloured by their OSM route relations, plus platform
 * outlines. Route relations (geometry and ordered stops) are also captured for the {@code transit_lines} table, along
 * with the configured track lines ({@code transit.trackLines}) for lines that have no route relation in OSM.
 */
public final class Transit extends Layer
  implements ForwardingProfile.OsmRelationPreprocessor, ForwardingProfile.LayerPostProcessor {

  public static final String NAME = "transit";

  private static final Map<String, String> KINDS = Map.of(
    "rail", "rail", "narrow_gauge", "rail", "preserved", "rail", "subway", "subway", "light_rail", "light_rail",
    "monorail", "monorail", "tram", "tram", "funicular", "funicular", "construction", "construction",
    "proposed", "proposed"
  );
  private static final Map<String, Integer> MINZOOM = Map.of(
    "rail", 6, "subway", 8, "light_rail", 8, "monorail", 8, "tram", 11, "funicular", 12, "construction", 9,
    "proposed", 10
  );
  /** Modes whose proposed alignments are shown; proposed mainline railways are too speculative to map. */
  private static final Set<String> PROPOSED_MODES = Set.of("subway", "light_rail", "monorail", "tram");
  private static final Map<String, Integer> MODE_PRIORITY = Map.of(
    "subway", 0, "light_rail", 0, "monorail", 0, "train", 1, "tram", 2
  );

  private record TrackLine(TagRule rule, Records.Route route) {}

  private record RouteOverride(TagRule rule, RegionConfig.LineSpec spec) {}

  private final Set<String> modes;
  private final Set<String> trainNetworks;
  private final List<TrackLine> trackLines = new ArrayList<>();
  private final List<RouteOverride> routeOverrides = new ArrayList<>();
  /**
   * Planetiler only tracks relation membership for ways, so stop/platform nodes are indexed here during the relation
   * pre-pass (pass 1) and looked up by node id when the nodes are processed in pass 2.
   */
  private final Map<Long, List<Long>> stopNodeRoutes = new ConcurrentHashMap<>();

  public Transit(LayerContext ctx) {
    super(ctx);
    this.modes = ctx.region().transitModes();
    var transit = ctx.region().transit();
    this.trainNetworks = transit == null || transit.trainNetworks() == null ? Set.of()
      : Set.copyOf(transit.trainNetworks());
    var overrides = ctx.region().routeOverrides();
    for (int i = 0; i < overrides.size(); i++) {
      routeOverrides.add(new RouteOverride(overrides.get(i).rule("transit.routeOverrides[" + i + "]"),
        overrides.get(i)));
    }
    var tracks = ctx.region().trackLines();
    for (int i = 0; i < tracks.size(); i++) {
      var spec = tracks.get(i);
      String colour = ctx.region().normaliseColour(spec.colour());
      String label = Tags.clean(spec.label());
      var route = new Records.Route(-(i + 1L), spec.mode(), Tags.clean(spec.ref()), label, label,
        colour != null ? colour : ctx.region().modeColour(spec.mode()), Tags.clean(spec.network()),
        Tags.clean(spec.operator()), null, null, null, List.of());
      ctx.sink().routes.put(route.id(), route);
      trackLines.add(new TrackLine(spec.rule("transit.trackLines[" + i + "]"), route));
    }
  }

  @Override
  public String name() {
    return NAME;
  }

  @Override
  public Expression filter() {
    return Expression.or(
      Expression.matchAny("railway", "rail", "narrow_gauge", "preserved", "subway", "light_rail", "monorail", "tram",
        "funicular", "construction", "proposed", "platform", "stop", "halt", "station", "tram_stop"),
      Expression.matchAny("public_transport", "stop_position", "platform", "station")
    );
  }

  @Override
  public List<OsmRelationInfo> preprocessOsmRelation(OsmElement.Relation relation) {
    if (!relation.hasTag("type", "route")) {
      return null;
    }
    String mode = relation.getString("route");
    if (mode == null || !modes.contains(mode)) {
      return null;
    }
    String network = relation.getString("network");
    if ("train".equals(mode) && (network == null || !trainNetworks.contains(network))) {
      return null;
    }
    String ref = Tags.clean(relation.getString("ref"));
    String label = null;
    String colour = ctx.region().normaliseColour(relation.getString("colour"));
    String operator = Tags.clean(relation.getString("operator"));
    for (var override : routeOverrides) {
      if (override.rule().test(relation)) {
        var spec = override.spec();
        ref = spec.ref() != null ? spec.ref() : ref;
        label = Tags.clean(spec.label());
        network = spec.network() != null ? spec.network() : network;
        colour = spec.colour() != null ? ctx.region().normaliseColour(spec.colour()) : colour;
        operator = spec.operator() != null ? spec.operator() : operator;
        break;
      }
    }
    if (colour == null) {
      colour = ctx.region().modeColour(mode);
    }
    List<Long> stops = memberNodes(relation, "stop");
    if (stops.isEmpty()) {
      stops = memberNodes(relation, "platform");
    }
    var route = new Records.Route(relation.id(), mode, ref, Tags.name(relation), label, colour, network, operator,
      Tags.clean(relation.getString("from")), Tags.clean(relation.getString("to")),
      Tags.clean(relation.getString("state")), stops);
    ctx.sink().routes.put(relation.id(), route);
    for (Long node : stops) {
      stopNodeRoutes.computeIfAbsent(node, k -> new CopyOnWriteArrayList<>()).add(relation.id());
    }
    return List.of(route);
  }

  private static List<Long> memberNodes(OsmElement.Relation relation, String rolePrefix) {
    List<Long> result = new ArrayList<>();
    for (var member : relation.members()) {
      if (member.type() == OsmElement.Type.NODE && member.role() != null && member.role().startsWith(rolePrefix)) {
        result.add(member.ref());
      }
    }
    return List.copyOf(result);
  }

  @Override
  public void processFeature(SourceFeature f, FeatureCollector features) {
    List<OsmReader.RelationMember<Records.Route>> memberships = f.relationInfo(Records.Route.class);
    try {
      if (f.isPoint()) {
        collectStop(f);
      } else if (f.canBePolygon() && (f.hasTag("railway", "platform") || f.hasTag("public_transport", "platform"))) {
        if (f.hasTag("railway", "platform") || f.hasTag("train", "yes") || f.hasTag("subway", "yes")
          || f.hasTag("monorail", "yes") || f.hasTag("light_rail", "yes")) {
          features.polygon(NAME).setAttr("kind", "platform").setMinZoom(14).setMinPixelSize(1);
        }
      } else if (f.canBeLine()) {
        line(f, features, memberships);
      }
    } catch (GeometryException e) {
      e.log("transit");
    }
  }

  private void collectStop(SourceFeature f) throws GeometryException {
    if (Tags.typeCode(f) != 1) {
      return;
    }
    List<Long> routes = stopNodeRoutes.get(f.id());
    if (routes == null || !ctx.inBounds(f)) {
      return;
    }
    var anchor = ctx.anchor(f);
    String name = Tags.name(f);
    for (Long route : routes) {
      ctx.sink().routeStops.add(new Records.RouteStop(route, f.id(), name, anchor.x, anchor.y));
    }
  }

  private void line(SourceFeature f, FeatureCollector features, List<OsmReader.RelationMember<Records.Route>> memberships)
    throws GeometryException {
    String railway = f.getString("railway");
    String kind = railway == null ? null : KINDS.get(railway);
    if (kind == null) {
      return;
    }
    // Lifecycle: railway=construction|proposed + construction|proposed=<mode>, or an in-service railway value tagged
    // construction=yes (a common shortcut for metro viaducts being built).
    String lifecycle = null;
    String target = null;
    if ("construction".equals(kind) || "proposed".equals(kind)) {
      lifecycle = kind;
      target = Tags.clean(f.getString(kind));
      if (target != null && !KINDS.containsKey(target)) {
        return;
      }
    } else if (!"rail".equals(kind) && f.hasTag("construction") && !f.hasTag("construction", "no")) {
      lifecycle = "construction";
      target = kind;
      kind = "construction";
    }
    String service = Tags.clean(f.getString("service"));

    List<Records.Route> routes = new ArrayList<>();
    for (var m : memberships) {
      String role = m.role();
      if (role == null || !(role.startsWith("platform") || role.startsWith("stop"))) {
        routes.add(m.relation());
      }
    }
    if (service == null) {
      for (var track : trackLines) {
        if (track.rule().test(f)) {
          routes.add(track.route());
          break;
        }
      }
    }
    if ("proposed".equals(kind) && routes.isEmpty() && (target == null || !PROPOSED_MODES.contains(target))) {
      return;
    }
    routes.sort(Comparator.<Records.Route>comparingInt(r -> MODE_PRIORITY.getOrDefault(r.mode(), 9))
      .thenComparing(r -> r.ref() == null ? "~" : r.ref()));
    var refs = new LinkedHashSet<String>();
    routes.forEach(r -> {
      if (r.ref() != null) {
        refs.add(r.ref());
      }
    });
    Records.Route primary = routes.isEmpty() ? null : routes.getFirst();
    String colour = primary != null ? primary.colour()
      : "rail".equals(kind) ? null : ctx.region().modeColour(target != null ? target : kind);

    int minzoom = MINZOOM.getOrDefault(kind, 12);
    if (service != null) {
      minzoom = Math.max(minzoom, 12);
    }
    features.line(NAME)
      .setAttr("kind", kind)
      .setAttr("colour", colour)
      .setAttr("tone", tone(colour))
      .setAttrWithMinzoom("construction", "construction".equals(kind) ? target : null, 9)
      .setAttrWithMinzoom("proposed", "proposed".equals(kind) ? target : null, 10)
      .setAttrWithMinzoom("service", service, 12)
      .setAttrWithMinzoom("usage", Tags.clean(f.getString("usage")), 10)
      .setAttrWithMinzoom("ref", refs.isEmpty() ? null : String.join(",", refs), 9)
      .setAttrWithMinzoom("network", primary == null ? null : primary.network(), 11)
      .setAttrWithMinzoom("name", Tags.name(f), 13)
      .setAttrWithMinzoom("brunnel", Roads.brunnel(f), 11)
      .setAttrWithMinzoom("layer", Roads.layer(f), 11)
      .setMinZoom(minzoom)
      .setMinPixelSize(0)
      .setSortKey(primary == null ? 0 : 10 - MODE_PRIORITY.getOrDefault(primary.mode(), 9));

    if (!routes.isEmpty() && ctx.inBounds(f)) {
      var line = f.latLonGeometry();
      for (var route : routes) {
        ctx.sink().routeWays.add(new Records.RouteWay(route.id(), f.id(), line, lifecycle));
      }
    }
  }

  @Override
  public List<VectorTile.Feature> postProcess(int zoom, List<VectorTile.Feature> items) {
    return zoom < 14 ? FeatureMerge.mergeLineStrings(items, 0.5, 0.1, 4) : items;
  }

  /**
   * {@code dark} for line colours too dark to stand out on the dark map (indigo, deep purple) and {@code light} for
   * those too pale for the light map (yellows), from their WCAG relative luminance; null otherwise. Styles give such
   * lines a contrasting casing.
   */
  static String tone(String hex) {
    if (hex == null || !hex.matches("#[0-9a-f]{6}")) {
      return null;
    }
    double luminance = 0;
    double[] weights = {0.2126, 0.7152, 0.0722};
    for (int i = 0; i < 3; i++) {
      double c = Integer.parseInt(hex.substring(1 + i * 2, 3 + i * 2), 16) / 255.0;
      luminance += weights[i] * (c <= 0.03928 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4));
    }
    return luminance < 0.08 ? "dark" : luminance > 0.45 ? "light" : null;
  }
}
