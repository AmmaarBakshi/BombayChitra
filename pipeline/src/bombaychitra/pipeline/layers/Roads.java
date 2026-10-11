package bombaychitra.pipeline.layers;

import bombaychitra.pipeline.data.Records;
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

    if ((name != null || ref != null) && !"construction".equals(kind) && ctx.inBounds(f)) {
      try {
        ctx.sink().roads.add(new Records.Road(Tags.oid(f), name, ref, kind, highway, f.latLonGeometry(),
          Tags.round1(f.lengthMeters()), f.tagsAsJson()));
      } catch (GeometryException e) {
        e.log("road");
      }
    }
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
