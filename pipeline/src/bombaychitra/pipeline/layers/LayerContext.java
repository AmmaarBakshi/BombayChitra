package bombaychitra.pipeline.layers;

import bombaychitra.pipeline.RegionConfig;
import bombaychitra.pipeline.Taxonomy;
import bombaychitra.pipeline.data.DataSink;
import com.onthegomap.planetiler.geo.GeoUtils;
import com.onthegomap.planetiler.geo.GeometryException;
import com.onthegomap.planetiler.reader.SourceFeature;
import org.locationtech.jts.geom.Coordinate;
import org.locationtech.jts.geom.Geometry;

/** Shared, immutable state handed to every layer handler. */
public record LayerContext(RegionConfig region, Taxonomy taxonomy, DataSink sink, int maxzoom) {

  /** True when any part of the feature falls inside the region bounds. */
  public boolean inBounds(SourceFeature feature) {
    try {
      return region.intersects(feature.latLonGeometry().getEnvelopeInternal());
    } catch (GeometryException e) {
      return false;
    }
  }

  /** Representative lon/lat point: the point itself, or a point guaranteed to lie on the line/polygon. */
  public Coordinate anchor(SourceFeature feature) throws GeometryException {
    Geometry world = feature.isPoint() ? feature.worldGeometry()
      : feature.canBePolygon() ? feature.pointOnSurface() : feature.lineMidpoint();
    Coordinate c = world.getCoordinate();
    return new Coordinate(GeoUtils.getWorldLon(c.x), GeoUtils.getWorldLat(c.y));
  }
}
