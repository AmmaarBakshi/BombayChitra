package bombaychitra.pipeline.layers;

import bombaychitra.pipeline.util.Tags;
import com.onthegomap.planetiler.FeatureCollector;
import com.onthegomap.planetiler.expression.Expression;
import com.onthegomap.planetiler.reader.SourceFeature;

/** {@code aeroway} — runways and taxiways (lines), aprons, helipads and runway areas (polygons). */
public final class Aeroways extends Layer {

  public static final String NAME = "aeroway";

  public Aeroways(LayerContext ctx) {
    super(ctx);
  }

  @Override
  public String name() {
    return NAME;
  }

  @Override
  public Expression filter() {
    return Expression.matchAny("aeroway", "runway", "taxiway", "apron", "helipad", "stopway");
  }

  @Override
  public void processFeature(SourceFeature f, FeatureCollector features) {
    String kind = f.getString("aeroway");
    boolean areaKind = "apron".equals(kind) || "helipad".equals(kind) || f.hasTag("area", "yes");
    if (areaKind && f.canBePolygon()) {
      features.polygon(NAME).setAttr("kind", kind).setMinZoom(11).setMinPixelSize(2);
    } else if (f.canBeLine() && !f.isPoint()) {
      features.line(NAME)
        .setAttr("kind", kind)
        .setAttrWithMinzoom("ref", Tags.clean(f.getString("ref")), 12)
        .setMinZoom("runway".equals(kind) ? 9 : 12)
        .setMinPixelSize(0);
    }
  }
}
