import type { StyleSpecification } from '@maplibre/maplibre-gl-style-spec';
import { buildLayers, SOURCE } from './layers.ts';
import { palettes, type ThemeName } from './palette.ts';

export type { ThemeName } from './palette.ts';
export { palettes } from './palette.ts';
export { SOURCE } from './layers.ts';

export interface StyleOptions {
  theme: ThemeName;
  /** Absolute origin of the BombayChitra server, or "{{BASE}}" for templated styles. */
  base: string;
  /** App mode: runtime POI badges + global-state filters (MapLibre GL JS ≥ 5.6). Off for portable styles. */
  app?: boolean;
}

/** Builds a MapLibre style for the BombayChitra tileset. */
export function buildStyle({ theme, base, app = false }: StyleOptions): StyleSpecification {
  const P = palettes[theme];
  const style: StyleSpecification = {
    version: 8,
    name: `BombayChitra ${theme === 'light' ? 'Light' : 'Dark'}`,
    metadata: { 'bombaychitra:theme': theme, 'bombaychitra:schema': 1 },
    center: [72.8777, 19.076],
    zoom: 11,
    glyphs: `${base}/fonts/{fontstack}/{range}.pbf`,
    sources: {
      [SOURCE]: { type: 'vector', url: `${base}/tiles/bombaychitra.json` },
    },
    light: { anchor: 'viewport', color: '#ffffff', intensity: theme === 'light' ? 0.35 : 0.25, position: [1.2, 200, 35] },
    layers: buildLayers(P, { app }),
  };
  if (app) {
    style.state = {
      hiddenCategories: { default: [] },
      area: { default: '' },
    };
  }
  return style;
}
