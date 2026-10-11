package bombaychitra.pipeline.util;

import org.locationtech.jts.geom.Coordinate;
import org.locationtech.jts.geom.Geometry;
import org.locationtech.jts.geom.GeometryCollection;
import org.locationtech.jts.geom.LineString;
import org.locationtech.jts.geom.MultiLineString;
import org.locationtech.jts.geom.MultiPoint;
import org.locationtech.jts.geom.MultiPolygon;
import org.locationtech.jts.geom.Point;
import org.locationtech.jts.geom.Polygon;

/** Compact GeoJSON geometry writer for lon/lat JTS geometries (coordinates rounded to 6 decimals ≈ 0.1 m). */
public final class GeoJson {

  private GeoJson() {}

  public static String write(Geometry geometry) {
    if (geometry == null || geometry.isEmpty()) {
      return null;
    }
    var sb = new StringBuilder(64 + geometry.getNumPoints() * 22);
    append(sb, geometry);
    return sb.toString();
  }

  private static void append(StringBuilder sb, Geometry g) {
    switch (g) {
      case Point p -> {
        sb.append("{\"type\":\"Point\",\"coordinates\":");
        coord(sb, p.getCoordinate());
        sb.append('}');
      }
      case LineString l -> {
        sb.append("{\"type\":\"LineString\",\"coordinates\":");
        coords(sb, l.getCoordinates());
        sb.append('}');
      }
      case Polygon p -> {
        sb.append("{\"type\":\"Polygon\",\"coordinates\":");
        polygon(sb, p);
        sb.append('}');
      }
      case MultiPoint mp -> {
        sb.append("{\"type\":\"MultiPoint\",\"coordinates\":");
        coords(sb, mp.getCoordinates());
        sb.append('}');
      }
      case MultiLineString ml -> {
        sb.append("{\"type\":\"MultiLineString\",\"coordinates\":[");
        for (int i = 0; i < ml.getNumGeometries(); i++) {
          if (i > 0) {
            sb.append(',');
          }
          coords(sb, ml.getGeometryN(i).getCoordinates());
        }
        sb.append("]}");
      }
      case MultiPolygon mp -> {
        sb.append("{\"type\":\"MultiPolygon\",\"coordinates\":[");
        for (int i = 0; i < mp.getNumGeometries(); i++) {
          if (i > 0) {
            sb.append(',');
          }
          polygon(sb, (Polygon) mp.getGeometryN(i));
        }
        sb.append("]}");
      }
      case GeometryCollection gc -> {
        sb.append("{\"type\":\"GeometryCollection\",\"geometries\":[");
        for (int i = 0; i < gc.getNumGeometries(); i++) {
          if (i > 0) {
            sb.append(',');
          }
          append(sb, gc.getGeometryN(i));
        }
        sb.append("]}");
      }
      default -> throw new IllegalArgumentException("Unsupported geometry " + g.getGeometryType());
    }
  }

  private static void polygon(StringBuilder sb, Polygon p) {
    sb.append('[');
    coords(sb, p.getExteriorRing().getCoordinates());
    for (int i = 0; i < p.getNumInteriorRing(); i++) {
      sb.append(',');
      coords(sb, p.getInteriorRingN(i).getCoordinates());
    }
    sb.append(']');
  }

  private static void coords(StringBuilder sb, Coordinate[] coords) {
    sb.append('[');
    for (int i = 0; i < coords.length; i++) {
      if (i > 0) {
        sb.append(',');
      }
      coord(sb, coords[i]);
    }
    sb.append(']');
  }

  private static void coord(StringBuilder sb, Coordinate c) {
    sb.append('[');
    number(sb, c.x);
    sb.append(',');
    number(sb, c.y);
    sb.append(']');
  }

  /** Appends a number with at most 6 decimals and no trailing zeros, without allocating a formatter. */
  static void number(StringBuilder sb, double value) {
    long scaled = Math.round(value * 1_000_000d);
    if (scaled < 0) {
      sb.append('-');
      scaled = -scaled;
    }
    long whole = scaled / 1_000_000;
    long frac = scaled % 1_000_000;
    sb.append(whole);
    if (frac != 0) {
      sb.append('.');
      String digits = Long.toString(frac);
      for (int i = digits.length(); i < 6; i++) {
        sb.append('0');
      }
      int end = digits.length();
      while (end > 0 && digits.charAt(end - 1) == '0') {
        end--;
      }
      sb.append(digits, 0, end);
    }
  }
}
