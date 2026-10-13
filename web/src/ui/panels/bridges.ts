import type { NamedBridge } from '../../api/types.ts';
import { t } from '../../i18n.ts';
import type { AppContext } from '../context.ts';
import { clear, h } from '../dom.ts';
import { formatDistance } from '../format.ts';
import { kindBadge } from '../icons.ts';
import type { LayersPanel } from './layers.ts';
import type { Panel } from './panel.ts';

const FILTERS: { id: string; label: string; kinds: string[] }[] = [
  { id: 'road', label: 'Road', kinds: ['road'] },
  { id: 'metro', label: 'Metro', kinds: ['metro'] },
  { id: 'rail', label: 'Rail', kinds: ['rail'] },
  { id: 'foot', label: 'Foot', kinds: ['foot'] },
  { id: 'construction', label: 'Under construction', kinds: ['construction'] },
];
const KIND_LABEL: Record<string, string> = { road: 'Road bridge', rail: 'Railway bridge', metro: 'Metro viaduct', foot: 'Foot over-bridge', construction: 'Under construction', aqueduct: 'Aqueduct' };
const KIND_COLOUR: Record<string, string> = { road: '#7048e8', rail: '#a0522d', metro: '#0c8599', foot: '#2b8a3e', construction: '#e67700', aqueduct: '#1971c2' };

export class BridgesPanel implements Panel {
  readonly id = 'bridges';
  readonly label = t('tab.bridges');
  readonly icon = 'bridge';
  private filter = 'road';
  private body: HTMLElement | null = null;
  private list = h('div', {});
  private abort: AbortController | null = null;

  constructor(private readonly ctx: AppContext, private readonly layers: LayersPanel) {
    layers.onOverlayChange((key) => key === 'bridges' && this.body && this.render(this.body));
  }

  render(body: HTMLElement): void {
    this.body = body;
    const on = this.ctx.map.getOverlays().bridges;
    const toggle = h('input', { type: 'checkbox', checked: on });
    toggle.addEventListener('change', () => this.layers.setOverlay('bridges', (toggle as HTMLInputElement).checked));
    clear(body);
    body.append(
      h('label', { class: 'switch-row', style: 'margin-bottom:6px' }, toggle, h('span', { class: 'switch', 'aria-hidden': 'true' }),
        h('span', { class: 'grow' }, h('div', { class: 'title' }, 'Bridge explorer'), h('div', { class: 'sub' }, 'Colour every bridge by type, with names and spans'))),
      h('div', { class: 'segmented', role: 'group', 'aria-label': 'Bridge type' }, ...FILTERS.map((f) =>
        h('button', { type: 'button', 'aria-pressed': String(f.id === this.filter), onclick: () => { this.filter = f.id; this.render(body); } }, f.label))),
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
    clear(this.list);
    this.list.append(h('div', { class: 'skeleton' }), h('div', { class: 'skeleton' }));
    const kinds = FILTERS.find((f) => f.id === this.filter)?.kinds;
    try {
      const res = await this.ctx.api.namedBridges({ kind: kinds, area: this.ctx.currentArea() ?? undefined, limit: 60 }, this.abort.signal);
      this.renderList(res.items);
    } catch (err) {
      if ((err as Error).name !== 'AbortError') {
        clear(this.list);
        this.list.append(h('p', { class: 'error-state' }, 'Bridges unavailable.'));
      }
    }
  }

  private renderList(items: NamedBridge[]): void {
    clear(this.list);
    if (!items.length) {
      this.list.append(h('p', { class: 'empty-state' }, 'No named bridges of this type here.'));
      return;
    }
    this.list.append(
      h('h3', { class: 'section', style: 'margin:4px 0 6px;font-size:11.5px;letter-spacing:.06em;text-transform:uppercase;color:var(--faint)' }, 'Longest named bridges'),
      h('ul', { class: 'list' }, ...items.map((b) => h('li', {},
        h('button', { class: 'list-item', type: 'button', onclick: () => this.open(b) },
          kindBadge('bridge', KIND_COLOUR[b.kind] ?? '#7048e8', 30),
          h('span', { class: 'grow' }, h('div', { class: 'title' }, b.name), h('div', { class: 'sub' }, [KIND_LABEL[b.kind] ?? b.kind, b.locality].filter(Boolean).join(' · '))),
          h('span', { class: 'meta' }, formatDistance(b.length_m)),
        ),
      ))),
    );
  }

  private open(b: NamedBridge): void {
    if (!this.ctx.map.getOverlays().bridges) this.layers.setOverlay('bridges', true);
    void this.ctx.inspector.namedBridge(b.id);
  }
}
