package bombaychitra.pipeline.data;

import bombaychitra.pipeline.util.Geo;
import com.carrotsearch.hppc.ByteArrayList;
import com.carrotsearch.hppc.FloatArrayList;
import com.carrotsearch.hppc.IntArrayList;
import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.io.UncheckedIOException;
import java.nio.ByteBuffer;
import java.nio.ByteOrder;
import java.nio.charset.StandardCharsets;
import java.sql.Connection;
import java.sql.SQLException;
import java.util.Arrays;
import java.util.Collection;
import java.util.HashMap;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.zip.GZIPOutputStream;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * The car routing graph behind "distance by road", stored in the {@code routing} table as gzip-compressed
 * little-endian arrays (format in {@code docs/database.md}).
 *
 * <p>Ways are split wherever they share a node with another way (or end), giving vertices and road segments. A
 * segment keeps its full shape and can be driven forward, backward or both ways; directed edges in compressed sparse
 * row form point from each vertex to its neighbours.
 */
public final class RoadGraph {

  private static final Logger LOGGER = LoggerFactory.getLogger(RoadGraph.class);
  public static final int VERSION = 1;

  private RoadGraph() {}

  private static long key(int lonE7, int latE7) {
    return ((long) lonE7 << 32) | (latE7 & 0xffffffffL);
  }

