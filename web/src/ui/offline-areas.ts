// "Offline maps" in the Layers panel: saves the vector tiles of the area in view into the browser's Cache Storage
// (one cache per area, bc-area-<id>), from which the service worker serves them when there is no connection.
import type { BBox } from '../api/types.ts';
import { offlinePlan, type Tile, tilesFor } from '../offline.ts';
import type { AppContext } from './context.ts';
import { clear, h, storageGet, storageSet } from './dom.ts';
import { formatBytes, formatDate, formatNumber } from './format.ts';
import { icon } from './icons.ts';

const AREAS_KEY = 'bc-offline-areas';
/** Tiles fetched at once while saving. */
const PARALLEL = 6;

/** An area saved for offline use. */
export interface SavedArea {
  id: string;
  name: string;
  bbox: BBox;
  minZoom: number;
  maxZoom: number;
  tiles: number;
  bytes: number;
  saved: string;
}

export function savedAreas(): SavedArea[] {
  try {
    return JSON.parse(storageGet(AREAS_KEY) ?? '[]') as SavedArea[];
  } catch {
    return [];
  }
}

function storeAreas(areas: SavedArea[]): void {
  storageSet(AREAS_KEY, JSON.stringify(areas));
}

/** Fetches tiles into a cache, a few at a time; returns the bytes stored. */
async function saveTiles(cacheName: string, tiles: Tile[], onProgress: (done: number) => void, signal: AbortSignal): Promise<number> {
  const cache = await caches.open(cacheName);
  let next = 0;
  let done = 0;
  let bytes = 0;
  const worker = async () => {
    while (next < tiles.length) {
      const [z, x, y] = tiles[next++];
      const url = `/tiles/${z}/${x}/${y}.mvt`;
      const res = await fetch(url, { signal });
      if (res.status === 204) {
        await cache.put(url, new Response(null, { status: 204 })); // open sea: nothing to draw
      } else if (res.ok) {
        const body = await res.arrayBuffer();
        bytes += body.byteLength;
        await cache.put(url, new Response(body, { headers: { 'Content-Type': res.headers.get('Content-Type') ?? 'application/vnd.mapbox-vector-tile' } }));
      } else {
        throw new Error(`tile ${z}/${x}/${y}: HTTP ${res.status}`);
      }
      onProgress(++done);
    }
  };
  await Promise.all(Array.from({ length: PARALLEL }, worker));
  return bytes;
}

export class OfflineAreas {
  readonly el = h('section', { class: 'section offline-areas' });
  private readonly estimate = h('p', { class: 'sub', style: 'white-space:normal' });
  private readonly saveButton = h('button', { class: 'btn', type: 'button', onclick: () => void this.save() }, icon('install', 15), 'Save this area') as HTMLButtonElement;
  private readonly progress = h('div', { class: 'offline-progress', hidden: true });
  private readonly list = h('ul', { class: 'offline-list' });
  private readonly usage = h('p', { class: 'sub' });
  private abort: AbortController | null = null;

  constructor(private readonly ctx: AppContext) {
    this.ctx.map.map.on('moveend', () => this.update());
  }

  /** Whether this browser can keep tiles (Cache Storage needs a secure context). */
  static get supported(): boolean {
    return typeof caches !== 'undefined';
  }

  render(): HTMLElement {
    clear(this.el);
    this.el.append(
      h('h3', {}, 'Offline maps'),
      this.estimate,
      h('div', { class: 'btn-row' }, this.saveButton),
      this.progress,
      this.list,
      this.usage,
    );
    this.update();
    void this.renderList();
    return this.el;
  }

