package bombaychitra.pipeline.layers;

import bombaychitra.pipeline.data.Records;
import bombaychitra.pipeline.util.Tags;
import com.onthegomap.planetiler.FeatureCollector;
import com.onthegomap.planetiler.expression.Expression;
import com.onthegomap.planetiler.geo.GeometryException;
import com.onthegomap.planetiler.reader.SourceFeature;
import java.util.Set;

/**
 * {@code bridges} — every road, rail, metro and foot bridge plus bridge outline polygons ({@code man_made=bridge}), with
 * computed length. Long bridges (sea links, creek crossings, flyovers) appear from low zooms so the bridge network can
 * be explored on its own.
 */
public final class Bridges extends Layer {

  public static final String NAME = "bridges";

  private static final Set<String> FOOT = Set.of("footway", "path", "pedestrian", "steps", "cycleway", "bridleway",
    "corridor", "track");
  private static final Set<String> METRO = Set.of("subway", "light_rail", "monorail", "tram");
  private static final Set<String> RAIL = Set.of("rail", "narrow_gauge", "preserved", "funicular");

  public Bridges(LayerContext ctx) {
    super(ctx);
  }

  @Override
  public String name() {
    return NAME;
  }

  @Override
  public Expression filter() {
    return Expression.or(Expression.matchField("bridge"), Expression.matchAny("man_made", "bridge"));
  }

  @Override
  public void processFeature(SourceFeature f, FeatureCollector features) {
    try {
      if (f.hasTag("man_made", "bridge") && f.canBePolygon()) {
        outline(f, features);
      } else if (Tags.isYes(f, "bridge") && f.canBeLine() && !f.hasTag("area", "yes")) {
        way(f, features);
      }
    } catch (GeometryException e) {
      e.log("bridge");
    }
  }

  private void way(SourceFeature f, FeatureCollector features) throws GeometryException {
    String highway = f.getString("highway");
    String railway = f.getString("railway");
    String cls;
    String kind;
    if (highway != null) {
      cls = highway;
      kind = FOOT.contains(highway) ? "foot" : "construction".equals(highway) ? "construction" : "road";
    } else if (railway != null) {
      cls = railway;
      kind = METRO.contains(railway) ? "metro" : RAIL.contains(railway) ? "rail"
        : "construction".equals(railway) ? "construction" : null;
    } else if (f.hasTag("waterway")) {
      cls = f.getString("waterway");
      kind = "aqueduct";
    } else {
      return;
    }
    if (kind == null) {
      return;
    }
    double length = f.lengthMeters();
    String name = Tags.first(f, "bridge:name:en", "bridge:name", "name:en", "name");
    String structure = f.hasTag("bridge", "yes") ? Tags.clean(f.getString("bridge:structure"))
      : f.getString("bridge");
    int minzoom = length >= 2000 ? 8 : length >= 500 ? 10 : length >= 100 ? 12 : 13;
    if ("foot".equals(kind)) {
      minzoom = Math.max(minzoom, 14);
    }

    features.line(NAME)
      .setAttr("kind", kind)
      .setAttr("class", cls)
      .setAttr("length", (int) Math.round(length))
      .setAttrWithMinzoom("name", name, Math.min(minzoom + 2, 13))
      .setAttrWithMinzoom("structure", structure, 13)
      .setAttrWithMinzoom("layer", Roads.layer(f), 12)
      .setAttrWithMinzoom("ref", Tags.clean(f.getString("ref")), 12)
      .setMinZoom(minzoom)
      .setMinPixelSize(0)
      .setSortKey(f.getWayZorder());

    if (ctx.inBounds(f)) {
      ctx.sink().bridges.add(new Records.Bridge(Tags.oid(f), name, kind, cls, f.latLonGeometry(),
        Tags.round1(length), 0, f.tagsAsJson()));
    }
  }

  private void outline(SourceFeature f, FeatureCollector features) throws GeometryException {
    double area = f.areaMeters();
    String name = Tags.first(f, "bridge:name:en", "bridge:name", "name:en", "name");
    features.polygon(NAME)
      .setAttr("kind", "outline")
      .setAttrWithMinzoom("name", name, 14)
      .setMinZoom(area > 50_000 ? 11 : 13)
      .setMinPixelSize(1);
    if (ctx.inBounds(f)) {
      ctx.sink().bridges.add(new Records.Bridge(Tags.oid(f), name, "outline", "man_made", f.latLonGeometry(), 0,
        Tags.round1(area), f.tagsAsJson()));
    }
  }
}
