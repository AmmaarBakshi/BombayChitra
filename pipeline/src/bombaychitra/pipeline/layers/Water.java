package bombaychitra.pipeline.layers;

import bombaychitra.pipeline.geo.Coastline;
import bombaychitra.pipeline.util.Tags;
import com.onthegomap.planetiler.FeatureCollector;
import com.onthegomap.planetiler.FeatureMerge;
import com.onthegomap.planetiler.ForwardingProfile;
import com.onthegomap.planetiler.VectorTile;
import com.onthegomap.planetiler.expression.Expression;
import com.onthegomap.planetiler.geo.GeometryException;
import com.onthegomap.planetiler.reader.SimpleFeature;
import com.onthegomap.planetiler.reader.SourceFeature;
import java.util.List;
import java.util.Map;
import java.util.function.Consumer;

/**
 * {@code water} — ocean (built from coastline), lakes, reservoirs, rivers, docks. Named water bodies also get a label
 * point in the {@code places} layer with {@code kind=water}.
 */
public final class Water extends Layer implements ForwardingProfile.FinishHandler, ForwardingProfile.LayerPostProcessor {

  public static final String NAME = "water";

  private static final Map<String, String> WATER_KINDS = Map.ofEntries(
    Map.entry("river", "river"), Map.entry("canal", "river"), Map.entry("stream", "river"),
    Map.entry("oxbow", "lake"), Map.entry("lake", "lake"), Map.entry("lagoon", "lagoon"),
    Map.entry("reservoir", "reservoir"), Map.entry("basin", "basin"), Map.entry("pond", "pond"),
    Map.entry("fishpond", "pond"), Map.entry("wastewater", "basin"), Map.entry("moat", "pond"),
    Map.entry("reflecting_pool", "pond"), Map.entry("tidal", "lagoon"), Map.entry("harbour", "dock")
  );

  private final Coastline coastline;

  public Water(LayerContext ctx) {
    super(ctx);
    this.coastline = new Coastline(ctx.region().envelope());
  }

  @Override
  public String name() {
    return NAME;
  }

  @Override
  public Expression filter() {
    return Expression.or(
      Expression.matchAny("natural", "water", "coastline"),
      Expression.matchAny("waterway", "riverbank", "dock"),
      Expression.matchAny("landuse", "reservoir", "basin")
    );
  }

  @Override
  public void processFeature(SourceFeature f, FeatureCollector features) {
    if (f.hasTag("natural", "coastline")) {
      if (f.canBeLine()) {
        try {
          coastline.add(f.line());
        } catch (GeometryException e) {
          e.log("coastline");
        }
      }
      return;
    }
    if (!f.canBePolygon() || f.hasTag("covered", "yes") || f.hasTag("location", "underground")) {
      return;
    }
    String kind = kind(f);
    features.polygon(NAME)
      .setAttr("kind", kind)
      .setMinZoom(4)
      .setMinPixelSize(2);

    String name = Tags.name(f);
    if (name != null) {
      try {
        double area = f.areaMeters();
        int minzoom = area > 2_000_000 ? 10 : area > 200_000 ? 12 : area > 20_000 ? 14 : 15;
        features.innermostPoint(Places.NAME)
          .setAttr("kind", "water")
          .setAttr("subkind", kind)
          .setAttr("name", name)
          .setAttr("name:mr", Tags.nameIn(f, "mr", name))
          .setAttr("name:hi", Tags.nameIn(f, "hi", name))
          .setAttr("rank", 6)
          .setMinZoom(Math.min(minzoom, maxzoom()));
      } catch (GeometryException e) {
        e.log("water label");
      }
    }
  }

  private static String kind(SourceFeature f) {
    if (f.hasTag("waterway", "riverbank")) {
      return "river";
    }
    if (f.hasTag("waterway", "dock")) {
      return "dock";
    }
    if (f.hasTag("landuse", "reservoir")) {
      return "reservoir";
    }
    if (f.hasTag("landuse", "basin")) {
      return "basin";
    }
    String water = f.getString("water");
    return water == null ? "lake" : WATER_KINDS.getOrDefault(water, "lake");
  }

  @Override
  public void finish(String sourceName, FeatureCollector.Factory factory, Consumer<FeatureCollector.Feature> emit) {
    if (!"osm".equals(sourceName)) {
      return;
    }
    for (var polygon : coastline.buildOcean()) {
      FeatureCollector collector = factory.get(SimpleFeature.fromWorldGeometry(polygon));
      collector.polygon(NAME)
        .setAttr("kind", "ocean")
        .setZoomRange(0, maxzoom())
        .setMinPixelSize(0);
      for (var feature : collector) {
        emit.accept(feature);
      }
    }
  }

  @Override
  public List<VectorTile.Feature> postProcess(int zoom, List<VectorTile.Feature> items) throws GeometryException {
    return zoom < maxzoom() ? FeatureMerge.mergeOverlappingPolygons(items, 1) : items;
  }
}
