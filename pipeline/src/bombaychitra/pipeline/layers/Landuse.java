package bombaychitra.pipeline.layers;

import com.onthegomap.planetiler.FeatureCollector;
import com.onthegomap.planetiler.FeatureMerge;
import com.onthegomap.planetiler.ForwardingProfile;
import com.onthegomap.planetiler.VectorTile;
import com.onthegomap.planetiler.expression.Expression;
import com.onthegomap.planetiler.geo.GeometryException;
import com.onthegomap.planetiler.reader.SourceFeature;
import java.util.List;
import java.util.Map;

/**
 * {@code landuse} — land cover and land use polygons with a normalised {@code kind}: mangrove, wetland, salt_pan,
 * forest, park, protected, grass, farmland, beach, rock, residential, commercial, industrial, port, railway, military,
 * cemetery, education, hospital, sports, playground, golf, zoo, airport, parking, construction, landfill, religious,
 * aquaculture, quarry.
 */
public final class Landuse extends Layer implements ForwardingProfile.LayerPostProcessor {

  public static final String NAME = "landuse";

  /** Minimum zoom per kind; large features still need {@code minPixelSize} to appear early. */
  private static final Map<String, Integer> MINZOOM = Map.ofEntries(
    Map.entry("protected", 6), Map.entry("forest", 6), Map.entry("mangrove", 6), Map.entry("wetland", 7),
    Map.entry("salt_pan", 8), Map.entry("park", 8), Map.entry("grass", 9), Map.entry("farmland", 9),
    Map.entry("beach", 8), Map.entry("rock", 9), Map.entry("quarry", 10), Map.entry("airport", 8),
    Map.entry("residential", 9), Map.entry("commercial", 10), Map.entry("industrial", 9), Map.entry("port", 9),
    Map.entry("railway", 10), Map.entry("military", 9), Map.entry("cemetery", 11), Map.entry("education", 11),
    Map.entry("hospital", 11), Map.entry("sports", 11), Map.entry("golf", 10), Map.entry("zoo", 11),
    Map.entry("playground", 14), Map.entry("parking", 14), Map.entry("construction", 11), Map.entry("landfill", 10),
    Map.entry("religious", 13), Map.entry("aquaculture", 10)
  );

  public Landuse(LayerContext ctx) {
    super(ctx);
  }

  @Override
  public String name() {
    return NAME;
  }

  @Override
  public Expression filter() {
    return Expression.or(
      Expression.matchField("landuse"),
      Expression.matchAny("natural", "wood", "wetland", "mud", "scrub", "heath", "grassland", "beach", "sand",
        "bare_rock", "scree"),
      Expression.matchAny("leisure", "park", "garden", "dog_park", "nature_reserve", "golf_course", "stadium",
        "pitch", "sports_centre", "track", "recreation_ground", "playground"),
      Expression.matchAny("boundary", "national_park", "protected_area"),
      Expression.matchAny("amenity", "school", "college", "university", "kindergarten", "hospital", "clinic",
        "grave_yard", "parking"),
      Expression.matchAny("tourism", "zoo"),
      Expression.matchAny("aeroway", "aerodrome"),
      Expression.matchField("military")
    );
  }

  @Override
  public void processFeature(SourceFeature f, FeatureCollector features) {
    if (!f.canBePolygon()) {
      return;
    }
    String kind = kind(f);
    if (kind == null) {
      return;
    }
    features.polygon(NAME)
      .setAttr("kind", kind)
      .setMinZoom(MINZOOM.getOrDefault(kind, 11))
      .setMinPixelSize(2);
  }

  static String kind(SourceFeature f) {
    if (f.hasTag("natural", "wetland")) {
      return f.hasTag("wetland", "mangrove") ? "mangrove" : "wetland";
    }
    if (f.hasTag("natural", "mud")) {
      return "wetland";
    }
    if (f.hasTag("landuse", "salt_pond")) {
      return "salt_pan";
    }
    if (f.hasTag("boundary", "national_park", "protected_area") || f.hasTag("leisure", "nature_reserve")) {
      return "protected";
    }
    if (f.hasTag("landuse", "forest") || f.hasTag("natural", "wood")) {
      return "forest";
    }
    if (f.hasTag("aeroway", "aerodrome")) {
      return "airport";
    }
    if (f.hasTag("leisure", "golf_course")) {
      return "golf";
    }
    if (f.hasTag("leisure", "park", "garden", "dog_park")) {
      return "park";
    }
    if (f.hasTag("tourism", "zoo")) {
      return "zoo";
    }
    if (f.hasTag("leisure", "playground")) {
      return "playground";
    }
    if (f.hasTag("leisure", "stadium", "pitch", "sports_centre", "track", "recreation_ground")
      || f.hasTag("landuse", "recreation_ground")) {
      return "sports";
    }
    if (f.hasTag("amenity", "school", "college", "university", "kindergarten") || f.hasTag("landuse", "education")) {
      return "education";
    }
    if (f.hasTag("amenity", "hospital", "clinic") || f.hasTag("landuse", "healthcare")) {
      return "hospital";
    }
    if (f.hasTag("amenity", "grave_yard") || f.hasTag("landuse", "cemetery")) {
      return "cemetery";
    }
    if (f.hasTag("amenity", "parking") || f.hasTag("landuse", "garages")) {
      return "parking";
    }
    if (f.hasTag("landuse", "military") || f.hasTag("military")) {
      return "military";
    }
    if (f.hasTag("natural", "beach", "sand")) {
      return "beach";
    }
    if (f.hasTag("natural", "bare_rock", "scree")) {
      return "rock";
    }
    if (f.hasTag("natural", "scrub", "heath", "grassland")
      || f.hasTag("landuse", "grass", "meadow", "village_green", "greenfield")) {
      return "grass";
    }
    String landuse = f.getString("landuse");
    if (landuse == null) {
      return null;
    }
    return switch (landuse) {
      case "farmland", "farmyard", "orchard", "plant_nursery", "allotments", "greenhouse_horticulture" -> "farmland";
      case "aquaculture" -> "aquaculture";
      case "residential" -> "residential";
      case "commercial", "retail" -> "commercial";
      case "industrial", "depot" -> "industrial";
      case "port" -> "port";
      case "railway" -> "railway";
      case "construction", "brownfield" -> "construction";
      case "landfill" -> "landfill";
      case "religious" -> "religious";
      case "quarry" -> "quarry";
      default -> null;
    };
  }

  @Override
  public List<VectorTile.Feature> postProcess(int zoom, List<VectorTile.Feature> items) throws GeometryException {
    return zoom < 13 ? FeatureMerge.mergeOverlappingPolygons(items, 1) : items;
  }
}
