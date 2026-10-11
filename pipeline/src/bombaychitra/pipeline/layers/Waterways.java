package bombaychitra.pipeline.layers;

import bombaychitra.pipeline.util.Tags;
import com.onthegomap.planetiler.FeatureCollector;
import com.onthegomap.planetiler.FeatureMerge;
import com.onthegomap.planetiler.ForwardingProfile;
import com.onthegomap.planetiler.VectorTile;
import com.onthegomap.planetiler.expression.Expression;
import com.onthegomap.planetiler.reader.SourceFeature;
import java.util.List;
import java.util.Map;

/** {@code waterway} — rivers, creeks, canals, streams and nallahs (drains) as lines. */
public final class Waterways extends Layer implements ForwardingProfile.LayerPostProcessor {

  public static final String NAME = "waterway";

  private static final Map<String, Integer> MINZOOM = Map.of(
    "river", 8, "canal", 10, "tidal_channel", 11, "stream", 12, "brook", 12, "drain", 13, "ditch", 14
  );

  public Waterways(LayerContext ctx) {
    super(ctx);
  }

  @Override
  public String name() {
    return NAME;
  }

  @Override
  public Expression filter() {
    return Expression.matchAny("waterway", MINZOOM.keySet().toArray());
  }

  @Override
  public void processFeature(SourceFeature f, FeatureCollector features) {
    if (!f.canBeLine()) {
      return;
    }
    String waterway = f.getString("waterway");
    String kind = "brook".equals(waterway) ? "stream" : waterway;
    boolean tunnel = f.hasTag("tunnel", "culvert", "yes") || f.hasTag("covered", "yes");
    features.line(NAME)
      .setAttr("kind", kind)
      .setAttrWithMinzoom("name", Tags.name(f), 12)
      .setAttrWithMinzoom("tunnel", tunnel ? 1 : null, 12)
      .setAttrWithMinzoom("intermittent", Tags.isYes(f, "intermittent") ? 1 : null, 13)
      .setMinZoom(MINZOOM.getOrDefault(waterway, 13))
      .setMinPixelSize(0);
  }

  @Override
  public List<VectorTile.Feature> postProcess(int zoom, List<VectorTile.Feature> items) {
    return zoom < maxzoom() ? FeatureMerge.mergeLineStrings(items, 0.5, 0.1, 4) : items;
  }
}
