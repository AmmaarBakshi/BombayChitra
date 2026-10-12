import type { RouteResult, RouteStep, SearchResult, TravelMode } from '../../api/types.ts';
import type { AppContext } from '../context.ts';
import { append, clear, debounce, h, storageGet, storageSet } from '../dom.ts';
import { formatDistance, formatDuration } from '../format.ts';
import { formatRoute, hashParams, parseRoute, updateHash } from '../hash.ts';
import { icon } from '../icons.ts';
import type { Panel } from './panel.ts';

export type RouteEndName = 'from' | 'to';

export interface RoutePoint {
  lon: number;
  lat: number;
  label: string;
}

const COLOUR = '#e8590c';
const MODE_KEY = 'bc-route-mode';
const MODES: { id: TravelMode; label: string; icon: string; verb: string }[] = [
  { id: 'car', label: 'Car', icon: 'car', verb: 'drive' },
  { id: 'two_wheeler', label: 'Two-wheeler', icon: 'scooter', verb: 'ride' },
  { id: 'foot', label: 'Walk', icon: 'walk', verb: 'walk' },
];
/** Rotation of the direction arrow for each step modifier (degrees, clockwise). */
const ARROW: Record<string, number> = { straight: 0, slight_right: 40, right: 90, sharp_right: 135, uturn: 180, sharp_left: -135, left: -90, slight_left: -40 };
const COORD = /^\s*(-?\d{1,2}(?:\.\d+)?)\s*,\s*(-?\d{1,3}(?:\.\d+)?)\s*$/;
const LABEL: Record<RouteEndName, string> = { from: 'From', to: 'To' };

/**
 * "Distance by road": the driving distance and time between two places over the road network, with the route drawn on
 * the map. Ends are chosen by search, by clicking the map or from any place card ("Route here").
 */
export class DistancePanel implements Panel {
  readonly id = 'distance';
  readonly label = 'Distance';
  readonly icon = 'route';

  /** The end being chosen by clicking the map, if any. */
  picking: RouteEndName | null = null;
  onPickingChange: (end: RouteEndName | null) => void = () => {};

  private points: Record<RouteEndName, RoutePoint | null> = { from: null, to: null };
  private optimize: 'time' | 'distance' = 'time';
  private mode: TravelMode = (MODES.find((m) => m.id === storageGet(MODE_KEY))?.id) ?? 'car';
  private avoidTolls = false;
  private readonly travelButtons: HTMLButtonElement[] = [];
  private readonly tollToggle = h('input', { type: 'checkbox' }) as HTMLInputElement;
  private readonly tollRow = h('label', { class: 'switch-row compact' }, this.tollToggle, h('span', { class: 'switch', 'aria-hidden': 'true' }), h('span', { class: 'grow' }, 'Avoid tolls'));
  private readonly inputs: Record<RouteEndName, HTMLInputElement>;
  private readonly menus: Record<RouteEndName, HTMLUListElement>;
  private readonly pickButtons: Record<RouteEndName, HTMLButtonElement>;
  private readonly result = h('div', { class: 'route-result', 'aria-live': 'polite' });
  private readonly modeButtons: HTMLButtonElement[] = [];
  private abort: AbortController | null = null;
  private suggestAbort: AbortController | null = null;
  private rendered = false;

  constructor(private readonly ctx: AppContext) {
    const field = (end: RouteEndName) => {
      const input = h('input', { type: 'search', autocomplete: 'off', spellcheck: 'false', placeholder: end === 'from' ? 'Start — search or pick on the map' : 'Destination', 'aria-label': `${LABEL[end]} place` });
      const menu = h('ul', { class: 'search-results route-menu', role: 'listbox', hidden: true });
      const pick = h('button', { class: 'icon-btn', type: 'button', title: `Choose ${LABEL[end].toLowerCase()} on the map`, 'aria-pressed': 'false', onclick: () => this.startPick(this.picking === end ? null : end) }, icon('crosshair', 17));
      const suggest = debounce(() => void this.suggest(end), 180);
      input.addEventListener('input', () => {
        this.points[end] = null;
        suggest();
      });
      input.addEventListener('keydown', (e) => {
        if (e.key === 'Enter') {
          e.preventDefault();
          (menu.querySelector('li[data-index]') as HTMLElement | null)?.click();
        } else if (e.key === 'Escape') {
          menu.hidden = true;
        }
      });
      input.addEventListener('blur', () => setTimeout(() => (menu.hidden = true), 150));
      return { input, menu, pick };
    };
    const from = field('from');
    const to = field('to');
    this.inputs = { from: from.input, to: to.input };
    this.menus = { from: from.menu, to: to.menu };
    this.pickButtons = { from: from.pick, to: to.pick };
    document.addEventListener('keydown', (e) => e.key === 'Escape' && this.picking && this.startPick(null));
  }

