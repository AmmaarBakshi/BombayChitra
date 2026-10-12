package bombaychitra.pipeline.layers;

import bombaychitra.pipeline.data.Records;
import bombaychitra.pipeline.data.RoadGraph;
import bombaychitra.pipeline.util.Tags;
import com.onthegomap.planetiler.FeatureCollector;
import com.onthegomap.planetiler.FeatureMerge;
import com.onthegomap.planetiler.ForwardingProfile;
import com.onthegomap.planetiler.VectorTile;
import com.onthegomap.planetiler.expression.Expression;
import com.onthegomap.planetiler.geo.GeometryException;
import com.onthegomap.planetiler.reader.SourceFeature;
import com.onthegomap.planetiler.util.Parse;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * {@code roads} — the full road and path network. {@code kind} is one of motorway, trunk, primary, secondary,
 * tertiary, minor, service, pedestrian, track, path, construction. Named or numbered roads are stored in the database
 * for search and reverse geocoding.
 */
public final class Roads extends Layer implements ForwardingProfile.LayerPostProcessor {

  public static final String NAME = "roads";

  private static final Map<String, String> KINDS = Map.ofEntries(
    Map.entry("motorway", "motorway"), Map.entry("motorway_link", "motorway"),
    Map.entry("trunk", "trunk"), Map.entry("trunk_link", "trunk"),
    Map.entry("primary", "primary"), Map.entry("primary_link", "primary"),
    Map.entry("secondary", "secondary"), Map.entry("secondary_link", "secondary"),
    Map.entry("tertiary", "tertiary"), Map.entry("tertiary_link", "tertiary"),
    Map.entry("unclassified", "minor"), Map.entry("residential", "minor"), Map.entry("living_street", "minor"),
    Map.entry("road", "minor"), Map.entry("busway", "minor"),
    Map.entry("service", "service"),
    Map.entry("pedestrian", "pedestrian"),
    Map.entry("track", "track"),
    Map.entry("footway", "path"), Map.entry("path", "path"), Map.entry("steps", "path"),
    Map.entry("cycleway", "path"), Map.entry("bridleway", "path"), Map.entry("corridor", "path"),
    Map.entry("construction", "construction")
  );

  private static final Map<String, Integer> MINZOOM = Map.ofEntries(
    Map.entry("motorway", 5), Map.entry("trunk", 6), Map.entry("primary", 8), Map.entry("secondary", 9),
    Map.entry("tertiary", 10), Map.entry("minor", 12), Map.entry("service", 13), Map.entry("pedestrian", 13),
    Map.entry("track", 13), Map.entry("path", 14), Map.entry("construction", 12)
  );

  private static final Set<String> MAJOR = Set.of("motorway", "trunk", "primary", "secondary");

  /**
   * Roads cars can use, with typical urban speeds in km/h for the routing graph. They are deliberately below most
   * speed limits: the estimate has no live traffic, and Mumbai traffic rarely allows the limit.
   */
  private static final Map<String, Float> DRIVE_SPEED = Map.ofEntries(
    Map.entry("motorway", 65f), Map.entry("motorway_link", 40f), Map.entry("trunk", 45f), Map.entry("trunk_link", 30f),
    Map.entry("primary", 35f), Map.entry("primary_link", 28f), Map.entry("secondary", 30f),
    Map.entry("secondary_link", 24f), Map.entry("tertiary", 26f), Map.entry("tertiary_link", 22f),
    Map.entry("unclassified", 22f), Map.entry("residential", 18f), Map.entry("road", 18f),
    Map.entry("living_street", 10f), Map.entry("service", 12f)
  );
  /** Road class codes stored in the routing graph (see docs/database.md). */
  private static final Map<String, Byte> GRAPH_CLASS = Map.of("motorway", (byte) 1, "trunk", (byte) 2, "primary",
    (byte) 3, "secondary", (byte) 4, "tertiary", (byte) 5, "minor", (byte) 6, "service", (byte) 7, "pedestrian",
    (byte) 8, "track", (byte) 9, "path", (byte) 10);
  /**
   * Ways people may walk on unless tagged otherwise. Motorways (expressways, sea links, freeways) are closed to
   * pedestrians in India, as they are to two-wheelers; busways are for buses.
   */
  private static final Set<String> WALKABLE = Set.of("trunk", "trunk_link", "primary", "primary_link", "secondary",
    "secondary_link", "tertiary", "tertiary_link", "unclassified", "residential", "road", "living_street", "service",
    "pedestrian", "track", "footway", "path", "steps", "cycleway", "bridleway", "corridor");
  private static final Set<String> ALLOWED = Set.of("yes", "designated", "permissive", "destination", "customers",
    "delivery");

