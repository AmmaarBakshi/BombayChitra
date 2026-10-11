package bombaychitra.pipeline.util;

import com.fasterxml.jackson.databind.JsonNode;
import com.onthegomap.planetiler.reader.WithTags;
import java.util.ArrayList;
import java.util.HashSet;
import java.util.List;
import java.util.Set;

/**
 * A set of tag conditions in the rule syntax shared by {@code config/categories.json} and {@code config/region.json}:
 * each key maps to a value, a list of values, {@code "*"} (any value except {@code no}) or {@code "!value"} (absent or
 * any other value). All conditions must hold.
 */
public record TagRule(List<Condition> conditions) {

  public record Condition(String key, Set<String> values, boolean any, boolean negated) {

    public boolean test(WithTags tags) {
      Object raw = tags.getTag(key);
      String value = raw == null ? null : raw.toString();
      if (negated) {
        return value == null || !values.contains(value);
      }
      if (value == null) {
        return false;
      }
      return any ? !"no".equals(value) : values.contains(value);
    }
  }

  /** Parses {@code {"railway": "construction", "name": ["Line 5", "Line-5"]}}; {@code context} prefixes errors. */
  public static TagRule parse(String context, JsonNode node) {
    if (node == null || !node.isObject()) {
      throw new IllegalArgumentException(context + ": a rule must be an object of tag conditions");
    }
    List<Condition> conditions = new ArrayList<>();
    for (var entry : node.properties()) {
      List<String> raw = new ArrayList<>();
      if (entry.getValue().isArray()) {
        entry.getValue().forEach(v -> raw.add(v.asText()));
      } else {
        raw.add(entry.getValue().asText());
      }
      boolean any = raw.contains("*");
      boolean negated = !raw.isEmpty() && raw.stream().allMatch(v -> v.startsWith("!"));
      if (!negated && raw.stream().anyMatch(v -> v.startsWith("!"))) {
        throw new IllegalArgumentException(context + "." + entry.getKey() + " mixes negated and plain values");
      }
      Set<String> values = new HashSet<>();
      for (String v : raw) {
        if (!"*".equals(v)) {
          values.add(negated ? v.substring(1) : v);
        }
      }
      conditions.add(new Condition(entry.getKey(), Set.copyOf(values), any, negated));
    }
    if (conditions.isEmpty()) {
      throw new IllegalArgumentException(context + ": empty rule");
    }
    return new TagRule(List.copyOf(conditions));
  }

  public boolean test(WithTags tags) {
    for (Condition condition : conditions) {
      if (!condition.test(tags)) {
        return false;
      }
    }
    return true;
  }
}
