package bombaychitra.pipeline.data;

import java.util.ArrayList;
import java.util.Arrays;
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
  public final Collection<Records.DriveWay> driveWays = new ConcurrentLinkedQueue<>();
  public final Map<Long, Records.Route> routes = new ConcurrentHashMap<>();

  /**
   * Statistics for every building, including the millions not stored individually: lon, lat, height (NaN when
   * unknown) and footprint area, packed four floats per building to keep memory flat.
   */
  private final List<float[]> statChunks = new ArrayList<>();
  private float[] statChunk = new float[STAT_CHUNK];
  private int statFill;
  private static final int STAT_CHUNK = 4 << 14;

  public synchronized void addBuildingStat(double lon, double lat, double heightM, double areaM2) {
    if (statFill == STAT_CHUNK) {
      statChunks.add(statChunk);
      statChunk = new float[STAT_CHUNK];
      statFill = 0;
    }
    statChunk[statFill++] = (float) lon;
    statChunk[statFill++] = (float) lat;
    statChunk[statFill++] = (float) heightM;
    statChunk[statFill++] = (float) areaM2;
  }

  /** Building statistics as arrays of (lon, lat, height, area) quadruples. */
  public synchronized List<float[]> buildingStats() {
    List<float[]> all = new ArrayList<>(statChunks);
    all.add(Arrays.copyOf(statChunk, statFill));
    return all;
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
    driveWays.clear();
    routes.clear();
    synchronized (this) {
      statChunks.clear();
      statFill = 0;
    }
  }
}
