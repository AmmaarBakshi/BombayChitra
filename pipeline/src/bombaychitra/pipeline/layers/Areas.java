package bombaychitra.pipeline.layers;

import bombaychitra.pipeline.RegionConfig;
import bombaychitra.pipeline.data.Records;
import bombaychitra.pipeline.util.Tags;
import com.onthegomap.planetiler.FeatureCollector;
import com.onthegomap.planetiler.ForwardingProfile;
import com.onthegomap.planetiler.expression.Expression;
import com.onthegomap.planetiler.geo.GeoUtils;
import com.onthegomap.planetiler.geo.GeometryException;
import com.onthegomap.planetiler.reader.SimpleFeature;
import com.onthegomap.planetiler.reader.SourceFeature;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.ConcurrentLinkedQueue;
import java.util.function.Consumer;
import org.locationtech.jts.geom.Geometry;
import org.locationtech.jts.operation.union.UnaryUnionOp;

/**
 * {@code areas} — the named areas from {@code region.json} (Mumbai City, Mumbai Suburban, MaKaBo, Thane, Navi Mumbai…)
 * as dissolved polygons with {@code id}, {@code name} and {@code kind}, for highlighting and filtering.
 */
public final class Areas extends Layer implements ForwardingProfile.FinishHandler {

  public static final String NAME = "areas";

  private final Map<Long, List<RegionConfig.Area>> byRelation = new HashMap<>();
  private final Map<String, ConcurrentLinkedQueue<Geometry>> worldParts = new ConcurrentHashMap<>();

  public Areas(LayerContext ctx) {
    super(ctx);
    if (ctx.region().areas() != null) {
      for (var area : ctx.region().areas()) {
        for (Long relation : area.relations()) {
          byRelation.computeIfAbsent(relation, k -> new ArrayList<>()).add(area);
        }
      }
    }
  }

  @Override
  public String name() {
    return NAME;
  }

  @Override
  public Expression filter() {
    return Expression.matchField("boundary");
  }

  @Override
  public void processFeature(SourceFeature f, FeatureCollector features) {
    if (Tags.typeCode(f) != 3 || !f.canBePolygon()) {
      return;
    }
    List<RegionConfig.Area> areas = byRelation.get(f.id());
    if (areas == null) {
      return;
    }
    try {
      Geometry world = f.polygon();
      Geometry latLon = f.latLonGeometry();
      for (var area : areas) {
        worldParts.computeIfAbsent(area.id(), k -> new ConcurrentLinkedQueue<>()).add(world);
        ctx.sink().areaParts.add(new Records.AreaPart(area.id(), f.id(), latLon));
      }
    } catch (GeometryException e) {
      e.log("area " + f.id());
    }
  }

  @Override
  public void finish(String sourceName, FeatureCollector.Factory factory, Consumer<FeatureCollector.Feature> emit) {
    if (!"osm".equals(sourceName) || ctx.region().areas() == null) {
      return;
    }
    for (var area : ctx.region().areas()) {
      var parts = worldParts.get(area.id());
      if (parts == null || parts.isEmpty()) {
        continue;
      }
      Geometry union = UnaryUnionOp.union(new ArrayList<>(parts));
      try {
        union = GeoUtils.fixPolygon(union);
      } catch (GeometryException e) {
        e.log("area union " + area.id());
      }
      FeatureCollector collector = factory.get(SimpleFeature.fromWorldGeometry(union));
      collector.polygon(NAME)
        .setAttr("id", area.id())
        .setAttr("name", area.name())
        .setAttr("kind", area.kind())
        .setZoomRange(4, maxzoom())
        .setMinPixelSize(0);
      for (var feature : collector) {
        emit.accept(feature);
      }
    }
  }
}
