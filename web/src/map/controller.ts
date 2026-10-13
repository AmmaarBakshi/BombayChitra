import type { LayerSpecification, StyleSpecification } from '@maplibre/maplibre-gl-style-spec';
import * as maplibregl from 'maplibre-gl';
import type { LngLatBoundsLike, MapGeoJSONFeature, MapMouseEvent } from 'maplibre-gl';
import workerUrl from 'maplibre-gl/dist/maplibre-gl-worker.mjs?url';
import { renderBadge } from './badges.ts';
import { buildStyle, palettes, type ThemeName } from './style/index.ts';
import type { Palette } from './style/palette.ts';

maplibregl.setWorkerUrl(workerUrl);

export interface MapPick {
  layer: string;
  feature: MapGeoJSONFeature;
  lngLat: [number, number];
}

export interface Overlays {
  buildings3d: boolean;
  bridges: boolean;
  railColours: boolean;
  boundaries: boolean;
  heritage: boolean;
  hotspots: boolean;
}

/** Layers that respond to clicks, in priority order (first match wins). */
const INTERACTIVE = [
  'heritage-sites', 'hotspot-icon', 'poi', 'place-major', 'place-minor', 'water-label', 'bridges-line', 'metro-bridge', 'metro', 'metro-tunnel', 'metro-construction',
  'metro-proposed', 'rail-lines', 'rail', 'road-bridge', 'road', 'road-tunnel', 'path-bridge', 'road-path', 'road-construction',
  'buildings-3d', 'buildings-2d', 'aeroway-runway', 'landuse-leisure', 'landuse-urban', 'landuse-natural',
  'landuse-protected', 'water',
];
const HOVERABLE = INTERACTIVE.slice(0, INTERACTIVE.indexOf('buildings-2d') + 1);

/** Which overlay controls which style layers ("!" = shown when the overlay is off). */
const OVERLAY_LAYERS: Record<keyof Overlays, string[]> = {
  buildings3d: ['buildings-3d', '!buildings-2d'],
  bridges: ['bridges-glow', 'bridges-line', 'bridges-outline', 'bridges-label'],
  railColours: ['rail-lines'],
  boundaries: ['boundary-ward', 'boundary-municipal', 'boundary-district'],
  heritage: ['heritage-sites', 'heritage-labels'],
  hotspots: ['hotspot-area', 'hotspot-icon', 'hotspot-labels'],
};

/** Colours of curated heritage sites by status. */
export const HERITAGE_COLOURS: Record<string, string> = { unesco: '#c9a227', precinct: '#9c36b5', monument: '#a0522d' };
export const HOTSPOT_COLOUR = '#1c7ed6';

const SEL = 'selection';
const MEASURE = 'measure';
const HERITAGE = 'heritage';
const HOTSPOTS = 'hotspots';
/** Metres per pixel at zoom 0 at Mumbai's latitude (19° N), for drawing hotspots at their true radius. */
const MPP_Z0 = 148_000;
const EMPTY: GeoJSON.FeatureCollection = { type: 'FeatureCollection', features: [] };

