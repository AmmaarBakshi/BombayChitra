import type { Overlays } from '../../map/controller.ts';
import { t } from '../../i18n.ts';
import { categories, groups } from '../../taxonomy.ts';
import type { AppContext } from '../context.ts';
import { clear, h, storageGet, storageSet } from '../dom.ts';
import { formatNumber } from '../format.ts';
import { categoryBadge, icon } from '../icons.ts';
import { OfflineAreas } from '../offline-areas.ts';
import type { Panel } from './panel.ts';

const HIDDEN_KEY = 'bc-hidden-categories';
const OVERLAYS_KEY = 'bc-overlays';
/** Overlays remembered between visits (3D needs a tilt, so it always starts off). */
const REMEMBERED: (keyof Overlays)[] = ['bridges', 'railColours', 'boundaries', 'heritage', 'hotspots'];
const OVERLAYS: { key: keyof Overlays; label: string; hint: string }[] = [
  { key: 'buildings3d', label: '3D buildings', hint: 'Extrude buildings by height and floors' },
  { key: 'bridges', label: 'Bridge explorer', hint: 'Highlight every bridge, flyover and viaduct' },
  { key: 'railColours', label: 'Suburban line colours', hint: 'Western, Central, Harbour and other lines' },
  { key: 'boundaries', label: 'Boundaries', hint: 'Districts, municipal corporations and wards' },
  { key: 'heritage', label: 'Heritage sites', hint: 'World Heritage Sites, heritage precincts and listed monuments' },
  { key: 'hotspots', label: 'Monsoon waterlogging', hint: 'Spots that flood in heavy rain, June to September' },
  { key: 'coverage', label: 'Data coverage', hint: 'Red where OpenStreetMap lacks places, green where well mapped' },
];

export class LayersPanel implements Panel {
  readonly id = 'layers';
  readonly label = t('tab.layers');
  readonly icon = 'layers';
  private hidden: Set<string>;
  private saved: Set<string> | null = null;
  private counts = new Map<string, number>();
  private body: HTMLElement | null = null;
  private overlayListeners: ((key: keyof Overlays, on: boolean) => void)[] = [];
  private readonly offline: OfflineAreas | null;

  constructor(private readonly ctx: AppContext) {
    this.offline = OfflineAreas.supported ? new OfflineAreas(ctx) : null;
    let initial: string[] = [];
    try {
      initial = JSON.parse(storageGet(HIDDEN_KEY) ?? '[]') as string[];
    } catch {
      initial = [];
    }
    this.hidden = new Set(initial.filter((id) => categories.some((c) => c.id === id)));
    this.ctx.map.setHiddenCategories([...this.hidden]);
    try {
      const saved = JSON.parse(storageGet(OVERLAYS_KEY) ?? '{}') as Partial<Record<keyof Overlays, boolean>>;
      for (const key of REMEMBERED) if (typeof saved[key] === 'boolean') this.ctx.map.setOverlay(key, saved[key]);
    } catch {
      /* ignore a corrupt preference */
    }
    void this.ctx.api.categories().then((res) => {
      for (const g of res.groups) for (const c of g.categories) this.counts.set(c.id, c.count);
      if (this.body) this.render(this.body);
    }).catch(() => undefined);
  }

  onOverlayChange(fn: (key: keyof Overlays, on: boolean) => void): void {
    this.overlayListeners.push(fn);
  }

  setOverlay(key: keyof Overlays, on: boolean): void {
    this.ctx.map.setOverlay(key, on);
    this.overlayListeners.forEach((fn) => fn(key, on));
    const overlays = this.ctx.map.getOverlays();
    storageSet(OVERLAYS_KEY, JSON.stringify(Object.fromEntries(REMEMBERED.map((k) => [k, overlays[k]]))));
    if (this.body) this.render(this.body);
  }

