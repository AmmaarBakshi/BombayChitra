package bombaychitra.pipeline.data;

import java.util.ArrayList;
import java.util.Collection;
import java.util.HashMap;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Locale;
import java.util.Map;
import java.util.function.Function;
import org.locationtech.jts.geom.Envelope;
import org.locationtech.jts.index.strtree.STRtree;

/**
 * Groups items that share a key (e.g. a road name) and lie near each other. OSM splits one street or one long bridge
 * into many ways; clustering turns them back into a single searchable entity while keeping distinct streets that
 * happen to share a common name ("Station Road") apart.
 */
final class Clusters {

  private Clusters() {}

  static <T> List<List<T>> of(Collection<T> items, Function<T, String> key, Function<T, Envelope> envelope,
    double gapDegrees) {
    Map<String, List<T>> groups = new LinkedHashMap<>();
    for (T item : items) {
      String k = key.apply(item);
      if (k != null) {
        groups.computeIfAbsent(k.toLowerCase(Locale.ROOT).strip(), x -> new ArrayList<>()).add(item);
      }
    }
    List<List<T>> result = new ArrayList<>();
    for (List<T> group : groups.values()) {
      if (group.size() == 1) {
        result.add(group);
        continue;
      }
      result.addAll(split(group, envelope, gapDegrees));
    }
    return result;
  }

  private static <T> List<List<T>> split(List<T> group, Function<T, Envelope> envelope, double gap) {
    int n = group.size();
    int[] parent = new int[n];
    for (int i = 0; i < n; i++) {
      parent[i] = i;
    }
    STRtree tree = new STRtree();
    Envelope[] envelopes = new Envelope[n];
    for (int i = 0; i < n; i++) {
      envelopes[i] = envelope.apply(group.get(i));
      tree.insert(envelopes[i], i);
    }
    for (int i = 0; i < n; i++) {
      Envelope probe = new Envelope(envelopes[i]);
      probe.expandBy(gap);
      for (Object o : tree.query(probe)) {
        union(parent, i, (Integer) o);
      }
    }
    Map<Integer, List<T>> clusters = new HashMap<>();
    for (int i = 0; i < n; i++) {
      clusters.computeIfAbsent(find(parent, i), r -> new ArrayList<>()).add(group.get(i));
    }
    return new ArrayList<>(clusters.values());
  }

  private static int find(int[] parent, int i) {
    while (parent[i] != i) {
      parent[i] = parent[parent[i]];
      i = parent[i];
    }
    return i;
  }

  private static void union(int[] parent, int a, int b) {
    int ra = find(parent, a);
    int rb = find(parent, b);
    if (ra != rb) {
      parent[ra] = rb;
    }
  }
}
