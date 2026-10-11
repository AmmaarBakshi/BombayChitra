package bombaychitra.pipeline;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import com.onthegomap.planetiler.reader.WithTags;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

/** The shipped taxonomy (config/categories.json) and the matching rules behind it. */
class TaxonomyTest {

  private static Taxonomy taxonomy;

  @BeforeAll
  static void load() throws Exception {
    taxonomy = Taxonomy.load(Path.of("config", "categories.json"));
  }

  private static String match(String... keyValues) {
    var tags = new HashMap<String, Object>();
    for (int i = 0; i < keyValues.length; i += 2) {
      tags.put(keyValues[i], keyValues[i + 1]);
    }
    var category = taxonomy.match(WithTags.from(tags));
    return category == null ? null : category.id();
  }

  @Test
  void commonPlaces() {
    assertEquals("hospital", match("amenity", "hospital", "name", "Lilavati Hospital"));
    assertEquals("atm", match("amenity", "atm"));
    assertEquals("bank", match("amenity", "bank"));
    assertEquals("temple", match("amenity", "place_of_worship", "religion", "hindu"));
    assertEquals("mosque", match("amenity", "place_of_worship", "religion", "muslim"));
    assertEquals("bus_stop", match("highway", "bus_stop"));
    assertNull(match("highway", "residential"));
  }

  @Test
  void stationsAreClassifiedByMode() {
    // A metro station is also a railway=station: the more specific rule comes first and wins.
    assertEquals("metro_station", match("railway", "station", "station", "subway", "name", "Ghatkopar"));
    assertEquals("monorail_station", match("railway", "station", "station", "monorail"));
    assertEquals("railway_station", match("railway", "station", "name", "Dadar"));
    assertEquals("railway_station", match("public_transport", "station", "train", "yes"));
  }

  @Test
  void institutionsMappedOnlyAsBuildings() {
    // building=school with a name and no amenity tag is a school ("!*" = tag absent)...
    assertEquals("school", match("building", "school", "name", "St. Xavier's High School"));
    assertEquals("college", match("building", "college", "name", "Mithibai College"));
    assertEquals("hospital", match("building", "hospital", "name", "Cooper Hospital"));
    // ...but when an amenity tag is present, that tag decides.
    assertEquals("college", match("building", "school", "amenity", "college", "name", "X"));
    // Unnamed school buildings are not points of interest.
    assertNull(match("building", "school"));
  }

  @Test
  void namedBuildingsFallBackToBuildingCategories() {
    assertEquals("landmark_building", match("building", "office", "name", "One Indiabulls Centre"));
    assertEquals("residential_building", match("building", "apartments", "name", "Lodha Bellissimo"));
    assertEquals("residential_building", match("building", "yes", "name", "Shanti Niketan"));
  }

  @Test
  void overtureCategoriesMapFromMostSpecific() {
    assertEquals("atm", taxonomy.matchOverture(List.of("atm", "bank_or_credit_union")).id());
    assertEquals("hospital", taxonomy.matchOverture(java.util.Arrays.asList(null, "hospital", "health_care")).id());
    assertNull(taxonomy.matchOverture(List.of("not_a_category")));
  }

  @Test
  void idsAreUniqueAndEveryCategoryHasAGroup() {
    var ids = new HashSet<String>();
    for (var group : taxonomy.groups()) {
      assertTrue(group.colour().matches("#[0-9A-Fa-f]{6}"), group.id() + " colour");
      for (var category : group.categories()) {
        assertTrue(ids.add(category.id()), "duplicate " + category.id());
        assertEquals(group.id(), category.group());
        assertNotNull(taxonomy.category(category.id()));
      }
    }
    assertTrue(ids.size() >= 80, "expected the full taxonomy, got " + ids.size());
  }

  @Test
  void firstMatchingCategoryWins(@TempDir Path dir) throws Exception {
    Path file = dir.resolve("categories.json");
    Files.writeString(file, """
      { "groups": [ { "id": "g", "label": "G", "colour": "#123456", "categories": [
        { "id": "specific", "label": "Specific", "match": [{ "shop": "books", "second_hand": "yes" }] },
        { "id": "general", "label": "General", "match": [{ "shop": "*" }] }
      ] } ] }
      """);
    var custom = Taxonomy.load(file);
    assertEquals("specific", custom.match(WithTags.from(java.util.Map.of("shop", "books", "second_hand", "yes"))).id());
    assertEquals("general", custom.match(WithTags.from(java.util.Map.of("shop", "books"))).id());
    assertNull(custom.match(WithTags.from(java.util.Map.of("shop", "no"))));
  }

  @Test
  void invalidTaxonomies(@TempDir Path dir) throws Exception {
    Path duplicate = dir.resolve("duplicate.json");
    Files.writeString(duplicate, """
      { "groups": [ { "id": "g", "label": "G", "categories": [
        { "id": "a", "label": "A", "match": [{ "shop": "books" }] },
        { "id": "a", "label": "A again", "match": [{ "shop": "toys" }] }
      ] } ] }
      """);
    assertThrows(IllegalArgumentException.class, () -> Taxonomy.load(duplicate));

    Path negativeOnly = dir.resolve("negative.json");
    Files.writeString(negativeOnly, """
      { "groups": [ { "id": "g", "label": "G", "categories": [
        { "id": "a", "label": "A", "match": [{ "shop": "!books" }] }
      ] } ] }
      """);
    assertThrows(IllegalArgumentException.class, () -> Taxonomy.load(negativeOnly));
  }
}
