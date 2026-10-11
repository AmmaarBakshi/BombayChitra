import { BombayChitraClient } from './api/client.ts';
import { MapController } from './map/controller.ts';
import type { AppContext } from './ui/context.ts';
import { $, h, storageGet, storageSet } from './ui/dom.ts';
import { formatCoord, formatDate } from './ui/format.ts';
import { hashParams, parseSelection, updateHash } from './ui/hash.ts';
import { icon } from './ui/icons.ts';
import { Inspector } from './ui/inspector.ts';
import { MeasureTool } from './ui/measure.ts';
import { BridgesPanel } from './ui/panels/bridges.ts';
import { BuildingsPanel } from './ui/panels/buildings.ts';
import { DistancePanel } from './ui/panels/distance.ts';
import { ExplorePanel } from './ui/panels/explore.ts';
import { LayersPanel } from './ui/panels/layers.ts';
import type { Panel } from './ui/panels/panel.ts';
import { TransitPanel } from './ui/panels/transit.ts';
import { Search } from './ui/search.ts';
import { ThemeManager } from './ui/theme.ts';

const TAB_KEY = 'bc-tab';
const PANEL_KEY = 'bc-panel';

export function createApp(): void {
  const theme = new ThemeManager();
  // The app normally runs on the BombayChitra server itself; VITE_API_BASE points it at another server.
  const apiBase = (import.meta.env.VITE_API_BASE ?? '').replace(/\/+$/, '');
  const origin = apiBase || location.origin;
  const api = new BombayChitraClient(apiBase);
  const inspector = new Inspector($('#inspector'));
  const panelEl = $('#panel');

  let area: string | null = hashParams().get('area');
  let measure: MeasureTool | null = null;
  let distance: DistancePanel | null = null;
  const panels: Panel[] = [];

  const map = new MapController($('#map'), origin, theme.resolved, (pick, e) => {
    if (distance?.picking) return void distance.pick([e.lngLat.lng, e.lngLat.lat]);
    if (measure?.active) return measure.add([e.lngLat.lng, e.lngLat.lat]);
    if (pick) {
      void inspector.fromPick(pick);
      if (window.innerWidth <= 820) panelEl.classList.add('collapsed');
    } else if (inspector.isOpen) {
      inspector.close();
    }
  });

  const toastEl = $('#toast');
  let toastTimer: ReturnType<typeof setTimeout> | undefined;
  const ctx: AppContext = {
    api,
    map,
    inspector,
    meta: null,
    toast(message) {
      toastEl.textContent = message;
      toastEl.hidden = false;
      clearTimeout(toastTimer);
      toastTimer = setTimeout(() => (toastEl.hidden = true), 2400);
    },
    currentArea: () => area,
    selectArea(id, fit = false) {
      area = id;
      map.setArea(id);
      updateHash({ area: id });
      const a = ctx.meta?.areas.find((x) => x.id === id);
      if (fit && a) map.fitBounds(a.bbox, 14);
      panels.forEach((p) => p.onAreaChange?.(id));
    },
    routeTo(point) {
      openDistance();
      distance!.setPoint('to', point);
      if (distance!.missing === 'from') distance!.startPick('from');
    },
  };
  inspector.attach(ctx);
  map.setArea(area);
  // Scripting hook for embedders and debugging: window.bombaychitra.map is the MapLibre map.
  (window as unknown as { bombaychitra: unknown }).bombaychitra = { map: map.map, api, inspector, selectArea: ctx.selectArea };
  theme.onChange((t) => map.setTheme(t));

  // ---------------------------------------------------------------- panels & tabs

  const layers = new LayersPanel(ctx);
  distance = new DistancePanel(ctx);
  panels.push(new ExplorePanel(ctx, layers), layers, new TransitPanel(ctx), new BridgesPanel(ctx, layers), new BuildingsPanel(ctx, layers), distance);
  const tabs = $('.tabs');
  const body = $('#panel-body');
  const rendered = new Set<string>();
  const bodies = new Map<string, HTMLElement>();
  const show = (id: string) => {
    const panel = panels.find((p) => p.id === id) ?? panels[0];
    tabs.querySelectorAll('button').forEach((b) => b.setAttribute('aria-selected', String(b.dataset.tab === panel.id)));
    $('#tool-route').setAttribute('aria-pressed', String(panel.id === 'distance'));
    for (const [pid, el] of bodies) el.hidden = pid !== panel.id;
    if (!rendered.has(panel.id)) {
      rendered.add(panel.id);
      void panel.render(bodies.get(panel.id)!);
    }
    storageSet(TAB_KEY, panel.id);
  };
  for (const p of panels) {
    const el = h('div', { role: 'tabpanel', id: `tab-${p.id}`, hidden: true });
    bodies.set(p.id, el);
    body.append(el);
    tabs.append(h('button', { type: 'button', role: 'tab', 'data-tab': p.id, 'aria-controls': `tab-${p.id}`, 'aria-selected': 'false', onclick: () => show(p.id) }, icon(p.icon, 18), p.label));
  }

  // ---------------------------------------------------------------- dashboard hider

  const panelBtn = $('#tool-panel');
  const reveal = $('#panel-reveal');
  panelBtn.append(icon('sidebar', 19));
  reveal.append(icon('chevron', 16), h('span', {}, 'Dashboard'));
  const setPanelHidden = (hidden: boolean, announce = true) => {
    document.body.classList.toggle('panel-off', hidden);
    panelEl.classList.toggle('panel-hidden', hidden);
    panelEl.toggleAttribute('inert', hidden);
    reveal.hidden = !hidden;
    panelBtn.setAttribute('aria-pressed', String(hidden));
    panelBtn.title = hidden ? 'Show dashboard (H)' : 'Hide dashboard (H)';
    panelBtn.setAttribute('aria-label', panelBtn.title);
    storageSet(PANEL_KEY, hidden ? 'hidden' : 'shown');
    if (announce) ctx.toast(hidden ? 'Dashboard hidden — press H to show it' : 'Dashboard shown');
  };
  const togglePanel = () => setPanelHidden(!panelEl.classList.contains('panel-hidden'));
  const openDistance = () => {
    if (panelEl.classList.contains('panel-hidden')) setPanelHidden(false, false);
    panelEl.classList.remove('collapsed');
    show('distance');
  };
  setPanelHidden(storageGet(PANEL_KEY) === 'hidden', false);
  panelBtn.addEventListener('click', togglePanel);
  reveal.addEventListener('click', () => setPanelHidden(false));
  document.addEventListener('keydown', (e) => {
    const target = e.target as HTMLElement | null;
    if (e.key.toLowerCase() !== 'h' || e.ctrlKey || e.metaKey || e.altKey) return;
    if (target && (target.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(target.tagName))) return;
    e.preventDefault();
    togglePanel();
  });

  // ---------------------------------------------------------------- toolbar

  const themeBtn = $('#tool-theme');
  const themeIcon = () => {
    themeBtn.replaceChildren(icon(theme.current === 'system' ? 'system' : theme.current === 'dark' ? 'moon' : 'sun', 19));
    themeBtn.title = `Theme: ${theme.current} (click to change)`;
    themeBtn.setAttribute('aria-label', themeBtn.title);
  };
  themeIcon();
  themeBtn.addEventListener('click', () => {
    theme.cycle();
    themeIcon();
    ctx.toast(`Theme: ${theme.current}`);
  });

  const toggleBtn = (sel: string, iconName: string, key: 'buildings3d' | 'bridges') => {
    const btn = $(sel);
    btn.append(icon(iconName, 19));
    btn.addEventListener('click', () => layers.setOverlay(key, !map.getOverlays()[key]));
    layers.onOverlayChange((k, on) => k === key && btn.setAttribute('aria-pressed', String(on)));
  };
  toggleBtn('#tool-3d', 'cube', 'buildings3d');
  toggleBtn('#tool-bridges', 'bridge', 'bridges');
  const routeBtn = $('#tool-route');
  routeBtn.append(icon('route', 19));
  routeBtn.addEventListener('click', () => {
    openDistance();
    if (distance!.missing) (document.querySelector('.route-field input') as HTMLInputElement | null)?.focus();
  });
  const measureBtn = $('#tool-measure');
  measureBtn.append(icon('ruler', 19));
  measure = new MeasureTool(map, measureBtn);
  measureBtn.addEventListener('click', () => measure!.toggle());
  $('.toolbar a').append(icon('docs', 19));

  // ---------------------------------------------------------------- search, status, gestures

  new Search(ctx);

  const coords = $('#status-coords');
  const zoom = $('#status-zoom');
  map.map.on('mousemove', (e) => (coords.textContent = formatCoord(e.lngLat.lng, e.lngLat.lat, 5)));
  const updateZoom = () => (zoom.textContent = `z${map.map.getZoom().toFixed(1)}`);
  map.map.on('zoom', updateZoom);
  updateZoom();

  map.map.on('contextmenu', (e) => {
    e.preventDefault();
    void inspector.reverse([e.lngLat.lng, e.lngLat.lat]);
  });
  // Long-press on touch screens = "What's here?"
  let pressTimer: ReturnType<typeof setTimeout> | undefined;
  map.map.on('touchstart', (e) => {
    if (e.originalEvent.touches.length !== 1) return;
    pressTimer = setTimeout(() => void inspector.reverse([e.lngLat.lng, e.lngLat.lat]), 600);
  });
  for (const ev of ['touchend', 'touchmove', 'movestart'] as const) map.map.on(ev, () => clearTimeout(pressTimer));

  // Bottom-sheet behaviour on small screens: tap the grip to cycle sizes.
  $('.panel-grip').addEventListener('click', () => {
    if (panelEl.classList.contains('collapsed')) panelEl.classList.remove('collapsed');
    else if (panelEl.classList.contains('expanded')) panelEl.classList.replace('expanded', 'collapsed');
    else panelEl.classList.add('expanded');
  });
  $('#search-input').addEventListener('focus', () => panelEl.classList.remove('collapsed'));

  // ---------------------------------------------------------------- data

  void api.meta().then((meta) => {
    ctx.meta = meta;
    $('#region-name').textContent = meta.region.name;
    $('#status-data').textContent = `OSM data ${formatDate(meta.dataset.source_timestamp ?? meta.dataset.built_at)}`;
    show(storageGet(TAB_KEY) ?? 'explore');
    if (area) ctx.selectArea(area, false);
    if (distance!.restore()) openDistance();
  }).catch(() => {
    show('layers');
    ctx.toast('Cannot reach the BombayChitra API');
  });

  map.map.once('load', () => {
    const sel = parseSelection(hashParams().get('sel'));
    if (!sel) return;
    switch (sel.kind) {
      case 'place': return void inspector.place(sel.id);
      case 'street': return void inspector.street(sel.id);
      case 'bridge': return void inspector.namedBridge(sel.id, false);
      case 'route': return void inspector.route(sel.id);
      case 'building': return void inspector.buildingById(sel.id, false);
      case 'pin': return void inspector.reverse([sel.lon, sel.lat]);
    }
  });
}
