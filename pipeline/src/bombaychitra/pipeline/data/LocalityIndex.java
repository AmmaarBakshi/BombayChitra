package bombaychitra.pipeline.data;

import bombaychitra.pipeline.util.Geo;
import java.util.Collection;
import java.util.Set;
import org.locationtech.jts.geom.Envelope;
import org.locationtech.jts.index.strtree.STRtree;

/** Nearest neighbourhood / suburb lookup used to give search results human context ("Station Road · Santacruz"). */
final class LocalityIndex {

  private static final Set<String> CONTEXT_KINDS = Set.of("suburb", "quarter", "neighbourhood", "village", "town");
  private static final double MAX_DISTANCE_M = 3_000;

  private final STRtree tree = new STRtree();
  private final boolean empty;

  LocalityIndex(Collection<Records.Locality> localities) {
    int count = 0;
    for (var locality : localities) {
      if (CONTEXT_KINDS.contains(locality.kind())) {
        tree.insert(new Envelope(locality.lon(), locality.lon(), locality.lat(), locality.lat()), locality);
        count++;
      }
    }
    empty = count == 0;
    if (!empty) {
      tree.build();
    }
  }

  String nearest(double lon, double lat) {
    if (empty) {
      return null;
    }
    var probe = new Records.Locality(0, "", "", lon, lat, null, "");
    // Planar degree distance keeps item distances consistent with the tree's envelope distances, so the
    // branch-and-bound search can prune; the true distance is checked with haversine below.
    var hit = (Records.Locality) tree.nearestNeighbour(new Envelope(lon, lon, lat, lat), probe,
      (a, b) -> {
        var x = (Records.Locality) a.getItem();
        var y = (Records.Locality) b.getItem();
        return Math.hypot(x.lon() - y.lon(), x.lat() - y.lat());
      });
    if (hit == null || Geo.haversine(lon, lat, hit.lon(), hit.lat()) > MAX_DISTANCE_M) {
      return null;
    }
    return hit.name();
  }
}