  public static void write(Connection db, Collection<Records.DriveWay> ways, Map<String, String> meta)
    throws SQLException {
    if (ways.isEmpty()) {
      return;
    }
    long start = System.nanoTime();

    // 1. Vertices: coordinates used by two or more ways (or twice by one), plus every way's ends.
    int total = 0;
    for (var way : ways) {
      total += way.coords().length / 2 + 2;
    }
    long[] keys = new long[total];
    int k = 0;
    for (var way : ways) {
      int[] c = way.coords();
      for (int i = 0; i < c.length; i += 2) {
        keys[k++] = key(c[i], c[i + 1]);
      }
      keys[k++] = key(c[0], c[1]);
      keys[k++] = key(c[c.length - 2], c[c.length - 1]);
    }
    Arrays.sort(keys);
    int vertexCount = 0;
    for (int i = 1; i < keys.length; i++) {
      if (keys[i] == keys[i - 1] && (vertexCount == 0 || keys[vertexCount - 1] != keys[i])) {
        keys[vertexCount++] = keys[i];
      }
    }
    long[] vertices = Arrays.copyOf(keys, vertexCount);
    keys = null;

    // 2. Segments between consecutive vertices along each way.
    Map<String, Integer> nameIndex = new HashMap<>();
    IntArrayList segFrom = new IntArrayList();
    IntArrayList segTo = new IntArrayList();
    FloatArrayList segLen = new FloatArrayList();
    FloatArrayList segSpeed = new FloatArrayList();
    IntArrayList segShape = new IntArrayList();
    IntArrayList shape = new IntArrayList();
    ByteArrayList segClass = new ByteArrayList();
    ByteArrayList segDir = new ByteArrayList();
    IntArrayList segName = new IntArrayList();
    segShape.add(0);
    for (var way : ways) {
      int[] c = way.coords();
      int points = c.length / 2;
      int first = 0;
      int from = Arrays.binarySearch(vertices, key(c[0], c[1]));
      double length = 0;
      for (int i = 1; i < points; i++) {
        length += Geo.haversine(c[(i - 1) * 2] / 1e7, c[(i - 1) * 2 + 1] / 1e7, c[i * 2] / 1e7, c[i * 2 + 1] / 1e7);
        int v = Arrays.binarySearch(vertices, key(c[i * 2], c[i * 2 + 1]));
        if (v < 0) {
          continue;
        }
        if (v != from && length > 0) {
          segFrom.add(from);
          segTo.add(v);
          segLen.add((float) length);
          segSpeed.add(way.speedKmh());
          segClass.add(way.roadClass());
          segDir.add((byte) (way.oneway() > 0 ? 1 : way.oneway() < 0 ? 2 : 3));
          segName.add(way.name() == null ? -1 : nameIndex.computeIfAbsent(way.name(), n -> nameIndex.size()));
          // Shape: first point absolute, the rest as deltas (small numbers compress well).
          shape.add(c[first * 2], c[first * 2 + 1]);
          for (int p = first + 1; p <= i; p++) {
            shape.add(c[p * 2] - c[(p - 1) * 2], c[p * 2 + 1] - c[(p - 1) * 2 + 1]);
          }
          segShape.add(shape.size() / 2);
        }
        first = i;
        from = v;
        length = 0;
      }
    }
    int segments = segFrom.size();

    // 3. Directed edges in CSR order.
    int[] degree = new int[vertexCount + 1];
    for (int s = 0; s < segments; s++) {
      byte dir = segDir.get(s);
      if ((dir & 1) != 0) {
        degree[segFrom.get(s) + 1]++;
      }
      if ((dir & 2) != 0) {
        degree[segTo.get(s) + 1]++;
      }
    }
    for (int i = 0; i < vertexCount; i++) {
      degree[i + 1] += degree[i];
    }
    int edges = degree[vertexCount];
    int[] cursor = Arrays.copyOf(degree, vertexCount);
    int[] edgeTo = new int[edges];
    int[] edgeSeg = new int[edges];
    float[] edgeTime = new float[edges];
    for (int s = 0; s < segments; s++) {
      byte dir = segDir.get(s);
      float seconds = segLen.get(s) / (segSpeed.get(s) / 3.6f);
      if ((dir & 1) != 0) {
        int e = cursor[segFrom.get(s)]++;
        edgeTo[e] = segTo.get(s);
        edgeSeg[e] = s;
        edgeTime[e] = seconds;
      }
      if ((dir & 2) != 0) {
        int e = cursor[segTo.get(s)]++;
        edgeTo[e] = segFrom.get(s);
        edgeSeg[e] = ~s;
        edgeTime[e] = seconds;
      }
    }

    int[] nodes = new int[vertexCount * 2];
    for (int i = 0; i < vertexCount; i++) {
      nodes[i * 2] = (int) (vertices[i] >> 32);
      nodes[i * 2 + 1] = (int) vertices[i];
    }
    String[] names = new String[nameIndex.size()];
    nameIndex.forEach((name, index) -> names[index] = name.replace('\n', ' '));

    Map<String, byte[]> blobs = new LinkedHashMap<>();
    blobs.put("nodes", ints(nodes));
    blobs.put("offsets", ints(degree));
    blobs.put("edge_to", ints(edgeTo));
    blobs.put("edge_seg", ints(edgeSeg));
    blobs.put("edge_time", floats(edgeTime));
    blobs.put("seg_len", floats(segLen.toArray()));
    blobs.put("seg_shape", ints(segShape.toArray()));
    blobs.put("shape", ints(shape.toArray()));
    blobs.put("seg_class", segClass.toArray());
    blobs.put("seg_dir", segDir.toArray());
    blobs.put("seg_name", ints(segName.toArray()));
    blobs.put("names", String.join("\n", List.of(names)).getBytes(StandardCharsets.UTF_8));

    long bytes = 0;
    try (var insert = db.prepareStatement("INSERT INTO routing(name, data) VALUES (?, ?)")) {
      for (var blob : blobs.entrySet()) {
        byte[] gz = gzip(blob.getValue());
        bytes += gz.length;
        insert.setString(1, blob.getKey());
        insert.setBytes(2, gz);
        insert.executeUpdate();
      }
    }
    meta.put("routing_version", String.valueOf(VERSION));
    meta.put("routing_profile", "car");
    meta.put("routing_vertices", String.valueOf(vertexCount));
    meta.put("routing_edges", String.valueOf(edges));
    meta.put("routing_segments", String.valueOf(segments));
    LOGGER.info("Routing graph: {} ways → {} vertices, {} segments, {} directed edges, {} MB in {}s", ways.size(),
      vertexCount, segments, edges, bytes / 1_000_000, (System.nanoTime() - start) / 1_000_000_000);
  }

  private static byte[] ints(int[] values) {
    ByteBuffer buffer = ByteBuffer.allocate(values.length * 4).order(ByteOrder.LITTLE_ENDIAN);
    buffer.asIntBuffer().put(values);
    return buffer.array();
  }

  private static byte[] floats(float[] values) {
    ByteBuffer buffer = ByteBuffer.allocate(values.length * 4).order(ByteOrder.LITTLE_ENDIAN);
    buffer.asFloatBuffer().put(values);
    return buffer.array();
  }

  private static byte[] gzip(byte[] data) {
    var out = new ByteArrayOutputStream(data.length / 3 + 64);
    try (var gz = new GZIPOutputStream(out, 1 << 16)) {
      gz.write(data);
    } catch (IOException e) {
      throw new UncheckedIOException(e);
    }
    return out.toByteArray();
  }
}
