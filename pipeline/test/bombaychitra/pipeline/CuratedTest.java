package bombaychitra.pipeline;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

class CuratedTest {

  @Test
  void shippedHeritageSitesAreValid() throws Exception {
    var curated = Curated.load(Path.of("config"));
    assertTrue(curated.heritage().size() >= 10);
    assertEquals(3, curated.heritage().stream().filter(s -> "unesco".equals(s.status())).count(),
      "CSMT, the Elephanta Caves and the Victorian Gothic and Art Deco Ensembles");
    var region = RegionConfig.load(Path.of("config", "region.json"));
    for (var site : curated.heritage()) {
      assertTrue(region.contains(site.lon(), site.lat()), site.id() + " is outside the region");
    }
  }

  @Test
  void shippedHotspotsAreValid() throws Exception {
    var curated = Curated.load(Path.of("config"));
    var region = RegionConfig.load(Path.of("config", "region.json"));
    assertTrue(curated.hotspots().size() >= 10);
    assertEquals(new Curated.Season(6, 9), curated.season(), "the monsoon");
    for (var spot : curated.hotspots()) {
      assertTrue(region.contains(spot.lon(), spot.lat()), spot.id() + " is outside the region");
      assertTrue(spot.radius() >= 100 && spot.radius() <= 500, spot.id() + " radius");
    }
  }

  @Test
  void missingFilesAreFine(@TempDir Path dir) throws Exception {
    var curated = Curated.load(dir);
    assertEquals(List.of(), curated.heritage());
    assertEquals(List.of(), curated.hotspots());
    assertEquals(new Curated.Season(6, 9), curated.season());
  }

  @Test
  void invalidHotspotsAreRejected(@TempDir Path dir) throws Exception {
    for (String file : new String[]{
      "{ \"season\": { \"from\": 0, \"to\": 9 }, \"hotspots\": [] }",
      "{ \"hotspots\": [ { \"id\": \"a\", \"name\": \"A\", \"lon\": 72.8, \"lat\": 19.0, \"radius_m\": 5000 } ] }",
      "{ \"hotspots\": [ { \"id\": \"a\", \"name\": \"A\", \"lon\": 0, \"lat\": 0 } ] }",
    }) {
      Files.writeString(dir.resolve("waterlogging.json"), file);
      assertThrows(IllegalArgumentException.class, () -> Curated.load(dir), file);
    }
  }

  @Test
  void invalidSitesAreRejected(@TempDir Path dir) throws Exception {
    String site = "{ \"id\": \"x\", \"name\": \"X\", \"status\": \"%s\", %s \"lon\": 72.8, \"lat\": 19.0 }";
    for (String sites : new String[]{
      String.format(site, "famous", ""),
      String.format(site, "unesco", ""),
      String.format(site, "monument", "") + ", " + String.format(site, "monument", ""),
      String.format(site, "monument", "").replace("\"X\"", "\"\""),
      String.format(site, "monument", "").replace("\"x\"", "\"Not Valid\""),
    }) {
      Files.writeString(dir.resolve("heritage.json"), "{ \"sites\": [" + sites + "] }");
      assertThrows(IllegalArgumentException.class, () -> Curated.load(dir), sites);
    }
    Files.writeString(dir.resolve("heritage.json"), "{ \"sites\": [" + String.format(site, "unesco", "\"year\": 2004,") + "] }");
    assertEquals(2004, Curated.load(dir).heritage().getFirst().year());
  }
}
