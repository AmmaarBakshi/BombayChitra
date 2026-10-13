import type { ExpressionSpecification, FilterSpecification, LayerSpecification } from '@maplibre/maplibre-gl-style-spec';
import { categories, groups } from '../../taxonomy.ts';
import { fonts, isBridge, isTunnel, labelField, type Language, matchKey, roadOrder, roadSortKey, zoomExp, zoomLinear } from './expr.ts';
import type { Palette } from './palette.ts';

export const SOURCE = 'bc';

export interface LayerOptions {
  /** App mode uses runtime-generated POI badges and global-state filters; standalone styles stay SDK-portable. */
  app: boolean;
  /** Label language (default: the names as mapped, mostly English). */
  lang?: Language;
}

type Layer = LayerSpecification;

// ------------------------------------------------------------------ road widths

/** Fill width (px) per road class at key zooms; MapLibre interpolates exponentially between them. */
const ROAD_WIDTH: Record<string, [number, number][]> = {
  motorway: [[5, 0.6], [9, 1.4], [12, 3], [14, 6], [16, 13], [18, 30]],
  trunk: [[6, 0.5], [9, 1.2], [12, 2.6], [14, 5.5], [16, 12], [18, 28]],
  primary: [[8, 0.5], [10, 1], [12, 2.2], [14, 5], [16, 11], [18, 26]],
  secondary: [[9, 0.4], [11, 0.9], [12, 1.8], [14, 4.5], [16, 10], [18, 22]],
  tertiary: [[10, 0.4], [12, 1.3], [14, 3.8], [16, 8.5], [18, 19]],
  minor: [[12, 0.4], [13, 0.9], [14, 2.4], [16, 6.5], [18, 15]],
  service: [[13, 0.3], [14, 0.8], [16, 3.5], [18, 8]],
  pedestrian: [[13, 0.3], [14, 1], [16, 4], [18, 9]],
  track: [[13, 0.4], [16, 1.5], [18, 3]],
  path: [[14, 0.5], [16, 1.2], [18, 2.2]],
  construction: [[12, 0.4], [14, 2], [16, 5], [18, 11]],
};
const WIDTH_ZOOMS = [5, 7, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18];

function widthAt(kind: string, z: number): number {
  const stops = ROAD_WIDTH[kind];
  if (!stops || z < stops[0][0]) return 0;
  for (let i = 0; i < stops.length - 1; i++) {
    const [z0, w0] = stops[i];
    const [z1, w1] = stops[i + 1];
    if (z >= z0 && z <= z1) return w0 + ((w1 - w0) * (z - z0)) / (z1 - z0);
  }
  return stops[stops.length - 1][1];
}

const linkFactor: ExpressionSpecification = ['case', ['==', ['get', 'link'], 1], 0.6, 1];

/** Road width expression; casing adds a zoom-dependent margin. */
function roadWidth(casing = false): ExpressionSpecification {
  const stops: [number, ExpressionSpecification][] = WIDTH_ZOOMS.map((z) => {
    const margin = casing ? (z < 13 ? 0.8 : z < 15 ? 1.4 : 2.2) : 0;
    const cases: Record<string, number> = {};
    for (const kind of Object.keys(ROAD_WIDTH)) {
      const w = widthAt(kind, z);
      cases[kind] = w > 0 ? +(w + (w > 0 ? margin : 0)).toFixed(2) : 0;
    }
    return [z, ['*', matchKey('kind', cases, 0), linkFactor] as ExpressionSpecification];
  });
  return zoomExp(1.5, ...stops);
}

const notPathLike: ExpressionSpecification = ['!', ['in', ['get', 'kind'], ['literal', ['path', 'construction']]]];

// ------------------------------------------------------------------ helpers

const kindIn = (...kinds: string[]): ExpressionSpecification => ['in', ['get', 'kind'], ['literal', kinds]];

