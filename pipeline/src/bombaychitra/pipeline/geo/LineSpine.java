package bombaychitra.pipeline.geo;

import java.util.ArrayList;
import java.util.Collection;
import java.util.Collections;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import org.locationtech.jts.densify.Densifier;
import org.locationtech.jts.geom.Coordinate;
import org.locationtech.jts.geom.Envelope;
import org.locationtech.jts.geom.GeometryFactory;
import org.locationtech.jts.geom.LineString;
import org.locationtech.jts.linearref.LengthIndexedLine;
import org.locationtech.jts.operation.distance.IndexedFacetDistance;

/**
 * The single-track "spine" of a transit line whose ways may include both tracks of a double line, gaps and extensions.
 * It measures the route length without counting parallel tracks twice and orders stations along the line.
 *
 * <p>Track is walked in {@value #STEP_M} m steps, longest piece first; a step within {@value #SAME_ALIGNMENT_M} m of
 * track already walked (another piece, or an earlier part of the same piece) belongs to an alignment that is already
 * covered. The remaining runs are chained end to end, nearest ends first. Computation is in a local equirectangular
 * projection, accurate to well under 1% across a metropolitan region.
 */
public final class LineSpine {

  private static final double SAME_ALIGNMENT_M = 40;
  private static final double STEP_M = 20;
  private static final double MIN_RUN_M = 60;
  /** Steps of the same piece closer than this along the track are neighbours, not a returning parallel track. */
  private static final double SELF_GAP_M = 300;
  private static final GeometryFactory FACTORY = new GeometryFactory();

  private final double lon0;
  private final double lat0;
  private final double kx;
  private final double ky;
  private final double lengthM;
  private final LengthIndexedLine chain;
  private final IndexedFacetDistance tracks;

  private record Sample(double x, double y, int piece, double along) {}

  private record Run(List<Coordinate> coords, double length) {

    Coordinate first() {
      return coords.getFirst();
    }

    Coordinate last() {
      return coords.getLast();
    }
  }

  private LineSpine(double lon0, double lat0, List<LineString> pieces) {
    this.lon0 = lon0;
    this.lat0 = lat0;
    this.kx = 111_320 * Math.cos(Math.toRadians(lat0));
    this.ky = 110_574;

    List<LineString> projected = new ArrayList<>();
    for (LineString piece : pieces) {
      Coordinate[] c = piece.getCoordinates();
      Coordinate[] p = new Coordinate[c.length];
      for (int i = 0; i < c.length; i++) {
        p[i] = project(c[i].x, c[i].y);
      }
      if (p.length >= 2) {
        projected.add(FACTORY.createLineString(p));
      }
    }
    projected.sort((a, b) -> Double.compare(b.getLength(), a.getLength()));
    this.tracks = new IndexedFacetDistance(FACTORY.createMultiLineString(projected.toArray(LineString[]::new)));

    Map<Long, List<Sample>> grid = new HashMap<>();
    List<Run> runs = new ArrayList<>();
    double total = 0;
    for (int piece = 0; piece < projected.size(); piece++) {
      Coordinate[] c = Densifier.densify(projected.get(piece), STEP_M).getCoordinates();
      List<Coordinate> run = null;
      double runLength = 0;
      double along = 0;
      for (int i = 0; i + 1 < c.length; i++) {
        double step = c[i].distance(c[i + 1]);
        double mx = (c[i].x + c[i + 1].x) / 2;
        double my = (c[i].y + c[i + 1].y) / 2;
        double mid = along + step / 2;
        if (covered(grid, mx, my, piece, mid)) {
          if (run != null && runLength >= MIN_RUN_M) {
            runs.add(new Run(run, runLength));
            total += runLength;
          }
          run = null;
          runLength = 0;
        } else {
          if (run == null) {
            run = new ArrayList<>();
            run.add(c[i]);
          }
          run.add(c[i + 1]);
          runLength += step;
        }
        grid.computeIfAbsent(cell(mx, my), k -> new ArrayList<>()).add(new Sample(mx, my, piece, mid));
        along += step;
      }
      if (run != null && runLength >= MIN_RUN_M) {
        runs.add(new Run(run, runLength));
        total += runLength;
      }
    }
    this.lengthM = total;
    this.chain = new LengthIndexedLine(FACTORY.createLineString(chain(runs).toArray(Coordinate[]::new)));
  }

