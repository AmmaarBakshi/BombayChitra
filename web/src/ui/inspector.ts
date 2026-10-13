import { ApiError } from '../api/client.ts';
import type { Area, HeritageSite, LineRef, PlaceDetail, SearchResult, TransitRoute } from '../api/types.ts';
import type { MapPick } from '../map/controller.ts';
import { categoryById, groupById } from '../taxonomy.ts';
import type { AppContext } from './context.ts';
import { clear, h } from './dom.ts';
import { capitalise, formatArea, formatCoord, formatDistance, formatDMS, formatNumber, formatOsmDate, isRecent, parseOsmDate } from './format.ts';
import { updateHash } from './hash.ts';
import { t } from '../i18n.ts';
import { categoryBadge, icon, kindBadge } from './icons.ts';

type Head = { badge: HTMLElement; title: string; kicker?: string };
type LonLat = [number, number];

const ROAD_LABELS: Record<string, string> = {
  motorway: 'Expressway', trunk: 'Highway', primary: 'Main road', secondary: 'Arterial road', tertiary: 'Road',
  minor: 'Street', service: 'Service road', pedestrian: 'Pedestrian street', track: 'Track', path: 'Footpath', construction: 'Under construction',
};
const MODE_LABELS: Record<string, string> = { subway: 'Metro', light_rail: 'Light rail', monorail: 'Monorail', train: 'Suburban railway', tram: 'Tram', rail: 'Railway', construction: 'Under construction', proposed: 'Proposed' };
const STATE_LABELS: Record<string, string> = { construction: 'Under construction', proposed: 'Proposed' };
const BRIDGE_LABELS: Record<string, string> = { road: 'Road bridge / flyover', rail: 'Railway bridge', metro: 'Metro viaduct', foot: 'Foot over-bridge', construction: 'Bridge under construction', aqueduct: 'Aqueduct', outline: 'Bridge structure' };

export class Inspector {
  private ctx!: AppContext;
  private abort: AbortController | null = null;

  constructor(private readonly el: HTMLElement) {
    document.addEventListener('keydown', (e) => {
      if (e.key === 'Escape' && !this.el.hidden) this.close();
    });
  }

  attach(ctx: AppContext): void {
    this.ctx = ctx;
  }

  get isOpen(): boolean {
    return !this.el.hidden;
  }

  close(): void {
    this.abort?.abort();
    this.el.hidden = true;
    clear(this.el);
    this.ctx.map.setSelection(null);
    this.ctx.map.setPin(null);
    setSelectionHash(null);
  }

  // ---------------------------------------------------------------- entry points

  async fromPick(pick: MapPick): Promise<void> {
    const f = pick.feature;
    const p = f.properties as Record<string, string | number | undefined>;
    const id = typeof f.id === 'number' ? f.id : undefined;
    switch (pick.layer) {
      case 'heritage-sites':
        if (p.place_id) return this.place(Number(p.place_id));
        return this.heritageSite(p, pick.lngLat);
      case 'hotspot-icon':
        return this.hotspot(p, pick.lngLat);
      case 'poi':
        if (id) return this.place(id);
        break;
      case 'place-major':
      case 'place-minor':
      case 'water-label':
        return this.locality(String(p.name ?? ''), String(p.kind ?? ''), pick.lngLat);
      case 'metro':
      case 'metro-tunnel':
      case 'metro-bridge':
      case 'metro-construction':
      case 'metro-proposed':
      case 'rail':
      case 'rail-lines':
        return this.railway(p, pick.lngLat);
      case 'bridges-line':
        if (id) return this.bridge(id, pick.lngLat);
        break;
      case 'road':
      case 'road-bridge':
      case 'road-tunnel':
      case 'road-path':
      case 'path-bridge':
      case 'road-construction':
        return this.road(id, p, pick.lngLat);
      case 'buildings-2d':
      case 'buildings-3d':
        return this.building(id, p, pick.lngLat);
    }
    return this.reverse(pick.lngLat, { title: capitalise(String(p.kind ?? pick.layer.replace(/-.*/, ''))) });
  }

  async searchResult(r: SearchResult): Promise<void> {
    switch (r.kind) {
      case 'place':
        return this.place(Number(r.id), true);
      case 'street':
        return this.street(Number(r.id));
      case 'bridge':
        return this.namedBridge(Number(r.id));
      case 'line':
        return this.route(Number(r.id));
      case 'area':
        if (r.category === 'pincode') return this.pinCode(String(r.id));
        this.close();
        this.ctx.selectArea(String(r.id), true);
        return;
      case 'locality':
        this.ctx.map.flyTo([r.lon, r.lat], 14.5);
        return this.locality(r.name, r.category ?? 'locality', [r.lon, r.lat]);
    }
  }

  // ---------------------------------------------------------------- places

