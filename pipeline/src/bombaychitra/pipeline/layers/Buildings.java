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
import java.util.List;
import java.util.Set;

/**
 * {@code buildings} — footprints and building parts with render heights for 3D extrusion. Buildings carrying useful
 * attributes (name, height, levels, address or a specific type) are also stored in the database.
 */
public final class Buildings extends Layer implements ForwardingProfile.LayerPostProcessor {

  public static final String NAME = "buildings";

  private static final Set<String> GENERIC = Set.of("yes", "building", "roof", "construction");

  public Buildings(LayerContext ctx) {
    super(ctx);
  }

  @Override
  public String name() {
    return NAME;
  }

  @Override
  public Expression filter() {
    return Expression.or(Expression.matchField("building"), Expression.matchField("building:part"));
  }

  @Override
  public void processFeature(SourceFeature f, FeatureCollector features) {
    if (!f.canBePolygon() || f.hasTag("location", "underground")) {
      return;
    }
    boolean part = !f.hasTag("building") && Tags.isYes(f, "building:part");
    if (!part && Tags.isNo(f, "building")) {
      return;
    }
    Double height = Tags.height(f);
    Double minHeight = Tags.minHeight(f);
    Integer levels = Tags.levels(f);
    String type = part ? "part" : f.getString("building");
    String kind = type == null || GENERIC.contains(type) && !"roof".equals(type) ? null : type;

    features.polygon(NAME)
      .setMinZoom(13)
      .setMinPixelSize(2)
      .setAttrWithMinzoom("height", height, 14)
      .setAttrWithMinzoom("min_height", minHeight, 14)
      .setAttrWithMinzoom("levels", levels, 14)
      .setAttrWithMinzoom("kind", kind, 14);

    if (part || !ctx.inBounds(f)) {
      return;
    }
    try {
      var anchor = ctx.anchor(f);
      double area = f.areaMeters();
      ctx.sink().addBuildingStat(anchor.x, anchor.y, height == null ? Double.NaN : height, area);
      String name = Tags.name(f);
      String address = Tags.address(f);
      if (name != null || height != null || levels != null || address != null || (kind != null && !"roof".equals(kind))) {
        ctx.sink().buildings.add(new Records.Building(Tags.oid(f), name, kind, levels, height, minHeight,
          Tags.round1(area), anchor.x, anchor.y, f.tagsAsJson()));
      }
    } catch (GeometryException e) {
      e.log("building");
    }
  }

  @Override
  public List<VectorTile.Feature> postProcess(int zoom, List<VectorTile.Feature> items) throws GeometryException {
    return zoom == 13 ? FeatureMerge.mergeNearbyPolygons(items, 3, 3, 0.5, 0.5) : items;
  }
}