function landuseFill(id: string, kinds: string[], P: Palette, opacity: number | ExpressionSpecification = 1): Layer {
  const colours: Record<string, string> = {};
  for (const k of kinds) colours[k] = P.landuse[k];
  return {
    id,
    type: 'fill',
    source: SOURCE,
    'source-layer': 'landuse',
    filter: kindIn(...kinds) as FilterSpecification,
    paint: { 'fill-color': matchKey('kind', colours, P.land), 'fill-opacity': opacity },
  };
}

function groupColourExpr(): ExpressionSpecification {
  return matchKey('group', Object.fromEntries(groups.map((g) => [g.id, g.colour])), '#777777');
}

/** Darken (light theme) or lighten (dark theme) a hex colour for legible label text. */
export function labelTone(hex: string, theme: 'light' | 'dark'): string {
  const n = parseInt(hex.slice(1), 16);
  const r = (n >> 16) & 255;
  const g = (n >> 8) & 255;
  const b = n & 255;
  const t = theme === 'light' ? 0.62 : 1.0;
  const mix = (c: number) => (theme === 'light' ? Math.round(c * t) : Math.round(c + (255 - c) * 0.35));
  return '#' + [mix(r), mix(g), mix(b)].map((c) => c.toString(16).padStart(2, '0')).join('');
}

// ------------------------------------------------------------------ layers

