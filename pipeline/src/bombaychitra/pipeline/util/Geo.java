package bombaychitra.pipeline.util;

import org.locationtech.jts.geom.Coordinate;
import org.locationtech.jts.geom.Envelope;
import org.locationtech.jts.geom.Geometry;

/** Small geodesic helpers for lon/lat geometries. */
public final class Geo {

  private static final double EARTH_RADIUS_M = 6_371_008.8;

  private Geo() {}

  public static double haversine(double lon1, double lat1, double lon2, double lat2) {
    double dLat = Math.toRadians(lat2 - lat1);
    double dLon = Math.toRadians(lon2 - lon1);
    double a = Math.sin(dLat / 2) * Math.sin(dLat / 2)
      + Math.cos(Math.toRadians(lat1)) * Math.cos(Math.toRadians(lat2)) * Math.sin(dLon / 2) * Math.sin(dLon / 2);
    return 2 * EARTH_RADIUS_M * Math.asin(Math.min(1, Math.sqrt(a)));
  }

  /** Length in metres of all line work in a lon/lat geometry. */
  public static double lengthMeters(Geometry geometry) {
    double total = 0;
    for (int i = 0; i < geometry.getNumGeometries(); i++) {
      Coordinate[] c = geometry.getGeometryN(i).getCoordinates();
      for (int j = 0; j + 1 < c.length; j++) {
        total += haversine(c[j].x, c[j].y, c[j + 1].x, c[j + 1].y);
      }
    }
    return total;
  }

  /** Approximate area in km² of a lon/lat polygon (equirectangular scaling at its centre latitude). */
  public static double areaKm2(Geometry geometry) {
    Envelope env = geometry.getEnvelopeInternal();
    double kmPerDegLat = 111.32;
    double kmPerDegLon = 111.32 * Math.cos(Math.toRadians((env.getMinY() + env.getMaxY()) / 2));
    return geometry.getArea() * kmPerDegLat * kmPerDegLon;
  }
}
