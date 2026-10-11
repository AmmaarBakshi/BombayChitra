package bombaychitra.pipeline.geo;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.util.List;
import org.junit.jupiter.api.Test;
import org.locationtech.jts.geom.Coordinate;
import org.locationtech.jts.geom.GeometryFactory;
import org.locationtech.jts.geom.LineString;

/** The spine orders stations along a line and measures it once even where the tracks run in pairs. */
class LineSpineTest {

  private static final GeometryFactory FACTORY = new GeometryFactory();

  private static LineString line(double... lonLat) {
    var coords = new Coordinate[lonLat.length / 2];
    for (int i = 0; i < coords.length; i++) {
      coords[i] = new Coordinate(lonLat[i * 2], lonLat[i * 2 + 1]);
    }
    return FACTORY.createLineString(coords);
  }

  @Test
  void lengthOfASingleTrack() {
    // 0.1° of latitude ≈ 11.1 km.
    var spine = LineSpine.of(List.of(line(72.85, 19.00, 72.85, 19.10)));
    assertEquals(11_120, spine.lengthMeters(), 150);
  }

  @Test
  void parallelTracksCountOnce() {
    // Up and down tracks ~10 m apart are one alignment.
    var up = line(72.8500, 19.00, 72.8500, 19.05);
    var down = line(72.8501, 19.05, 72.8501, 19.00);
    var spine = LineSpine.of(List.of(up, down));
    assertEquals(5_560, spine.lengthMeters(), 150);
  }

  @Test
  void stationsAreOrderedAlongTheLine() {
    var spine = LineSpine.of(List.of(line(72.85, 19.00, 72.85, 19.05), line(72.85, 19.05, 72.90, 19.05)));
    double south = spine.position(72.8502, 19.01);
    double corner = spine.position(72.8502, 19.049);
    double east = spine.position(72.89, 19.0502);
    assertTrue(south < corner && corner < east || south > corner && corner > east,
      "positions " + south + ", " + corner + ", " + east + " are not monotonic");
  }

  @Test
  void distanceToTrack() {
    var spine = LineSpine.of(List.of(line(72.85, 19.00, 72.85, 19.05)));
    // 0.001° of longitude at 19° N ≈ 105 m.
    assertEquals(105, spine.distanceMeters(72.851, 19.02), 5);
    assertEquals(0, spine.distanceMeters(72.85, 19.02), 1);
  }

  @Test
  void emptyInputIsRejected() {
    assertThrows(IllegalArgumentException.class, () -> LineSpine.of(List.of()));
  }
}