  async place(id: number, fly = false): Promise<void> {
    const signal = this.begin();
    this.loading();
    let d: PlaceDetail;
    try {
      d = await this.ctx.api.place(id, signal);
    } catch (err) {
      return this.failed(err);
    }
    if (signal.aborted) return;
    if (fly) this.ctx.map.flyTo([d.lon, d.lat], 17);
    this.select({ type: 'Feature', geometry: { type: 'Point', coordinates: [d.lon, d.lat] }, properties: {} });
    setSelectionHash(`place:${d.id}`);

    const cat = categoryById.get(d.category);
    const group = cat ? groupById.get(cat.group) : undefined;
    const rows: Node[] = [];
    if (d.heritage) rows.push(this.row('info', 'Heritage', heritageStatus(d.heritage), d.heritage.wikipedia ? h('span', {}, ' · ', wikipediaLink(d.heritage.wikipedia)) : null));
    if (d.lines?.length) rows.push(this.row('train', t('card.lines'), this.lineChips(d.lines)));
    if (STATION_CATEGORIES.has(d.category)) rows.push(...this.stationRows(d));
    const opened = d.tags.opening_date ?? d.tags.start_date;
    if (d.address) rows.push(this.row('pin', t('card.address'), d.address));
    if (d.phone) rows.push(this.row('phone', t('card.phone'), ...d.phone.split(/[;,]/).map((p) => h('a', { href: `tel:${p.trim().replace(/\s/g, '')}` }, p.trim(), ' '))));
    if (d.website) rows.push(this.row('globe', t('card.website'), h('a', { href: safeUrl(d.website), target: '_blank', rel: 'noopener' }, prettyUrl(d.website))));
    if (d.opening_hours) rows.push(this.row('clock', t('card.hours'), d.opening_hours));
    if (d.cuisine) rows.push(this.row('info', 'Cuisine', d.cuisine.split(';').map(capitalise).join(', ')));
    if (d.operator && !STATION_CATEGORIES.has(d.category)) rows.push(this.row('info', 'Operator', d.operator));
    if (d.brand && d.brand !== d.name) rows.push(this.row('info', 'Brand', d.brand));
    if (d.wheelchair) rows.push(this.row('wheelchair', 'Wheelchair', capitalise(d.wheelchair)));
    if (d.area_m2) rows.push(this.row('explore', 'Area', formatArea(d.area_m2)));
    if (d.areas.length) rows.push(this.row('explore', t('card.within'), d.areas.map((a) => a.name).join(' · ')));
    rows.push(this.coordRow(d.lon, d.lat));
    const overture = d.source === 'overture';
    if (overture) {
      const via = d.tags['overture:sources']?.split(';').join(', ');
      rows.push(this.row('info', 'Source', h('span', {}, 'Overture Maps', via ? ` (via ${via})` : '', ' · not yet on OpenStreetMap — ',
        h('a', { href: `https://www.openstreetmap.org/edit#map=19/${d.lat.toFixed(6)}/${d.lon.toFixed(6)}`, target: '_blank', rel: 'noopener' }, 'add it'))));
    }

    this.render(
      { badge: categoryBadge(d.category, 40), title: d.name ?? d.category_label, kicker: [d.category_label, d.locality].filter(Boolean).join(' · ') },
      [
        isRecent(opened) ? h('div', { class: 'pill new', title: `Opened ${formatOsmDate(opened!)}` }, `New · opened ${formatOsmDate(opened!)}`) : null,
        d.name_local ? h('div', { class: 'sub', lang: 'mr', style: 'margin:-2px 0 6px' }, d.name_local) : null,
        d.heritage?.description ? h('p', { class: 'sub', style: 'white-space:normal;margin:0 0 8px' }, d.heritage.description) : null,
        ...rows,
        this.actions(d.lon, d.lat, d.osm_url || undefined, d.name),
        this.tagTable(d.tags, overture ? 'Source attributes' : undefined),
      ],
      group?.colour,
    );
  }

  // ---------------------------------------------------------------- roads & streets

  private async road(id: number | undefined, p: Record<string, string | number | undefined>, at: LonLat): Promise<void> {
    if (id && (p.name || p.ref)) {
      const signal = this.begin();
      this.loading();
      try {
        const road = await this.ctx.api.road(id, signal);
        if (signal.aborted) return;
        return this.street(road.street_id, road.id);
      } catch (err) {
        if (!(err instanceof ApiError && err.status === 404)) return this.failed(err);
      }
    }
    const kind = String(p.kind ?? 'road');
    this.select(null);
    this.ctx.map.setPin(at);
    this.render({ badge: kindBadge('street', 'var(--muted)', 40), title: String(p.name ?? p.ref ?? ROAD_LABELS[kind] ?? 'Road'), kicker: ROAD_LABELS[kind] }, [
      this.roadFacts(p),
      this.coordRow(at[0], at[1]),
    ]);
  }

  async street(id: number, focusWay?: number): Promise<void> {
    const signal = this.begin();
    this.loading();
    try {
      const s = await this.ctx.api.street(id, signal);
      if (signal.aborted) return;
      this.select({ type: 'Feature', geometry: s.geometry, properties: { width: 6 } });
      if (!focusWay) this.ctx.map.fitBounds(s.bbox, 17);
      setSelectionHash(`street:${s.id}`);
      const way = s.ways.find((w) => w.id === focusWay) ?? s.ways.reduce((a, b) => (b.length_m > a.length_m ? b : a), s.ways[0]);
      const lanes = [...new Set(s.ways.map((w) => w.lanes).filter(Boolean))].join(' / ');
      const speeds = [...new Set(s.ways.map((w) => w.maxspeed).filter(Boolean))].join(' / ');
      const bridges = s.ways.filter((w) => w.bridge).length;
      const tunnels = s.ways.filter((w) => w.tunnel).length;
      this.render({ badge: kindBadge('street', 'var(--accent)', 40), title: s.name ?? s.ref ?? 'Road', kicker: [ROAD_LABELS[s.kind], s.locality].filter(Boolean).join(' · ') }, [
        h('dl', { class: 'kv' },
          s.ref && s.name ? [h('dt', {}, 'Route'), h('dd', {}, s.ref)] : null,
          h('dt', {}, 'Length'), h('dd', {}, formatDistance(s.length_m)),
          h('dt', {}, 'Segments'), h('dd', {}, `${s.way_count} OSM ways`),
          lanes ? [h('dt', {}, 'Lanes'), h('dd', {}, lanes)] : null,
          speeds ? [h('dt', {}, 'Speed limit'), h('dd', {}, speeds)] : null,
          way?.surface ? [h('dt', {}, 'Surface'), h('dd', {}, capitalise(way.surface))] : null,
          way?.oneway ? [h('dt', {}, 'Direction'), h('dd', {}, 'One-way')] : null,
          bridges ? [h('dt', {}, 'Bridges'), h('dd', {}, `${bridges} segment${bridges > 1 ? 's' : ''}`)] : null,
          tunnels ? [h('dt', {}, 'Tunnels'), h('dd', {}, `${tunnels} segment${tunnels > 1 ? 's' : ''}`)] : null,
          s.areas.length ? [h('dt', {}, 'Within'), h('dd', {}, s.areas.map((a) => a.name).join(' · '))] : null,
        ),
        this.actions(s.center[0], s.center[1], way?.osm_url, s.name),
      ]);
    } catch (err) {
      this.failed(err);
    }
  }