  render(body: HTMLElement): void {
    if (this.rendered) return;
    this.rendered = true;
    const row = (end: RouteEndName) => h('div', { class: 'route-field' },
      h('span', { class: `route-dot ${end}`, 'aria-hidden': 'true' }),
      h('div', { class: 'route-input' }, this.inputs[end], this.menus[end]),
      this.pickButtons[end],
    );
    const modes: { id: 'time' | 'distance'; label: string }[] = [{ id: 'time', label: 'Fastest' }, { id: 'distance', label: 'Shortest' }];
    for (const m of modes) {
      this.modeButtons.push(h('button', { type: 'button', 'aria-pressed': String(m.id === this.optimize), 'data-mode': m.id, onclick: () => this.setOptimize(m.id) }, m.label));
    }
    for (const m of MODES) {
      this.travelButtons.push(h('button', { type: 'button', 'aria-pressed': String(m.id === this.mode), 'data-travel': m.id, title: m.label, onclick: () => this.setMode(m.id) }, icon(m.icon, 16), h('span', {}, m.label)));
    }
    this.tollToggle.addEventListener('change', () => {
      this.avoidTolls = this.tollToggle.checked;
      if (this.points.from && this.points.to) void this.compute();
    });
    this.tollRow.hidden = this.mode === 'foot';
    body.append(
      h('div', { class: 'route-head' },
        h('div', { class: 'title' }, 'Distance by road'),
        h('div', { class: 'sub', style: 'white-space:normal' }, 'Distance, time and turn-by-turn directions between two places by car, two-wheeler or on foot, with the route on the map.'),
      ),
      h('div', { class: 'segmented travel-modes', role: 'group', 'aria-label': 'Travel by' }, ...this.travelButtons),
      h('div', { class: 'route-form' }, row('from'), row('to')),
      h('div', { class: 'route-tools' },
        h('div', { class: 'segmented', role: 'group', 'aria-label': 'Route' }, ...this.modeButtons),
        h('button', { class: 'btn', type: 'button', title: 'Swap start and destination', onclick: () => this.swap() }, icon('swap', 15), 'Swap'),
        'geolocation' in navigator ? h('button', { class: 'btn', type: 'button', title: 'Start from my location', onclick: () => this.useMyLocation() }, icon('locate', 15), 'Me') : null,
      ),
      this.tollRow,
      this.result,
    );
    this.renderEmpty();
  }

  /** Sets one end of the route (e.g. "Route here" on a place card) and computes when both ends are known. */
  setPoint(end: RouteEndName, point: RoutePoint | null): void {
    this.points[end] = point;
    this.inputs[end].value = point?.label ?? '';
    this.menus[end].hidden = true;
    if (this.picking === end) this.startPick(null);
    if (this.points.from && this.points.to) void this.compute();
    else {
      this.drawEnds();
      this.renderEmpty();
    }
  }

  get missing(): RouteEndName | null {
    return !this.points.from ? 'from' : !this.points.to ? 'to' : null;
  }

  startPick(end: RouteEndName | null): void {
    this.picking = end;
    for (const e of ['from', 'to'] as const) this.pickButtons[e].setAttribute('aria-pressed', String(e === end));
    this.ctx.map.map.getCanvas().style.cursor = end ? 'crosshair' : '';
    this.onPickingChange(end);
    if (end) this.ctx.toast(`Click the map to set the ${end === 'from' ? 'start' : 'destination'}`);
  }

  /** A map click while picking. */
  async pick(lngLat: [number, number]): Promise<void> {
    const end = this.picking;
    if (!end) return;
    this.startPick(null);
    this.setPoint(end, { lon: lngLat[0], lat: lngLat[1], label: `${lngLat[1].toFixed(5)}, ${lngLat[0].toFixed(5)}` });
    await this.nameEnd(end);
  }