export function buildLayers(P: Palette, opts: LayerOptions): Layer[] {
  const theme = P.name;
  const nameField = labelField(opts.lang);
  const layers: Layer[] = [];
  const add = (...l: Layer[]) => layers.push(...l);

  add({ id: 'background', type: 'background', paint: { 'background-color': P.land } });

  // Land cover & land use, natural first so urban and leisure areas draw on top.
  add(
    landuseFill('landuse-natural', ['forest', 'mangrove', 'wetland', 'salt_pan', 'grass', 'farmland', 'beach', 'rock', 'quarry', 'aquaculture'], P),
    landuseFill('landuse-urban', ['residential', 'commercial', 'industrial', 'port', 'railway', 'military', 'construction', 'landfill', 'airport', 'parking', 'religious'], P, zoomLinear([9, 0.6], [12, 1])),
    landuseFill('landuse-leisure', ['park', 'golf', 'zoo', 'sports', 'playground', 'cemetery', 'education', 'hospital'], P),
    {
      id: 'landuse-protected',
      type: 'fill',
      source: SOURCE,
      'source-layer': 'landuse',
      filter: kindIn('protected') as FilterSpecification,
      paint: { 'fill-color': P.landuse.protected, 'fill-opacity': 0.45 },
    },
    {
      id: 'landuse-protected-outline',
      type: 'line',
      source: SOURCE,
      'source-layer': 'landuse',
      filter: kindIn('protected') as FilterSpecification,
      minzoom: 9,
      paint: { 'line-color': P.protectedOutline, 'line-width': zoomLinear([9, 0.6], [14, 1.6]), 'line-dasharray': [3, 2], 'line-opacity': 0.7 },
    },
  );

  // Water
  add(
    {
      id: 'water',
      type: 'fill',
      source: SOURCE,
      'source-layer': 'water',
      filter: ['==', ['geometry-type'], 'Polygon'],
      paint: { 'fill-color': matchKey('kind', { ocean: P.ocean }, P.water) },
    },
    {
      id: 'waterway-minor',
      type: 'line',
      source: SOURCE,
      'source-layer': 'waterway',
      filter: kindIn('stream', 'drain', 'ditch') as FilterSpecification,
      minzoom: 12,
      layout: { 'line-cap': 'round', 'line-join': 'round' },
      paint: {
        'line-color': P.waterway,
        'line-width': zoomExp(1.4, [12, 0.5], [16, 1.6], [18, 3]),
        'line-opacity': ['case', ['==', ['get', 'tunnel'], 1], 0.45, 1],
      },
    },
    {
      id: 'waterway-major',
      type: 'line',
      source: SOURCE,
      'source-layer': 'waterway',
      filter: kindIn('river', 'canal', 'tidal_channel') as FilterSpecification,
      layout: { 'line-cap': 'round', 'line-join': 'round' },
      paint: { 'line-color': P.waterway, 'line-width': zoomExp(1.4, [8, 0.6], [12, 1.6], [16, 5], [18, 10]) },
    },
  );

  // Airports
  add(
    {
      id: 'aeroway-area',
      type: 'fill',
      source: SOURCE,
      'source-layer': 'aeroway',
      filter: ['==', ['geometry-type'], 'Polygon'],
      paint: { 'fill-color': P.apron },
    },
    {
      id: 'aeroway-taxiway',
      type: 'line',
      source: SOURCE,
      'source-layer': 'aeroway',
      filter: ['all', ['==', ['geometry-type'], 'LineString'], ['==', ['get', 'kind'], 'taxiway']],
      minzoom: 12,
      paint: { 'line-color': P.aeroway, 'line-width': zoomExp(1.5, [12, 1], [17, 12]) },
    },
    {
      id: 'aeroway-runway',
      type: 'line',
      source: SOURCE,
      'source-layer': 'aeroway',
      filter: ['all', ['==', ['geometry-type'], 'LineString'], ['==', ['get', 'kind'], 'runway']],
      paint: { 'line-color': P.aeroway, 'line-width': zoomExp(1.5, [9, 1], [12, 5], [17, 60]) },
    },
    {
      id: 'transit-platform',
      type: 'fill',
      source: SOURCE,
      'source-layer': 'transit',
      filter: ['all', ['==', ['geometry-type'], 'Polygon'], ['==', ['get', 'kind'], 'platform']],
      minzoom: 14,
      paint: { 'fill-color': P.platform },
    },
  );

  // Buildings (flat). The 3D layer further up is toggled by the app.
  add({
    id: 'buildings-2d',
    type: 'fill',
    source: SOURCE,
    'source-layer': 'buildings',
    minzoom: 13,
    paint: {
      'fill-color': P.building,
      'fill-outline-color': P.buildingOutline,
      'fill-opacity': zoomLinear([13, 0.6], [15, 1]),
    },
  });

  // Tunnels
  add(
    {
      id: 'road-tunnel-casing',
      type: 'line',
      source: SOURCE,
      'source-layer': 'roads',
      filter: ['all', isTunnel, notPathLike],
      layout: { 'line-cap': 'butt', 'line-join': 'round', 'line-sort-key': roadSortKey },
      paint: { 'line-color': matchKey('kind', P.roadCasing, P.roadCasing.minor), 'line-width': roadWidth(true), 'line-dasharray': [0.6, 0.4] },
    },
    {
      id: 'road-tunnel',
      type: 'line',
      source: SOURCE,
      'source-layer': 'roads',
      filter: ['all', isTunnel, notPathLike],
      layout: { 'line-cap': 'round', 'line-join': 'round', 'line-sort-key': roadSortKey },
      paint: { 'line-color': P.tunnelFill, 'line-width': roadWidth() },
    },
  );

  // Roads at grade
  const atGrade: ExpressionSpecification = ['all', ['!', isTunnel], ['!', isBridge]];
  add(
    {
      id: 'road-path',
      type: 'line',
      source: SOURCE,
      'source-layer': 'roads',
      filter: ['all', ['==', ['get', 'kind'], 'path'], ['!', isTunnel]],
      minzoom: 14,
      layout: { 'line-join': 'round' },
      paint: { 'line-color': P.road.path, 'line-width': roadWidth(), 'line-dasharray': [2, 1.2] },
    },
    {
      id: 'road-casing',
      type: 'line',
      source: SOURCE,
      'source-layer': 'roads',
      filter: ['all', atGrade, notPathLike],
      minzoom: 11,
      layout: { 'line-cap': 'round', 'line-join': 'round', 'line-sort-key': roadSortKey },
      paint: { 'line-color': matchKey('kind', P.roadCasing, P.roadCasing.minor), 'line-width': roadWidth(true) },
    },
    {
      id: 'road',
      type: 'line',
      source: SOURCE,
      'source-layer': 'roads',
      filter: ['all', atGrade, notPathLike],
      layout: { 'line-cap': 'round', 'line-join': 'round', 'line-sort-key': roadSortKey },
      paint: { 'line-color': matchKey('kind', P.road, P.road.minor), 'line-width': roadWidth() },
    },
    {
      id: 'road-construction',
      type: 'line',
      source: SOURCE,
      'source-layer': 'roads',
      filter: ['==', ['get', 'kind'], 'construction'],
      minzoom: 12,
      paint: { 'line-color': P.road.construction, 'line-width': roadWidth(), 'line-dasharray': [2, 1.5] },
    },
  );

  // Railways and transit
  const railKinds = kindIn('rail');
  const metroKinds = kindIn('subway', 'light_rail', 'monorail', 'tram');
  const metroColour: ExpressionSpecification = ['coalesce', ['get', 'colour'], P.selection];
  add(
    {
      id: 'rail',
      type: 'line',
      source: SOURCE,
      'source-layer': 'transit',
      filter: ['all', railKinds, ['!', ['has', 'service']]],
      layout: { 'line-join': 'round' },
      paint: { 'line-color': P.rail, 'line-width': zoomExp(1.5, [6, 0.5], [10, 1.2], [14, 2.6], [18, 5]) },
    },
    {
      id: 'rail-dash',
      type: 'line',
      source: SOURCE,
      'source-layer': 'transit',
      filter: ['all', railKinds, ['!', ['has', 'service']]],
      minzoom: 13,
      paint: { 'line-color': P.railDash, 'line-width': zoomExp(1.5, [13, 0.8], [18, 3]), 'line-dasharray': [3, 3] },
    },
    {
      id: 'rail-service',
      type: 'line',
      source: SOURCE,
      'source-layer': 'transit',
      filter: ['all', railKinds, ['has', 'service']],
      minzoom: 13,
      paint: { 'line-color': P.railService, 'line-width': zoomExp(1.5, [13, 0.5], [18, 2]) },
    },
    {
      id: 'rail-lines',
      type: 'line',
      source: SOURCE,
      'source-layer': 'transit',
      filter: ['all', railKinds, ['has', 'colour'], ['!', ['has', 'service']]],
      minzoom: 9,
      layout: { 'line-join': 'round' },
      paint: {
        'line-color': ['get', 'colour'],
        'line-width': zoomExp(1.5, [9, 1], [12, 1.6], [15, 2.6], [18, 4]),
        'line-opacity': zoomLinear([9, 0.9], [14, 0.75], [16, 0.55]),
        'line-offset': zoomExp(1.5, [12, 0], [15, 2.5], [18, 5]),
      },
    },
    {
      id: 'metro-construction',
      type: 'line',
      source: SOURCE,
      'source-layer': 'transit',
      filter: ['==', ['get', 'kind'], 'construction'],
      minzoom: 9,
      paint: {
        'line-color': ['coalesce', ['get', 'colour'], P.textFaint],
        'line-width': zoomExp(1.5, [9, 1], [14, 2.5], [18, 5]),
        'line-dasharray': [2, 2],
        'line-opacity': 0.75,
      },
    },
    {
      id: 'metro-proposed',
      type: 'line',
      source: SOURCE,
      'source-layer': 'transit',
      filter: ['==', ['get', 'kind'], 'proposed'],
      minzoom: 10,
      layout: { 'line-cap': 'round' },
      paint: {
        'line-color': ['coalesce', ['get', 'colour'], P.textFaint],
        'line-width': zoomExp(1.5, [10, 2], [14, 3.5], [18, 6]),
        'line-dasharray': [0.1, 1.8],
        'line-opacity': 0.8,
      },
    },
    {
      id: 'metro-tunnel',
      type: 'line',
      source: SOURCE,
      'source-layer': 'transit',
      filter: ['all', metroKinds, isTunnel],
      paint: {
        'line-color': metroColour,
        'line-width': zoomExp(1.5, [8, 1.2], [11, 2], [14, 3.2], [18, 6]),
        'line-dasharray': [1.6, 1],
        'line-opacity': 0.7,
      },
    },
    {
      id: 'metro-casing',
      type: 'line',
      source: SOURCE,
      'source-layer': 'transit',
      filter: ['all', metroKinds, ['!', isTunnel]],
      minzoom: 11,
      layout: { 'line-join': 'round', 'line-cap': 'round' },
      paint: { 'line-color': P.transitCasing, 'line-width': zoomExp(1.5, [11, 3], [14, 5.5], [18, 10]) },
    },
    {
      id: 'metro',
      type: 'line',
      source: SOURCE,
      'source-layer': 'transit',
      filter: ['all', metroKinds, ['!', isTunnel]],
      layout: { 'line-join': 'round', 'line-cap': 'round' },
      paint: { 'line-color': metroColour, 'line-width': zoomExp(1.5, [8, 1.2], [11, 2], [14, 3.5], [18, 7]) },
    },
  );

  // Bridges above everything at grade
  add(
    {
      id: 'road-bridge-casing',
      type: 'line',
      source: SOURCE,
      'source-layer': 'roads',
      filter: ['all', isBridge, notPathLike],
      layout: { 'line-cap': 'butt', 'line-join': 'round', 'line-sort-key': roadSortKey },
      paint: { 'line-color': P.bridgeCasing, 'line-width': roadWidth(true) },
    },
    {
      id: 'road-bridge',
      type: 'line',
      source: SOURCE,
      'source-layer': 'roads',
      filter: ['all', isBridge, notPathLike],
      layout: { 'line-cap': 'round', 'line-join': 'round', 'line-sort-key': roadSortKey },
      paint: { 'line-color': matchKey('kind', P.road, P.road.minor), 'line-width': roadWidth() },
    },
    {
      id: 'path-bridge',
      type: 'line',
      source: SOURCE,
      'source-layer': 'roads',
      filter: ['all', isBridge, ['==', ['get', 'kind'], 'path']],
      minzoom: 14,
      paint: { 'line-color': P.road.path, 'line-width': zoomExp(1.5, [14, 1], [18, 3]) },
    },
    {
      id: 'metro-bridge',
      type: 'line',
      source: SOURCE,
      'source-layer': 'transit',
      filter: ['all', metroKinds, isBridge],
      minzoom: 12,
      layout: { 'line-join': 'round', 'line-cap': 'round' },
      paint: { 'line-color': metroColour, 'line-width': zoomExp(1.5, [12, 2.4], [14, 3.5], [18, 7]) },
    },
  );

  // Oneway arrows
  add({
    id: 'road-oneway',
    type: 'symbol',
    source: SOURCE,
    'source-layer': 'roads',
    filter: ['all', ['has', 'oneway'], notPathLike],
    minzoom: 16,
    layout: {
      'symbol-placement': 'line',
      'symbol-spacing': 90,
      'text-field': ['case', ['==', ['get', 'oneway'], -1], '←', '→'],
      'text-font': fonts.regular,
      'text-size': 11,
      'text-keep-upright': false,
      'text-allow-overlap': true,
    },
    paint: { 'text-color': P.textFaint, 'text-opacity': 0.8 },
  });

  // 3D buildings (hidden until enabled)
  add({
    id: 'buildings-3d',
    type: 'fill-extrusion',
    source: SOURCE,
    'source-layer': 'buildings',
    minzoom: 14,
    layout: { visibility: 'none' },
    paint: {
      'fill-extrusion-color': [
        'interpolate', ['linear'], ['coalesce', ['get', 'height'], 6],
        0, P.buildingLow, 40, P.buildingMid, 150, P.buildingHigh,
      ],
      'fill-extrusion-height': ['coalesce', ['get', 'height'], ['*', ['coalesce', ['get', 'levels'], 2], 3.2]],
      'fill-extrusion-base': ['coalesce', ['get', 'min_height'], 0],
      'fill-extrusion-opacity': 0.92,
      'fill-extrusion-vertical-gradient': true,
    },
  });

  // Administrative boundaries: state/district/taluka, municipal corporations, wards.
  const onLand: ExpressionSpecification = ['!=', ['get', 'maritime'], 1];
  const level = (min: number, max: number): FilterSpecification =>
    ['all', onLand, ['>=', ['get', 'admin_level'], min], ['<=', ['get', 'admin_level'], max]];
  add(
    {
      id: 'boundary-ward',
      type: 'line',
      source: SOURCE,
      'source-layer': 'boundaries',
      filter: level(9, 10),
      minzoom: 12,
      paint: { 'line-color': P.boundary, 'line-width': 0.7, 'line-dasharray': [1, 2], 'line-opacity': 0.6 },
    },
    {
      id: 'boundary-municipal',
      type: 'line',
      source: SOURCE,
      'source-layer': 'boundaries',
      filter: level(7, 8),
      minzoom: 9,
      paint: { 'line-color': P.boundary, 'line-width': zoomLinear([9, 0.6], [14, 1.1]), 'line-dasharray': [2, 2], 'line-opacity': 0.8 },
    },
    {
      id: 'boundary-district',
      type: 'line',
      source: SOURCE,
      'source-layer': 'boundaries',
      filter: level(2, 6),
      layout: { 'line-join': 'round' },
      paint: {
        'line-color': P.boundaryMajor,
        'line-width': ['step', ['get', 'admin_level'], 1.8, 5, 1.3, 6, 0.9],
        'line-dasharray': [4, 2],
        'line-opacity': 0.85,
      },
    },
  );

  // Highlighted area (app only)
  if (opts.app) {
    const selectedArea: FilterSpecification = ['==', ['get', 'id'], ['global-state', 'area']];
    add(
      {
        id: 'area-fill',
        type: 'fill',
        source: SOURCE,
        'source-layer': 'areas',
        filter: selectedArea,
        paint: { 'fill-color': P.areaHighlight, 'fill-opacity': 0.05 },
      },
      {
        id: 'area-line',
        type: 'line',
        source: SOURCE,
        'source-layer': 'areas',
        filter: selectedArea,
        layout: { 'line-join': 'round' },
        paint: { 'line-color': P.areaHighlight, 'line-width': zoomLinear([8, 1.5], [14, 3]), 'line-opacity': 0.85 },
      },
    );
  }

  // Bridge explorer overlay (hidden until enabled)
  const bridgeColour = matchKey('kind', P.bridgeHighlight, P.bridgeHighlight.road);
  add(
    {
      id: 'bridges-glow',
      type: 'line',
      source: SOURCE,
      'source-layer': 'bridges',
      filter: ['==', ['geometry-type'], 'LineString'],
      layout: { visibility: 'none', 'line-cap': 'round', 'line-join': 'round' },
      paint: { 'line-color': bridgeColour, 'line-width': zoomExp(1.5, [8, 4], [13, 9], [17, 22]), 'line-opacity': 0.25, 'line-blur': 3 },
    },
    {
      id: 'bridges-line',
      type: 'line',
      source: SOURCE,
      'source-layer': 'bridges',
      filter: ['==', ['geometry-type'], 'LineString'],
      layout: { visibility: 'none', 'line-cap': 'round', 'line-join': 'round' },
      paint: { 'line-color': bridgeColour, 'line-width': zoomExp(1.5, [8, 1.5], [13, 3], [17, 7]) },
    },
    {
      id: 'bridges-outline',
      type: 'line',
      source: SOURCE,
      'source-layer': 'bridges',
      filter: ['==', ['geometry-type'], 'Polygon'],
      minzoom: 13,
      layout: { visibility: 'none' },
      paint: { 'line-color': P.bridgeHighlight.road, 'line-width': 1.2, 'line-dasharray': [2, 1] },
    },
  );

  // ---------------------------------------------------------------- labels

  add(
    {
      id: 'waterway-label',
      type: 'symbol',
      source: SOURCE,
      'source-layer': 'waterway',
      filter: ['has', 'name'],
      minzoom: 12,
      layout: {
        'symbol-placement': 'line',
        'text-field': nameField,
        'text-font': fonts.italic,
        'text-size': zoomLinear([12, 10.5], [17, 13]),
        'text-letter-spacing': 0.05,
        'symbol-spacing': 400,
      },
      paint: { 'text-color': P.waterLabel, 'text-halo-color': P.halo, 'text-halo-width': 1.2 },
    },
    {
      id: 'road-label',
      type: 'symbol',
      source: SOURCE,
      'source-layer': 'roads',
      filter: ['any', ['has', 'name'], ['has', 'ref']],
      minzoom: 11,
      layout: {
        'symbol-placement': 'line',
        'symbol-sort-key': ['-', 10, roadSortKey],
        'text-field': ['case', ['has', 'name'], nameField, ['get', 'ref']],
        'text-font': fonts.regular,
        'text-size': zoomLinear(
          [11, matchKey('kind', { motorway: 10, trunk: 10, primary: 10 }, 9)],
          [15, matchKey('kind', { motorway: 13, trunk: 13, primary: 12.5, secondary: 12, tertiary: 11.5 }, 10.5)],
          [18, matchKey('kind', { motorway: 15, trunk: 15, primary: 14.5, secondary: 14, tertiary: 13.5 }, 12.5)],
        ),
        'text-max-angle': 30,
        'symbol-spacing': 320,
        'text-padding': 4,
      },
      paint: { 'text-color': P.roadLabel, 'text-halo-color': P.halo, 'text-halo-width': 1.4 },
    },
    {
      id: 'bridges-label',
      type: 'symbol',
      source: SOURCE,
      'source-layer': 'bridges',
      filter: ['all', ['==', ['geometry-type'], 'LineString'], ['has', 'name'], ['>=', ['get', 'length'], 150]],
      minzoom: 11,
      layout: {
        visibility: 'none',
        'symbol-placement': 'line-center',
        'text-field': [
          'concat', ['get', 'name'], ' · ',
          ['case', ['>=', ['get', 'length'], 1000],
            ['concat', ['to-string', ['/', ['round', ['/', ['get', 'length'], 100]], 10]], ' km'],
            ['concat', ['to-string', ['get', 'length']], ' m']],
        ],
        'text-font': fonts.medium,
        'text-size': 12,
        'text-max-angle': 35,
      },
      paint: { 'text-color': bridgeColour, 'text-halo-color': P.halo, 'text-halo-width': 1.6 },
    },
    {
      id: 'housenumbers',
      type: 'symbol',
      source: SOURCE,
      'source-layer': 'addresses',
      minzoom: 17,
      layout: { 'text-field': ['get', 'housenumber'], 'text-font': fonts.regular, 'text-size': 10, 'text-padding': 2 },
      paint: { 'text-color': P.textFaint, 'text-halo-color': P.halo, 'text-halo-width': 1 },
    },
  );

  // Points of interest
  const poiLabelColour = matchKey('group', Object.fromEntries(groups.map((g) => [g.id, labelTone(g.colour, theme)])), P.textMuted);
  const poiText: ExpressionSpecification = [
    'step', ['zoom'],
    ['case', ['<=', ['get', 'rank'], 1], nameField, ''],
    14, ['case', ['<=', ['get', 'rank'], 2], nameField, ''],
    15, ['case', ['<=', ['get', 'rank'], 3], nameField, ''],
    16, nameField,
  ];
  const poiFilter: FilterSpecification = opts.app
    ? ['!', ['in', ['get', 'category'], ['global-state', 'hiddenCategories']]]
    : ['has', 'category'];
  const poiSortKey: ExpressionSpecification = ['+', ['*', ['get', 'rank'], 10], ['case', ['has', 'name'], 0, 5]];

  if (opts.app) {
    add({
      id: 'poi',
      type: 'symbol',
      source: SOURCE,
      'source-layer': 'pois',
      filter: poiFilter,
      layout: {
        'symbol-sort-key': poiSortKey,
        'icon-image': ['concat', 'poi-', ['get', 'category']],
        'icon-size': zoomLinear([10, 0.62], [14, 0.8], [17, 1]),
        'icon-padding': 1,
        'text-field': poiText,
        'text-font': fonts.medium,
        'text-size': zoomLinear([12, 10.5], [17, 12.5]),
        'text-anchor': 'top',
        'text-offset': [0, 0.95],
        'text-max-width': 9,
        'text-optional': true,
        'text-padding': 2,
      },
      paint: { 'text-color': poiLabelColour, 'text-halo-color': P.halo, 'text-halo-width': 1.4 },
    });
  } else {
    add(
      {
        id: 'poi-dot',
        type: 'circle',
        source: SOURCE,
        'source-layer': 'pois',
        filter: poiFilter,
        layout: { 'circle-sort-key': poiSortKey },
        paint: {
          'circle-color': groupColourExpr(),
          'circle-radius': zoomLinear([10, 2.5], [14, 4], [17, 6]),
          'circle-stroke-color': P.poiStroke,
          'circle-stroke-width': 1.2,
        },
      },
      {
        id: 'poi-label',
        type: 'symbol',
        source: SOURCE,
        'source-layer': 'pois',
        filter: poiFilter,
        layout: {
          'symbol-sort-key': poiSortKey,
          'text-field': poiText,
          'text-font': fonts.medium,
          'text-size': zoomLinear([12, 10.5], [17, 12.5]),
          'text-anchor': 'top',
          'text-offset': [0, 0.6],
          'text-max-width': 9,
          'text-padding': 2,
        },
        paint: { 'text-color': poiLabelColour, 'text-halo-color': P.halo, 'text-halo-width': 1.4 },
      },
    );
  }

  // Place names
  add(
    {
      id: 'water-label',
      type: 'symbol',
      source: SOURCE,
      'source-layer': 'places',
      filter: ['==', ['get', 'kind'], 'water'],
      layout: {
        'text-field': nameField,
        'text-font': fonts.italic,
        'text-size': zoomLinear([10, 11], [16, 14]),
        'text-max-width': 7,
      },
      paint: { 'text-color': P.waterLabel, 'text-halo-color': P.halo, 'text-halo-width': 1.2 },
    },
    {
      id: 'place-minor',
      type: 'symbol',
      source: SOURCE,
      'source-layer': 'places',
      filter: kindIn('neighbourhood', 'hamlet', 'locality', 'islet') as FilterSpecification,
      layout: {
        'symbol-sort-key': ['get', 'rank'],
        'text-field': nameField,
        'text-font': ['match', ['get', 'kind'], 'islet', ['literal', fonts.italic], ['literal', fonts.regular]],
        'text-size': zoomLinear([12, 10.5], [16, 13]),
        'text-max-width': 8,
        'text-padding': 3,
      },
      paint: { 'text-color': P.textFaint, 'text-halo-color': P.halo, 'text-halo-width': 1.3 },
    },
    {
      id: 'place-major',
      type: 'symbol',
      source: SOURCE,
      'source-layer': 'places',
      filter: kindIn('suburb', 'quarter', 'village', 'island', 'town', 'city') as FilterSpecification,
      layout: {
        'symbol-sort-key': ['get', 'rank'],
        'text-field': nameField,
        'text-font': ['match', ['get', 'kind'], 'island', ['literal', fonts.italic], ['literal', fonts.medium]],
        'text-transform': ['match', ['get', 'kind'], ['suburb', 'quarter'], 'uppercase', 'none'],
        'text-letter-spacing': ['match', ['get', 'kind'], ['suburb', 'quarter'], 0.08, 'city', 0.04, 0.01],
        'text-size': [
          'interpolate', ['linear'], ['zoom'],
          6, matchKey('kind', { city: 13, town: 10 }, 9),
          10, matchKey('kind', { city: 18, town: 13, suburb: 11, island: 12 }, 10),
          14, matchKey('kind', { city: 22, town: 16, suburb: 14, quarter: 12.5, island: 15, village: 13 }, 12),
        ],
        'text-max-width': 8,
        'text-padding': 4,
      },
      paint: {
        'text-color': matchKey('kind', { suburb: P.textMuted, quarter: P.textMuted, island: P.textMuted }, P.text),
        'text-halo-color': P.halo,
        'text-halo-width': 1.6,
        'text-opacity': ['interpolate', ['linear'], ['zoom'], 11, 1, 12.5, matchKey('kind', { city: 0 }, 1)],
      },
    },
  );

  return layers;
}

/** Category → icon lookup used by the app's runtime badge generator. */
export const categoryIcons = Object.fromEntries(categories.map((c) => [c.id, c.icon]));
export { roadOrder };