  private roadFacts(p: Record<string, string | number | undefined>): Node {
    return h('dl', { class: 'kv' },
      p.ref ? [h('dt', {}, 'Route'), h('dd', {}, String(p.ref))] : null,
      p.lanes ? [h('dt', {}, 'Lanes'), h('dd', {}, String(p.lanes))] : null,
      p.maxspeed ? [h('dt', {}, 'Speed limit'), h('dd', {}, String(p.maxspeed))] : null,
      p.surface ? [h('dt', {}, 'Surface'), h('dd', {}, capitalise(String(p.surface)))] : null,
      p.oneway ? [h('dt', {}, 'Direction'), h('dd', {}, 'One-way')] : null,
      p.brunnel ? [h('dt', {}, 'Structure'), h('dd', {}, capitalise(String(p.brunnel)))] : null,
      p.service ? [h('dt', {}, 'Service'), h('dd', {}, capitalise(String(p.service)))] : null,
    );
  }

  // ---------------------------------------------------------------- bridges

  private async bridge(id: number, at: LonLat): Promise<void> {
    const signal = this.begin();
    this.loading();
    try {
      const b = await this.ctx.api.bridge(id, signal);
      if (signal.aborted) return;
      if (b.named_bridge_id) return this.namedBridge(b.named_bridge_id, false);
      if (b.geometry) this.select({ type: 'Feature', geometry: b.geometry, properties: { width: 7, colour: '#7048e8' } });
      this.render({ badge: kindBadge('bridge', '#7048e8', 40), title: b.name ?? BRIDGE_LABELS[b.kind] ?? 'Bridge', kicker: BRIDGE_LABELS[b.kind] }, [
        h('dl', { class: 'kv' },
          h('dt', {}, 'Length'), h('dd', {}, formatDistance(b.length_m)),
          b.class ? [h('dt', {}, 'Carries'), h('dd', {}, capitalise(b.class))] : null,
          b.structure ? [h('dt', {}, 'Structure'), h('dd', {}, capitalise(b.structure))] : null,
          b.lanes ? [h('dt', {}, 'Lanes'), h('dd', {}, String(b.lanes))] : null,
          b.layer !== undefined ? [h('dt', {}, 'Level'), h('dd', {}, String(b.layer))] : null,
        ),
        this.coordRow(at[0], at[1]),
        this.actions(b.lon, b.lat, b.osm_url, b.name),
        b.tags ? this.tagTable(b.tags) : null,
      ]);
    } catch (err) {
      this.failed(err);
    }
  }

  async namedBridge(id: number, fit = true): Promise<void> {
    const signal = this.begin();
    this.loading();
    try {
      const n = await this.ctx.api.namedBridge(id, signal);
      if (signal.aborted) return;
      if (n.geometry) this.select({ type: 'Feature', geometry: n.geometry, properties: { width: 7, colour: '#7048e8' } });
      if (fit) this.ctx.map.fitBounds(n.bbox, 16);
      setSelectionHash(`bridge:${n.id}`);
      const structures = [...new Set((n.ways ?? []).map((w) => w.structure).filter(Boolean))].join(', ');
      const lanes = [...new Set((n.ways ?? []).map((w) => w.lanes).filter(Boolean))].join(' / ');
      this.render({ badge: kindBadge('bridge', '#7048e8', 40), title: n.name, kicker: [BRIDGE_LABELS[n.kind], n.locality].filter(Boolean).join(' · ') }, [
        h('dl', { class: 'kv' },
          h('dt', {}, 'Span'), h('dd', {}, formatDistance(n.length_m)),
          n.total_length_m > n.length_m * 1.05 ? [h('dt', {}, 'All decks'), h('dd', {}, formatDistance(n.total_length_m))] : null,
          h('dt', {}, 'Segments'), h('dd', {}, `${n.way_count} OSM ways`),
          structures ? [h('dt', {}, 'Structure'), h('dd', {}, capitalise(structures))] : null,
          lanes ? [h('dt', {}, 'Lanes'), h('dd', {}, lanes)] : null,
        ),
        this.coordRow(n.center[0], n.center[1]),
        this.actions(n.center[0], n.center[1], n.ways?.[0]?.osm_url, n.name),
      ]);
    } catch (err) {
      this.failed(err);
    }
  }

  // ---------------------------------------------------------------- buildings

  private async building(id: number | undefined, p: Record<string, string | number | undefined>, at: LonLat): Promise<void> {
    if (id) {
      const signal = this.begin();
      this.loading();
      try {
        const b = await this.ctx.api.building(id, signal);
        if (signal.aborted) return;
        return this.buildingCard(b.id, b.name, b.kind, b.levels, b.height_m, b.footprint_m2, b.address, b.locality, [b.lon, b.lat], b.osm_url, b.tags);
      } catch (err) {
        if (!(err instanceof ApiError && err.status === 404)) return this.failed(err);
      }
    }
    const levels = p.levels !== undefined ? Number(p.levels) : undefined;
    const height = p.height !== undefined ? Number(p.height) : undefined;
    this.buildingCard(undefined, undefined, p.kind ? String(p.kind) : undefined, levels, height, undefined, undefined, undefined, at);
  }