  /** Restores a route from the URL (`#dist=lat,lon~lat,lon`). Returns true when one was found. */
  restore(): boolean {
    const params = hashParams();
    const route = parseRoute(params.get('dist'));
    if (!route) return false;
    if (params.get('opt') === 'distance') this.optimize = 'distance';
    const travel = MODES.find((m) => m.id === params.get('mode'));
    if (travel) this.mode = travel.id;
    this.avoidTolls = params.get('avoid') === 'toll';
    this.tollToggle.checked = this.avoidTolls;
    this.tollRow.hidden = this.mode === 'foot';
    const [a, b] = [route.from, route.to];
    this.modeButtons.forEach((btn) => btn.setAttribute('aria-pressed', String(btn.dataset.mode === this.optimize)));
    this.travelButtons.forEach((btn) => btn.setAttribute('aria-pressed', String(btn.dataset.travel === this.mode)));
    this.points.from = { lat: a[0], lon: a[1], label: `${a[0].toFixed(5)}, ${a[1].toFixed(5)}` };
    this.setPoint('to', { lat: b[0], lon: b[1], label: `${b[0].toFixed(5)}, ${b[1].toFixed(5)}` });
    this.inputs.from.value = this.points.from.label;
    void this.nameEnd('from');
    void this.nameEnd('to');
    return true;
  }

  /** Replaces an end's coordinate label with the nearest road and locality. */
  private async nameEnd(end: RouteEndName): Promise<void> {
    const p = this.points[end];
    if (!p) return;
    try {
      const r = await this.ctx.api.reverse(p.lon, p.lat);
      const label = [r.road?.name ?? r.road?.ref, r.locality?.name].filter(Boolean).join(', ') || r.label;
      if (label && this.points[end] === p) {
        p.label = label;
        this.inputs[end].value = label;
        const cell = this.result.querySelector(`[data-end="${end}"]`);
        if (cell) cell.firstChild!.textContent = label;
      }
    } catch {
      /* keep the coordinates */
    }
  }

  // ---------------------------------------------------------------- internals

  private setMode(mode: TravelMode): void {
    this.mode = mode;
    storageSet(MODE_KEY, mode);
    this.travelButtons.forEach((btn) => btn.setAttribute('aria-pressed', String(btn.dataset.travel === mode)));
    this.tollRow.hidden = mode === 'foot';
    if (this.points.from && this.points.to) void this.compute();
  }

  private setOptimize(mode: 'time' | 'distance'): void {
    this.optimize = mode;
    this.modeButtons.forEach((btn) => btn.setAttribute('aria-pressed', String(btn.dataset.mode === mode)));
    if (this.points.from && this.points.to) void this.compute();
  }

  private swap(): void {
    const { from, to } = this.points;
    this.points = { from: to, to: from };
    this.inputs.from.value = to?.label ?? '';
    this.inputs.to.value = from?.label ?? '';
    if (this.points.from && this.points.to) void this.compute();
  }

  private useMyLocation(): void {
    navigator.geolocation.getCurrentPosition(
      (pos) => this.setPoint('from', { lon: pos.coords.longitude, lat: pos.coords.latitude, label: 'My location' }),
      () => this.ctx.toast('Location unavailable'),
      { enableHighAccuracy: true, timeout: 10000 },
    );
  }

  private async suggest(end: RouteEndName): Promise<void> {
    const q = this.inputs[end].value.trim();
    const menu = this.menus[end];
    this.suggestAbort?.abort();
    clear(menu);
    if (q.length < 2) {
      menu.hidden = true;
      return;
    }
    const coord = COORD.exec(q);
    if (coord) {
      const lat = Number(coord[1]);
      const lon = Number(coord[2]);
      menu.append(this.option(end, 0, { lon, lat, label: `${lat.toFixed(5)}, ${lon.toFixed(5)}` }, 'Coordinates'));
      menu.hidden = false;
      return;
    }
    this.suggestAbort = new AbortController();
    try {
      const c = this.ctx.map.map.getCenter();
      const res = await this.ctx.api.search(q, { limit: 6, near: [c.lng, c.lat], kind: ['place', 'locality', 'street', 'line', 'area', 'bridge'] }, this.suggestAbort.signal);
      clear(menu);
      res.items.forEach((r: SearchResult, i) => menu.append(this.option(end, i, { lon: r.lon, lat: r.lat, label: r.name }, r.detail)));
      if (!res.items.length) menu.append(h('li', { class: 'empty' }, 'No matches — try another name or click the map'));
      menu.hidden = document.activeElement !== this.inputs[end];
    } catch (err) {
      if ((err as Error).name !== 'AbortError') menu.hidden = true;
    }
  }

