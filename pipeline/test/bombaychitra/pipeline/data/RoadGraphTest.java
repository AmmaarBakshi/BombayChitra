package bombaychitra.pipeline.data;

import static org.junit.jupiter.api.Assertions.assertArrayEquals;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.io.ByteArrayInputStream;
import java.io.IOException;
import java.nio.ByteBuffer;
import java.nio.ByteOrder;
import java.sql.Connection;
import java.sql.DriverManager;
import java.util.HashMap;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.zip.GZIPInputStream;
import org.junit.jupiter.api.Test;

/** The routing graph: ways split at shared nodes into segments, with directed edges in CSR order. */
class RoadGraphTest {

  private static int[] coords(double... lonLat) {
    int[] out = new int[lonLat.length];
    for (int i = 0; i < lonLat.length; i++) {
      out[i] = (int) Math.round(lonLat[i] * 1e7);
    }
    return out;
  }

  /** Writes the graph to an in-memory database and returns the decompressed arrays. */
  private static Map<String, byte[]> build(List<Records.DriveWay> ways, Map<String, String> meta) throws Exception {
    try (Connection db = DriverManager.getConnection("jdbc:sqlite::memory:")) {
      db.createStatement().execute("CREATE TABLE routing (name TEXT PRIMARY KEY, data BLOB NOT NULL)");
      RoadGraph.write(db, ways, meta);
      Map<String, byte[]> blobs = new HashMap<>();
      try (var rs = db.createStatement().executeQuery("SELECT name, data FROM routing")) {
        while (rs.next()) {
          blobs.put(rs.getString(1), gunzip(rs.getBytes(2)));
        }
      }
      return blobs;
    }
  }

  private static byte[] gunzip(byte[] data) throws IOException {
    try (var in = new GZIPInputStream(new ByteArrayInputStream(data))) {
      return in.readAllBytes();
    }
  }

  private static int[] ints(byte[] b) {
    int[] out = new int[b.length / 4];
    ByteBuffer.wrap(b).order(ByteOrder.LITTLE_ENDIAN).asIntBuffer().get(out);
    return out;
  }

  @Test
  void crossingWaysAreSplitAtTheSharedNode() throws Exception {
    // A plus sign: two two-way roads crossing at (72.85, 19.05).
    var ns = new Records.DriveWay(coords(72.85, 19.04, 72.85, 19.05, 72.85, 19.06), (byte) 3, (byte) 0, 35, "N-S Road");
    var ew = new Records.DriveWay(coords(72.84, 19.05, 72.85, 19.05, 72.86, 19.05), (byte) 6, (byte) 0, 18, "E-W Road");
    var meta = new LinkedHashMap<String, String>();
    var blobs = build(List.of(ns, ew), meta);

    assertEquals("5", meta.get("routing_vertices"), "four ends and the crossing");
    assertEquals("4", meta.get("routing_segments"));
    assertEquals("8", meta.get("routing_edges"), "every segment both ways");
    int[] offsets = ints(blobs.get("offsets"));
    assertEquals(6, offsets.length);
    assertEquals(8, offsets[5]);
    // The crossing is the only vertex with four outgoing edges.
    int busiest = 0;
    for (int v = 0; v < 5; v++) {
      busiest = Math.max(busiest, offsets[v + 1] - offsets[v]);
    }
    assertEquals(4, busiest);
    assertEquals("N-S Road\nE-W Road", new String(blobs.get("names"), java.nio.charset.StandardCharsets.UTF_8));
  }

  @Test
  void oneWayRoadsOnlyGetForwardEdges() throws Exception {
    var a = new Records.DriveWay(coords(72.80, 19.00, 72.81, 19.00), (byte) 6, (byte) 1, 18, null);
    var b = new Records.DriveWay(coords(72.81, 19.00, 72.82, 19.00), (byte) 6, (byte) -1, 18, null);
    var meta = new LinkedHashMap<String, String>();
    var blobs = build(List.of(a, b), meta);
    assertEquals("3", meta.get("routing_vertices"));
    assertEquals("2", meta.get("routing_edges"));
    assertArrayEquals(new byte[]{1, 2}, blobs.get("seg_dir"));
    int[] segName = ints(blobs.get("seg_name"));
    assertArrayEquals(new int[]{-1, -1}, segName, "unnamed segments");
  }

  @Test
  void shapesKeepIntermediatePointsAsDeltas() throws Exception {
    // A bent road with no junction in the middle stays one segment with three shape points.
    var way = new Records.DriveWay(coords(72.80, 19.00, 72.805, 19.004, 72.81, 19.00), (byte) 5, (byte) 0, 26, "Bend");
    var blobs = build(List.of(way), new LinkedHashMap<>());
    int[] segShape = ints(blobs.get("seg_shape"));
    int[] shape = ints(blobs.get("shape"));
    assertArrayEquals(new int[]{0, 3}, segShape);
    assertEquals(728_000_000, shape[0]);
    assertEquals(50_000, shape[2], "second point is stored relative to the first");
    float length = ByteBuffer.wrap(blobs.get("seg_len")).order(ByteOrder.LITTLE_ENDIAN).getFloat();
    assertTrue(length > 1360 && length < 1395, "two legs of ~689 m, not the 1.05 km chord: " + length);
  }
}
