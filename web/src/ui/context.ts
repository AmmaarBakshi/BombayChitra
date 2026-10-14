import type { BombayChitraClient } from '../api/client.ts';
import type { Meta } from '../api/types.ts';
import type { MapController } from '../map/controller.ts';
import type { Inspector } from './inspector.ts';
import type { RoutePoint, Travel } from './panels/distance.ts';

/** Services shared by the UI modules. */
export interface AppContext {
  api: BombayChitraClient;
  map: MapController;
  inspector: Inspector;
  meta: Meta | null;
  toast(message: string): void;
  selectArea(id: string | null, fit?: boolean): void;
  currentArea(): string | null;
  /** Opens "distance by road" with this point as the destination. */
  routeTo(point: { lon: number; lat: number; label: string }): void;
  /** Opens "distance by road" between two places, optionally by a given way of travelling. */
  directions(from: RoutePoint, to: RoutePoint, mode?: Travel): void;
}
