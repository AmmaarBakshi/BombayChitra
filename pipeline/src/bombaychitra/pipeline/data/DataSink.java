package bombaychitra.pipeline.data;

import java.util.ArrayList;
import java.util.Collection;
import java.util.List;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.ConcurrentLinkedQueue;

/**
 * Thread-safe accumulator for everything the database needs. Planetiler calls layer handlers from many threads, so
 * every collection here is lock-free.
 */
public final class DataSink {

  public final Collection<Records.Place> places = new ConcurrentLinkedQueue<>();
  public final Collection<Records.Road> roads = new ConcurrentLinkedQueue<>();
  public final Collection<Records.Bridge> bridges = new ConcurrentLinkedQueue<>();
  public final Collection<Records.Building> buildings = new ConcurrentLinkedQueue<>();
  public final Collection<Records.Locality> localities = new ConcurrentLinkedQueue<>();
  public final Collection<Records.RouteWay> routeWays = new ConcurrentLinkedQueue<>();
  public final Collection<Records.RouteStop> routeStops = new ConcurrentLinkedQueue<>();
  public final Collection<Records.AreaPart> areaParts = new ConcurrentLinkedQueue<>();
  public final Map<Long, Records.Route> routes = new ConcurrentHashMap<>();

  /** Building statistics are kept for every building, including the ones not stored individually. */
  private final ConcurrentLinkedQueue<double[]> buildingStats = new ConcurrentLinkedQueue<>();

  public void addBuildingStat(double lon, double lat, double heightM, double areaM2) {
    buildingStats.add(new double[]{lon, lat, heightM, areaM2});
  }

  public List<double[]> buildingStats() {
    return new ArrayList<>(buildingStats);
  }

  public void clear() {
    places.clear();
    roads.clear();
    bridges.clear();
    buildings.clear();
    localities.clear();
    routeWays.clear();
    routeStops.clear();
    areaParts.clear();
    routes.clear();
    buildingStats.clear();
  }
}
