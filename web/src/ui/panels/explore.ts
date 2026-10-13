import type { Place } from '../../api/types.ts';
import { categories } from '../../taxonomy.ts';
import type { AppContext } from '../context.ts';
import { clear, h } from '../dom.ts';
import { formatDate, formatDistance, formatNumber } from '../format.ts';
import { categoryBadge } from '../icons.ts';
import type { Panel } from './panel.ts';
import type { LayersPanel } from './layers.ts';

const QUICK = ['hospital', 'school', 'college', 'bank', 'atm', 'cafe', 'restaurant', 'hotel', 'metro_station', 'railway_station', 'bus_stop', 'bus_station', 'pharmacy', 'police', 'park', 'ground', 'sports_club', 'mall'];

export class ExplorePanel implements Panel {
  readonly id = 'explore';
  readonly label = 'Explore';
  readonly icon = 'explore';
  private body!: HTMLElement;
  private statsEl = h('div', { class: 'stats' });
  private resultsEl = h('div', {});
  private focused: string | null = null;
  private abort: AbortController | null = null;

  constructor(private readonly ctx: AppContext, private readonly layers: LayersPanel) {}

  render(body: HTMLElement): void {
    this.body = body;
    const areas = this.ctx.meta?.areas ?? [];
    const current = this.ctx.currentArea();
    const chip = (id: string | null, label: string, title?: string) =>
      h('button', { class: 'chip', type: 'button', title, 'aria-pressed': String(current === id), 'data-area': id ?? '', onclick: () => this.ctx.selectArea(id, true) }, label);

    const meta = this.ctx.meta;
    // The 24 BMC wards get a compact grid of their codes (A, F/N, R/C…) below the larger areas.
    const wards = areas.filter((a) => a.kind === 'ward');
    const others = areas.filter((a) => a.kind !== 'ward');
    clear(body);
    body.append(
      h('section', { class: 'section' },
        h('h3', {}, 'Areas'),
        h('div', { class: 'chips' }, chip(null, 'Everything'), ...others.map((a) => chip(a.id, a.short_name ?? a.name, a.description ?? a.name))),
        wards.length
          ? h('div', { class: 'ward-picker' },
            h('div', { class: 'sub ward-head', id: 'ward-label' }, 'BMC wards',
              h('button', { class: 'link-btn', type: 'button', onclick: () => void this.ctx.inspector.compareWards() }, 'Compare wards')),
            h('div', { class: 'chips ward-chips', role: 'group', 'aria-labelledby': 'ward-label' }, ...wards.map((a) => chip(a.id, a.short_name ?? a.name, a.description ? `${a.name} — ${a.description.replace(/^BMC ward [^:]+: /, '')}` : a.name))),
          )
          : null,
      ),
      h('section', { class: 'section' }, h('h3', {}, 'At a glance'), this.statsEl),
      h('section', { class: 'section' },
        h('h3', {}, 'Find in view', h('button', { class: 'link-btn', type: 'button', onclick: () => this.unfocus() }, 'Clear')),
        h('div', { class: 'chips' }, ...QUICK.filter((id) => categories.some((c) => c.id === id)).map((id) => {
          const c = categories.find((x) => x.id === id)!;
          return h('button', { class: 'chip', type: 'button', 'data-cat': id, 'aria-pressed': 'false', onclick: () => this.focus(id) }, categoryBadge(id, 20), c.label);
        })),
        this.resultsEl,
      ),
      h('section', { class: 'section' },
        h('h3', {}, 'Data'),
        h('p', {}, 'Built from ', h('a', { href: 'https://www.openstreetmap.org/copyright', target: '_blank', rel: 'noopener' }, 'OpenStreetMap'),
          meta?.dataset.source_timestamp ? ` data of ${formatDate(meta.dataset.source_timestamp)}` : '', '. Everything here is open — use the ',
          h('a', { href: '/docs/api/' }, 'REST API'), ', ', h('a', { href: '/docs/integrations/' }, 'SDKs & styles'), ' or ',
          h('a', { href: '/docs/data/' }, 'download the data'), '.'),
        h('p', { class: 'foot-note' }, 'Tip: right-click (or long-press) anywhere for “What’s here?”. Press / to search.'),
      ),
    );
    void this.loadStats();
  }

