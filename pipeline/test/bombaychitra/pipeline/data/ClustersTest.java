package bombaychitra.pipeline.data;

import static org.junit.jupiter.api.Assertions.assertEquals;

import java.util.Comparator;
import java.util.List;
import org.junit.jupiter.api.Test;
import org.locationtech.jts.geom.Envelope;

/** Street and bridge clustering: OSM ways sharing a name are merged only when they are near each other. */
class ClustersTest {

  record Way(String name, double lon1, double lat1, double lon2, double lat2) {

    Envelope envelope() {
      return new Envelope(lon1, lon2, lat1, lat2);
    }
  }

  private static List<List<Way>> cluster(List<Way> ways, double gap) {
    var clusters = Clusters.of(ways, Way::name, Way::envelope, gap);
    clusters.forEach(c -> c.sort(Comparator.comparingDouble(Way::lon1)));
    clusters.sort(Comparator.comparingDouble(c -> c.getFirst().lon1()));
    return clusters;
  }

  @Test
  void consecutiveWaysOfOneRoadMerge() {
    var ways = List.of(
      new Way("S V Road", 72.840, 19.050, 72.841, 19.060),
      new Way("S V Road", 72.841, 19.060, 72.842, 19.070),
      new Way("S V Road", 72.842, 19.070, 72.843, 19.080));
    assertEquals(1, cluster(ways, 0.004).size());
  }

  @Test
  void distantRoadsWithACommonNameStayApart() {
    var ways = List.of(
      new Way("Station Road", 72.840, 19.050, 72.842, 19.052), // Bandra
      new Way("Station Road", 72.975, 19.185, 72.977, 19.187), // Thane
      new Way("Station Road", 72.976, 19.187, 72.978, 19.189));
    var clusters = cluster(ways, 0.004);
    assertEquals(2, clusters.size());
    assertEquals(1, clusters.get(0).size());
    assertEquals(2, clusters.get(1).size());
  }

  @Test
  void namesAreComparedCaseInsensitively() {
    var ways = List.of(
      new Way("Bandra-Worli Sea Link", 72.81, 19.03, 72.82, 19.04),
      new Way("bandra-worli sea link ", 72.82, 19.04, 72.83, 19.05));
    assertEquals(1, cluster(ways, 0.003).size());
  }

  @Test
  void bridgeSpansBridgedByParallelDecks() {
    // Two carriageways of a flyover, each split in two, join through their shared ends: one bridge.
    var ways = List.of(
      new Way("Kherwadi Flyover", 72.8400, 19.0600, 72.8420, 19.0610),
      new Way("Kherwadi Flyover", 72.8420, 19.0610, 72.8440, 19.0620),
      new Way("Kherwadi Flyover", 72.8401, 19.0602, 72.8421, 19.0612),
      new Way("Kherwadi Flyover", 72.8421, 19.0612, 72.8441, 19.0622));
    assertEquals(1, cluster(ways, 0.003).size());
  }

  @Test
  void unnamedItemsAreSkipped() {
    var ways = List.of(new Way(null, 72.8, 19.0, 72.9, 19.1), new Way("Linking Road", 72.83, 19.06, 72.84, 19.07));
    assertEquals(1, cluster(ways, 0.004).size());
  }
}