  public Roads(LayerContext ctx) {
    super(ctx);
  }

  @Override
  public String name() {
    return NAME;
  }

  @Override
  public Expression filter() {
    return Expression.matchAny("highway", KINDS.keySet().toArray());
  }

  @Override
  public void processFeature(SourceFeature f, FeatureCollector features) {
    if (!f.canBeLine() || f.hasTag("area", "yes")) {
      return;
    }
    String highway = f.getString("highway");
    String kind = KINDS.get(highway);
    if (kind == null) {
      return;
    }
    boolean link = highway.endsWith("_link");
    String construction = "construction".equals(kind) ? f.getString("construction") : null;
    String brunnel = brunnel(f);
    String name = Tags.name(f);
    String ref = Tags.clean(f.getString("ref"));
    int minzoom = MINZOOM.getOrDefault(kind, 14);
    if (link) {
      minzoom = Math.max(minzoom, 12);
    }
    if ("construction".equals(kind) && construction != null && KINDS.containsKey(construction)) {
      minzoom = Math.max(MINZOOM.getOrDefault(KINDS.get(construction), 13), 11);
    }

    features.line(NAME)
      .setAttr("kind", kind)
      .setAttrWithMinzoom("link", link ? 1 : null, 10)
      .setAttrWithMinzoom("highway", highway, 13)
      .setAttrWithMinzoom("construction", construction, 12)
      .setAttrWithMinzoom("name", name, MAJOR.contains(kind) ? 11 : 13)
      .setAttrWithMinzoom("ref", ref, 9)
      .setAttrWithMinzoom("brunnel", brunnel, 11)
      .setAttrWithMinzoom("layer", layer(f), 11)
      .setAttrWithMinzoom("oneway", oneway(f), 14)
      .setAttrWithMinzoom("service", f.getString("service"), 14)
      .setAttrWithMinzoom("access", access(f), 14)
      .setAttrWithMinzoom("lanes", Parse.parseIntOrNull(f.getString("lanes")), maxzoom())
      .setAttrWithMinzoom("maxspeed", Tags.clean(f.getString("maxspeed")), maxzoom())
      .setAttrWithMinzoom("surface", Tags.clean(f.getString("surface")), maxzoom())
      .setAttrWithMinzoom("toll", Tags.isYes(f, "toll") ? 1 : null, maxzoom())
      .setMinZoom(minzoom)
      .setMinPixelSize(0)
      .setSortKey(f.getWayZorder());

    byte modes = modes(f, highway);
    if (modes != 0 && ctx.inBounds(f)) {
      try {
        graphWay(f, kind, highway, modes, name != null ? name : ref);
      } catch (GeometryException e) {
        e.log("graph way");
      }
    }

    if ((name != null || ref != null) && !"construction".equals(kind) && ctx.inBounds(f)) {
      try {
        ctx.sink().roads.add(new Records.Road(Tags.oid(f), name, ref, kind, highway, f.latLonGeometry(),
          Tags.round1(f.lengthMeters()), f.tagsAsJson()));
      } catch (GeometryException e) {
        e.log("road");
      }
    }
  }

  /** The travel modes ({@link RoadGraph} bits) that may use a way. */
  static byte modes(SourceFeature f, String highway) {
    byte modes = 0;
    boolean motorway = highway.startsWith("motorway");
    boolean emergency = f.hasTag("service", "emergency_access");
    if (DRIVE_SPEED.containsKey(highway)) {
      if (permits(f, !emergency, "motorcar", "motor_vehicle", "vehicle", "access")) {
        modes |= RoadGraph.CAR;
      }
      // Two-wheelers are banned from expressways, sea links and freeways (OSM marks most with motorcycle=no too).
      if (permits(f, !emergency && !motorway, "motorcycle", "motor_vehicle", "vehicle", "access")) {
        modes |= RoadGraph.TWO_WHEELER;
      }
    }
    if ((WALKABLE.contains(highway) || motorway) && permits(f, !motorway, "foot", "access")) {
      modes |= RoadGraph.FOOT;
    }
    return modes;
  }

