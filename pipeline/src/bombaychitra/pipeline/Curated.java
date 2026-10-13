package bombaychitra.pipeline;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
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
public record Curated(List<HeritageSite> heritage) {

  private static final Set<String> HERITAGE_STATUS = Set.of("unesco", "precinct", "monument");

  /** A heritage site from {@code config/heritage.json}; {@code place} is the representing place's feature id. */
  @JsonIgnoreProperties(ignoreUnknown = true)
  public record HeritageSite(String id, String name, String status, Integer year, Long place, double lon, double lat,
    String description, String wikipedia) {}

  @JsonIgnoreProperties(ignoreUnknown = true)
  private record HeritageFile(List<HeritageSite> sites) {}

  public static Curated load(Path configDir) throws IOException {
    var mapper = new ObjectMapper().configure(DeserializationFeature.FAIL_ON_UNKNOWN_PROPERTIES, false);
    List<HeritageSite> heritage = List.of();
    Path file = configDir.resolve("heritage.json");
    if (Files.exists(file)) {
      var parsed = mapper.readValue(file.toFile(), HeritageFile.class);
      heritage = parsed.sites() == null ? List.of() : List.copyOf(parsed.sites());
      validateHeritage(heritage);
    }
    return new Curated(heritage);
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
