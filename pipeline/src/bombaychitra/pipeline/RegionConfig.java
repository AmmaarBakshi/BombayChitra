package bombaychitra.pipeline;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.annotation.JsonProperty;
import com.fasterxml.jackson.databind.DeserializationFeature;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.io.IOException;
import java.nio.file.Path;
import java.util.List;
import java.util.Locale;
import java.util.Map;
import java.util.Set;
import org.locationtech.jts.geom.Envelope;

/** Region definition loaded from {@code config/region.json}. */
@JsonIgnoreProperties(ignoreUnknown = true)
public record RegionConfig(
  String id,
  String name,
  String description,
  double[] bounds,
  double[] center,
  int zoom,
  int maxzoom,
  Source source,
  List<Area> areas,
  Transit transit
) {

  @JsonIgnoreProperties(ignoreUnknown = true)
  public record Source(String name, String url, String file) {}

  @JsonIgnoreProperties(ignoreUnknown = true)
  public record Area(
    String id,
    String name,
    @JsonProperty("short") String shortName,
    String kind,
    String description,
    List<Long> relations
  ) {}

  @JsonIgnoreProperties(ignoreUnknown = true)
  public record Transit(
    List<String> modes,
    List<String> trainNetworks,
    Map<String, String> colourAliases,
    Map<String, String> modeColours
  ) {}

  public static RegionConfig load(Path path) throws IOException {
    var mapper = new ObjectMapper().configure(DeserializationFeature.FAIL_ON_UNKNOWN_PROPERTIES, false);
    var config = mapper.readValue(path.toFile(), RegionConfig.class);
    config.validate();
    return config;
  }

  private void validate() {
    if (bounds == null || bounds.length != 4 || bounds[0] >= bounds[2] || bounds[1] >= bounds[3]) {
      throw new IllegalArgumentException("region.json: bounds must be [minLon, minLat, maxLon, maxLat]");
    }
    if (source == null || source.file() == null) {
      throw new IllegalArgumentException("region.json: source.file is required");
    }
    if (maxzoom < 10 || maxzoom > 16) {
      throw new IllegalArgumentException("region.json: maxzoom must be between 10 and 16");
    }
  }

  public Envelope envelope() {
    return new Envelope(bounds[0], bounds[2], bounds[1], bounds[3]);
  }

  public String boundsArg() {
    return String.format(Locale.ROOT, "%.5f,%.5f,%.5f,%.5f", bounds[0], bounds[1], bounds[2], bounds[3]);
  }

  public boolean contains(double lon, double lat) {
    return lon >= bounds[0] && lon <= bounds[2] && lat >= bounds[1] && lat <= bounds[3];
  }

  public boolean intersects(Envelope latLon) {
    return latLon.intersects(envelope());
  }

  public Set<String> transitModes() {
    return transit == null || transit.modes() == null ? Set.of("subway", "light_rail", "monorail", "train")
      : Set.copyOf(transit.modes());
  }

  /** Normalises an OSM {@code colour} value to a {@code #rrggbb} hex string, or null. */
  public String normaliseColour(String colour) {
    if (colour == null || colour.isBlank()) {
      return null;
    }
    String value = colour.trim().toLowerCase(Locale.ROOT);
    if (value.matches("#[0-9a-f]{6}")) {
      return value;
    }
    if (value.matches("#[0-9a-f]{3}")) {
      return "#" + value.charAt(1) + value.charAt(1) + value.charAt(2) + value.charAt(2) + value.charAt(3)
        + value.charAt(3);
    }
    if (value.matches("[0-9a-f]{6}")) {
      return "#" + value;
    }
    if (transit != null && transit.colourAliases() != null) {
      String alias = transit.colourAliases().get(value);
      if (alias != null) {
        return alias.toLowerCase(Locale.ROOT);
      }
    }
    return null;
  }

  public String modeColour(String mode) {
    if (transit == null || transit.modeColours() == null) {
      return null;
    }
    String colour = transit.modeColours().get(mode);
    return colour == null ? null : colour.toLowerCase(Locale.ROOT);
  }
}