  private option(end: RouteEndName, index: number, point: RoutePoint, detail?: string): HTMLLIElement {
    return h('li', { role: 'option', 'data-index': String(index), onmousedown: (e: Event) => e.preventDefault(), onclick: () => this.setPoint(end, point) },
      h('span', { class: 'grow' }, h('div', { class: 'title' }, point.label), detail ? h('div', { class: 'sub' }, detail) : null),
    );
  }

  private async compute(): Promise<void> {
    const { from, to } = this.points;
    if (!from || !to) return;
    this.abort?.abort();
    this.abort = new AbortController();
    this.writeHash();
    clear(this.result);
    this.result.append(h('div', { class: 'skeleton', style: 'height:64px' }), h('div', { class: 'skeleton' }));
    try {
      const avoid: 'toll'[] = this.avoidTolls && this.mode !== 'foot' ? ['toll'] : [];
      const res = await this.ctx.api.route([from.lon, from.lat], [to.lon, to.lat], { mode: this.mode, optimize: this.optimize, avoid }, this.abort.signal);
      this.show(res);
    } catch (err) {
      if ((err as Error).name === 'AbortError') return;
      clear(this.result);
      this.drawEnds();
      const message = (err as Error).message || 'could not find a road route';
      this.result.append(h('p', { class: 'error-state' }, message.charAt(0).toUpperCase() + message.slice(1) + '.'));
    }
  }

  private show(res: RouteResult): void {
    const { from, to } = this.points;
    this.ctx.inspector.close();
    this.ctx.map.setSelection({
      type: 'FeatureCollection',
      features: [
        { type: 'Feature', geometry: res.geometry, properties: { colour: COLOUR, width: 6 } },
        ...this.endFeatures(),
      ],
    });
    this.ctx.map.fitBounds(res.bbox, 16);

    const detour = res.straight_m > 0 ? res.distance_m / res.straight_m : 1;
    const named = new Map<string, number>();
    for (const r of res.roads) if (r.name) named.set(r.name, (named.get(r.name) ?? 0) + r.distance_m);
    const main = [...named].sort((a, b) => b[1] - a[1]).slice(0, 6);
    const verb = MODES.find((m) => m.id === res.mode)?.verb ?? 'drive';
    clear(this.result);
    append(this.result, [
      h('div', { class: 'route-summary' },
        h('div', {}, h('b', { class: 'route-big' }, formatDistance(res.distance_m)), h('span', { class: 'sub' }, res.mode === 'foot' ? 'on foot' : 'by road')),
        h('div', {}, h('b', { class: 'route-big' }, formatDuration(res.duration_s)), h('span', { class: 'sub' }, res.optimize === 'time' ? `fastest ${verb}` : 'on the shortest route')),
      ),
      res.toll_m > 0
        ? h('div', { class: 'route-notice' }, icon('toll', 16), h('span', { class: 'grow' }, `Includes ${formatDistance(res.toll_m)} of toll road`),
          h('button', { class: 'link-btn', type: 'button', onclick: () => { this.tollToggle.checked = true; this.tollToggle.dispatchEvent(new Event('change')); } }, 'Avoid tolls'))
        : null,
      h('dl', { class: 'kv' },
        h('dt', {}, 'Straight line'), h('dd', {}, `${formatDistance(res.straight_m)} · road is ${detour.toFixed(2)}× longer`),
        h('dt', {}, 'From'), h('dd', { 'data-end': 'from' }, from?.label ?? '', res.from.snapped.offset_m > 30 ? h('span', { class: 'sub' }, ` (joins ${res.from.snapped.road ?? 'the road'} ${formatDistance(res.from.snapped.offset_m)} away)`) : null),
        h('dt', {}, 'To'), h('dd', { 'data-end': 'to' }, to?.label ?? '', res.to.snapped.offset_m > 30 ? h('span', { class: 'sub' }, ` (from ${res.to.snapped.road ?? 'the road'} ${formatDistance(res.to.snapped.offset_m)} away)`) : null),
      ),
      main.length
        ? h('div', { class: 'route-roads' }, h('h3', {}, 'Main roads'), h('ol', {}, ...main.map(([name, m]) => h('li', {}, h('span', { class: 'grow' }, name), h('span', { class: 'mono' }, formatDistance(m))))))
        : null,
      res.steps?.length ? this.stepList(res.steps) : null,
      h('div', { class: 'btn-row', style: 'margin-top:10px' },
        h('button', { class: 'btn', type: 'button', onclick: () => this.ctx.map.fitBounds(res.bbox, 16) }, icon('route', 15), 'Show route'),
        h('button', { class: 'btn', type: 'button', onclick: () => this.share() }, icon('link', 15), 'Share'),
        h('button', { class: 'btn', type: 'button', onclick: () => this.reset() }, icon('close', 15), 'Clear'),
      ),
      h('p', { class: 'foot-note' }, res.mode === 'foot'
        ? 'Walking at 4.8 km/h along roads, footpaths, steps and foot over-bridges; one-way rules do not apply. Expressways, sea links and freeways are closed to pedestrians.'
        : `Estimated from road types and one-way rules, without live traffic; peak-hour journeys take longer. Turn restrictions are not considered.${res.mode === 'two_wheeler' ? ' Two-wheelers are not allowed on expressways, sea links and freeways.' : ''}`),
    ]);
  }

