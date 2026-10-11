package bombaychitra.pipeline.geo;

import com.onthegomap.planetiler.geo.GeoUtils;
import java.util.ArrayList;
import java.util.Collection;
import java.util.List;
import java.util.concurrent.ConcurrentLinkedQueue;
import org.locationtech.jts.algorithm.Orientation;
import org.locationtech.jts.geom.Coordinate;
import org.locationtech.jts.geom.Envelope;
import org.locationtech.jts.geom.Geometry;
import org.locationtech.jts.geom.GeometryFactory;
import org.locationtech.jts.geom.LineSegment;
import org.locationtech.jts.geom.LineString;
import org.locationtech.jts.geom.Polygon;
import org.locationtech.jts.index.strtree.STRtree;
import org.locationtech.jts.operation.polygonize.Polygonizer;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * Builds ocean polygons for a bounded region directly from OSM {@code natural=coastline} ways.
 *
 * <p>This avoids downloading the ~1 GB global water-polygon shapefile for a city-sized extract. The coastline and the
 * clip rectangle are noded together and polygonised; each face is classified as water when it lies on the right-hand
 * side of the coastline (OSM convention: land on the left). All geometry is in Planetiler world coordinates (y grows
 * southwards), where "right of travel" corresponds to a positive cross product.
 */
public final class Coastline {

  private static final Logger LOGGER = LoggerFactory.getLogger(Coastline.class);
  private static final double ON_LINE_TOLERANCE = 1e-10;

  private final Envelope worldBounds;
  private final Collection<Geometry> lines = new ConcurrentLinkedQueue<>();

  /** @param latLonBounds region bounds; expanded slightly so ocean edges fall outside the visible area */
  public Coastline(Envelope latLonBounds) {
    Envelope expanded = new Envelope(latLonBounds);
    expanded.expandBy(0.05);
    this.worldBounds = GeoUtils.toWorldBounds(expanded);
  }

  /** Thread-safe: adds one coastline way (world coordinates). */
  public void add(Geometry worldLine) {
    if (worldLine != null && !worldLine.isEmpty() && worldLine.getEnvelopeInternal().intersects(worldBounds)) {
      lines.add(worldLine);
    }
  }

  public boolean isEmpty() {
    return lines.isEmpty();
  }

  /** Returns water polygons covering the sea inside the (expanded) bounds, in world coordinates. */
  public List<Polygon> buildOcean() {
    if (lines.isEmpty()) {
      return List.of();
    }
    GeometryFactory factory = GeoUtils.JTS_FACTORY;
    Polygon box = (Polygon) factory.toGeometry(worldBounds);

    List<Geometry> clipped = new ArrayList<>();
    STRtree segments = new STRtree();
    for (Geometry line : lines) {
      Geometry part = line.intersection(box);
      if (part.isEmpty()) {
        continue;
      }
      clipped.add(part);
      for (int i = 0; i < part.getNumGeometries(); i++) {
        if (part.getGeometryN(i) instanceof LineString ls) {
          Coordinate[] c = ls.getCoordinates();
          for (int j = 0; j + 1 < c.length; j++) {
            var segment = new LineSegment(c[j], c[j + 1]);
            segments.insert(new Envelope(c[j], c[j + 1]), segment);
          }
        }
      }
    }
    if (clipped.isEmpty()) {
      return List.of();
    }
    clipped.add(box.getExteriorRing());
    Geometry noded = factory.buildGeometry(clipped).union();

    var polygonizer = new Polygonizer();
    polygonizer.add(noded);
    List<Polygon> water = new ArrayList<>();
    int unknown = 0;
    for (Object face : polygonizer.getPolygons()) {
      Polygon polygon = (Polygon) face;
      Boolean isWater = classify(polygon, segments);
      if (isWater == null) {
        unknown++;
      } else if (isWater) {
        water.add(polygon);
      }
    }
    LOGGER.info("Coastline: {} ways → {} ocean polygons ({} unclassified faces)", lines.size(), water.size(), unknown);
    return water;
  }

  /**
   * Finds a boundary edge of {@code face} that lies on a coastline segment and compares directions: if the face is on
   * the right-hand side of the coastline it is water.
   */
  private static Boolean classify(Polygon face, STRtree segments) {
    Coordinate[] ring = face.getExteriorRing().getCoordinates();
    boolean ccw = Orientation.isCCW(ring);
    for (int i = 0; i + 1 < ring.length; i++) {
      Coordinate a = ring[i];
      Coordinate b = ring[i + 1];
      Coordinate mid = new Coordinate((a.x + b.x) / 2, (a.y + b.y) / 2);
      Envelope probe = new Envelope(mid);
      probe.expandBy(ON_LINE_TOLERANCE);
      for (Object item : segments.query(probe)) {
        LineSegment seg = (LineSegment) item;
        if (seg.distance(mid) > ON_LINE_TOLERANCE) {
          continue;
        }
        double dot = (b.x - a.x) * (seg.p1.x - seg.p0.x) + (b.y - a.y) * (seg.p1.y - seg.p0.y);
        if (dot == 0) {
          continue;
        }
        // Interior of a CCW ring lies on the positive-cross side of each edge. Water lies on the positive-cross
        // side of the coastline in world coordinates.
        boolean faceOnPositiveSideOfEdge = ccw;
        boolean edgeAlongCoastline = dot > 0;
        return faceOnPositiveSideOfEdge == edgeAlongCoastline;
      }
    }
    return null;
  }
}
