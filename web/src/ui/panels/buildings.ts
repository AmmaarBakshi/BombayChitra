import type { Building } from '../../api/types.ts';
import { t } from '../../i18n.ts';
import type { AppContext } from '../context.ts';
import { clear, h } from '../dom.ts';
import { capitalise, formatArea, formatNumber } from '../format.ts';
import { kindBadge } from '../icons.ts';
import type { LayersPanel } from './layers.ts';
import type { Panel } from './panel.ts';

const SORTS = [
  { id: 'height', label: 'Tallest' },
  { id: 'levels', label: 'Most floors' },
  { id: 'footprint', label: 'Largest' },
];

export class BuildingsPanel implements Panel {
  readonly id = 'buildings';
  readonly label = t('tab.buildings');
  readonly icon = 'building';
  private sort = 'height';
  private body: HTMLElement | null = null;
  private list = h('div', {});
  private stats = h('div', { class: 'stats', style: 'margin-bottom:14px' });
  private abort: AbortController | null = null;

  constructor(private readonly ctx: AppContext, private readonly layers: LayersPanel) {
    layers.onOverlayChange((key) => key === 'buildings3d' && this.body && this.render(this.body));
  }

  render(body: HTMLElement): void {
    this.body = body;
    const toggle = h('input', { type: 'checkbox', checked: this.ctx.map.getOverlays().buildings3d });
    toggle.addEventListener('change', () => this.layers.setOverlay('buildings3d', (toggle as HTMLInputElement).checked));
    clear(body);
    body.append(
      h('label', { class: 'switch-row', style: 'margin-bottom:8px' }, toggle, h('span', { class: 'switch', 'aria-hidden': 'true' }),
        h('span', { class: 'grow' }, h('div', { class: 'title' }, '3D buildings'), h('div', { class: 'sub' }, 'Extrude by mapped height or floor count'))),
      this.stats,
      h('div', { class: 'segmented', role: 'group', 'aria-label': 'Sort buildings' }, ...SORTS.map((s) =>
        h('button', { type: 'button', 'aria-pressed': String(s.id === this.sort), onclick: () => { this.sort = s.id; this.render(body); } }, s.label))),
      this.list,
    );
    void this.load();
  }

  onAreaChange(): void {
    if (this.body) void this.load();
  }

  private async load(): Promise<void> {
    this.abort?.abort();
    this.abort = new AbortController();
    const signal = this.abort.signal;
    const area = this.ctx.currentArea() ?? undefined;
    clear(this.list);
    this.list.append(h('div', { class: 'skeleton' }), h('div', { class: 'skeleton' }));
    try {
      const [stats, res] = await Promise.all([
        this.ctx.api.stats(area, signal),
        this.ctx.api.buildings({ area, sort: this.sort, limit: 50, named: this.sort !== 'footprint' }, signal),
      ]);
      const t = stats.totals;
      const stat = (v: string, l: string) => h('div', { class: 'stat' }, h('b', {}, v), h('span', {}, l));
      clear(this.stats);
      this.stats.append(
        stat(formatNumber(t.buildings ?? 0), 'footprints'),
        stat(t.building_avg_height_m ? `${t.building_avg_height_m.toFixed(1)} m` : '—', 'avg. mapped height'),
        stat(t.building_max_height_m ? `${Math.round(t.building_max_height_m)} m` : '—', 'tallest'),
      );
      this.renderList(res.items);
    } catch (err) {
      if ((err as Error).name !== 'AbortError') {
        clear(this.list);
        this.list.append(h('p', { class: 'error-state' }, 'Buildings unavailable.'));
      }
    }
  }

  private renderList(items: Building[]): void {
    clear(this.list);
    if (!items.length) {
      this.list.append(h('p', { class: 'empty-state' }, 'No buildings with this data here yet.'));
      return;
    }
    this.list.append(h('ul', { class: 'list' }, ...items.map((b, i) => h('li', {},
      h('button', { class: 'list-item', type: 'button', onclick: () => this.open(b) },
        kindBadge('building', '#7a6e64', 30),
        h('span', { class: 'grow' },
          h('div', { class: 'title' }, `${i + 1}. ${b.name ?? (b.kind ? capitalise(b.kind) : 'Building')}`),
          h('div', { class: 'sub' }, [b.levels ? `${b.levels} floors` : null, b.height_m ? `${Math.round(b.height_m)} m` : null, this.sort === 'footprint' ? formatArea(b.footprint_m2) : null, b.locality].filter(Boolean).join(' · ')),
        ),
      ),
    ))));
  }

  private open(b: Building): void {
    if (!this.ctx.map.getOverlays().buildings3d) this.layers.setOverlay('buildings3d', true);
    void this.ctx.inspector.buildingById(b.id);
  }
}
