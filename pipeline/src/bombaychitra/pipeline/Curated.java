package bombaychitra.pipeline;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.annotation.JsonProperty;
import com.fasterxml.jackson.databind.DeserializationFeature;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.HashSet;
import java.util.List;
import java.util.Set;

/**
 * Hand-curated data from {@code config/}: knowledge that OpenStreetMap does not hold, such as which buildings are World
 * Heritage Sites. Every file is optional.
 */
public record Curated(List<HeritageSite> heritage, List<Hotspot> hotspots, Season season) {

  private static final Set<String> HERITAGE_STATUS = Set.of("unesco", "precinct", "monument");

  /** A heritage site from {@code config/heritage.json}; {@code place} is the representing place's feature id. */
  @JsonIgnoreProperties(ignoreUnknown = true)
  public record HeritageSite(String id, String name, String status, Integer year, Long place, double lon, double lat,
    String description, String wikipedia) {}

  @JsonIgnoreProperties(ignoreUnknown = true)
  private record HeritageFile(List<HeritageSite> sites) {}

  /** A spot that floods in heavy rain, from {@code config/waterlogging.json}. */
  @JsonIgnoreProperties(ignoreUnknown = true)
  public record Hotspot(String id, String name, double lon, double lat, @JsonProperty("radius_m") Double radiusM,
    String description) {

    public double radius() {
      return radiusM == null ? 200 : radiusM;
    }
  }

  /** The months (1–12, inclusive) when hotspots flood; the monsoon is June to September. */
  @JsonIgnoreProperties(ignoreUnknown = true)
  public record Season(int from, int to) {}

  @JsonIgnoreProperties(ignoreUnknown = true)
  private record WaterloggingFile(Season season, List<Hotspot> hotspots) {}

  public static Curated load(Path configDir) throws IOException {
    var mapper = new ObjectMapper().configure(DeserializationFeature.FAIL_ON_UNKNOWN_PROPERTIES, false);
    List<HeritageSite> heritage = List.of();
    Path file = configDir.resolve("heritage.json");
    if (Files.exists(file)) {
      var parsed = mapper.readValue(file.toFile(), HeritageFile.class);
      heritage = parsed.sites() == null ? List.of() : List.copyOf(parsed.sites());
      validateHeritage(heritage);
    }
    List<Hotspot> hotspots = List.of();
    Season season = new Season(6, 9);
    file = configDir.resolve("waterlogging.json");
    if (Files.exists(file)) {
      var parsed = mapper.readValue(file.toFile(), WaterloggingFile.class);
      hotspots = parsed.hotspots() == null ? List.of() : List.copyOf(parsed.hotspots());
      if (parsed.season() != null) {
        season = parsed.season();
      }
      validateHotspots(hotspots, season);
    }
    return new Curated(heritage, hotspots, season);
  }

  static void validateHotspots(List<Hotspot> hotspots, Season season) {
    if (season.from() < 1 || season.from() > 12 || season.to() < 1 || season.to() > 12) {
      throw new IllegalArgumentException("waterlogging.json: season months must be 1-12");
    }
    Set<String> ids = new HashSet<>();
    for (var spot : hotspots) {
      String context = "waterlogging.json: " + spot.id();
      if (spot.id() == null || !spot.id().matches("[a-z0-9-]+") || !ids.add(spot.id())) {
        throw new IllegalArgumentException(context + ": ids must be unique, lower-case and URL-safe");
      }
      if (spot.name() == null || spot.name().isBlank()) {
        throw new IllegalArgumentException(context + ": name is required");
      }
      if (spot.radius() <= 0 || spot.radius() > 2000) {
        throw new IllegalArgumentException(context + ": radius_m must be between 0 and 2000");
      }
      if (Math.abs(spot.lat()) > 90 || Math.abs(spot.lon()) > 180 || spot.lat() == 0 && spot.lon() == 0) {
        throw new IllegalArgumentException(context + ": lon/lat are missing or invalid");
      }
    }
  }

  static void validateHeritage(List<HeritageSite> sites) {
    Set<String> ids = new HashSet<>();
    for (var site : sites) {
      String context = "heritage.json: " + site.id();
      if (site.id() == null || !site.id().matches("[a-z0-9-]+") || !ids.add(site.id())) {
        throw new IllegalArgumentException(context + ": ids must be unique, lower-case and URL-safe");
      }
      if (site.name() == null || site.name().isBlank()) {
        throw new IllegalArgumentException(context + ": name is required");
      }
      if (!HERITAGE_STATUS.contains(site.status())) {
        throw new IllegalArgumentException(context + ": status must be one of " + HERITAGE_STATUS);
      }
      if ("unesco".equals(site.status()) && site.year() == null) {
        throw new IllegalArgumentException(context + ": a World Heritage Site needs its year of inscription");
      }
      if (Math.abs(site.lat()) > 90 || Math.abs(site.lon()) > 180 || site.lat() == 0 && site.lon() == 0) {
        throw new IllegalArgumentException(context + ": lon/lat are missing or invalid");
      }
    }
  }
}
