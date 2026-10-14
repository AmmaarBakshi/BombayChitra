// Which vector tiles cover an area, for saving the map of that area for offline use (see ui/offline.ts and the service
// worker in build/sw.js, which serves saved tiles when the network is unavailable).
import type { BBox } from './api/types.ts';

/** Tiles at most saved for one area: about 30 MB of vector tiles. */
export const MAX_TILES = 4000;
/** Zoom levels saved below the current one, so zooming out offline still shows the surroundings. */
export const ZOOMS_BELOW = 3;

export type Tile = [z: number, x: number, y: number];

/** The x and y tile ranges covering a bounding box at zoom z (Web Mercator, as the tiles). */
export function tileRange(bbox: BBox, z: number): { minX: number; maxX: number; minY: number; maxY: number } {
  const n = 2 ** z;
  const x = (lon: number) => Math.min(n - 1, Math.max(0, Math.floor(((lon + 180) / 360) * n)));
  const y = (lat: number) => {
    const r = (Math.max(-85.0511, Math.min(85.0511, lat)) * Math.PI) / 180;
    return Math.min(n - 1, Math.max(0, Math.floor(((1 - Math.log(Math.tan(r) + 1 / Math.cos(r)) / Math.PI) / 2) * n)));
  };
  // North is the smaller y.
  return { minX: x(bbox[0]), maxX: x(bbox[2]), minY: y(bbox[3]), maxY: y(bbox[1]) };
}

/** The number of tiles covering a bounding box from zoom minZoom to maxZoom. */
export function countTiles(bbox: BBox, minZoom: number, maxZoom: number): number {
  let count = 0;
  for (let z = minZoom; z <= maxZoom; z++) {
    const r = tileRange(bbox, z);
    count += (r.maxX - r.minX + 1) * (r.maxY - r.minY + 1);
  }
  return count;
}

/** Every tile covering a bounding box from zoom minZoom to maxZoom, lowest zoom first. */
export function tilesFor(bbox: BBox, minZoom: number, maxZoom: number): Tile[] {
  const tiles: Tile[] = [];
  for (let z = minZoom; z <= maxZoom; z++) {
    const r = tileRange(bbox, z);
    for (let x = r.minX; x <= r.maxX; x++) for (let y = r.minY; y <= r.maxY; y++) tiles.push([z, x, y]);
  }
  return tiles;
}

/**
 * The zoom levels to save for the area in view at the given map zoom: a few levels below it for context, up to the
 * dataset's most detailed tiles (the map draws closer zooms from those). `tiles` is the count, `ok` whether it is
 * within MAX_TILES.
 */
export function offlinePlan(bbox: BBox, zoom: number, maxzoom: number): { minZoom: number; maxZoom: number; tiles: number; ok: boolean } {
  const maxZoom = Math.max(0, Math.round(maxzoom));
  const minZoom = Math.min(maxZoom, Math.max(0, Math.floor(zoom) - ZOOMS_BELOW));
  const tiles = countTiles(bbox, minZoom, maxZoom);
  return { minZoom, maxZoom, tiles, ok: tiles <= MAX_TILES };
}
