package bombaychitra.pipeline;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.nio.file.Files;
import java.nio.file.Path;
import java.util.HashSet;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

class RegionConfigTest {

  private static RegionConfig shipped() throws Exception {
    return RegionConfig.load(Path.of("config", "region.json"));
  }

  @Test
  void shippedRegionIsValid() throws Exception {
    var region = shipped();
    assertEquals("mmr", region.id());
    assertTrue(region.contains(72.8777, 19.0760), "Mumbai is inside the region");
    assertFalse(region.contains(73.86, 18.52), "Pune is outside the region");
    var ids = new HashSet<String>();
    for (var area : region.areas()) {
      assertTrue(ids.add(area.id()), "duplicate area " + area.id());
      assertTrue(area.id().matches("[a-z0-9-]+"), "area ids are URL-safe: " + area.id());
      assertFalse(area.relations().isEmpty(), area.id() + " has no relations");
    }
    assertTrue(ids.contains("makabo"));
  }

  @Test
  void coloursAreNormalised() throws Exception {
    var region = shipped();
    assertEquals("#2b6cd4", region.normaliseColour("blue"));
    assertEquals("#2b6cd4", region.normaliseColour(" Blue "));
    assertEquals("#aabbcc", region.normaliseColour("#ABC"));
    assertEquals("#00a3b4", region.normaliseColour("00A3B4"));
    assertEquals("#e03131", region.normaliseColour("#E03131"));
    assertNull(region.normaliseColour("not-a-colour"));
    assertNull(region.normaliseColour(""));
    assertEquals("#b04fa0", region.modeColour("monorail"));
  }

  @Test
  void invalidRegions(@TempDir Path dir) throws Exception {
    String valid = """
      { "id": "x", "name": "X", "bounds": [72.7, 18.8, 73.8, 19.8], "center": [72.8, 19.0], "zoom": 10, "maxzoom": 15,
        "source": { "file": "x.osm.pbf" } }
      """;
    Path ok = dir.resolve("ok.json");
    Files.writeString(ok, valid);
    assertEquals("x", RegionConfig.load(ok).id());

    for (String broken : new String[]{
      valid.replace("[72.7, 18.8, 73.8, 19.8]", "[73.8, 18.8, 72.7, 19.8]"),
      valid.replace("\"maxzoom\": 15", "\"maxzoom\": 18"),
      valid.replace("\"source\": { \"file\": \"x.osm.pbf\" }", "\"source\": {}"),
      valid.replace("\"source\"", "\"transit\": { \"trackLines\": [ { \"match\": { \"name\": \"L\" }, \"mode\": \"bus\", \"ref\": \"1\" } ] }, \"source\""),
    }) {
      Path file = dir.resolve("broken.json");
      Files.writeString(file, broken);
      assertThrows(IllegalArgumentException.class, () -> RegionConfig.load(file), broken);
    }
  }
}