  async buildingById(id: number, fly = true): Promise<void> {
    const signal = this.begin();
    this.loading();
    try {
      const b = await this.ctx.api.building(id, signal);
      if (signal.aborted) return;
      if (fly) this.ctx.map.flyTo([b.lon, b.lat], 16.5);
      this.buildingCard(b.id, b.name, b.kind, b.levels, b.height_m, b.footprint_m2, b.address, b.locality, [b.lon, b.lat], b.osm_url, b.tags);
    } catch (err) {
      this.failed(err);
    }
  }

  private buildingCard(id: number | undefined, name: string | undefined, kind: string | undefined, levels: number | undefined, height: number | undefined, footprint: number | undefined, address: string | undefined, locality: string | undefined, at: LonLat, osmUrl?: string, tags?: Record<string, string>): void {
    this.select({ type: 'Feature', geometry: { type: 'Point', coordinates: at }, properties: {} });
    if (id) setSelectionHash(`building:${id}`);
    this.render({ badge: kindBadge('building', '#7a6e64', 40), title: name ?? (kind ? `${capitalise(kind)} building` : 'Building'), kicker: [kind ? capitalise(kind) : 'Building', locality].filter(Boolean).join(' · ') }, [
      h('dl', { class: 'kv' },
        levels !== undefined ? [h('dt', {}, 'Floors'), h('dd', {}, String(levels))] : null,
        height !== undefined ? [h('dt', {}, 'Height'), h('dd', {}, `${height.toFixed(1)} m`)] : null,
        footprint ? [h('dt', {}, 'Footprint'), h('dd', {}, formatArea(footprint))] : null,
        levels && footprint ? [h('dt', {}, 'Floor area'), h('dd', {}, `≈ ${formatArea(levels * footprint)}`)] : null,
        address ? [h('dt', {}, 'Address'), h('dd', {}, address)] : null,
      ),
      levels === undefined && height === undefined ? h('p', { class: 'sub' }, 'No height data in OpenStreetMap yet — you can add building:levels on openstreetmap.org.') : null,
      this.coordRow(at[0], at[1]),
      this.actions(at[0], at[1], osmUrl, name),
      tags ? this.tagTable(tags) : null,
    ]);
  }

  // ---------------------------------------------------------------- transit

  private railway(p: Record<string, string | number | undefined>, at: LonLat): void {
    const refs = String(p.ref ?? '').split(',').filter(Boolean);
    const kind = String(p.kind ?? 'rail');
    const colour = String(p.colour ?? '#868e96');
    this.ctx.map.setPin(null);
    this.select(null);
    const network = p.network ? String(p.network) : undefined;
    const state = kind === 'construction' || kind === 'proposed' ? kind : undefined;
    const target = p.construction ?? p.proposed;
    this.render({ badge: kindBadge('line', colour, 40), title: String(p.name ?? (refs.length ? `${network ?? MODE_LABELS[kind]} · ${refs.join(', ')}` : MODE_LABELS[kind] ?? 'Railway')), kicker: [MODE_LABELS[kind], network].filter(Boolean).join(' · ') }, [
      refs.length
        ? this.row('train', 'Lines', h('span', {}, ...refs.map((ref) => {
          const key = `${network ? `${network}:` : ''}${ref}${state ? `:${state}` : ''}`;
          return h('button', { class: `line-chip${state ? ' planned' : ''}`, style: `--line:${colour}`, type: 'button', onclick: () => this.lineByKey(key) }, ref);
        })))
        : null,
      state ? this.row('info', 'Status', `${STATE_LABELS[state]}${target ? ` (${MODE_LABELS[String(target)] ?? capitalise(String(target))})` : ''}`) : null,
      p.brunnel ? this.row('bridge', 'Structure', capitalise(String(p.brunnel))) : null,
      p.service ? this.row('info', 'Track', capitalise(String(p.service))) : null,
      this.coordRow(at[0], at[1]),
    ]);
  }

  async lineByKey(key: string): Promise<void> {
    try {
      const lines = await this.ctx.api.transitLines();
      const base = key.replace(/:(construction|proposed)$/, '');
      const line = lines.items.find((l) => l.key === key) ?? lines.items.find((l) => l.key === base) ?? lines.items.find((l) => l.key.endsWith(`:${key}`));
      if (line) return this.route(line.routes[0].id);
      this.ctx.toast('Line details are not available');
    } catch (err) {
      this.failed(err);
    }
  }