  /** The saved areas, each with its size and buttons to show or delete it, and the storage they use. */
  private async renderList(): Promise<void> {
    const areas = savedAreas();
    clear(this.list);
    for (const area of areas) {
      this.list.append(h('li', {},
        h('span', { class: 'grow' },
          h('div', { class: 'title' }, area.name),
          h('div', { class: 'sub' }, `${formatNumber(area.tiles)} tiles · ${formatBytes(area.bytes)} · saved ${formatDate(area.saved)}`),
        ),
        h('button', { class: 'link-btn', type: 'button', onclick: () => this.ctx.map.fitBounds(area.bbox, 15) }, 'Show'),
        h('button', { class: 'link-btn danger', type: 'button', 'aria-label': `Delete ${area.name}`, onclick: () => void this.remove(area) }, 'Delete'),
      ));
    }
    this.list.hidden = areas.length === 0;
    const estimate = await navigator.storage?.estimate?.().catch(() => undefined);
    this.usage.hidden = !areas.length || !estimate?.usage;
    if (estimate?.usage) {
      this.usage.textContent = `The app uses ${formatBytes(estimate.usage)} of storage on this device${estimate.quota ? ` (of ${formatBytes(estimate.quota)} available)` : ''}.`;
    }
  }

  private async remove(area: SavedArea): Promise<void> {
    await caches.delete(`bc-area-${area.id}`);
    storeAreas(savedAreas().filter((a) => a.id !== area.id));
    this.ctx.toast(`${area.name} removed from offline maps`);
    await this.renderList();
  }

  private plan() {
    const zoom = this.ctx.map.map.getZoom();
    return { bbox: this.ctx.map.visibleBounds(), ...offlinePlan(this.ctx.map.visibleBounds(), zoom, this.ctx.meta?.region.maxzoom ?? 15) };
  }

  /** Describes what saving the view would take, or why it cannot be saved. */
  private update(): void {
    if (this.abort) return;
    const plan = this.plan();
    this.saveButton.disabled = !plan.ok;
    this.estimate.textContent = plan.ok
      ? `Keep the map of the area in view for use without a connection: ${formatNumber(plan.tiles)} tiles from zoom ${plan.minZoom} to ${plan.maxZoom}.`
      : `The area in view needs ${formatNumber(plan.tiles)} tiles; zoom in to save a smaller area (up to 4,000 tiles).`;
  }

  private async save(): Promise<void> {
    const plan = this.plan();
    if (!plan.ok || this.abort) return;
    const tiles = tilesFor(plan.bbox, plan.minZoom, plan.maxZoom);
    const id = Date.now().toString(36);
    const abort = (this.abort = new AbortController());
    const bar = h('progress', { max: tiles.length, value: 0 }) as HTMLProgressElement;
    const label = h('span', { class: 'sub' }, `0 of ${formatNumber(tiles.length)} tiles`);
    clear(this.progress);
    this.progress.append(bar, h('div', { class: 'btn-row' }, label, h('button', { class: 'link-btn', type: 'button', onclick: () => abort.abort() }, 'Cancel')));
    this.progress.hidden = false;
    this.saveButton.disabled = true;
    try {
      const name = await this.name(plan.bbox);
      const bytes = await saveTiles(`bc-area-${id}`, tiles, (done) => {
        bar.value = done;
        label.textContent = `${formatNumber(done)} of ${formatNumber(tiles.length)} tiles`;
      }, abort.signal);
      storeAreas([...savedAreas(), { id, name, bbox: plan.bbox, minZoom: plan.minZoom, maxZoom: plan.maxZoom, tiles: tiles.length, bytes, saved: new Date().toISOString() }]);
      // Ask the browser not to clear saved maps when space runs low.
      void navigator.storage?.persist?.().catch(() => false);
      this.ctx.toast(`${name} is saved for offline use`);
      await this.renderList();
    } catch (err) {
      await caches.delete(`bc-area-${id}`);
      if ((err as Error).name !== 'AbortError') this.ctx.toast(`Could not save the area: ${(err as Error).message}`);
    } finally {
      this.abort = null;
      this.progress.hidden = true;
      this.update();
    }
  }

  /** Names an area after the locality at its centre. */
  private async name(bbox: BBox): Promise<string> {
    const lon = (bbox[0] + bbox[2]) / 2;
    const lat = (bbox[1] + bbox[3]) / 2;
    try {
      const r = await this.ctx.api.reverse(lon, lat);
      return r.locality?.name ?? r.label ?? `Area at ${lat.toFixed(3)}, ${lon.toFixed(3)}`;
    } catch {
      return `Area at ${lat.toFixed(3)}, ${lon.toFixed(3)}`;
    }
  }
}