/** App-only layers drawn above the basemap: curated Mumbai layers, the current selection and the measuring tool. */
function overlayLayers(P: Palette): LayerSpecification[] {
  const radius = ['interpolate', ['exponential', 2], ['zoom'], 10, ['/', ['*', ['get', 'radius_m'], 1024], MPP_Z0], 20, ['/', ['*', ['get', 'radius_m'], 1048576], MPP_Z0]] as unknown as number;
  return [
    { id: 'hotspot-area', type: 'circle', source: HOTSPOTS, minzoom: 10, paint: { 'circle-radius': radius, 'circle-color': HOTSPOT_COLOUR, 'circle-opacity': 0.16, 'circle-stroke-color': HOTSPOT_COLOUR, 'circle-stroke-width': 1.2, 'circle-stroke-opacity': 0.6 } },
    { id: 'hotspot-icon', type: 'circle', source: HOTSPOTS, paint: { 'circle-radius': ['interpolate', ['linear'], ['zoom'], 9, 3.5, 14, 6], 'circle-color': HOTSPOT_COLOUR, 'circle-stroke-color': P.halo, 'circle-stroke-width': 2 } },
    { id: 'hotspot-labels', type: 'symbol', source: HOTSPOTS, minzoom: 12, layout: { 'text-field': ['get', 'name'], 'text-font': ['Noto Sans Medium'], 'text-size': 11.5, 'text-anchor': 'top', 'text-offset': [0, 0.8], 'text-optional': true }, paint: { 'text-color': HOTSPOT_COLOUR, 'text-halo-color': P.halo, 'text-halo-width': 1.6 } },
    { id: 'heritage-sites', type: 'circle', source: HERITAGE, paint: { 'circle-radius': ['interpolate', ['linear'], ['zoom'], 9, ['match', ['get', 'status'], 'unesco', 6, 4], 15, ['match', ['get', 'status'], 'unesco', 11, 8]], 'circle-color': ['match', ['get', 'status'], 'unesco', HERITAGE_COLOURS.unesco, 'precinct', HERITAGE_COLOURS.precinct, HERITAGE_COLOURS.monument], 'circle-stroke-color': P.halo, 'circle-stroke-width': 2.5 } },
    { id: 'heritage-labels', type: 'symbol', source: HERITAGE, minzoom: 11, layout: { 'text-field': ['get', 'name'], 'text-font': ['Noto Sans Medium'], 'text-size': 12, 'text-anchor': 'top', 'text-offset': [0, 1], 'text-max-width': 9, 'text-optional': true }, paint: { 'text-color': P.text, 'text-halo-color': P.halo, 'text-halo-width': 1.8 } },
    { id: 'sel-fill', type: 'fill', source: SEL, filter: ['==', ['geometry-type'], 'Polygon'], paint: { 'fill-color': P.selection, 'fill-opacity': 0.12 } },
    { id: 'sel-casing', type: 'line', source: SEL, filter: ['!=', ['geometry-type'], 'Point'], layout: { 'line-cap': 'round', 'line-join': 'round' }, paint: { 'line-color': P.halo, 'line-width': ['+', ['coalesce', ['get', 'width'], 5], 4], 'line-opacity': 0.9 } },
    { id: 'sel-line', type: 'line', source: SEL, filter: ['all', ['!=', ['geometry-type'], 'Point'], ['!', ['to-boolean', ['get', 'dashed']]]], layout: { 'line-cap': 'round', 'line-join': 'round' }, paint: { 'line-color': ['coalesce', ['get', 'colour'], P.selection], 'line-width': ['coalesce', ['get', 'width'], 5] } },
    // Walking routes: dashes read as "on foot" in every map app.
    { id: 'sel-line-dashed', type: 'line', source: SEL, filter: ['all', ['!=', ['geometry-type'], 'Point'], ['to-boolean', ['get', 'dashed']]], layout: { 'line-cap': 'butt', 'line-join': 'round' }, paint: { 'line-color': ['coalesce', ['get', 'colour'], P.selection], 'line-width': ['coalesce', ['get', 'width'], 5], 'line-dasharray': [1.4, 1] } },
    { id: 'sel-stop', type: 'circle', source: SEL, filter: ['all', ['==', ['geometry-type'], 'Point'], ['==', ['get', 'role'], 'stop']], paint: { 'circle-radius': 5, 'circle-color': P.halo, 'circle-stroke-color': ['coalesce', ['get', 'colour'], P.selection], 'circle-stroke-width': 3 } },
    { id: 'sel-stop-label', type: 'symbol', source: SEL, filter: ['all', ['==', ['geometry-type'], 'Point'], ['==', ['get', 'role'], 'stop']], minzoom: 11, layout: { 'text-field': ['coalesce', ['get', 'name'], ''], 'text-font': ['Noto Sans Medium'], 'text-size': 11.5, 'text-anchor': 'left', 'text-offset': [0.9, 0], 'text-optional': true }, paint: { 'text-color': P.text, 'text-halo-color': P.halo, 'text-halo-width': 1.6 } },
    { id: 'sel-point', type: 'circle', source: SEL, filter: ['all', ['==', ['geometry-type'], 'Point'], ['!=', ['get', 'role'], 'stop']], paint: { 'circle-radius': 19, 'circle-color': P.selection, 'circle-opacity': 0.16, 'circle-stroke-color': P.selection, 'circle-stroke-width': 2 } },
    { id: 'measure-line', type: 'line', source: MEASURE, filter: ['==', ['geometry-type'], 'LineString'], layout: { 'line-cap': 'round' }, paint: { 'line-color': P.areaHighlight, 'line-width': 3, 'line-dasharray': [2, 1.5] } },
    { id: 'measure-points', type: 'circle', source: MEASURE, filter: ['==', ['geometry-type'], 'Point'], paint: { 'circle-radius': 5, 'circle-color': P.halo, 'circle-stroke-color': P.areaHighlight, 'circle-stroke-width': 2.5 } },
  ];
}

