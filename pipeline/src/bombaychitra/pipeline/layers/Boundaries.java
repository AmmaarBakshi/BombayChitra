package bombaychitra.pipeline.layers;

import com.onthegomap.planetiler.FeatureCollector;
import com.onthegomap.planetiler.FeatureMerge;
import com.onthegomap.planetiler.ForwardingProfile;
import com.onthegomap.planetiler.VectorTile;
import com.onthegomap.planetiler.expression.Expression;
import com.onthegomap.planetiler.reader.SourceFeature;
import com.onthegomap.planetiler.reader.osm.OsmElement;
import com.onthegomap.planetiler.reader.osm.OsmRelationInfo;
import com.onthegomap.planetiler.util.Parse;
import java.util.List;

/**
 * {@code boundaries} — administrative boundary lines built from relation membership: state (4), district (5), taluka
 * (6), municipal corporation (8), BMC zone (9) and ward (10). Each way carries the lowest admin level it belongs to.
 */
public final class Boundaries extends Layer
  implements ForwardingProfile.OsmRelationPreprocessor, ForwardingProfile.LayerPostProcessor {

  public static final String NAME = "boundaries";

  private record AdminRelation(long id, int level) implements OsmRelationInfo {}

  public Boundaries(LayerContext ctx) {
    super(ctx);
  }

  @Override
  public String name() {
    return NAME;
  }

  @Override
  public List<OsmRelationInfo> preprocessOsmRelation(OsmElement.Relation relation) {
    if (!relation.hasTag("type", "boundary", "multipolygon") || !relation.hasTag("boundary", "administrative")) {
      return null;
    }
    Integer level = Parse.parseIntOrNull(relation.getString("admin_level"));
    if (level == null || level < 2 || level > 10) {
      return null;
    }
    return List.of(new AdminRelation(relation.id(), level));
  }

  @Override
  public void processFeature(SourceFeature f, FeatureCollector features) {
    if (!f.canBeLine() || f.isPoint() || !f.hasRelationInfo()) {
      return;
    }
    int level = Integer.MAX_VALUE;
    for (var member : f.relationInfo(AdminRelation.class)) {
      level = Math.min(level, member.relation().level());
    }
    if (level == Integer.MAX_VALUE) {
      return;
    }
    int minzoom = level <= 4 ? 4 : level == 5 ? 7 : level == 6 ? 9 : level <= 8 ? 10 : 12;
    features.line(NAME)
      .setAttr("admin_level", level)
      .setAttr("maritime", f.hasTag("maritime", "yes") || f.hasTag("natural", "coastline") ? 1 : null)
      .setMinZoom(minzoom)
      .setMinPixelSize(0);
  }

  @Override
  public Expression filter() {
    return Expression.TRUE;
  }

  @Override
  public List<VectorTile.Feature> postProcess(int zoom, List<VectorTile.Feature> items) {
    return FeatureMerge.mergeLineStrings(items, 0, 0.1, 4);
  }
}
