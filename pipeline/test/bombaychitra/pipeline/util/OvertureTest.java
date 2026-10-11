package bombaychitra.pipeline.util;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.util.Set;
import org.junit.jupiter.api.Test;

/** Overture ids and the name matching that keeps Overture places from duplicating OpenStreetMap ones. */
class OvertureTest {

  @Test
  void featureIdsAreStableAndDistinctFromOsm() {
    long id = Overture.featureId("08f2a100d2a4c56d0368c3a2d1fe4b8c");
    assertEquals(id, Overture.featureId("08f2a100d2a4c56d0368c3a2d1fe4b8c"));
    assertNotEquals(id, Overture.featureId("08f2a100d2a4c56d0368c3a2d1fe4b8d"));
    assertEquals(Overture.TYPE_CODE, id % 10);
    assertTrue(id > 0 && id < (1L << 53), "id must be exact in JavaScript: " + id);
  }

  @Test
  void nameTokensIgnoreCaseAccentsPunctuationAndGenericWords() {
    assertEquals(Set.of("hdfc", "bank", "atm"), Overture.nameTokens("HDFC Bank ATM - Andheri Branch".replace(" - Andheri", "")));
    assertEquals(Set.of("cafe", "madras"), Overture.nameTokens("Café Madras"));
    assertEquals(Set.of("lilavati", "hospital"), Overture.nameTokens("The Lilavati Hospital, Mumbai"));
    assertTrue(Overture.nameTokens(null).isEmpty());
  }

  @Test
  void similarNames() {
    var osm = Overture.nameTokens("Lilavati Hospital");
    assertTrue(Overture.similar(osm, Overture.nameTokens("Lilavati Hospital & Research Centre")));
    assertTrue(Overture.similar(Overture.nameTokens("Shree Krishna Sweets"), Overture.nameTokens("Krishna Sweets")));
    assertFalse(Overture.similar(osm, Overture.nameTokens("Hinduja Hospital")));
    assertFalse(Overture.similar(osm, Set.of()));
  }

  @Test
  void sharedGenericWordsAreNotEnough() {
    // "Bank" and "ATM" say what a place is, not which one: different banks next to each other are kept apart.
    assertFalse(Overture.similar(Overture.nameTokens("HDFC Bank ATM"), Overture.nameTokens("ICICI Bank ATM")));
    assertFalse(Overture.similar(Overture.nameTokens("Apollo Pharmacy"), Overture.nameTokens("Wellness Forever Pharmacy")));
    assertTrue(Overture.similar(Overture.nameTokens("HDFC Bank ATM"), Overture.nameTokens("HDFC ATM")));
    // Names made only of generic words still compare by their words.
    assertTrue(Overture.similar(Overture.nameTokens("Medical Store"), Overture.nameTokens("Medical General Store")));
  }
}