export class MapController {
  readonly map: maplibregl.Map;
  private theme: ThemeName;
  private ready = false;
  private hidden: string[] = [];
  private area = '';
  private overlays: Overlays = { buildings3d: false, bridges: false, railColours: true, boundaries: true, heritage: false, hotspots: false };
  private selection: GeoJSON.FeatureCollection = EMPTY;
  private measure: GeoJSON.FeatureCollection = EMPTY;
  private pinMarker: maplibregl.Marker | null = null;
  private pinListener: (() => void) | null = null;

  constructor(
    container: HTMLElement,
    private readonly base: string,
    theme: ThemeName,
    private readonly onPick: (pick: MapPick | null, e: MapMouseEvent) => void,
  ) {
    this.theme = theme;
    this.map = new maplibregl.Map({
      container,
      style: this.composeStyle(),
      center: [72.8777, 19.076],
      zoom: 11,
      minZoom: 7,
      maxZoom: 20,
      maxBounds: [[71.9, 18.2], [74.6, 20.4]],
      hash: 'map',
      attributionControl: { compact: true },
      fadeDuration: 150,
      maxPitch: 75,
    });
    this.map.addControl(new maplibregl.NavigationControl({ visualizePitch: true }), 'bottom-right');
    this.map.addControl(new maplibregl.GeolocateControl({ positionOptions: { enableHighAccuracy: true }, trackUserLocation: false }), 'bottom-right');
    this.map.addControl(new maplibregl.ScaleControl({ unit: 'metric', maxWidth: 120 }), 'bottom-left');

    // POI badges are drawn on demand, the first time a category appears on screen.
    this.map.setMissingStyleImageResolver((id) => this.provideImage(id));
    this.map.once('style.load', () => {
      this.ready = true;
      // Apply anything that changed between construction and the first style load.
      this.map.setGlobalStateProperty('hiddenCategories', this.hidden);
      this.map.setGlobalStateProperty('area', this.area);
      this.applyOverlays();
    });
    this.map.on('click', (e) => this.onPick(this.pick(e.point), e));
    let hoverFrame = 0;
    this.map.on('mousemove', (e) => {
      cancelAnimationFrame(hoverFrame);
      hoverFrame = requestAnimationFrame(() => {
        if (!this.ready) return;
        const hit = this.map.queryRenderedFeatures(e.point, { layers: this.visibleLayers(HOVERABLE) });
        this.map.getCanvas().style.cursor = hit.length ? 'pointer' : '';
      });
    });
  }

  /** The complete style for the current theme, including app overlays and the current UI state. */
  private composeStyle(): StyleSpecification {
    const P = palettes[this.theme];
    const style = buildStyle({ theme: this.theme, base: this.base, app: true });
    style.state = { hiddenCategories: { default: this.hidden }, area: { default: this.area } };
    style.sources[HERITAGE] = { type: 'geojson', data: `${this.base}/api/v1/heritage?format=geojson` };
    style.sources[HOTSPOTS] = { type: 'geojson', data: `${this.base}/api/v1/hotspots?format=geojson` };
    style.sources[SEL] = { type: 'geojson', data: this.selection };
    style.sources[MEASURE] = { type: 'geojson', data: this.measure };
    style.layers.push(...overlayLayers(P));
    const visibility = this.visibilityMap();
    for (const layer of style.layers) {
      const v = visibility.get(layer.id);
      if (v) layer.layout = { ...(layer.layout ?? {}), visibility: v } as typeof layer.layout;
    }
    return style;
  }

  private visibilityMap(): Map<string, 'visible' | 'none'> {
    const out = new Map<string, 'visible' | 'none'>();
    for (const [key, ids] of Object.entries(OVERLAY_LAYERS) as [keyof Overlays, string[]][]) {
      for (const raw of ids) {
        const inverted = raw.startsWith('!');
        const on = inverted ? !this.overlays[key] : this.overlays[key];
        out.set(inverted ? raw.slice(1) : raw, on ? 'visible' : 'none');
      }
    }
    return out;
  }

  // ---------------------------------------------------------------- style state

  setTheme(theme: ThemeName): void {
    if (theme === this.theme) return;
    this.theme = theme;
    // Badges carry a theme-dependent ring; drop them so the resolver redraws them.
    for (const id of this.map.listImages()) if (id.startsWith('poi-')) this.map.removeImage(id);
    this.map.setStyle(this.composeStyle(), { diff: true });
  }

  getTheme(): ThemeName {
    return this.theme;
  }

  setHiddenCategories(ids: string[]): void {
    this.hidden = ids;
    if (this.ready) this.map.setGlobalStateProperty('hiddenCategories', ids);
  }

