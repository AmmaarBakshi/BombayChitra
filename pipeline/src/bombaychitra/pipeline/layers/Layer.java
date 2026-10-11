package bombaychitra.pipeline.layers;

import com.onthegomap.planetiler.ForwardingProfile;

/**
 * Base class for a vector-tile layer. Each subclass owns exactly one tile layer (its {@link #name()}), declares the
 * tags it cares about via {@link #filter()} so Planetiler only dispatches relevant features to it, and may also feed
 * rows into the SQLite database through {@link LayerContext#sink()}.
 */
public abstract class Layer implements ForwardingProfile.HandlerForLayer, ForwardingProfile.FeatureProcessor {

  protected final LayerContext ctx;

  protected Layer(LayerContext ctx) {
    this.ctx = ctx;
  }

  protected int maxzoom() {
    return ctx.maxzoom();
  }
}