  onAreaChange(area: string | null): void {
    if (!this.body) return;
    this.body.querySelectorAll<HTMLElement>('[data-area]').forEach((el) => el.setAttribute('aria-pressed', String((el.dataset.area || null) === area)));
    void this.loadStats();
    if (this.focused) void this.focus(this.focused, true);
  }

  private async loadStats(): Promise<void> {
    clear(this.statsEl);
    this.statsEl.append(h('div', { class: 'skeleton' }));
    try {
      const area = this.ctx.currentArea() ?? undefined;
      const s = await this.ctx.api.stats(area);
      const t = s.totals;
      const p = s.places_by_category;
      const stat = (value: number | undefined, label: string) => h('div', { class: 'stat' }, h('b', {}, value === undefined ? '—' : formatNumber(Math.round(value))), h('span', {}, label));
      clear(this.statsEl);
      this.statsEl.append(
        stat(t.places, 'places'),
        stat(t.buildings, 'buildings'),
        stat(t.streets, 'named roads'),
        stat(t.bridges, 'bridges'),
        stat((p.hospital ?? 0) + (p.clinic ?? 0), 'hospitals & clinics'),
        stat((p.school ?? 0) + (p.college ?? 0) + (p.university ?? 0), 'schools & colleges'),
      );
    } catch {
      clear(this.statsEl);
      this.statsEl.append(h('p', { class: 'error-state' }, 'Statistics unavailable.'));
    }
  }

  /** Shows only one category on the map and lists it around the current view. */
  private async focus(category: string, refresh = false): Promise<void> {
    if (this.focused === category && !refresh) return this.unfocus();
    this.focused = category;
    this.body.querySelectorAll<HTMLElement>('[data-cat]').forEach((el) => el.setAttribute('aria-pressed', String(el.dataset.cat === category)));
    this.layers.isolate(category);
    this.abort?.abort();
    this.abort = new AbortController();
    clear(this.resultsEl);
    this.resultsEl.append(h('div', { class: 'skeleton' }), h('div', { class: 'skeleton' }));
    const center = this.ctx.map.map.getCenter();
    const [w, s, e, n] = this.ctx.map.visibleBounds();
    const radius = Math.min(10000, Math.max(800, (Math.hypot((e - w) * 105000, (n - s) * 111000) / 2)));
    try {
      const area = this.ctx.currentArea() ?? undefined;
      const res = await this.ctx.api.nearby(center.lng, center.lat, { radius, category: [category], area, limit: 40 }, this.abort.signal);
      this.renderResults(res.items, radius);
    } catch (err) {
      if ((err as Error).name !== 'AbortError') {
        clear(this.resultsEl);
        this.resultsEl.append(h('p', { class: 'error-state' }, 'Could not load places.'));
      }
    }
  }

  private renderResults(items: Place[], radius: number): void {
    clear(this.resultsEl);
    if (!items.length) {
      this.resultsEl.append(h('p', { class: 'empty-state' }, `None within ${formatDistance(radius)} of the map centre — zoom out or move the map.`));
      return;
    }
    this.resultsEl.append(
      h('p', { class: 'sub', style: 'margin:10px 0 4px' }, `${items.length} nearest to the map centre`),
      this.ctx.inspector.placeList(items),
    );
  }

  private unfocus(): void {
    this.focused = null;
    this.abort?.abort();
    this.body.querySelectorAll<HTMLElement>('[data-cat]').forEach((el) => el.setAttribute('aria-pressed', 'false'));
    clear(this.resultsEl);
    this.layers.restore();
  }
}