  async route(id: number): Promise<void> {
    const signal = this.begin();
    this.loading();
    let r: TransitRoute;
    try {
      r = await this.ctx.api.transitRoute(id, signal);
    } catch (err) {
      return this.failed(err);
    }
    if (signal.aborted) return;
    const colour = r.colour ?? '#1c7ed6';
    const stops = r.stops ?? [];
    const features: GeoJSON.Feature[] = [];
    if (r.geometry) features.push({ type: 'Feature', geometry: r.geometry, properties: { colour, width: 6 } });
    for (const s of stops) features.push({ type: 'Feature', geometry: { type: 'Point', coordinates: [s.lon, s.lat] }, properties: { role: 'stop', name: s.name ?? '', colour } });
    this.ctx.map.setSelection({ type: 'FeatureCollection', features });
    this.ctx.map.fitBounds(r.bbox, 15);
    setSelectionHash(`route:${r.id}`);

    let others: TransitRoute[] = [];
    try {
      const lines = await this.ctx.api.transitLines({}, signal);
      others = lines.items.find((l) => l.key === r.line_key)?.routes.filter((x) => x.id !== r.id) ?? [];
    } catch {
      /* optional */
    }
    if (signal.aborted) return;
    const stopList = h('ol', { class: 'stops', style: `--line:${colour}` },
      ...stops.map((s) => h('li', { onclick: () => this.ctx.map.flyTo([s.lon, s.lat], 16) }, s.name ?? 'Unnamed stop')),
    );
    this.render({ badge: kindBadge('line', colour, 40), title: r.label, kicker: [r.network ?? MODE_LABELS[r.mode], MODE_LABELS[r.mode]].filter((v, i, a) => v && a.indexOf(v) === i).join(' · ') }, [
      h('dl', { class: 'kv' },
        r.from || r.to ? [h('dt', {}, 'Route'), h('dd', {}, `${r.from ?? '…'} → ${r.to ?? '…'}`)] : [h('dt', {}, 'Route'), h('dd', {}, r.name ?? r.label)],
        h('dt', {}, 'Length'), h('dd', {}, formatDistance(r.length_m)),
        h('dt', {}, 'Stops'), h('dd', {}, String(r.stop_count)),
        r.operator ? [h('dt', {}, 'Operator'), h('dd', {}, r.operator)] : null,
        r.state ? [h('dt', {}, 'Status'), h('dd', {}, STATE_LABELS[r.state] ?? capitalise(r.state))] : null,
      ),
      r.osm_type === 'way' ? h('p', { class: 'sub', style: 'white-space:normal;margin:4px 0 8px' }, 'Assembled from the mapped track: OpenStreetMap has no route relation for this line yet, so stations are matched along the track.') : null,
      others.length
        ? h('div', { class: 'btn-row', style: 'margin:6px 0 4px' }, ...others.slice(0, 6).map((o) => h('button', { class: 'btn', type: 'button', onclick: () => this.route(o.id) }, o.from && o.to ? `${o.from} → ${o.to}` : o.name ?? 'Variant')))
        : null,
      stops.length ? stopList : h('p', { class: 'sub' }, 'Stop sequence not mapped for this route.'),
      this.actions((r.bbox[0] + r.bbox[2]) / 2, (r.bbox[1] + r.bbox[3]) / 2, r.osm_url),
    ]);
  }

  /** What OpenStreetMap knows about a station: code, platforms, opening, operator, Wi-Fi and height above the sea. */
  private stationRows(d: PlaceDetail): HTMLElement[] {
    const t = d.tags;
    const facts: string[] = [];
    if (t.ref && /^[A-Z]{1,6}$/.test(t.ref)) facts.push(`Code ${t.ref}`);
    if (t.platforms && /^\d+$/.test(t.platforms)) facts.push(`${t.platforms} platform${t.platforms === '1' ? '' : 's'}`);
    const opened = t.opening_date ?? t.start_date;
    const openedOn = parseOsmDate(opened);
    if (opened && openedOn) facts.push(`${openedOn > new Date() ? 'Opening' : 'Opened'} ${formatOsmDate(opened)}`);
    const rows: HTMLElement[] = [];
    if (facts.length) rows.push(this.row('train', 'Station', facts.join(' · ')));
    const operator = OPERATORS[t.operator ?? ''] ?? t.operator;
    if (operator) rows.push(this.row('info', 'Operator', operator));
    const amenities = [t.internet_access === 'wlan' || t.internet_access === 'yes' ? 'Wi-Fi' : null, t.toilets === 'yes' ? 'Toilets' : null, t.air_conditioning === 'yes' ? 'Air-conditioned' : null].filter(Boolean);
    if (amenities.length) rows.push(this.row('info', 'Facilities', amenities.join(' · ')));
    if (t.ele && Number.isFinite(Number(t.ele))) rows.push(this.row('info', 'Elevation', `${Number(t.ele).toFixed(1)} m above sea level`));
    return rows;
  }

  private lineChips(lines: LineRef[]): Node {
    return h('span', {}, ...lines.map((l) => h('button', { class: `line-chip${l.state ? ' planned' : ''}`, type: 'button', style: `--line:${l.colour ?? '#868e96'}`, title: l.state ? `${l.label} (${STATE_LABELS[l.state]?.toLowerCase() ?? l.state})` : l.label, onclick: () => this.lineByKey(l.key) }, l.ref ?? l.label)));
  }

  // ---------------------------------------------------------------- curated layers

  /** A heritage site without a representing place (its feature was removed from OpenStreetMap). */
  private heritageSite(p: Record<string, string | number | undefined>, at: LonLat): void {
    const site = { status: String(p.status), year: p.year === undefined ? undefined : Number(p.year) } as HeritageSite;
    this.select({ type: 'Feature', geometry: { type: 'Point', coordinates: at }, properties: {} });
    this.render({ badge: kindBadge('building', '#c9a227', 40), title: String(p.name ?? 'Heritage site'), kicker: heritageStatus(site) }, [
      p.description ? h('p', { class: 'sub', style: 'white-space:normal' }, String(p.description)) : null,
      p.wikipedia ? this.row('globe', 'Wikipedia', wikipediaLink(String(p.wikipedia))) : null,
      this.coordRow(at[0], at[1]),
      this.actions(at[0], at[1], undefined, String(p.name ?? '')),
    ]);
  }

