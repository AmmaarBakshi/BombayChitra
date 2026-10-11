import type { TransitLine } from '../../api/types.ts';
import type { AppContext } from '../context.ts';
import { clear, h } from '../dom.ts';
import { formatDistance } from '../format.ts';
import type { Panel } from './panel.ts';

const MODE_ORDER: Record<string, number> = { subway: 0, light_rail: 0, monorail: 1, train: 2, tram: 3 };
const MODE_TITLE: Record<string, string> = { subway: 'Metro', light_rail: 'Light rail', monorail: 'Monorail', train: 'Suburban railway', tram: 'Tram' };

export class TransitPanel implements Panel {
  readonly id = 'transit';
  readonly label = 'Transit';
  readonly icon = 'train';
  private lines: TransitLine[] | null = null;

  constructor(private readonly ctx: AppContext) {}

  async render(body: HTMLElement): Promise<void> {
    if (!this.lines) {
      clear(body);
      body.append(h('div', { class: 'skeleton' }), h('div', { class: 'skeleton' }), h('div', { class: 'skeleton' }));
      try {
        this.lines = (await this.ctx.api.transitLines()).items;
      } catch {
        clear(body);
        body.append(h('p', { class: 'error-state' }, 'Transit lines unavailable.'));
        return;
      }
    }
    const byNetwork = new Map<string, TransitLine[]>();
    for (const line of [...this.lines].sort((a, b) => (MODE_ORDER[a.mode] ?? 9) - (MODE_ORDER[b.mode] ?? 9))) {
      const key = line.network ?? MODE_TITLE[line.mode] ?? line.mode;
      byNetwork.set(key, [...(byNetwork.get(key) ?? []), line]);
    }
    clear(body);
    body.append(h('p', { class: 'sub', style: 'margin-bottom:12px;white-space:normal' }, 'Every mapped metro, monorail and suburban line. Select a line to see its route and stations; click any track or station on the map for details.'));
    for (const [network, lines] of byNetwork) {
      body.append(h('section', { class: 'section' },
        h('h3', {}, network),
        h('ul', { class: 'list' }, ...lines.map((line) => {
          const stops = Math.max(...line.routes.map((r) => r.stop_count));
          const underConstruction = line.routes.every((r) => /u\/c|construction|proposed/i.test(`${r.name ?? ''} ${r.state ?? ''}`));
          return h('li', {}, h('button', { class: 'list-item', type: 'button', onclick: () => this.ctx.inspector.route(line.routes[0].id) },
            h('span', { class: `swatch${underConstruction ? ' dashed' : ''}`, style: `--swatch:${line.colour ?? '#868e96'}` }),
            h('span', { class: 'grow' },
              h('div', { class: 'title' }, line.label, line.ref && !line.label.includes(line.ref) ? ` (${line.ref})` : ''),
              h('div', { class: 'sub' }, [formatDistance(line.length_m), stops ? `${stops} stations` : null, underConstruction ? 'under construction' : null].filter(Boolean).join(' · ')),
            ),
          ));
        })),
      ));
    }
    body.append(h('p', { class: 'foot-note' }, 'Line data comes from OpenStreetMap route relations; colours follow the operators’ line colours where tagged.'));
  }
}
