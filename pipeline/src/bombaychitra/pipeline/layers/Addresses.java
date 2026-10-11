package bombaychitra.pipeline.layers;

import bombaychitra.pipeline.util.Tags;
import com.onthegomap.planetiler.FeatureCollector;
import com.onthegomap.planetiler.expression.Expression;
import com.onthegomap.planetiler.reader.SourceFeature;

/** {@code addresses} — house numbers at the highest zoom for door-level precision. */
public final class Addresses extends Layer {

  public static final String NAME = "addresses";

  public Addresses(LayerContext ctx) {
    super(ctx);
  }

  @Override
  public String name() {
    return NAME;
  }

  @Override
  public Expression filter() {
    return Expression.matchField("addr:housenumber");
  }

  @Override
  public void processFeature(SourceFeature f, FeatureCollector features) {
    String number = Tags.clean(f.getString("addr:housenumber"));
    if (number == null || number.length() > 12) {
      return;
    }
    FeatureCollector.Feature feature;
    if (f.isPoint()) {
      feature = features.point(NAME);
    } else if (f.canBePolygon()) {
      feature = features.pointOnSurface(NAME);
    } else {
      return;
    }
    feature.setAttr("housenumber", number).setMinZoom(maxzoom());
  }
}