  /** A monsoon waterlogging hotspot. */
  private hotspot(p: Record<string, string | number | undefined>, at: LonLat): void {
    this.select({ type: 'Feature', geometry: { type: 'Point', coordinates: at }, properties: {} });
    this.render({ badge: kindBadge('locality', '#1c7ed6', 40), title: String(p.name ?? 'Waterlogging hotspot'), kicker: 'Monsoon waterlogging hotspot' }, [
      p.description ? h('p', { class: 'sub', style: 'white-space:normal' }, String(p.description)) : null,
      h('dl', { class: 'kv' },
        h('dt', {}, 'When'), h('dd', {}, 'Floods in heavy rain, June to September'),
        h('dt', {}, 'Warning radius'), h('dd', {}, formatDistance(Number(p.radius_m ?? 0))),
      ),
      h('p', { class: 'foot-note' }, 'Routes passing within the radius warn about it during the monsoon. The BMC publishes the full list of flood-prone spots every May.'),
      this.coordRow(at[0], at[1]),
    ]);
  }

  // ---------------------------------------------------------------- ward comparison

  /** The 24 BMC wards side by side: places, hospitals, schools, buildings and bridges, sortable by any column. */
  async compareWards(): Promise<void> {
    const signal = this.begin();
    this.loading();
    let wards: Area[];
    try {
      wards = (await this.ctx.api.areas({ kind: 'ward', stats: true }, signal)).items;
    } catch (err) {
      return this.failed(err);
    }
    if (signal.aborted) return;
    this.select(null);
    const sum = (a: Area, ...cats: string[]) => cats.reduce((n, c) => n + (a.stats?.[`places:${c}`] ?? 0), 0);
    const columns: { label: string; title: string; value: (a: Area) => number }[] = [
      { label: 'Places', title: 'Places of every kind', value: (a) => a.stats?.places ?? 0 },
      { label: 'Health', title: 'Hospitals and clinics', value: (a) => sum(a, 'hospital', 'clinic') },
      { label: 'Schools', title: 'Schools, colleges and universities', value: (a) => sum(a, 'school', 'college', 'university') },
      { label: 'Buildings', title: 'Building footprints', value: (a) => a.stats?.buildings ?? 0 },
      { label: 'km²', title: 'Area in square kilometres', value: (a) => a.area_km2 },
    ];
    let sortBy = 0;
    const tbody = h('tbody', {});
    const fill = () => {
      clear(tbody);
      const sorted = [...wards].sort((a, b) => columns[sortBy].value(b) - columns[sortBy].value(a));
      for (const a of sorted) {
        tbody.append(h('tr', { tabindex: 0, onclick: () => this.ctx.selectArea(a.id, true), onkeydown: (e: Event) => (e as KeyboardEvent).key === 'Enter' && this.ctx.selectArea(a.id, true) },
          h('th', { scope: 'row', title: a.description ?? a.name }, a.short_name ?? a.name),
          ...columns.map((c) => h('td', {}, formatNumber(Math.round(c.value(a))))),
        ));
      }
    };
    const head = h('tr', {}, h('th', { scope: 'col' }, 'Ward'), ...columns.map((c, i) => h('th', { scope: 'col' },
      h('button', { class: 'link-btn', type: 'button', title: `${c.title} — sort`, 'aria-pressed': String(i === sortBy), onclick: (e: Event) => {
        sortBy = i;
        (e.currentTarget as HTMLElement).closest('tr')!.querySelectorAll('button').forEach((b, j) => b.setAttribute('aria-pressed', String(j === i)));
        fill();
      } }, c.label))));
    fill();
    this.render({ badge: kindBadge('locality', 'var(--accent)', 40), title: 'BMC wards compared', kicker: '24 wards of the Brihanmumbai Municipal Corporation' }, [
      h('div', { class: 'table-scroll' }, h('table', { class: 'compare' }, h('thead', {}, head), tbody)),
      h('p', { class: 'foot-note' }, 'Counts are what OpenStreetMap and Overture Maps have mapped, not official registers. Choose a ward to show it on the map.'),
    ]);
  }

  // ---------------------------------------------------------------- PIN codes

  /** A PIN code area: its approximate outline (from addressed places), localities and what is in it. */
  async pinCode(pin: string): Promise<void> {
    const signal = this.begin();
    this.loading();
    try {
      const a = await this.ctx.api.area(pin, signal);
      if (signal.aborted) return;
      if (a.geometry) this.select({ type: 'Feature', geometry: a.geometry, properties: { colour: '#e8590c', width: 3 } });
      this.ctx.map.fitBounds(a.bbox, 15);
      setSelectionHash(`pincode:${pin}`);
      const stats = a.stats ?? {};
      const top = Object.entries(stats)
        .filter(([k]) => k.startsWith('places:'))
        .map(([k, v]) => [k.slice(7), v] as [string, number])
        .sort((x, y) => y[1] - x[1])
        .slice(0, 6);
      this.render({ badge: kindBadge('locality', '#e8590c', 40), title: `PIN ${pin}`, kicker: ['PIN code', a.description].filter(Boolean).join(' · ') }, [
        h('dl', { class: 'kv' },
          h('dt', {}, 'Places'), h('dd', {}, formatNumber(stats.places ?? 0)),
          h('dt', {}, 'Area'), h('dd', {}, `≈ ${a.area_km2.toLocaleString('en-IN')} km²`),
          ...top.map(([cat, n]) => [h('dt', {}, categoryById.get(cat)?.label ?? capitalise(cat)), h('dd', {}, formatNumber(n))]),
        ),
        h('p', { class: 'sub', style: 'white-space:normal;margin:6px 0' }, 'The outline is drawn around the places whose address has this PIN; postal delivery boundaries are not published as open data.'),
        h('div', { class: 'btn-row', style: 'margin:10px 0 4px' },
          h('button', { class: 'btn btn-primary', type: 'button', onclick: () => this.ctx.selectArea(pin, false) }, icon('explore', 15), 'Use as area'),
          h('button', { class: 'btn', type: 'button', onclick: () => this.copy(location.href, 'Link copied') }, icon('link', 15), 'Share'),
        ),
      ]);
    } catch (err) {
      this.failed(err);
    }
  }