  /** Turn-by-turn directions; choosing a step shows where it happens. */
  private stepList(steps: RouteStep[]): HTMLElement {
    const list = h('ol', { class: 'route-steps' }, ...steps.map((step) => {
      const glyph = step.type === 'arrive' ? icon('flag', 16) : icon('arrow', 16);
      if (step.type !== 'arrive') glyph.style.transform = `rotate(${step.type === 'depart' ? 0 : ARROW[step.modifier ?? 'straight'] ?? 0}deg)`;
      return h('li', {},
        h('button', { type: 'button', class: 'route-step', onclick: () => this.ctx.map.flyTo(step.location, 17) },
          h('span', { class: 'route-step-icon', 'aria-hidden': 'true' }, glyph),
          h('span', { class: 'grow' }, step.instruction),
          step.distance_m > 0 ? h('span', { class: 'mono meta' }, formatDistance(step.distance_m)) : null,
        ),
      );
    }));
    return h('details', { class: 'route-directions', open: steps.length <= 12 },
      h('summary', {}, `Directions · ${steps.length - 1} step${steps.length === 2 ? '' : 's'}`),
      list,
    );
  }

  private endFeatures(): GeoJSON.Feature[] {
    return (['from', 'to'] as const).flatMap((end) => {
      const p = this.points[end];
      return p ? [{ type: 'Feature' as const, geometry: { type: 'Point' as const, coordinates: [p.lon, p.lat] }, properties: { role: 'stop', colour: end === 'from' ? '#2b8a3e' : COLOUR } }] : [];
    });
  }

  private drawEnds(): void {
    const features = this.endFeatures();
    this.ctx.map.setSelection(features.length ? { type: 'FeatureCollection', features } : null);
  }

  private renderEmpty(): void {
    clear(this.result);
    this.result.append(h('p', { class: 'empty-state', style: 'white-space:normal' },
      this.points.from || this.points.to ? `Now choose the ${this.missing === 'from' ? 'start' : 'destination'}.` : 'Search for two places, or use the target buttons to pick them on the map. Tip: “Route here” on any place card sets the destination.'));
  }

  private reset(): void {
    this.abort?.abort();
    this.points = { from: null, to: null };
    this.inputs.from.value = '';
    this.inputs.to.value = '';
    this.ctx.map.setSelection(null);
    this.writeHash(true);
    this.renderEmpty();
  }

  private writeHash(remove = false): void {
    const { from, to } = this.points;
    if (remove || !from || !to) {
      updateHash({ dist: null, opt: null, mode: null, avoid: null });
    } else {
      updateHash({
        dist: formatRoute([from.lat, from.lon], [to.lat, to.lon]),
        opt: this.optimize === 'distance' ? 'distance' : null,
        mode: this.mode === 'car' ? null : this.mode,
        avoid: this.avoidTolls && this.mode !== 'foot' ? 'toll' : null,
      });
    }
  }

  private share(): void {
    void navigator.clipboard?.writeText(location.href).then(() => this.ctx.toast('Route link copied'), () => this.ctx.toast(location.href));
  }
}
