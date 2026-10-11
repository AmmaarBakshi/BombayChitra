import type { BombayChitraClient } from '../api/client.ts';
import type { Meta } from '../api/types.ts';
import type { MapController } from '../map/controller.ts';
import type { Inspector } from './inspector.ts';

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
}
