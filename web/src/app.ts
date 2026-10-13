import { BombayChitraClient } from './api/client.ts';
import { MapController } from './map/controller.ts';
import type { AppContext } from './ui/context.ts';
import { $, h, storageGet, storageSet } from './ui/dom.ts';
import { formatCoord, formatDate } from './ui/format.ts';
import { hashParams, parseSelection, updateHash } from './ui/hash.ts';
import { type Language, language, setLanguage, t } from './i18n.ts';
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
  const lang = language();
  document.documentElement.lang = lang;
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
  }, lang);

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
    // Roving tabindex: only the selected tab is in the Tab order; arrow keys move between tabs.
    tabs.querySelectorAll('button').forEach((b) => {
      b.setAttribute('aria-selected', String(b.dataset.tab === panel.id));
      b.tabIndex = b.dataset.tab === panel.id ? 0 : -1;
    });
    $('#tool-route').setAttribute('aria-pressed', String(panel.id === 'distance'));
    for (const [pid, el] of bodies) el.hidden = pid !== panel.id;
    if (!rendered.has(panel.id)) {
      rendered.add(panel.id);
      void panel.render(bodies.get(panel.id)!);
    }
    storageSet(TAB_KEY, panel.id);
  };
  for (const p of panels) {
    const el = h('div', { role: 'tabpanel', id: `tab-${p.id}`, 'aria-labelledby': `tabbtn-${p.id}`, tabindex: 0, hidden: true });
    bodies.set(p.id, el);
    body.append(el);
    tabs.append(h('button', { type: 'button', role: 'tab', id: `tabbtn-${p.id}`, 'data-tab': p.id, 'aria-controls': `tab-${p.id}`, 'aria-selected': 'false', tabindex: -1, onclick: () => show(p.id) }, icon(p.icon, 18), p.label));
  }
  tabs.addEventListener('keydown', (e) => {
    const buttons = [...tabs.querySelectorAll<HTMLButtonElement>('button[role="tab"]')];
    const i = buttons.indexOf(document.activeElement as HTMLButtonElement);
    if (i < 0) return;
    const next = { ArrowRight: i + 1, ArrowLeft: i - 1, Home: 0, End: buttons.length - 1 }[e.key];
    if (next === undefined) return;
    e.preventDefault();
    const target = buttons[(next + buttons.length) % buttons.length];
    target.focus();
    show(target.dataset.tab!);
  });

  // ---------------------------------------------------------------- dashboard hider

  const panelBtn = $('#tool-panel');
  const reveal = $('#panel-reveal');
  panelBtn.append(icon('sidebar', 19));
  reveal.append(icon('chevron', 16), h('span', {}, t('tool.dashboard')));
  reveal.title = t('tool.showPanel');
  const setPanelHidden = (hidden: boolean, announce = true) => {
    document.body.classList.toggle('panel-off', hidden);
    panelEl.classList.toggle('panel-hidden', hidden);
    panelEl.toggleAttribute('inert', hidden);
    reveal.hidden = !hidden;
    panelBtn.setAttribute('aria-pressed', String(hidden));
    panelBtn.title = hidden ? t('tool.showPanel') : t('tool.hidePanel');
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
    themeBtn.title = `${t('tool.theme')}: ${theme.current}`;
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

  // Interface language (also the map's label language); switching reloads the app in the new language.
  const toolTitles: [string, Parameters<typeof t>[0]][] = [['#tool-3d', 'tool.3d'], ['#tool-bridges', 'tool.bridges'], ['#tool-route', 'tool.route'], ['#tool-measure', 'tool.measure'], ['.toolbar a', 'tool.docs']];
  for (const [sel, key] of toolTitles) {
    $(sel).title = t(key);
    $(sel).setAttribute('aria-label', t(key));
  }
  const langBtn = $('#tool-lang');
  const order: Language[] = ['en', 'mr', 'hi'];
  const next = order[(order.indexOf(lang) + 1) % order.length];
  langBtn.textContent = t('lang.short');
  langBtn.title = `${t('lang.switch')}: ${t('lang.name')} → ${t('lang.name', {}, next)}`;
  langBtn.setAttribute('aria-label', langBtn.title);
  langBtn.addEventListener('click', () => setLanguage(next));
  // "Skip to the map" focuses the map canvas, so the arrow keys pan it straight away.
  $('[data-skip="map"]').addEventListener('click', (e) => {
    e.preventDefault();
    map.map.getCanvas().focus();
  });
  const searchInput = $('#search-input') as HTMLInputElement;
  searchInput.placeholder = t('search.placeholder');
  $('label[for="search-input"]').textContent = t('search.label');

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
    distance!.setAvailableModes(meta.routing?.modes ?? ['car']);
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
      case 'pincode': return void inspector.pinCode(sel.pincode);
    }
  });
}
