package bombaychitra.pipeline.layers;

import bombaychitra.pipeline.Taxonomy;
import bombaychitra.pipeline.data.Records;
import bombaychitra.pipeline.util.Geo;
import bombaychitra.pipeline.util.Overture;
import bombaychitra.pipeline.util.Tags;
import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.onthegomap.planetiler.FeatureCollector;
import com.onthegomap.planetiler.ForwardingProfile;
import com.onthegomap.planetiler.expression.Expression;
import com.onthegomap.planetiler.geo.GeometryException;
import com.onthegomap.planetiler.reader.SourceFeature;
import com.onthegomap.planetiler.reader.Struct;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.atomic.LongAdder;
import java.util.function.Consumer;
import java.util.regex.Pattern;
import org.locationtech.jts.geom.Envelope;
import org.locationtech.jts.index.strtree.STRtree;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * Adds <a href="https://docs.overturemaps.org/guides/places/">Overture Maps places</a> to the {@code pois} layer —
 * shops, clinics, schools and offices that OpenStreetMap does not have yet, which fills in fast-growing areas such as
 * Taloja, Kalamboli, Panvel and Vasai-Virar.
 *
 * <p>A place is kept when its confidence reaches {@code overture.places.minConfidence}, its category maps to the
 * taxonomy ({@code "overture"} lists in {@code categories.json}) and it does not duplicate an OpenStreetMap feature:
 * the same category within {@value #SAME_SPOT_M} m, or a similar name in the same group within {@value #NEAR_M} m.
 * OpenStreetMap always wins, so Overture only adds.
 */
public final class OverturePlaces extends Layer implements ForwardingProfile.FinishHandler {

  public static final String SOURCE = "overture_places";

  private static final Logger LOGGER = LoggerFactory.getLogger(OverturePlaces.class);
  private static final ObjectMapper JSON = new ObjectMapper();
  private static final double SAME_SPOT_M = 25;
  private static final Pattern ATM = Pattern.compile("\\bATM\\b", Pattern.CASE_INSENSITIVE);
  private static final double NEAR_M = 120;
  /** Duplicate Overture records (the same business from several sources) are merged within cells of this size. */
  private static final double DEDUPE_CELL_DEG = 0.002;

  private record Known(Records.Place place, Set<String> tokens) {}

  private final double minConfidence;
  private volatile STRtree osmPlaces;
  private final Map<String, Boolean> seen = new ConcurrentHashMap<>();
  private final LongAdder kept = new LongAdder();
  private final LongAdder duplicates = new LongAdder();

  public OverturePlaces(LayerContext ctx) {
    super(ctx);
    var overture = ctx.region().overture();
    this.minConfidence = overture == null || overture.places() == null ? 0.6 : overture.places().confidence();
  }

  @Override
  public String name() {
    return Pois.NAME;
  }

  @Override
  public Expression filter() {
    return Expression.matchSource(SOURCE);
  }

  @Override
  public void processFeature(SourceFeature f, FeatureCollector features) {
    if (!f.isPoint() || !ctx.inBounds(f)) {
      return;
    }
    Double confidence = f.getStruct("confidence").asDouble();
    if (confidence == null || confidence < minConfidence
      || "permanently_closed".equals(f.getStruct("operating_status").asString())) {
      return;
    }
    String name = Tags.clean(f.getStruct("names").get("primary").asString());
    if (name == null || name.length() < 2 || name.length() > 90 || name.chars().noneMatch(Character::isLetter)) {
      return;
    }
    Taxonomy.Category category = ctx.taxonomy().matchOverture(categories(f));
    if (category == null) {
      return;
    }
    // Overture files cash machines under banks; their names say which they are ("HDFC Bank ATM").
    if ("bank".equals(category.id()) && ATM.matcher(name).find() && ctx.taxonomy().category("atm") != null) {
      category = ctx.taxonomy().category("atm");
    }
    try {
      var anchor = ctx.anchor(f);
      Set<String> tokens = Overture.nameTokens(name);
      if (duplicatesOsm(category, tokens, anchor.x, anchor.y) || !firstOfItsKind(category, tokens, anchor.x, anchor.y)) {
        duplicates.increment();
        return;
      }
      String gers = f.getStruct("id").asString();
      long id = Overture.featureId(gers);
      features.point(Pois.NAME)
        .setId(id)
        .setAttr("category", category.id())
        .setAttr("group", category.group())
        .setAttr("rank", category.rank())
        .setAttr("name", name)
        .setAttr("name:mr", localName(f, "mr", name))
        .setAttr("name:hi", localName(f, "hi", name))
        .setMinZoom(Math.max(8, Math.min(category.minzoom(), maxzoom())))
        // OpenStreetMap features win label collisions.
        .setSortKey(category.rank() * 10 + 6)
        .setPointLabelGridSizeAndLimit(maxzoom() - 1, 24, 8)
        .setBufferPixels(64);
      ctx.sink().places.add(new Records.Place(id, category.id(), category.group(), name, anchor.x, anchor.y, 0,
        tagsJson(f, gers, name, confidence)));
      kept.increment();
    } catch (GeometryException e) {
      e.log("overture place");
    }
  }

  /** The place's common name in a language, when it differs from its primary name. */
  private static String localName(SourceFeature f, String lang, String name) {
    String value = Tags.clean(f.getStruct("names").get("common").get(lang).asString());
    return value == null || value.equals(name) ? null : value;
  }

  /** Overture categories from the most specific to the most general. */
  private static List<String> categories(SourceFeature f) {
    List<String> result = new ArrayList<>();
    result.add(f.getStruct("basic_category").asString());
    Struct taxonomy = f.getStruct("taxonomy");
    result.add(taxonomy.get("primary").asString());
    List<Struct> hierarchy = taxonomy.get("hierarchy").asList();
    for (int i = hierarchy.size() - 1; i >= 0; i--) {
      result.add(hierarchy.get(i).asString());
    }
    // Releases before the 2026 taxonomy carry `categories.primary` instead.
    result.add(f.getStruct("categories").get("primary").asString());
    return result;
  }

  private boolean duplicatesOsm(Taxonomy.Category category, Set<String> tokens, double lon, double lat) {
    STRtree index = osmIndex();
    Envelope probe = new Envelope(lon, lon, lat, lat);
    probe.expandBy(NEAR_M / 100_000);
    for (Object o : index.query(probe)) {
      var known = (Known) o;
      var place = known.place();
      double d = Geo.haversine(lon, lat, place.lon(), place.lat());
      if (place.category().equals(category.id()) && d <= SAME_SPOT_M) {
        return true;
      }
      if (place.group().equals(category.group()) && d <= NEAR_M && Overture.similar(tokens, known.tokens())) {
        return true;
      }
    }
    return false;
  }

  /** OpenStreetMap places are complete when the Overture source is read, so the index is built once, on first use. */
  private STRtree osmIndex() {
    STRtree index = osmPlaces;
    if (index == null) {
      synchronized (this) {
        index = osmPlaces;
        if (index == null) {
          index = new STRtree();
          for (var place : ctx.sink().places) {
            index.insert(new Envelope(place.lon(), place.lon(), place.lat(), place.lat()),
              new Known(place, Overture.nameTokens(place.name())));
          }
          index.build();
          osmPlaces = index;
        }
      }
    }
    return index;
  }

  /**
   * One place per category, name and grid cell. Only the cell itself is checked (not its neighbours) so the result
   * does not depend on the order in which threads read the file, and every build keeps the same number of places.
   */
  private boolean firstOfItsKind(Taxonomy.Category category, Set<String> tokens, double lon, double lat) {
    String nameKey = String.join(" ", tokens.stream().sorted().toList());
    long cx = (long) Math.floor(lon / DEDUPE_CELL_DEG);
    long cy = (long) Math.floor(lat / DEDUPE_CELL_DEG);
    return seen.putIfAbsent(category.id() + "|" + nameKey + "|" + cx + "|" + cy, Boolean.TRUE) == null;
  }

  /** OpenStreetMap-style tags so the database and API treat both sources alike; provenance under {@code overture:*}. */
  private static String tagsJson(SourceFeature f, String gers, String name, double confidence) {
    Map<String, Object> tags = new LinkedHashMap<>();
    tags.put("name", name);
    Struct common = f.getStruct("names").get("common");
    for (String lang : List.of("mr", "hi", "en", "gu")) {
      String value = Tags.clean(common.get(lang).asString());
      if (value != null && !value.equals(name)) {
        tags.put("name:" + lang, value);
      }
    }
    Struct address = f.getStruct("addresses").get(0);
    put(tags, "addr:full", address.get("freeform").asString());
    put(tags, "addr:city", address.get("locality").asString());
    put(tags, "addr:postcode", address.get("postcode").asString());
    put(tags, "phone", f.getStruct("phones").get(0).asString());
    put(tags, "website", f.getStruct("websites").get(0).asString());
    put(tags, "email", f.getStruct("emails").get(0).asString());
    put(tags, "brand", f.getStruct("brand").get("names").get("primary").asString());
    put(tags, "brand:wikidata", f.getStruct("brand").get("wikidata").asString());
    tags.put("source", "Overture Maps");
    tags.put("overture:id", gers);
    tags.put("overture:category", f.getStruct("basic_category").asString());
    tags.put("overture:confidence", Math.round(confidence * 100) / 100.0);
    Set<String> datasets = new LinkedHashSet<>();
    for (Struct source : f.getStruct("sources").asList()) {
      String dataset = source.get("dataset").asString();
      if (dataset != null && !"Overture".equals(dataset)) {
        datasets.add(dataset);
      }
    }
    if (!datasets.isEmpty()) {
      tags.put("overture:sources", String.join(";", datasets));
    }
    try {
      return JSON.writeValueAsString(tags);
    } catch (JsonProcessingException e) {
      throw new IllegalStateException(e);
    }
  }

  private static void put(Map<String, Object> tags, String key, String value) {
    String clean = Tags.clean(value);
    if (clean != null) {
      tags.put(key, clean);
    }
  }

  @Override
  public void finish(String sourceName, FeatureCollector.Factory factory, Consumer<FeatureCollector.Feature> emit) {
    if (SOURCE.equals(sourceName)) {
      LOGGER.info("Overture places: {} added, {} skipped as duplicates of OpenStreetMap or of each other", kept.sum(),
        duplicates.sum());
    }
  }
}
