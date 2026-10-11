package bombaychitra.pipeline.util;

import com.onthegomap.planetiler.reader.SourceFeature;
import com.onthegomap.planetiler.reader.WithTags;
import com.onthegomap.planetiler.reader.osm.OsmElement;
import com.onthegomap.planetiler.reader.osm.OsmSourceFeature;
import com.onthegomap.planetiler.util.Parse;
import java.util.StringJoiner;

/** Tag helpers shared by every layer. */
public final class Tags {

  /** Average storey height used to derive building heights from {@code building:levels}. */
  public static final double LEVEL_HEIGHT_M = 3.2;

  private Tags() {}

  /**
   * Stable numeric id for an OSM element: {@code osmId * 10 + type} where type is 1=node, 2=way, 3=relation. Fits in a
   * JavaScript number and is reversible in any language.
   */
  public static long oid(SourceFeature feature) {
    return feature.id() * 10 + typeCode(feature);
  }

  public static int typeCode(SourceFeature feature) {
    if (feature instanceof OsmSourceFeature<?> osm) {
      return switch (osm.originalElement()) {
        case OsmElement.Node ignored -> 1;
        case OsmElement.Way ignored -> 2;
        case OsmElement.Relation ignored -> 3;
        default -> 0;
      };
    }
    return 0;
  }

  public static String typeName(long oid) {
    return switch ((int) (oid % 10)) {
      case 1 -> "node";
      case 2 -> "way";
      case 3 -> "relation";
      default -> "other";
    };
  }

  /** Display name: {@code name:en} when present, otherwise {@code name}. */
  public static String name(WithTags tags) {
    String en = clean(tags.getString("name:en"));
    return en != null ? en : clean(tags.getString("name"));
  }

  public static String clean(String value) {
    if (value == null) {
      return null;
    }
    String trimmed = value.strip();
    return trimmed.isEmpty() ? null : trimmed;
  }

  public static String first(WithTags tags, String... keys) {
    for (String key : keys) {
      String value = clean(tags.getString(key));
      if (value != null) {
        return value;
      }
    }
    return null;
  }

  /** Upper bounds that reject obvious tagging errors (Mumbai's tallest towers are ~90 storeys / ~320 m). */
  public static final int MAX_LEVELS = 100;
  public static final double MAX_HEIGHT_M = 350;

  public static Integer levels(WithTags tags) {
    Integer levels = Parse.parseIntSubstring(tags.getString("building:levels"));
    return levels == null || levels < 0 || levels > MAX_LEVELS ? null : levels;
  }

  /**
   * Height in metres from {@code height} / {@code building:height}, validated against {@code building:levels}; falls
   * back to levels × {@link #LEVEL_HEIGHT_M}. Null when unknown or implausible.
   */
  public static Double height(WithTags tags) {
    Integer levels = levels(tags);
    Double tagged = Parse.meters(tags.getTag("height"));
    if (tagged == null) {
      tagged = Parse.meters(tags.getTag("building:height"));
    }
    if (tagged != null) {
      boolean plausible = tagged > 0 && tagged <= MAX_HEIGHT_M;
      if (plausible && levels != null && levels > 0) {
        double perLevel = tagged / levels;
        plausible = perLevel >= 1.8 && perLevel <= 8;
      }
      if (plausible) {
        return round1(tagged);
      }
    }
    if (levels != null && levels > 0) {
      Integer roof = Parse.parseIntSubstring(tags.getString("roof:levels"));
      return round1((levels + (roof == null ? 0 : Math.min(roof, 3))) * LEVEL_HEIGHT_M);
    }
    return null;
  }

  public static Double minHeight(WithTags tags) {
    Double min = Parse.meters(tags.getTag("min_height"));
    if (min == null) {
      Integer minLevel = Parse.parseIntSubstring(tags.getString("building:min_level"));
      if (minLevel != null && minLevel > 0) {
        min = minLevel * LEVEL_HEIGHT_M;
      }
    }
    return min == null || min <= 0 || min > 1000 ? null : round1(min);
  }

  /** Human-readable single-line address assembled from {@code addr:*} tags. */
  public static String address(WithTags tags) {
    String full = clean(tags.getString("addr:full"));
    if (full != null) {
      return full;
    }
    var joiner = new StringJoiner(", ");
    String number = clean(tags.getString("addr:housenumber"));
    String house = clean(tags.getString("addr:housename"));
    String street = clean(tags.getString("addr:street"));
    if (house != null) {
      joiner.add(number != null ? number + ", " + house : house);
    } else if (number != null && street == null) {
      joiner.add(number);
    }
    if (street != null) {
      joiner.add(number != null && house == null ? number + " " + street : street);
    }
    for (String key : new String[]{"addr:place", "addr:suburb", "addr:city"}) {
      String value = clean(tags.getString(key));
      if (value != null) {
        joiner.add(value);
      }
    }
    String postcode = clean(tags.getString("addr:postcode"));
    if (postcode != null) {
      joiner.add(postcode);
    }
    String result = joiner.toString();
    return result.isEmpty() ? null : result;
  }

  public static boolean isNo(WithTags tags, String key) {
    return tags.hasTag(key, "no", "false", "0");
  }

  public static boolean isYes(WithTags tags, String key) {
    return tags.hasTag(key) && !isNo(tags, key);
  }

  public static double round1(double value) {
    return Math.round(value * 10) / 10d;
  }

  public static double round6(double value) {
    return Math.round(value * 1e6) / 1e6;
  }
}
