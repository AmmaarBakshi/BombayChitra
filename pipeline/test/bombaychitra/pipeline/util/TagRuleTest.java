package bombaychitra.pipeline.util;

import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.onthegomap.planetiler.reader.WithTags;
import java.util.Map;
import org.junit.jupiter.api.Test;

class TagRuleTest {

  private static final ObjectMapper JSON = new ObjectMapper();

  private static TagRule rule(String json) throws Exception {
    return TagRule.parse("test", JSON.readTree(json));
  }

  private static WithTags tags(Object... keyValues) {
    var map = new java.util.HashMap<String, Object>();
    for (int i = 0; i < keyValues.length; i += 2) {
      map.put((String) keyValues[i], keyValues[i + 1]);
    }
    return WithTags.from(map);
  }

  @Test
  void exactValueAndList() throws Exception {
    var rule = rule("{\"amenity\": \"hospital\"}");
    assertTrue(rule.test(tags("amenity", "hospital")));
    assertFalse(rule.test(tags("amenity", "clinic")));
    assertFalse(rule.test(tags()));

    var list = rule("{\"railway\": [\"station\", \"halt\"]}");
    assertTrue(list.test(tags("railway", "halt")));
    assertFalse(list.test(tags("railway", "platform")));
  }

  @Test
  void everyConditionMustHold() throws Exception {
    var rule = rule("{\"railway\": \"station\", \"station\": \"subway\"}");
    assertTrue(rule.test(tags("railway", "station", "station", "subway")));
    assertFalse(rule.test(tags("railway", "station")));
    assertFalse(rule.test(tags("station", "subway")));
  }

  @Test
  void wildcardMatchesAnyValueButNo() throws Exception {
    var rule = rule("{\"name\": \"*\"}");
    assertTrue(rule.test(tags("name", "Lilavati Hospital")));
    assertFalse(rule.test(tags("name", "no")));
    assertFalse(rule.test(tags()));
  }

  @Test
  void negatedValue() throws Exception {
    var rule = rule("{\"building\": \"school\", \"amenity\": \"!kindergarten\"}");
    assertTrue(rule.test(tags("building", "school")));
    assertTrue(rule.test(tags("building", "school", "amenity", "school")));
    assertFalse(rule.test(tags("building", "school", "amenity", "kindergarten")));
  }

  @Test
  void negatedWildcardMeansAbsent() throws Exception {
    // "!*" is how categories.json says "only when the tag is missing" (e.g. building=school without amenity).
    var rule = rule("{\"building\": \"school\", \"amenity\": \"!*\"}");
    assertTrue(rule.test(tags("building", "school")));
    assertFalse(rule.test(tags("building", "school", "amenity", "school")));
    assertFalse(rule.test(tags("building", "school", "amenity", "no")));
  }

  @Test
  void numbersAreComparedAsText() throws Exception {
    var rule = rule("{\"admin_level\": \"10\"}");
    assertTrue(rule.test(WithTags.from(Map.of("admin_level", 10))));
  }

  @Test
  void invalidRules() {
    assertThrows(IllegalArgumentException.class, () -> rule("{}"));
    assertThrows(IllegalArgumentException.class, () -> rule("[\"amenity\"]"));
    assertThrows(IllegalArgumentException.class, () -> rule("{\"amenity\": [\"school\", \"!college\"]}"));
  }
}
