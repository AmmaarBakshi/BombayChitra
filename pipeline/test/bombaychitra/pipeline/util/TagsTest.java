package bombaychitra.pipeline.util;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertTrue;

import com.onthegomap.planetiler.reader.WithTags;
import java.util.HashMap;
import org.junit.jupiter.api.Test;

/** Building heights for 3D, addresses and names. */
class TagsTest {

  private static WithTags tags(String... keyValues) {
    var map = new HashMap<String, Object>();
    for (int i = 0; i < keyValues.length; i += 2) {
      map.put(keyValues[i], keyValues[i + 1]);
    }
    return WithTags.from(map);
  }

  @Test
  void heightFromTagOrLevels() {
    assertEquals(85.0, Tags.height(tags("height", "85 m")));
    assertEquals(32.0, Tags.height(tags("building:levels", "10")));
    // Roof levels add at most three storeys.
    assertEquals(41.6, Tags.height(tags("building:levels", "10", "roof:levels", "5")));
  }

  @Test
  void implausibleHeightsAreRejected() {
    // 300 m for 3 floors is a tagging error: fall back to the floors.
    assertEquals(9.6, Tags.height(tags("height", "300", "building:levels", "3")));
    assertNull(Tags.height(tags("height", "900")));
    assertNull(Tags.height(tags("building:levels", "250")));
    assertNull(Tags.height(tags()));
  }

  @Test
  void minHeight() {
    assertEquals(9.6, Tags.minHeight(tags("building:min_level", "3")));
    assertEquals(12.0, Tags.minHeight(tags("min_height", "12")));
    assertNull(Tags.minHeight(tags("min_height", "0")));
  }

  @Test
  void addresses() {
    assertEquals("12 Hill Road, Bandra West, Mumbai, 400050", Tags.address(tags("addr:housenumber", "12",
      "addr:street", "Hill Road", "addr:suburb", "Bandra West", "addr:city", "Mumbai", "addr:postcode", "400050")));
    assertEquals("4, Sea Breeze, Carter Road", Tags.address(tags("addr:housenumber", "4", "addr:housename", "Sea Breeze",
      "addr:street", "Carter Road")));
    assertEquals("Plot 7, Sector 15, Kharghar", Tags.address(tags("addr:full", "Plot 7, Sector 15, Kharghar",
      "addr:street", "ignored")));
    assertNull(Tags.address(tags("name", "No address")));
  }

  @Test
  void namesPreferEnglish() {
    assertEquals("Dadar", Tags.name(tags("name", "दादर", "name:en", "Dadar")));
    assertEquals("दादर", Tags.name(tags("name", " दादर ")));
    assertNull(Tags.name(tags("name", "  ")));
  }

  @Test
  void namesInOtherLanguages() {
    var dadar = tags("name", "Dadar", "name:mr", "दादर", "name:hi", "दादर");
    assertEquals("दादर", Tags.nameIn(dadar, "mr", "Dadar"));
    assertEquals("दादर", Tags.nameIn(dadar, "hi", "Dadar"));
    assertNull(Tags.nameIn(dadar, "gu", "Dadar"));
    // Not repeated when the display name is already in that language.
    assertNull(Tags.nameIn(tags("name", "दादर", "name:mr", "दादर"), "mr", "दादर"));
  }

  @Test
  void yesAndNo() {
    assertTrue(Tags.isYes(tags("bridge", "viaduct"), "bridge"));
    assertFalse(Tags.isYes(tags("bridge", "no"), "bridge"));
    assertFalse(Tags.isYes(tags(), "bridge"));
    assertTrue(Tags.isNo(tags("oneway", "false"), "oneway"));
  }
}
