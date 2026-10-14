import { describe, expect, it } from 'vitest';
import type { BBox } from '../src/api/types.ts';
import { countTiles, MAX_TILES, offlinePlan, tileRange, tilesFor } from '../src/offline.ts';

// Around CSMT, and a larger box over South Mumbai to Bandra.
const csmt: BBox = [72.8355, 18.9399, 72.8355, 18.9399];
const southMumbai: BBox = [72.8, 18.9, 72.95, 19.1];

describe('offline tiles', () => {
  it('finds the tile under a point', () => {
    expect(tileRange(csmt, 0)).toEqual({ minX: 0, maxX: 0, minY: 0, maxY: 0 });
    expect(tileRange(csmt, 14)).toEqual({ minX: 11506, maxX: 11506, minY: 7313, maxY: 7313 });
    expect(tileRange(csmt, 15)).toEqual({ minX: 23013, maxX: 23013, minY: 14627, maxY: 14627 });
  });

  it('covers a box from its north-west to its south-east corner', () => {
    expect(tileRange(southMumbai, 12)).toEqual({ minX: 2876, maxX: 2878, minY: 1826, maxY: 1828 });
    expect(countTiles(southMumbai, 12, 12)).toBe(9);
    const tiles = tilesFor(southMumbai, 11, 12);
    expect(tiles).toHaveLength(countTiles(southMumbai, 11, 12));
    expect(tiles[0][0]).toBe(11);
    expect(new Set(tiles.map((t) => t.join('/'))).size).toBe(tiles.length);
  });

  it('plans a few zooms below the view up to the most detailed tiles', () => {
    const plan = offlinePlan([72.82, 18.92, 72.85, 18.95], 14.6, 15);
    expect(plan).toMatchObject({ minZoom: 11, maxZoom: 15, ok: true });
    expect(plan.tiles).toBe(countTiles([72.82, 18.92, 72.85, 18.95], 11, 15));
  });

  it('refuses areas too large to save', () => {
    const plan = offlinePlan([72.6, 18.8, 73.4, 19.6], 10, 15);
    expect(plan.tiles).toBeGreaterThan(MAX_TILES);
    expect(plan.ok).toBe(false);
  });
});
