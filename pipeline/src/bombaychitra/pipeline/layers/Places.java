package bombaychitra.pipeline.layers;

import bombaychitra.pipeline.data.Records;
import bombaychitra.pipeline.util.Tags;
import com.onthegomap.planetiler.FeatureCollector;
import com.onthegomap.planetiler.expression.Expression;
import com.onthegomap.planetiler.geo.GeometryException;
import com.onthegomap.planetiler.reader.SourceFeature;
import com.onthegomap.planetiler.util.Parse;
import java.util.Map;

/**
 * {@code places} — labels for settlements and localities (city, town, suburb, quarter, neighbourhood, village, hamlet,
 * locality, island). Water bodies add {@code kind=water} labels to this layer from {@link Water}. Localities are also
 * stored in the database so they are searchable.
 */
public final class Places extends Layer {

  public static final String NAME = "places";

  private record Spec(int minzoom, int rank) {}

  private static final Map<String, Spec> KINDS = Map.of(
    "city", new Spec(3, 1),
    "town", new Spec(8, 2),
    "suburb", new Spec(10, 3),
    "quarter", new Spec(11, 4),
    "village", new Spec(11, 4),
    "neighbourhood", new Spec(12, 5),
    "hamlet", new Spec(13, 6),
    "locality", new Spec(14, 6),
    "island", new Spec(9, 3),
    "islet", new Spec(13, 6)
  );

  public Places(LayerContext ctx) {
    super(ctx);
  }

  @Override
  public String name() {
    return NAME;
  }

  @Override
  public Expression filter() {
    return Expression.matchAny("place", KINDS.keySet().toArray());
  }

  @Override
  public void processFeature(SourceFeature f, FeatureCollector features) {
    String kind = f.getString("place");
    Spec spec = KINDS.get(kind);
    String name = Tags.name(f);
    if (spec == null || name == null) {
      return;
    }
    if (!f.isPoint() && !f.canBePolygon()) {
      return;
    }
    Long population = Parse.parseLongOrNull(f.getString("population"));
    int minzoom = spec.minzoom();
    if (population != null && population > 1_000_000 && minzoom > 3) {
      minzoom = Math.max(3, minzoom - 3);
    }
    FeatureCollector.Feature feature = f.isPoint() ? features.point(NAME) : features.pointOnSurface(NAME);
    feature
      .setAttr("kind", kind)
      .setAttr("name", name)
      .setAttr("name:mr", Tags.nameIn(f, "mr", name))
      .setAttr("name:hi", Tags.nameIn(f, "hi", name))
      .setAttr("rank", spec.rank())
      .setAttrWithMinzoom("population", population, 8)
      .setMinZoom(Math.min(minzoom, maxzoom()))
      .setSortKey(spec.rank())
      .setBufferPixels(128);

    if (ctx.inBounds(f)) {
      try {
        var anchor = ctx.anchor(f);
        ctx.sink().localities.add(new Records.Locality(Tags.oid(f), name, kind, anchor.x, anchor.y, population,
          f.tagsAsJson()));
      } catch (GeometryException e) {
        e.log("place");
      }
    }
  }
}
