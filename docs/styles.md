# Map styles & theming

BombayChitra ships two MapLibre styles — **light** (warm, paper-like) and **dark** (low-glare, for night use) —
generated from a single TypeScript definition, so every layer exists in both and they never drift apart.

## Using the styles

| URL | Description |
|---|---|
| `/styles/light.json` | Light theme |
| `/styles/dark.json` | Dark theme |

The server writes its own public origin into the tile, TileJSON and glyph URLs, so the styles can be loaded by any
client — see [Integrations](integrations.md). They use only standard style-spec features and work in MapLibre GL JS,
MapLibre Native (Android/iOS), Flutter, React Native and QGIS.

### The web app's variant

The web app builds an **app** variant of the same style at runtime, which adds:

- POI badges drawn on the fly (category colour + [Maki](https://labs.mapbox.com/maki-icons/) glyph) instead of
  plain dots;
- `global-state` filters, so toggling categories or highlighting an area costs one state update rather than a
  restyle;
- selection, route and measurement overlays.

The portable styles render POIs as coloured dots with labels, so they need no sprite sheet.

## How the style is built

```text
web/src/map/style/
├── palette.ts   light and dark colour tokens (land, water, every road class, labels…)
├── layers.ts    all layers as functions of the palette
├── expr.ts      expression helpers (zoom interpolation, matches)
└── index.ts     buildStyle({ theme, base, app }) → StyleSpecification
```

At build time a Vite plugin validates every variant against the MapLibre style specification and emits
`styles/light.json` and `styles/dark.json`. An invalid expression fails the build.

### Layer order

Background → land use (natural, urban, amenities) → water and waterways → airports → buildings → tunnels → roads
(paths, casings, fills) → railways and metro → bridges (roads, then metro viaducts) → 3D buildings → boundaries →
area highlight → bridge explorer → labels (waterways, roads, house numbers) → POIs → place names.

### Cartographic choices

- Road widths are defined per class at key zooms and interpolated exponentially, so the network reads well from
  the whole region (z8) down to a single lane (z18).
- Bridges are drawn above the roads and railways they cross, with a darker casing; tunnels are dashed and faded.
- Suburban railway lines are drawn in their line colours, slightly offset from the track. Metro lines use their
  official colours, underground sections are dashed, lines under construction are dashed and translucent, and
  proposed lines are dotted.
- Labels use Noto Sans (bundled glyphs, SIL OFL) and are prioritised by importance (`symbol-sort-key`).
- City names fade out once you are inside the city; suburbs are uppercase and letter-spaced.

## Customising

To change colours, edit `palette.ts` (both themes) and rebuild the web app. To add a theme:

1. Add the name to the `ThemeName` union and a `Palette` (for example `sepia`) to `palettes` in `palette.ts`.
2. Add it to `THEMES` in `web/build/styles.ts` so `styles/sepia.json` is emitted.
3. Optionally expose it in the app's theme switcher (`web/src/ui/theme.ts`).

To restyle a single layer, edit its definition in `layers.ts`. Layer ids are stable and documented above, so you can
also post-process a downloaded style JSON in your own app.