  render(body: HTMLElement): void {
    this.body = body;
    const overlays = this.ctx.map.getOverlays();
    clear(body);
    body.append(
      h('section', { class: 'section' },
        h('h3', {}, 'Map overlays'),
        ...OVERLAYS.map((o) => {
          const input = h('input', { type: 'checkbox', checked: overlays[o.key] });
          input.addEventListener('change', () => this.setOverlay(o.key, input.checked));
          return h('label', { class: 'switch-row' }, input, h('span', { class: 'switch', 'aria-hidden': 'true' }), h('span', { class: 'grow' }, h('div', { class: 'title' }, o.label), h('div', { class: 'sub' }, o.hint)));
        }),
      ),
      ...(this.offline ? [this.offline.render()] : []),
      h('section', { class: 'section' },
        h('h3', {}, 'Places',
          h('span', {},
            h('button', { class: 'link-btn', type: 'button', onclick: () => this.setAll(true) }, 'Show all'),
            ' · ',
            h('button', { class: 'link-btn', type: 'button', onclick: () => this.setAll(false) }, 'Hide all'),
          ),
        ),
        ...groups.map((g) => this.groupEl(g.id)),
      ),
    );
  }

  private groupEl(groupId: string): HTMLElement {
    const g = groups.find((x) => x.id === groupId)!;
    const visible = g.categories.filter((c) => !this.hidden.has(c.id)).length;
    const total = g.categories.reduce((n, c) => n + (this.counts.get(c.id) ?? 0), 0);
    const master = h('input', { type: 'checkbox', checked: visible > 0, 'aria-label': `Show ${g.label}` }) as HTMLInputElement;
    master.indeterminate = visible > 0 && visible < g.categories.length;
    master.addEventListener('change', () => this.setMany(g.categories.map((c) => c.id), master.checked));
    const wrap = h('div', { class: 'group' });
    const expand = h('button', { class: 'expand', type: 'button', 'aria-label': `Show the ${g.label} categories`, 'aria-expanded': 'false', 'aria-controls': `group-${g.id}` }, icon('chevron', 16));
    expand.addEventListener('click', () => {
      const open = wrap.toggleAttribute('open');
      expand.setAttribute('aria-expanded', String(open));
    });
    wrap.append(
      h('div', { class: 'group-head' },
        expand,
        h('label', { class: 'check grow' }, master, h('span', { class: 'badge', style: `--badge:${g.colour};width:12px;height:12px` }), h('span', { class: 'grow title' }, g.label), h('span', { class: 'meta count' }, total ? formatNumber(total) : '')),
      ),
      h('div', { class: 'group-body', id: `group-${g.id}` }, ...g.categories.map((c) => {
        const input = h('input', { type: 'checkbox', checked: !this.hidden.has(c.id) }) as HTMLInputElement;
        input.addEventListener('change', () => this.setMany([c.id], input.checked));
        const count = this.counts.get(c.id);
        return h('label', { class: 'check' }, input, categoryBadge(c.id, 20), h('span', {}, c.label), h('span', { class: 'meta count' }, count !== undefined ? formatNumber(count) : ''));
      })),
    );
    return wrap;
  }

  private setMany(ids: string[], visible: boolean): void {
    this.saved = null;
    for (const id of ids) {
      if (visible) this.hidden.delete(id);
      else this.hidden.add(id);
    }
    this.apply();
  }

  private setAll(visible: boolean): void {
    this.setMany(categories.map((c) => c.id), visible);
  }

  /** Temporarily show a single category (used by Explore → Find in view). */
  isolate(category: string): void {
    if (!this.saved) this.saved = new Set(this.hidden);
    this.hidden = new Set(categories.map((c) => c.id).filter((id) => id !== category));
    this.ctx.map.setHiddenCategories([...this.hidden]);
  }

  restore(): void {
    if (!this.saved) return;
    this.hidden = this.saved;
    this.saved = null;
    this.ctx.map.setHiddenCategories([...this.hidden]);
  }

  private apply(): void {
    this.ctx.map.setHiddenCategories([...this.hidden]);
    storageSet(HIDDEN_KEY, JSON.stringify([...this.hidden]));
    if (this.body) {
      const open = new Set([...this.body.querySelectorAll('.group[open] .title')].map((el) => el.textContent));
      this.render(this.body);
      this.body.querySelectorAll('.group').forEach((el) => {
        if (open.has(el.querySelector('.title')?.textContent ?? '')) el.setAttribute('open', '');
      });
    }
  }
}