  // ---------------------------------------------------------------- localities & pins

  private async locality(name: string, kind: string, at: LonLat): Promise<void> {
    const signal = this.begin();
    this.select(null);
    this.ctx.map.setPin(null);
    this.render({ badge: kindBadge('locality', 'var(--accent)', 40), title: name || capitalise(kind), kicker: capitalise(kind) }, [h('div', { class: 'skeleton' }), h('div', { class: 'skeleton' })]);
    try {
      const [rev, nearby] = await Promise.all([
        this.ctx.api.reverse(at[0], at[1], signal),
        this.ctx.api.nearby(at[0], at[1], { radius: 1200, named: true, limit: 8, category: ['railway_station', 'metro_station', 'hospital', 'college', 'school', 'park', 'bank', 'police'] }, signal),
      ]);
      if (signal.aborted) return;
      this.render({ badge: kindBadge('locality', 'var(--accent)', 40), title: name || capitalise(kind), kicker: [capitalise(kind), rev.areas.at(-1)?.name].filter(Boolean).join(' · ') }, [
        rev.areas.length ? this.row('explore', 'Within', rev.areas.map((a) => a.name).join(' · ')) : null,
        this.coordRow(at[0], at[1]),
        nearby.items.length ? h('div', { class: 'section', style: 'margin-top:12px' }, h('h3', {}, 'Key places nearby'), this.placeList(nearby.items)) : null,
      ]);
    } catch (err) {
      this.failed(err);
    }
  }

  /** "What's here?" — reverse geocode a coordinate and drop a draggable pin. */
  async reverse(at: LonLat, opts: { title?: string } = {}): Promise<void> {
    const signal = this.begin();
    this.select(null);
    this.ctx.map.setPin(at, (moved) => this.reverse(moved, opts));
    setSelectionHash(`pin:${at[1].toFixed(6)},${at[0].toFixed(6)}`);
    this.render({ badge: kindBadge('locality', '#e8590c', 40), title: opts.title ?? 'Dropped pin', kicker: formatCoord(at[0], at[1], 5) }, [h('div', { class: 'skeleton' }), h('div', { class: 'skeleton' })]);
    try {
      const r = await this.ctx.api.reverse(at[0], at[1], signal);
      if (signal.aborted) return;
      this.render({ badge: kindBadge('locality', '#e8590c', 40), title: opts.title ?? (r.label.split(',')[0] || 'Dropped pin'), kicker: r.label }, [
        r.road ? this.row('route', 'Nearest road', h('span', {}, h('b', {}, r.road.name ?? r.road.ref ?? 'Unnamed road'), ` · ${ROAD_LABELS[r.road.kind] ?? r.road.kind} · ${formatDistance(r.road.distance_m)} away`)) : null,
        r.locality ? this.row('pin', 'Locality', `${r.locality.name} (${r.locality.kind})`) : null,
        r.postcode ? this.row('pin', 'PIN code', h('button', { class: 'link-btn', type: 'button', onclick: () => this.pinCode(r.postcode!) }, r.postcode)) : null,
        r.areas.length ? this.row('explore', 'Within', r.areas.map((a) => a.name).join(' · ')) : null,
        this.coordRow(at[0], at[1]),
        this.actions(at[0], at[1]),
        r.nearby.length ? h('div', { class: 'section', style: 'margin-top:12px' }, h('h3', {}, 'Right here'), this.placeList(r.nearby)) : null,
      ]);
    } catch (err) {
      this.failed(err);
    }
  }

  // ---------------------------------------------------------------- rendering helpers

  placeList(items: { id: number; name?: string; category: string; distance_m?: number; locality?: string }[]): HTMLElement {
    return h('ul', { class: 'list' }, ...items.map((p) => h('li', {},
      h('button', { class: 'list-item', type: 'button', onclick: () => this.place(p.id, true) },
        categoryBadge(p.category, 28),
        h('span', { class: 'grow' }, h('div', { class: 'title' }, p.name ?? categoryById.get(p.category)?.label ?? 'Place'), h('div', { class: 'sub' }, [categoryById.get(p.category)?.label, p.locality].filter(Boolean).join(' · '))),
        p.distance_m !== undefined ? h('span', { class: 'meta' }, formatDistance(p.distance_m)) : null,
      ),
    )));
  }

  private row(iconName: string, label: string, ...value: (Node | string | null)[]): HTMLElement {
    const v = h('div', { class: 'v' });
    for (const x of value) if (x !== null) v.append(x);
    return h('div', { class: 'row' }, icon(iconName, 16), h('div', { class: 'k' }, label), v);
  }

  private coordRow(lon: number, lat: number): HTMLElement {
    const text = formatCoord(lon, lat);
    return this.row('pin', t('card.coordinates'),
      h('span', { class: 'mono', title: formatDMS(lon, lat) }, text),
      h('button', { class: 'link-btn', type: 'button', style: 'margin-left:8px', onclick: () => this.copy(text, 'Coordinates copied') }, t('card.copy')),
    );
  }

  private actions(lon: number, lat: number, osmUrl?: string, name?: string): HTMLElement {
    const dest = `${lat.toFixed(6)},${lon.toFixed(6)}`;
    return h('div', { class: 'btn-row', style: 'margin:12px 0 4px' },
      h('button', { class: 'btn btn-primary', type: 'button', title: 'Distance and route by road to here', onclick: () => this.ctx.routeTo({ lon, lat, label: name ?? dest }) }, icon('directions', 15), t('card.routeHere')),
      h('button', { class: 'btn', type: 'button', onclick: () => this.copy(location.href, 'Link copied') }, icon('link', 15), t('card.share')),
      h('button', { class: 'btn', type: 'button', onclick: () => this.nearbyFrom(lon, lat, name) }, icon('search', 15), t('card.nearby')),
      osmUrl ? h('a', { class: 'btn', href: osmUrl, target: '_blank', rel: 'noopener', title: 'View or edit on OpenStreetMap' }, icon('external', 15), 'OSM') : null,
    );
  }

