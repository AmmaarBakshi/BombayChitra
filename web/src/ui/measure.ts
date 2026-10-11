import type { MapController } from '../map/controller.ts';
import { h } from './dom.ts';
import { distance, formatDistance } from './format.ts';
import { icon } from './icons.ts';

/** Click-to-measure polyline distances. */
export class MeasureTool {
  private points: [number, number][] = [];
  private hud: HTMLElement | null = null;
  private total = h('span', { class: 'mono' }, '0 m');
  active = false;

  constructor(private readonly map: MapController, private readonly button: HTMLElement) {
    document.addEventListener('keydown', (e) => {
      if (!this.active) return;
      if (e.key === 'Escape') this.stop();
      if ((e.key === 'Backspace' || e.key === 'z') && !(e.target as HTMLElement).closest('input')) this.undo();
    });
  }

  toggle(): void {
    if (this.active) this.stop();
    else this.start();
  }

  start(): void {
    this.active = true;
    this.points = [];
    this.button.setAttribute('aria-pressed', 'true');
    this.map.map.getCanvas().style.cursor = 'crosshair';
    this.hud = h('div', { class: 'measure-hud', role: 'status' },
      icon('ruler', 16), this.total,
      h('button', { class: 'btn', type: 'button', onclick: () => this.undo() }, 'Undo'),
      h('button', { class: 'btn', type: 'button', onclick: () => this.clear() }, 'Clear'),
      h('button', { class: 'btn btn-primary', type: 'button', onclick: () => this.stop() }, 'Done'),
    );
    document.body.append(this.hud);
    this.update();
  }

  stop(): void {
    this.active = false;
    this.button.setAttribute('aria-pressed', 'false');
    this.map.map.getCanvas().style.cursor = '';
    this.hud?.remove();
    this.hud = null;
    this.points = [];
    this.map.setMeasure(null);
  }

  add(lngLat: [number, number]): void {
    this.points.push(lngLat);
    this.update();
  }

  private undo(): void {
    this.points.pop();
    this.update();
  }

  private clear(): void {
    this.points = [];
    this.update();
  }

  private update(): void {
    let total = 0;
    for (let i = 1; i < this.points.length; i++) total += distance(this.points[i - 1], this.points[i]);
    this.total.textContent = this.points.length < 2 ? 'Click the map to measure' : formatDistance(total);
    const features: GeoJSON.Feature[] = this.points.map((p) => ({ type: 'Feature', geometry: { type: 'Point', coordinates: p }, properties: {} }));
    if (this.points.length > 1) features.push({ type: 'Feature', geometry: { type: 'LineString', coordinates: this.points }, properties: {} });
    this.map.setMeasure({ type: 'FeatureCollection', features });
  }
}
