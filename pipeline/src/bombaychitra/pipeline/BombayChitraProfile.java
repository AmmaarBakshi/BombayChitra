package bombaychitra.pipeline;

import bombaychitra.pipeline.layers.Addresses;
import bombaychitra.pipeline.layers.Aeroways;
import bombaychitra.pipeline.layers.Areas;
import bombaychitra.pipeline.layers.Boundaries;
import bombaychitra.pipeline.layers.Bridges;
import bombaychitra.pipeline.layers.Buildings;
import bombaychitra.pipeline.layers.LayerContext;
import bombaychitra.pipeline.layers.Landuse;
import bombaychitra.pipeline.layers.OvertureBuildings;
import bombaychitra.pipeline.layers.OverturePlaces;
import bombaychitra.pipeline.layers.Places;
import bombaychitra.pipeline.layers.Pois;
import bombaychitra.pipeline.layers.Roads;
import bombaychitra.pipeline.layers.Transit;
import bombaychitra.pipeline.layers.Water;
import bombaychitra.pipeline.layers.Waterways;
import com.onthegomap.planetiler.ForwardingProfile;
import com.onthegomap.planetiler.config.PlanetilerConfig;
import java.util.Map;

/**
 * The BombayChitra vector tile schema. Every layer is an independent handler in {@code bombaychitra.pipeline.layers};
 * add a layer by writing a handler and registering it here. Use {@code --only_layers} / {@code --exclude_layers} to
 * build a subset.
 */
public final class BombayChitraProfile extends ForwardingProfile {

  public static final String VERSION = "1.0.0";

  private final RegionConfig region;

  public BombayChitraProfile(PlanetilerConfig config, LayerContext ctx) {
    super(config,
      new Water(ctx),
      new Waterways(ctx),
      new Landuse(ctx),
      new Buildings(ctx),
      new Roads(ctx),
      new Bridges(ctx),
      new Transit(ctx),
      new Pois(ctx),
      new Places(ctx),
      new Boundaries(ctx),
      new Areas(ctx),
      new Aeroways(ctx),
      new Addresses(ctx),
      new OverturePlaces(ctx),
      new OvertureBuildings(ctx)
    );
    this.region = ctx.region();
  }

  @Override
  public String name() {
    return "BombayChitra";
  }

  @Override
  public String description() {
    return "BombayChitra · " + region.name() + " — detailed open map of roads, bridges, buildings, transit and places";
  }

  @Override
  public String attribution() {
    return "<a href=\"https://github.com/AmmaarBakshi/BombayChitra\" target=\"_blank\">BombayChitra</a> "
      + "<a href=\"https://www.openstreetmap.org/copyright\" target=\"_blank\">&copy; OpenStreetMap contributors</a>"
      + (region.overture() == null ? "" : " <a href=\"https://docs.overturemaps.org/attribution/\" target=\"_blank\">"
        + "&copy; Overture Maps Foundation</a>");
  }

  @Override
  public String version() {
    return VERSION;
  }

  @Override
  public boolean isOverlay() {
    return false;
  }

  @Override
  public Map<String, String> extraArchiveMetadata() {
    return Map.of("region", region.id(), "schema", "bombaychitra/" + VERSION);
  }
}
