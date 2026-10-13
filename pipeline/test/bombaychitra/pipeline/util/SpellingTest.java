package bombaychitra.pipeline.util;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.nio.file.Files;
import java.nio.file.Path;
import org.junit.jupiter.api.Test;

class SpellingTest {

  /** The cases shared with the server's implementation. */
  @Test
  void sharedCases() throws Exception {
    int cases = 0;
    for (String line : Files.readAllLines(Path.of("testdata", "search-spelling.tsv"))) {
      if (line.startsWith("#") || line.isBlank()) {
        continue;
      }
      String[] parts = line.split("\t", -1);
      assertEquals(String.join(" ", Spelling.words(parts[0])), parts[1], parts[0]);
      cases++;
    }
    assertTrue(cases >= 15);
  }

  @Test
  void indexAddsJoinedNames() {
    assertEquals("vile parle vileparle", Spelling.index("Vile Parle"));
    assertEquals("chatrapati shivaji terminus chatrapatishivajiterminus csmt cst vt",
      Spelling.index("Chhatrapati Shivaji Terminus", "CSMT;CST;VT"));
    assertNull(Spelling.index("दादर"));
    assertNull(Spelling.index((String) null));
  }

  @Test
  void variantsFoldTogether() {
    assertEquals(Spelling.words("Chhatrapati"), Spelling.words("chatrapati"));
    assertEquals(Spelling.words("Bhayandar"), Spelling.words("Bayandar"));
    assertEquals(Spelling.words("Mattunga"), Spelling.words("Matunga"));
    assertEquals(Spelling.words("Goregaon"), Spelling.words("Goregaon"));
  }
}
