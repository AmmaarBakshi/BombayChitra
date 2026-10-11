package bombaychitra.pipeline.data;

import com.onthegomap.planetiler.reader.osm.OsmRelationInfo;
import java.util.List;
import org.locationtech.jts.geom.Geometry;

/** Immutable rows collected while Planetiler streams the OSM extract; written to SQLite afterwards. */
public final class Records {

  private Records() {}

  /** A categorised point of interest. Geometry is reduced to a representative lon/lat point. */
  public record Place(long id, String category, String group, String name, double lon, double lat, double areaM2,
    String tagsJson) {}

  /** A named or numbered road segment (one OSM way). */
  public record Road(long id, String name, String ref, String kind, String highway, Geometry line, double lengthM,
    String tagsJson) {}

  /** A bridge way or bridge outline polygon. */
  public record Bridge(long id, String name, String kind, String cls, Geometry geometry, double lengthM,
    double areaM2, String tagsJson) {}

  /** A building with at least one descriptive attribute (name, height, levels, address or a specific type). */
  public record Building(long id, String name, String kind, Integer levels, Double heightM, Double minHeightM,
    double areaM2, double lon, double lat, String tagsJson) {}

  /** A populated place / locality label. */
  public record Locality(long id, String name, String kind, double lon, double lat, Long population,
    String tagsJson) {}

  /** A transit line: an OSM route relation captured in the relation pre-pass, or a configured track line. */
  public record Route(long id, String mode, String ref, String name, String label, String colour, String network,
    String operator, String from, String to, String state, List<Long> stopNodeIds) implements OsmRelationInfo {

    /** Lines assembled from tracks ({@code transit.trackLines}) use negative placeholder ids. */
    public boolean fromTracks() {
      return id < 0;
    }
  }

  /**
   * One track way of a line (lon/lat). {@code lifecycle} is {@code construction} or {@code proposed} for tracks that
   * are not yet in service, otherwise null.
   */
  public record RouteWay(long routeId, long wayId, Geometry line, String lifecycle) {}

  /** A stop or platform node belonging to a route relation. */
  public record RouteStop(long routeId, long nodeId, String name, double lon, double lat) {}

  /**
   * A way cars can drive on, for the routing graph: coordinates as interleaved lon/lat × 10<sup>7</sup> (exact, so
   * shared OSM nodes match), {@code oneway} 1 forward, -1 backward, 0 both, and a typical speed in km/h.
   */
  public record DriveWay(int[] coords, byte roadClass, byte oneway, float speedKmh, String name) {}

  /** One polygon making up a configured area (lon/lat). */
  public record AreaPart(String areaId, long relationId, Geometry polygon) {}
}
