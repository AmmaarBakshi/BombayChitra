package bombaychitra.pipeline.data;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNull;

import java.util.List;
import org.junit.jupiter.api.Test;
import org.locationtech.jts.geom.Coordinate;
import org.locationtech.jts.geom.GeometryFactory;

/** How OSM route relations become line labels and keys, and when a line counts as not yet open. */
class TransitLinesTest {

  private static Records.Route route(String mode, String network, String ref, String name, String label) {
    return new Records.Route(1, mode, ref, name, label, null, network, null, null, null, null, List.of());
  }

  @Test
  void labelsDropDirectionsAndQualifiers() {
    assertEquals("Line 2", DatabaseWriter.lineLabel(route("subway", "Mumbai Metro", "2", "Line 2 (Andheri West → Dahisar (East))", null)));
    assertEquals("Western Line", DatabaseWriter.lineLabel(route("train", null, null, "Western Line: Churchgate => Virar", null)));
    assertEquals("Aqua Line", DatabaseWriter.lineLabel(route("subway", null, "3", "Aqua Line (Line 3)", null)));
    assertEquals("Metro 7", DatabaseWriter.lineLabel(route("subway", null, "7", null, null)));
    assertEquals("Line 2A", DatabaseWriter.lineLabel(route("subway", null, "2", "Line 2", "Line 2A")));
  }

  @Test
  void keysSeparateLinesNotYetOpen() {
    var line9 = route("subway", "Mumbai Metro", "9", "Line 9", null);
    assertEquals("Mumbai Metro:9", DatabaseWriter.lineKey(line9, null));
    assertEquals("Mumbai Metro:9:construction", DatabaseWriter.lineKey(line9, "construction"));
    assertEquals("Mumbai Metro:9:proposed", DatabaseWriter.lineKey(line9, "proposed"));
    assertEquals("monorail:Monorail", DatabaseWriter.lineKey(route("monorail", null, null, null, "Monorail"), null));
  }

  @Test
  void lifecycleFromTracks() {
    var f = new GeometryFactory();
    var line = f.createLineString(new Coordinate[]{new Coordinate(72.8, 19.0), new Coordinate(72.9, 19.1)});
    var open = new Records.RouteWay(1, 10, line, null);
    var building = new Records.RouteWay(1, 11, line, "construction");
    var planned = new Records.RouteWay(1, 12, line, "proposed");
    // Any track in service means the line is in service.
    assertNull(DatabaseWriter.lifecycle(List.of(open, building)));
    assertEquals("construction", DatabaseWriter.lifecycle(List.of(building, planned)));
    assertEquals("proposed", DatabaseWriter.lifecycle(List.of(planned)));
  }
}
