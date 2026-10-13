import { validateStyleMin } from '@maplibre/maplibre-gl-style-spec';
import { describe, expect, it } from 'vitest';
import { buildStyle, LANGUAGES } from '../src/map/style/index.ts';
import { labelTone } from '../src/map/style/layers.ts';
import { palettes, type ThemeName } from '../src/map/style/palette.ts';

/** Layers in the vector tileset (see docs/tiles.md). */
const TILE_LAYERS = ['water', 'waterway', 'landuse', 'buildings', 'roads', 'bridges', 'transit', 'pois', 'places', 'boundaries', 'areas', 'aeroway', 'addresses'];

const variants: [ThemeName, boolean][] = [['light', false], ['dark', false], ['light', true], ['dark', true]];

describe.each(variants)('%s style (app mode: %s)', (theme, app) => {
  const style = buildStyle({ theme, base: 'https://maps.example.com', app });

  it('is valid against the style specification', () => {
    expect(validateStyleMin(style as Parameters<typeof validateStyleMin>[0])).toEqual([]);
  });

  it('has unique layer ids', () => {
    const ids = style.layers.map((l) => l.id);
    expect(new Set(ids).size).toBe(ids.length);
  });

  it('only reads layers the tileset has', () => {
    for (const layer of style.layers) {
      if ('source-layer' in layer && layer['source-layer']) expect(TILE_LAYERS, layer.id).toContain(layer['source-layer']);
    }
  });

  it('points at the server it was built for', () => {
    expect(style.glyphs).toBe('https://maps.example.com/fonts/{fontstack}/{range}.pbf');
    expect(JSON.stringify(style.sources)).toContain('https://maps.example.com/tiles/bombaychitra.json');
  });

  it(app ? 'declares the UI state' : 'stays portable (no global state)', () => {
    if (app) expect(Object.keys(style.state ?? {})).toEqual(['hiddenCategories', 'area']);
    else expect(style.state).toBeUndefined();
  });
});

describe('palettes', () => {
  it('define the same keys for both themes', () => {
    expect(Object.keys(palettes.dark).sort()).toEqual(Object.keys(palettes.light).sort());
  });

  it('darken label colours on light maps and lighten them on dark maps', () => {
    const lum = (hex: string) => parseInt(hex.slice(1, 3), 16) + parseInt(hex.slice(3, 5), 16) + parseInt(hex.slice(5, 7), 16);
    expect(lum(labelTone('#0b84e6', 'light'))).toBeLessThan(lum('#0b84e6'));
    expect(lum(labelTone('#0b84e6', 'dark'))).toBeGreaterThan(lum('#0b84e6'));
  });
});

describe('label languages', () => {
  it.each(LANGUAGES)('%s styles are valid and label in that language', (lang) => {
    const style = buildStyle({ theme: 'light', base: 'https://maps.example.com', lang });
    expect(validateStyleMin(style as Parameters<typeof validateStyleMin>[0])).toEqual([]);
    const text = JSON.stringify(style.layers);
    if (lang === 'en') expect(text).not.toContain('name:');
    else expect(text).toContain(`["coalesce",["get","name:${lang}"],["get","name"],""]`);
    expect(style.metadata).toMatchObject({ 'bombaychitra:language': lang });
  });
});
