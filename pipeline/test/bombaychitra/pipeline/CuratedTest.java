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
  void missingFilesAreFine(@TempDir Path dir) throws Exception {
    assertEquals(List.of(), Curated.load(dir).heritage());
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