  private async nearbyFrom(lon: number, lat: number, name?: string): Promise<void> {
    const signal = this.begin();
    try {
      const res = await this.ctx.api.nearby(lon, lat, { radius: 600, named: true, limit: 25 }, signal);
      if (signal.aborted) return;
      this.render({ badge: kindBadge('locality', 'var(--accent)', 40), title: 'Nearby', kicker: name ? `Within 600 m of ${name}` : 'Within 600 m' }, [
        res.items.length ? this.placeList(res.items) : h('p', { class: 'empty-state' }, 'Nothing mapped nearby.'),
      ]);
    } catch (err) {
      this.failed(err);
    }
  }

  private tagTable(tags: Record<string, string>, title = 'All OpenStreetMap tags'): HTMLElement | null {
    const entries = Object.entries(tags).sort(([a], [b]) => a.localeCompare(b));
    if (!entries.length) return null;
    return h('details', { class: 'tags' },
      h('summary', {}, `${title} (${formatNumber(entries.length)})`),
      h('table', {}, ...entries.map(([k, v]) => h('tr', {}, h('td', {}, k), h('td', {}, String(v))))),
    );
  }

  private render(head: Head, body: (Node | null)[], accent?: string): void {
    clear(this.el);
    const title = h('h2', { id: 'insp-title' }, head.title);
    this.el.setAttribute('aria-labelledby', 'insp-title');
    this.el.append(
      h('header', { class: 'insp-head', style: accent ? `border-top:3px solid ${accent}` : '' },
        head.badge,
        h('div', { class: 'grow' }, title, head.kicker ? h('div', { class: 'kicker' }, head.kicker) : null),
        h('button', { class: 'insp-close', type: 'button', 'aria-label': 'Close', onclick: () => this.close() }, icon('close', 16)),
      ),
      h('div', { class: 'insp-body' }, ...body.filter((n): n is Node => n !== null)),
    );
    this.el.hidden = false;
  }

  private loading(): void {
    clear(this.el);
    this.el.append(h('div', { class: 'insp-body', style: 'padding-top:18px' }, h('div', { class: 'skeleton', style: 'width:60%;height:20px' }), h('div', { class: 'skeleton' }), h('div', { class: 'skeleton' }), h('div', { class: 'skeleton', style: 'width:80%' })));
    this.el.hidden = false;
  }

  private failed(err: unknown): void {
    if (err instanceof DOMException && err.name === 'AbortError') return;
    const message = err instanceof ApiError && err.status === 404 ? 'No details are available for this feature.' : 'Could not load details. Is the BombayChitra server running?';
    clear(this.el);
    this.el.append(
      h('header', { class: 'insp-head' }, h('div', { class: 'grow' }, h('h2', {}, 'Not available')), h('button', { class: 'insp-close', type: 'button', 'aria-label': 'Close', onclick: () => this.close() }, icon('close', 16))),
      h('div', { class: 'insp-body' }, h('p', { class: 'error-state' }, message)),
    );
    this.el.hidden = false;
  }

  private begin(): AbortSignal {
    this.abort?.abort();
    this.abort = new AbortController();
    return this.abort.signal;
  }

  private select(feature: GeoJSON.Feature | null): void {
    this.ctx.map.setSelection(feature ? { type: 'FeatureCollection', features: [feature] } : null);
    if (feature) this.ctx.map.setPin(null);
  }

  private async copy(text: string, done: string): Promise<void> {
    try {
      await navigator.clipboard.writeText(text);
      this.ctx.toast(done);
    } catch {
      this.ctx.toast(text);
    }
  }
}

// ---------------------------------------------------------------- URL selection state

/** Stores the open card in the URL hash (alongside MapLibre's `map=` param) so links reopen it. */
export function setSelectionHash(value: string | null): void {
  updateHash({ sel: value });
}

const STATION_CATEGORIES = new Set(['railway_station', 'metro_station', 'monorail_station']);
/** Operator codes used on station tags. */
const OPERATORS: Record<string, string> = { CR: 'Central Railway', WR: 'Western Railway', MMRDA: 'Mumbai Metropolitan Region Development Authority', MMRCL: 'Mumbai Metro Rail Corporation', MMOPL: 'Mumbai Metro One', CIDCO: 'CIDCO' };

const HERITAGE_LABELS: Record<string, string> = { unesco: 'UNESCO World Heritage Site', precinct: 'Heritage precinct', monument: 'Listed monument' };

function heritageStatus(site: Pick<HeritageSite, 'status' | 'year'>): string {
  const label = HERITAGE_LABELS[site.status] ?? 'Heritage site';
  return site.status === 'unesco' && site.year ? `${label} (${site.year})` : label;
}

/** A link to a Wikipedia article given as `lang:Title`. */
function wikipediaLink(ref: string): HTMLElement {
  const [lang, ...title] = ref.split(':');
  const url = `https://${lang}.wikipedia.org/wiki/${encodeURIComponent(title.join(':').replace(/ /g, '_'))}`;
  return h('a', { href: url, target: '_blank', rel: 'noopener' }, 'Wikipedia');
}

function safeUrl(url: string): string {
  const u = url.trim();
  return /^https?:\/\//i.test(u) ? u : `https://${u}`;
}

function prettyUrl(url: string): string {
  return url.replace(/^https?:\/\/(www\.)?/i, '').replace(/\/$/, '');
}
