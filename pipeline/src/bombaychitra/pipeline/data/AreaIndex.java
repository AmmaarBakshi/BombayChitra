package bombaychitra.pipeline.data;

import java.util.ArrayList;
import java.util.Comparator;
import java.util.List;
import org.locationtech.jts.geom.Coordinate;
import org.locationtech.jts.geom.Envelope;
import org.locationtech.jts.geom.Geometry;
import org.locationtech.jts.geom.GeometryFactory;
import org.locationtech.jts.geom.Point;
import org.locationtech.jts.geom.prep.PreparedGeometry;
import org.locationtech.jts.geom.prep.PreparedGeometryFactory;
import org.locationtech.jts.index.strtree.STRtree;

/** Point-in-polygon lookup of configured areas. Thread-safe once constructed. */
final class AreaIndex {

  record Entry(String id, int sort, double areaKm2, Geometry geometry) {}

  private record Indexed(Entry entry, PreparedGeometry prepared) {}

  private static final GeometryFactory FACTORY = new GeometryFactory();

  private final STRtree tree = new STRtree();
  private final List<Entry> entries;

  AreaIndex(List<Entry> entries) {
    this.entries = List.copyOf(entries);
    Point probe = FACTORY.createPoint(new Coordinate(0, 0));
    for (Entry entry : entries) {
      var prepared = PreparedGeometryFactory.prepare(entry.geometry());
      prepared.contains(probe); // build internal indexes before concurrent use
      tree.insert(entry.geometry().getEnvelopeInternal(), new Indexed(entry, prepared));
    }
    tree.build();
  }

  List<Entry> entries() {
    return entries;
  }

  /** Areas containing the point, ordered as configured. */
  List<String> at(double lon, double lat) {
    Point point = FACTORY.createPoint(new Coordinate(lon, lat));
    List<Indexed> hits = new ArrayList<>();
    for (Object item : tree.query(new Envelope(lon, lon, lat, lat))) {
      Indexed indexed = (Indexed) item;
      if (indexed.prepared().contains(point)) {
        hits.add(indexed);
      }
    }
    hits.sort(Comparator.comparingInt(i -> i.entry().sort()));
    List<String> ids = new ArrayList<>(hits.size());
    hits.forEach(i -> ids.add(i.entry().id()));
    return ids;
  }
}
