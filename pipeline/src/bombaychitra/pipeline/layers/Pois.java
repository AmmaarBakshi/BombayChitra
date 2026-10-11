package bombaychitra.pipeline.layers;

import bombaychitra.pipeline.Taxonomy;
import bombaychitra.pipeline.data.Records;
import bombaychitra.pipeline.util.Tags;
import com.onthegomap.planetiler.FeatureCollector;
import com.onthegomap.planetiler.expression.Expression;
import com.onthegomap.planetiler.geo.GeometryException;
import com.onthegomap.planetiler.reader.SourceFeature;

/**
 * {@code pois} — every feature classified by {@code config/categories.json}: hospitals, schools, colleges, banks,
 * stations, bus stops, cafés, restaurants, hotels, grounds, clubs and ~90 other categories. Tiles carry only what the
 * map needs to draw ({@code category}, {@code group}, {@code name}, {@code rank}); full details live in the database
 * keyed by the feature id.
 */
public final class Pois extends Layer {

  public static final String NAME = "pois";

  public Pois(LayerContext ctx) {
    super(ctx);
  }

  @Override
  public String name() {
    return NAME;
  }

  @Override
  public Expression filter() {
    return Expression.or(ctx.taxonomy().triggerKeys().stream().map(Expression::matchField).toList());
  }

  @Override
  public void processFeature(SourceFeature f, FeatureCollector features) {
    Taxonomy.Category category = ctx.taxonomy().match(f);
    if (category == null) {
      return;
    }
    // Skip linear features that are not areas (e.g. a pitch drawn as an unclosed way): they have no sensible anchor.
    if (!f.isPoint() && !f.canBePolygon()) {
      return;
    }
    try {
      double area = f.isPoint() ? 0 : f.areaMeters();
      int minzoom = category.minzoom();
      if (area > 500_000) {
        minzoom -= 2;
      } else if (area > 50_000) {
        minzoom -= 1;
      }
      minzoom = Math.max(8, Math.min(minzoom, maxzoom()));
      String name = Tags.name(f);

      FeatureCollector.Feature feature = f.isPoint() ? features.point(NAME) : features.innermostPoint(NAME);
      feature
        .setAttr("category", category.id())
        .setAttr("group", category.group())
        .setAttr("rank", category.rank())
        .setAttr("name", name)
        .setMinZoom(minzoom)
        .setSortKey(category.rank() * 10 + (name == null ? 5 : 0))
        .setPointLabelGridSizeAndLimit(maxzoom() - 1, 24, 8)
        .setBufferPixels(64);

      if (ctx.inBounds(f)) {
        var anchor = ctx.anchor(f);
        ctx.sink().places.add(new Records.Place(Tags.oid(f), category.id(), category.group(), name, anchor.x, anchor.y,
          Tags.round1(area), f.tagsAsJson()));
      }
    } catch (GeometryException e) {
      e.log("poi");
    }
  }
}
