// "Near me" in the Explore panel: the nearest station, hospital or ATM from where you are — or from the centre of the
// map when your location is not available — each opening its card.
import type { Place } from '../api/types.ts';
import { type Key, t } from '../i18n.ts';
import { categoryById } from '../taxonomy.ts';
import type { AppContext } from './context.ts';
import { clear, h } from './dom.ts';
import { formatDistance } from './format.ts';
import { categoryBadge } from './icons.ts';

/** What can be looked for, and the place categories that count. */
const KINDS: { id: string; label: Key; badge: string; categories: string[] }[] = [
  { id: 'station', label: 'near.station', badge: 'railway_station', categories: ['railway_station', 'metro_station', 'monorail_station'] },
  { id: 'hospital', label: 'near.hospital', badge: 'hospital', categories: ['hospital'] },
  { id: 'atm', label: 'near.atm', badge: 'atm', categories: ['atm'] },
];
const RADIUS_M = 5000;
const WALK_SPEED = 4.8 / 3.6; // m/s
const DETOUR = 1.3; // metres walked per metre of straight line

/** Where to look from: your location, or the centre of the map. */
export interface Origin {
  lon: number;
  lat: number;
  located: boolean;
}

export class NearMe {
  readonly el = h('section', { class: 'section near-me' });
  private readonly buttons: HTMLButtonElement[] = [];
  private readonly results = h('div', { 'aria-live': 'polite' });
  private abort: AbortController | null = null;
  /** The last place looked from. */
  origin: Origin | null = null;

  constructor(private readonly ctx: AppContext) {}

  render(): HTMLElement {
    clear(this.el);
    this.buttons.length = 0;
    for (const kind of KINDS) {
      this.buttons.push(h('button', { class: 'chip', type: 'button', 'data-near': kind.id, 'aria-pressed': 'false', onclick: () => void this.find(kind) }, categoryBadge(kind.badge, 20), t(kind.label)) as HTMLButtonElement);
    }
    this.el.append(h('h3', {}, t('near.title')), h('div', { class: 'chips' }, ...this.buttons), this.results);
    return this.el;
  }

  /** Your location, or the centre of the map when the browser cannot (or may not) tell. */
  private locate(): Promise<Origin> {
    const centre = (): Origin => {
      const c = this.ctx.map.map.getCenter();
      return { lon: c.lng, lat: c.lat, located: false };
    };
    if (!('geolocation' in navigator)) return Promise.resolve(centre());
    return new Promise((resolve) => navigator.geolocation.getCurrentPosition(
      (pos) => resolve({ lon: pos.coords.longitude, lat: pos.coords.latitude, located: true }),
      () => resolve(centre()),
      { enableHighAccuracy: true, timeout: 8000, maximumAge: 60_000 },
    ));
  }

  private async find(kind: (typeof KINDS)[number]): Promise<void> {
    this.buttons.forEach((b) => b.setAttribute('aria-pressed', String(b.dataset.near === kind.id)));
    this.abort?.abort();
    const abort = (this.abort = new AbortController());
    clear(this.results);
    this.results.append(h('div', { class: 'skeleton' }), h('div', { class: 'skeleton' }));
    const origin = await this.locate();
    this.origin = origin;
    try {
      const res = await this.ctx.api.nearby(origin.lon, origin.lat, { category: kind.categories, radius: RADIUS_M, limit: 3 }, abort.signal);
      clear(this.results);
      if (!origin.located) this.results.append(h('p', { class: 'sub', style: 'white-space:normal' }, t('near.fromMap')));
      if (!res.items.length) {
        this.results.append(h('p', { class: 'empty-state' }, t('near.none')));
        return;
      }
      this.results.append(h('ul', { class: 'list' }, ...res.items.map((p) => this.item(p))));
      const nearest = res.items[0];
      this.ctx.map.fitBounds([Math.min(origin.lon, nearest.lon), Math.min(origin.lat, nearest.lat), Math.max(origin.lon, nearest.lon), Math.max(origin.lat, nearest.lat)], 17);
    } catch (err) {
      if ((err as Error).name === 'AbortError') return;
      clear(this.results);
      this.results.append(h('p', { class: 'error-state' }, (err as Error).message));
    }
  }

  private item(p: Place): HTMLLIElement {
    const metres = p.distance_m ?? 0;
    const minutes = Math.max(1, Math.round((metres * DETOUR) / WALK_SPEED / 60));
    return h('li', {},
      h('button', { class: 'list-item', type: 'button', onclick: () => void this.ctx.inspector.place(p.id, true) },
        categoryBadge(p.category, 26),
        h('span', { class: 'grow' },
          h('div', { class: 'title' }, p.name ?? categoryById.get(p.category)?.label ?? p.category),
          h('div', { class: 'sub' }, t('near.walk', { min: minutes })),
        ),
        h('span', { class: 'meta' }, formatDistance(metres)),
      ),
    );
  }
}