  /** The first of {@code keys} that is tagged decides; untagged ways get {@code fallback}. */
  private static boolean permits(SourceFeature f, boolean fallback, String... keys) {
    for (String key : keys) {
      String value = Tags.clean(f.getString(key));
      if (value != null) {
        return ALLOWED.contains(value);
      }
    }
    return fallback;
  }

  private void graphWay(SourceFeature f, String kind, String highway, byte modes, String name)
    throws GeometryException {
    var coords = f.latLonGeometry().getCoordinates();
    if (coords.length < 2) {
      return;
    }
    int[] packed = new int[coords.length * 2];
    for (int i = 0; i < coords.length; i++) {
      packed[i * 2] = (int) Math.round(coords[i].x * 1e7);
      packed[i * 2 + 1] = (int) Math.round(coords[i].y * 1e7);
    }
    Integer oneway = oneway(f);
    float speed = 0;
    if ((modes & (RoadGraph.CAR | RoadGraph.TWO_WHEELER)) != 0) {
      speed = DRIVE_SPEED.get(highway);
      if (Parse.parseIntOrNull(f.getString("maxspeed")) instanceof Integer max && max > 0) {
        speed = Math.min(speed, max);
      }
      if (f.hasTag("service", "parking_aisle", "drive-through", "driveway")) {
        speed = Math.min(speed, 8f);
      }
    }
    ctx.sink().graphWays.add(new Records.GraphWay(packed, GRAPH_CLASS.getOrDefault(kind, (byte) 6),
      (byte) (oneway == null ? 0 : oneway), speed, modes, flags(f, highway), name == null ? null : name.intern()));
  }

  /** Routing segment flags: tolls (Atal Setu, the sea link, expressways), steps, structures, roundabouts, ramps. */
  static byte flags(SourceFeature f, String highway) {
    int flags = 0;
    if (Tags.isYes(f, "toll") || Tags.isYes(f, "toll:motorcar")) {
      flags |= RoadGraph.TOLL;
    }
    if ("steps".equals(highway)) {
      flags |= RoadGraph.STEPS;
    }
    String brunnel = brunnel(f);
    if ("bridge".equals(brunnel)) {
      flags |= RoadGraph.BRIDGE;
    } else if ("tunnel".equals(brunnel)) {
      flags |= RoadGraph.TUNNEL;
    }
    if (f.hasTag("junction", "roundabout", "circular")) {
      flags |= RoadGraph.ROUNDABOUT;
    }
    if (highway.endsWith("_link")) {
      flags |= RoadGraph.LINK;
    }
    return (byte) flags;
  }

  static String brunnel(SourceFeature f) {
    if (Tags.isYes(f, "bridge")) {
      return "bridge";
    }
    if (Tags.isYes(f, "tunnel") || f.hasTag("covered", "yes")) {
      return "tunnel";
    }
    return null;
  }

  static Integer layer(SourceFeature f) {
    Integer layer = Parse.parseIntOrNull(f.getString("layer"));
    return layer == null || layer == 0 ? null : Math.max(-5, Math.min(5, layer));
  }

  private static Integer oneway(SourceFeature f) {
    int direction = f.getDirection("oneway");
    if (direction == 0 && (f.hasTag("junction", "roundabout") || f.hasTag("highway", "motorway"))) {
      direction = 1;
    }
    return direction == 0 ? null : direction;
  }

  private static String access(SourceFeature f) {
    String access = f.getString("access");
    return access != null && (access.equals("private") || access.equals("no")) ? access : null;
  }

  @Override
  public List<VectorTile.Feature> postProcess(int zoom, List<VectorTile.Feature> items) {
    // Merging may reverse line direction, so stop before oneway arrows appear at z14.
    return zoom < 14 ? FeatureMerge.mergeLineStrings(items, 0.5, 0.1, 4) : items;
  }
}