  /** Builds the spine of lon/lat line strings (typically the output of a {@code LineMerger}). */
  public static LineSpine of(Collection<LineString> lonLatLines) {
    if (lonLatLines.isEmpty()) {
      throw new IllegalArgumentException("no track");
    }
    Envelope env = new Envelope();
    lonLatLines.forEach(l -> env.expandToInclude(l.getEnvelopeInternal()));
    return new LineSpine(env.centre().x, env.centre().y, List.copyOf(lonLatLines));
  }

  /** Route length in metres, counting each alignment once. */
  public double lengthMeters() {
    return lengthM;
  }

  /** Distance in metres from a lon/lat point to the nearest track. */
  public double distanceMeters(double lon, double lat) {
    return tracks.distance(FACTORY.createPoint(project(lon, lat)));
  }

  /** Position of a lon/lat point along the spine, for ordering stations from one end of the line to the other. */
  public double position(double lon, double lat) {
    return chain.project(project(lon, lat));
  }

  private Coordinate project(double lon, double lat) {
    return new Coordinate((lon - lon0) * kx, (lat - lat0) * ky);
  }

  private static long cell(double x, double y) {
    return (long) Math.floor(x / SAME_ALIGNMENT_M) * 1_000_003L + (long) Math.floor(y / SAME_ALIGNMENT_M);
  }

  private static boolean covered(Map<Long, List<Sample>> grid, double x, double y, int piece, double along) {
    long cx = (long) Math.floor(x / SAME_ALIGNMENT_M);
    long cy = (long) Math.floor(y / SAME_ALIGNMENT_M);
    for (long dx = -1; dx <= 1; dx++) {
      for (long dy = -1; dy <= 1; dy++) {
        for (Sample s : grid.getOrDefault((cx + dx) * 1_000_003L + cy + dy, List.of())) {
          if (Math.hypot(s.x() - x, s.y() - y) <= SAME_ALIGNMENT_M
            && (s.piece() != piece || along - s.along() > SELF_GAP_M)) {
            return true;
          }
        }
      }
    }
    return false;
  }

  /** Joins runs into one line, longest first, repeatedly attaching the run whose end is nearest a chain end. */
  private static List<Coordinate> chain(List<Run> runs) {
    if (runs.isEmpty()) {
      return List.of(new Coordinate(0, 0), new Coordinate(0, 0));
    }
    List<Run> remaining = new ArrayList<>(runs);
    remaining.sort((a, b) -> Double.compare(b.length(), a.length()));
    List<Coordinate> chain = new ArrayList<>(remaining.removeFirst().coords());
    while (!remaining.isEmpty()) {
      int best = -1;
      int how = 0;
      double bestDistance = Double.POSITIVE_INFINITY;
      for (int i = 0; i < remaining.size(); i++) {
        Run r = remaining.get(i);
        double[] d = {
          chain.getLast().distance(r.first()), chain.getLast().distance(r.last()),
          chain.getFirst().distance(r.last()), chain.getFirst().distance(r.first())
        };
        for (int k = 0; k < d.length; k++) {
          if (d[k] < bestDistance) {
            bestDistance = d[k];
            best = i;
            how = k;
          }
        }
      }
      List<Coordinate> coords = new ArrayList<>(remaining.remove(best).coords());
      if (how == 1 || how == 3) {
        Collections.reverse(coords);
      }
      if (how <= 1) {
        chain.addAll(coords);
      } else {
        chain.addAll(0, coords);
      }
    }
    return chain;
  }
}
