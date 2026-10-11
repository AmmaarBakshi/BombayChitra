package bombaychitra.pipeline;

import bombaychitra.pipeline.util.TagRule;
import bombaychitra.pipeline.util.TagRule.Condition;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.onthegomap.planetiler.reader.WithTags;
import java.io.IOException;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Comparator;
import java.util.HashMap;
import java.util.LinkedHashMap;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * The point-of-interest taxonomy loaded from {@code config/categories.json}.
 *
 * <p>Rules are compiled into an index keyed by the first exact tag condition of each rule, so classifying a feature
 * costs a handful of hash lookups regardless of how many categories are configured.
 */
public final class Taxonomy {

  public record Group(String id, String label, String colour, int sort, List<Category> categories) {}

  public record Category(String id, String group, String label, String icon, int rank, int minzoom, int sort) {}

  private record Rule(Category category, int priority, List<Condition> conditions) {

    boolean test(WithTags tags) {
      for (Condition condition : conditions) {
        if (!condition.test(tags)) {
          return false;
        }
      }
      return true;
    }
  }

  private final List<Group> groups;
  private final Map<String, Category> byId;
  /** key → value → candidate rules ordered by priority. */
  private final Map<String, Map<String, Rule[]>> exactIndex;
  /** key → candidate rules (first condition is a wildcard) ordered by priority. */
  private final Map<String, Rule[]> wildcardIndex;
  private final Set<String> triggerKeys;

  private Taxonomy(List<Group> groups, List<Rule> rules) {
    this.groups = List.copyOf(groups);
    this.byId = new LinkedHashMap<>();
    for (Group group : groups) {
      for (Category category : group.categories()) {
        if (byId.put(category.id(), category) != null) {
          throw new IllegalArgumentException("categories.json: duplicate category id " + category.id());
        }
      }
    }
    Map<String, Map<String, List<Rule>>> exact = new HashMap<>();
    Map<String, List<Rule>> wildcard = new HashMap<>();
    for (Rule rule : rules) {
      Condition indexed = rule.conditions().stream().filter(c -> !c.any() && !c.negated()).findFirst().orElse(null);
      if (indexed != null) {
        for (String value : indexed.values()) {
          exact.computeIfAbsent(indexed.key(), k -> new HashMap<>()).computeIfAbsent(value, v -> new ArrayList<>())
            .add(rule);
        }
      } else {
        Condition any = rule.conditions().stream().filter(Condition::any).findFirst()
          .orElseThrow(() -> new IllegalArgumentException(
            "categories.json: rule for " + rule.category().id() + " needs at least one positive condition"));
        wildcard.computeIfAbsent(any.key(), k -> new ArrayList<>()).add(rule);
      }
    }
    Comparator<Rule> byPriority = Comparator.comparingInt(Rule::priority);
    this.exactIndex = new HashMap<>();
    exact.forEach((key, values) -> {
      Map<String, Rule[]> compiled = new HashMap<>();
      values.forEach((value, list) -> compiled.put(value, list.stream().sorted(byPriority).toArray(Rule[]::new)));
      exactIndex.put(key, compiled);
    });
    this.wildcardIndex = new HashMap<>();
    wildcard.forEach((key, list) -> wildcardIndex.put(key, list.stream().sorted(byPriority).toArray(Rule[]::new)));
    this.triggerKeys = new LinkedHashSet<>();
    triggerKeys.addAll(exactIndex.keySet());
    triggerKeys.addAll(wildcardIndex.keySet());
  }

  public static Taxonomy load(Path path) throws IOException {
    JsonNode root = new ObjectMapper().readTree(path.toFile());
    List<Group> groups = new ArrayList<>();
    List<Rule> rules = new ArrayList<>();
    int categorySort = 0;
    int groupSort = 0;
    for (JsonNode groupNode : root.path("groups")) {
      String groupId = required(groupNode, "id");
      List<Category> categories = new ArrayList<>();
      for (JsonNode catNode : groupNode.path("categories")) {
        var category = new Category(
          required(catNode, "id"),
          groupId,
          required(catNode, "label"),
          catNode.path("icon").asText("marker"),
          catNode.path("rank").asInt(3),
          catNode.path("minzoom").asInt(15),
          categorySort++
        );
        categories.add(category);
        for (JsonNode ruleNode : catNode.path("match")) {
          rules.add(new Rule(category, category.sort(),
            TagRule.parse("categories.json: " + category.id(), ruleNode).conditions()));
        }
      }
      groups.add(new Group(groupId, required(groupNode, "label"), groupNode.path("colour").asText("#888888"),
        groupSort++, List.copyOf(categories)));
    }
    if (rules.isEmpty()) {
      throw new IllegalArgumentException("categories.json: no rules defined");
    }
    return new Taxonomy(groups, rules);
  }

  private static String required(JsonNode node, String field) {
    String value = node.path(field).asText(null);
    if (value == null || value.isBlank()) {
      throw new IllegalArgumentException("categories.json: missing '" + field + "' in " + node);
    }
    return value;
  }

  /** Returns the highest-priority category matching {@code tags}, or {@code null}. */
  public Category match(WithTags tags) {
    Rule best = null;
    for (var entry : exactIndex.entrySet()) {
      Object value = tags.getTag(entry.getKey());
      if (value == null) {
        continue;
      }
      Rule[] candidates = entry.getValue().get(value.toString());
      if (candidates != null) {
        best = firstMatch(candidates, tags, best);
      }
    }
    for (var entry : wildcardIndex.entrySet()) {
      if (tags.hasTag(entry.getKey())) {
        best = firstMatch(entry.getValue(), tags, best);
      }
    }
    return best == null ? null : best.category();
  }

  private static Rule firstMatch(Rule[] candidates, WithTags tags, Rule best) {
    for (Rule rule : candidates) {
      if (best != null && rule.priority() >= best.priority()) {
        return best;
      }
      if (rule.test(tags)) {
        return rule;
      }
    }
    return best;
  }

  /** Tag keys that can trigger a match; used to build the Planetiler dispatch filter. */
  public List<String> triggerKeys() {
    return List.copyOf(triggerKeys);
  }

  public List<Group> groups() {
    return groups;
  }

  public Category category(String id) {
    return byId.get(id);
  }

  public Group group(String id) {
    return groups.stream().filter(g -> g.id().equals(id)).findFirst().orElse(null);
  }

  @Override
  public String toString() {
    return "Taxonomy" + Arrays.toString(groups.stream().map(Group::id).toArray());
  }
}
