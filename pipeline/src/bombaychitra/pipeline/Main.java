package bombaychitra.pipeline;

import bombaychitra.pipeline.data.DataSink;
import bombaychitra.pipeline.data.DatabaseWriter;
import bombaychitra.pipeline.layers.LayerContext;
import bombaychitra.pipeline.layers.OvertureBuildings;
import bombaychitra.pipeline.layers.OverturePlaces;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.onthegomap.planetiler.Planetiler;
import com.onthegomap.planetiler.config.Arguments;
import com.onthegomap.planetiler.reader.osm.OsmInputFile;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.time.temporal.ChronoUnit;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * BombayChitra data pipeline: OpenStreetMap extract → {@code bombaychitra.pmtiles} (vector tiles) and
 * {@code bombaychitra.db} (SQLite with spatial and full-text indexes), in a single pass.
 *
 * <pre>
 * java -Xmx4g -cp planetiler.jar pipeline/src/bombaychitra/pipeline/Main.java --download
 * </pre>
 *
 * <p>Options (plus every Planetiler option, e.g. {@code --threads}, {@code --only_layers}):
 * <ul>
 *   <li>{@code --config_dir=config} directory with {@code region.json} and {@code categories.json}</li>
 *   <li>{@code --data_dir=data} working directory ({@code sources/}, {@code build/})</li>
 *   <li>{@code --tiles=…} / {@code --database=…} override output paths</li>
 *   <li>{@code --skip_database} only build tiles</li>
 * </ul>
 */
public final class Main {

  private static final Logger LOGGER = LoggerFactory.getLogger(Main.class);

  private Main() {}

  public static void main(String[] args) throws Exception {
    Arguments arguments = Arguments.fromArgsOrConfigFile(args);
    Path configDir = arguments.file("config_dir", "directory containing region.json and categories.json",
      Path.of("config"));
    Path dataDir = arguments.file("data_dir", "working directory for sources and build output", Path.of("data"));

    RegionConfig region = RegionConfig.load(configDir.resolve("region.json"));
    Taxonomy taxonomy = Taxonomy.load(configDir.resolve("categories.json"));
    Curated curated = Curated.load(configDir);

    Path build = dataDir.resolve("build");
    Path osm = dataDir.resolve("sources").resolve(region.source().file());
    Path tiles = arguments.file("tiles", "PMTiles output", build.resolve("bombaychitra.pmtiles"));
    Path database = arguments.file("database", "SQLite output", build.resolve("bombaychitra.db"));
    boolean skipDatabase = arguments.getBoolean("skip_database", "only build vector tiles", false);
    Files.createDirectories(build);

    arguments = arguments.orElse(Arguments.of(
      "bounds", region.boundsArg(),
      "maxzoom", String.valueOf(region.maxzoom()),
      "render_maxzoom", String.valueOf(region.maxzoom()),
      "tmpdir", build.resolve("tmp").toString()
    ));

    var sink = new DataSink();
    var started = Instant.now();
    var planetiler = Planetiler.create(arguments)
      .setProfile(p -> new BombayChitraProfile(p.config(),
        new LayerContext(region, taxonomy, sink, p.config().maxzoom())))
      .addOsmSource("osm", osm, region.source().url());
    // Overture sources are read after OpenStreetMap, so OpenStreetMap features are known when deduplicating.
    Map<String, String> overtureMeta = new LinkedHashMap<>();
    var overture = region.overture();
    if (overture != null) {
      Path sources = dataDir.resolve("sources");
      addParquet(planetiler, OverturePlaces.SOURCE, overture.places() == null ? null : overture.places().file(),
        sources, overtureMeta);
      addParquet(planetiler, OvertureBuildings.SOURCE,
        overture.buildings() == null ? null : overture.buildings().file(), sources, overtureMeta);
    }
    planetiler.overwriteOutput(tiles).run();

    if (skipDatabase) {
      return;
    }
    Map<String, String> meta = new LinkedHashMap<>();
    meta.put("built_at", Instant.now().truncatedTo(ChronoUnit.SECONDS).toString());
    meta.put("build_seconds", String.valueOf(Instant.now().getEpochSecond() - started.getEpochSecond()));
    meta.put("generator", "BombayChitra pipeline " + BombayChitraProfile.VERSION);
    meta.put("tiles_file", tiles.getFileName().toString());
    meta.put("source_file", osm.getFileName().toString());
    try {
      var header = new OsmInputFile(osm).getHeader();
      if (header.instant() != null) {
        meta.put("source_timestamp", header.instant().toString());
      }
    } catch (RuntimeException e) {
      LOGGER.warn("Could not read OSM header timestamp: {}", e.getMessage());
    }
    meta.putAll(overtureMeta);
    new DatabaseWriter(region, taxonomy, curated, sink, meta).write(database);
    sink.clear();
  }

  /** Adds an Overture GeoParquet extract when present; its release is read from the downloader's state file. */
  private static void addParquet(Planetiler planetiler, String name, String file, Path sources,
    Map<String, String> meta) {
    if (file == null) {
      return;
    }
    Path path = sources.resolve(file);
    if (!Files.exists(path)) {
      LOGGER.warn("Overture source {} not found at {} — run the build script to download it, or remove it from "
        + "region.json", name, path);
      return;
    }
    planetiler.addParquetSource(name, List.of(path));
    meta.put(name + "_file", path.getFileName().toString());
    Path state = path.resolveSibling(path.getFileName() + ".state");
    try {
      if (Files.exists(state)) {
        String release = new ObjectMapper().readTree(state.toFile()).path("last_release").asText(null);
        if (release != null) {
          meta.put("overture_release", release);
        }
      }
    } catch (IOException e) {
      LOGGER.warn("Could not read {}: {}", state, e.getMessage());
    }
  }
}
