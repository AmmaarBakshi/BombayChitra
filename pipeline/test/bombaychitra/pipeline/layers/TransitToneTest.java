package bombaychitra.pipeline.layers;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNull;

import org.junit.jupiter.api.Test;

/** Line colours that need a contrasting casing on one of the two maps. */
class TransitToneTest {

  @Test
  void darkAndLightLineColours() {
    assertEquals("dark", Transit.tone("#4b0082"), "indigo is too dark for the dark map");
    assertEquals("dark", Transit.tone("#303888"));
    assertEquals("dark", Transit.tone("#77248b"));
    assertEquals("light", Transit.tone("#f2b705"), "Line 2A yellow is too pale for the light map");
    assertEquals("light", Transit.tone("#f0d808"));
    assertNull(Transit.tone("#e03131"), "red reads on both maps");
    assertNull(Transit.tone("#2b6cd4"));
    assertNull(Transit.tone("blue"));
    assertNull(Transit.tone(null));
  }
}
