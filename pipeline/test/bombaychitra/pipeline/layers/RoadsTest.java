package bombaychitra.pipeline.layers;

import static org.junit.jupiter.api.Assertions.assertEquals;

import bombaychitra.pipeline.data.RoadGraph;
import com.onthegomap.planetiler.reader.SimpleFeature;
import java.util.HashMap;
import org.junit.jupiter.api.Test;
import org.locationtech.jts.geom.Coordinate;
import org.locationtech.jts.geom.GeometryFactory;

/** Who may use which way in the routing graph, and the flags segments carry. */
class RoadsTest {

  private static final int CAR = RoadGraph.CAR;
  private static final int TWO = RoadGraph.TWO_WHEELER;
  private static final int FOOT = RoadGraph.FOOT;

  private static SimpleFeature way(String... keyValues) {
    var tags = new HashMap<String, Object>();
    for (int i = 0; i < keyValues.length; i += 2) {
      tags.put(keyValues[i], keyValues[i + 1]);
    }
    var line = new GeometryFactory().createLineString(new Coordinate[]{new Coordinate(0, 0), new Coordinate(1, 1)});
    return SimpleFeature.create(line, tags);
  }

  private static int modes(String... keyValues) {
    var f = way(keyValues);
    return Roads.modes(f, f.getString("highway"));
  }

  private static int flags(String... keyValues) {
    var f = way(keyValues);
    return Roads.flags(f, f.getString("highway"));
  }

  @Test
  void ordinaryStreetsAreOpenToEveryone() {
    assertEquals(CAR | TWO | FOOT, modes("highway", "residential"));
    assertEquals(CAR | TWO | FOOT, modes("highway", "primary", "name", "S V Road"));
  }

  @Test
  void motorwaysAreForCarsOnly() {
    // Atal Setu, the sea link and the freeways: no two-wheelers, no pedestrians.
    assertEquals(CAR, modes("highway", "motorway", "name", "Atal Setu", "toll", "yes"));
    assertEquals(CAR, modes("highway", "motorway_link"));
    assertEquals(CAR, modes("highway", "trunk", "motorcycle", "no", "foot", "no"));
    // Explicit permission wins over the motorway default.
    assertEquals(CAR | TWO, modes("highway", "motorway", "motorcycle", "yes"));
  }

  @Test
  void footpathsAreForPedestrians() {
    assertEquals(FOOT, modes("highway", "footway", "bridge", "yes"));
    assertEquals(FOOT, modes("highway", "steps"));
    assertEquals(FOOT, modes("highway", "pedestrian", "name", "Fashion Street"));
    assertEquals(FOOT, modes("highway", "footway", "access", "no", "foot", "yes"));
    assertEquals(0, modes("highway", "footway", "foot", "no"));
  }

  @Test
  void accessTags() {
    assertEquals(0, modes("highway", "service", "access", "private"));
    assertEquals(FOOT, modes("highway", "service", "motor_vehicle", "no"));
    assertEquals(TWO | FOOT, modes("highway", "residential", "motorcar", "no"));
    assertEquals(FOOT, modes("highway", "service", "service", "emergency_access"));
    assertEquals(0, modes("highway", "busway"));
  }

  @Test
  void segmentFlags() {
    assertEquals(RoadGraph.TOLL | RoadGraph.BRIDGE, flags("highway", "motorway", "toll", "yes", "bridge", "yes"));
    assertEquals(RoadGraph.STEPS, flags("highway", "steps"));
    assertEquals(RoadGraph.TUNNEL, flags("highway", "primary", "tunnel", "yes"));
    assertEquals(RoadGraph.ROUNDABOUT, flags("highway", "tertiary", "junction", "roundabout"));
    assertEquals(RoadGraph.LINK, flags("highway", "trunk_link"));
    assertEquals(0, flags("highway", "residential", "toll", "no"));
  }
}
