package bombaychitra.pipeline.layers;

import bombaychitra.pipeline.util.Tags;
import com.onthegomap.planetiler.FeatureCollector;
import com.onthegomap.planetiler.expression.Expression;
import com.onthegomap.planetiler.geo.GeometryException;
import com.onthegomap.planetiler.reader.SourceFeature;
import com.onthegomap.planetiler.reader.Struct;

/**
 * Adds the building footprints that <a href="https://docs.overturemaps.org/guides/buildings/">Overture Maps</a> has
 * and OpenStreetMap does not — machine-learned outlines from Google Open Buildings and Microsoft ML Buildings — to the
 * {@code buildings} layer. Overture already conflates them with OpenStreetMap, so its OpenStreetMap-derived records are
 * skipped and OpenStreetMap stays the source of truth wherever it has a building.
 */
public final class OvertureBuildings extends Layer {

  public static final String SOURCE = "overture_buildings";

  public OvertureBuildings(LayerContext ctx) {
    super(ctx);
  }

  @Override
  public String name() {
    return Buildings.NAME;
  }

  @Override
  public Expression filter() {
    return Expression.matchSource(SOURCE);
  }

  @Override
  public void processFeature(SourceFeature f, FeatureCollector features) {
    if (!f.canBePolygon() || Boolean.TRUE.equals(f.getStruct("is_underground").asBoolean())) {
      return;
    }
    for (Struct source : f.getStruct("sources").asList()) {
      if ("OpenStreetMap".equals(source.get("dataset").asString())) {
        return;
      }
    }
    Integer floors = f.getStruct("num_floors").asInt();
    if (floors != null && (floors <= 0 || floors > Tags.MAX_LEVELS)) {
      floors = null;
    }
    Double height = f.getStruct("height").asDouble();
    if (height != null && (height <= 0 || height > Tags.MAX_HEIGHT_M)) {
      height = null;
    }
    if (height == null && floors != null) {
      height = Tags.round1(floors * Tags.LEVEL_HEIGHT_M);
    }
    features.polygon(Buildings.NAME)
      .setMinZoom(13)
      .setMinPixelSize(2)
      .setAttrWithMinzoom("height", height == null ? null : Tags.round1(height), 14)
      .setAttrWithMinzoom("levels", floors, 14);

    if (ctx.inBounds(f)) {
      try {
        var anchor = ctx.anchor(f);
        ctx.sink().addBuildingStat(anchor.x, anchor.y, height == null ? Double.NaN : height, f.areaMeters());
      } catch (GeometryException e) {
        e.log("overture building");
      }
    }
  }
}