  setArea(id: string | null): void {
    this.area = id ?? '';
    if (this.ready) this.map.setGlobalStateProperty('area', this.area);
  }

  setOverlay<K extends keyof Overlays>(key: K, on: boolean): void {
    this.overlays[key] = on;
    this.applyOverlays();
    if (key === 'buildings3d') {
      if (on && this.map.getPitch() < 30) this.map.easeTo({ pitch: 55, zoom: Math.max(this.map.getZoom(), 15.5), duration: 900 });
      if (!on && this.map.getPitch() > 0) this.map.easeTo({ pitch: 0, duration: 600 });
    }
  }

  getOverlays(): Overlays {
    return { ...this.overlays };
  }

  private applyOverlays(): void {
    if (!this.ready) return;
    for (const [id, v] of this.visibilityMap()) {
      if (this.map.getLayer(id)) this.map.setLayoutProperty(id, 'visibility', v);
    }
  }

  private provideImage(id: string): void {
    if (!id.startsWith('poi-') || this.map.hasImage(id)) return;
    const badge = renderBadge(id.slice(4), this.theme);
    if (badge) this.map.addImage(id, badge.image, { pixelRatio: badge.pixelRatio });
  }

  // ---------------------------------------------------------------- selection, measuring, pin

  setSelection(fc: GeoJSON.FeatureCollection | null): void {
    this.selection = fc ?? EMPTY;
    (this.map.getSource(SEL) as maplibregl.GeoJSONSource | undefined)?.setData(this.selection);
  }

  setMeasure(fc: GeoJSON.FeatureCollection | null): void {
    this.measure = fc ?? EMPTY;
    (this.map.getSource(MEASURE) as maplibregl.GeoJSONSource | undefined)?.setData(this.measure);
  }

  setPin(lngLat: [number, number] | null, onMove?: (lngLat: [number, number]) => void): void {
    if (!lngLat) {
      this.pinMarker?.remove();
      this.pinMarker = null;
      this.pinListener = null;
      return;
    }
    if (!this.pinMarker) {
      this.pinMarker = new maplibregl.Marker({ color: '#e8590c', draggable: true }).setLngLat(lngLat).addTo(this.map);
    } else {
      this.pinMarker.setLngLat(lngLat);
    }
    const marker = this.pinMarker;
    if (this.pinListener) marker.off('dragend', this.pinListener);
    this.pinListener = onMove
      ? () => {
          const p = marker.getLngLat();
          onMove([p.lng, p.lat]);
        }
      : null;
    if (this.pinListener) marker.on('dragend', this.pinListener);
  }

  // ---------------------------------------------------------------- camera & picking

  fitBounds(bbox: [number, number, number, number], maxZoom = 16): void {
    this.map.fitBounds(bbox as LngLatBoundsLike, { padding: this.padding(), maxZoom, duration: 900 });
  }

  flyTo(lngLat: [number, number], zoom = 16.5): void {
    this.map.flyTo({ center: lngLat, zoom: Math.max(zoom, this.map.getZoom()), padding: this.padding(), duration: 1000, essential: true });
  }

  /** Keeps fitted content clear of the side panel and the inspector. */
  /** Map padding that keeps fitted features clear of the dashboard (unless hidden), inspector and attribution. */
  private padding() {
    const hidden = document.body.classList.contains('panel-off');
    if (window.innerWidth > 820) return { top: 80, bottom: 60, left: hidden ? 60 : 420, right: 440 };
    return { top: 70, bottom: hidden ? 90 : Math.round(window.innerHeight * 0.5) + 56, left: 30, right: 64 };
  }

  visibleBounds(): [number, number, number, number] {
    const b = this.map.getBounds();
    return [b.getWest(), b.getSouth(), b.getEast(), b.getNorth()];
  }

  private visibleLayers(ids: string[]): string[] {
    return ids.filter((id) => this.map.getLayer(id) && this.map.getLayoutProperty(id, 'visibility') !== 'none');
  }

  private pick(point: maplibregl.Point): MapPick | null {
    if (!this.ready) return null;
    const box: [maplibregl.PointLike, maplibregl.PointLike] = [[point.x - 5, point.y - 5], [point.x + 5, point.y + 5]];
    const hits = this.map.queryRenderedFeatures(box, { layers: this.visibleLayers(INTERACTIVE) });
    if (!hits.length) return null;
    hits.sort((a, b) => INTERACTIVE.indexOf(a.layer.id) - INTERACTIVE.indexOf(b.layer.id));
    const ll = this.map.unproject(point);
    return { layer: hits[0].layer.id, feature: hits[0], lngLat: [ll.lng, ll.lat] };
  }
}
